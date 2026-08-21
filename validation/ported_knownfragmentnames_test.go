package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/KnownFragmentNamesRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_KnownFragmentNames(t *testing.T) {
	runPorted(t, validation.KnownFragmentNamesRule, []portedCase{
		{
			name: `known fragment names are valid`,
			steps: []portedStep{
				{
					query: `
      {
        human(id: 4) {
          ...HumanFields1
          ... on Human {
            ...HumanFields2
          }
          ... {
            name
          }
        }
      }
      fragment HumanFields1 on Human {
        name
        ...HumanFields3
      }
      fragment HumanFields2 on Human {
        name
      }
      fragment HumanFields3 on Human {
        name
      }
    `,
				},
			},
		},
		{
			name: `unknown fragment names are invalid`,
			steps: []portedStep{
				{
					query: `
      {
        human(id: 4) {
          ...UnknownFragment1
          ... on Human {
            ...UnknownFragment2
          }
        }
      }
      fragment HumanFields on Human {
        name
        ...UnknownFragment3
      }
    `,
					want: []want{
						{At: []at{{4, 14}}},
						{At: []at{{6, 16}}},
						{At: []at{{12, 12}}},
					},
				},
			},
		},
	})
}
