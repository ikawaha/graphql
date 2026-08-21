package execution_test

// Ported from graphql-js src/execution/__tests__/directives-test.ts. The schema
// is a single object type with two String fields, answered from a root value.

import (
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

func TestPortedDirectives(t *testing.T) {
	s, root := testDirectivesSchema(t)
	runPorted(t, s, root, nil, []portedCase{
		{
			name:  `basic query works`,
			query: `{ a, b }`,
			want:  `{"data": {"a": "a", "b": "b"}}`,
		},
		{
			name:  `if true includes scalar`,
			query: `{ a, b @include(if: true) }`,
			want:  `{"data": {"a": "a", "b": "b"}}`,
		},
		{
			name:  `if false omits on scalar`,
			query: `{ a, b @include(if: false) }`,
			want:  `{"data": {"a": "a"}}`,
		},
		{
			name:  `unless false includes scalar`,
			query: `{ a, b @skip(if: false) }`,
			want:  `{"data": {"a": "a", "b": "b"}}`,
		},
		{
			name:  `unless true omits scalar`,
			query: `{ a, b @skip(if: true) }`,
			want:  `{"data": {"a": "a"}}`,
		},
		{
			name: `if false omits fragment spread`,
			query: `
        query {
          a
          ...Frag @include(if: false)
        }
        fragment Frag on TestType {
          b
        }
      `,
			want: `{"data": {"a": "a"}}`,
		},
		{
			name: `if true includes fragment spread`,
			query: `
        query {
          a
          ...Frag @include(if: true)
        }
        fragment Frag on TestType {
          b
        }
      `,
			want: `{"data": {"a": "a", "b": "b"}}`,
		},
		{
			name: `unless false includes fragment spread`,
			query: `
        query {
          a
          ...Frag @skip(if: false)
        }
        fragment Frag on TestType {
          b
        }
      `,
			want: `{"data": {"a": "a", "b": "b"}}`,
		},
		{
			name: `unless true omits fragment spread`,
			query: `
        query {
          a
          ...Frag @skip(if: true)
        }
        fragment Frag on TestType {
          b
        }
      `,
			want: `{"data": {"a": "a"}}`,
		},
		{
			name: `if false omits inline fragment`,
			query: `
        query {
          a
          ... on TestType @include(if: false) {
            b
          }
        }
      `,
			want: `{"data": {"a": "a"}}`,
		},
		{
			name: `if true includes inline fragment`,
			query: `
        query {
          a
          ... on TestType @include(if: true) {
            b
          }
        }
      `,
			want: `{"data": {"a": "a", "b": "b"}}`,
		},
		{
			name: `unless false includes inline fragment`,
			query: `
        query {
          a
          ... on TestType @skip(if: false) {
            b
          }
        }
      `,
			want: `{"data": {"a": "a", "b": "b"}}`,
		},
		{
			name: `unless true includes inline fragment`,
			query: `
        query {
          a
          ... on TestType @skip(if: true) {
            b
          }
        }
      `,
			want: `{"data": {"a": "a"}}`,
		},
		{
			name: `if false omits anonymous inline fragment`,
			query: `
        query {
          a
          ... @include(if: false) {
            b
          }
        }
      `,
			want: `{"data": {"a": "a"}}`,
		},
		{
			name: `if true includes anonymous inline fragment`,
			query: `
        query {
          a
          ... @include(if: true) {
            b
          }
        }
      `,
			want: `{"data": {"a": "a", "b": "b"}}`,
		},
		{
			name: `unless false includes anonymous inline fragment`,
			query: `
        query Q {
          a
          ... @skip(if: false) {
            b
          }
        }
      `,
			want: `{"data": {"a": "a", "b": "b"}}`,
		},
		{
			name: `unless true includes anonymous inline fragment`,
			query: `
        query {
          a
          ... @skip(if: true) {
            b
          }
        }
      `,
			want: `{"data": {"a": "a"}}`,
		},
		{
			name: `include and no skip`,
			query: `
        {
          a
          b @include(if: true) @skip(if: false)
        }
      `,
			want: `{"data": {"a": "a", "b": "b"}}`,
		},
		{
			name: `include and skip`,
			query: `
        {
          a
          b @include(if: true) @skip(if: true)
        }
      `,
			want: `{"data": {"a": "a"}}`,
		},
		{
			name: `no include or skip`,
			query: `
        {
          a
          b @include(if: false) @skip(if: false)
        }
      `,
			want: `{"data": {"a": "a"}}`,
		},
	})
}

// Not ported, because each of these is written in a way this could not
// follow:

// testDirectivesSchema is graphql-js's own schema from directives-test.ts.
func testDirectivesSchema(t *testing.T) (*schema.Schema, any) {
	t.Helper()
	s, err := utilities.BuildSchema(`
		schema { query: TestType }
		type TestType { a: String b: String }
	`)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	return s, map[string]any{"a": "a", "b": "b"}
}
