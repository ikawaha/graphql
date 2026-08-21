package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/UniqueOperationNamesRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_UniqueOperationNames(t *testing.T) {
	runPorted(t, validation.UniqueOperationNamesRule, []portedCase{
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
			name: `one named operation`,
			steps: []portedStep{
				{
					query: `
      query Foo {
        field
      }
    `,
				},
			},
		},
		{
			name: `multiple operations`,
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
			name: `multiple operations of different types`,
			steps: []portedStep{
				{
					query: `
      query Foo {
        field
      }

      mutation Bar {
        field
      }

      subscription Baz {
        field
      }
    `,
				},
			},
		},
		{
			name: `fragment and operation named the same`,
			steps: []portedStep{
				{
					query: `
      query Foo {
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
			name: `multiple operations of same name`,
			steps: []portedStep{
				{
					query: `
      query Foo {
        fieldA
      }
      query Foo {
        fieldB
      }
    `,
					want: []want{
						{At: []at{{2, 13}, {5, 13}}},
					},
				},
			},
		},
		{
			name: `multiple ops of same name of different types (mutation)`,
			steps: []portedStep{
				{
					query: `
      query Foo {
        fieldA
      }
      mutation Foo {
        fieldB
      }
    `,
					want: []want{
						{At: []at{{2, 13}, {5, 16}}},
					},
				},
			},
		},
		{
			name: `multiple ops of same name of different types (subscription)`,
			steps: []portedStep{
				{
					query: `
      query Foo {
        fieldA
      }
      subscription Foo {
        fieldB
      }
    `,
					want: []want{
						{At: []at{{2, 13}, {5, 20}}},
					},
				},
			},
		},
	})
}
