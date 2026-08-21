package language

import (
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"
)

// readFixture loads a document from testdata.
func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return string(b)
}

// readFixtureBytes loads a fixture for a benchmark, where there is no *testing.T
// to report through.
func readFixtureBytes(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join("testdata", name))
}

// lexAll runs the lexer to completion and returns every non-ignored token.
func lexAll(t *testing.T, source *Source) []*Token {
	t.Helper()
	lexer := NewLexer(source)
	var tokens []*Token
	for {
		tok, err := lexer.Advance()
		if err != nil {
			t.Fatalf("%s: %v", source.Name, err)
		}
		if tok.Kind == TokenEOF {
			return tokens
		}
		tokens = append(tokens, tok)
	}
}

// The kitchen sink document exercises every syntactic construct in the
// language, so lexing it end to end is a broad smoke test.
func TestLexer_KitchenSink(t *testing.T) {
	body := readFixture(t, "kitchen-sink.graphql")
	tokens := lexAll(t, NewSource(body, SourceName("kitchen-sink.graphql")))
	if len(tokens) < 100 {
		t.Fatalf("lexed %d tokens, want a substantial document", len(tokens))
	}

	// Tokens must tile the source in order, without overlapping or moving
	// backwards, and every span must lie inside the body.
	prevEnd := 0
	for i, tok := range tokens {
		if tok.Start < prevEnd {
			t.Fatalf("token %d (%v) starts at %d, before the previous token ended at %d",
				i, tok.Kind, tok.Start, prevEnd)
		}
		if tok.End < tok.Start || tok.End > len(body) {
			t.Fatalf("token %d (%v) has span [%d,%d), outside the body of length %d",
				i, tok.Kind, tok.Start, tok.End, len(body))
		}
		if tok.Line < 1 || tok.Column < 1 {
			t.Fatalf("token %d (%v) has line:column %d:%d, want both positive",
				i, tok.Kind, tok.Line, tok.Column)
		}
		prevEnd = tok.End
	}
}

// A large real-world schema catches anything that only shows up at scale.
func TestLexer_GitHubSchema(t *testing.T) {
	body := readFixture(t, "github-schema.graphql")
	tokens := lexAll(t, NewSource(body, SourceName("github-schema.graphql")))
	if len(tokens) < 10000 {
		t.Fatalf("lexed %d tokens, want a large document", len(tokens))
	}
}

// Every token must report the line and column that an independent scan of the
// source derives for its start offset. The lexer tracks these incrementally,
// memoizing columns as it goes, so agreeing with a plain left-to-right walk
// over a large document is what proves that bookkeeping correct.
//
// The walk is deliberately linear. Calling GetLocation per token would rescan
// the document from the start each time, making the check quadratic.
func TestLexer_PositionsAgreeWithIndependentScan(t *testing.T) {
	for _, name := range []string{"kitchen-sink.graphql", "github-schema.graphql"} {
		t.Run(name, func(t *testing.T) {
			source := NewSource(readFixture(t, name), SourceName(name))
			body := source.Body
			tokens := lexAll(t, source)

			next := 0
			line, col := 1, 1
			for pos := 0; pos <= len(body); {
				for next < len(tokens) && tokens[next].Start == pos {
					tok := tokens[next]
					if tok.Line != line || tok.Column != col {
						t.Fatalf("token %v at offset %d reports %d:%d, want %d:%d",
							tok.Kind, pos, tok.Line, tok.Column, line, col)
					}
					next++
				}
				if pos == len(body) {
					break
				}
				switch body[pos] {
				case '\n':
					pos++
					line, col = line+1, 1
				case '\r':
					if pos+1 < len(body) && body[pos+1] == '\n' {
						pos += 2
					} else {
						pos++
					}
					line, col = line+1, 1
				default:
					_, size := utf8.DecodeRuneInString(body[pos:])
					if size < 1 {
						size = 1
					}
					pos += size
					col++
				}
			}
			if next != len(tokens) {
				t.Fatalf("checked %d of %d tokens; the scan and the lexer disagree on offsets", next, len(tokens))
			}
		})
	}
}

// GetLocation must agree with the lexer too. It rescans from the start of the
// document, so this runs only on the small fixture.
func TestGetLocation_AgreesWithLexer(t *testing.T) {
	source := NewSource(readFixture(t, "kitchen-sink.graphql"), SourceName("kitchen-sink.graphql"))
	for _, tok := range lexAll(t, source) {
		want := GetLocation(source, tok.Start)
		if tok.Line != want.Line || tok.Column != want.Column {
			t.Fatalf("token %v at offset %d reports %d:%d, want %d:%d",
				tok.Kind, tok.Start, tok.Line, tok.Column, want.Line, want.Column)
		}
	}
}

func BenchmarkLexer_GitHubSchema(b *testing.B) {
	body, err := os.ReadFile(filepath.Join("testdata", "github-schema.graphql"))
	if err != nil {
		b.Fatalf("reading fixture: %v", err)
	}
	source := NewSource(string(body))
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	for b.Loop() {
		lexer := NewLexer(source)
		for {
			tok, err := lexer.Advance()
			if err != nil {
				b.Fatal(err)
			}
			if tok.Kind == TokenEOF {
				break
			}
		}
	}
}
