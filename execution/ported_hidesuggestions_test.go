package execution_test

// Ported from graphql-js's execution and top-level cases for hideSuggestions:
// a request may ask that no message say what it might have meant.

import (
	"context"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/language"
)

// A default value that will not coerce is explained while the request is being
// prepared, so the request's say over suggestions has to reach that far.
func TestPortedWithoutSuggestions_AnInvalidDefaultValue(t *testing.T) {
	s := testVariablesSchema(t)
	const query = `query ($input: TestInputObject = { c: "ok", aa: "x" }) ` +
		`{ fieldWithObjectInput(input: $input) }`

	for _, tt := range []struct {
		name string
		hide bool
		says string
	}{
		{
			name: "with suggestions",
			says: `Variable "$input" has invalid default value: Expected value of type ` +
				`"TestInputObject" not to include unknown field "aa". Did you mean "a"? ` +
				`Found: { c: "ok", aa: "x" }.`,
		},
		{
			name: "without",
			hide: true,
			says: `Variable "$input" has invalid default value: Expected value of type ` +
				`"TestInputObject" not to include unknown field "aa", found: { c: "ok", aa: "x" }.`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := language.ParseString(query)
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}
			result := execution.Execute(context.Background(), execution.Request{
				Schema: s, Document: doc, HideSuggestions: tt.hide,
			})
			if len(result.Errors) != 1 {
				t.Fatalf("errors = %v, want one", result.Errors)
			}
			if got := result.Errors[0].Message; got != tt.says {
				t.Errorf("says %q\nwant %q", got, tt.says)
			}
		})
	}
}

// An argument whose value will not coerce is explained while the field runs.
func TestPortedWithoutSuggestions_AnArgumentValue(t *testing.T) {
	s := buildSchema(t, `
		enum Colour { RED GREEN }
		type Query { paint(in: Colour): String }
	`)
	const query = `{ paint(in: red) }`

	for _, tt := range []struct {
		name string
		hide bool
		says string
	}{
		{
			name: "with suggestions",
			says: `Argument "Query.paint(in:)" has invalid value: Value "red" does not exist ` +
				`in "Colour" enum. Did you mean the enum value "RED"?`,
		},
		{
			name: "without",
			hide: true,
			says: `Argument "Query.paint(in:)" has invalid value: Value "red" does not exist ` +
				`in "Colour" enum.`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result := execution.Execute(context.Background(), execution.Request{
				Schema: s, Document: mustParse(t, query),
				RootValue: map[string]any{}, HideSuggestions: tt.hide,
			})
			if len(result.Errors) != 1 {
				t.Fatalf("errors = %v, want one", result.Errors)
			}
			if got := result.Errors[0].Message; got != tt.says {
				t.Errorf("says %q\nwant %q", got, tt.says)
			}
		})
	}
}
