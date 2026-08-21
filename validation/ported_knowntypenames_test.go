package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/KnownTypeNamesRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_KnownTypeNames(t *testing.T) {
	runPorted(t, validation.KnownTypeNamesRule, []portedCase{
		{
			name: `known type names are valid`,
			steps: []portedStep{
				{
					query: `
      query Foo(
        $var: String
        $required: [Int!]!
        $introspectionType: __EnumValue
      ) {
        user(id: 4) {
          pets { ... on Pet { name }, ...PetFields, ... { name } }
        }
      }

      fragment PetFields on Pet {
        name
      }
    `,
				},
			},
		},
		{
			name: `unknown type names are invalid`,
			steps: []portedStep{
				{
					query: `
      query Foo($var: [JumbledUpLetters!]!) {
        user(id: 4) {
          name
          pets { ... on Badger { name }, ...PetFields }
        }
      }
      fragment PetFields on Peat {
        name
      }
    `,
					want: []want{
						{At: []at{{2, 24}}},
						{At: []at{{5, 25}}},
						{At: []at{{8, 29}}},
					},
				},
			},
		},
		{
			name: `unknown type names are invalid (no suggestions)`,
			steps: []portedStep{
				{
					query: `
      query Foo($var: [JumbledUpLetters!]!) {
        user(id: 4) {
          name
          pets { ... on Badger { name }, ...PetFields }
        }
      }
      fragment PetFields on Peat {
        name
      }
    `,
					want: []want{
						{At: []at{{2, 24}}},
						{At: []at{{5, 25}}},
						{At: []at{{8, 29}}},
					},
				},
			},
		},
		{
			name: `use standard types`,
			steps: []portedStep{
				{
					query: `
        type Query {
          string: String
          int: Int
          float: Float
          boolean: Boolean
          id: ID
          introspectionType: __EnumValue
        }
      `,
					sdl: true,
				},
			},
		},
		{
			name: `reference types defined inside the same document`,
			steps: []portedStep{
				{
					query: `
        union SomeUnion = SomeObject | AnotherObject

        type SomeObject implements SomeInterface {
          someScalar(arg: SomeInputObject): SomeScalar
        }

        type AnotherObject {
          foo(arg: SomeInputObject): String
        }

        type SomeInterface {
          someScalar(arg: SomeInputObject): SomeScalar
        }

        input SomeInputObject {
          someScalar: SomeScalar
        }

        scalar SomeScalar

        type RootQuery {
          someInterface: SomeInterface
          someUnion: SomeUnion
          someScalar: SomeScalar
          someObject: SomeObject
        }

        schema {
          query: RootQuery
        }
      `,
					sdl: true,
				},
			},
		},
		{
			name: `unknown type references`,
			steps: []portedStep{
				{
					query: `
        type A
        type B

        type SomeObject implements C {
          e(d: D): E
        }

        union SomeUnion = F | G

        interface SomeInterface {
          i(h: H): I
        }

        input SomeInput {
          j: J
        }

        directive @SomeDirective(k: K) on QUERY

        schema {
          query: L
          mutation: M
          subscription: N
        }
      `,
					sdl: true,
					want: []want{
						{At: []at{{5, 36}}},
						{At: []at{{6, 16}}},
						{At: []at{{6, 20}}},
						{At: []at{{9, 27}}},
						{At: []at{{9, 31}}},
						{At: []at{{12, 16}}},
						{At: []at{{12, 20}}},
						{At: []at{{16, 14}}},
						{At: []at{{19, 37}}},
						{At: []at{{22, 18}}},
						{At: []at{{23, 21}}},
						{At: []at{{24, 25}}},
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
        directive @Foo on QUERY

        type Query {
          foo: Foo
        }
      `,
					sdl: true,
					want: []want{
						{At: []at{{7, 16}}},
					},
				},
			},
		},
	})
}

// Not ported, because each of these is written in a way this could not
// follow:
//   - references to standard scalars that are missing in schema: a document that is not written out
//   - reference standard types inside extension document: a document that is not written out
//   - reference types inside extension document: a document that is not written out
//   - unknown type references inside extension document: a document that is not written out
