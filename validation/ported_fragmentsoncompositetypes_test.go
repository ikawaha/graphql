package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/FragmentsOnCompositeTypesRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_FragmentsOnCompositeTypes(t *testing.T) {
	runPorted(t, validation.FragmentsOnCompositeTypesRule, []portedCase{
		{
			name: `object is valid fragment type`,
			steps: []portedStep{
				{
					query: `
      fragment validFragment on Dog {
        barks
      }
    `,
				},
			},
		},
		{
			name: `interface is valid fragment type`,
			steps: []portedStep{
				{
					query: `
      fragment validFragment on Pet {
        name
      }
    `,
				},
			},
		},
		{
			name: `object is valid inline fragment type`,
			steps: []portedStep{
				{
					query: `
      fragment validFragment on Pet {
        ... on Dog {
          barks
        }
      }
    `,
				},
			},
		},
		{
			name: `interface is valid inline fragment type`,
			steps: []portedStep{
				{
					query: `
      fragment validFragment on Mammal {
        ... on Canine {
          name
        }
      }
    `,
				},
			},
		},
		{
			name: `inline fragment without type is valid`,
			steps: []portedStep{
				{
					query: `
      fragment validFragment on Pet {
        ... {
          name
        }
      }
    `,
				},
			},
		},
		{
			name: `union is valid fragment type`,
			steps: []portedStep{
				{
					query: `
      fragment validFragment on CatOrDog {
        __typename
      }
    `,
				},
			},
		},
		{
			name: `scalar is invalid fragment type`,
			steps: []portedStep{
				{
					query: `
      fragment scalarFragment on Boolean {
        bad
      }
    `,
					want: []want{
						{At: []at{{2, 34}}},
					},
				},
			},
		},
		{
			name: `enum is invalid fragment type`,
			steps: []portedStep{
				{
					query: `
      fragment scalarFragment on FurColor {
        bad
      }
    `,
					want: []want{
						{At: []at{{2, 34}}},
					},
				},
			},
		},
		{
			name: `input object is invalid fragment type`,
			steps: []portedStep{
				{
					query: `
      fragment inputFragment on ComplexInput {
        stringField
      }
    `,
					want: []want{
						{At: []at{{2, 33}}},
					},
				},
			},
		},
		{
			name: `scalar is invalid inline fragment type`,
			steps: []portedStep{
				{
					query: `
      fragment invalidFragment on Pet {
        ... on String {
          barks
        }
      }
    `,
					want: []want{
						{At: []at{{3, 16}}},
					},
				},
			},
		},
	})
}
