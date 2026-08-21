package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/StreamDirectiveOnListFieldRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_StreamDirectiveOnListField(t *testing.T) {
	runPorted(t, validation.StreamDirectiveOnListFieldRule, []portedCase{
		{
			name: `Stream on list field`,
			steps: []portedStep{
				{
					query: `
      fragment objectFieldSelection on Human {
        pets @stream(initialCount: 0) {
          name
        }
      }
    `,
				},
			},
		},
		{
			name: `Stream on non-null list field`,
			steps: []portedStep{
				{
					query: `
      fragment objectFieldSelection on Human {
        relatives @stream(initialCount: 0) {
          name
        }
      }
    `,
				},
			},
		},
		{
			name: `Doesn't validate other directives on list fields`,
			steps: []portedStep{
				{
					query: `
    fragment objectFieldSelection on Human {
      pets @include(if: true) {
        name
      }
    }
    `,
				},
			},
		},
		{
			name: `Doesn't validate other directives on non-list fields`,
			steps: []portedStep{
				{
					query: `
      fragment objectFieldSelection on Human {
        pets {
          name @include(if: true)
        }
      }
    `,
				},
			},
		},
		{
			name: `Doesn't validate misplaced stream directives`,
			steps: []portedStep{
				{
					query: `
      fragment objectFieldSelection on Human {
        ... @stream(initialCount: 0) {
          name
        }
      }
    `,
				},
			},
		},
		{
			name: `reports errors when stream is used on non-list field`,
			steps: []portedStep{
				{
					query: `
      fragment objectFieldSelection on Human {
        name @stream(initialCount: 0)
      }
    `,
					want: []want{
						{At: []at{{3, 14}}},
					},
				},
			},
		},
	})
}
