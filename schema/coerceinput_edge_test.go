package schema_test

import (
	"errors"
	"github.com/ikawaha/graphql/value"
	"reflect"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// A value arriving from Go code, rather than from a JSON decoder, may be a map
// of a concrete type rather than a map[string]any.
func TestCoerceInputValue_TypedMaps(t *testing.T) {
	got, ok := schema.CoerceInputValue(map[string]string{"optional": "x"}, testInputObject())
	if !ok {
		t.Fatal("a typed map was rejected")
	}
	if v := got.(map[string]any)["optional"]; v != "x" {
		t.Errorf("optional = %#v, want x", v)
	}

	// A map keyed by something other than a string is not an input object.
	if _, ok := schema.CoerceInputValue(map[int]any{1: "x"}, testInputObject()); ok {
		t.Error("a map with non-string keys was accepted")
	}
}

// A type that cannot appear in an input position has no conversion at all. The
// schema validator reports the mistake; here the value simply does not fit.
func TestCoerceInputValue_NonInputType(t *testing.T) {
	object := schema.NewObject(schema.ObjectConfig{
		Name:   "User",
		Fields: []*schema.Field{schema.NewField("a", schema.FieldConfig{Type: schema.String})},
	})
	if _, ok := schema.CoerceInputValue("x", object); ok {
		t.Error("an object type accepted an input value")
	}
	if _, ok := schema.CoerceInputLiteral(&language.StringValue{Value: "x"}, object, schema.VariableValues{}); ok {
		t.Error("an object type accepted a literal")
	}
}

// A scalar that reads literals itself is handed the node, so that it can
// accept a syntax the generic conversion would not.
func TestCoerceInputLiteral_CustomScalar(t *testing.T) {
	rejected := errors.New("no")
	custom := schema.NewScalar(schema.ScalarConfig{
		Name: "Odd",
		CoerceInputLiteral: func(literal language.Value) (value.Maybe[any], error) {
			lit, isEnum := literal.(*language.EnumValue)
			if !isEnum {
				return value.Nothing[any](), rejected
			}
			return value.Just[any]("read:" + lit.Value), nil
		},
	})

	// An enum literal would never reach an ordinary scalar, but this one takes
	// it.
	got, ok := schema.CoerceInputLiteral(&language.EnumValue{Value: "ODD"}, custom, schema.VariableValues{})
	if !ok || got != "read:ODD" {
		t.Errorf("= %#v, %v, want the scalar's own reading", got, ok)
	}
	if _, ok := schema.CoerceInputLiteral(&language.IntValue{Value: "1"}, custom, schema.VariableValues{}); ok {
		t.Error("the scalar's refusal was ignored")
	}
}

// A scalar with no opinion about literals gets the literal reduced to a plain
// value first, and may still refuse it.
func TestCoerceInputLiteral_GenericScalarPath(t *testing.T) {
	if _, ok := schema.CoerceInputLiteral(&language.FloatValue{Value: "1.5"}, schema.Int, schema.VariableValues{}); ok {
		t.Error("Int accepted a float literal")
	}
	got, ok := schema.CoerceInputLiteral(&language.IntValue{Value: "1"}, schema.Float, schema.VariableValues{})
	if !ok || got != float64(1) {
		t.Errorf("= %#v, %v, want an integer literal to serve as a Float", got, ok)
	}
}

func TestCoerceInputLiteral_ListFailures(t *testing.T) {
	listOfInt := schema.NewList(schema.Int)

	bad := &language.ListValue{Values: []language.Value{
		&language.IntValue{Value: "1"},
		&language.StringValue{Value: "no"},
	}}
	if _, ok := schema.CoerceInputLiteral(bad, listOfInt, schema.VariableValues{}); ok {
		t.Error("a list with a bad element was accepted")
	}

	// An entry written as a variable the caller left out is null, which a list
	// may hold. Where the entries may not be null there is nothing to put in
	// its place, and the list as a whole is unusable.
	missing := &language.ListValue{Values: []language.Value{
		&language.Variable{Name: &language.Name{Value: "v"}},
	}}
	held, ok := schema.CoerceInputLiteral(missing, listOfInt, schema.VariableValues{})
	if !ok {
		t.Error("a list holding an unsupplied variable was refused")
	} else if items, isList := held.([]any); !isList || len(items) != 1 || items[0] != nil {
		t.Errorf("the list came out as %#v, want one null in it", held)
	}
	if _, ok := schema.CoerceInputLiteral(
		missing, schema.NewList(schema.NewNonNull(schema.Int)), schema.VariableValues{}); ok {
		t.Error("a list of non-nulls holding an unsupplied variable was accepted")
	}

	// A lone literal that does not fit is rejected rather than wrapped.
	if _, ok := schema.CoerceInputLiteral(&language.StringValue{Value: "no"}, listOfInt, schema.VariableValues{}); ok {
		t.Error("a lone value of the wrong type was wrapped into a list")
	}
}

func TestCoerceInputLiteral_ObjectFailures(t *testing.T) {
	strict := schema.NewInputObject(schema.InputObjectConfig{
		Name: "Strict",
		Fields: []*schema.InputField{
			schema.NewInputField("needed", schema.InputFieldConfig{Type: schema.NewNonNull(schema.String)}),
		},
	})

	t.Run("not an object literal", func(t *testing.T) {
		if _, ok := schema.CoerceInputLiteral(&language.IntValue{Value: "1"}, strict, schema.VariableValues{}); ok {
			t.Error("accepted")
		}
	})

	t.Run("a required field left out", func(t *testing.T) {
		if _, ok := schema.CoerceInputLiteral(&language.ObjectValue{}, strict, schema.VariableValues{}); ok {
			t.Error("accepted")
		}
	})

	// A required field written as a variable the caller left out is still
	// missing, so it is refused rather than silently defaulted.
	t.Run("a required field written as an unsupplied variable", func(t *testing.T) {
		literal := &language.ObjectValue{Fields: []*language.ObjectField{{
			Name:  &language.Name{Value: "needed"},
			Value: &language.Variable{Name: &language.Name{Value: "v"}},
		}}}
		if _, ok := schema.CoerceInputLiteral(literal, strict, schema.VariableValues{}); ok {
			t.Error("accepted")
		}
	})

	t.Run("a field of the wrong type", func(t *testing.T) {
		literal := &language.ObjectValue{Fields: []*language.ObjectField{{
			Name:  &language.Name{Value: "needed"},
			Value: &language.IntValue{Value: "1"},
		}}}
		if _, ok := schema.CoerceInputLiteral(literal, strict, schema.VariableValues{}); ok {
			t.Error("accepted")
		}
	})

	t.Run("a malformed literal", func(t *testing.T) {
		literal := &language.ObjectValue{Fields: []*language.ObjectField{nil}}
		if _, ok := schema.CoerceInputLiteral(literal, strict, schema.VariableValues{}); ok {
			t.Error("a literal with a missing field was accepted")
		}
	})
}

func TestCoerceInputLiteral_OneOf(t *testing.T) {
	oneOf := schema.NewInputObject(schema.InputObjectConfig{
		Name:    "Ref",
		IsOneOf: true,
		Fields: []*schema.InputField{
			schema.NewInputField("id", schema.InputFieldConfig{Type: schema.ID}),
			schema.NewInputField("name", schema.InputFieldConfig{Type: schema.String}),
		},
	})
	field := func(name string, v language.Value) *language.ObjectField {
		return &language.ObjectField{Name: &language.Name{Value: name}, Value: v}
	}

	good := &language.ObjectValue{Fields: []*language.ObjectField{
		field("id", &language.StringValue{Value: "1"}),
	}}
	got, ok := schema.CoerceInputLiteral(good, oneOf, schema.VariableValues{})
	if !ok || !reflect.DeepEqual(got, map[string]any{"id": "1"}) {
		t.Errorf("= %#v, %v", got, ok)
	}

	for _, tt := range []struct {
		name    string
		literal *language.ObjectValue
	}{
		{"nothing at all", &language.ObjectValue{}},
		{"two fields", &language.ObjectValue{Fields: []*language.ObjectField{
			field("id", &language.StringValue{Value: "1"}),
			field("name", &language.StringValue{Value: "a"}),
		}}},
		{"one field written as null", &language.ObjectValue{Fields: []*language.ObjectField{
			field("id", &language.NullValue{}),
		}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := schema.CoerceInputLiteral(tt.literal, oneOf, schema.VariableValues{}); ok {
				t.Error("accepted, want it rejected")
			}
		})
	}
}

// A variable node with no name is malformed rather than merely unsupplied.
func TestCoerceInputLiteral_MalformedVariable(t *testing.T) {
	if _, ok := schema.CoerceInputLiteral(&language.Variable{}, schema.Int, schema.NewVariableValues(map[string]any{"v": 1}, nil)); ok {
		t.Error("a variable with no name was accepted")
	}
}

func TestValueFromASTUntyped_Failures(t *testing.T) {
	tests := []struct {
		name    string
		literal language.Value
	}{
		{"a float that is not a number", &language.FloatValue{Value: "not a number"}},
		{"a list holding an unsupplied variable", &language.ListValue{Values: []language.Value{
			&language.Variable{Name: &language.Name{Value: "v"}},
		}}},
		{"an object holding an unsupplied variable", &language.ObjectValue{Fields: []*language.ObjectField{{
			Name:  &language.Name{Value: "a"},
			Value: &language.Variable{Name: &language.Name{Value: "v"}},
		}}}},
		{"an object with a missing field", &language.ObjectValue{Fields: []*language.ObjectField{nil}}},
		{"a variable with no name", &language.Variable{}},
		// There is no case here for "a node that is not a value": the marker
		// method on the Value interface is unexported, so nothing outside the
		// language package can be one, and the compiler refuses such a test.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, ok := schema.ValueFromASTUntyped(tt.literal, schema.VariableValues{}); ok {
				t.Errorf("accepted, giving %#v", got)
			}
		})
	}
}

func TestValueFromASTUntyped_Variables(t *testing.T) {
	variable := &language.Variable{Name: &language.Name{Value: "v"}}

	got, ok := schema.ValueFromASTUntyped(variable, schema.NewVariableValues(map[string]any{"v": 7}, nil))
	if !ok || got != 7 {
		t.Errorf("= %#v, %v, want the supplied value", got, ok)
	}
	// Supplied as null is still supplied.
	if got, ok := schema.ValueFromASTUntyped(variable, schema.NewVariableValues(map[string]any{"v": nil}, nil)); !ok || got != nil {
		t.Errorf("= %#v, %v, want null", got, ok)
	}
	if _, ok := schema.ValueFromASTUntyped(variable, schema.VariableValues{}); ok {
		t.Error("an unsupplied variable was accepted")
	}
}

// A scalar with no opinion about literals sees the literal reduced to a plain
// value first, so a literal that cannot be reduced never reaches it.
func TestCoerceInputLiteral_ScalarPathFailures(t *testing.T) {
	// A variable the caller left out cannot be reduced.
	unsupplied := &language.Variable{Name: &language.Name{Value: "v"}}
	if _, ok := schema.CoerceInputLiteral(unsupplied, schema.String, schema.VariableValues{}); ok {
		t.Error("a scalar accepted an unsupplied variable")
	}

	// A list holding one cannot be reduced either, and a scalar refuses a list
	// in any case.
	list := &language.ListValue{Values: []language.Value{unsupplied}}
	if _, ok := schema.CoerceInputLiteral(list, schema.String, schema.VariableValues{}); ok {
		t.Error("a scalar accepted a list")
	}

	// A float literal that is not a number is refused before the scalar sees
	// it.
	if _, ok := schema.CoerceInputLiteral(&language.FloatValue{Value: "nope"}, schema.Float, schema.VariableValues{}); ok {
		t.Error("Float accepted digits that are not a number")
	}

	// A scalar that reads literals itself is still asked, even for a list.
	seen := 0
	counting := schema.NewScalar(schema.ScalarConfig{
		Name: "Counting",
		CoerceInputLiteral: func(language.Value) (value.Maybe[any], error) {
			seen++
			return value.Just[any]("ok"), nil
		},
	})
	if _, ok := schema.CoerceInputLiteral(list, counting, schema.VariableValues{}); !ok || seen != 1 {
		t.Errorf("the scalar was asked %d times, want it asked once and accepted", seen)
	}
}

// Every built-in scalar reads literals itself, so the fallback path is only
// taken by a scalar that does not, which is what most user-defined scalars
// are. The literal is reduced to a plain value and handed to the ordinary
// input conversion.
func TestCoerceInputLiteral_FallbackForPlainScalars(t *testing.T) {
	plain := schema.NewScalar(schema.ScalarConfig{
		Name: "Upper",
		CoerceInputValue: func(v any) (value.Maybe[any], error) {
			s, isString := v.(string)
			if !isString {
				return value.Nothing[any](), errors.New("Upper cannot represent a non string value")
			}
			return value.Just[any](strings.ToUpper(s)), nil
		},
	})
	if plain.CoerceInputLiteral != nil {
		t.Fatal("this scalar was meant to have no literal reader")
	}

	got, ok := schema.CoerceInputLiteral(&language.StringValue{Value: "hi"}, plain, schema.VariableValues{})
	if !ok || got != "HI" {
		t.Errorf("= %#v, %v, want the scalar's conversion of the reduced value", got, ok)
	}

	// The scalar's refusal of the reduced value comes through.
	if _, ok := schema.CoerceInputLiteral(&language.IntValue{Value: "1"}, plain, schema.VariableValues{}); ok {
		t.Error("the scalar's refusal was ignored")
	}

	// A literal that cannot be reduced never reaches the scalar.
	unsupplied := &language.Variable{Name: &language.Name{Value: "v"}}
	list := &language.ListValue{Values: []language.Value{unsupplied}}
	if _, ok := schema.CoerceInputLiteral(list, plain, schema.VariableValues{}); ok {
		t.Error("a literal that could not be reduced was accepted")
	}
}
