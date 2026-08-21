package schema_test

// Ported from graphql-js src/utilities/__tests__/valueToLiteral-test.ts, which
// is the other direction from coercion: a Go value turned back into the
// literal a document would have written for it.
//
// A want of "" means the value cannot be written as that type at all.

import (
	"errors"
	"math"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

func TestPortedLiteralFromValue(t *testing.T) {
	literalEnum := schema.NewEnum(schema.EnumConfig{
		Name: "MyEnum",
		Values: []*schema.EnumValue{
			schema.NewEnumValue("HELLO", schema.EnumValueConfig{}),
			schema.NewEnumValue("COMPLEX", schema.EnumValueConfig{
				Value: schema.InternalValue(map[string]any{"someArbitrary": "complexValue"}),
			}),
		},
	})
	literalInputObject := schema.NewInputObject(schema.InputObjectConfig{
		Name: "MyInputObj",
		Fields: []*schema.InputField{
			schema.NewInputField("foo", schema.InputFieldConfig{Type: schema.NewNonNull(schema.Float)}),
			schema.NewInputField("bar", schema.InputFieldConfig{Type: schema.ID}),
		},
	})
	// A scalar that writes its own literals, one that refuses to, and one that
	// says nothing and takes what the generic conversion makes.
	scalarWritingItsOwn := schema.NewScalar(schema.ScalarConfig{
		Name: "CustomScalar",
		ValueToLiteral: func(internal any, _ schema.Type) (language.Value, error) {
			text, isText := internal.(string)
			if !isText || len(text) == 0 || text[0] != '#' {
				return nil, errors.New("CustomScalar wants a value beginning with #")
			}
			return &language.EnumValue{Value: text[1:]}, nil
		},
	})
	scalarThatRefuses := schema.NewScalar(schema.ScalarConfig{
		Name: "CustomScalar",
		ValueToLiteral: func(any, schema.Type) (language.Value, error) {
			return nil, errors.New("no")
		},
	})
	scalarWithNothingToSay := schema.NewScalar(schema.ScalarConfig{Name: "CustomScalar"})

	for _, tt := range []struct {
		name string
		in   any
		as   schema.Type
		want string
	}{
		{
			name: `converts null values to Null AST: null`,
			in:   nil,
			as:   schema.String,
			want: `null`,
		},
		{
			name: `converts null values to Null AST: undefined`,
			in:   nil,
			as:   schema.String,
			want: `null`,
		},
		{
			name: `converts null values to Null AST: null (2)`,
			in:   nil,
			as:   schema.NewNonNull(schema.String),
			want: "",
		},
		{
			name: `converts boolean values to Boolean ASTs: true`,
			in:   true,
			as:   schema.Boolean,
			want: `true`,
		},
		{
			name: `converts boolean values to Boolean ASTs: false`,
			in:   false,
			as:   schema.Boolean,
			want: `false`,
		},
		{
			name: `converts boolean values to Boolean ASTs: 'false'`,
			in:   "false",
			as:   schema.Boolean,
			want: "",
		},
		{
			name: `converts int number values to Int ASTs: 0`,
			in:   0,
			as:   schema.Int,
			want: `0`,
		},
		{
			name: `converts int number values to Int ASTs: -1`,
			in:   -1,
			as:   schema.Int,
			want: `-1`,
		},
		{
			name: `converts int number values to Int ASTs: 2147483647`,
			in:   2147483647,
			as:   schema.Int,
			want: `2147483647`,
		},
		{
			name: `converts int number values to Int ASTs: 2147483648`,
			in:   2147483648,
			as:   schema.Int,
			want: "",
		},
		{
			name: `converts int number values to Int ASTs: 0.5`,
			in:   0.5,
			as:   schema.Int,
			want: "",
		},
		{
			name: `converts float number values to Float ASTs: 123.5`,
			in:   123.5,
			as:   schema.Float,
			want: `123.5`,
		},
		{
			name: `converts float number values to Float ASTs: 2e40`,
			in:   2e+40,
			as:   schema.Float,
			want: `2e+40`,
		},
		{
			name: `converts float number values to Float ASTs: 1099511627776`,
			in:   1099511627776,
			as:   schema.Float,
			want: `1099511627776`,
		},
		{
			name: `converts float number values to Float ASTs: '0.5'`,
			in:   "0.5",
			as:   schema.Float,
			want: "",
		},
		{
			name: `converts float number values to Float ASTs: NaN`,
			in:   math.NaN(),
			as:   schema.Float,
			want: "",
		},
		{
			name: `converts float number values to Float ASTs: Infinity`,
			in:   math.Inf(1),
			as:   schema.Float,
			want: "",
		},
		{
			name: `converts String ASTs to String values: 'hello world'`,
			in:   "hello world",
			as:   schema.String,
			want: `"hello world"`,
		},
		{
			name: `converts String ASTs to String values: 123`,
			in:   123,
			as:   schema.String,
			want: "",
		},
		{
			name: `converts ID values to Int/String ASTs: 'hello world'`,
			in:   "hello world",
			as:   schema.ID,
			want: `"hello world"`,
		},
		{
			name: `converts ID values to Int/String ASTs: '123'`,
			in:   "123",
			as:   schema.ID,
			want: `123`,
		},
		{
			name: `converts ID values to Int/String ASTs: 123`,
			in:   123,
			as:   schema.ID,
			want: `123`,
		},
		{
			name: `converts ID values to Int/String ASTs: '123456789123456789123456789123456789'`,
			in:   "123456789123456789123456789123456789",
			as:   schema.ID,
			want: `123456789123456789123456789123456789`,
		},
		{
			name: `converts ID values to Int/String ASTs: 123.5`,
			in:   123.5,
			as:   schema.ID,
			want: "",
		},
		{
			name: `converts Enum names to Enum ASTs: 'HELLO'`,
			in:   "HELLO",
			as:   literalEnum,
			want: `HELLO`,
		},
		{
			name: `converts Enum names to Enum ASTs: 'COMPLEX'`,
			in:   "COMPLEX",
			as:   literalEnum,
			want: `COMPLEX`,
		},
		{
			name: `converts Enum names to Enum ASTs: 'GOODBYE'`,
			in:   "GOODBYE",
			as:   literalEnum,
			want: "",
		},
		{
			name: `converts Enum names to Enum ASTs: 123`,
			in:   123,
			as:   literalEnum,
			want: "",
		},
		{
			name: `converts List ASTs to array values: ['FOO', 'BAR']`,
			in:   []any{"FOO", "BAR"},
			as:   schema.NewList(schema.String),
			want: `["FOO", "BAR"]`,
		},
		{
			name: `converts List ASTs to array values: ['123', 123]`,
			in:   []any{"123", 123},
			as:   schema.NewList(schema.ID),
			want: `[123, 123]`,
		},
		{
			name: `converts List ASTs to array values: ['FOO', 123]`,
			in:   []any{"FOO", 123},
			as:   schema.NewList(schema.String),
			want: "",
		},
		{
			name: `converts List ASTs to array values: 'FOO'`,
			in:   "FOO",
			as:   schema.NewList(schema.String),
			want: `"FOO"`,
		},
		{
			name: `converts input objects: { foo: 3, bar: '3' }`,
			in:   map[string]any{"foo": 3, "bar": "3"},
			as:   literalInputObject,
			want: `{ foo: 3, bar: 3 }`,
		},
		{
			name: `converts input objects: { foo: 3 }`,
			in:   map[string]any{"foo": 3},
			as:   literalInputObject,
			want: `{ foo: 3 }`,
		},
		{
			name: `converts input objects: '123'`,
			in:   "123",
			as:   literalInputObject,
			want: "",
		},
		{
			name: `converts input objects: { foo: '3' }`,
			in:   map[string]any{"foo": "3"},
			as:   literalInputObject,
			want: "",
		},
		{
			name: `converts input objects: { bar: 3 }`,
			in:   map[string]any{"bar": 3},
			as:   literalInputObject,
			want: "",
		},
		{
			name: `custom scalar types may define valueToLiteral: '#FOO'`,
			in:   "#FOO",
			as:   scalarWritingItsOwn,
			want: `FOO`,
		},
		{
			name: `custom scalar types may define valueToLiteral: 'FOO'`,
			in:   "FOO",
			as:   scalarWritingItsOwn,
			want: "",
		},
		{
			name: `custom scalar types may throw errors from valueToLiteral: 'FOO'`,
			in:   "FOO",
			as:   scalarThatRefuses,
			want: "",
		},
		{
			name: `custom scalar types may fall back on default valueToLiteral: { foo: 'bar' }`,
			in:   map[string]any{"foo": "bar"},
			as:   scalarWithNothingToSay,
			want: `{ foo: "bar" }`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := schema.LiteralFromValue(tt.in, tt.as)
			if tt.want == "" {
				if ok {
					t.Errorf("wrote %s, want nothing", language.Print(got))
				}
				return
			}
			if !ok {
				t.Fatalf("nothing was written, want %s", tt.want)
			}
			if written := language.Print(got); written != tt.want {
				t.Errorf("wrote %s, want %s", written, tt.want)
			}
		})
	}
}
