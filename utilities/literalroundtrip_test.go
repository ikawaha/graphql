package utilities_test

import (
	"reflect"
	"testing"

	"github.com/ikawaha/graphql/schema"
)

// Rendering a value as a literal and reading that literal back has to give the
// value again. The two directions are written separately, so without this they
// could drift: a default would be printed into a schema that, read back, meant
// something else.
func TestLiteral_RoundTripsThroughCoercion(t *testing.T) {
	colour := schema.NewEnum(schema.EnumConfig{
		Name: "Colour",
		Values: []*schema.EnumValue{
			schema.NewEnumValue("RED", schema.EnumValueConfig{}),
			schema.NewEnumValue("GREEN", schema.EnumValueConfig{Value: schema.InternalValue(2)}),
		},
	})
	filter := schema.NewInputObject(schema.InputObjectConfig{
		Name: "Filter",
		Fields: []*schema.InputField{
			schema.NewInputField("term", schema.InputFieldConfig{Type: schema.String}),
			schema.NewInputField("limit", schema.InputFieldConfig{Type: schema.Int}),
		},
	})

	tests := []struct {
		name string
		in   any
		typ  schema.Type
		want any
	}{
		{"an Int", int32(1), schema.Int, int32(1)},
		{"a negative Int", int32(-1), schema.Int, int32(-1)},
		{"a Float", 1.5, schema.Float, 1.5},
		{"a whole Float", float64(1), schema.Float, float64(1)},
		{"a String", "hi", schema.String, "hi"},
		{"a String with quotes", `a"b`, schema.String, `a"b`},
		{"a Boolean", true, schema.Boolean, true},
		{"an ID", "abc", schema.ID, "abc"},
		{"null", nil, schema.String, nil},
		{"an enum member", "RED", colour, "RED"},
		{"an enum member with its own value", 2, colour, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			literal, ok := schema.LiteralFromValue(tt.in, tt.typ)
			if !ok {
				t.Fatalf("%#v could not be written as %s", tt.in, tt.typ)
			}
			got, ok := schema.CoerceInputLiteral(literal, tt.typ, schema.VariableValues{})
			if !ok {
				t.Fatalf("the literal it produced could not be read back")
			}
			if got != tt.want {
				t.Errorf("round trip = %#v, want %#v", got, tt.want)
			}
		})
	}

	// A structured value round trips too, though its defaults are filled in on
	// the way back, which is what reading a value against a type does.
	literal, ok := schema.LiteralFromValue(map[string]any{"term": "x", "limit": 10}, filter)
	if !ok {
		t.Fatal("the input object could not be written")
	}
	got, ok := schema.CoerceInputLiteral(literal, filter, schema.VariableValues{})
	if !ok {
		t.Fatal("the literal it produced could not be read back")
	}
	want := map[string]any{"term": "x", "limit": int32(10)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip = %#v, want %#v", got, want)
	}

	list, ok := schema.LiteralFromValue([]any{1, 2}, schema.NewList(schema.Int))
	if !ok {
		t.Fatal("the list could not be written")
	}
	gotList, ok := schema.CoerceInputLiteral(list, schema.NewList(schema.Int), schema.VariableValues{})
	if !ok || !reflect.DeepEqual(gotList, []any{int32(1), int32(2)}) {
		t.Errorf("round trip = %#v, %v", gotList, ok)
	}
}

// Whatever the conversion refuses is exactly what has no literal form, so a
// value that survives it always produces something readable.
func TestLiteral_RefusedValuesHaveNoForm(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   any
		typ  schema.Type
	}{
		{"a string where an Int belongs", "no", schema.Int},
		{"a number where a String belongs", 1, schema.String},
		{"a value naming no enum member", "BLUE", schema.NewEnum(schema.EnumConfig{
			Name:   "Colour",
			Values: []*schema.EnumValue{schema.NewEnumValue("RED", schema.EnumValueConfig{})},
		})},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if literal, ok := schema.LiteralFromValue(tt.in, tt.typ); ok {
				t.Errorf("written as %v, want it refused", literal)
			}
		})
	}
}
