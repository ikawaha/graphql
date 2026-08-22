package schema_test

import (
	"strings"
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// These cover the parts of the package a caller can reach but nothing else in
// the tests did. Code that has never run is code nobody has checked.

// TestParent covers what an argument and an input object field say about
// where they belong, which is what an error message about one is built from.
func TestParent(t *testing.T) {
	arg := schema.NewArgument("a", schema.ArgumentConfig{Type: schema.String})
	field := schema.NewField("f", schema.FieldConfig{
		Type: schema.String,
		Args: []*schema.Argument{arg},
	})
	object := schema.NewObject(schema.ObjectConfig{
		Name:   "Query",
		Fields: []*schema.Field{field},
	})
	// A type takes its configuration rather than borrowing it, so the argument
	// that belongs to the object is the one reached through it.
	held := object.Field("f").Arg("a")
	parent, isField := held.Parent().(*schema.Field)
	if !isField {
		t.Fatalf("the argument's parent is %T, wanted a field", held.Parent())
	}
	if got, want := parent.String(), "Query.f"; got != want {
		t.Errorf("parent = %q, want %q", got, want)
	}

	// One that was never given to a field or a directive belongs nowhere.
	if loose := schema.NewArgument("b", schema.ArgumentConfig{Type: schema.String}); loose.Parent() != nil {
		t.Errorf("a loose argument says it belongs to %v", loose.Parent())
	}
	var absent *schema.Argument
	if absent.Parent() != nil {
		t.Error("a nil argument says it belongs somewhere")
	}

	input := schema.NewInputObject(schema.InputObjectConfig{
		Name: "In",
		Fields: []*schema.InputField{
			schema.NewInputField("a", schema.InputFieldConfig{Type: schema.String}),
		},
	})
	if got := input.Field("a").Parent(); got != input {
		t.Errorf("the input field's parent is %v, wanted the input object", got)
	}
}

// TestToConfig covers taking a built type apart again, which is what a schema
// mapper does before it puts one back together.
func TestToConfig(t *testing.T) {
	t.Run("a field", func(t *testing.T) {
		field := schema.NewField("f", schema.FieldConfig{
			Description:       value.Just("what it is"),
			Type:              schema.NewNonNull(schema.Int),
			Args:              []*schema.Argument{schema.NewArgument("a", schema.ArgumentConfig{Type: schema.String})},
			DeprecationReason: schema.DeprecatedFor("gone"),
		})
		config := field.ToConfig()
		if config.Description != value.Just("what it is") {
			t.Errorf("description = %v", config.Description)
		}
		if config.Type != field.Type {
			t.Errorf("type = %v", config.Type)
		}
		if len(config.Args) != 1 || config.Args[0].Name() != "a" {
			t.Errorf("args = %v", config.Args)
		}
		if config.DeprecationReason != schema.DeprecatedFor("gone") {
			t.Errorf("deprecation = %v", config.DeprecationReason)
		}
		// The list is a copy: adding to it must not reach the field.
		config.Args = append(config.Args, schema.NewArgument("b", schema.ArgumentConfig{Type: schema.String}))
		if len(field.Args) != 1 {
			t.Error("the field gained an argument from its own configuration")
		}
	})

	t.Run("an argument", func(t *testing.T) {
		arg := schema.NewArgument("a", schema.ArgumentConfig{
			Type:    schema.Int,
			Default: schema.DefaultValue(7),
		})
		config := arg.ToConfig()
		if config.Type != schema.Int {
			t.Errorf("type = %v", config.Type)
		}
		held, given := config.Default.Get()
		if !given || held.Value != 7 {
			t.Errorf("default = %v (given %v)", held, given)
		}
	})

	t.Run("an input object field", func(t *testing.T) {
		f := schema.NewInputField("a", schema.InputFieldConfig{
			Type:    schema.String,
			Default: schema.DefaultValue("x"),
		})
		config := f.ToConfig()
		if config.Type != schema.String {
			t.Errorf("type = %v", config.Type)
		}
		if held, given := config.Default.Get(); !given || held.Value != "x" {
			t.Errorf("default = %v (given %v)", held, given)
		}
	})

	t.Run("an enum member", func(t *testing.T) {
		v := schema.NewEnumValue("RED", schema.EnumValueConfig{
			Value:       schema.InternalValue(1),
			Description: value.Just("the colour"),
		})
		config := v.ToConfig()
		if held, given := config.Value.Get(); !given || held != 1 {
			t.Errorf("value = %v (given %v)", held, given)
		}
		if config.Description != value.Just("the colour") {
			t.Errorf("description = %v", config.Description)
		}
	})
}

// TestVariableValues covers the map a fragment scope is built from.
func TestVariableValues(t *testing.T) {
	outer := schema.NewVariableValues(
		map[string]any{"a": 1, "b": 2},
		map[string]schema.Type{"a": schema.Int, "b": schema.Int},
	)
	if got := outer.Len(); got != 2 {
		t.Errorf("Len = %d, want 2", got)
	}

	// A copy can be written to without the original noticing, which is what
	// makes it safe to build a fragment's scope from the request's variables.
	scope := outer.Clone()
	scope.Set("c", 3, schema.String)
	scope.Delete("a")
	if outer.Len() != 2 {
		t.Errorf("the original changed: Len = %d", outer.Len())
	}
	if _, has := outer.Get("c"); has {
		t.Error("the original gained c")
	}
	if _, has := outer.Get("a"); !has {
		t.Error("the original lost a")
	}
	if got := scope.Len(); got != 2 {
		t.Errorf("the copy has %d, want 2", got)
	}
	if held, has := scope.Get("c"); !has || held != 3 {
		t.Errorf("c in the copy = %v (has %v)", held, has)
	}
	if got := scope.TypeOf("c"); got != schema.String {
		t.Errorf("the type of c = %v", got)
	}

	// Setting with no declared type forgets the type but keeps the value,
	// which is what a fragment argument with no signature comes to.
	scope.Set("c", 4, nil)
	if held, has := scope.Get("c"); !has || held != 4 {
		t.Errorf("c after setting again = %v (has %v)", held, has)
	}
	if got := scope.TypeOf("c"); got != nil {
		t.Errorf("the type of c = %v, want none", got)
	}

	// The zero value takes a write without being made first.
	var fresh schema.VariableValues
	fresh.Set("a", 1, schema.Int)
	if held, has := fresh.Get("a"); !has || held != 1 {
		t.Errorf("a = %v (has %v)", held, has)
	}
	if got := fresh.TypeOf("a"); got != schema.Int {
		t.Errorf("the type of a = %v", got)
	}
}

// TestNestedDefaultFailure covers the explanation for a default written on a
// field of an input object nested inside a type, where that default will not
// coerce. An executor says this where the schema was never validated.
func TestNestedDefaultFailure(t *testing.T) {
	sound := schema.NewInputObject(schema.InputObjectConfig{
		Name: "Sound",
		Fields: []*schema.InputField{
			schema.NewInputField("a", schema.InputFieldConfig{
				Type: schema.Int, Default: schema.DefaultValue(1),
			}),
		},
	})
	// A field whose default is not of the field's type.
	unsound := schema.NewInputObject(schema.InputObjectConfig{
		Name: "Unsound",
		Fields: []*schema.InputField{
			schema.NewInputField("a", schema.InputFieldConfig{
				Type: schema.Int, Default: schema.DefaultValue("not a number"),
			}),
		},
	})
	// An input object holding one, to show the walk goes inwards.
	outer := schema.NewInputObject(schema.InputObjectConfig{
		Name: "Outer",
		Fields: []*schema.InputField{
			schema.NewInputField("inner", schema.InputFieldConfig{Type: unsound}),
		},
	})
	// One that holds itself, to show the walk still ends.
	var cyclic *schema.InputObjectType
	cyclic = schema.NewInputObject(schema.InputObjectConfig{
		Name: "Cyclic",
		FieldsThunk: func() []*schema.InputField {
			return []*schema.InputField{
				schema.NewInputField("self", schema.InputFieldConfig{Type: cyclic}),
			}
		},
	})

	tests := []struct {
		name  string
		typ   schema.Type
		fails bool
	}{
		{"a scalar has nothing nested", schema.String, false},
		{"every default sound", sound, false},
		{"a default that will not coerce", unsound, true},
		{"nested inside another", outer, true},
		{"reached through a list and a non-null",
			schema.NewList(schema.NewNonNull(unsound)), true},
		{"an input object holding itself", cyclic, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			why := schema.NestedDefaultFailure(test.typ)
			if test.fails && why == "" {
				t.Error("a default that cannot be written was allowed")
			}
			if !test.fails && why != "" {
				t.Errorf("a sound type was refused: %s", why)
			}
			if test.fails && !strings.Contains(why, "not a number") {
				t.Errorf("the message does not name the value: %s", why)
			}
		})
	}
}
