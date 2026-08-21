package graphql_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ikawaha/graphql"
)

// jsonOf renders a result the way it would go over the wire, which is the form
// worth asserting on: it shows the order of the keys, tells null apart from
// absent, and reads as what a client would receive.
func jsonOf(t *testing.T, v any) string {
	t.Helper()
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("rendering the response: %v", err)
	}
	return string(out)
}

// Ported from graphql-js's "passes validation options through to validate":
// what Params says about suggestions reaches both halves of a request.
func TestDo_HideSuggestions(t *testing.T) {
	s, err := graphql.BuildSchema(`
		enum Colour { RED }
		type Query { greeting: String paint(in: Colour): String }
	`)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}

	for _, tt := range []struct {
		name  string
		query string
		with  string
		hide  string
	}{
		{
			name:  "a field the schema does not have, caught while checking",
			query: `{ greetng }`,
			with:  `Cannot query field "greetng" on type "Query". Did you mean "greeting"?`,
			hide:  `Cannot query field "greetng" on type "Query".`,
		},
		{
			name:  "a value that will not coerce, caught while checking",
			query: `{ paint(in: red) }`,
			with: `Value "red" does not exist in "Colour" enum. ` +
				`Did you mean the enum value "RED"?`,
			hide: `Value "red" does not exist in "Colour" enum.`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			shown := graphql.Do(context.Background(), graphql.Params{Schema: s, Query: tt.query})
			if len(shown.Errors) != 1 || shown.Errors[0].Message != tt.with {
				t.Errorf("with suggestions: %v, want %q", shown.Errors, tt.with)
			}
			hidden := graphql.Do(context.Background(), graphql.Params{
				Schema: s, Query: tt.query, HideSuggestions: true,
			})
			if len(hidden.Errors) != 1 || hidden.Errors[0].Message != tt.hide {
				t.Errorf("without: %v, want %q", hidden.Errors, tt.hide)
			}
		})
	}
}

// MaxErrors bounds how many problems checking reports. graphql-js exposes it
// as a validation option too; here it rides alongside HideSuggestions.
func TestDo_MaxErrors(t *testing.T) {
	s, err := graphql.BuildSchema(`type Query { a: String }`)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	const query = `{ b c d e f }`

	all := graphql.Do(context.Background(), graphql.Params{Schema: s, Query: query})
	if len(all.Errors) != 5 {
		t.Fatalf("%d errors, want one per unknown field", len(all.Errors))
	}

	capped := graphql.Do(context.Background(), graphql.Params{
		Schema: s, Query: query, MaxErrors: 2,
	})
	// The cap is reached and the walk gives up, which is itself reported.
	if len(capped.Errors) > 3 {
		t.Errorf("%d errors with a cap of 2: %v", len(capped.Errors), capped.Errors)
	}
	if len(capped.Errors) < 2 {
		t.Errorf("%d errors with a cap of 2, want it to report up to the cap", len(capped.Errors))
	}
}
