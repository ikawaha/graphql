package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/PossibleFragmentSpreadsRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_PossibleFragmentSpreads(t *testing.T) {
	runPorted(t, validation.PossibleFragmentSpreadsRule, []portedCase{
		{
			name: `of the same object`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment objectWithinObject on Dog { ...dogFragment }
      fragment dogFragment on Dog { barkVolume }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `of the same object with inline fragment`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment objectWithinObjectAnon on Dog { ... on Dog { barkVolume } }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `object into an implemented interface`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment objectWithinInterface on Pet { ...dogFragment }
      fragment dogFragment on Dog { barkVolume }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `object into containing union`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment objectWithinUnion on CatOrDog { ...dogFragment }
      fragment dogFragment on Dog { barkVolume }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `union into contained object`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment unionWithinObject on Dog { ...catOrDogFragment }
      fragment catOrDogFragment on CatOrDog { __typename }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `union into overlapping interface`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment unionWithinInterface on Pet { ...catOrDogFragment }
      fragment catOrDogFragment on CatOrDog { __typename }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `union into overlapping union`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment unionWithinUnion on DogOrHuman { ...catOrDogFragment }
      fragment catOrDogFragment on CatOrDog { __typename }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `interface into implemented object`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment interfaceWithinObject on Dog { ...petFragment }
      fragment petFragment on Pet { name }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `interface into overlapping interface`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment interfaceWithinInterface on Pet { ...beingFragment }
      fragment beingFragment on Being { name }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `interface into overlapping interface in inline fragment`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment interfaceWithinInterface on Pet { ... on Being { name } }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `interface into overlapping union`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment interfaceWithinUnion on CatOrDog { ...petFragment }
      fragment petFragment on Pet { name }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `ignores incorrect type (caught by FragmentsOnCompositeTypesRule)`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment petFragment on Pet { ...badInADifferentWay }
      fragment badInADifferentWay on String { name }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `ignores unknown fragments (caught by KnownFragmentNamesRule)`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment petFragment on Pet { ...UnknownFragment }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `different object into object`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment invalidObjectWithinObject on Cat { ...dogFragment }
      fragment dogFragment on Dog { barkVolume }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{2, 51}}},
					},
				},
			},
		},
		{
			name: `different object into object in inline fragment`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment invalidObjectWithinObjectAnon on Cat {
        ... on Dog { barkVolume }
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
			name: `object into not implementing interface`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment invalidObjectWithinInterface on Pet { ...humanFragment }
      fragment humanFragment on Human { pets { name } }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{2, 54}}},
					},
				},
			},
		},
		{
			name: `object into not containing union`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment invalidObjectWithinUnion on CatOrDog { ...humanFragment }
      fragment humanFragment on Human { pets { name } }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{2, 55}}},
					},
				},
			},
		},
		{
			name: `union into not contained object`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment invalidUnionWithinObject on Human { ...catOrDogFragment }
      fragment catOrDogFragment on CatOrDog { __typename }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{2, 52}}},
					},
				},
			},
		},
		{
			name: `union into non overlapping interface`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment invalidUnionWithinInterface on Pet { ...humanOrAlienFragment }
      fragment humanOrAlienFragment on HumanOrAlien { __typename }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{2, 53}}},
					},
				},
			},
		},
		{
			name: `union into non overlapping union`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment invalidUnionWithinUnion on CatOrDog { ...humanOrAlienFragment }
      fragment humanOrAlienFragment on HumanOrAlien { __typename }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{2, 54}}},
					},
				},
			},
		},
		{
			name: `interface into non implementing object`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment invalidInterfaceWithinObject on Cat { ...intelligentFragment }
      fragment intelligentFragment on Intelligent { iq }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{2, 54}}},
					},
				},
			},
		},
		{
			name: `interface into non overlapping interface`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment invalidInterfaceWithinInterface on Pet {
        ...intelligentFragment
      }
      fragment intelligentFragment on Intelligent { iq }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 9}}},
					},
				},
			},
		},
		{
			name: `interface into non overlapping interface in inline fragment`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment invalidInterfaceWithinInterfaceAnon on Pet {
        ...on Intelligent { iq }
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
			name: `interface into non overlapping union`,
			ownSchema: `
  interface Being {
    name: String
  }

  interface Pet implements Being {
    name: String
  }

  type Dog implements Being & Pet {
    name: String
    barkVolume: Int
  }

  type Cat implements Being & Pet {
    name: String
    meowVolume: Int
  }

  union CatOrDog = Cat | Dog

  interface Intelligent {
    iq: Int
  }

  type Human implements Being & Intelligent {
    name: String
    pets: [Pet]
    iq: Int
  }

  type Alien implements Being & Intelligent {
    name: String
    iq: Int
  }

  union DogOrHuman = Dog | Human

  union HumanOrAlien = Human | Alien

  type Query {
    catOrDog: CatOrDog
    dogOrHuman: DogOrHuman
    humanOrAlien: HumanOrAlien
  }`,
			steps: []portedStep{
				{
					query: `
      fragment invalidInterfaceWithinUnion on HumanOrAlien { ...petFragment }
      fragment petFragment on Pet { name }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{2, 62}}},
					},
				},
			},
		},
	})
}
