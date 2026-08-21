package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestNoUnusedVariables(t *testing.T) {
	s := testSchema(t)
	rule := validation.NoUnusedVariablesRule

	t.Run("all variables used", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo($a: String, $b: String) {
				complicatedArgs {
					stringArgField(stringArg: $a)
					booleanArgField(booleanArg: $b)
				}
			}
		`)
	})

	t.Run("used by a fragment", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo($a: String) {
				complicatedArgs { ...FragA }
			}
			fragment FragA on ComplicatedArgs {
				stringArgField(stringArg: $a)
			}
		`)
	})

	t.Run("a variable never used", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query Foo($a: String, $b: String, $c: String) {
				complicatedArgs {
					stringArgField(stringArg: $a)
					booleanArgField(booleanArg: $b)
				}
			}
		`,
			want{Message: `Variable "$c" is never used in operation "Foo".`, At: []at{{1, 35}}},
		)
	})

	t.Run("a variable never used in an anonymous operation", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query ($a: String) {
				dog { name }
			}
		`,
			want{Message: `Variable "$a" is never used.`, At: []at{{1, 8}}},
		)
	})

	// A variable used only by a fragment the operation does not spread is not
	// used by that operation.
	t.Run("used by an unrelated fragment", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query Foo($a: String) {
				complicatedArgs { ...FragA }
			}
			fragment FragA on ComplicatedArgs {
				intArgField(intArg: 1)
			}
			fragment FragB on ComplicatedArgs {
				stringArgField(stringArg: $a)
			}
		`,
			want{Message: `Variable "$a" is never used in operation "Foo".`, At: []at{{1, 11}}},
		)
	})
}
