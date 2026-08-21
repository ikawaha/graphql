package execution_test

// Ported from graphql-js src/execution/__tests__/oneof-test.ts.

import (
	"context"
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

func TestPortedOneOf(t *testing.T) {
	runPorted(t, testOneOfSchema(t), nil, nil, []portedCase{
		{
			name: `accepts a good default value`,
			query: `
        query ($input: TestInputObject! = {a: "abc"}) {
          test(input: $input) {
            a
            b
          }
        }
      `,
			want: `{"data": {"test": {"a": "abc", "b": null}}}`,
		},
		{
			name: `rejects a bad default value`,
			query: `
        query ($input: TestInputObject! = {a: "abc", b: 123}) {
          test(input: $input) {
            a
            b
          }
        }
      `,
			want: `{"errors": [{"locations": [{"column": 16, "line": 2}], "message": "Variable \"$input\" has invalid default value: Within OneOf Input Object type \"TestInputObject\", exactly one field must be specified, and the value for that field must be non-null."}]}`,
		},
		{
			name: `accepts a good variable`,
			query: `
        query ($input: TestInputObject!) {
          test(input: $input) {
            a
            b
          }
        }
      `,
			variables: `{"input": {"a": "abc"}}`,
			want:      `{"data": {"test": {"a": "abc", "b": null}}}`,
		},
		{
			name: `accepts a good variable with an undefined key`,
			query: `
        query ($input: TestInputObject!) {
          test(input: $input) {
            a
            b
          }
        }
      `,
			variables: `{"input": {"a": "abc", "b": "@@undefined"}}`,
			want:      `{"data": {"test": {"a": "abc", "b": null}}}`,
		},
		{
			name: `rejects a variable with a nulled key`,
			query: `
        query ($input: TestInputObject!) {
          test(input: $input) {
            a
            b
          }
        }
      `,
			variables: `{"input": {"a": null}}`,
			want:      `{"errors": [{"message": "Variable \"$input\" has invalid value at .a: Within OneOf Input Object type \"TestInputObject\", exactly one field must be specified, and the value for that field must be non-null.", "locations": [{"line": 2, "column": 16}]}]}`,
		},
		{
			name: `rejects a variable with multiple non-null keys`,
			query: `
        query ($input: TestInputObject!) {
          test(input: $input) {
            a
            b
          }
        }
      `,
			variables: `{"input": {"a": "abc", "b": 123}}`,
			want:      `{"errors": [{"locations": [{"column": 16, "line": 2}], "message": "Variable \"$input\" has invalid value: Within OneOf Input Object type \"TestInputObject\", exactly one field must be specified, and the value for that field must be non-null."}]}`,
		},
		{
			name: `rejects a variable with multiple nullable keys`,
			query: `
        query ($input: TestInputObject!) {
          test(input: $input) {
            a
            b
          }
        }
      `,
			variables: `{"input": {"a": "abc", "b": null}}`,
			want:      `{"errors": [{"locations": [{"column": 16, "line": 2}], "message": "Variable \"$input\" has invalid value: Within OneOf Input Object type \"TestInputObject\", exactly one field must be specified, and the value for that field must be non-null."}]}`,
		},
		{
			name: `errors with nulled variable for field`,
			query: `
        query ($a: String) {
          test(input: { a: $a }) {
            a
            b
          }
        }
      `,
			variables: `{"a": null}`,
			want:      `{"data": {"test": null}, "errors": [{"message": "Argument \"Query.test(input:)\" has invalid value: Expected variable \"$a\" provided to field \"a\" for OneOf Input Object type \"TestInputObject\" not to be null.", "locations": [{"line": 3, "column": 23}], "path": ["test"]}]}`,
		},
		{
			name: `errors with missing variable for field`,
			query: `
        query ($a: String) {
          test(input: { a: $a }) {
            a
            b
          }
        }
      `,
			want: `{"data": {"test": null}, "errors": [{"message": "Argument \"Query.test(input:)\" has invalid value: Expected variable \"$a\" provided to field \"a\" for OneOf Input Object type \"TestInputObject\" to provide a runtime value.", "locations": [{"line": 3, "column": 23}], "path": ["test"]}]}`,
		},
		{
			name: `errors with missing variable as an additional field`,
			query: `
        query ($a: String, $b: Int) {
          test(input: { a: $a, b: $b }) {
            a
            b
          }
        }
      `,
			variables: `{"a": "abc"}`,
			want:      `{"data": {"test": null}, "errors": [{"message": "Argument \"Query.test(input:)\" has invalid value: Expected variable \"$b\" provided to field \"b\" for OneOf Input Object type \"TestInputObject\" to provide a runtime value.", "locations": [{"line": 3, "column": 23}], "path": ["test"]}]}`,
		},
	})
}

// Not ported, because each of these is written in a way this could not
// follow:
//   - errors with missing fragment variable for field: fragment arguments are not executed here
//   - errors with nulled fragment variable for field: fragment arguments are not executed here

// testOneOfSchema is graphql-js's own schema from oneof-test.ts. Its one field
// answers with the input it was given, so that what a resolver saw is what the
// response shows.
func testOneOfSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := utilities.BuildSchema(`
		type Query {
			test(input: TestInputObject!): TestObject
		}

		input TestInputObject @oneOf {
			a: String
			b: Int
		}

		type TestObject {
			a: String
			b: Int
		}
	`)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	s.QueryType().Field("test").Resolve = func(
		_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo,
	) (any, error) {
		held, _ := args.Get("input")
		return held, nil
	}
	return s
}
