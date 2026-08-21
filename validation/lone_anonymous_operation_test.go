package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestLoneAnonymousOperation(t *testing.T) {
	s := testSchema(t)
	rule := validation.LoneAnonymousOperationRule

	t.Run("no operations", func(t *testing.T) {
		expectValid(t, s, rule, `
			fragment fragA on Dog {
				name
			}
		`)
	})

	t.Run("one anonymous operation", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				dog {
					name
				}
			}
		`)
	})

	t.Run("several named operations", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo {
				dog { name }
			}
			query Bar {
				cat { name }
			}
		`)
	})

	// A fragment is not an operation, so an anonymous operation alongside one
	// is still the only operation.
	t.Run("anonymous operation with a fragment", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				dog {
					...Frag
				}
			}
			fragment Frag on Dog {
				name
			}
		`)
	})

	t.Run("anonymous operation with another anonymous one", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				dog { name }
			}
			{
				cat { name }
			}
		`,
			want{Message: "anonymous operation must be the only defined operation", At: []at{{1, 1}}},
			want{Message: "anonymous operation must be the only defined operation", At: []at{{4, 1}}},
		)
	})

	t.Run("anonymous operation with a named one", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				dog { name }
			}
			query Foo {
				cat { name }
			}
		`,
			want{Message: "anonymous operation must be the only defined operation", At: []at{{1, 1}}},
		)
	})

	t.Run("anonymous operation with a mutation", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				dog { name }
			}
			mutation Foo {
				testMutation(arg: "x")
			}
		`,
			want{Message: "anonymous operation must be the only defined operation", At: []at{{1, 1}}},
		)
	})
}
