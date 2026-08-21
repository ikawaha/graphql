package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/NoUnusedVariablesRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_NoUnusedVariables(t *testing.T) {
	runPorted(t, validation.NoUnusedVariablesRule, []portedCase{
		{
			name: `uses all variables`,
			steps: []portedStep{
				{
					query: `
      query ($a: String, $b: String, $c: String) {
        field(a: $a, b: $b, c: $c)
      }
    `,
				},
			},
		},
		{
			name: `uses all variables deeply`,
			steps: []portedStep{
				{
					query: `
      query Foo($a: String, $b: String, $c: String) {
        field(a: $a) {
          field(b: $b) {
            field(c: $c)
          }
        }
      }
    `,
				},
			},
		},
		{
			name: `uses all variables deeply in inline fragments`,
			steps: []portedStep{
				{
					query: `
      query Foo($a: String, $b: String, $c: String) {
        ... on Type {
          field(a: $a) {
            field(b: $b) {
              ... on Type {
                field(c: $c)
              }
            }
          }
        }
      }
    `,
				},
			},
		},
		{
			name: `uses all variables in fragments`,
			steps: []portedStep{
				{
					query: `
      query Foo($a: String, $b: String, $c: String) {
        ...FragA
      }
      fragment FragA on Type {
        field(a: $a) {
          ...FragB
        }
      }
      fragment FragB on Type {
        field(b: $b) {
          ...FragC
        }
      }
      fragment FragC on Type {
        field(c: $c)
      }
    `,
				},
			},
		},
		{
			name: `variable used by fragment in multiple operations`,
			steps: []portedStep{
				{
					query: `
      query Foo($a: String) {
        ...FragA
      }
      query Bar($b: String) {
        ...FragB
      }
      fragment FragA on Type {
        field(a: $a)
      }
      fragment FragB on Type {
        field(b: $b)
      }
    `,
				},
			},
		},
		{
			name: `variable used by recursive fragment`,
			steps: []portedStep{
				{
					query: `
      query Foo($a: String) {
        ...FragA
      }
      fragment FragA on Type {
        field(a: $a) {
          ...FragA
        }
      }
    `,
				},
			},
		},
		{
			name: `variable not used`,
			steps: []portedStep{
				{
					query: `
      query ($a: String, $b: String, $c: String) {
        field(a: $a, b: $b)
      }
    `,
					want: []want{
						{At: []at{{2, 38}}},
					},
				},
			},
		},
		{
			name: `multiple variables not used`,
			steps: []portedStep{
				{
					query: `
      query Foo($a: String, $b: String, $c: String) {
        field(b: $b)
      }
    `,
					want: []want{
						{At: []at{{2, 17}}},
						{At: []at{{2, 41}}},
					},
				},
			},
		},
		{
			name: `variable not used in fragments`,
			steps: []portedStep{
				{
					query: `
      query Foo($a: String, $b: String, $c: String) {
        ...FragA
      }
      fragment FragA on Type {
        field(a: $a) {
          ...FragB
        }
      }
      fragment FragB on Type {
        field(b: $b) {
          ...FragC
        }
      }
      fragment FragC on Type {
        field
      }
    `,
					want: []want{
						{At: []at{{2, 41}}},
					},
				},
			},
		},
		{
			name: `multiple variables not used in fragments`,
			steps: []portedStep{
				{
					query: `
      query Foo($a: String, $b: String, $c: String) {
        ...FragA
      }
      fragment FragA on Type {
        field {
          ...FragB
        }
      }
      fragment FragB on Type {
        field(b: $b) {
          ...FragC
        }
      }
      fragment FragC on Type {
        field
      }
    `,
					want: []want{
						{At: []at{{2, 17}}},
						{At: []at{{2, 41}}},
					},
				},
			},
		},
		{
			name: `variable not used by unreferenced fragment`,
			steps: []portedStep{
				{
					query: `
      query Foo($b: String) {
        ...FragA
      }
      fragment FragA on Type {
        field(a: $a)
      }
      fragment FragB on Type {
        field(b: $b)
      }
    `,
					want: []want{
						{At: []at{{2, 17}}},
					},
				},
			},
		},
		{
			name: `variable not used by fragment used by other operation`,
			steps: []portedStep{
				{
					query: `
      query Foo($b: String) {
        ...FragA
      }
      query Bar($a: String) {
        ...FragB
      }
      fragment FragA on Type {
        field(a: $a)
      }
      fragment FragB on Type {
        field(b: $b)
      }
    `,
					want: []want{
						{At: []at{{2, 17}}},
						{At: []at{{5, 17}}},
					},
				},
			},
		},
		{
			name: `fragment defined arguments are not unused variables`,
			steps: []portedStep{
				{
					query: `
      query Foo {
        ...FragA
      }
      fragment FragA($a: String) on Type {
        field1(a: $a)
      }
    `,
				},
			},
		},
		{
			name: `defined variables used as fragment arguments are not unused variables`,
			steps: []portedStep{
				{
					query: `
      query Foo($b: String) {
        ...FragA(a: $b)
      }
      fragment FragA($a: String) on Type {
        field1(a: $a)
      }
    `,
				},
			},
		},
		{
			name: `unused fragment variables are reported`,
			steps: []portedStep{
				{
					query: `
      query Foo {
        ...FragA(a: "value")
      }
      fragment FragA($a: String) on Type {
        field1
      }
    `,
					want: []want{
						{At: []at{{5, 22}}},
					},
				},
			},
		},
	})
}
