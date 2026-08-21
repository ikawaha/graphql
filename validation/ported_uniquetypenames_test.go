package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/UniqueTypeNamesRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_UniqueTypeNames(t *testing.T) {
	runPorted(t, validation.UniqueTypeNamesRule, []portedCase{
		{
			name: `no types`,
			steps: []portedStep{
				{
					query: `
      directive @test on SCHEMA
    `,
					sdl: true,
				},
			},
		},
		{
			name: `one type`,
			steps: []portedStep{
				{
					query: `
      type Foo
    `,
					sdl: true,
				},
			},
		},
		{
			name: `many types`,
			steps: []portedStep{
				{
					query: `
      type Foo
      type Bar
      type Baz
    `,
					sdl: true,
				},
			},
		},
		{
			name: `type and non-type definitions named the same`,
			steps: []portedStep{
				{
					query: `
      query Foo { __typename }
      fragment Foo on Query { __typename }
      directive @Foo on SCHEMA

      type Foo
    `,
					sdl: true,
				},
			},
		},
		{
			name: `types named the same`,
			steps: []portedStep{
				{
					query: `
      type Foo

      scalar Foo
      type Foo
      interface Foo
      union Foo
      enum Foo
      input Foo
    `,
					sdl: true,
					want: []want{
						{At: []at{{2, 12}, {4, 14}}},
						{At: []at{{2, 12}, {5, 12}}},
						{At: []at{{2, 12}, {6, 17}}},
						{At: []at{{2, 12}, {7, 13}}},
						{At: []at{{2, 12}, {8, 12}}},
						{At: []at{{2, 12}, {9, 13}}},
					},
				},
			},
		},
	})
}

// Not ported, because each of these is written in a way this could not
// follow:
//   - adding new type to existing schema: a document that is not written out
//   - adding new type to existing schema with same-named directive: a document that is not written out
//   - adding conflicting types to existing schema: a document that is not written out
