package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/NoFragmentCyclesRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_NoFragmentCycles(t *testing.T) {
	runPorted(t, validation.NoFragmentCyclesRule, []portedCase{
		{
			name: `single reference is valid`,
			steps: []portedStep{
				{
					query: `
      fragment fragA on Dog { ...fragB }
      fragment fragB on Dog { name }
    `,
				},
			},
		},
		{
			name: `spreading twice is not circular`,
			steps: []portedStep{
				{
					query: `
      fragment fragA on Dog { ...fragB, ...fragB }
      fragment fragB on Dog { name }
    `,
				},
			},
		},
		{
			name: `spreading twice indirectly is not circular`,
			steps: []portedStep{
				{
					query: `
      fragment fragA on Dog { ...fragB, ...fragC }
      fragment fragB on Dog { ...fragC }
      fragment fragC on Dog { name }
    `,
				},
			},
		},
		{
			name: `double spread within abstract types`,
			steps: []portedStep{
				{
					query: `
      fragment nameFragment on Pet {
        ... on Dog { name }
        ... on Cat { name }
      }

      fragment spreadsInAnon on Pet {
        ... on Dog { ...nameFragment }
        ... on Cat { ...nameFragment }
      }
    `,
				},
			},
		},
		{
			name: `does not false positive on unknown fragment`,
			steps: []portedStep{
				{
					query: `
      fragment nameFragment on Pet {
        ...UnknownFragment
      }
    `,
				},
			},
		},
		{
			name: `spreading recursively within field fails`,
			steps: []portedStep{
				{
					query: `
      fragment fragA on Human { relatives { ...fragA } },
    `,
					want: []want{
						{At: []at{{2, 45}}},
					},
				},
			},
		},
		{
			name: `no spreading itself directly`,
			steps: []portedStep{
				{
					query: `
      fragment fragA on Dog { ...fragA }
    `,
					want: []want{
						{At: []at{{2, 31}}},
					},
				},
			},
		},
		{
			name: `no spreading itself directly within inline fragment`,
			steps: []portedStep{
				{
					query: `
      fragment fragA on Pet {
        ... on Dog {
          ...fragA
        }
      }
    `,
					want: []want{
						{At: []at{{4, 11}}},
					},
				},
			},
		},
		{
			name: `no spreading itself indirectly`,
			steps: []portedStep{
				{
					query: `
      fragment fragA on Dog { ...fragB }
      fragment fragB on Dog { ...fragA }
    `,
					want: []want{
						{At: []at{{2, 31}, {3, 31}}},
					},
				},
			},
		},
		{
			name: `no spreading itself indirectly reports opposite order`,
			steps: []portedStep{
				{
					query: `
      fragment fragB on Dog { ...fragA }
      fragment fragA on Dog { ...fragB }
    `,
					want: []want{
						{At: []at{{2, 31}, {3, 31}}},
					},
				},
			},
		},
		{
			name: `no spreading itself indirectly within inline fragment`,
			steps: []portedStep{
				{
					query: `
      fragment fragA on Pet {
        ... on Dog {
          ...fragB
        }
      }
      fragment fragB on Pet {
        ... on Dog {
          ...fragA
        }
      }
    `,
					want: []want{
						{At: []at{{4, 11}, {9, 11}}},
					},
				},
			},
		},
		{
			name: `no spreading itself deeply`,
			steps: []portedStep{
				{
					query: `
      fragment fragA on Dog { ...fragB }
      fragment fragB on Dog { ...fragC }
      fragment fragC on Dog { ...fragO }
      fragment fragX on Dog { ...fragY }
      fragment fragY on Dog { ...fragZ }
      fragment fragZ on Dog { ...fragO }
      fragment fragO on Dog { ...fragP }
      fragment fragP on Dog { ...fragA, ...fragX }
    `,
					want: []want{
						{At: []at{{2, 31}, {3, 31}, {4, 31}, {8, 31}, {9, 31}}},
						{At: []at{{8, 31}, {9, 41}, {5, 31}, {6, 31}, {7, 31}}},
					},
				},
			},
		},
		{
			name: `no spreading itself deeply two paths`,
			steps: []portedStep{
				{
					query: `
      fragment fragA on Dog { ...fragB, ...fragC }
      fragment fragB on Dog { ...fragA }
      fragment fragC on Dog { ...fragA }
    `,
					want: []want{
						{At: []at{{2, 31}, {3, 31}}},
						{At: []at{{2, 41}, {4, 31}}},
					},
				},
			},
		},
		{
			name: `no spreading itself deeply two paths -- alt traverse order`,
			steps: []portedStep{
				{
					query: `
      fragment fragA on Dog { ...fragC }
      fragment fragB on Dog { ...fragC }
      fragment fragC on Dog { ...fragA, ...fragB }
    `,
					want: []want{
						{At: []at{{2, 31}, {4, 31}}},
						{At: []at{{4, 41}, {3, 31}}},
					},
				},
			},
		},
		{
			name: `no spreading itself deeply and immediately`,
			steps: []portedStep{
				{
					query: `
      fragment fragA on Dog { ...fragB }
      fragment fragB on Dog { ...fragB, ...fragC }
      fragment fragC on Dog { ...fragA, ...fragB }
    `,
					want: []want{
						{At: []at{{3, 31}}},
						{At: []at{{2, 31}, {3, 41}, {4, 31}}},
						{At: []at{{3, 41}, {4, 41}}},
					},
				},
			},
		},
	})
}
