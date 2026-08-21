package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestUniqueDirectivesPerLocation(t *testing.T) {
	s := testSchema(t)
	rule := validation.UniqueDirectivesPerLocationRule

	t.Run("no directives", func(t *testing.T) {
		expectValid(t, s, rule, `fragment Test on Dog { name }`)
	})

	t.Run("different directives in one place", func(t *testing.T) {
		expectValid(t, s, rule, `
			fragment Test on Dog {
				name @include(if: true) @skip(if: false)
			}
		`)
	})

	t.Run("the same directive in different places", func(t *testing.T) {
		expectValid(t, s, rule, `
			fragment Test on Dog {
				name @onField
				nickname @onField
			}
		`)
	})

	// A directive declared repeatable may be written as often as wanted.
	t.Run("a repeatable directive twice", func(t *testing.T) {
		expectValid(t, s, rule, `
			fragment Test on Dog {
				name @repeatableDirective(arg: 1) @repeatableDirective(arg: 2)
			}
		`)
	})

	t.Run("the same directive twice in one place", func(t *testing.T) {
		expectErrors(t, s, rule, `
			fragment Test on Dog {
				name @onField @onField
			}
		`,
			want{Message: `The directive "@onField" can only be used once at this location.`, At: []at{{2, 7}, {2, 16}}},
		)
	})

	t.Run("many duplicates", func(t *testing.T) {
		expectErrors(t, s, rule, `
			fragment Test on Dog @onFragmentDefinition @onFragmentDefinition {
				name @onField @onField @onField
			}
		`,
			want{Message: `"@onFragmentDefinition" can only be used once`, At: []at{{1, 22}, {1, 44}}},
			want{Message: `"@onField" can only be used once`, At: []at{{2, 7}, {2, 16}}},
			want{Message: `"@onField" can only be used once`, At: []at{{2, 7}, {2, 25}}},
		)
	})

	// A type and its extensions are one place: the directive ends up on the
	// same type either way.
	t.Run("a directive on a type and on an extension of it", func(t *testing.T) {
		expectSDLErrors(t, nil, rule, `
			directive @onObject on OBJECT
			type Thing @onObject { a: String }
			extend type Thing @onObject
		`,
			want{Message: `"@onObject" can only be used once`, At: []at{{2, 12}, {3, 19}}},
		)
	})

	t.Run("a directive on two different types", func(t *testing.T) {
		expectValidSDL(t, nil, rule, `
			directive @onObject on OBJECT
			type A @onObject { a: String }
			type B @onObject { b: String }
		`)
	})
}
