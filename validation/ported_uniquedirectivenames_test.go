package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/UniqueDirectiveNamesRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_UniqueDirectiveNames(t *testing.T) {
	runPorted(t, validation.UniqueDirectiveNamesRule, []portedCase{
		{
			name: `no directive`,
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
			name: `one directive`,
			steps: []portedStep{
				{
					query: `
      directive @foo on SCHEMA
    `,
					sdl: true,
				},
			},
		},
		{
			name: `many directives`,
			steps: []portedStep{
				{
					query: `
      directive @foo on SCHEMA
      directive @bar on SCHEMA
      directive @baz on SCHEMA
    `,
					sdl: true,
				},
			},
		},
		{
			name: `directive and non-directive definitions named the same`,
			steps: []portedStep{
				{
					query: `
      query foo { __typename }
      fragment foo on foo { __typename }
      type foo

      directive @foo on SCHEMA
    `,
					sdl: true,
				},
			},
		},
		{
			name: `directives named the same`,
			steps: []portedStep{
				{
					query: `
      directive @foo on SCHEMA

      directive @foo on SCHEMA
    `,
					sdl: true,
					want: []want{
						{At: []at{{2, 18}, {4, 18}}},
					},
				},
			},
		},
	})
}

// Not ported, because each of these is written in a way this could not
// follow:
//   - adding new directive to existing schema: a document that is not written out
//   - adding new directive with standard name to existing schema: a document that is not written out
//   - adding new directive to existing schema with same-named type: a document that is not written out
//   - adding conflicting directives to existing schema: a document that is not written out
