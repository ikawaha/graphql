package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/ScalarLeafsRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_ScalarLeafs(t *testing.T) {
	runPorted(t, validation.ScalarLeafsRule, []portedCase{
		{
			name: `valid scalar selection`,
			steps: []portedStep{
				{
					query: `
      fragment scalarSelection on Dog {
        barks
      }
    `,
				},
			},
		},
		{
			name: `object type missing selection`,
			steps: []portedStep{
				{
					query: `
      query directQueryOnObjectWithoutSubFields {
        human
      }
    `,
					want: []want{
						{At: []at{{3, 9}}},
					},
				},
			},
		},
		{
			name: `interface type missing selection`,
			steps: []portedStep{
				{
					query: `
      {
        human { pets }
      }
    `,
					want: []want{
						{At: []at{{3, 17}}},
					},
				},
			},
		},
		{
			name: `valid scalar selection with args`,
			steps: []portedStep{
				{
					query: `
      fragment scalarSelectionWithArgs on Dog {
        doesKnowCommand(dogCommand: SIT)
      }
    `,
				},
			},
		},
		{
			name: `scalar selection not allowed on Boolean`,
			steps: []portedStep{
				{
					query: `
      fragment scalarSelectionsNotAllowedOnBoolean on Dog {
        barks { sinceWhen }
      }
    `,
					want: []want{
						{At: []at{{3, 15}}},
					},
				},
			},
		},
		{
			name: `scalar selection not allowed on Enum`,
			steps: []portedStep{
				{
					query: `
      fragment scalarSelectionsNotAllowedOnEnum on Cat {
        furColor { inHexDec }
      }
    `,
					want: []want{
						{At: []at{{3, 18}}},
					},
				},
			},
		},
		{
			name: `scalar selection not allowed with args`,
			steps: []portedStep{
				{
					query: `
      fragment scalarSelectionsNotAllowedWithArgs on Dog {
        doesKnowCommand(dogCommand: SIT) { sinceWhen }
      }
    `,
					want: []want{
						{At: []at{{3, 42}}},
					},
				},
			},
		},
		{
			name: `Scalar selection not allowed with directives`,
			steps: []portedStep{
				{
					query: `
      fragment scalarSelectionsNotAllowedWithDirectives on Dog {
        name @include(if: true) { isAlsoHumanName }
      }
    `,
					want: []want{
						{At: []at{{3, 33}}},
					},
				},
			},
		},
		{
			name: `Scalar selection not allowed with directives and args`,
			steps: []portedStep{
				{
					query: `
      fragment scalarSelectionsNotAllowedWithDirectivesAndArgs on Dog {
        doesKnowCommand(dogCommand: SIT) @include(if: true) { sinceWhen }
      }
    `,
					want: []want{
						{At: []at{{3, 61}}},
					},
				},
			},
		},
	})
}

// Not ported, because each of these is written in a way this could not
// follow:
//   - object type having only one selection: nothing to run
//   - object type having only one selection: nothing to run
