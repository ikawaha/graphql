package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/UniqueArgumentNamesRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_UniqueArgumentNames(t *testing.T) {
	runPorted(t, validation.UniqueArgumentNamesRule, []portedCase{
		{
			name: `no arguments on field`,
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
			name: `no arguments on directive`,
			steps: []portedStep{
				{
					query: `
      {
        field @directive
      }
    `,
				},
			},
		},
		{
			name: `argument on field`,
			steps: []portedStep{
				{
					query: `
      {
        field(arg: "value")
      }
    `,
				},
			},
		},
		{
			name: `argument on directive`,
			steps: []portedStep{
				{
					query: `
      {
        field @directive(arg: "value")
      }
    `,
				},
			},
		},
		{
			name: `same argument on two fields`,
			steps: []portedStep{
				{
					query: `
      {
        one: field(arg: "value")
        two: field(arg: "value")
      }
    `,
				},
			},
		},
		{
			name: `same argument on field and directive`,
			steps: []portedStep{
				{
					query: `
      {
        field(arg: "value") @directive(arg: "value")
      }
    `,
				},
			},
		},
		{
			name: `same argument on two directives`,
			steps: []portedStep{
				{
					query: `
      {
        field @directive1(arg: "value") @directive2(arg: "value")
      }
    `,
				},
			},
		},
		{
			name: `multiple field arguments`,
			steps: []portedStep{
				{
					query: `
      {
        field(arg1: "value", arg2: "value", arg3: "value")
      }
    `,
				},
			},
		},
		{
			name: `multiple directive arguments`,
			steps: []portedStep{
				{
					query: `
      {
        field @directive(arg1: "value", arg2: "value", arg3: "value")
      }
    `,
				},
			},
		},
		{
			name: `duplicate field arguments`,
			steps: []portedStep{
				{
					query: `
      {
        field(arg1: "value", arg1: "value")
      }
    `,
					want: []want{
						{At: []at{{3, 15}, {3, 30}}},
					},
				},
			},
		},
		{
			name: `many duplicate field arguments`,
			steps: []portedStep{
				{
					query: `
      {
        field(arg1: "value", arg1: "value", arg1: "value")
      }
    `,
					want: []want{
						{At: []at{{3, 15}, {3, 30}, {3, 45}}},
					},
				},
			},
		},
		{
			name: `duplicate directive arguments`,
			steps: []portedStep{
				{
					query: `
      {
        field @directive(arg1: "value", arg1: "value")
      }
    `,
					want: []want{
						{At: []at{{3, 26}, {3, 41}}},
					},
				},
			},
		},
		{
			name: `many duplicate directive arguments`,
			steps: []portedStep{
				{
					query: `
      {
        field @directive(arg1: "value", arg1: "value", arg1: "value")
      }
    `,
					want: []want{
						{At: []at{{3, 26}, {3, 41}, {3, 56}}},
					},
				},
			},
		},
	})
}
