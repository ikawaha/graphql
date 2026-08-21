package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/UniqueVariableNamesRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_UniqueVariableNames(t *testing.T) {
	runPorted(t, validation.UniqueVariableNamesRule, []portedCase{
		{
			name: `unique variable names`,
			steps: []portedStep{
				{
					query: `
      query A($x: Int, $y: String) { __typename }
      query B($x: String, $y: Int) { __typename }
    `,
				},
			},
		},
		{
			name: `duplicate variable names`,
			steps: []portedStep{
				{
					query: `
      query A($x: Int, $x: Int, $x: String) { __typename }
      query B($x: String, $x: Int) { __typename }
      query C($x: Int, $x: Int) { __typename }
    `,
					want: []want{
						{At: []at{{2, 16}, {2, 25}, {2, 34}}},
						{At: []at{{3, 16}, {3, 28}}},
						{At: []at{{4, 16}, {4, 25}}},
					},
				},
			},
		},
	})
}
