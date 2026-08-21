package schema_test

// Ported from graphql-js src/utilities/__tests__/valueFromAST-test.ts, which
// covers what this implementation calls [schema.CoerceInputLiteral].
//
// The cases that turn on a variable are the point of the file: a literal may
// name a variable the request did not supply, and what that comes to depends
// on where the variable sits.

import (
	"reflect"
	"testing"

	"github.com/ikawaha/graphql/schema"
)

func TestPortedValueFromAST_Literal(t *testing.T) {
	nonNullBool := schema.NewNonNull(schema.Boolean)
	listOfBool := schema.NewList(schema.Boolean)
	listOfNonNullBool := schema.NewList(nonNullBool)

	testInput := schema.NewInputObject(schema.InputObjectConfig{
		Name: "TestInput",
		Fields: []*schema.InputField{
			schema.NewInputField("int", schema.InputFieldConfig{
				Type: schema.Int, Default: schema.DefaultValue(int32(42)),
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

	for _, tt := range []struct {
		name      string
		literal   string
		typ       schema.Type
		variables map[string]any
		want      any
		refused   bool
	}{
		// accepts variable values assuming already coerced
		{name: "a variable nothing was supplied for", literal: "$var", typ: schema.Boolean,
			variables: map[string]any{}, refused: true},
		{name: "a variable holding true", literal: "$var", typ: schema.Boolean,
			variables: map[string]any{"var": true}, want: true},
		{name: "a variable holding null", literal: "$var", typ: schema.Boolean,
			variables: map[string]any{"var": nil}, want: nil},
		{name: "a variable holding null where null is not allowed", literal: "$var", typ: nonNullBool,
			variables: map[string]any{"var": nil}, refused: true},

		// asserts variables are provided as items in lists
		{name: "a missing variable as an entry of a nullable list", literal: "[ $foo ]", typ: listOfBool,
			variables: map[string]any{}, want: []any{nil}},
		{name: "a missing variable as an entry that may not be null", literal: "[ $foo ]",
			typ: listOfNonNullBool, variables: map[string]any{}, refused: true},
		{name: "a supplied variable as an entry", literal: "[ $foo ]", typ: listOfNonNullBool,
			variables: map[string]any{"foo": true}, want: []any{true}},
		{name: "a variable standing for the whole list is not wrapped", literal: "$foo",
			typ: listOfNonNullBool, variables: map[string]any{"foo": true}, want: true},
		{name: "a variable standing for the whole list", literal: "$foo",
			typ: listOfNonNullBool, variables: map[string]any{"foo": []any{true}}, want: []any{true}},

		// omits input object fields for unprovided variables
		{
			name:      "fields whose variable was not supplied are left out",
			literal:   "{ int: $foo, bool: $foo, requiredBool: true }",
			typ:       testInput,
			variables: map[string]any{},
			want:      map[string]any{"int": int32(42), "requiredBool": true},
		},
		{
			name:      "a required field whose variable was not supplied",
			literal:   "{ requiredBool: $foo }",
			typ:       testInput,
			variables: map[string]any{},
			refused:   true,
		},
		{
			name:      "a required field whose variable was supplied",
			literal:   "{ requiredBool: $foo }",
			typ:       testInput,
			variables: map[string]any{"foo": true},
			want:      map[string]any{"int": int32(42), "requiredBool": true},
		},

		// rejects multiple oneOf fields when one variable is unprovided
		{
			name:      "two fields of a oneOf input where one variable was not supplied",
			literal:   "{ a: $a, b: $b }",
			typ:       testOneOf,
			variables: map[string]any{"a": "abc"},
			refused:   true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := schema.CoerceInputLiteral(literalOf(t, tt.literal), tt.typ, schema.NewVariableValues(tt.variables, nil))
			if tt.refused {
				if ok {
					t.Errorf("the literal was read as %#v, want it refused", got)
				}
				return
			}
			if !ok {
				t.Fatalf("the literal was refused, want %#v", tt.want)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("read as %#v, want %#v", got, tt.want)
			}
		})
	}
}
