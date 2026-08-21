package execution_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// knownVariablesDivergences are the cases this implementation does not match,
// and why. Each is asserted to *still* diverge, so that closing one cannot go
// unnoticed.
// graphql-js's own test defines one GraphQLError at module scope and throws it
// from every faulty coercion. Wrapping it prefixes the message in place, so the
// argument case that runs first leaves its prefix on the shared error and the
// variable case that follows reads it back. Run on its own, graphql-js answers
// what this answers, to the location and the extensions; the expectation in the
// upstream test is of the polluted message.
var knownVariablesDivergences = map[string]string{
	// graphql-js builds this scalar's error once, at the top of the file, and
	// the argument case before this one rewrites that one object's message. By
	// the time this case runs, the message it expects carries the earlier
	// case's prefix. Here each complaint is its own error, so the message is
	// only about this one.
	"errors on faulty scalar type input (2)": "the upstream test expects a message an earlier case left behind",
}

// testVariablesSchema is graphql-js's own schema from variables-test.ts.
func testVariablesSchema(t *testing.T) *schema.Schema {
	t.Helper()

	faulty := gqlerror.New("FaultyScalarErrorMessage",
		gqlerror.WithExtensions(map[string]any{"code": "FaultyScalarErrorExtensionCode"}))
	faultyScalar := schema.NewScalar(schema.ScalarConfig{
		Name:               "FaultyScalar",
		CoerceInputValue:   func(any) (value.Maybe[any], error) { return value.Nothing[any](), faulty },
		CoerceInputLiteral: func(language.Value) (value.Maybe[any], error) { return value.Nothing[any](), faulty },
	})

	complexScalar := schema.NewScalar(schema.ScalarConfig{
		Name: "ComplexScalar",
		CoerceInputValue: func(external any) (value.Maybe[any], error) {
			if external != "ExternalValue" {
				return value.Nothing[any](), fmt.Errorf("expected ExternalValue, got %v", external)
			}
			return value.Just[any]("InternalValue"), nil
		},
		CoerceInputLiteral: func(literal language.Value) (value.Maybe[any], error) {
			written, isString := literal.(*language.StringValue)
			if !isString || written.Value != "ExternalValue" {
				return value.Nothing[any](), fmt.Errorf("expected ExternalValue, got %s", language.Print(literal))
			}
			return value.Just[any]("InternalValue"), nil
		},
	})

	jsonScalar := schema.NewScalar(schema.ScalarConfig{
		Name:             "JSONScalar",
		CoerceInputValue: func(external any) (value.Maybe[any], error) { return value.Just[any](external), nil },
		CoerceInputLiteral: func(literal language.Value) (value.Maybe[any], error) {
			// The literal is already a constant: ReplaceVariables put what the
			// request supplied in place before this was called.
			held, ok := schema.ValueFromASTUntyped(literal, schema.VariableValues{})
			if !ok {
				return value.Nothing[any](), errors.New("cannot read the literal")
			}
			return value.Just[any](held), nil
		},
	})

	testInputObject := schema.NewInputObject(schema.InputObjectConfig{
		Name: "TestInputObject",
		Fields: []*schema.InputField{
			schema.NewInputField("a", schema.InputFieldConfig{Type: schema.String}),
			schema.NewInputField("b", schema.InputFieldConfig{Type: schema.NewList(schema.String)}),
			schema.NewInputField("c", schema.InputFieldConfig{Type: schema.NewNonNull(schema.String)}),
			schema.NewInputField("d", schema.InputFieldConfig{Type: complexScalar}),
			schema.NewInputField("e", schema.InputFieldConfig{Type: faultyScalar}),
		},
	})
	testOneOfInputObject := schema.NewInputObject(schema.InputObjectConfig{
		Name:    "TestOneOfInputObject",
		IsOneOf: true,
		Fields: []*schema.InputField{
			schema.NewInputField("a", schema.InputFieldConfig{Type: schema.String}),
			schema.NewInputField("b", schema.InputFieldConfig{Type: schema.String}),
		},
	})
	testNestedInputObject := schema.NewInputObject(schema.InputObjectConfig{
		Name: "TestNestedInputObject",
		Fields: []*schema.InputField{
			schema.NewInputField("na", schema.InputFieldConfig{Type: schema.NewNonNull(testInputObject)}),
			schema.NewInputField("nb", schema.InputFieldConfig{Type: schema.NewNonNull(schema.String)}),
		},
	})

	// NULL and UNDEFINED are graphql-js's members whose values are null and
	// undefined. Go has neither to give a member, so both take their names,
	// which is what the two divergences above are about.
	testEnum := schema.NewEnum(schema.EnumConfig{
		Name: "TestEnum",
		Values: []*schema.EnumValue{
			// graphql-js writes { value: null } here, which is a member whose
			// internal value really is null rather than one that says nothing.
			schema.NewEnumValue("NULL", schema.EnumValueConfig{Value: schema.InternalValue(nil)}),
			schema.NewEnumValue("UNDEFINED", schema.EnumValueConfig{}),
			schema.NewEnumValue("NAN", schema.EnumValueConfig{Value: schema.InternalValue(math.NaN())}),
			schema.NewEnumValue("FALSE", schema.EnumValueConfig{Value: schema.InternalValue(false)}),
			schema.NewEnumValue("CUSTOM", schema.EnumValueConfig{Value: schema.InternalValue("custom value")}),
			schema.NewEnumValue("DEFAULT_VALUE", schema.EnumValueConfig{}),
		},
	})

	nestedType := schema.NewObject(schema.ObjectConfig{
		Name:   "NestedType",
		Fields: []*schema.Field{fieldWithInputArg("echo", schema.String, value.Nothing[schema.DefaultInput]())},
	})

	hello := value.Just(schema.DefaultInput{Value: "Hello World"})
	query := schema.NewObject(schema.ObjectConfig{
		Name: "TestType",
		Fields: []*schema.Field{
			fieldWithInputArg("fieldWithEnumInput", testEnum, value.Nothing[schema.DefaultInput]()),
			fieldWithInputArg("fieldWithNonNullableEnumInput", schema.NewNonNull(testEnum), value.Nothing[schema.DefaultInput]()),
			fieldWithInputArg("fieldWithObjectInput", testInputObject, value.Nothing[schema.DefaultInput]()),
			fieldWithInputArg("fieldWithOneOfObjectInput", testOneOfInputObject, value.Nothing[schema.DefaultInput]()),
			fieldWithInputArg("fieldWithNullableStringInput", schema.String, value.Nothing[schema.DefaultInput]()),
			fieldWithInputArg("fieldWithNonNullableStringInput", schema.NewNonNull(schema.String), value.Nothing[schema.DefaultInput]()),
			fieldWithInputArg("fieldWithDefaultArgumentValue", schema.String, hello),
			fieldWithInputArg("fieldWithNonNullableStringInputAndDefaultArgumentValue",
				schema.NewNonNull(schema.String), hello),
			fieldWithInputArg("fieldWithNestedInputObject", testNestedInputObject, value.Nothing[schema.DefaultInput]()),
			fieldWithInputArg("fieldWithJSONScalarInput", jsonScalar, value.Nothing[schema.DefaultInput]()),
			schema.NewField("fieldWithPrototypeNamedArgument", schema.FieldConfig{
				Type: schema.String,
				Args: []*schema.Argument{
					schema.NewArgument("toString", schema.ArgumentConfig{Type: schema.String}),
				},
				Resolve: func(_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo) (any, error) {
					held, supplied := args.Get("toString")
					if !supplied {
						return "missing", nil
					}
					return value.Describe(held), nil
				},
			}),
			fieldWithInputArg("list", schema.NewList(schema.String), value.Nothing[schema.DefaultInput]()),
			schema.NewField("nested", schema.FieldConfig{
				Type: nestedType,
				Resolve: func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
					return map[string]any{}, nil
				},
			}),
			fieldWithInputArg("nnList", schema.NewNonNull(schema.NewList(schema.String)), value.Nothing[schema.DefaultInput]()),
			fieldWithInputArg("listNN", schema.NewList(schema.NewNonNull(schema.String)), value.Nothing[schema.DefaultInput]()),
			fieldWithInputArg("nnListNN",
				schema.NewNonNull(schema.NewList(schema.NewNonNull(schema.String))), value.Nothing[schema.DefaultInput]()),
		},
	})

	// graphql-js gives this schema a @skip of its own, whose `if` defaults to
	// true, and keeps the standard @include.
	skip := schema.NewDirective(schema.DirectiveConfig{
		Name: "skip",
		Locations: []language.DirectiveLocation{
			language.DirectiveLocationField,
			language.DirectiveLocationFragmentSpread,
			language.DirectiveLocationInlineFragment,
		},
		Args: []*schema.Argument{
			schema.NewArgument("if", schema.ArgumentConfig{
				Type:    schema.NewNonNull(schema.Boolean),
				Default: value.Just(schema.DefaultInput{Value: true}),
			}),
		},
	})

	s := schema.New(schema.Config{
		Query:      query,
		Directives: []*schema.Directive{skip, schema.Include},
	})
	if err := schema.AssertValidSchema(s); err != nil {
		t.Fatalf("the ported schema is not sound: %v", err)
	}
	return s
}

// fieldWithInputArg is graphql-js's helper of the same name: a String field
// taking one argument, answering with the argument as a resolver saw it.
//
// An argument the caller left out is answered with nothing at all, which is
// how the tests tell "not supplied" from "supplied as null".
func fieldWithInputArg(name string, argType schema.Type, def value.Maybe[schema.DefaultInput]) *schema.Field {
	return schema.NewField(name, schema.FieldConfig{
		Type: schema.String,
		Args: []*schema.Argument{
			schema.NewArgument("input", schema.ArgumentConfig{Type: argType, Default: def}),
		},
		Resolve: func(_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo) (any, error) {
			held, supplied := args.Get("input")
			if !supplied {
				return nil, nil
			}
			return value.Describe(held), nil
		},
	})
}
