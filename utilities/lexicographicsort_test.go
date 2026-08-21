package utilities_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

func TestLexicographicSortSchema(t *testing.T) {
	s := mustBuild(t, `
		directive @zed on FIELD
		directive @alpha on FIELD

		type Query implements Zeta & Alpha {
			zebra(zed: Int, alpha: Int): String
			apple: String
			middle: Union
			colour: Colour
			filter(f: Filter): String
		}

		interface Zeta { middle: Union }
		interface Alpha { middle: Union }

		union Union = Zulu | Alfa
		type Zulu implements Zeta & Alpha { middle: Union }
		type Alfa implements Zeta & Alpha { middle: Union }

		enum Colour { ZED GREEN ALPHA }
		input Filter { zed: String alpha: String }
	`)

	sorted := utilities.LexicographicSortSchema(s)
	if err := schema.AssertValidSchema(sorted); err != nil {
		t.Fatalf("the sorted schema is not sound:\n%v", err)
	}
	text := utilities.PrintSchema(sorted)

	t.Run("types", func(t *testing.T) {
		assertOrder(t, text, "type Alfa", "interface Alpha", "enum Colour", "input Filter",
			"type Query", "union Union", "interface Zeta", "type Zulu")
	})

	// A field name may appear on several types, so the order is checked
	// within the type being tested rather than across the whole document.
	t.Run("fields", func(t *testing.T) {
		assertOrder(t, blockOf(t, text, "type Query"),
			"apple: String", "colour: Colour", "filter(", "middle: Union", "zebra(")
	})

	t.Run("arguments", func(t *testing.T) {
		if !strings.Contains(text, "zebra(alpha: Int, zed: Int)") {
			t.Errorf("the arguments are not sorted:\n%s", text)
		}
	})

	t.Run("enum members", func(t *testing.T) {
		assertOrder(t, text, "ALPHA", "GREEN", "ZED")
	})

	t.Run("input fields", func(t *testing.T) {
		assertOrder(t, blockOf(t, text, "input Filter"), "alpha: String", "zed: String")
	})

	t.Run("union members", func(t *testing.T) {
		if !strings.Contains(text, "union Union = Alfa | Zulu") {
			t.Errorf("the union members are not sorted:\n%s", text)
		}
	})

	t.Run("interfaces", func(t *testing.T) {
		if !strings.Contains(text, "type Query implements Alpha & Zeta") {
			t.Errorf("the implemented interfaces are not sorted:\n%s", text)
		}
	})

	t.Run("directives", func(t *testing.T) {
		assertOrder(t, text, "directive @alpha", "directive @zed")
	})
}

// Sorting must not change what the schema says, only the order it says it in.
func TestLexicographicSortSchema_SaysTheSameThing(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "language", "testdata", "github-schema.graphql"))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	original := mustBuild(t, string(body))
	sorted := utilities.LexicographicSortSchema(original)

	if err := schema.AssertValidSchema(sorted); err != nil {
		t.Fatalf("the sorted schema is not sound:\n%v", err)
	}
	if got, want := len(sorted.Types()), len(original.Types()); got != want {
		t.Errorf("%d types after sorting, want %d", got, want)
	}
	for _, typ := range original.Types() {
		if sorted.Type(typ.Name()) == nil {
			t.Errorf("%s was lost", typ.Name())
		}
	}
	// The roots still point at the right types in the new schema.
	if sorted.QueryType() == nil || sorted.QueryType().Name() != original.QueryType().Name() {
		t.Error("the query root changed")
	}

	// Sorting an already sorted schema changes nothing, so the order is
	// settled rather than merely different.
	once := utilities.PrintSchema(sorted)
	twice := utilities.PrintSchema(utilities.LexicographicSortSchema(sorted))
	if once != twice {
		t.Error("sorting twice gave something different from sorting once")
	}

	// And what comes out still reads back as a schema.
	rebuilt, err := utilities.BuildSchema(once)
	if err != nil {
		t.Fatalf("reading back the sorted schema: %v", err)
	}
	if utilities.PrintSchema(rebuilt) != once {
		t.Error("the sorted schema does not survive being written and read")
	}
}

// Two schemas that differ only in the order things were written in sort to the
// same text, which is what makes this useful for comparing them.
func TestLexicographicSortSchema_MakesSchemasComparable(t *testing.T) {
	a := mustBuild(t, `
		type Query { b: String a: String }
		enum E { Y X }
	`)
	b := mustBuild(t, `
		enum E { X Y }
		type Query { a: String b: String }
	`)
	sortedA := utilities.PrintSchema(utilities.LexicographicSortSchema(a))
	sortedB := utilities.PrintSchema(utilities.LexicographicSortSchema(b))
	if sortedA != sortedB {
		t.Errorf("the same schema written two ways did not sort alike:\n%s\n\n%s", sortedA, sortedB)
	}
	// Without sorting they differ, or the test would prove nothing.
	if utilities.PrintSchema(a) == utilities.PrintSchema(b) {
		t.Fatal("the two schemas already print alike; the test proves nothing")
	}
}

func TestLexicographicSortSchema_Nothing(t *testing.T) {
	if got := utilities.LexicographicSortSchema(nil); got != nil {
		t.Errorf("sorting nothing gave %v", got)
	}
}

// blockOf returns the braced definition beginning with the given header, so
// that an order can be checked within one type rather than across a document
// where the same field name appears on several.
func blockOf(t *testing.T, text, header string) string {
	t.Helper()
	start := strings.Index(text, header)
	if start < 0 {
		t.Fatalf("the output has no %q:\n%s", header, text)
	}
	end := strings.Index(text[start:], "\n}")
	if end < 0 {
		t.Fatalf("%q is not a braced definition:\n%s", header, text[start:])
	}
	return text[start : start+end]
}

// assertOrder checks that the given fragments appear in the text in the order
// given.
func assertOrder(t *testing.T, text string, wanted ...string) {
	t.Helper()
	at := -1
	for _, fragment := range wanted {
		found := strings.Index(text, fragment)
		if found < 0 {
			t.Fatalf("the output does not contain %q:\n%s", fragment, text)
		}
		if found < at {
			t.Errorf("%q comes before something that should precede it:\n%s", fragment, text)
			return
		}
		at = found
	}
}
