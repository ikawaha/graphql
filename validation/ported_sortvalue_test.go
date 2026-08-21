package validation_test

// Ported from graphql-js src/utilities/__tests__/sortValueNode-test.ts.
//
// Upstream sorts a literal into a settled form and exports the function; here
// the rule lives inside the two places that need it, so it is asked for
// through them. This is the second: whether two selections agree about their
// arguments. The first is in utilities/ported_sortvalue_test.go.
//
// The rule: the fields of an input object are unordered and compare by name; a
// list is ordered and stays as written.

import (
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
	"github.com/ikawaha/graphql/validation"
)

func TestPortedSortValue_ArgumentsThatOnlyLookDifferent(t *testing.T) {
	s, err := utilities.BuildSchema(`
		input Deep { d: Int e: Int f: Int g: Int }
		input Thing { a: Deep b: Deep c: Int d: Int }
		type Query { f(arg: Thing, args: [Thing]): String }
	`)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}

	for _, tt := range []struct {
		name      string
		query     string
		mergeable bool
	}{
		{
			name:      "object fields in another order",
			query:     `{ f(arg: { c: 3, d: 4 }) f(arg: { d: 4, c: 3 }) }`,
			mergeable: true,
		},
		{
			name:      "nested object fields in another order",
			query:     `{ f(arg: { a: { e: 5, d: 4 } }) f(arg: { a: { d: 4, e: 5 } }) }`,
			mergeable: true,
		},
		{
			name:      "objects inside a list",
			query:     `{ f(args: [{ c: 3, d: 4 }]) f(args: [{ d: 4, c: 3 }]) }`,
			mergeable: true,
		},
		{
			name: "objects nested two deep",
			query: `{ f(arg: { b: { g: 7, f: 6 }, c: 3, a: { d: 4, e: 5 } })` +
				` f(arg: { a: { d: 4, e: 5 }, b: { f: 6, g: 7 }, c: 3 }) }`,
			mergeable: true,
		},
		{
			name:      "a list in another order",
			query:     `{ f(args: [{ c: 3 }, { d: 4 }]) f(args: [{ d: 4 }, { c: 3 }]) }`,
			mergeable: false,
		},
		{
			name:      "a field with another value",
			query:     `{ f(arg: { c: 3 }) f(arg: { c: 4 }) }`,
			mergeable: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			expectAgreementOnArguments(t, s, tt.query, tt.mergeable)
		})
	}
}

func expectAgreementOnArguments(t *testing.T, s *schema.Schema, query string, mergeable bool) {
	t.Helper()
	if mergeable {
		expectValid(t, s, validation.OverlappingFieldsCanBeMergedRule, query)
		return
	}
	expectErrors(t, s, validation.OverlappingFieldsCanBeMergedRule, query,
		want{Message: `Fields "f" conflict because they have differing arguments. ` +
			`Use different aliases on the fields to fetch both if this was intentional.`})
}
