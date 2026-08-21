package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/UniqueEnumValueNamesRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_UniqueEnumValueNames(t *testing.T) {
	runPorted(t, validation.UniqueEnumValueNamesRule, []portedCase{
		{
			name: `no values`,
			steps: []portedStep{
				{
					query: `
      enum SomeEnum
    `,
					sdl: true,
				},
			},
		},
		{
			name: `one value`,
			steps: []portedStep{
				{
					query: `
      enum SomeEnum {
        FOO
      }
    `,
					sdl: true,
				},
			},
		},
		{
			name: `multiple values`,
			steps: []portedStep{
				{
					query: `
      enum SomeEnum {
        FOO
        BAR
      }
    `,
					sdl: true,
				},
			},
		},
		{
			name: `duplicate values inside the same enum definition`,
			steps: []portedStep{
				{
					query: `
      enum SomeEnum {
        FOO
        BAR
        FOO
      }
    `,
					sdl: true,
					want: []want{
						{At: []at{{3, 9}, {5, 9}}},
					},
				},
			},
		},
		{
			name: `extend enum with new value`,
			steps: []portedStep{
				{
					query: `
      enum SomeEnum {
        FOO
      }
      extend enum SomeEnum {
        BAR
      }
      extend enum SomeEnum {
        BAZ
      }
    `,
					sdl: true,
				},
			},
		},
		{
			name: `extend enum with duplicate value`,
			steps: []portedStep{
				{
					query: `
      extend enum SomeEnum {
        FOO
      }
      enum SomeEnum {
        FOO
      }
    `,
					sdl: true,
					want: []want{
						{At: []at{{3, 9}, {6, 9}}},
					},
				},
			},
		},
		{
			name: `duplicate value inside extension`,
			steps: []portedStep{
				{
					query: `
      enum SomeEnum
      extend enum SomeEnum {
        FOO
        BAR
        FOO
      }
    `,
					sdl: true,
					want: []want{
						{At: []at{{4, 9}, {6, 9}}},
					},
				},
			},
		},
		{
			name: `duplicate value inside different extensions`,
			steps: []portedStep{
				{
					query: `
      enum SomeEnum
      extend enum SomeEnum {
        FOO
      }
      extend enum SomeEnum {
        FOO
      }
    `,
					sdl: true,
					want: []want{
						{At: []at{{4, 9}, {7, 9}}},
					},
				},
			},
		},
	})
}

// Not ported, because each of these is written in a way this could not
// follow:
//   - adding new value to the type inside existing schema: a document that is not written out
//   - adding conflicting value to existing schema twice: a document that is not written out
//   - adding enum values to existing schema twice: a document that is not written out
