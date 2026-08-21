package schema_test

// Ported from graphql-js src/utilities/__tests__/validateInputValue-test.ts.
//
// The two halves of the checking are the same shape: a value or a literal, the
// type it is being read as, and what should be said about it. What is compared
// is the message and the path, since saying where inside a value the trouble
// is is most of what this code is for.

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// complaint is one thing said about a value: where inside it the trouble is,
// and what is wrong there.
type complaint struct {
	path []any
	says string
}

// knownValidateDivergences are the cases this implementation does not match,
// and why. Each is asserted to *still* diverge, so that closing one cannot go
// unnoticed.
var knownValidateDivergences = map[string]string{
	// The value graphql-js is given here is { value: undefined } — an object
	// with a key whose value is undefined. A Go map either has a key or it
	// does not, so the nearest thing is the empty object, and the complaint
	// names that instead. What the scalar makes of it agrees; only the value
	// written into the message differs.
	"for GraphQLScalar: { value: undefined }": "an object cannot hold undefined as a value in Go",

	// A Go map has no order, so a message showing one sorts its keys.
}

func TestPortedValidateInputValue(t *testing.T) {
	// The scalar graphql-js writes as one that reads { value } out of what it
	// is given, and fails on { error }.
	valueTestScalar := schema.NewScalar(schema.ScalarConfig{
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
	valueThrowScalar := schema.NewScalar(schema.ScalarConfig{
		Name: "TestScalar",
		CoerceInputValue: func(any) (value.Maybe[any], error) {
			return value.Nothing[any](), errors.New("Not an error object.")
		},
	})
	var valueTestInputObject *schema.InputObjectType
	valueTestInputObject = schema.NewInputObject(schema.InputObjectConfig{
		Name: "TestInputObject",
		FieldsThunk: func() []*schema.InputField {
			return []*schema.InputField{
				schema.NewInputField("foo", schema.InputFieldConfig{Type: schema.NewNonNull(schema.Int)}),
				schema.NewInputField("bar", schema.InputFieldConfig{Type: schema.Int}),
				schema.NewInputField("nested", schema.InputFieldConfig{Type: valueTestInputObject}),
			}
		},
	})

	runValidateValues(t, []validateCase{
		{
			name: `for GraphQLNonNull: 1`,
			in:   value.Just[any](1),
			as:   schema.NewNonNull(schema.Int),
			want: nil,
		},
		{
			name: `for GraphQLNonNull: undefined`,
			in:   value.Nothing[any](),
			as:   schema.NewNonNull(schema.Int),
			want: []complaint{{says: "Expected a value of non-null type \"Int!\" to be provided."}},
		},
		{
			name: `for GraphQLNonNull: null`,
			in:   value.Just[any](nil),
			as:   schema.NewNonNull(schema.Int),
			want: []complaint{{says: "Expected value of non-null type \"Int!\" not to be null."}},
		},
		{
			name: `for GraphQLScalar: { value: 1 }`,
			in:   value.Just[any](map[string]any{"value": 1}),
			as:   valueTestScalar,
			want: nil,
		},
		{
			name: `for GraphQLScalar: { value: null }`,
			in:   value.Just[any](map[string]any{"value": nil}),
			as:   valueTestScalar,
			want: nil,
		},
		{
			name: `for GraphQLScalar: { value: NaN }`,
			in:   value.Just[any](map[string]any{"value": math.NaN()}),
			as:   valueTestScalar,
			want: nil,
		},
		{
			name: `for GraphQLScalar: { value: undefined }`,
			in:   value.Just[any](map[string]any{}),
			as:   valueTestScalar,
			want: []complaint{{says: "Expected value of type \"TestScalar\", found: { value: undefined }."}},
		},
		{
			name: `for GraphQLEnum: 'FOO'`,
			in:   value.Just[any]("FOO"),
			as:   testEnum,
			want: nil,
		},
		{
			name: `for GraphQLEnum: 'BAR'`,
			in:   value.Just[any]("BAR"),
			as:   testEnum,
			want: nil,
		},
		{
			name: `for GraphQLEnum: 'UNKNOWN'`,
			in:   value.Just[any]("UNKNOWN"),
			as:   testEnum,
			want: []complaint{{says: "Value \"UNKNOWN\" does not exist in \"TestEnum\" enum."}},
		},
		{
			name: `for GraphQLEnum: 'foo'`,
			in:   value.Just[any]("foo"),
			as:   testEnum,
			want: []complaint{{says: "Value \"foo\" does not exist in \"TestEnum\" enum. Did you mean the enum value \"FOO\"?"}},
		},
		{
			name: `for GraphQLEnum: 123`,
			in:   value.Just[any](123),
			as:   testEnum,
			want: []complaint{{says: "Enum \"TestEnum\" cannot represent non-string value: 123."}},
		},
		{
			name: `for GraphQLEnum: { field: 'value' }`,
			in:   value.Just[any](map[string]any{"field": "value"}),
			as:   testEnum,
			want: []complaint{{says: "Enum \"TestEnum\" cannot represent non-string value: { field: \"value\" }."}},
		},
		{
			name: `for GraphQLEnum: {}`,
			in:   value.Just[any](map[string]any{}),
			as:   valueThrowScalar,
			want: []complaint{{says: "Expected value of type \"TestScalar\", but encountered error \"Not an error object.\"; found: {}."}},
		},
		{
			name: `for GraphQLInputObject: { foo: 123 }`,
			in:   value.Just[any](map[string]any{"foo": 123}),
			as:   valueTestInputObject,
			want: nil,
		},
		{
			name: `for GraphQLInputObject: 123`,
			in:   value.Just[any](123),
			as:   valueTestInputObject,
			want: []complaint{{says: "Expected value of type \"TestInputObject\" to be an object, found: 123."}},
		},
		{
			name: `for GraphQLInputObject: { foo: NaN }`,
			in:   value.Just[any](map[string]any{"foo": math.NaN()}),
			as:   valueTestInputObject,
			want: []complaint{{path: []any{"foo"}, says: "Int cannot represent non-integer value: NaN"}},
		},
		{
			name: `for GraphQLInputObject: { foo: 'abc', bar: 'def' }`,
			in:   value.Just[any](map[string]any{"foo": "abc", "bar": "def"}),
			as:   valueTestInputObject,
			want: []complaint{{path: []any{"foo"}, says: "Int cannot represent non-integer value: \"abc\""}, {path: []any{"bar"}, says: "Int cannot represent non-integer value: \"def\""}},
		},
		{
			name: `for GraphQLInputObject: { bar: 123 }`,
			in:   value.Just[any](map[string]any{"bar": 123}),
			as:   valueTestInputObject,
			want: []complaint{{says: "Expected value of type \"TestInputObject\" to include required field \"foo\", found: { bar: 123 }."}},
		},
		{
			name: `for GraphQLInputObject: [{ foo: 123 }, { bar: 456 }]`,
			in:   value.Just[any]([]any{map[string]any{"foo": 123}, map[string]any{"bar": 456}}),
			as:   valueTestInputObject,
			want: []complaint{{says: "Expected value of type \"TestInputObject\" to be an object, found: [{ foo: 123 }, { bar: 456 }]."}},
		},
		{
			name: `for GraphQLInputObject: { foo: 123, nested: [{ foo: 123 }, { bar: 456 }] }`,
			in:   value.Just[any](map[string]any{"foo": 123, "nested": []any{map[string]any{"foo": 123}, map[string]any{"bar": 456}}}),
			as:   valueTestInputObject,
			want: []complaint{{path: []any{"nested"}, says: "Expected value of type \"TestInputObject\" to be an object, found: [{ foo: 123 }, { bar: 456 }]."}},
		},
		{
			name: `for GraphQLInputObject: { foo: 123, bart: 123 }`,
			// A JavaScript object literal has an order, so the faithful stand-in
			// for one is an ordered map rather than a Go map.
			in:   value.Just[any](orderedOf("foo", 123, "bart", 123)),
			as:   valueTestInputObject,
			want: []complaint{{says: "Expected value of type \"TestInputObject\" not to include unknown field \"bart\". Did you mean \"bar\"? Found: { foo: 123, bart: 123 }."}},
		},
		{
			name: `for GraphQLInputObject with default value: { foo: 5 }`,
			in:   value.Just[any](map[string]any{"foo": 5}),
			as:   objectDefaulting(7),
			want: nil,
		},
		{
			name: `for GraphQLInputObject with default value: {}`,
			in:   value.Just[any](map[string]any{}),
			as:   objectDefaulting(7),
			want: nil,
		},
		{
			name: `for GraphQLInputObject with default value: {} (2)`,
			in:   value.Just[any](map[string]any{}),
			as:   objectDefaulting(nil),
			want: nil,
		},
		{
			name: `for GraphQLInputObject with default value: {} (3)`,
			in:   value.Just[any](map[string]any{}),
			as:   objectDefaulting(math.NaN()),
			want: nil,
		},
		{
			name: `for GraphQLInputObject that isOneOf: { foo: 123 }`,
			in:   value.Just[any](map[string]any{"foo": 123}),
			as:   testOneOfInput,
			want: nil,
		},
		{
			name: `for GraphQLInputObject that isOneOf: { foo: 123, bar: null }`,
			in:   value.Just[any](map[string]any{"foo": 123, "bar": nil}),
			as:   testOneOfInput,
			want: []complaint{{says: "Within OneOf Input Object type \"TestInputObject\", exactly one field must be specified, and the value for that field must be non-null."}},
		},
		{
			name: `for GraphQLInputObject that isOneOf: { foo: 123, bar: undefined }`,
			in:   value.Just[any](map[string]any{"foo": 123}),
			as:   testOneOfInput,
			want: nil,
		},
		{
			name: `for GraphQLInputObject that isOneOf: { bar: null }`,
			in:   value.Just[any](map[string]any{"bar": nil}),
			as:   testOneOfInput,
			want: []complaint{{path: []any{"bar"}, says: "Within OneOf Input Object type \"TestInputObject\", exactly one field must be specified, and the value for that field must be non-null."}},
		},
		{
			name: `for GraphQLInputObject that isOneOf: { foo: NaN }`,
			in:   value.Just[any](map[string]any{"foo": math.NaN()}),
			as:   testOneOfInput,
			want: []complaint{{path: []any{"foo"}, says: "Int cannot represent non-integer value: NaN"}},
		},
		{
			name: `for GraphQLInputObject that isOneOf: { foo: 'abc', bar: 'def' }`,
			in:   value.Just[any](map[string]any{"foo": "abc", "bar": "def"}),
			as:   testOneOfInput,
			want: []complaint{{path: []any{"foo"}, says: "Int cannot represent non-integer value: \"abc\""}, {path: []any{"bar"}, says: "Int cannot represent non-integer value: \"def\""}, {says: "Within OneOf Input Object type \"TestInputObject\", exactly one field must be specified, and the value for that field must be non-null."}},
		},
		{
			name: `for GraphQLInputObject that isOneOf: { bart: 123 }`,
			in:   value.Just[any](map[string]any{"bart": 123}),
			as:   testOneOfInput,
			want: []complaint{{says: "Expected value of type \"TestInputObject\" not to include unknown field \"bart\". Did you mean \"bar\"? Found: { bart: 123 }."}, {says: "Within OneOf Input Object type \"TestInputObject\", exactly one field must be specified, and the value for that field must be non-null."}},
		},
		{
			name: `for GraphQLList: [1, 2, 3]`,
			in:   value.Just[any]([]any{1, 2, 3}),
			as:   listOfInt,
			want: nil,
		},
		{
			name: `for GraphQLList: [1, 'b', true, 4]`,
			in:   value.Just[any]([]any{1, "b", true, 4}),
			as:   listOfInt,
			want: []complaint{{path: []any{1}, says: "Int cannot represent non-integer value: \"b\""}, {path: []any{2}, says: "Int cannot represent non-integer value: true"}},
		},
		{
			name: `for GraphQLList: 42`,
			in:   value.Just[any](42),
			as:   listOfInt,
			want: nil,
		},
		{
			name: `for GraphQLList: 'INVALID'`,
			in:   value.Just[any]("INVALID"),
			as:   listOfInt,
			want: []complaint{{says: "Int cannot represent non-integer value: \"INVALID\""}},
		},
		{
			name: `for GraphQLList: null`,
			in:   value.Just[any](nil),
			as:   listOfInt,
			want: nil,
		},
		{
			name: `for nested GraphQLList: [[1], [2, 3]]`,
			in:   value.Just[any]([]any{[]any{1}, []any{2, 3}}),
			as:   nestedListOfInt,
			want: nil,
		},
		{
			name: `for nested GraphQLList: 42`,
			in:   value.Just[any](42),
			as:   nestedListOfInt,
			want: nil,
		},
		{
			name: `for nested GraphQLList: null`,
			in:   value.Just[any](nil),
			as:   nestedListOfInt,
			want: nil,
		},
		{
			name: `for nested GraphQLList: [1, 2, 3]`,
			in:   value.Just[any]([]any{1, 2, 3}),
			as:   nestedListOfInt,
			want: nil,
		},
		{
			name: `for nested GraphQLList: [42, [null], null]`,
			in:   value.Just[any]([]any{42, []any{nil}, nil}),
			as:   nestedListOfInt,
			want: nil,
		},
	})
}

func TestPortedValidateInputLiteral(t *testing.T) {
	literalTestScalar := schema.NewScalar(schema.ScalarConfig{
		Name: "TestScalar",
		CoerceInputLiteral: func(literal language.Value) (value.Maybe[any], error) {
			// The literal reaching a scalar is already a constant.
			given, ok := schema.ValueFromASTUntyped(literal, schema.VariableValues{})
			held, isObject := given.(map[string]any)
			if !ok || !isObject {
				return value.Nothing[any](), errors.New("TestScalar wants an object")
			}
			if why, failed := held["error"]; failed {
				return value.Nothing[any](), errors.New(why.(string))
			}
			written, present := held["value"]
			if !present {
				// As above: undefined in graphql-js, nothing here.
				return value.Nothing[any](), nil
			}
			return value.Just[any](written), nil
		},
	})
	literalThrowScalar := schema.NewScalar(schema.ScalarConfig{
		Name: "TestScalar",
		CoerceInputLiteral: func(language.Value) (value.Maybe[any], error) {
			return value.Nothing[any](), errors.New("Not an error object.")
		},
	})
	// The literal half of the file gives this type a field that must be given
	// but has a default, which the value half does not.
	literalTestInputObject := schema.NewInputObject(schema.InputObjectConfig{
		Name: "TestInputObject",
		Fields: []*schema.InputField{
			schema.NewInputField("foo", schema.InputFieldConfig{Type: schema.NewNonNull(schema.Int)}),
			schema.NewInputField("bar", schema.InputFieldConfig{Type: schema.Int}),
			schema.NewInputField("optional", schema.InputFieldConfig{
				Type:    schema.NewNonNull(schema.Int),
				Default: value.Just(schema.DefaultInput{Value: 42}),
			}),
		},
	})

	runValidateLiterals(t, []validateCase{
		{
			name: `'$var'`,
			in:   "$var",
			as:   schema.NewNonNull(schema.Int),
			want: nil,
		},
		{
			name: `for GraphQLNonNull: '1'`,
			in:   "1",
			as:   schema.NewNonNull(schema.Int),
			want: nil,
		},
		{
			name: `for GraphQLNonNull: 'null'`,
			in:   "null",
			as:   schema.NewNonNull(schema.Int),
			want: []complaint{{says: "Expected value of non-null type \"Int!\" not to be null."}},
		},
		{
			name: `for GraphQLScalar: '{ value: 1 }'`,
			in:   "{ value: 1 }",
			as:   literalTestScalar,
			want: nil,
		},
		{
			name: `for GraphQLScalar: '{ value: null }'`,
			in:   "{ value: null }",
			as:   literalTestScalar,
			want: nil,
		},
		{
			name: `for GraphQLScalar: '{ value: NaN }'`,
			in:   "{ value: NaN }",
			as:   literalTestScalar,
			want: nil,
		},
		{
			name: `for GraphQLScalar: '{}'`,
			in:   "{}",
			as:   literalTestScalar,
			want: []complaint{{says: "Expected value of type \"TestScalar\", found: {  }."}},
		},
		{
			name: `for GraphQLScalar: '{}' (2)`,
			in:   "{}",
			as:   literalThrowScalar,
			want: []complaint{{says: "Expected value of type \"TestScalar\", but encountered error \"Not an error object.\"; found: {  }."}},
		},
		{
			name: `for GraphQLEnum: 'FOO'`,
			in:   "FOO",
			as:   testEnum,
			want: nil,
		},
		{
			name: `for GraphQLEnum: 'BAR'`,
			in:   "BAR",
			as:   testEnum,
			want: nil,
		},
		{
			name: `for GraphQLEnum: 'UNKNOWN'`,
			in:   "UNKNOWN",
			as:   testEnum,
			want: []complaint{{says: "Value \"UNKNOWN\" does not exist in \"TestEnum\" enum."}},
		},
		{
			name: `for GraphQLEnum: 'foo'`,
			in:   "foo",
			as:   testEnum,
			want: []complaint{{says: "Value \"foo\" does not exist in \"TestEnum\" enum. Did you mean the enum value \"FOO\"?"}},
		},
		{
			name: `for GraphQLEnum: '"FOO"'`,
			in:   "\"FOO\"",
			as:   testEnum,
			want: []complaint{{says: "Enum \"TestEnum\" cannot represent non-enum value: \"FOO\". Did you mean the enum value \"FOO\"?"}},
		},
		{
			name: `for GraphQLEnum: '"UNKNOWN"'`,
			in:   "\"UNKNOWN\"",
			as:   testEnum,
			want: []complaint{{says: "Enum \"TestEnum\" cannot represent non-enum value: \"UNKNOWN\"."}},
		},
		{
			name: `for GraphQLEnum: '123'`,
			in:   "123",
			as:   testEnum,
			want: []complaint{{says: "Enum \"TestEnum\" cannot represent non-enum value: 123."}},
		},
		{
			name: `for GraphQLEnum: '{ field: "value" }'`,
			in:   "{ field: \"value\" }",
			as:   testEnum,
			want: []complaint{{says: "Enum \"TestEnum\" cannot represent non-enum value: { field: \"value\" }."}},
		},
		{
			name: `for GraphQLInputObject: '{ foo: 123 }'`,
			in:   "{ foo: 123 }",
			as:   literalTestInputObject,
			want: nil,
		},
		{
			name: `for GraphQLInputObject: '123'`,
			in:   "123",
			as:   literalTestInputObject,
			want: []complaint{{says: "Expected value of type \"TestInputObject\" to be an object, found: 123."}},
		},
		{
			name: `for GraphQLInputObject: '{ foo: 1.5 }'`,
			in:   "{ foo: 1.5 }",
			as:   literalTestInputObject,
			want: []complaint{{path: []any{"foo"}, says: "Int cannot represent non-integer value: 1.5"}},
		},
		{
			name: `for GraphQLInputObject: '{ foo: "abc", bar: "def" }'`,
			in:   "{ foo: \"abc\", bar: \"def\" }",
			as:   literalTestInputObject,
			want: []complaint{{path: []any{"foo"}, says: "Int cannot represent non-integer value: \"abc\""}, {path: []any{"bar"}, says: "Int cannot represent non-integer value: \"def\""}},
		},
		{
			name: `for GraphQLInputObject: '{ bar: 123 }'`,
			in:   "{ bar: 123 }",
			as:   literalTestInputObject,
			want: []complaint{{says: "Expected value of type \"TestInputObject\" to include required field \"foo\", found: { bar: 123 }."}},
		},
		{
			name: `for GraphQLInputObject: '{ foo: 123, bart: 123 }'`,
			in:   "{ foo: 123, bart: 123 }",
			as:   literalTestInputObject,
			want: []complaint{{says: "Expected value of type \"TestInputObject\" not to include unknown field \"bart\". Did you mean \"bar\"? Found: { foo: 123, bart: 123 }."}},
		},
		{
			name: `for GraphQLInputObject: '{ foo: $var }'`,
			in:   "{ foo: $var }",
			as:   literalTestInputObject,
			want: nil,
		},
		{
			name: `for GraphQLInputObject with default value: '{ foo: 5 }'`,
			in:   "{ foo: 5 }",
			as:   objectDefaulting(7),
			want: nil,
		},
		{
			name: `for GraphQLInputObject with default value: '{}'`,
			in:   "{}",
			as:   objectDefaulting(7),
			want: nil,
		},
		{
			name: `for GraphQLInputObject with default value: '{}' (2)`,
			in:   "{}",
			as:   objectDefaulting(nil),
			want: nil,
		},
		{
			name: `for GraphQLInputObject with default value: '{}' (3)`,
			in:   "{}",
			as:   objectDefaulting(math.NaN()),
			want: nil,
		},
		{
			name: `for GraphQLInputObject that isOneOf: '{ foo: 123 }'`,
			in:   "{ foo: 123 }",
			as:   testOneOfInput,
			want: nil,
		},
		{
			name: `for GraphQLInputObject that isOneOf: '{ foo: 123, bar: null }'`,
			in:   "{ foo: 123, bar: null }",
			as:   testOneOfInput,
			want: []complaint{{says: "Within OneOf Input Object type \"TestInputObject\", exactly one field must be specified, and the value for that field must be non-null."}},
		},
		{
			name: `for GraphQLInputObject that isOneOf: '{ bar: null }'`,
			in:   "{ bar: null }",
			as:   testOneOfInput,
			want: []complaint{{path: []any{"bar"}, says: "Within OneOf Input Object type \"TestInputObject\", exactly one field must be specified, and the value for that field must be non-null."}},
		},
		{
			name: `for GraphQLInputObject that isOneOf: '123'`,
			in:   "123",
			as:   testOneOfInput,
			want: []complaint{{says: "Expected value of type \"TestInputObject\" to be an object, found: 123."}},
		},
		{
			name: `for GraphQLInputObject that isOneOf: '{ foo: 1.5 }'`,
			in:   "{ foo: 1.5 }",
			as:   testOneOfInput,
			want: []complaint{{path: []any{"foo"}, says: "Int cannot represent non-integer value: 1.5"}},
		},
		{
			name: `for GraphQLInputObject that isOneOf: '{ foo: 123, bart: 123 }'`,
			in:   "{ foo: 123, bart: 123 }",
			as:   testOneOfInput,
			want: []complaint{{says: "Expected value of type \"TestInputObject\" not to include unknown field \"bart\". Did you mean \"bar\"? Found: { foo: 123, bart: 123 }."}},
		},
		{
			name: `for GraphQLInputObject that isOneOf: '{ foo: $var }'`,
			in:   "{ foo: $var }",
			as:   testOneOfInput,
			want: nil,
		},
		{
			name: `for GraphQLList: '[1, 2, 3]'`,
			in:   "[1, 2, 3]",
			as:   listOfInt,
			want: nil,
		},
		{
			name: `for GraphQLList: '[1, "b", true, 4]'`,
			in:   "[1, \"b\", true, 4]",
			as:   listOfInt,
			want: []complaint{{path: []any{1}, says: "Int cannot represent non-integer value: \"b\""}, {path: []any{2}, says: "Int cannot represent non-integer value: true"}},
		},
		{
			name: `for GraphQLList: '42'`,
			in:   "42",
			as:   listOfInt,
			want: nil,
		},
		{
			name: `for GraphQLList: '"INVALID"'`,
			in:   "\"INVALID\"",
			as:   listOfInt,
			want: []complaint{{says: "Int cannot represent non-integer value: \"INVALID\""}},
		},
		{
			name: `for GraphQLList: 'null'`,
			in:   "null",
			as:   listOfInt,
			want: nil,
		},
		{
			name: `for GraphQLList: '[1, $var, 3]'`,
			in:   "[1, $var, 3]",
			as:   listOfInt,
			want: nil,
		},
		{
			name: `for nested GraphQLList: '[[1], [2, 3]]'`,
			in:   "[[1], [2, 3]]",
			as:   nestedListOfInt,
			want: nil,
		},
		{
			name: `for nested GraphQLList: '42'`,
			in:   "42",
			as:   nestedListOfInt,
			want: nil,
		},
		{
			name: `for nested GraphQLList: 'null'`,
			in:   "null",
			as:   nestedListOfInt,
			want: nil,
		},
		{
			name: `for nested GraphQLList: '[1, 2, 3]'`,
			in:   "[1, 2, 3]",
			as:   nestedListOfInt,
			want: nil,
		},
		{
			name: `for nested GraphQLList: '[42, [null], null]'`,
			in:   "[42, [null], null]",
			as:   nestedListOfInt,
			want: nil,
		},
	})
}

// validateCase is one of graphql-js's assertions.
type validateCase struct {
	name string
	// in is the value, or the literal as it is written in a document.
	in any
	// as is the type it is read as.
	as schema.Type
	// want is what should be said about it, in the order it should be said.
	want []complaint
}

// The fixtures both halves of the file share.
var (
	testEnum = schema.NewEnum(schema.EnumConfig{
		Name: "TestEnum",
		Values: []*schema.EnumValue{
			schema.NewEnumValue("FOO", schema.EnumValueConfig{Value: schema.InternalValue("InternalFoo")}),
			schema.NewEnumValue("BAR", schema.EnumValueConfig{Value: schema.InternalValue(123456789)}),
		},
	})
	testOneOfInput = schema.NewInputObject(schema.InputObjectConfig{
		Name:    "TestInputObject",
		IsOneOf: true,
		Fields: []*schema.InputField{
			schema.NewInputField("foo", schema.InputFieldConfig{Type: schema.Int}),
			schema.NewInputField("bar", schema.InputFieldConfig{Type: schema.Int}),
		},
	})
	listOfInt       = schema.NewList(schema.Int)
	nestedListOfInt = schema.NewList(schema.NewList(schema.Int))
)

// objectDefaulting is graphql-js's makeTestInputObject: one field of a scalar
// that accepts anything, with the given default.
func objectDefaulting(fallback any) *schema.InputObjectType {
	return schema.NewInputObject(schema.InputObjectConfig{
		Name: "TestInputObject",
		Fields: []*schema.InputField{
			schema.NewInputField("foo", schema.InputFieldConfig{
				Type:    schema.NewScalar(schema.ScalarConfig{Name: "TestScalar"}),
				Default: value.Just(schema.DefaultInput{Value: fallback}),
			}),
		},
	})
}

func runValidateValues(t *testing.T, cases []validateCase) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			found := schema.ValidateSuppliedInputValue(tt.in.(value.Maybe[any]), tt.as)
			got := make([]complaint, len(found))
			for i, why := range found {
				got[i] = complaint{path: why.Path, says: why.Message}
			}
			checkComplaints(t, tt.name, got, tt.want)
		})
	}
}

func runValidateLiterals(t *testing.T, cases []validateCase) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			found := schema.ValidateInputLiteral(literalOf(t, tt.in.(string)), tt.as, schema.VariableValues{})
			got := make([]complaint, len(found))
			for i, why := range found {
				got[i] = complaint{path: why.Path, says: why.Message}
			}
			checkComplaints(t, tt.name, got, tt.want)
		})
	}
}

func checkComplaints(t *testing.T, name string, got, want []complaint) {
	t.Helper()
	same := len(got) == len(want)
	if same {
		for i := range got {
			if got[i].says != want[i].says || !samePath(got[i].path, want[i].path) {
				same = false
				break
			}
		}
	}
	if why, listed := knownValidateDivergences[name]; listed {
		if same {
			t.Errorf("this case now matches graphql-js; remove it from the known divergences (%s)", why)
		} else {
			t.Logf("known divergence: %s", why)
		}
		return
	}
	if !same {
		t.Errorf("said %v, want %v", got, want)
	}
}

// samePath compares two paths, counting an empty one and a missing one as the
// same: both mean the value as a whole.
func samePath(got, want []any) bool {
	if len(got) == 0 && len(want) == 0 {
		return true
	}
	return reflect.DeepEqual(got, want)
}

// Not ported: the six cases that ask for the suggestions to be left out.
// There is no such option here — a message either has something to suggest or
// it does not.

// orderedOf builds an ordered map from alternating keys and values, standing
// in for the object literal graphql-js's own test writes.
func orderedOf(pairs ...any) *value.OrderedMap {
	o := value.NewOrderedMapSize(len(pairs) / 2)
	for i := 0; i+1 < len(pairs); i += 2 {
		o.Set(pairs[i].(string), pairs[i+1])
	}
	return o
}
