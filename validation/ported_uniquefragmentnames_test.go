package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/UniqueFragmentNamesRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_UniqueFragmentNames(t *testing.T) {
	runPorted(t, validation.UniqueFragmentNamesRule, []portedCase{
		{
			name: `no fragments`,
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
			name: `one fragment`,
			steps: []portedStep{
				{
					query: `
      {
        ...fragA
      }

      fragment fragA on Type {
        field
      }
    `,
				},
			},
		},
		{
			name: `many fragments`,
			steps: []portedStep{
				{
					query: `
      {
        ...fragA
        ...fragB
        ...fragC
      }
      fragment fragA on Type {
        fieldA
      }
      fragment fragB on Type {
        fieldB
      }
      fragment fragC on Type {
        fieldC
      }
    `,
				},
			},
		},
		{
			name: `inline fragments are always unique`,
			steps: []portedStep{
				{
					query: `
      {
        ...on Type {
          fieldA
        }
        ...on Type {
          fieldB
        }
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
			name: `fragments named the same`,
			steps: []portedStep{
				{
					query: `
      {
        ...fragA
      }
      fragment fragA on Type {
        fieldA
      }
      fragment fragA on Type {
        fieldB
      }
    `,
					want: []want{
						{At: []at{{5, 16}, {8, 16}}},
					},
				},
			},
		},
		{
			name: `fragments named the same without being referenced`,
			steps: []portedStep{
				{
					query: `
      fragment fragA on Type {
        fieldA
      }
      fragment fragA on Type {
        fieldB
      }
    `,
					want: []want{
						{At: []at{{2, 16}, {5, 16}}},
					},
				},
			},
		},
	})
}
