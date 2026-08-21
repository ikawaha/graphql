package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/UniqueOperationTypesRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_UniqueOperationTypes(t *testing.T) {
	runPorted(t, validation.UniqueOperationTypesRule, []portedCase{
		{
			name: `no schema definition`,
			steps: []portedStep{
				{
					query: `
      type Foo
    `,
					sdl: true,
				},
			},
		},
		{
			name: `schema definition with all types`,
			steps: []portedStep{
				{
					query: `
      type Foo

      schema {
        query: Foo
        mutation: Foo
        subscription: Foo
      }
    `,
					sdl: true,
				},
			},
		},
		{
			name: `schema definition with single extension`,
			steps: []portedStep{
				{
					query: `
      type Foo

      schema { query: Foo }

      extend schema {
        mutation: Foo
        subscription: Foo
      }
    `,
					sdl: true,
				},
			},
		},
		{
			name: `schema definition with separate extensions`,
			steps: []portedStep{
				{
					query: `
      type Foo

      schema { query: Foo }
      extend schema { mutation: Foo }
      extend schema { subscription: Foo }
    `,
					sdl: true,
				},
			},
		},
		{
			name: `extend schema before definition`,
			steps: []portedStep{
				{
					query: `
      type Foo

      extend schema { mutation: Foo }
      extend schema { subscription: Foo }

      schema { query: Foo }
    `,
					sdl: true,
				},
			},
		},
		{
			name: `duplicate operation types inside single schema definition`,
			steps: []portedStep{
				{
					query: `
      type Foo

      schema {
        query: Foo
        mutation: Foo
        subscription: Foo

        query: Foo
        mutation: Foo
        subscription: Foo
      }
    `,
					sdl: true,
					want: []want{
						{At: []at{{5, 9}, {9, 9}}},
						{At: []at{{6, 9}, {10, 9}}},
						{At: []at{{7, 9}, {11, 9}}},
					},
				},
			},
		},
		{
			name: `duplicate operation types inside schema extension`,
			steps: []portedStep{
				{
					query: `
      type Foo

      schema {
        query: Foo
        mutation: Foo
        subscription: Foo
      }

      extend schema {
        query: Foo
        mutation: Foo
        subscription: Foo
      }
    `,
					sdl: true,
					want: []want{
						{At: []at{{5, 9}, {11, 9}}},
						{At: []at{{6, 9}, {12, 9}}},
						{At: []at{{7, 9}, {13, 9}}},
					},
				},
			},
		},
		{
			name: `duplicate operation types inside schema extension twice`,
			steps: []portedStep{
				{
					query: `
      type Foo

      schema {
        query: Foo
        mutation: Foo
        subscription: Foo
      }

      extend schema {
        query: Foo
        mutation: Foo
        subscription: Foo
      }

      extend schema {
        query: Foo
        mutation: Foo
        subscription: Foo
      }
    `,
					sdl: true,
					want: []want{
						{At: []at{{5, 9}, {11, 9}}},
						{At: []at{{6, 9}, {12, 9}}},
						{At: []at{{7, 9}, {13, 9}}},
						{At: []at{{5, 9}, {17, 9}}},
						{At: []at{{6, 9}, {18, 9}}},
						{At: []at{{7, 9}, {19, 9}}},
					},
				},
			},
		},
		{
			name: `duplicate operation types inside second schema extension`,
			steps: []portedStep{
				{
					query: `
      type Foo

      schema {
        query: Foo
      }

      extend schema {
        mutation: Foo
        subscription: Foo
      }

      extend schema {
        query: Foo
        mutation: Foo
        subscription: Foo
      }
    `,
					sdl: true,
					want: []want{
						{At: []at{{5, 9}, {14, 9}}},
						{At: []at{{9, 9}, {15, 9}}},
						{At: []at{{10, 9}, {16, 9}}},
					},
				},
			},
		},
	})
}

// Not ported, because each of these is written in a way this could not
// follow:
//   - define schema inside extension SDL: a document that is not written out
//   - define and extend schema inside extension SDL: a document that is not written out
//   - adding new operation types to existing schema: a document that is not written out
//   - adding conflicting operation types to existing schema: a document that is not written out
//   - adding conflicting operation types to existing schema twice: a document that is not written out
