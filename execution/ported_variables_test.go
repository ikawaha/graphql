package execution_test

// Ported from graphql-js src/execution/__tests__/variables-test.ts. The schema
// these run against is testVariablesSchema.

import (
	"context"
	"fmt"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

func TestPortedVariables(t *testing.T) {
	runPorted(t, testVariablesSchema(t), nil, knownVariablesDivergences, []portedCase{
		{
			name: `executes with complex input`,
			query: `
          {
            fieldWithObjectInput(input: {a: "foo", b: ["bar"], c: "baz"})
          }
        `,
			want: `{"data": {"fieldWithObjectInput": "{ a: \"foo\", b: [\"bar\"], c: \"baz\" }"}}`,
		},
		{
			name: `properly parses single value to list`,
			query: `
          {
            fieldWithObjectInput(input: {a: "foo", b: "bar", c: "baz"})
          }
        `,
			want: `{"data": {"fieldWithObjectInput": "{ a: \"foo\", b: [\"bar\"], c: \"baz\" }"}}`,
		},
		{
			name: `properly parses null value to null`,
			query: `
          {
            fieldWithObjectInput(input: {a: null, b: null, c: "C", d: null})
          }
        `,
			want: `{"data": {"fieldWithObjectInput": "{ a: null, b: null, c: \"C\", d: null }"}}`,
		},
		{
			name: `properly parses null value in list`,
			query: `
          {
            fieldWithObjectInput(input: {b: ["A",null,"C"], c: "C"})
          }
        `,
			want: `{"data": {"fieldWithObjectInput": "{ b: [\"A\", null, \"C\"], c: \"C\" }"}}`,
		},
		{
			name: `does not use incorrect value`,
			query: `
          {
            fieldWithObjectInput(input: ["foo", "bar", "baz"])
          }
        `,
			want: `{"data": {"fieldWithObjectInput": null}, "errors": [{"message": "Argument \"TestType.fieldWithObjectInput(input:)\" has invalid value: Expected value of type \"TestInputObject\" to be an object, found: [\"foo\", \"bar\", \"baz\"].", "path": ["fieldWithObjectInput"], "locations": [{"line": 3, "column": 41}]}]}`,
		},
		{
			name: `properly runs coerceInputLiteral on complex scalar types`,
			query: `
          {
            fieldWithObjectInput(input: {c: "foo", d: "ExternalValue"})
          }
        `,
			want: `{"data": {"fieldWithObjectInput": "{ c: \"foo\", d: \"InternalValue\" }"}}`,
		},
		{
			name: `errors on faulty scalar type input`,
			query: `
          {
            fieldWithObjectInput(input: {c: "foo", e: "bar"})
          }
        `,
			want: `{"data": {"fieldWithObjectInput": null}, "errors": [{"message": "Argument \"TestType.fieldWithObjectInput(input:)\" has invalid value at .e: FaultyScalarErrorMessage", "path": ["fieldWithObjectInput"], "locations": [{"line": 3, "column": 13}], "extensions": {"code": "FaultyScalarErrorExtensionCode"}}]}`,
		},
		{
			name: `executes with complex input (2)`,
			query: `
        query ($input: TestInputObject) {
          fieldWithObjectInput(input: $input)
        }
      `,
			variables: `{"input": {"a": "foo", "b": ["bar"], "c": "baz"}}`,
			want:      `{"data": {"fieldWithObjectInput": "{ a: \"foo\", b: [\"bar\"], c: \"baz\" }"}}`,
		},
		{
			name: `uses undefined when variable not provided`,
			query: `
          query q($input: String) {
            fieldWithNullableStringInput(input: $input)
          }`,
			variables: `{}`,
			want:      `{"data": {"fieldWithNullableStringInput": null}}`,
		},
		{
			name: `uses null when variable provided explicit null value`,
			query: `
          query q($input: String) {
            fieldWithNullableStringInput(input: $input)
          }`,
			variables: `{"input": null}`,
			want:      `{"data": {"fieldWithNullableStringInput": "null"}}`,
		},
		{
			name: `preserves explicit null variables within input object literals`,
			query: `
          query q($input: String) {
            fieldWithObjectInput(input: { a: $input, c: "baz" })
          }`,
			variables: `{"input": null}`,
			want:      `{"data": {"fieldWithObjectInput": "{ a: null, c: \"baz\" }"}}`,
		},
		{
			name: `treats explicitly undefined variable values as omitted`,
			query: `
          query q($input: String = "Default value") {
            fieldWithNullableStringInput(input: $input)
          }`,
			variables: `{"input": "@@undefined"}`,
			want:      `{"data": {"fieldWithNullableStringInput": "\"Default value\""}}`,
		},
		{
			name: `uses default value when not provided`,
			query: `
          query ($input: TestInputObject = {a: "foo", b: ["bar"], c: "baz"}) {
            fieldWithObjectInput(input: $input)
          }
        `,
			want: `{"data": {"fieldWithObjectInput": "{ a: \"foo\", b: [\"bar\"], c: \"baz\" }"}}`,
		},
		{
			name:  `reports invalid default values with variable definition locations`,
			query: `query ($input: String = 123) { fieldWithNullableStringInput(input: $input) }`,
			want:  `{"errors": [{"message": "Variable \"$input\" has invalid default value: String cannot represent a non string value: 123", "locations": [{"line": 1, "column": 8}]}]}`,
		},
		{
			name: `does not use default value when provided`,
			query: `
            query q($input: String = "Default value") {
              fieldWithNullableStringInput(input: $input)
            }
          `,
			variables: `{"input": "Variable value"}`,
			want:      `{"data": {"fieldWithNullableStringInput": "\"Variable value\""}}`,
		},
		{
			name: `uses explicit null value instead of default value`,
			query: `
          query q($input: String = "Default value") {
            fieldWithNullableStringInput(input: $input)
          }`,
			variables: `{"input": null}`,
			want:      `{"data": {"fieldWithNullableStringInput": "null"}}`,
		},
		{
			name: `treats explicitly undefined variable values as omitted (2)`,
			query: `
          query q($input: String = "Default value") {
            fieldWithNullableStringInput(input: $input)
          }`,
			variables: `{"input": "@@undefined"}`,
			want:      `{"data": {"fieldWithNullableStringInput": "\"Default value\""}}`,
		},
		{
			name: `uses null default value when not provided`,
			query: `
          query q($input: String = null) {
            fieldWithNullableStringInput(input: $input)
          }`,
			variables: `{}`,
			want:      `{"data": {"fieldWithNullableStringInput": "null"}}`,
		},
		{
			name: `properly parses single value to list (2)`,
			query: `
        query ($input: TestInputObject) {
          fieldWithObjectInput(input: $input)
        }
      `,
			variables: `{"input": {"a": "foo", "b": "bar", "c": "baz"}}`,
			want:      `{"data": {"fieldWithObjectInput": "{ a: \"foo\", b: [\"bar\"], c: \"baz\" }"}}`,
		},
		{
			name: `executes with complex scalar input`,
			query: `
        query ($input: TestInputObject) {
          fieldWithObjectInput(input: $input)
        }
      `,
			variables: `{"input": {"c": "foo", "d": "ExternalValue"}}`,
			want:      `{"data": {"fieldWithObjectInput": "{ c: \"foo\", d: \"InternalValue\" }"}}`,
		},
		{
			name: `errors on faulty scalar type input (2)`,
			query: `
        query ($input: TestInputObject) {
          fieldWithObjectInput(input: $input)
        }
      `,
			variables: `{"input": {"c": "foo", "e": "ExternalValue"}}`,
			want:      `{"errors": [{"message": "Variable \"$input\" has invalid value at .e: Argument \"TestType.fieldWithObjectInput(input:)\" has invalid value at .e: FaultyScalarErrorMessage", "locations": [{"line": 2, "column": 16}], "extensions": {"code": "FaultyScalarErrorExtensionCode"}}]}`,
		},
		{
			name: `errors on null for nested non-null`,
			query: `
        query ($input: TestInputObject) {
          fieldWithObjectInput(input: $input)
        }
      `,
			variables: `{"input": {"a": "foo", "b": "bar", "c": null}}`,
			want:      `{"errors": [{"message": "Variable \"$input\" has invalid value at .c: Expected value of non-null type \"String!\" not to be null.", "locations": [{"line": 2, "column": 16}]}]}`,
		},
		{
			name: `errors on incorrect type`,
			query: `
        query ($input: TestInputObject) {
          fieldWithObjectInput(input: $input)
        }
      `,
			variables: `{"input": "foo bar"}`,
			want:      `{"errors": [{"message": "Variable \"$input\" has invalid value: Expected value of type \"TestInputObject\" to be an object, found: \"foo bar\".", "locations": [{"line": 2, "column": 16}]}]}`,
		},
		{
			name: `errors on omission of nested non-null`,
			query: `
        query ($input: TestInputObject) {
          fieldWithObjectInput(input: $input)
        }
      `,
			variables: `{"input": {"a": "foo", "b": "bar"}}`,
			want:      `{"errors": [{"message": "Variable \"$input\" has invalid value: Expected value of type \"TestInputObject\" to include required field \"c\", found: { a: \"foo\", b: \"bar\" }.", "locations": [{"line": 2, "column": 16}]}]}`,
		},
		{
			name: `errors on deep nested errors and with many errors`,
			query: `
          query ($input: TestNestedInputObject) {
            fieldWithNestedObjectInput(input: $input)
          }
        `,
			variables: `{"input": {"na": {"a": "foo"}}}`,
			want:      `{"errors": [{"message": "Variable \"$input\" has invalid value at .na: Expected value of type \"TestInputObject\" to include required field \"c\", found: { a: \"foo\" }.", "locations": [{"line": 2, "column": 18}]}, {"message": "Variable \"$input\" has invalid value: Expected value of type \"TestNestedInputObject\" to include required field \"nb\", found: { na: { a: \"foo\" } }.", "locations": [{"line": 2, "column": 18}]}]}`,
		},
		{
			name: `errors on addition of unknown input field`,
			query: `
        query ($input: TestInputObject) {
          fieldWithObjectInput(input: $input)
        }
      `,
			variables: `{"input": {"a": "foo", "b": "bar", "c": "baz", "extra": "dog"}}`,
			want:      `{"errors": [{"message": "Variable \"$input\" has invalid value: Expected value of type \"TestInputObject\" not to include unknown field \"extra\", found: { a: \"foo\", b: \"bar\", c: \"baz\", extra: \"dog\" }.", "locations": [{"line": 2, "column": 16}]}]}`,
		},
		{
			name: `allows custom enum values as inputs`,
			query: `
        {
          null: fieldWithEnumInput(input: NULL)
          NaN: fieldWithEnumInput(input: NAN)
          false: fieldWithEnumInput(input: FALSE)
          customValue: fieldWithEnumInput(input: CUSTOM)
          defaultValue: fieldWithEnumInput(input: DEFAULT_VALUE)
        }
      `,
			want: `{"data": {"null": "null", "NaN": "NaN", "false": "false", "customValue": "\"custom value\"", "defaultValue": "\"DEFAULT_VALUE\""}}`,
		},
		{
			name: `allows non-nullable inputs to have null as enum custom value`,
			query: `
        {
          fieldWithNonNullableEnumInput(input: NULL)
        }
      `,
			want: `{"data": {"fieldWithNonNullableEnumInput": "null"}}`,
		},
		{
			name: `allows nullable inputs to be omitted`,
			query: `
        {
          fieldWithNullableStringInput
        }
      `,
			want: `{"data": {"fieldWithNullableStringInput": null}}`,
		},
		{
			name: `allows nullable inputs to be omitted in a variable`,
			query: `
        query ($value: String) {
          fieldWithNullableStringInput(input: $value)
        }
      `,
			want: `{"data": {"fieldWithNullableStringInput": null}}`,
		},
		{
			name: `allows nullable inputs to be omitted in an unlisted variable`,
			query: `
        query {
          fieldWithNullableStringInput(input: $value)
        }
      `,
			want: `{"data": {"fieldWithNullableStringInput": null}}`,
		},
		{
			name: `allows nullable inputs to be set to null in a variable`,
			query: `
        query ($value: String) {
          fieldWithNullableStringInput(input: $value)
        }
      `,
			variables: `{"value": null}`,
			want:      `{"data": {"fieldWithNullableStringInput": "null"}}`,
		},
		{
			name: `allows nullable inputs to be set to a value in a variable`,
			query: `
        query ($value: String) {
          fieldWithNullableStringInput(input: $value)
        }
      `,
			variables: `{"value": "a"}`,
			want:      `{"data": {"fieldWithNullableStringInput": "\"a\""}}`,
		},
		{
			name: `allows nullable inputs to be set to a value directly`,
			query: `
        {
          fieldWithNullableStringInput(input: "a")
        }
      `,
			want: `{"data": {"fieldWithNullableStringInput": "\"a\""}}`,
		},
		{
			name: `allows non-nullable variable to be omitted given a default`,
			query: `
        query ($value: String! = "default") {
          fieldWithNullableStringInput(input: $value)
        }
      `,
			want: `{"data": {"fieldWithNullableStringInput": "\"default\""}}`,
		},
		{
			name: `allows non-nullable inputs to be omitted given a default`,
			query: `
        query ($value: String = "default") {
          fieldWithNonNullableStringInput(input: $value)
        }
      `,
			want: `{"data": {"fieldWithNonNullableStringInput": "\"default\""}}`,
		},
		{
			name: `does not allow non-nullable inputs to be omitted in a variable`,
			query: `
        query ($value: String!) {
          fieldWithNonNullableStringInput(input: $value)
        }
      `,
			want: `{"errors": [{"message": "Variable \"$value\" has invalid value: Expected a value of non-null type \"String!\" to be provided.", "locations": [{"line": 2, "column": 16}]}]}`,
		},
		{
			name: `does not allow non-nullable inputs to be set to null in a variable`,
			query: `
        query ($value: String!) {
          fieldWithNonNullableStringInput(input: $value)
        }
      `,
			variables: `{"value": null}`,
			want:      `{"errors": [{"message": "Variable \"$value\" has invalid value: Expected value of non-null type \"String!\" not to be null.", "locations": [{"line": 2, "column": 16}]}]}`,
		},
		{
			name: `allows non-nullable inputs to be set to a value in a variable`,
			query: `
        query ($value: String!) {
          fieldWithNonNullableStringInput(input: $value)
        }
      `,
			variables: `{"value": "a"}`,
			want:      `{"data": {"fieldWithNonNullableStringInput": "\"a\""}}`,
		},
		{
			name: `allows non-nullable inputs to be set to a value directly`,
			query: `
        {
          fieldWithNonNullableStringInput(input: "a")
        }
      `,
			want: `{"data": {"fieldWithNonNullableStringInput": "\"a\""}}`,
		},
		{
			name:  `reports error for missing non-nullable inputs`,
			query: `{ fieldWithNonNullableStringInput }`,
			want:  `{"data": {"fieldWithNonNullableStringInput": null}, "errors": [{"message": "Argument \"TestType.fieldWithNonNullableStringInput(input:)\" of required type \"String!\" was not provided.", "locations": [{"line": 1, "column": 3}], "path": ["fieldWithNonNullableStringInput"]}]}`,
		},
		{
			name: `reports error for array passed into string input`,
			query: `
        query ($value: String!) {
          fieldWithNonNullableStringInput(input: $value)
        }
      `,
			variables: `{"value": [1, 2, 3]}`,
			want:      `{"errors": [{"message": "Variable \"$value\" has invalid value: String cannot represent a non string value: [1, 2, 3]", "locations": [{"line": 2, "column": 16}]}]}`,
		},
		{
			name: `reports error for non-provided variables for non-nullable inputs`,
			query: `
        {
          fieldWithNonNullableStringInput(input: $foo)
        }
      `,
			want: `{"data": {"fieldWithNonNullableStringInput": null}, "errors": [{"message": "Argument \"TestType.fieldWithNonNullableStringInput(input:)\" has invalid value: Expected variable \"$foo\" provided to type \"String!\" to provide a runtime value.", "locations": [{"line": 3, "column": 50}], "path": ["fieldWithNonNullableStringInput"]}]}`,
		},
		{
			name: `allows custom scalars`,
			query: `
        {
          fieldWithJSONScalarInput(input: { a: "foo", b: ["bar"], c: "baz" })
        }
      `,
			want: `{"data": {"fieldWithJSONScalarInput": "{ a: \"foo\", b: [\"bar\"], c: \"baz\" }"}}`,
		},
		{
			name: `allows custom scalars with non-embedded variables`,
			query: `
          query ($input: JSONScalar) {
            fieldWithJSONScalarInput(input: $input)
          }
        `,
			variables: `{"input": {"a": "foo", "b": ["bar"], "c": "baz"}}`,
			want:      `{"data": {"fieldWithJSONScalarInput": "{ a: \"foo\", b: [\"bar\"], c: \"baz\" }"}}`,
		},
		{
			name: `allows custom scalars with embedded operation variables`,
			query: `
          query ($input: String) {
            fieldWithJSONScalarInput(input: { a: $input, b: ["bar"], c: "baz" })
          }
        `,
			variables: `{"input": "foo"}`,
			want:      `{"data": {"fieldWithJSONScalarInput": "{ a: \"foo\", b: [\"bar\"], c: \"baz\" }"}}`,
		},
		{
			name: `allows lists to be null`,
			query: `
        query ($input: [String]) {
          list(input: $input)
        }
      `,
			variables: `{"input": null}`,
			want:      `{"data": {"list": "null"}}`,
		},
		{
			name: `allows lists to contain values`,
			query: `
        query ($input: [String]) {
          list(input: $input)
        }
      `,
			variables: `{"input": ["A"]}`,
			want:      `{"data": {"list": "[\"A\"]"}}`,
		},
		{
			name: `allows lists to contain null`,
			query: `
        query ($input: [String]) {
          list(input: $input)
        }
      `,
			variables: `{"input": ["A", null, "B"]}`,
			want:      `{"data": {"list": "[\"A\", null, \"B\"]"}}`,
		},
		{
			name: `does not allow non-null lists to be null`,
			query: `
        query ($input: [String]!) {
          nnList(input: $input)
        }
      `,
			variables: `{"input": null}`,
			want:      `{"errors": [{"message": "Variable \"$input\" has invalid value: Expected value of non-null type \"[String]!\" not to be null.", "locations": [{"line": 2, "column": 16}]}]}`,
		},
		{
			name: `allows non-null lists to contain values`,
			query: `
        query ($input: [String]!) {
          nnList(input: $input)
        }
      `,
			variables: `{"input": ["A"]}`,
			want:      `{"data": {"nnList": "[\"A\"]"}}`,
		},
		{
			name: `allows non-null lists to contain null`,
			query: `
        query ($input: [String]!) {
          nnList(input: $input)
        }
      `,
			variables: `{"input": ["A", null, "B"]}`,
			want:      `{"data": {"nnList": "[\"A\", null, \"B\"]"}}`,
		},
		{
			name: `allows lists of non-nulls to be null`,
			query: `
        query ($input: [String!]) {
          listNN(input: $input)
        }
      `,
			variables: `{"input": null}`,
			want:      `{"data": {"listNN": "null"}}`,
		},
		{
			name: `allows lists of non-nulls to contain values`,
			query: `
        query ($input: [String!]) {
          listNN(input: $input)
        }
      `,
			variables: `{"input": ["A"]}`,
			want:      `{"data": {"listNN": "[\"A\"]"}}`,
		},
		{
			name: `does not allow lists of non-nulls to contain null`,
			query: `
        query ($input: [String!]) {
          listNN(input: $input)
        }
      `,
			variables: `{"input": ["A", null, "B"]}`,
			want:      `{"errors": [{"message": "Variable \"$input\" has invalid value at [1]: Expected value of non-null type \"String!\" not to be null.", "locations": [{"line": 2, "column": 16}]}]}`,
		},
		{
			name: `does not allow non-null lists of non-nulls to be null`,
			query: `
        query ($input: [String!]!) {
          nnListNN(input: $input)
        }
      `,
			variables: `{"input": null}`,
			want:      `{"errors": [{"message": "Variable \"$input\" has invalid value: Expected value of non-null type \"[String!]!\" not to be null.", "locations": [{"line": 2, "column": 16}]}]}`,
		},
		{
			name: `allows non-null lists of non-nulls to contain values`,
			query: `
        query ($input: [String!]!) {
          nnListNN(input: $input)
        }
      `,
			variables: `{"input": ["A"]}`,
			want:      `{"data": {"nnListNN": "[\"A\"]"}}`,
		},
		{
			name: `does not allow non-null lists of non-nulls to contain null`,
			query: `
        query ($input: [String!]!) {
          nnListNN(input: $input)
        }
      `,
			variables: `{"input": ["A", null, "B"]}`,
			want:      `{"errors": [{"message": "Variable \"$input\" has invalid value at [1]: Expected value of non-null type \"String!\" not to be null.", "locations": [{"line": 2, "column": 16}]}]}`,
		},
		{
			name: `does not allow invalid types to be used as values`,
			query: `
        query ($input: TestType!) {
          fieldWithObjectInput(input: $input)
        }
      `,
			variables: `{"input": {"list": ["A", "B"]}}`,
			want:      `{"errors": [{"message": "Variable \"$input\" expected value of type \"TestType!\" which cannot be used as an input type.", "locations": [{"line": 2, "column": 24}]}]}`,
		},
		{
			name: `does not allow unknown types to be used as values`,
			query: `
        query ($input: UnknownType!) {
          fieldWithObjectInput(input: $input)
        }
      `,
			variables: `{"input": "WhoKnows"}`,
			want:      `{"errors": [{"message": "Variable \"$input\" expected value of type \"UnknownType!\" which cannot be used as an input type.", "locations": [{"line": 2, "column": 24}]}]}`,
		},
		{
			name:  `when no argument provided`,
			query: `{ fieldWithDefaultArgumentValue }`,
			want:  `{"data": {"fieldWithDefaultArgumentValue": "\"Hello World\""}}`,
		},
		{
			name: `when omitted variable provided`,
			query: `
        query ($optional: String) {
          fieldWithDefaultArgumentValue(input: $optional)
        }
      `,
			want: `{"data": {"fieldWithDefaultArgumentValue": "\"Hello World\""}}`,
		},
		{
			name: `not when argument cannot be coerced`,
			query: `
        {
          fieldWithDefaultArgumentValue(input: WRONG_TYPE)
        }
      `,
			want: `{"data": {"fieldWithDefaultArgumentValue": null}, "errors": [{"message": "Argument \"TestType.fieldWithDefaultArgumentValue(input:)\" has invalid value: String cannot represent a non string value: WRONG_TYPE", "locations": [{"line": 3, "column": 48}], "path": ["fieldWithDefaultArgumentValue"]}]}`,
		},
		{
			name: `when no runtime value is provided to a non-null argument`,
			query: `
        query optionalVariable($optional: String) {
          fieldWithNonNullableStringInputAndDefaultArgumentValue(input: $optional)
        }
      `,
			want: `{"data": {"fieldWithNonNullableStringInputAndDefaultArgumentValue": "\"Hello World\""}}`,
		},
		{
			name: `does not expose prototype argument names when omitted`,
			query: `
        {
          fieldWithPrototypeNamedArgument
        }
      `,
			want: `{"data": {"fieldWithPrototypeNamedArgument": "missing"}}`,
		},
	})
}

// Not ported, because each of these is written in a way this could not
// follow:
//   - allows custom scalars with embedded fragment variables: fragment arguments are not executed here
//   - allows custom scalars with embedded nested fragment variables: fragment arguments are not executed here
//   - does not allow invalid types to be used as fragment variables: fragment arguments are not executed here
//   - does not expose prototype variable names when omitted: it does not call executeQuery
//   - hides suggestions for invalid default values when specified: it does not call executeQuery
//   - includes suggestions for invalid default values: it does not call executeQuery
//   - localizes invalid default value errors during execution: it does not call executeQuery
//   - localizes nested invalid field default value errors during execution: it does not call executeQuery
//   - return all errors by default: it does not call executeQuery
//   - still returns provided variables with colliding names: it does not call executeQuery
//   - treats explicit undefined values as omitted: it does not call executeQuery
//   - when a definition has a default, is not provided, and spreads another fragment: fragment arguments are not executed here
//   - when a fragment is used with different args: fragment arguments are not executed here
//   - when a fragment-variable is shadowed by an intermediate fragment-spread but defined in the operation-variables: fragment arguments are not executed here
//   - when a nullable argument to a directive with a field default is not provided and shadowed by an operation variable: it does not call executeQuery
//   - when a nullable argument with a field default is not provided and shadowed by an operation variable: fragment arguments are not executed here
//   - when a nullable argument without a field default is not provided and shadowed by an operation variable: fragment arguments are not executed here
//   - when a value is required and not provided: fragment arguments are not executed here
//   - when a value is required and provided: fragment arguments are not executed here
//   - when an argument is shadowed by an operation variable: fragment arguments are not executed here
//   - when argument passed in as list: fragment arguments are not executed here
//   - when argument passed to a directive: fragment arguments are not executed here
//   - when argument passed to a directive on a nested field: fragment arguments are not executed here
//   - when argument variables are used recursively: fragment arguments are not executed here
//   - when argument variables with the same name are used directly and recursively: fragment arguments are not executed here
//   - when maxErrors is equal to number of errors: it does not call executeQuery
//   - when maxErrors is less than number of errors: it does not call executeQuery
//   - when the argument variable is nested in a complex type: fragment arguments are not executed here
//   - when the definition has a default and is not provided: fragment arguments are not executed here
//   - when the definition has a default and is provided: fragment arguments are not executed here
//   - when the definition has a non-nullable default and is provided null: fragment arguments are not executed here
//   - when the definition has an invalid default and is not provided: fragment arguments are not executed here
//   - when the definition has no default and is not provided: fragment arguments are not executed here
//   - when there are no fragment arguments: fragment arguments are not executed here

// TestPortedVariables_MaxCoercionErrors is graphql-js's "getVariableValues:
// limit maximum number of coercion errors". A request that says nothing is
// bounded at fifty; one that says so is bounded where it asked, and is told
// there was more.
func TestPortedVariables_MaxCoercionErrors(t *testing.T) {
	const query = `
      query ($input: [String!]) {
        listNN(input: $input)
      }
    `
	invalid := func(v, index int) string {
		return fmt.Sprintf(
			`Variable "$input" has invalid value at [%d]: String cannot represent a non string value: %d`,
			index, v)
	}
	run := func(t *testing.T, bound value.Maybe[int]) []*gqlerror.Error {
		t.Helper()
		doc, err := language.ParseString(query)
		if err != nil {
			t.Fatalf("parsing: %v", err)
		}
		return execution.Execute(context.Background(), execution.Request{
			Schema:            testVariablesSchema(t),
			Document:          doc,
			Variables:         map[string]value.Maybe[any]{"input": value.Just[any]([]any{0, 1, 2})},
			MaxCoercionErrors: bound,
		}).Errors
	}
	expect := func(t *testing.T, got []*gqlerror.Error, want []string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%d errors, want %d: %v", len(got), len(want), got)
		}
		for i, w := range want {
			if got[i].Message != w {
				t.Errorf("error %d:\n got %s\nwant %s", i, got[i].Message, w)
			}
		}
	}

	const tooMany = "Too many errors processing variables, error limit reached. Execution aborted."

	t.Run("no bound asked for, so the default", func(t *testing.T) {
		expect(t, run(t, value.Nothing[int]()),
			[]string{invalid(0, 0), invalid(1, 1), invalid(2, 2)})
	})
	t.Run("a bound above what there is", func(t *testing.T) {
		expect(t, run(t, value.Just(9)),
			[]string{invalid(0, 0), invalid(1, 1), invalid(2, 2)})
	})
	t.Run("a bound equal to what there is", func(t *testing.T) {
		expect(t, run(t, value.Just(3)),
			[]string{invalid(0, 0), invalid(1, 1), invalid(2, 2)})
	})
	t.Run("a bound below what there is", func(t *testing.T) {
		expect(t, run(t, value.Just(2)), []string{invalid(0, 0), invalid(1, 1), tooMany})
	})
	// Zero is a bound like any other, which is what graphql-js makes of it:
	// the first problem is already one too many.
	t.Run("a bound of zero", func(t *testing.T) {
		expect(t, run(t, value.Just(0)), []string{tooMany})
	})
	t.Run("no bound at all", func(t *testing.T) {
		expect(t, run(t, value.Just(-1)),
			[]string{invalid(0, 0), invalid(1, 1), invalid(2, 2)})
	})
}

// TestPortedVariables_InvalidArgumentDefaults is graphql-js's "localizes
// invalid field default value errors during execution" and its nested
// counterpart: a schema that was never validated can carry a default the type
// will not take, and the executor says so where the argument is used rather
// than letting the field quietly go without.
func TestPortedVariables_InvalidArgumentDefaults(t *testing.T) {
	badLeaf := schema.New(schema.Config{AssumeValid: true, Query: schema.NewObject(schema.ObjectConfig{
		Name: "TestTypeWithInvalidDefaultArgumentValue",
		Fields: []*schema.Field{
			schema.NewField("fieldWithInvalidDefaultArgumentValue", schema.FieldConfig{
				Type: schema.String,
				Args: []*schema.Argument{schema.NewArgument("input", schema.ArgumentConfig{
					Type: schema.String, Default: schema.DefaultValue(123),
				})},
			}),
		},
	})})

	// The argument's own default is sound; the input object it names has a
	// field whose default is not.
	held := schema.NewInputObject(schema.InputObjectConfig{
		Name: "InputWithInvalidNestedFieldDefault",
		Fields: []*schema.InputField{
			schema.NewInputField("foo", schema.InputFieldConfig{
				Type: schema.String, Default: schema.DefaultValue(123),
			}),
		},
	})
	badNested := schema.New(schema.Config{AssumeValid: true, Query: schema.NewObject(schema.ObjectConfig{
		Name: "TestTypeWithInvalidNestedDefaultArgumentValue",
		Fields: []*schema.Field{
			schema.NewField("fieldWithInvalidNestedDefaultArgumentValue", schema.FieldConfig{
				Type: schema.String,
				Args: []*schema.Argument{schema.NewArgument("input", schema.ArgumentConfig{
					Type: held, Default: schema.DefaultValue(map[string]any{}),
				})},
			}),
		},
	})})

	tests := []struct {
		name  string
		s     *schema.Schema
		query string
		want  string
	}{
		{
			name:  "a default the argument's own type will not take",
			s:     badLeaf,
			query: "{ fieldWithInvalidDefaultArgumentValue }",
			want: `Argument "TestTypeWithInvalidDefaultArgumentValue.fieldWithInvalidDefaultArgumentValue(input:)"` +
				` has invalid default value: String cannot represent a non string value: 123`,
		},
		{
			name:  "a default on a field of an input object it holds",
			s:     badNested,
			query: "{ fieldWithInvalidNestedDefaultArgumentValue }",
			want: `Argument "TestTypeWithInvalidNestedDefaultArgumentValue.fieldWithInvalidNestedDefaultArgumentValue(input:)"` +
				` has invalid default value: Expected value of type "String" to be valid, found: 123.`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := language.ParseString(tt.query)
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}
			got := execution.Execute(context.Background(), execution.Request{Schema: tt.s, Document: doc})
			if len(got.Errors) != 1 {
				t.Fatalf("%d errors, want 1: %v", len(got.Errors), got.Errors)
			}
			if got.Errors[0].Message != tt.want {
				t.Errorf("\n got %s\nwant %s", got.Errors[0].Message, tt.want)
			}
			// The field it happened under is where the response says it was.
			if len(got.Errors[0].Path) != 1 {
				t.Errorf("path = %v, want the field it happened under", got.Errors[0].Path)
			}
		})
	}
}
