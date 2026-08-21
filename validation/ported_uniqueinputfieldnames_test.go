package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/UniqueInputFieldNamesRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_UniqueInputFieldNames(t *testing.T) {
	runPorted(t, validation.UniqueInputFieldNamesRule, []portedCase{
		{
			name: `input object with fields`,
			steps: []portedStep{
				{
					query: `
      {
        field(arg: { f: true })
      }
    `,
				},
			},
		},
		{
			name: `same input object within two args`,
			steps: []portedStep{
				{
					query: `
      {
        field(arg1: { f: true }, arg2: { f: true })
      }
    `,
				},
			},
		},
		{
			name: `multiple input object fields`,
			steps: []portedStep{
				{
					query: `
      {
        field(arg: { f1: "value", f2: "value", f3: "value" })
      }
    `,
				},
			},
		},
		{
			name: `allows for nested input objects with similar fields`,
			steps: []portedStep{
				{
					query: `
      {
        field(arg: {
          deep: {
            deep: {
              id: 1
            }
            id: 1
          }
          id: 1
        })
      }
    `,
				},
			},
		},
		{
			name: `duplicate input object fields`,
			steps: []portedStep{
				{
					query: `
      {
        field(arg: { f1: "value", f1: "value" })
      }
    `,
					want: []want{
						{At: []at{{3, 22}, {3, 35}}},
					},
				},
			},
		},
		{
			name: `many duplicate input object fields`,
			steps: []portedStep{
				{
					query: `
      {
        field(arg: { f1: "value", f1: "value", f1: "value" })
      }
    `,
					want: []want{
						{At: []at{{3, 22}, {3, 35}}},
						{At: []at{{3, 22}, {3, 48}}},
					},
				},
			},
		},
		{
			name: `nested duplicate input object fields`,
			steps: []portedStep{
				{
					query: `
      {
        field(arg: { f1: {f2: "value", f2: "value" }})
      }
    `,
					want: []want{
						{At: []at{{3, 27}, {3, 40}}},
					},
				},
			},
		},
	})
}
