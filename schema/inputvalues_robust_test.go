package schema_test

import (
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

type weirdIn struct {
	A  string
	Fn func()
}

// The same values on the way in: coercion, validation and rendering back to a
// literal all see whatever a caller supplied.
func TestSchema_SurvivesAnyInputValue(t *testing.T) {
	self := map[string]any{}
	self["self"] = self
	cyclic := make([]any, 1)
	cyclic[0] = cyclic

	values := map[string]any{
		"nil":                    nil,
		"a channel":              make(chan int),
		"a function":             func() {},
		"a cyclic map":           self,
		"a cyclic slice":         cyclic,
		"an uncomparable struct": weirdIn{A: "x", Fn: func() {}},
		"a non-string map":       map[int]any{1: 2},
		"a complex number":       complex(1, 2),
		"a typed nil":            (*weirdIn)(nil),
	}

	colour := schema.NewEnum(schema.EnumConfig{
		Name:   "Colour",
		Values: []*schema.EnumValue{schema.NewEnumValue("RED", schema.EnumValueConfig{})},
	})
	input := schema.NewInputObject(schema.InputObjectConfig{
		Name:   "In",
		Fields: []*schema.InputField{schema.NewInputField("a", schema.InputFieldConfig{Type: schema.String})},
	})
	types := map[string]schema.Type{
		"String":  schema.String,
		"Int":     schema.Int,
		"Float":   schema.Float,
		"Boolean": schema.Boolean,
		"ID":      schema.ID,
		"enum":    colour,
		"input":   input,
		"list":    schema.NewList(schema.String),
		"nonNull": schema.NewNonNull(schema.String),
	}

	for typeName, t2 := range types {
		for valueName, held := range values {
			t.Run(typeName+"/"+valueName, func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("PANIC: %v", r)
					}
				}()
				_, _ = schema.CoerceInputValue(held, t2)
				_ = schema.ValidateInputValue(value.Just(held), t2)
				_, _ = schema.LiteralFromValue(held, t2)
				if e, isEnum := t2.(*schema.EnumType); isEnum {
					_ = e.ValueFor(held)
				}
				_ = value.Describe(held)
			})
		}
	}
}
