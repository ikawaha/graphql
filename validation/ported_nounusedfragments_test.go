package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/NoUnusedFragmentsRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_NoUnusedFragments(t *testing.T) {
	runPorted(t, validation.NoUnusedFragmentsRule, []portedCase{
		{
			name: `all fragment names are used`,
			steps: []portedStep{
				{
					query: `
      {
        human(id: 4) {
          ...HumanFields1
          ... on Human {
            ...HumanFields2
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
			name: `all fragment names are used by multiple operations`,
			steps: []portedStep{
				{
					query: `
      query Foo {
        human(id: 4) {
          ...HumanFields1
        }
      }
      query Bar {
        human(id: 4) {
          ...HumanFields2
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
			name: `contains unknown fragments`,
			steps: []portedStep{
				{
					query: `
      query Foo {
        human(id: 4) {
          ...HumanFields1
        }
      }
      query Bar {
        human(id: 4) {
          ...HumanFields2
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
      fragment Unused1 on Human {
        name
      }
      fragment Unused2 on Human {
        name
      }
    `,
					want: []want{
						{At: []at{{22, 7}}},
						{At: []at{{25, 7}}},
					},
				},
			},
		},
		{
			name: `contains unknown fragments with ref cycle`,
			steps: []portedStep{
				{
					query: `
      query Foo {
        human(id: 4) {
          ...HumanFields1
        }
      }
      query Bar {
        human(id: 4) {
          ...HumanFields2
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
      fragment Unused1 on Human {
        name
        ...Unused2
      }
      fragment Unused2 on Human {
        name
        ...Unused1
      }
    `,
					want: []want{
						{At: []at{{22, 7}}},
						{At: []at{{26, 7}}},
					},
				},
			},
		},
		{
			name: `contains unknown and undef fragments`,
			steps: []portedStep{
				{
					query: `
      query Foo {
        human(id: 4) {
          ...bar
        }
      }
      fragment foo on Human {
        name
      }
    `,
					want: []want{
						{At: []at{{7, 7}}},
					},
				},
			},
		},
	})
}
