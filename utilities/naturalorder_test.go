package utilities_test

import (
	"strings"
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

// Sorting a schema puts names in the order a person reading them would expect,
// which is graphql-js's naturalCompare: a run of digits counts as the number
// it spells, so a2 comes before a10 rather than after it.
func TestLexicographicSortSchema_NaturalOrder(t *testing.T) {
	s, err := utilities.BuildSchema(`
		type Query { a10: String a2: String a1: String field10: String field9: String }
	`)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	sorted := utilities.LexicographicSortSchema(s)

	var names []string
	for _, f := range sorted.QueryType().Fields() {
		names = append(names, f.Name())
	}
	if got, want := strings.Join(names, " "), "a1 a2 a10 field9 field10"; got != want {
		t.Errorf("the fields came out as %q, want %q", got, want)
	}
}

// The same order settles which suggestion comes first where two are equally
// close to what was written.
func TestSuggestionList_NaturalOrder(t *testing.T) {
	// Both are one edit from what was written, so nothing but the order
	// settles which comes first.
	got := schema.SuggestionList("x19", []string{"x10", "x9"})
	if want := "x9 x10"; strings.Join(got, " ") != want {
		t.Errorf("suggestions came out as %v, want %s", got, want)
	}
}
