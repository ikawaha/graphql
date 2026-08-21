package utilities_test

// Ported from graphql-js src/utilities/__tests__/sortValueNode-test.ts.
//
// Upstream sorts a literal into a settled form and exports the function;
// here the same rule lives inside the two places that need it — comparing two
// schemas' default values, and deciding whether two field selections agree
// about their arguments — so it is asked for through them.
//
// The rule: the fields of an input object are unordered and are compared by
// name; a list is ordered and stays as written.

import (
	"fmt"
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

func TestPortedSortValue_DefaultsThatOnlyLookDifferent(t *testing.T) {
	for _, tt := range []struct {
		name          string
		before, after string
		// same says the two defaults mean the same thing, so nothing changed.
		same bool
	}{
		{
			name:   "object fields in another order",
			before: `{ b: 2, a: 1 }`,
			after:  `{ a: 1, b: 2 }`,
			same:   true,
		},
		{
			name:   "nested object fields in another order",
			before: `{ a: { c: 3, b: 2 } }`,
			after:  `{ a: { b: 2, c: 3 } }`,
			same:   true,
		},
		{
			name:   "objects inside a list",
			before: `[{ b: 2, a: 1 }, { d: 4, c: 3 }]`,
			after:  `[{ a: 1, b: 2 }, { c: 3, d: 4 }]`,
			same:   true,
		},
		{
			name:   "objects nested two deep",
			before: `{ b: { g: 7, f: 6 }, c: 3, a: { d: 4, e: 5 } }`,
			after:  `{ a: { d: 4, e: 5 }, b: { f: 6, g: 7 }, c: 3 }`,
			same:   true,
		},
		{
			name:   "a list in another order really is different",
			before: `[{ a: 1 }, { b: 2 }]`,
			after:  `[{ b: 2 }, { a: 1 }]`,
			same:   false,
		},
		{
			name:   "a field with another value",
			before: `{ a: 1, b: 2 }`,
			after:  `{ a: 1, b: 3 }`,
			same:   false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			const shape = `
				input Deep { d: Int e: Int f: Int g: Int }
				input Thing { a: Deep b: Deep c: Int d: Int }
				type Query { f(arg: [Thing] = %s): String }
			`
			before := buildForChanges(t, shape, tt.before)
			after := buildForChanges(t, shape, tt.after)

			changes := utilities.FindSchemaChanges(before, after)
			var about []string
			for _, change := range changes {
				if change.Coordinate == "Query.f(arg:)" {
					about = append(about, change.Message)
				}
			}
			if tt.same && len(about) != 0 {
				t.Errorf("two spellings of one default were reported as a change: %v", about)
			}
			if !tt.same && len(about) == 0 {
				t.Error("a default that really changed was not reported")
			}
		})
	}
}

// buildForChanges builds a schema from a shape with one default written into
// it.
func buildForChanges(t *testing.T, shape, written string) *schema.Schema {
	t.Helper()
	s, err := utilities.BuildSchema(fmt.Sprintf(shape, written))
	if err != nil {
		t.Fatalf("building with default %s: %v", written, err)
	}
	return s
}
