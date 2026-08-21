package utilities_test

// Ported from graphql-js src/utilities/__tests__/concatAST-test.ts and
// getOperationAST-test.ts.

import (
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/utilities"
)

func TestPortedConcatDocuments(t *testing.T) {
	a := mustParseDocument(t, "\n      { a, b, ...Frag }\n    ")
	b := mustParseDocument(t, "\n      fragment Frag on T {\n        c\n      }\n    ")

	joined := utilities.ConcatDocuments(a, b)
	const want = `{
  a
  b
  ...Frag
}

fragment Frag on T {
  c
}`
	if got := language.Print(joined); got != want {
		t.Errorf("wrote\n%s\nwant\n%s", got, want)
	}
}

// Joining nothing, or documents that are not there, gives an empty document
// rather than nil, so a caller can hand the result straight on.
func TestConcatDocuments_NothingToJoin(t *testing.T) {
	for _, tt := range []struct {
		name string
		docs []*language.Document
	}{
		{"no documents", nil},
		{"only absent ones", []*language.Document{nil, nil}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			joined := utilities.ConcatDocuments(tt.docs...)
			if joined == nil {
				t.Fatal("joined to nothing at all")
			}
			if len(joined.Definitions) != 0 {
				t.Errorf("%d definitions, want none", len(joined.Definitions))
			}
		})
	}
}

// The token count carries over, since it is what bounds what a request may
// cost and a joined document costs what its parts did.
func TestConcatDocuments_AddsUpTheTokenCount(t *testing.T) {
	a := mustParseDocument(t, "{ a }")
	b := mustParseDocument(t, "{ b }")
	joined := utilities.ConcatDocuments(a, b)
	if want := a.TokenCount + b.TokenCount; joined.TokenCount != want {
		t.Errorf("TokenCount = %d, want %d", joined.TokenCount, want)
	}
}

func TestPortedFindOperation(t *testing.T) {
	const several = `
      query TestQ { field }
      mutation TestM { field }
      subscription TestS { field }
    `
	for _, tt := range []struct {
		name, doc, asked string
		// want is the index of the definition expected, or -1 for none.
		want int
	}{
		{name: "a simple document", doc: "{ field }", want: 0},
		{name: "a named mutation", doc: "mutation Test { field }", want: 0},
		{name: "a named subscription", doc: "subscription Test { field }", want: 0},
		{name: "a document with no operation", doc: "type Foo { field: String }", want: -1},
		{
			name: "an ambiguous unnamed operation",
			doc: `
      { field }
      mutation Test { field }
      subscription TestSub { field }
    `,
			want: -1,
		},
		{name: "an ambiguous named operation", doc: several, want: -1},
		{
			name: "a name nothing answers to",
			doc: `
      { field }

      query TestQ { field }
      mutation TestM { field }
      subscription TestS { field }
    `,
			asked: "Unknown",
			want:  -1,
		},
		{name: "the first by name", doc: several, asked: "TestQ", want: 0},
		{name: "the second by name", doc: several, asked: "TestM", want: 1},
		{name: "the third by name", doc: several, asked: "TestS", want: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc := mustParseDocument(t, tt.doc)
			got := utilities.FindOperation(doc, tt.asked)
			if tt.want < 0 {
				if got != nil {
					t.Errorf("found %v, want none", got.Name)
				}
				return
			}
			if got == nil {
				t.Fatalf("found none, want definition %d", tt.want)
			}
			if want := doc.Definitions[tt.want]; got != want {
				t.Errorf("found the wrong operation")
			}
		})
	}

	t.Run("a document that is not there", func(t *testing.T) {
		if utilities.FindOperation(nil, "") != nil {
			t.Error("an operation was found in nothing")
		}
	})
}
