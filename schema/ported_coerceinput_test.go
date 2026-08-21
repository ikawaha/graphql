package schema_test

// Ported from graphql-js src/utilities/__tests__/coerceInputValue-test.ts.
//
// graphql-js has three answers where Go has two: a value, null, and undefined
// meaning "this does not fit". Here a coercion answers with a value and a
// flag, and a type that will not take a value says so by returning an error,
// so nil is always null. The cases that turn on a coercer returning undefined
// are listed as known divergences below.

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// coerceCase is one of graphql-js's assertions: a value or a literal, the type
// it is being read as, and what should come back.
type coerceCase struct {
	name string
	// in is the value, or the literal as it is written in a document.
	in any
	// as is the type it is read as.
	as schema.Type
	// want is what should come back, and fits says whether it fits at all.
	// A nil want with fits true is GraphQL null.
	want any
	fits bool
	// variables are what the literal's variables hold, for the cases about
	// a literal that names one.
	variables map[string]any
}

// knownCoerceDivergences are the cases this implementation does not match, and
// why. Each is asserted to *still* diverge, so that closing one cannot go
// unnoticed.
var knownCoerceDivergences = map[string]string{
	// A Go coercer has no way to answer "undefined": what it returns is the
	// value, and it says the value does not fit by returning an error.
}

func TestPortedCoerceInputValue(t *testing.T) {
	// The scalar graphql-js writes as one that reads { value } out of what it
	// is given, and fails on { error }.
	testScalar := schema.NewScalar(schema.ScalarConfig{
		Name: "TestScalar",
		CoerceInputValue: func(external any) (value.Maybe[any], error) {
			given, isObject := external.(map[string]any)
			if !isObject {
				return value.Nothing[any](), errors.New("TestScalar wants an object")
			}
			if why, failed := given["error"]; failed {
				return value.Nothing[any](), errors.New(why.(string))
			}
			held, present := given["value"]
			if !present {
				// graphql-js's scalar answers with input.value, which is
				// undefined when the key is not there. Answering with nothing
				// says the same: the value does not fit, and nothing more.
				return value.Nothing[any](), nil
			}
			return value.Just[any](held), nil
		},
	})
	testEnum := schema.NewEnum(schema.EnumConfig{
		Name: "TestEnum",
		Values: []*schema.EnumValue{
			schema.NewEnumValue("FOO", schema.EnumValueConfig{Value: schema.InternalValue("InternalFoo")}),
			schema.NewEnumValue("BAR", schema.EnumValueConfig{Value: schema.InternalValue(123456789)}),
		},
	})

	var testInputObject *schema.InputObjectType
	testInputObject = schema.NewInputObject(schema.InputObjectConfig{
		Name: "TestInputObject",
		FieldsThunk: func() []*schema.InputField {
			return []*schema.InputField{
				schema.NewInputField("foo", schema.InputFieldConfig{Type: schema.NewNonNull(schema.Int)}),
				schema.NewInputField("bar", schema.InputFieldConfig{Type: schema.Int}),
				schema.NewInputField("nestedObject", schema.InputFieldConfig{Type: testInputObject}),
			}
		},
	})
	oneOf := schema.NewInputObject(schema.InputObjectConfig{
		Name:    "TestOneOfInputObject",
		IsOneOf: true,
		Fields: []*schema.InputField{
			schema.NewInputField("foo", schema.InputFieldConfig{Type: schema.Int}),
			schema.NewInputField("bar", schema.InputFieldConfig{Type: schema.Int}),
		},
	})
	oneOfWithDefault := schema.NewInputObject(schema.InputObjectConfig{
		Name:    "TestInvalidOneOfInputObjectWithDefault",
		IsOneOf: true,
		Fields: []*schema.InputField{
			schema.NewInputField("foo", schema.InputFieldConfig{
				Type:    schema.Int,
				Default: value.Just(schema.DefaultInput{Value: 123}),
			}),
		},
	})
	withDefault := func(fallback any) *schema.InputObjectType {
		return schema.NewInputObject(schema.InputObjectConfig{
			Name: "TestInputObject",
			Fields: []*schema.InputField{
				schema.NewInputField("foo", schema.InputFieldConfig{
					Type:    schema.Int,
					Default: value.Just(schema.DefaultInput{Value: fallback}),
				}),
			},
		})
	}
	listOfObjects := schema.NewList(schema.NewInputObject(schema.InputObjectConfig{
		Name: "TestObject",
		Fields: []*schema.InputField{
			schema.NewInputField("length", schema.InputFieldConfig{Type: schema.Int}),
		},
	}))

	nonNullInt := schema.NewNonNull(schema.Int)
	listOfInt := schema.NewList(schema.Int)
	nestedList := schema.NewList(schema.NewList(schema.Int))

	runCoerceValues(t, []coerceCase{
		// For a non-null type.
		{name: "a value where one is required", in: 1, as: nonNullInt, want: int32(1), fits: true},
		{name: "null where one is required", in: nil, as: nonNullInt},

		// For a scalar.
		{name: "a scalar that answers", in: map[string]any{"value": 1}, as: testScalar, want: 1, fits: true},
		{name: "a scalar that answers null", in: map[string]any{"value": nil}, as: testScalar, fits: true},
		{name: "a scalar that answers with nothing", in: map[string]any{}, as: testScalar},
		{name: "a scalar that refuses", in: map[string]any{"error": "Some error message"}, as: testScalar},

		// For an enum.
		{name: "a known member name", in: "FOO", as: testEnum, want: "InternalFoo", fits: true},
		{name: "another known member name", in: "BAR", as: testEnum, want: 123456789, fits: true},
		{name: "a misspelled member", in: "foo", as: testEnum},
		{name: "a number where a member is wanted", in: 123, as: testEnum},
		{name: "an object where a member is wanted", in: map[string]any{"field": "value"}, as: testEnum},

		// For an input object.
		{name: "an object that fits", in: map[string]any{"foo": 123}, as: testInputObject,
			want: map[string]any{"foo": int32(123)}, fits: true},
		{name: "a number where an object is wanted", in: 123, as: testInputObject},
		{name: "a field that does not fit", in: map[string]any{"foo": math.NaN()}, as: testInputObject},
		{name: "two fields that do not fit",
			in: map[string]any{"foo": "abc", "bar": "def"}, as: testInputObject},
		{name: "a required field left out", in: map[string]any{"bar": 123}, as: testInputObject},
		{name: "a field the type does not have",
			in: map[string]any{"foo": 123, "unknownField": 123}, as: testInputObject},
		{name: "a list where an object is wanted",
			in: []any{map[string]any{"foo": 123}, map[string]any{"bar": 456}}, as: testInputObject},

		// For a OneOf input object.
		{name: "one field of a OneOf", in: map[string]any{"foo": 123}, as: oneOf,
			want: map[string]any{"foo": int32(123)}, fits: true},
		{name: "two fields of a OneOf", in: map[string]any{"foo": 123, "bar": nil}, as: oneOf},
		{name: "the one field of a OneOf given as null", in: map[string]any{"bar": nil}, as: oneOf},
		{name: "a OneOf whose only field has a default", in: map[string]any{}, as: oneOfWithDefault},
		{name: "a field of a OneOf that does not fit",
			in: map[string]any{"foo": math.NaN()}, as: oneOf},
		{name: "a OneOf given a field the type does not have",
			in: map[string]any{"foo": 123, "unknownField": 123}, as: oneOf},

		// A default fills a field that was left out.
		{name: "a field given rather than defaulted", in: map[string]any{"foo": 5}, as: withDefault(7),
			want: map[string]any{"foo": int32(5)}, fits: true},
		{name: "a field left out and defaulted", in: map[string]any{}, as: withDefault(7),
			want: map[string]any{"foo": int32(7)}, fits: true},
		{name: "a field left out and defaulted to null", in: map[string]any{}, as: withDefault(nil),
			want: map[string]any{"foo": nil}, fits: true},

		// For a list.
		{name: "a list that fits", in: []any{1, 2, 3}, as: listOfInt,
			want: []any{int32(1), int32(2), int32(3)}, fits: true},
		{name: "a list with an entry that does not fit", in: []any{1, "b", true, 4}, as: listOfInt},
		{name: "a lone value where a list is wanted", in: 42, as: listOfInt,
			want: []any{int32(42)}, fits: true},
		{name: "a lone object where a list of objects is wanted",
			in: map[string]any{"length": 100500}, as: listOfObjects,
			want: []any{map[string]any{"length": int32(100500)}}, fits: true},
		{name: "a lone value that does not fit", in: "INVALID", as: listOfInt},
		{name: "null where a list is wanted", in: nil, as: listOfInt, fits: true},

		// For a list of lists.
		{name: "a nested list that fits", in: []any{[]any{1}, []any{2, 3}}, as: nestedList,
			want: []any{[]any{int32(1)}, []any{int32(2), int32(3)}}, fits: true},
		{name: "a lone value where a nested list is wanted", in: 42, as: nestedList,
			want: []any{[]any{int32(42)}}, fits: true},
		{name: "null where a nested list is wanted", in: nil, as: nestedList, fits: true},
		{name: "a flat list where a nested one is wanted", in: []any{1, 2, 3}, as: nestedList,
			want: []any{[]any{int32(1)}, []any{int32(2)}, []any{int32(3)}}, fits: true},
		{name: "a nested list with a null in it", in: []any{42, []any{nil}, nil}, as: nestedList,
			want: []any{[]any{int32(42)}, []any{nil}, nil}, fits: true},
	})
}

func runCoerceValues(t *testing.T, cases []coerceCase) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, fits := schema.CoerceInputValue(tt.in, tt.as)
			checkCoerced(t, tt.name, got, fits, tt.want, tt.fits)
		})
	}
}

func checkCoerced(t *testing.T, name string, got any, fits bool, want any, wantFits bool) {
	t.Helper()
	if why, listed := knownCoerceDivergences[name]; listed {
		if fits == wantFits && sameCoerced(got, want) {
			t.Errorf("this case now matches graphql-js; remove it from the known divergences (%s)", why)
		} else {
			t.Logf("known divergence: %s", why)
		}
		return
	}
	if fits != wantFits {
		t.Fatalf("fits = %v, want %v (value %#v)", fits, wantFits, got)
	}
	if fits && !sameCoerced(got, want) {
		t.Errorf("coerced to %#v, want %#v", got, want)
	}
}

// sameCoerced compares two coerced values, counting two NaNs as the same:
// nothing else in Go does, and a scalar answering NaN is one of the cases.
func sameCoerced(got, want any) bool {
	if left, isFloat := got.(float64); isFloat && math.IsNaN(left) {
		right, isFloat := want.(float64)
		return isFloat && math.IsNaN(right)
	}
	return reflect.DeepEqual(got, want)
}

func TestPortedCoerceInputLiteral(t *testing.T) {
	passthrough := schema.NewScalar(schema.ScalarConfig{
		Name: "PassthroughScalar",
		CoerceInputLiteral: func(literal language.Value) (value.Maybe[any], error) {
			written, isString := literal.(*language.StringValue)
			if !isString {
				return value.Nothing[any](), errors.New("PassthroughScalar wants a string")
			}
			return value.Just[any](written.Value), nil
		},
	})
	printing := schema.NewScalar(schema.ScalarConfig{
		Name: "PrintScalar",
		CoerceInputLiteral: func(literal language.Value) (value.Maybe[any], error) {
			return value.Just[any]("~~~" + language.Print(literal) + "~~~"), nil
		},
	})
	refusing := schema.NewScalar(schema.ScalarConfig{
		Name: "ThrowScalar",
		CoerceInputLiteral: func(language.Value) (value.Maybe[any], error) {
			return value.Nothing[any](), errors.New("Test")
		},
	})
	silent := schema.NewScalar(schema.ScalarConfig{
		Name: "ReturnUndefinedScalar",
		CoerceInputLiteral: func(language.Value) (value.Maybe[any], error) {
			// graphql-js's returns undefined; this says the same.
			return value.Nothing[any](), nil
		},
	})

	// NULL is graphql-js's member whose value is null.
	testEnum := schema.NewEnum(schema.EnumConfig{
		Name: "TestColor",
		Values: []*schema.EnumValue{
			schema.NewEnumValue("RED", schema.EnumValueConfig{Value: schema.InternalValue(1)}),
			schema.NewEnumValue("GREEN", schema.EnumValueConfig{Value: schema.InternalValue(2)}),
			schema.NewEnumValue("BLUE", schema.EnumValueConfig{Value: schema.InternalValue(3)}),
			// graphql-js writes { value: null } here, which is a member whose
			// internal value really is null rather than one that says nothing.
			schema.NewEnumValue("NULL", schema.EnumValueConfig{Value: schema.InternalValue(nil)}),
			schema.NewEnumValue("NAN", schema.EnumValueConfig{Value: schema.InternalValue(math.NaN())}),
			schema.NewEnumValue("NO_CUSTOM_VALUE", schema.EnumValueConfig{}),
		},
	})

	nonNullBool := schema.NewNonNull(schema.Boolean)
	listOfBool := schema.NewList(schema.Boolean)
	listOfNonNullBool := schema.NewList(nonNullBool)
	nonNullListOfBool := schema.NewNonNull(listOfBool)
	nonNullListOfNonNullBool := schema.NewNonNull(listOfNonNullBool)

	// An input object whose fields are filled in from defaults where the
	// literal leaves them out.
	defaulted := schema.NewInputObject(schema.InputObjectConfig{
		Name: "TestInput",
		Fields: []*schema.InputField{
			schema.NewInputField("int", schema.InputFieldConfig{
				Type:    schema.Int,
				Default: value.Just(schema.DefaultInput{Value: 42}),
			}),
			schema.NewInputField("float", schema.InputFieldConfig{
				Type:    schema.Float,
				Default: value.Just(schema.DefaultInput{Literal: literalOf(t, "3.14")}),
			}),
		},
	})
	testInput := schema.NewInputObject(schema.InputObjectConfig{
		Name: "TestInput",
		Fields: []*schema.InputField{
			schema.NewInputField("int", schema.InputFieldConfig{
				Type:    schema.Int,
				Default: value.Just(schema.DefaultInput{Value: 42}),
			}),
			schema.NewInputField("bool", schema.InputFieldConfig{Type: schema.Boolean}),
			schema.NewInputField("requiredBool", schema.InputFieldConfig{Type: nonNullBool}),
		},
	})
	testOneOf := schema.NewInputObject(schema.InputObjectConfig{
		Name:    "TestOneOfInput",
		IsOneOf: true,
		Fields: []*schema.InputField{
			schema.NewInputField("a", schema.InputFieldConfig{Type: schema.String}),
			schema.NewInputField("b", schema.InputFieldConfig{Type: schema.String}),
		},
	})

	// A variable nothing was supplied for is absent from the map; one supplied
	// as null is present holding nil. graphql-js spells the first `undefined`.
	held := func(name string, v any) map[string]any { return map[string]any{name: v} }

	runCoerceLiterals(t, []coerceCase{
		// The built-in scalars.
		{name: "true as Boolean", in: "true", as: schema.Boolean, want: true, fits: true},
		{name: "false as Boolean", in: "false", as: schema.Boolean, want: false, fits: true},
		{name: "123 as Int", in: "123", as: schema.Int, want: int32(123), fits: true},
		{name: "123 as Float", in: "123", as: schema.Float, want: 123.0, fits: true},
		{name: "123.456 as Float", in: "123.456", as: schema.Float, want: 123.456, fits: true},
		{name: `"abc123" as String`, in: `"abc123"`, as: schema.String, want: "abc123", fits: true},
		{name: "123456 as ID", in: "123456", as: schema.ID, want: "123456", fits: true},
		{name: `"123456" as ID`, in: `"123456"`, as: schema.ID, want: "123456", fits: true},
		{name: "123 as Boolean", in: "123", as: schema.Boolean},
		{name: "123.456 as Int", in: "123.456", as: schema.Int},
		{name: "true as Int", in: "true", as: schema.Int},
		{name: `"123" as Int`, in: `"123"`, as: schema.Int},
		{name: `"123" as Float`, in: `"123"`, as: schema.Float},
		{name: "123 as String", in: "123", as: schema.String},
		{name: "true as String", in: "true", as: schema.String},
		{name: "123.456 as String", in: "123.456", as: schema.String},
		{name: "123.456 as ID", in: "123.456", as: schema.ID},

		// A scalar that reads literals itself.
		{name: "a literal a scalar reads", in: `"value"`, as: passthrough, want: "value", fits: true},
		{name: "a literal a scalar prints", in: `"value"`, as: printing,
			want: `~~~"value"~~~`, fits: true},
		{name: "a literal a scalar refuses", in: "value", as: refusing},
		{name: "a literal a scalar answers nothing for", in: "value", as: silent},

		// An enum.
		{name: "a member name", in: "RED", as: testEnum, want: 1, fits: true},
		{name: "another member name", in: "BLUE", as: testEnum, want: 3, fits: true},
		{name: "a number where a member is wanted", in: "3", as: testEnum},
		{name: "a string where a member is wanted", in: `"BLUE"`, as: testEnum},
		{name: "null where a member is wanted", in: "null", as: testEnum, fits: true},
		{name: "an enum member whose value is null", in: "NULL", as: testEnum, fits: true},
		{name: "an enum member whose value is null, as a non-null", in: "NULL",
			as: schema.NewNonNull(testEnum), fits: true},
		{name: "an enum member whose value is not a number", in: "NAN", as: testEnum,
			want: math.NaN(), fits: true},
		{name: "an enum member with no value of its own", in: "NO_CUSTOM_VALUE", as: testEnum,
			want: "NO_CUSTOM_VALUE", fits: true},

		// Null and non-null.
		{name: "null as Boolean", in: "null", as: schema.Boolean, fits: true},
		{name: "null as Boolean!", in: "null", as: nonNullBool},

		// Lists.
		{name: "a lone value as [Boolean]", in: "true", as: listOfBool,
			want: []any{true}, fits: true},
		{name: "a lone value that does not fit as [Boolean]", in: "123", as: listOfBool},
		{name: "null as [Boolean]", in: "null", as: listOfBool, fits: true},
		{name: "a list as [Boolean]", in: "[true, false]", as: listOfBool,
			want: []any{true, false}, fits: true},
		{name: "a list with an entry that does not fit as [Boolean]", in: "[true, 123]", as: listOfBool},
		{name: "a list holding null as [Boolean]", in: "[true, null]", as: listOfBool,
			want: []any{true, nil}, fits: true},
		{name: "an object as [Boolean]", in: "{ true: true }", as: listOfBool},

		{name: "a lone value as [Boolean]!", in: "true", as: nonNullListOfBool,
			want: []any{true}, fits: true},
		{name: "a lone value that does not fit as [Boolean]!", in: "123", as: nonNullListOfBool},
		{name: "null as [Boolean]!", in: "null", as: nonNullListOfBool},
		{name: "a list as [Boolean]!", in: "[true, false]", as: nonNullListOfBool,
			want: []any{true, false}, fits: true},
		{name: "a list with an entry that does not fit as [Boolean]!", in: "[true, 123]",
			as: nonNullListOfBool},
		{name: "a list holding null as [Boolean]!", in: "[true, null]", as: nonNullListOfBool,
			want: []any{true, nil}, fits: true},

		{name: "a lone value as [Boolean!]", in: "true", as: listOfNonNullBool,
			want: []any{true}, fits: true},
		{name: "a lone value that does not fit as [Boolean!]", in: "123", as: listOfNonNullBool},
		{name: "null as [Boolean!]", in: "null", as: listOfNonNullBool, fits: true},
		{name: "a list as [Boolean!]", in: "[true, false]", as: listOfNonNullBool,
			want: []any{true, false}, fits: true},
		{name: "a list with an entry that does not fit as [Boolean!]", in: "[true, 123]",
			as: listOfNonNullBool},
		{name: "a list holding null as [Boolean!]", in: "[true, null]", as: listOfNonNullBool},

		{name: "a lone value as [Boolean!]!", in: "true", as: nonNullListOfNonNullBool,
			want: []any{true}, fits: true},
		{name: "a lone value that does not fit as [Boolean!]!", in: "123",
			as: nonNullListOfNonNullBool},
		{name: "null as [Boolean!]!", in: "null", as: nonNullListOfNonNullBool},
		{name: "a list as [Boolean!]!", in: "[true, false]", as: nonNullListOfNonNullBool,
			want: []any{true, false}, fits: true},
		{name: "a list with an entry that does not fit as [Boolean!]!", in: "[true, 123]",
			as: nonNullListOfNonNullBool},
		{name: "a list holding null as [Boolean!]!", in: "[true, null]",
			as: nonNullListOfNonNullBool},

		// Input objects, and the defaults that fill what a literal leaves out.
		{name: "an empty object, filled in from defaults", in: "{}", as: defaulted,
			want: map[string]any{"int": int32(42), "float": 3.14}, fits: true},
		{name: "null as an input object", in: "null", as: testInput, fits: true},
		{name: "a number as an input object", in: "123", as: testInput},
		{name: "a list as an input object", in: "[]", as: testInput},
		{name: "an object with the required field", in: "{ requiredBool: true }", as: testInput,
			want: map[string]any{"int": int32(42), "requiredBool": true}, fits: true},
		{name: "an object with a field given as null", in: "{ int: null, requiredBool: true }",
			as: testInput, want: map[string]any{"int": nil, "requiredBool": true}, fits: true},
		{name: "an object with every field given", in: "{ int: 123, requiredBool: false }",
			as: testInput, want: map[string]any{"int": int32(123), "requiredBool": false}, fits: true},
		{name: "an object with an optional field given", in: "{ bool: true, requiredBool: false }",
			as:   testInput,
			want: map[string]any{"int": int32(42), "bool": true, "requiredBool": false}, fits: true},
		{name: "an object with a field that does not fit", in: "{ int: true, requiredBool: true }",
			as: testInput},
		{name: "an object with the required field given as null", in: "{ requiredBool: null }",
			as: testInput},
		{name: "an object with the required field left out", in: "{ bool: true }", as: testInput},
		{name: "an object with a field the type does not have",
			in: "{ requiredBool: true, unknown: 123 }", as: testInput},

		// OneOf input objects.
		{name: "one field of a OneOf literal", in: `{ a: "abc" }`, as: testOneOf,
			want: map[string]any{"a": "abc"}, fits: true},
		{name: "the other field of a OneOf literal", in: `{ b: "def" }`, as: testOneOf,
			want: map[string]any{"b": "def"}, fits: true},
		{name: "a OneOf literal with a null alongside", in: `{ a: "abc", b: null }`, as: testOneOf},
		{name: "a OneOf literal given null", in: `{ a: null }`, as: testOneOf},
		{name: "a OneOf literal whose field does not fit", in: `{ a: 1 }`, as: testOneOf},
		{name: "a OneOf literal with two fields", in: `{ a: "abc", b: "def" }`, as: testOneOf},
		{name: "a OneOf literal with an unknown field", in: `{ a: "abc", c: "def" }`, as: testOneOf},
		{name: "an empty OneOf literal", in: "{}", as: testOneOf},
		{name: "a OneOf literal with only an unknown field", in: `{ c: "abc" }`, as: testOneOf},

		// A literal naming a variable stands for whatever the variable holds.
		{name: "a variable nothing was supplied for", in: "$var", as: schema.Boolean},
		{name: "a variable nothing was supplied for, in a list", in: "[ $foo ]", as: listOfBool,
			want: []any{nil}, fits: true},
		{name: "a variable nothing was supplied for, in a list of non-nulls", in: "[ $foo ]",
			as: listOfNonNullBool},
		{name: "a variable, in a list of non-nulls", in: "[ $foo ]", as: listOfNonNullBool,
			variables: held("foo", true), want: []any{true}, fits: true},

		// A variable has already been coerced, so a lone one is not wrapped
		// into a list of one the way a lone literal would be.
		{name: "a lone variable where a list is wanted", in: "$foo", as: listOfNonNullBool,
			variables: held("foo", true), want: true, fits: true},
		{name: "a lone variable already holding a list", in: "$foo", as: listOfNonNullBool,
			variables: held("foo", []any{true}), want: []any{true}, fits: true},

		{name: "variables nothing was supplied for, in an input object",
			in: "{ int: $foo, bool: $foo, requiredBool: true }", as: testInput,
			want: map[string]any{"int": int32(42), "requiredBool": true}, fits: true},
		{name: "a variable nothing was supplied for, for a required field",
			in: "{ requiredBool: $foo }", as: testInput},
		{name: "a variable for a required field", in: "{ requiredBool: $foo }", as: testInput,
			variables: held("foo", true),
			want:      map[string]any{"int": int32(42), "requiredBool": true}, fits: true},
		{name: "a variable holding null, in an input object",
			in: "{ int: $foo, requiredBool: true }", as: testInput, variables: held("foo", nil),
			want: map[string]any{"int": nil, "requiredBool": true}, fits: true},

		{name: "a OneOf whose second field names a variable with no value",
			in: "{ a: $a, b: $b }", as: testOneOf, variables: held("a", "abc")},
		{name: "a OneOf whose field names a variable holding null", in: "{ a: $a }",
			as: testOneOf, variables: held("a", nil)},
	})
}

func runCoerceLiterals(t *testing.T, cases []coerceCase) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, fits := schema.CoerceInputLiteral(literalOf(t, tt.in.(string)), tt.as, schema.NewVariableValues(tt.variables, nil))
			checkCoerced(t, tt.name, got, fits, tt.want, tt.fits)
		})
	}
}
