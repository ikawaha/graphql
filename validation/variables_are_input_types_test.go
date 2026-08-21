package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestVariablesAreInputTypes(t *testing.T) {
	s := testSchema(t)
	rule := validation.VariablesAreInputTypesRule

	t.Run("input types are accepted", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo($a: String, $b: [Boolean!]!, $c: ComplexInput, $d: FurColor) {
				human(id: 4) { name }
			}
		`)
	})

	// A type the schema does not have is a different complaint, made by
	// another rule, so this one says nothing about it.
	t.Run("an unknown type is left to another rule", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo($a: Unknown) {
				human(id: 4) { name }
			}
		`)
	})

	t.Run("output types are rejected", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query Foo($a: Dog, $b: [[CatOrDog!]]!, $c: Pet) {
				human(id: 4) { name }
			}
		`,
			want{Message: `Variable "$a" cannot be non-input type "Dog"`, At: []at{{1, 15}}},
			want{Message: `Variable "$b" cannot be non-input type "[[CatOrDog!]]!"`, At: []at{{1, 24}}},
			want{Message: `Variable "$c" cannot be non-input type "Pet"`, At: []at{{1, 44}}},
		)
	})
}
