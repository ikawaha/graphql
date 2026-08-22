package language_test

import (
	"strings"
	"testing"

	"github.com/ikawaha/graphql/language"
)

// Properties that hold for every input, checked by the fuzzer.
//
// graphql-js fuzzes by enumerating every string over a small alphabet, which
// is ported beside this in the blockString and stripIgnoredCharacters files.
// What Go adds is a fuzzer that goes looking: these say what must be true of
// any input at all, and `go test -fuzz` finds the inputs that break it.
//
// They need no recording of what graphql-js answered, because none of them
// compares against it. What they assert is that the parser and the printer
// agree with each other.

// seeds are the shapes a fuzzer would take a long time to reach on its own.
var seeds = []string{
	"",
	"{a}",
	"{ a b c }",
	"query Q($v: Int = 1) @dir { f(a: $v) { ...F @skip(if: true) } }",
	"fragment F on T { __typename ... on U { x } }",
	"mutation { m(input: { a: [1, 2.5, null, true, ENUM, \"s\"] }) }",
	"subscription { s }",
	"{ f(a: \"\"\"\n  block\n  string\n\"\"\") }",
	"type T implements I & J @d { f(a: Int = 1): [String!]! }",
	"schema { query: Q } scalar S @specifiedBy(url: \"u\")",
	"interface I { f: Int } union U = A | B enum E { A B } input In { a: Int }",
	"extend type T { g: Int } directive @d(a: Int) repeatable on FIELD",
	"{ f(a: 1e100, b: -0.0, c: \"\\u0041\\n\\t\") }",
	"{ éé }",
	"# comment\n{ a } # trailing",
	"{a,b,,,c}",
}

// FuzzParse says the parser answers rather than falling over, whatever it is
// handed. A syntax error is a fine answer; a panic is not, and neither is a
// document that came back nil without one.
func FuzzParse(f *testing.F) {
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, body string) {
		doc, err := language.ParseString(body)
		switch {
		case err != nil && doc != nil:
			t.Errorf("both a document and an error for %q", body)
		case err == nil && doc == nil:
			t.Errorf("neither a document nor an error for %q", body)
		}
	})
}

// FuzzPrintParse says printing a document and reading it back gives the same
// document, which is what makes the printer safe to round-trip through.
//
// The comparison is between two printings rather than two trees: the first
// printing is the normal form, so printing it again has to leave it alone.
func FuzzPrintParse(f *testing.F) {
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, body string) {
		doc, err := language.ParseString(body)
		if err != nil {
			return // Not a document; there is nothing to print.
		}
		once := language.Print(doc)
		again, err := language.ParseString(once)
		if err != nil {
			t.Fatalf("the printing of %q does not parse: %v\n%s", body, err, once)
		}
		if twice := language.Print(again); twice != once {
			t.Errorf("printing is not settled for %q\n first: %s\nsecond: %s", body, once, twice)
		}
	})
}

// FuzzParseValue and FuzzParseType say the same of the two smaller parsers,
// which a client reaches for when it has a value or a type on its own.
func FuzzParseValue(f *testing.F) {
	for _, s := range []string{
		"1", "-0.0", "1e100", `"s"`, `"""b"""`, "true", "null", "ENUM",
		"$v", "[1, [2], {}]", `{ a: 1, b: { c: [null] } }`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, body string) {
		v, err := language.ParseValue(language.NewSource(body))
		if err != nil {
			return
		}
		once := language.Print(v)
		again, err := language.ParseValue(language.NewSource(once))
		if err != nil {
			t.Fatalf("the printing of %q does not parse: %v\n%s", body, err, once)
		}
		if twice := language.Print(again); twice != once {
			t.Errorf("printing is not settled for %q\n first: %s\nsecond: %s", body, once, twice)
		}
	})
}

func FuzzParseType(f *testing.F) {
	for _, s := range []string{"T", "T!", "[T]", "[T!]!", "[[T]]"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, body string) {
		ty, err := language.ParseType(language.NewSource(body))
		if err != nil {
			return
		}
		once := language.Print(ty)
		if once != strings.TrimSpace(once) {
			t.Errorf("a type printed with space around it: %q", once)
		}
		again, err := language.ParseType(language.NewSource(once))
		if err != nil {
			t.Fatalf("the printing of %q does not parse: %v", body, err)
		}
		if twice := language.Print(again); twice != once {
			t.Errorf("printing is not settled for %q: %s then %s", body, once, twice)
		}
	})
}
