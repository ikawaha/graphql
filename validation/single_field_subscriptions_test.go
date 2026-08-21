package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestSingleFieldSubscriptions(t *testing.T) {
	s := testSchema(t)
	rule := validation.SingleFieldSubscriptionsRule

	t.Run("one field", func(t *testing.T) {
		expectValid(t, s, rule, `subscription S { newMessage { body } }`)
	})

	t.Run("one field through a fragment", func(t *testing.T) {
		expectValid(t, s, rule, `
			subscription S { ...F }
			fragment F on Subscription { newMessage { body } }
		`)
	})

	// A query may select as many as it likes; only a subscription is limited.
	t.Run("a query with several fields", func(t *testing.T) {
		expectValid(t, s, rule, `query { dog { name } cat { name } }`)
	})

	t.Run("two fields", func(t *testing.T) {
		expectErrors(t, s, rule, `
			subscription S {
				newMessage { body }
				disallowedSecondRootField
			}
		`,
			want{Message: `Subscription "S" must select only one top level field.`, At: []at{{3, 2}}},
		)
	})

	t.Run("two fields in an anonymous subscription", func(t *testing.T) {
		expectErrors(t, s, rule, `
			subscription {
				newMessage { body }
				disallowedSecondRootField
			}
		`,
			want{Message: "Anonymous Subscription must select only one top level field.", At: []at{{3, 2}}},
		)
	})

	t.Run("two fields through fragments", func(t *testing.T) {
		expectErrors(t, s, rule, `
			subscription S { ...F }
			fragment F on Subscription {
				newMessage { body }
				disallowedSecondRootField
			}
		`,
			want{Message: "must select only one top level field", At: []at{{4, 2}}},
		)
	})

	// Whether a top level field is selected must not depend on anything but the
	// document, so neither directive may be used there — not even with a
	// condition that is settled where it is written.
	t.Run("a conditional at the top level", func(t *testing.T) {
		expectErrors(t, s, rule, `
			subscription S {
				newMessage { body }
				disallowedSecondRootField @skip(if: true)
			}
		`,
			want{Message: "must not use `@skip` or `@include` directives in the top level selection",
				At: []at{{3, 28}}},
		)
	})

	// Below the top level they are ordinary.
	t.Run("a conditional below the top level", func(t *testing.T) {
		expectValid(t, s, rule, `
			subscription S {
				newMessage { body @skip(if: true) }
			}
		`)
	})

	// The same field asked for twice is one field, so it is allowed.
	t.Run("the same field twice", func(t *testing.T) {
		expectValid(t, s, rule, `
			subscription S {
				newMessage { body }
				newMessage { sender }
			}
		`)
	})

	t.Run("an introspection field", func(t *testing.T) {
		expectErrors(t, s, rule, `subscription S { __typename }`,
			want{Message: `Subscription "S" must not select an introspection top level field.`, At: []at{{1, 18}}},
		)
	})
}
