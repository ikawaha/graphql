package utilities_test

import (
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/utilities"
)

// FuzzStripIgnoredCharacters says that taking the ignored characters out of a
// document leaves the same document.
//
// graphql-js fuzzes this by putting random ignored tokens between every pair
// of tokens, which is ported beside this. What a Go fuzzer adds is inputs
// nobody thought to enumerate — a string holding what looks like a comment, a
// block string holding what looks like a string, and so on, where taking a
// character out would change what the document says.
func FuzzStripIgnoredCharacters(f *testing.F) {
	for _, s := range []string{
		"{ a }",
		"# comment\n{ a }",
		`{ f(a: "# not a comment") }`,
		"{ f(a: \"\"\"\n  keep  these\n  spaces\n\"\"\") }",
		`{ f(a: "  ") }`,
		"query Q ( $v : Int = 1 ) @d { f ( a : $v ) { ... F } }",
		"{a,,,b}",
		"{ f(a: \"\\\"\") }",
		"\ufeff{ a }",
		"{ a }\r\n",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, body string) {
		before, err := language.ParseString(body)
		if err != nil {
			return // Not a document; stripping it says nothing.
		}
		stripped, err := utilities.StripIgnoredCharacters(body)
		if err != nil {
			t.Fatalf("a document that parses would not strip: %v\n%q", err, body)
		}
		after, err := language.ParseString(stripped)
		if err != nil {
			t.Fatalf("the stripping of %q does not parse: %v\n%q", body, err, stripped)
		}
		if want, got := language.Print(before), language.Print(after); want != got {
			t.Errorf("stripping changed %q\nbefore: %s\n after: %s", body, want, got)
		}
	})
}
