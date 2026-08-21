package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/LoneAnonymousOperationRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_LoneAnonymousOperation(t *testing.T) {
	runPorted(t, validation.LoneAnonymousOperationRule, []portedCase{
		{
			name: `no operations`,
			steps: []portedStep{
				{
					query: `
      fragment fragA on Type {
        field
      }
    `,
				},
			},
		},
		{
			name: `one anon operation`,
			steps: []portedStep{
				{
					query: `
      {
        field
      }
    `,
				},
			},
		},
		{
			name: `multiple named operations`,
			steps: []portedStep{
				{
					query: `
      query Foo {
        field
      }

      query Bar {
        field
      }
    `,
				},
			},
		},
		{
			name: `anon operation with fragment`,
			steps: []portedStep{
				{
					query: `
      {
        ...Foo
      }
      fragment Foo on Type {
        field
      }
    `,
				},
			},
		},
		{
			name: `multiple anon operations`,
			steps: []portedStep{
				{
					query: `
      {
        fieldA
      }
      {
        fieldB
      }
    `,
					want: []want{
						{At: []at{{2, 7}}},
						{At: []at{{5, 7}}},
					},
				},
			},
		},
		{
			name: `anon operation with a mutation`,
			steps: []portedStep{
				{
					query: `
      {
        fieldA
      }
      mutation Foo {
        fieldB
      }
    `,
					want: []want{
						{At: []at{{2, 7}}},
					},
				},
			},
		},
		{
			name: `anon operation with a subscription`,
			steps: []portedStep{
				{
					query: `
      {
        fieldA
      }
      subscription Foo {
        fieldB
      }
    `,
					want: []want{
						{At: []at{{2, 7}}},
					},
				},
			},
		},
	})
}
