package utilities_test

import (
	"strings"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/utilities"
)

// printed renders a separated document, which is how a test says what came out.
func printed(t *testing.T, doc *language.Document) string {
	t.Helper()
	if doc == nil {
		return "<nothing>"
	}
	return language.Print(doc)
}

func TestSeparateOperations(t *testing.T) {
	t.Run("one operation", func(t *testing.T) {
		doc := mustParseDocument(t, `{ a }`)
		got := utilities.SeparateOperations(doc)
		if len(got) != 1 {
			t.Fatalf("%d documents, want 1", len(got))
		}
		// An unnamed operation is keyed by the empty string.
		if _, present := got[""]; !present {
			t.Errorf("the unnamed operation is not under the empty name; keys = %v", keysOf(got))
		}
	})

	t.Run("each operation gets its own document", func(t *testing.T) {
		doc := mustParseDocument(t, `
			query One { a }
			query Two { b }
			mutation Three { c }
		`)
		got := utilities.SeparateOperations(doc)
		if len(got) != 3 {
			t.Fatalf("%d documents, want 3: %v", len(got), keysOf(got))
		}
		if text := printed(t, got["One"]); !strings.Contains(text, "a") || strings.Contains(text, "b") {
			t.Errorf("One =\n%s", text)
		}
	})

	// Each document carries the fragments its operation reaches, and no more:
	// that is the whole point of separating them.
	t.Run("only the fragments each operation reaches", func(t *testing.T) {
		doc := mustParseDocument(t, `
			query One { ...A }
			query Two { ...B }
			fragment A on Query { a }
			fragment B on Query { b ...C }
			fragment C on Query { c }
			fragment Unused on Query { d }
		`)
		got := utilities.SeparateOperations(doc)

		one := printed(t, got["One"])
		if !strings.Contains(one, "fragment A") {
			t.Errorf("One lost the fragment it uses:\n%s", one)
		}
		for _, absent := range []string{"fragment B", "fragment C", "fragment Unused"} {
			if strings.Contains(one, absent) {
				t.Errorf("One carries %s, which it does not use:\n%s", absent, one)
			}
		}

		// A fragment reached through another is carried too.
		two := printed(t, got["Two"])
		for _, wanted := range []string{"fragment B", "fragment C"} {
			if !strings.Contains(two, wanted) {
				t.Errorf("Two lost %s:\n%s", wanted, two)
			}
		}
		if strings.Contains(two, "fragment A") {
			t.Errorf("Two carries a fragment it does not use:\n%s", two)
		}
	})

	// Two operations may share a fragment, and each gets it.
	t.Run("a shared fragment", func(t *testing.T) {
		doc := mustParseDocument(t, `
			query One { ...Shared }
			query Two { ...Shared }
			fragment Shared on Query { a }
		`)
		got := utilities.SeparateOperations(doc)
		for _, name := range []string{"One", "Two"} {
			if !strings.Contains(printed(t, got[name]), "fragment Shared") {
				t.Errorf("%s lost the shared fragment", name)
			}
		}
	})

	// A cycle between fragments is a separate complaint; following one here
	// must terminate rather than spin.
	t.Run("a fragment cycle terminates", func(t *testing.T) {
		doc := mustParseDocument(t, `
			query One { ...A }
			fragment A on Query { a ...B }
			fragment B on Query { b ...A }
		`)
		got := utilities.SeparateOperations(doc)
		one := printed(t, got["One"])
		for _, wanted := range []string{"fragment A", "fragment B"} {
			if !strings.Contains(one, wanted) {
				t.Errorf("One lost %s:\n%s", wanted, one)
			}
		}
	})

	// The definitions keep the order the original document gave them, so a
	// separated document prints the same way every time.
	t.Run("a settled order", func(t *testing.T) {
		doc := mustParseDocument(t, `
			query One { ...C ...A ...B }
			fragment A on Query { a }
			fragment B on Query { b }
			fragment C on Query { c }
		`)
		first := printed(t, utilities.SeparateOperations(doc)["One"])
		for range 10 {
			if again := printed(t, utilities.SeparateOperations(doc)["One"]); again != first {
				t.Fatalf("the order drifted:\n%s\n\n%s", first, again)
			}
		}
		// And that order is the one the document was written in.
		if a, b := strings.Index(first, "fragment A"), strings.Index(first, "fragment B"); a > b {
			t.Errorf("the fragments are not in the order they were defined:\n%s", first)
		}
	})

	t.Run("no operations", func(t *testing.T) {
		if got := utilities.SeparateOperations(mustParseDocument(t, `fragment A on Query { a }`)); len(got) != 0 {
			t.Errorf("%d documents, want none", len(got))
		}
		if got := utilities.SeparateOperations(nil); got != nil {
			t.Errorf("separating nothing gave %v", got)
		}
	})

	// A separated document has to be one the rest of the library accepts.
	t.Run("what comes out parses back", func(t *testing.T) {
		doc := mustParseDocument(t, `
			query One($v: Int) { field(arg: $v) { ...A } }
			fragment A on Thing { a ... on Other { b } }
		`)
		text := printed(t, utilities.SeparateOperations(doc)["One"])
		if _, err := language.ParseString(text); err != nil {
			t.Errorf("the separated document does not parse: %v\n%s", err, text)
		}
	})
}

// keysOf lists the names a separation produced, for a failure message.
func keysOf(docs map[string]*language.Document) []string {
	names := make([]string, 0, len(docs))
	for name := range docs {
		names = append(names, name)
	}
	return names
}

// mustParseDocument parses a test document.
func mustParseDocument(t *testing.T, body string) *language.Document {
	t.Helper()
	doc, err := language.ParseString(body)
	if err != nil {
		t.Fatalf("parsing: %v\n%s", err, body)
	}
	return doc
}
