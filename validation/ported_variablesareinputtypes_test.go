package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/VariablesAreInputTypesRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_VariablesAreInputTypes(t *testing.T) {
	runPorted(t, validation.VariablesAreInputTypesRule, []portedCase{
		{
			name: `unknown types are ignored`,
			steps: []portedStep{
				{
					query: `
      query Foo($a: Unknown, $b: [[Unknown!]]!) {
        field(a: $a, b: $b)
      }
      fragment Bar($a: Unknown, $b: [[Unknown!]]!) on Query {
        field(a: $a, b: $b)
      }
    `,
				},
			},
		},
		{
			name: `input types are valid`,
			steps: []portedStep{
				{
					query: `
      query Foo($a: String, $b: [Boolean!]!, $c: ComplexInput) {
        field(a: $a, b: $b, c: $c)
      }
      fragment Bar($a: String, $b: [Boolean!]!, $c: ComplexInput) on Query {
        field(a: $a, b: $b, c: $c)
      }
    `,
				},
			},
		},
		{
			name: `output types are invalid`,
			steps: []portedStep{
				{
					query: `
      query Foo($a: Dog, $b: [[CatOrDog!]]!, $c: Pet) {
        field(a: $a, b: $b, c: $c)
      }
    `,
					want: []want{
						{At: []at{{2, 21}}},
						{At: []at{{2, 30}}},
						{At: []at{{2, 50}}},
					},
				},
			},
		},
		{
			name: `output types on fragment arguments are invalid`,
			steps: []portedStep{
				{
					query: `
      fragment Bar($a: Dog, $b: [[CatOrDog!]]!, $c: Pet) on Query {
        field(a: $a, b: $b, c: $c)
      }
    `,
					want: []want{
						{At: []at{{2, 24}}},
						{At: []at{{2, 33}}},
						{At: []at{{2, 53}}},
					},
				},
			},
		},
	})
}
