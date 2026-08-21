package schema_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

func TestCoerceInputValue_Scalars(t *testing.T) {
	tests := []struct {
		name   string
		in     any
		typ    schema.Type
		want   any
		wantOK bool
	}{
		{"an Int", 1, schema.Int, int32(1), true},
		{"a String", "hi", schema.String, "hi", true},
		{"a Boolean", true, schema.Boolean, true, true},
		{"an ID from a number", 42, schema.ID, "42", true},
		{"a Float from an Int", 1, schema.Float, float64(1), true},

		{"a String where an Int is wanted", "1", schema.Int, nil, false},
		{"a number where a String is wanted", 1, schema.String, nil, false},
		{"a fraction where an Int is wanted", 1.5, schema.Int, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := schema.CoerceInputValue(tt.in, tt.typ)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (got %#v)", ok, tt.wantOK, got)
			}
			if ok && got != tt.want {
				t.Errorf("= %#v, want %#v", got, tt.want)
			}
		})
	}
}

// Null is a value, and whether it is allowed is the whole job of a non-null
// type.
func TestCoerceInputValue_Null(t *testing.T) {
	got, ok := schema.CoerceInputValue(nil, schema.String)
	if !ok {
		t.Error("null was rejected where the type allows it")
	}
	if got != nil {
		t.Errorf("= %#v, want nil", got)
	}

	if _, ok := schema.CoerceInputValue(nil, schema.NewNonNull(schema.String)); ok {
		t.Error("null was accepted where the type forbids it")
	}
}

func TestCoerceInputValue_Lists(t *testing.T) {
	listOfInt := schema.NewList(schema.Int)

	t.Run("a list", func(t *testing.T) {
		got, ok := schema.CoerceInputValue([]any{1, 2}, listOfInt)
		if !ok {
			t.Fatal("a list was rejected")
		}
		if !reflect.DeepEqual(got, []any{int32(1), int32(2)}) {
			t.Errorf("= %#v", got)
		}
	})

	// A single value stands for a list of one, which is what lets a caller
	// write one item without brackets.
	t.Run("a lone value becomes a list of one", func(t *testing.T) {
		got, ok := schema.CoerceInputValue(1, listOfInt)
		if !ok {
			t.Fatal("a lone value was rejected")
		}
		if !reflect.DeepEqual(got, []any{int32(1)}) {
			t.Errorf("= %#v, want a list of one", got)
		}
	})

	t.Run("a typed Go slice", func(t *testing.T) {
		got, ok := schema.CoerceInputValue([]int{1, 2}, listOfInt)
		if !ok || !reflect.DeepEqual(got, []any{int32(1), int32(2)}) {
			t.Errorf("= %#v, %v", got, ok)
		}
	})

	// A string is a sequence but not a list: writing one where a list of
	// strings is wanted means a list holding it, not a list of its characters.
	t.Run("a string is not taken apart", func(t *testing.T) {
		got, ok := schema.CoerceInputValue("hi", schema.NewList(schema.String))
		if !ok || !reflect.DeepEqual(got, []any{"hi"}) {
			t.Errorf("= %#v, %v, want a list holding the whole string", got, ok)
		}
	})

	t.Run("one bad element spoils the list", func(t *testing.T) {
		if _, ok := schema.CoerceInputValue([]any{1, "no"}, listOfInt); ok {
			t.Error("a list with a bad element was accepted")
		}
	})

	t.Run("null inside a list of nullables", func(t *testing.T) {
		got, ok := schema.CoerceInputValue([]any{1, nil}, listOfInt)
		if !ok || !reflect.DeepEqual(got, []any{int32(1), nil}) {
			t.Errorf("= %#v, %v", got, ok)
		}
	})

	t.Run("null inside a list of non-nulls", func(t *testing.T) {
		strict := schema.NewList(schema.NewNonNull(schema.Int))
		if _, ok := schema.CoerceInputValue([]any{1, nil}, strict); ok {
			t.Error("a null element was accepted where the element type forbids it")
		}
	})
}

// testInputObject has one field of each interesting shape.
func testInputObject() *schema.InputObjectType {
	return schema.NewInputObject(schema.InputObjectConfig{
		Name: "Filter",
		Fields: []*schema.InputField{
			schema.NewInputField("optional", schema.InputFieldConfig{Type: schema.String}),
			schema.NewInputField("defaulted", schema.InputFieldConfig{
				Type:    schema.String,
				Default: schema.DefaultValue("fallback"),
			}),
			schema.NewInputField("required", schema.InputFieldConfig{
				Type:    schema.NewNonNull(schema.String),
				Default: schema.DefaultValue("given"),
			}),
		},
	})
}

// This is the three states in one test. A field left out with no default stays
// out of the result; one left out with a default appears holding it; one given
// as null appears holding null. Collapsing any two of those would be wrong.
func TestCoerceInputValue_InputObjectThreeStates(t *testing.T) {
	got, ok := schema.CoerceInputValue(map[string]any{"optional": nil}, testInputObject())
	if !ok {
		t.Fatal("a sound value was rejected")
	}
	fields, isMap := got.(map[string]any)
	if !isMap {
		t.Fatalf("= %T, want a map", got)
	}

	// Given as null: present, holding nil.
	v, present := fields["optional"]
	if !present {
		t.Error("a field given as null is absent from the result")
	}
	if v != nil {
		t.Errorf("optional = %#v, want nil", v)
	}

	// Left out with a default: present, holding the default.
	if v, present := fields["defaulted"]; !present || v != "fallback" {
		t.Errorf("defaulted = %#v (present=%v), want the default", v, present)
	}
	if v, present := fields["required"]; !present || v != "given" {
		t.Errorf("required = %#v (present=%v), want the default", v, present)
	}

	// Left out with no default: absent entirely.
	bare, ok := schema.CoerceInputValue(map[string]any{}, testInputObject())
	if !ok {
		t.Fatal("an empty value was rejected")
	}
	if _, present := bare.(map[string]any)["optional"]; present {
		t.Error("a field left out with no default appears in the result")
	}
}

func TestCoerceInputValue_InputObjectRejections(t *testing.T) {
	strict := schema.NewInputObject(schema.InputObjectConfig{
		Name: "Strict",
		Fields: []*schema.InputField{
			schema.NewInputField("needed", schema.InputFieldConfig{Type: schema.NewNonNull(schema.String)}),
		},
	})

	tests := []struct {
		name string
		in   any
		typ  *schema.InputObjectType
	}{
		{"not an object at all", "hi", testInputObject()},
		{"a list", []any{1}, testInputObject()},
		{"a field the type does not have", map[string]any{"nope": 1}, testInputObject()},
		{"a required field left out", map[string]any{}, strict},
		{"a required field given as null", map[string]any{"needed": nil}, strict},
		{"a field of the wrong type", map[string]any{"optional": 1}, testInputObject()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, ok := schema.CoerceInputValue(tt.in, tt.typ); ok {
				t.Errorf("accepted, giving %#v", got)
			}
		})
	}
}

// Exactly one field of a oneOf input object is supplied, and it may not be
// null.
func TestCoerceInputValue_OneOf(t *testing.T) {
	oneOf := schema.NewInputObject(schema.InputObjectConfig{
		Name:    "Ref",
		IsOneOf: true,
		Fields: []*schema.InputField{
			schema.NewInputField("id", schema.InputFieldConfig{Type: schema.ID}),
			schema.NewInputField("name", schema.InputFieldConfig{Type: schema.String}),
		},
	})

	got, ok := schema.CoerceInputValue(map[string]any{"id": "1"}, oneOf)
	if !ok {
		t.Fatal("one field was rejected")
	}
	if !reflect.DeepEqual(got, map[string]any{"id": "1"}) {
		t.Errorf("= %#v", got)
	}

	for _, tt := range []struct {
		name string
		in   map[string]any
	}{
		{"nothing at all", map[string]any{}},
		{"two fields", map[string]any{"id": "1", "name": "a"}},
		{"one field given as null", map[string]any{"id": nil}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := schema.CoerceInputValue(tt.in, oneOf); ok {
				t.Error("accepted, want it rejected")
			}
		})
	}
}

func TestCoerceInputValue_Enum(t *testing.T) {
	colour := schema.NewEnum(schema.EnumConfig{
		Name: "Colour",
		Values: []*schema.EnumValue{
			schema.NewEnumValue("RED", schema.EnumValueConfig{Value: schema.InternalValue(1)}),
		},
	})

	// A caller names a member; the resolver sees the internal value.
	got, ok := schema.CoerceInputValue("RED", colour)
	if !ok || got != 1 {
		t.Errorf("= %#v, %v, want the internal value", got, ok)
	}
	for _, in := range []any{"BLUE", 1, true} {
		if _, ok := schema.CoerceInputValue(in, colour); ok {
			t.Errorf("%#v was accepted as a member", in)
		}
	}
}

func TestCoerceInputLiteral(t *testing.T) {
	colour := schema.NewEnum(schema.EnumConfig{
		Name:   "Colour",
		Values: []*schema.EnumValue{schema.NewEnumValue("RED", schema.EnumValueConfig{Value: schema.InternalValue(1)})},
	})

	tests := []struct {
		name    string
		literal language.Value
		typ     schema.Type
		want    any
		wantOK  bool
	}{
		{"an Int", &language.IntValue{Value: "1"}, schema.Int, int32(1), true},
		{"a String", &language.StringValue{Value: "hi"}, schema.String, "hi", true},
		{"a Boolean", &language.BooleanValue{Value: true}, schema.Boolean, true, true},
		{"null", &language.NullValue{}, schema.String, nil, true},
		{"an enum member", &language.EnumValue{Value: "RED"}, colour, 1, true},

		{"null where it is forbidden", &language.NullValue{}, schema.NewNonNull(schema.String), nil, false},
		{"a string where an Int is wanted", &language.StringValue{Value: "1"}, schema.Int, nil, false},
		{"an unknown enum member", &language.EnumValue{Value: "BLUE"}, colour, nil, false},
		{"a string where an enum is wanted", &language.StringValue{Value: "RED"}, colour, nil, false},
		{"nothing", nil, schema.String, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := schema.CoerceInputLiteral(tt.literal, tt.typ, schema.VariableValues{})
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (got %#v)", ok, tt.wantOK, got)
			}
			if ok && got != tt.want {
				t.Errorf("= %#v, want %#v", got, tt.want)
			}
		})
	}
}

// A variable stands for whatever it was given. One the caller left out makes
// the literal holding it unusable, which is how a field left out of a request
// stays left out rather than turning into null.
func TestCoerceInputLiteral_Variables(t *testing.T) {
	variable := &language.Variable{Name: &language.Name{Value: "v"}}

	got, ok := schema.CoerceInputLiteral(variable, schema.Int, schema.NewVariableValues(map[string]any{"v": int32(7)}, nil))
	if !ok || got != int32(7) {
		t.Errorf("= %#v, %v, want the supplied value", got, ok)
	}

	if _, ok := schema.CoerceInputLiteral(variable, schema.Int, schema.VariableValues{}); ok {
		t.Error("a variable that was not supplied was accepted")
	}

	// A variable supplied as null is a value, unless the type forbids it.
	supplied := map[string]any{"v": nil}
	if got, ok := schema.CoerceInputLiteral(variable, schema.Int, schema.NewVariableValues(supplied, nil)); !ok || got != nil {
		t.Errorf("= %#v, %v, want null", got, ok)
	}
	if _, ok := schema.CoerceInputLiteral(variable, schema.NewNonNull(schema.Int), schema.NewVariableValues(supplied, nil)); ok {
		t.Error("a null variable was accepted where the type forbids null")
	}
}

// A field written as a variable the caller left out counts as not written, so
// the field's default applies rather than the field becoming null.
func TestCoerceInputLiteral_MissingVariableFallsBackToDefault(t *testing.T) {
	in := schema.NewInputObject(schema.InputObjectConfig{
		Name: "Filter",
		Fields: []*schema.InputField{
			schema.NewInputField("term", schema.InputFieldConfig{
				Type:    schema.String,
				Default: schema.DefaultValue("fallback"),
			}),
		},
	})
	literal := &language.ObjectValue{Fields: []*language.ObjectField{{
		Name:  &language.Name{Value: "term"},
		Value: &language.Variable{Name: &language.Name{Value: "v"}},
	}}}

	got, ok := schema.CoerceInputLiteral(literal, in, schema.VariableValues{})
	if !ok {
		t.Fatal("rejected, want the default to apply")
	}
	if !reflect.DeepEqual(got, map[string]any{"term": "fallback"}) {
		t.Errorf("= %#v, want the default", got)
	}
}

func TestCoerceInputLiteral_ListsAndObjects(t *testing.T) {
	in := testInputObject()

	literal := &language.ObjectValue{Fields: []*language.ObjectField{{
		Name:  &language.Name{Value: "optional"},
		Value: &language.StringValue{Value: "x"},
	}}}
	got, ok := schema.CoerceInputLiteral(literal, in, schema.VariableValues{})
	if !ok {
		t.Fatal("a sound literal was rejected")
	}
	want := map[string]any{"optional": "x", "defaulted": "fallback", "required": "given"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("= %#v, want %#v", got, want)
	}

	list := &language.ListValue{Values: []language.Value{
		&language.IntValue{Value: "1"},
		&language.IntValue{Value: "2"},
	}}
	gotList, ok := schema.CoerceInputLiteral(list, schema.NewList(schema.Int), schema.VariableValues{})
	if !ok || !reflect.DeepEqual(gotList, []any{int32(1), int32(2)}) {
		t.Errorf("= %#v, %v", gotList, ok)
	}

	// A lone literal becomes a list of one, as a lone value does.
	single, ok := schema.CoerceInputLiteral(&language.IntValue{Value: "1"}, schema.NewList(schema.Int), schema.VariableValues{})
	if !ok || !reflect.DeepEqual(single, []any{int32(1)}) {
		t.Errorf("= %#v, %v", single, ok)
	}

	// A field the type does not have.
	unknown := &language.ObjectValue{Fields: []*language.ObjectField{{
		Name:  &language.Name{Value: "nope"},
		Value: &language.IntValue{Value: "1"},
	}}}
	if _, ok := schema.CoerceInputLiteral(unknown, in, schema.VariableValues{}); ok {
		t.Error("a literal naming an unknown field was accepted")
	}
}

func TestCoerceDefaultInput(t *testing.T) {
	t.Run("no default", func(t *testing.T) {
		if _, ok := schema.CoerceDefaultInput(schema.NoDefault(), schema.String); ok {
			t.Error("an absent default was reported as present")
		}
	})

	t.Run("a Go value", func(t *testing.T) {
		got, ok := schema.CoerceDefaultInput(schema.DefaultValue(1), schema.Int)
		if !ok || got != int32(1) {
			t.Errorf("= %#v, %v", got, ok)
		}
	})

	t.Run("a literal", func(t *testing.T) {
		def := schema.DefaultLiteral(&language.IntValue{Value: "2"})
		got, ok := schema.CoerceDefaultInput(def, schema.Int)
		if !ok || got != int32(2) {
			t.Errorf("= %#v, %v", got, ok)
		}
	})

	// A default of null is a default, and it survives conversion as null.
	t.Run("a default of null", func(t *testing.T) {
		got, ok := schema.CoerceDefaultInput(schema.DefaultValue(nil), schema.String)
		if !ok || got != nil {
			t.Errorf("= %#v, %v, want null", got, ok)
		}
	})

	t.Run("a default that does not fit its type", func(t *testing.T) {
		if _, ok := schema.CoerceDefaultInput(schema.DefaultValue("no"), schema.Int); ok {
			t.Error("a default that does not fit was accepted")
		}
	})
}

func TestValueFromASTUntyped(t *testing.T) {
	tests := []struct {
		name    string
		literal language.Value
		want    any
	}{
		{"an Int", &language.IntValue{Value: "1"}, int64(1)},
		{"a Float", &language.FloatValue{Value: "1.5"}, 1.5},
		{"a String", &language.StringValue{Value: "hi"}, "hi"},
		{"a Boolean", &language.BooleanValue{Value: true}, true},
		{"null", &language.NullValue{}, nil},
		// Without a type there is nothing to tell an enum member from a
		// string, so both come out as strings.
		{"an enum member", &language.EnumValue{Value: "RED"}, "RED"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := schema.ValueFromASTUntyped(tt.literal, schema.VariableValues{})
			if !ok {
				t.Fatal("rejected")
			}
			if got != tt.want {
				t.Errorf("= %#v, want %#v", got, tt.want)
			}
		})
	}

	list, ok := schema.ValueFromASTUntyped(&language.ListValue{Values: []language.Value{
		&language.IntValue{Value: "1"},
	}}, schema.VariableValues{})
	if !ok || !reflect.DeepEqual(list, []any{int64(1)}) {
		t.Errorf("a list = %#v, %v", list, ok)
	}

	object, ok := schema.ValueFromASTUntyped(&language.ObjectValue{Fields: []*language.ObjectField{{
		Name:  &language.Name{Value: "a"},
		Value: &language.IntValue{Value: "1"},
	}}}, schema.VariableValues{})
	if !ok || !reflect.DeepEqual(object, map[string]any{"a": int64(1)}) {
		t.Errorf("an object = %#v, %v", object, ok)
	}

	if _, ok := schema.ValueFromASTUntyped(nil, schema.VariableValues{}); ok {
		t.Error("nothing was accepted")
	}
}

// An integer too large for an int64 keeps its digits rather than being widened
// to a float64, so that an identifier written out in full survives.
func TestValueFromASTUntyped_LargeInteger(t *testing.T) {
	const huge = "99999999999999999999"
	got, ok := schema.ValueFromASTUntyped(&language.IntValue{Value: huge}, schema.VariableValues{})
	if !ok {
		t.Fatal("rejected")
	}
	coerced, err := schema.ID.CoerceInputValue(got)
	if err != nil {
		t.Fatalf("ID rejected it: %v", err)
	}
	if coerced.Or(nil) != huge {
		t.Errorf("= %v, want the digits unchanged", coerced)
	}
}

// A large identifier written as a literal survives the whole way through.
func TestCoerceInputLiteral_KeepsLargeIdentifiers(t *testing.T) {
	const huge = "99999999999999999999"
	got, ok := schema.CoerceInputLiteral(&language.IntValue{Value: huge}, schema.ID, schema.VariableValues{})
	if !ok {
		t.Fatal("rejected")
	}
	if got != huge {
		t.Errorf("= %v, want the digits unchanged", got)
	}
}

// The order of an input object's fields in the result does not matter, but its
// contents do; this pins the set so a stray field would be noticed.
func TestCoerceInputValue_ResultHoldsOnlyKnownFields(t *testing.T) {
	got, ok := schema.CoerceInputValue(map[string]any{"optional": "x"}, testInputObject())
	if !ok {
		t.Fatal("rejected")
	}
	var names []string
	for name := range got.(map[string]any) {
		names = append(names, name)
	}
	slices.Sort(names)
	want := []string{"defaulted", "optional", "required"}
	if !slices.Equal(names, want) {
		t.Errorf("fields = %v, want %v", names, want)
	}
}
