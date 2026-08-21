package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/LoneSchemaDefinitionRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_LoneSchemaDefinition(t *testing.T) {
	runPorted(t, validation.LoneSchemaDefinitionRule, []portedCase{
		{
			name: `no schema`,
			steps: []portedStep{
				{
					query: `
      type Query {
        foo: String
      }
    `,
					sdl: true,
				},
			},
		},
		{
			name: `one schema definition`,
			steps: []portedStep{
				{
					query: `
      schema {
        query: Foo
      }

      type Foo {
        foo: String
      }
    `,
					sdl: true,
				},
			},
		},
		{
			name: `multiple schema definitions`,
			steps: []portedStep{
				{
					query: `
      schema {
        query: Foo
      }

      type Foo {
        foo: String
      }

      schema {
        mutation: Foo
      }

      schema {
        subscription: Foo
      }
    `,
					sdl: true,
					want: []want{
						{At: []at{{10, 7}}},
						{At: []at{{14, 7}}},
					},
				},
			},
		},
		{
			name: `define schema in schema extension`,
			ownSchema: `
      type Foo {
        foo: String
      }
    `,
			steps: []portedStep{
				{
					query: `
        schema {
          query: Foo
        }
      `,
					sdl:              true,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `redefine schema in schema extension`,
			ownSchema: `
      schema {
        query: Foo
      }

      type Foo {
        foo: String
      }
    `,
			steps: []portedStep{
				{
					query: `
        schema {
          mutation: Foo
        }
      `,
					sdl:              true,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{2, 9}}},
					},
				},
			},
		},
		{
			name: `redefine implicit schema in schema extension`,
			ownSchema: `
      type Query {
        fooField: Foo
      }

      type Foo {
        foo: String
      }
    `,
			steps: []portedStep{
				{
					query: `
        schema {
          mutation: Foo
        }
      `,
					sdl:              true,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{2, 9}}},
					},
				},
			},
		},
		{
			name: `extend schema in schema extension`,
			ownSchema: `
      type Query {
        fooField: Foo
      }

      type Foo {
        foo: String
      }
    `,
			steps: []portedStep{
				{
					query: `
        extend schema {
          mutation: Foo
        }
      `,
					sdl:              true,
					againstOwnSchema: true,
				},
			},
		},
	})
}
