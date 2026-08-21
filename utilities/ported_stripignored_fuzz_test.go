package utilities_test

// Ported from graphql-js src/utilities/__tests__/stripIgnoredCharacters-fuzz.ts.
//
// Every combination of the tokens the grammar ignores is placed around and
// between every kind of token, and what comes out is checked against the rule:
// an ignored token goes away, except where removing it would run two tokens
// together, and except inside a string.
//
// Stripping twice is checked to give the same thing as stripping once, which
// is what says the result is itself a document.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/utilities"
)

var (
	fuzzIgnored = []string{
		"\ufeff",  // byte order mark
		"\t", " ", // white space
		"\n", "\r", "\r\n", // line terminators
		"# \"Comment\" string\n", // a comment
		",",                      // a comma
	}
	fuzzPunctuators = []string{
		"!", "$", "(", ")", "...", ":", "=", "@", "[", "]", "{", "|", "}",
	}
	fuzzOthers = []string{
		"name_token", "1", "3.14", `"some string value"`, "\"\"\"block\nstring\nvalue\"\"\"",
	}
)

// expectStripped checks what a document strips to, and that stripping the
// result again changes nothing.
func expectStripped(t *testing.T, document, want string) {
	t.Helper()
	stripped, err := utilities.StripIgnoredCharacters(document)
	if err != nil {
		t.Fatalf("stripping %q: %v", document, err)
	}
	if stripped != want {
		t.Fatalf("stripping %q gave %q, want %q", document, stripped, want)
	}
	again, err := utilities.StripIgnoredCharacters(stripped)
	if err != nil {
		t.Fatalf("stripping %q again: %v", stripped, err)
	}
	if again != stripped {
		t.Fatalf("stripping %q again gave %q", stripped, again)
	}
}

func TestPortedStripIgnoredCharacters_Fuzz(t *testing.T) {
	all := strings.Join(fuzzIgnored, "")

	t.Run("strips a document of nothing but ignored tokens", func(t *testing.T) {
		for _, ignored := range fuzzIgnored {
			expectStripped(t, ignored, "")
			for _, another := range fuzzIgnored {
				expectStripped(t, ignored+another, "")
			}
		}
		expectStripped(t, all, "")
	})

	t.Run("strips ignored tokens before and after a token", func(t *testing.T) {
		for _, token := range append(append([]string{}, fuzzPunctuators...), fuzzOthers...) {
			for _, ignored := range fuzzIgnored {
				expectStripped(t, ignored+token, token)
				expectStripped(t, token+ignored, token)
				for _, another := range fuzzIgnored {
					expectStripped(t, token+ignored+ignored, token)
					expectStripped(t, ignored+another+token, token)
				}
			}
			expectStripped(t, all+token, token)
			expectStripped(t, token+all, token)
		}
	})

	t.Run("strips ignored tokens between two punctuators", func(t *testing.T) {
		for _, left := range fuzzPunctuators {
			for _, right := range fuzzPunctuators {
				for _, ignored := range fuzzIgnored {
					expectStripped(t, left+ignored+right, left+right)
					for _, another := range fuzzIgnored {
						expectStripped(t, left+ignored+another+right, left+right)
					}
				}
				expectStripped(t, left+all+right, left+right)
			}
		}
	})

	t.Run("strips ignored tokens between a punctuator and one that is not", func(t *testing.T) {
		for _, other := range fuzzOthers {
			for _, punctuator := range fuzzPunctuators {
				for _, ignored := range fuzzIgnored {
					expectStripped(t, punctuator+ignored+other, punctuator+other)
					for _, another := range fuzzIgnored {
						expectStripped(t, punctuator+ignored+another+other, punctuator+other)
					}
				}
				expectStripped(t, punctuator+all+other, punctuator+other)
			}
		}
	})

	t.Run("strips ignored tokens between one that is not a punctuator and one that is", func(t *testing.T) {
		for _, other := range fuzzOthers {
			for _, punctuator := range fuzzPunctuators {
				// The spread is the exception, and has its own case below.
				if punctuator == "..." {
					continue
				}
				for _, ignored := range fuzzIgnored {
					expectStripped(t, other+ignored+punctuator, other+punctuator)
					for _, another := range fuzzIgnored {
						expectStripped(t, other+ignored+another+punctuator, other+punctuator)
					}
				}
				expectStripped(t, other+all+punctuator, other+punctuator)
			}
		}
	})

	t.Run("leaves a space before a spread", func(t *testing.T) {
		for _, other := range fuzzOthers {
			for _, ignored := range fuzzIgnored {
				expectStripped(t, other+ignored+"...", other+" ...")
				for _, another := range fuzzIgnored {
					expectStripped(t, other+ignored+another+" ...", other+" ...")
				}
			}
			expectStripped(t, other+all+"...", other+" ...")
		}
	})

	t.Run("leaves a space between two tokens that are not punctuators", func(t *testing.T) {
		for _, left := range fuzzOthers {
			for _, right := range fuzzOthers {
				for _, ignored := range fuzzIgnored {
					expectStripped(t, left+ignored+right, left+" "+right)
					for _, another := range fuzzIgnored {
						expectStripped(t, left+ignored+another+right, left+" "+right)
					}
				}
				expectStripped(t, left+all+right, left+" "+right)
			}
		}
	})

	t.Run("leaves what is inside a string alone", func(t *testing.T) {
		for _, ignored := range fuzzIgnored {
			quoted := quoteJSON(t, ignored)
			expectStripped(t, quoted, quoted)
			for _, another := range fuzzIgnored {
				quoted := quoteJSON(t, ignored+another)
				expectStripped(t, quoted, quoted)
			}
		}
		quoted := quoteJSON(t, all)
		expectStripped(t, quoted, quoted)
	})

	t.Run("leaves what is inside a block string alone", func(t *testing.T) {
		var keep []string
		for _, ignored := range fuzzIgnored {
			switch ignored {
			case "\n", "\r", "\r\n", "\t", " ":
				// A block string is reindented, so these do not survive.
			default:
				keep = append(keep, ignored)
			}
		}
		for _, ignored := range keep {
			document := `"""|` + ignored + `|"""`
			expectStripped(t, document, document)
			for _, another := range keep {
				document := `"""|` + ignored + another + `|"""`
				expectStripped(t, document, document)
			}
		}
	})
}

// quoteJSON writes a string the way a GraphQL document would, which for the
// characters here is what JSON does.
func quoteJSON(t *testing.T, s string) string {
	t.Helper()
	quoted, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("quoting %q: %v", s, err)
	}
	return string(quoted)
}
