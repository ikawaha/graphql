package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestFieldsOnCorrectType(t *testing.T) {
	s := testSchema(t)
	rule := validation.FieldsOnCorrectTypeRule

	t.Run("a field on an object", func(t *testing.T) {
		expectValid(t, s, rule, `fragment objectFieldSelection on Dog { __typename name }`)
	})

	t.Run("a field on an interface", func(t *testing.T) {
		expectValid(t, s, rule, `fragment interfaceFieldSelection on Pet { __typename name }`)
	})

	t.Run("a field of an implementation reached through a fragment", func(t *testing.T) {
		expectValid(t, s, rule, `
			fragment f on Pet {
				... on Dog { barks }
			}
		`)
	})

	t.Run("a field the type does not have", func(t *testing.T) {
		expectErrors(t, s, rule, `fragment fieldNotDefined on Dog { meowVolume }`,
			want{Message: `Cannot query field "meowVolume" on type "Dog". Did you mean "barkVolume"?`, At: []at{{1, 35}}},
		)
	})

	// A union has no fields, so nothing about a name would help.
	t.Run("a field on a union", func(t *testing.T) {
		expectErrors(t, s, rule, `fragment f on CatOrDog { name }`,
			want{Message: `Cannot query field "name" on type "CatOrDog".`, At: []at{{1, 26}}},
		)
	})

	// The field exists on some of what the interface stands for, so what is
	// missing is a fragment rather than a correction to the name.
	t.Run("a field of an implementation asked of the interface", func(t *testing.T) {
		expectErrors(t, s, rule, `fragment f on Pet { nickname }`,
			want{Message: `Did you mean to use an inline fragment on "Cat" or "Dog"?`, At: []at{{1, 21}}},
		)
	})

	// Where the field is on an interface that several implementations share,
	// that interface is the better suggestion because one fragment covers all.
	t.Run("an interface is suggested ahead of its implementations", func(t *testing.T) {
		expectErrors(t, s, rule, `fragment f on CatOrDog { name }`,
			want{Message: `Did you mean to use an inline fragment on "Pet"`, At: []at{{1, 26}}},
		)
	})

	t.Run("a misspelt field deep in a query", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				human(id: 4) {
					pets {
						nam
					}
				}
			}
		`,
			want{Message: `Cannot query field "nam" on type "Pet". Did you mean "name"?`, At: []at{{4, 4}}},
		)
	})
}
