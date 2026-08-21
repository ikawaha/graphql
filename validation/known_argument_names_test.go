package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestKnownArgumentNames(t *testing.T) {
	s := testSchema(t)
	rule := validation.KnownArgumentNamesRule

	t.Run("known arguments", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				dog {
					doesKnowCommand(dogCommand: SIT)
					isAtLocation(x: 0, y: 0)
				}
				complicatedArgs {
					multipleReqs(req1: 1, req2: 2)
				}
			}
		`)
	})

	t.Run("a known directive argument", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				dog @include(if: true) { name }
			}
		`)
	})

	t.Run("a misspelt field argument", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				dog {
					doesKnowCommand(DogCommand: SIT)
				}
			}
		`,
			want{Message: `Unknown argument "DogCommand" on field "Dog.doesKnowCommand". Did you mean "dogCommand"?`, At: []at{{3, 19}}},
		)
	})

	t.Run("an argument the field does not take", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				dog {
					isAtLocation(x: 0, z: 0)
				}
			}
		`,
			want{Message: `Unknown argument "z" on field "Dog.isAtLocation"`, At: []at{{3, 22}}},
		)
	})

	t.Run("an unknown directive argument", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				dog @include(unless: false) { name }
			}
		`,
			want{Message: `Unknown argument "unless" on directive "@include".`, At: []at{{2, 15}}},
		)
	})

	// An unknown directive is a separate complaint, so nothing is said about
	// what it was given.
	t.Run("arguments of an unknown directive", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				dog @unknown(anything: 1) { name }
			}
		`)
	})

	t.Run("a directive argument in SDL", func(t *testing.T) {
		expectSDLErrors(t, nil, rule, `
			directive @tag(name: String) on OBJECT
			type Query @tag(nmae: "x") { a: String }
		`,
			want{Message: `Unknown argument "nmae" on directive "@tag". Did you mean "name"?`, At: []at{{2, 17}}},
		)
	})
}
