package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/FieldsOnCorrectTypeRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_FieldsOnCorrectType(t *testing.T) {
	runPorted(t, validation.FieldsOnCorrectTypeRule, []portedCase{
		{
			name: `Object field selection`,
			ownSchema: `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
    nickname: String
    barkVolume: Int
  }

  type Cat implements Pet {
    name: String
    nickname: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  type Human {
    name: String
    pets: [Pet]
  }

  type Query {
    human: Human
  }`,
			steps: []portedStep{
				{
					query: `
      fragment objectFieldSelection on Dog {
        __typename
        name
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Aliased object field selection`,
			ownSchema: `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
    nickname: String
    barkVolume: Int
  }

  type Cat implements Pet {
    name: String
    nickname: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  type Human {
    name: String
    pets: [Pet]
  }

  type Query {
    human: Human
  }`,
			steps: []portedStep{
				{
					query: `
      fragment aliasedObjectFieldSelection on Dog {
        tn : __typename
        otherName : name
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Interface field selection`,
			ownSchema: `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
    nickname: String
    barkVolume: Int
  }

  type Cat implements Pet {
    name: String
    nickname: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  type Human {
    name: String
    pets: [Pet]
  }

  type Query {
    human: Human
  }`,
			steps: []portedStep{
				{
					query: `
      fragment interfaceFieldSelection on Pet {
        __typename
        name
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Aliased interface field selection`,
			ownSchema: `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
    nickname: String
    barkVolume: Int
  }

  type Cat implements Pet {
    name: String
    nickname: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  type Human {
    name: String
    pets: [Pet]
  }

  type Query {
    human: Human
  }`,
			steps: []portedStep{
				{
					query: `
      fragment interfaceFieldSelection on Pet {
        otherName : name
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Lying alias selection`,
			ownSchema: `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
    nickname: String
    barkVolume: Int
  }

  type Cat implements Pet {
    name: String
    nickname: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  type Human {
    name: String
    pets: [Pet]
  }

  type Query {
    human: Human
  }`,
			steps: []portedStep{
				{
					query: `
      fragment lyingAliasSelection on Dog {
        name : nickname
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Ignores fields on unknown type`,
			ownSchema: `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
    nickname: String
    barkVolume: Int
  }

  type Cat implements Pet {
    name: String
    nickname: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  type Human {
    name: String
    pets: [Pet]
  }

  type Query {
    human: Human
  }`,
			steps: []portedStep{
				{
					query: `
      fragment unknownSelection on UnknownType {
        unknownField
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `reports errors when type is known again`,
			ownSchema: `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
    nickname: String
    barkVolume: Int
  }

  type Cat implements Pet {
    name: String
    nickname: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  type Human {
    name: String
    pets: [Pet]
  }

  type Query {
    human: Human
  }`,
			steps: []portedStep{
				{
					query: `
      fragment typeKnownAgain on Pet {
        unknown_pet_field {
          ... on Cat {
            unknown_cat_field
          }
        }
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 9}}},
						{At: []at{{5, 13}}},
					},
				},
			},
		},
		{
			name: `Field not defined on fragment`,
			ownSchema: `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
    nickname: String
    barkVolume: Int
  }

  type Cat implements Pet {
    name: String
    nickname: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  type Human {
    name: String
    pets: [Pet]
  }

  type Query {
    human: Human
  }`,
			steps: []portedStep{
				{
					query: `
      fragment fieldNotDefined on Dog {
        meowVolume
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 9}}},
					},
				},
			},
		},
		{
			name: `Field not defined on fragment (no suggestions)`,
			ownSchema: `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
    nickname: String
    barkVolume: Int
  }

  type Cat implements Pet {
    name: String
    nickname: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  type Human {
    name: String
    pets: [Pet]
  }

  type Query {
    human: Human
  }`,
			steps: []portedStep{
				{
					query: `
      fragment fieldNotDefined on Dog {
        meowVolume
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 9}}},
					},
				},
			},
		},
		{
			name: `Ignores deeply unknown field`,
			ownSchema: `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
    nickname: String
    barkVolume: Int
  }

  type Cat implements Pet {
    name: String
    nickname: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  type Human {
    name: String
    pets: [Pet]
  }

  type Query {
    human: Human
  }`,
			steps: []portedStep{
				{
					query: `
      fragment deepFieldNotDefined on Dog {
        unknown_field {
          deeper_unknown_field
        }
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 9}}},
					},
				},
			},
		},
		{
			name: `Sub-field not defined`,
			ownSchema: `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
    nickname: String
    barkVolume: Int
  }

  type Cat implements Pet {
    name: String
    nickname: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  type Human {
    name: String
    pets: [Pet]
  }

  type Query {
    human: Human
  }`,
			steps: []portedStep{
				{
					query: `
      fragment subFieldNotDefined on Human {
        pets {
          unknown_field
        }
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 11}}},
					},
				},
			},
		},
		{
			name: `Field not defined on inline fragment`,
			ownSchema: `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
    nickname: String
    barkVolume: Int
  }

  type Cat implements Pet {
    name: String
    nickname: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  type Human {
    name: String
    pets: [Pet]
  }

  type Query {
    human: Human
  }`,
			steps: []portedStep{
				{
					query: `
      fragment fieldNotDefined on Pet {
        ... on Dog {
          meowVolume
        }
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 11}}},
					},
				},
			},
		},
		{
			name: `Aliased field target not defined`,
			ownSchema: `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
    nickname: String
    barkVolume: Int
  }

  type Cat implements Pet {
    name: String
    nickname: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  type Human {
    name: String
    pets: [Pet]
  }

  type Query {
    human: Human
  }`,
			steps: []portedStep{
				{
					query: `
      fragment aliasedFieldTargetNotDefined on Dog {
        volume : mooVolume
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 9}}},
					},
				},
			},
		},
		{
			name: `Aliased lying field target not defined`,
			ownSchema: `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
    nickname: String
    barkVolume: Int
  }

  type Cat implements Pet {
    name: String
    nickname: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  type Human {
    name: String
    pets: [Pet]
  }

  type Query {
    human: Human
  }`,
			steps: []portedStep{
				{
					query: `
      fragment aliasedLyingFieldTargetNotDefined on Dog {
        barkVolume : kawVolume
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 9}}},
					},
				},
			},
		},
		{
			name: `Not defined on interface`,
			ownSchema: `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
    nickname: String
    barkVolume: Int
  }

  type Cat implements Pet {
    name: String
    nickname: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  type Human {
    name: String
    pets: [Pet]
  }

  type Query {
    human: Human
  }`,
			steps: []portedStep{
				{
					query: `
      fragment notDefinedOnInterface on Pet {
        tailLength
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 9}}},
					},
				},
			},
		},
		{
			name: `Defined on implementors but not on interface`,
			ownSchema: `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
    nickname: String
    barkVolume: Int
  }

  type Cat implements Pet {
    name: String
    nickname: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  type Human {
    name: String
    pets: [Pet]
  }

  type Query {
    human: Human
  }`,
			steps: []portedStep{
				{
					query: `
      fragment definedOnImplementorsButNotInterface on Pet {
        nickname
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 9}}},
					},
				},
			},
		},
		{
			name: `Meta field selection on union`,
			ownSchema: `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
    nickname: String
    barkVolume: Int
  }

  type Cat implements Pet {
    name: String
    nickname: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  type Human {
    name: String
    pets: [Pet]
  }

  type Query {
    human: Human
  }`,
			steps: []portedStep{
				{
					query: `
      fragment directFieldSelectionOnUnion on CatOrDog {
        __typename
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Direct field selection on union`,
			ownSchema: `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
    nickname: String
    barkVolume: Int
  }

  type Cat implements Pet {
    name: String
    nickname: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  type Human {
    name: String
    pets: [Pet]
  }

  type Query {
    human: Human
  }`,
			steps: []portedStep{
				{
					query: `
      fragment directFieldSelectionOnUnion on CatOrDog {
        directField
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 9}}},
					},
				},
			},
		},
		{
			name: `Defined on implementors queried on union`,
			ownSchema: `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
    nickname: String
    barkVolume: Int
  }

  type Cat implements Pet {
    name: String
    nickname: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  type Human {
    name: String
    pets: [Pet]
  }

  type Query {
    human: Human
  }`,
			steps: []portedStep{
				{
					query: `
      fragment definedOnImplementorsQueriedOnUnion on CatOrDog {
        name
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 9}}},
					},
				},
			},
		},
		{
			name: `valid field in inline fragment`,
			ownSchema: `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
    nickname: String
    barkVolume: Int
  }

  type Cat implements Pet {
    name: String
    nickname: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  type Human {
    name: String
    pets: [Pet]
  }

  type Query {
    human: Human
  }`,
			steps: []portedStep{
				{
					query: `
      fragment objectFieldSelection on Pet {
        ... on Dog {
          name
        }
        ... {
          name
        }
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
	})
}

// Not ported, because each of these is written in a way this could not
// follow:
//   - Works with no suggestions: nothing to run
//   - Works with no small numbers of type suggestions: nothing to run
//   - Works with no small numbers of field suggestions: nothing to run
//   - Only shows one set of suggestions at a time, preferring types: nothing to run
//   - Sort type suggestions based on inheritance order: builds more than one schema
//   - Limits lots of type suggestions: nothing to run
//   - Limits lots of field suggestions: nothing to run
