package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/NoUndefinedVariablesRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_NoUndefinedVariables(t *testing.T) {
	runPorted(t, validation.NoUndefinedVariablesRule, []portedCase{
		{
			name: `all variables defined`,
			steps: []portedStep{
				{
					query: `
      query Foo($a: String, $b: String, $c: String) {
        field(a: $a, b: $b, c: $c)
      }
    `,
				},
			},
		},
		{
			name: `all variables deeply defined`,
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
			name: `all variables deeply in inline fragments defined`,
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
			name: `all variables in fragments deeply defined`,
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
			name: `variable within single fragment defined in multiple operations`,
			steps: []portedStep{
				{
					query: `
      query Foo($a: String) {
        ...FragA
      }
      query Bar($a: String) {
        ...FragA
      }
      fragment FragA on Type {
        field(a: $a)
      }
    `,
				},
			},
		},
		{
			name: `variable within fragments defined in operations`,
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
			name: `variable within recursive fragment defined`,
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
			name: `variable not defined`,
			steps: []portedStep{
				{
					query: `
      query Foo($a: String, $b: String, $c: String) {
        field(a: $a, b: $b, c: $c, d: $d)
      }
    `,
					want: []want{
						{At: []at{{3, 39}, {2, 7}}},
					},
				},
			},
		},
		{
			name: `variable not defined by un-named query`,
			steps: []portedStep{
				{
					query: `
      {
        field(a: $a)
      }
    `,
					want: []want{
						{At: []at{{3, 18}, {2, 7}}},
					},
				},
			},
		},
		{
			name: `multiple variables not defined`,
			steps: []portedStep{
				{
					query: `
      query Foo($b: String) {
        field(a: $a, b: $b, c: $c)
      }
    `,
					want: []want{
						{At: []at{{3, 18}, {2, 7}}},
						{At: []at{{3, 32}, {2, 7}}},
					},
				},
			},
		},
		{
			name: `variable in fragment not defined by un-named query`,
			steps: []portedStep{
				{
					query: `
      {
        ...FragA
      }
      fragment FragA on Type {
        field(a: $a)
      }
    `,
					want: []want{
						{At: []at{{6, 18}, {2, 7}}},
					},
				},
			},
		},
		{
			name: `variable in fragment not defined by operation`,
			steps: []portedStep{
				{
					query: `
      query Foo($a: String, $b: String) {
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
					want: []want{
						{At: []at{{16, 18}, {2, 7}}},
					},
				},
			},
		},
		{
			name: `multiple variables in fragments not defined`,
			steps: []portedStep{
				{
					query: `
      query Foo($b: String) {
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
					want: []want{
						{At: []at{{6, 18}, {2, 7}}},
						{At: []at{{16, 18}, {2, 7}}},
					},
				},
			},
		},
		{
			name: `single variable in fragment not defined by multiple operations`,
			steps: []portedStep{
				{
					query: `
      query Foo($a: String) {
        ...FragAB
      }
      query Bar($a: String) {
        ...FragAB
      }
      fragment FragAB on Type {
        field(a: $a, b: $b)
      }
    `,
					want: []want{
						{At: []at{{9, 25}, {2, 7}}},
						{At: []at{{9, 25}, {5, 7}}},
					},
				},
			},
		},
		{
			name: `variables in fragment not defined by multiple operations`,
			steps: []portedStep{
				{
					query: `
      query Foo($b: String) {
        ...FragAB
      }
      query Bar($a: String) {
        ...FragAB
      }
      fragment FragAB on Type {
        field(a: $a, b: $b)
      }
    `,
					want: []want{
						{At: []at{{9, 18}, {2, 7}}},
						{At: []at{{9, 25}, {5, 7}}},
					},
				},
			},
		},
		{
			name: `variable in fragment used by other operation`,
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
						{At: []at{{9, 18}, {2, 7}}},
						{At: []at{{12, 18}, {5, 7}}},
					},
				},
			},
		},
		{
			name: `multiple undefined variables produce multiple errors`,
			steps: []portedStep{
				{
					query: `
      query Foo($b: String) {
        ...FragAB
      }
      query Bar($a: String) {
        ...FragAB
      }
      fragment FragAB on Type {
        field1(a: $a, b: $b)
        ...FragC
        field3(a: $a, b: $b)
      }
      fragment FragC on Type {
        field2(c: $c)
      }
    `,
					want: []want{
						{At: []at{{9, 19}, {2, 7}}},
						{At: []at{{11, 19}, {2, 7}}},
						{At: []at{{14, 19}, {2, 7}}},
						{At: []at{{9, 26}, {5, 7}}},
						{At: []at{{11, 26}, {5, 7}}},
						{At: []at{{14, 19}, {5, 7}}},
					},
				},
			},
		},
		{
			name: `fragment defined arguments are not undefined variables`,
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
			name: `defined variables used as fragment arguments are not undefined variables`,
			steps: []portedStep{
				{
					query: `
      query Foo($b: String) {
        ...FragA(a: $b)
      }
      fragment FragA($a: String) on Type {
        field1
      }
    `,
				},
			},
		},
		{
			name: `variables used as fragment arguments may be undefined variables`,
			steps: []portedStep{
				{
					query: `
      query Foo {
        ...FragA(a: $a)
      }
      fragment FragA($a: String) on Type {
        field1
      }
    `,
					want: []want{
						{At: []at{{3, 21}, {2, 7}}},
					},
				},
			},
		},
		{
			name: `variables shadowed by parent fragment arguments are still undefined variables`,
			steps: []portedStep{
				{
					query: `
      query Foo {
        ...FragA
      }
      fragment FragA($a: String) on Type {
        ...FragB
      }
      fragment FragB on Type {
        field1(a: $a)
      }
    `,
					want: []want{
						{At: []at{{9, 19}, {2, 7}}},
					},
				},
			},
		},
	})
}
