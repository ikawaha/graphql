package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/PossibleTypeExtensionsRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_PossibleTypeExtensions(t *testing.T) {
	runPorted(t, validation.PossibleTypeExtensionsRule, []portedCase{
		{
			name: `no extensions`,
			steps: []portedStep{
				{
					query: `
      scalar FooScalar
      type FooObject
      interface FooInterface
      union FooUnion
      enum FooEnum
      input FooInputObject
    `,
					sdl: true,
				},
			},
		},
		{
			name: `one extension per type`,
			steps: []portedStep{
				{
					query: `
      scalar FooScalar
      type FooObject
      interface FooInterface
      union FooUnion
      enum FooEnum
      input FooInputObject

      extend scalar FooScalar @dummy
      extend type FooObject @dummy
      extend interface FooInterface @dummy
      extend union FooUnion @dummy
      extend enum FooEnum @dummy
      extend input FooInputObject @dummy
    `,
					sdl: true,
				},
			},
		},
		{
			name: `many extensions per type`,
			steps: []portedStep{
				{
					query: `
      scalar FooScalar
      type FooObject
      interface FooInterface
      union FooUnion
      enum FooEnum
      input FooInputObject

      extend scalar FooScalar @dummy
      extend type FooObject @dummy
      extend interface FooInterface @dummy
      extend union FooUnion @dummy
      extend enum FooEnum @dummy
      extend input FooInputObject @dummy

      extend scalar FooScalar @dummy
      extend type FooObject @dummy
      extend interface FooInterface @dummy
      extend union FooUnion @dummy
      extend enum FooEnum @dummy
      extend input FooInputObject @dummy
    `,
					sdl: true,
				},
			},
		},
		{
			name: `extending unknown type`,
			steps: []portedStep{
				{
					query: `
      type Known

      extend scalar Unknown @dummy
      extend type Unknown @dummy
      extend interface Unknown @dummy
      extend union Unknown @dummy
      extend enum Unknown @dummy
      extend input Unknown @dummy
    `,
					sdl: true,
					want: []want{
						{At: []at{{4, 21}}},
						{At: []at{{5, 19}}},
						{At: []at{{6, 24}}},
						{At: []at{{7, 20}}},
						{At: []at{{8, 19}}},
						{At: []at{{9, 20}}},
					},
				},
			},
		},
		{
			name: `does not consider non-type definitions`,
			steps: []portedStep{
				{
					query: `
      query Foo { __typename }
      fragment Foo on Query { __typename }
      directive @Foo on SCHEMA

      extend scalar Foo @dummy
      extend type Foo @dummy
      extend interface Foo @dummy
      extend union Foo @dummy
      extend enum Foo @dummy
      extend input Foo @dummy
    `,
					sdl: true,
					want: []want{
						{At: []at{{6, 21}}},
						{At: []at{{7, 19}}},
						{At: []at{{8, 24}}},
						{At: []at{{9, 20}}},
						{At: []at{{10, 19}}},
						{At: []at{{11, 20}}},
					},
				},
			},
		},
		{
			name: `extending with different kinds`,
			steps: []portedStep{
				{
					query: `
      scalar FooScalar
      type FooObject
      interface FooInterface
      union FooUnion
      enum FooEnum
      input FooInputObject

      extend type FooScalar @dummy
      extend interface FooObject @dummy
      extend union FooInterface @dummy
      extend enum FooUnion @dummy
      extend input FooEnum @dummy
      extend scalar FooInputObject @dummy
    `,
					sdl: true,
					want: []want{
						{At: []at{{2, 7}, {9, 7}}},
						{At: []at{{3, 7}, {10, 7}}},
						{At: []at{{4, 7}, {11, 7}}},
						{At: []at{{5, 7}, {12, 7}}},
						{At: []at{{6, 7}, {13, 7}}},
						{At: []at{{7, 7}, {14, 7}}},
					},
				},
			},
		},
	})
}

// Not ported, because each of these is written in a way this could not
// follow:
//   - extending types within existing schema: a document that is not written out
//   - extending unknown types within existing schema: a document that is not written out
//   - extending types with different kinds within existing schema: a document that is not written out
