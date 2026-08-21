package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestKnownTypeNames(t *testing.T) {
	s := testSchema(t)
	rule := validation.KnownTypeNamesRule

	t.Run("known type names", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo($var: String, $required: [Int!]!) {
				human(id: 4) {
					pets { ... on Pet { name } }
				}
			}
			fragment PetFields on Pet {
				name
			}
		`)
	})

	t.Run("unknown type names", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query Foo($var: JumbledUpLetters) {
				human(id: 4) {
					name
					pets { ... on Badger { name } }
				}
			}
			fragment PetFields on Peat {
				name
			}
		`,
			want{Message: `Unknown type "JumbledUpLetters"`, At: []at{{1, 17}}},
			want{Message: `Unknown type "Badger"`, At: []at{{4, 17}}},
			// "Peat" is one letter from "Pet", so the message says so.
			want{Message: `Unknown type "Peat". Did you mean "Pet" or "Cat"?`, At: []at{{7, 23}}},
		)
	})

	// SDL leaves the built-in scalars undeclared, so naming one there is not a
	// reference to a missing type.
	t.Run("built-ins are known inside SDL", func(t *testing.T) {
		expectValidSDL(t, nil, rule, `
			type Query {
				id: ID
				name: String
				count: Int
				ratio: Float
				flag: Boolean
			}
		`)
	})

	t.Run("unknown type in SDL", func(t *testing.T) {
		expectSDLErrors(t, nil, rule, `
			type Query {
				thing: NotDefined
			}
		`,
			want{Message: `Unknown type "NotDefined"`, At: []at{{2, 9}}},
		)
	})

	// A document may refer to a type it defines itself.
	t.Run("types defined by the document are known", func(t *testing.T) {
		expectValidSDL(t, nil, rule, `
			type Query {
				thing: Thing
			}
			type Thing {
				name: String
			}
		`)
	})

	// Extending a schema means its types are in scope too.
	t.Run("types of the schema being extended are known", func(t *testing.T) {
		expectValidSDL(t, s, rule, `
			extend type Query {
				anotherDog: Dog
			}
		`)
	})
}
