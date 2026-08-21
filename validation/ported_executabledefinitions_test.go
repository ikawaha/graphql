package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/ExecutableDefinitionsRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_ExecutableDefinitions(t *testing.T) {
	runPorted(t, validation.ExecutableDefinitionsRule, []portedCase{
		{
			name: `with only operation`,
			steps: []portedStep{
				{
					query: `
      query Foo {
        dog {
          name
        }
      }
    `,
				},
			},
		},
		{
			name: `with operation and fragment`,
			steps: []portedStep{
				{
					query: `
      query Foo {
        dog {
          name
          ...Frag
        }
      }

      fragment Frag on Dog {
        name
      }
    `,
				},
			},
		},
		{
			name: `with type definition`,
			steps: []portedStep{
				{
					query: `
      query Foo {
        dog {
          name
        }
      }

      type Cow {
        name: String
      }

      extend type Dog {
        color: String
      }
    `,
					want: []want{
						{At: []at{{8, 7}}},
						{At: []at{{12, 7}}},
					},
				},
			},
		},
		{
			name: `with schema definition`,
			steps: []portedStep{
				{
					query: `
      schema {
        query: Query
      }

      type Query {
        test: String
      }

      extend schema @directive
    `,
					want: []want{
						{At: []at{{2, 7}}},
						{At: []at{{6, 7}}},
						{At: []at{{10, 7}}},
					},
				},
			},
		},
	})
}
