package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
	"github.com/ikawaha/graphql/validation"
)

// @defer and @stream are experimental, so a schema has them only by declaring
// them. Declared here with the arguments the specification gives them.
func deferSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := utilities.BuildSchema(`
		directive @defer(if: Boolean = true, label: String) on FRAGMENT_SPREAD | INLINE_FRAGMENT
		directive @stream(if: Boolean = true, label: String, initialCount: Int = 0) on FIELD

		type Query {
			name: String
			friends: [String]
		}
		type Mutation {
			setName(name: String): String
			names: [String]
		}
		type Subscription {
			nameChanged: String
			namesChanged: [String]
		}
	`)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	if err := schema.AssertValidSchema(s); err != nil {
		t.Fatalf("the schema is not sound: %v", err)
	}
	return s
}

func TestDeferStreamDirectiveOnRootField(t *testing.T) {
	s := deferSchema(t)
	rule := validation.DeferStreamDirectiveOnRootFieldRule

	t.Run("on a query", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				... @defer { name }
				friends @stream
			}
		`)
	})

	t.Run("on a root mutation field", func(t *testing.T) {
		expectErrors(t, s, rule, `
			mutation {
				... @defer { setName(name: "a") }
				names @stream
			}
		`,
			want{Message: `Defer directive cannot be used on root mutation type "Mutation".`, At: []at{{2, 6}}},
			want{Message: `Stream directive cannot be used on root mutation type "Mutation".`, At: []at{{3, 8}}},
		)
	})

	t.Run("on a root subscription field", func(t *testing.T) {
		expectErrors(t, s, rule, `
			subscription {
				... @defer { nameChanged }
			}
		`,
			want{Message: `Defer directive cannot be used on root subscription type "Subscription".`, At: []at{{2, 6}}},
		)
	})
}

func TestStreamDirectiveOnListField(t *testing.T) {
	s := deferSchema(t)
	rule := validation.StreamDirectiveOnListFieldRule

	t.Run("on a list field", func(t *testing.T) {
		expectValid(t, s, rule, `{ friends @stream }`)
	})

	t.Run("on a field that is not a list", func(t *testing.T) {
		expectErrors(t, s, rule, `{ name @stream }`,
			want{Message: `Directive "@stream" cannot be used on non-list field "Query.name".`, At: []at{{1, 8}}},
		)
	})
}

func TestDeferStreamDirectiveLabel(t *testing.T) {
	s := deferSchema(t)
	rule := validation.DeferStreamDirectiveLabelRule

	t.Run("no labels", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				... @defer { name }
				friends @stream
			}
		`)
	})

	t.Run("different labels", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				... @defer(label: "a") { name }
				friends @stream(label: "b")
			}
		`)
	})

	// A label picks out one piece of the response, so it cannot name two.
	t.Run("the same label twice", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				... @defer(label: "same") { name }
				friends @stream(label: "same")
			}
		`,
			want{Message: "must be unique across all Defer/Stream directive usages", At: []at{{2, 6}, {3, 10}}},
		)
	})

	// A variable is not known until the request runs, by which time the label
	// is already needed.
	t.Run("a label given as a variable", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query ($label: String) {
				... @defer(label: $label) { name }
			}
		`,
			want{Message: `Argument "@defer(label:)" must be a static string.`, At: []at{{2, 6}}},
		)
	})
}

func TestDeferStreamDirectiveOnValidOperations(t *testing.T) {
	s := deferSchema(t)
	rule := validation.DeferStreamDirectiveOnValidOperationsRule

	t.Run("on a query", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				... @defer { name }
				friends @stream
			}
		`)
	})

	t.Run("on a subscription", func(t *testing.T) {
		expectErrors(t, s, rule, `
			subscription {
				... @defer { nameChanged }
				namesChanged @stream
			}
		`,
			want{Message: "Defer directive not supported on subscription operations", At: []at{{2, 6}}},
			want{Message: "Stream directive not supported on subscription operations", At: []at{{3, 15}}},
		)
	})

	// Turning it off is allowed, so one document can serve both a query and a
	// subscription.
	t.Run("switched off on a subscription", func(t *testing.T) {
		expectValid(t, s, rule, `
			subscription {
				... @defer(if: false) { nameChanged }
				namesChanged @stream(if: false)
			}
		`)
	})

	// A fragment is in a subscription if a subscription spreads it, wherever
	// it is written. The error points at the whole route, so that a reader can
	// see how the deferral ended up in a subscription.
	t.Run("inside a fragment a subscription spreads", func(t *testing.T) {
		expectErrors(t, s, rule, `
			subscription {
				...F
			}
			fragment F on Subscription {
				... @defer { nameChanged }
			}
		`,
			want{
				Message: "Defer directive not supported on subscription operations",
				At:      []at{{5, 6}, {2, 2}},
			},
		)
	})

	t.Run("inside a fragment only a query spreads", func(t *testing.T) {
		expectValid(t, s, rule, `
			query {
				...F
			}
			fragment F on Query {
				... @defer { name }
			}
		`)
	})
}
