package schema_test

import (
	"fmt"
	"testing"

	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
	"github.com/ikawaha/graphql/value"
)

// testEnumWithInternalValues is the enum both suggested-fix cases use: its
// members stand for values a resolver would use rather than for their own
// names, which is the mistake the suggestion is there to catch.
func testEnumWithInternalValues() *schema.EnumType {
	return schema.NewEnum(schema.EnumConfig{
		Name: "TestEnum",
		Values: []*schema.EnumValue{
			schema.NewEnumValue("ONE", schema.EnumValueConfig{Value: schema.InternalValue(1)}),
			schema.NewEnumValue("TWO", schema.EnumValueConfig{Value: schema.InternalValue(2)}),
		},
	})
}

// wantSuggestedFix is what validating either of the two schemas below has to
// say. Only the first argument has a fix to suggest; the other two are there
// to show what happens when there is none.
//
// The one departure from graphql-js is the order the fields of the default are
// written in. A Go map has no order of its own, so a message that named one
// would differ from run to run; ours are named in the order a person would
// look them up. graphql-js writes them in the order the object was built.
var wantSuggestedFix = []string{
	`Query.field(argWithPossibleFix:) has invalid default value: { enum: 2, self: null, string: [1] }. Did you mean: { enum: ["TWO"], self: null, string: ["1"] }?`,
	`Query.field(argWithInvalidPossibleFix:) has invalid default value at .string: Expected value of non-null type "[String]!" not to be null.`,
	`Query.field(argWithoutPossibleFix:) has invalid default value: Expected value of type "TestInput" to include required field "string", found: { enum: "Exotic" }.`,
	`Query.field(argWithoutPossibleFix:) has invalid default value at .enum: Value "Exotic" does not exist in "TestEnum" enum.`,
}

// The three defaults the cases install, held apart from how the schema is
// built so that both ways of building it install the same ones.
//
// graphql-js uses a Symbol for the exotic enum value, to make the point that
// an internal value need not be anything a schema could write down. An integer
// makes the same point in Go and is what an enum built in code usually holds.
var defaultsWithPossibleFix = map[string]any{
	"argWithPossibleFix":        map[string]any{"self": nil, "string": []any{1}, "enum": 2},
	"argWithInvalidPossibleFix": map[string]any{"string": nil},
	"argWithoutPossibleFix":     map[string]any{"enum": "Exotic"},
}

// Ported from graphql-js src/type/__tests__/validation-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPortedSchemaValidation_SuggestedFixForDefaultValue(t *testing.T) {
	t.Run("programmatic", func(t *testing.T) {
		enum := testEnumWithInternalValues()
		var input *schema.InputObjectType
		input = schema.NewInputObject(schema.InputObjectConfig{
			Name: "TestInput",
			FieldsThunk: func() []*schema.InputField {
				return []*schema.InputField{
					schema.NewInputField("self", schema.InputFieldConfig{Type: input}),
					schema.NewInputField("string", schema.InputFieldConfig{
						Type: schema.NewNonNull(schema.NewList(schema.String)),
					}),
					schema.NewInputField("enum", schema.InputFieldConfig{Type: schema.NewList(enum)}),
				}
			},
		})

		args := make([]*schema.Argument, 0, len(defaultsWithPossibleFix))
		for _, name := range []string{"argWithPossibleFix", "argWithInvalidPossibleFix", "argWithoutPossibleFix"} {
			args = append(args, schema.NewArgument(name, schema.ArgumentConfig{
				Type:    input,
				Default: schema.DefaultValue(defaultsWithPossibleFix[name]),
			}))
		}
		s := schema.New(schema.Config{
			Query: schema.NewObject(schema.ObjectConfig{
				Name: "Query",
				Fields: []*schema.Field{
					schema.NewField("field", schema.FieldConfig{Type: schema.Int, Args: args}),
				},
			}),
		})

		assertSchemaErrors(t, s, wantSuggestedFix)
	})

	t.Run("SDL", func(t *testing.T) {
		s, err := utilities.BuildSchema(`
			enum TestEnum {
				ONE
				TWO
			}

			input TestInput {
				self: TestInput
				string: [String]!
				enum: [TestEnum]
			}

			type Query {
				field(
					argWithPossibleFix: TestInput
					argWithInvalidPossibleFix: TestInput
					argWithoutPossibleFix: TestInput
				): Int
			}
		`)
		if err != nil {
			t.Fatalf("building the schema: %v", err)
		}

		// A schema written in SDL cannot say what a member stands for, nor
		// hold a default in the internal form, so both are put in afterwards.
		// graphql-js works around the same two limits in the same way.
		input, ok := s.Type("TestInput").(*schema.InputObjectType)
		if !ok {
			t.Fatal("TestInput did not come back as an input object")
		}
		input.Field("enum").Type = schema.NewList(testEnumWithInternalValues())
		for _, arg := range s.QueryType().Field("field").Args {
			arg.Default = schema.DefaultValue(defaultsWithPossibleFix[arg.Name()])
		}

		assertSchemaErrors(t, s, wantSuggestedFix)
	})
}

// TestUncoerceDefaultValue_Corners covers the turning-back itself, which the
// two cases above only reach through an input object.
func TestUncoerceDefaultValue_Corners(t *testing.T) {
	// A scalar whose two directions are each other's opposite, so that a
	// default written in either form can be told apart from a wrong one.
	weekday := schema.NewScalar(schema.ScalarConfig{
		Name: "Weekday",
		CoerceOutputValue: func(internal any) (value.Maybe[any], error) {
			day, isDay := internal.(int)
			if !isDay || day < 1 || day > 7 {
				return value.Nothing[any](), gqlerror.New(fmt.Sprintf("Weekday cannot represent value: %v", internal))
			}
			return value.Just[any]([...]string{"MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"}[day-1]), nil
		},
		CoerceInputValue: func(external any) (value.Maybe[any], error) {
			name, isName := external.(string)
			if !isName {
				// A GraphQL error is how a type says it meant the message; a
				// plain one is wrapped in a complaint naming the type.
				return value.Nothing[any](), gqlerror.New("Weekday cannot represent a non-string value")
			}
			for i, day := range []string{"MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"} {
				if day == name {
					return value.Just[any](i + 1), nil
				}
			}
			return value.Nothing[any](), fmt.Errorf("Weekday has no day named %q", name)
		},
	})

	for _, tt := range []struct {
		name     string
		typ      schema.Type
		def      any
		wantErrs []string
	}{
		{
			name: "a scalar written in the form a resolver receives",
			typ:  weekday,
			def:  3,
			wantErrs: []string{
				`Query.field(arg:) has invalid default value: 3. Did you mean: "WED"?`,
			},
		},
		{
			name: "a scalar written in neither form",
			typ:  weekday,
			def:  99,
			wantErrs: []string{
				`Query.field(arg:) has invalid default value: Weekday cannot represent a non-string value`,
			},
		},
		{
			name: "a lone value where a list is wanted",
			typ:  schema.NewList(weekday),
			def:  3,
			wantErrs: []string{
				`Query.field(arg:) has invalid default value: 3. Did you mean: ["WED"]?`,
			},
		},
		{
			name: "under a non-null",
			typ:  schema.NewNonNull(weekday),
			def:  3,
			wantErrs: []string{
				`Query.field(arg:) has invalid default value: 3. Did you mean: "WED"?`,
			},
		},
		{
			name:     "a default that is already right",
			typ:      weekday,
			def:      "WED",
			wantErrs: nil,
		},
		{
			name:     "an explicit null",
			typ:      weekday,
			def:      nil,
			wantErrs: nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assertSchemaErrors(t, schemaWithDefault(tt.typ, schema.DefaultValue(tt.def)), tt.wantErrs)
		})
	}

	t.Run("a value the type cannot be turned back into", func(t *testing.T) {
		// An unknown field leaves nothing to turn back, so the complaint about
		// the field itself is all there is to say.
		input := schema.NewInputObject(schema.InputObjectConfig{
			Name: "TestInput",
			Fields: []*schema.InputField{
				schema.NewInputField("day", schema.InputFieldConfig{Type: weekday}),
			},
		})
		assertSchemaErrors(t, schemaWithDefault(input, schema.DefaultValue(map[string]any{"day": 3, "week": 1})), []string{
			`Query.field(arg:) has invalid default value at .day: Weekday cannot represent a non-string value`,
			`Query.field(arg:) has invalid default value: Expected value of type "TestInput" not to include unknown field "week", found: { day: 3, week: 1 }.`,
		})
	})
}

// schemaWithDefault returns the smallest schema holding one argument with one
// default.
func schemaWithDefault(t schema.Type, def value.Maybe[schema.DefaultInput]) *schema.Schema {
	return schema.New(schema.Config{
		Query: schema.NewObject(schema.ObjectConfig{
			Name: "Query",
			Fields: []*schema.Field{
				schema.NewField("field", schema.FieldConfig{
					Type: schema.Int,
					Args: []*schema.Argument{
						schema.NewArgument("arg", schema.ArgumentConfig{Type: t, Default: def}),
					},
				}),
			},
		}),
	})
}

func assertSchemaErrors(t *testing.T, s *schema.Schema, want []string) {
	t.Helper()
	got := schema.ValidateSchema(s)
	if len(got) != len(want) {
		for _, e := range got {
			t.Logf("got: %s", e.Message)
		}
		t.Fatalf("got %d errors, want %d", len(got), len(want))
	}
	for i, e := range got {
		if e.Message != want[i] {
			t.Errorf("error %d:\n got %s\nwant %s", i, e.Message, want[i])
		}
	}
}
