package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestPossibleFragmentSpreads(t *testing.T) {
	s := testSchema(t)
	rule := validation.PossibleFragmentSpreadsRule

	t.Run("of the same object", func(t *testing.T) {
		expectValid(t, s, rule, `
			fragment objectWithinObject on Dog { ...dogFragment }
			fragment dogFragment on Dog { barkVolume }
		`)
	})

	t.Run("an object inside an interface it implements", func(t *testing.T) {
		expectValid(t, s, rule, `
			fragment objectWithinInterface on Pet { ...dogFragment }
			fragment dogFragment on Dog { barkVolume }
		`)
	})

	t.Run("an object inside a union it belongs to", func(t *testing.T) {
		expectValid(t, s, rule, `
			fragment objectWithinUnion on CatOrDog { ...dogFragment }
			fragment dogFragment on Dog { barkVolume }
		`)
	})

	t.Run("an interface inside an object implementing it", func(t *testing.T) {
		expectValid(t, s, rule, `
			fragment interfaceWithinObject on Dog { ...petFragment }
			fragment petFragment on Pet { name }
		`)
	})

	// Two interfaces overlap where some type implements both, even though
	// neither is a subtype of the other.
	t.Run("interfaces that share an implementation", func(t *testing.T) {
		expectValid(t, s, rule, `
			fragment interfaceWithinInterface on Pet { ...canineFragment }
			fragment canineFragment on Canine { name }
		`)
	})

	t.Run("an inline fragment that applies", func(t *testing.T) {
		expectValid(t, s, rule, `
			fragment objectWithinObject on Dog { ... on Dog { barkVolume } }
		`)
	})

	t.Run("two unrelated objects", func(t *testing.T) {
		expectErrors(t, s, rule, `
			fragment invalidObjectWithinObject on Cat { ...dogFragment }
			fragment dogFragment on Dog { barkVolume }
		`,
			want{Message: `Fragment "dogFragment" cannot be spread here as objects of type "Cat" can never be of type "Dog".`, At: []at{{1, 45}}},
		)
	})

	t.Run("an object not in a union", func(t *testing.T) {
		expectErrors(t, s, rule, `
			fragment invalidObjectWithinUnion on CatOrDog { ...humanFragment }
			fragment humanFragment on Human { pets { name } }
		`,
			want{Message: `can never be of type "Human"`, At: []at{{1, 49}}},
		)
	})

	t.Run("an inline fragment that cannot apply", func(t *testing.T) {
		expectErrors(t, s, rule, `
			fragment invalidObjectWithinObject on Cat { ... on Dog { barkVolume } }
		`,
			want{Message: `Fragment cannot be spread here as objects of type "Cat" can never be of type "Dog".`, At: []at{{1, 45}}},
		)
	})

	// An interface a type does not implement can still overlap it if some
	// other type does both; Human implements nothing, so it cannot.
	t.Run("an interface inside an unrelated object", func(t *testing.T) {
		expectErrors(t, s, rule, `
			fragment invalidInterfaceWithinObject on Human { ...petFragment }
			fragment petFragment on Pet { name }
		`,
			want{Message: `can never be of type "Pet"`, At: []at{{1, 50}}},
		)
	})
}
