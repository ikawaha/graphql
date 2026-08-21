package language_test

// Ported from graphql-js src/language/__tests__/lexer-test.ts: what each piece
// of source lexes into, and what is said about source that will not lex.
//
// Where a token's start and end are given they are compared as byte offsets,
// which is what indexes a Go string; graphql-js counts UTF-16 code units, so a
// case whose source is not ASCII is listed as a known divergence.

import (
	"testing"

	"github.com/ikawaha/graphql/language"
)

// knownLexerDivergences are the cases this implementation does not match, and
// why. Each is asserted to *still* diverge, so that closing one cannot go
// unnoticed.
var knownLexerDivergences = map[string]string{
	// A token's start and end are byte offsets here and UTF-16 code units in
	// graphql-js, so they part company as soon as the source is not ASCII.
	// See COMPATIBILITY.md.
	`ignores BOM header: \ufeff foo`:                                            "offsets are counted in bytes",
	"lexes strings: \"unescaped unicode outside BMP \U0001F600\"":               "offsets are counted in bytes",
	"lexes block strings: \"\"\"unescaped unicode outside BMP \U0001F600\"\"\"": "offsets are counted in bytes",
}

func TestPortedLexer_Tokens(t *testing.T) {
	for _, tt := range []struct {
		name, in_ string
		// second says to take the token after the first, which is how a case
		// about what follows something is written.
		second                   bool
		kind                     language.TokenKind
		start, end, line, column int
		value                    string
	}{
		{name: "ignores BOM header: \\ufeff foo", in_: "\uFEFF foo", second: false, kind: language.TokenName, start: 2, end: 5, value: "foo"},
		{name: "tracks line breaks: foo", in_: "foo", second: false, kind: language.TokenName, start: 0, end: 3, line: 1, column: 1, value: "foo"},
		{name: "tracks line breaks: \\nfoo", in_: "\nfoo", second: false, kind: language.TokenName, start: 1, end: 4, line: 2, column: 1, value: "foo"},
		{name: "tracks line breaks: \\rfoo", in_: "\rfoo", second: false, kind: language.TokenName, start: 1, end: 4, line: 2, column: 1, value: "foo"},
		{name: "tracks line breaks: \\r\\nfoo", in_: "\r\nfoo", second: false, kind: language.TokenName, start: 2, end: 5, line: 2, column: 1, value: "foo"},
		{name: "tracks line breaks: \\n\\rfoo", in_: "\n\rfoo", second: false, kind: language.TokenName, start: 2, end: 5, line: 3, column: 1, value: "foo"},
		{name: "tracks line breaks: \\r\\r\\n\\nfoo", in_: "\r\r\n\nfoo", second: false, kind: language.TokenName, start: 4, end: 7, line: 4, column: 1, value: "foo"},
		{name: "tracks line breaks: \\n\\n\\r\\rfoo", in_: "\n\n\r\rfoo", second: false, kind: language.TokenName, start: 4, end: 7, line: 5, column: 1, value: "foo"},
		{name: "records line and column: \\n \\r\\n \\r  foo\\n", in_: "\n \r\n \r  foo\n", second: false, kind: language.TokenName, start: 8, end: 11, line: 4, column: 3, value: "foo"},
		{name: "skips whitespace and comments: \\t\\tfoo\\t\\t", in_: "\t\tfoo\t\t", second: false, kind: language.TokenName, start: 2, end: 5, value: "foo"},
		{name: "skips whitespace and comments: ,,,foo,,,", in_: ",,,foo,,,", second: false, kind: language.TokenName, start: 3, end: 6, value: "foo"},
		{name: "lexes strings: \"\"", in_: "\"\"", second: false, kind: language.TokenString, start: 0, end: 2, value: ""},
		{name: "lexes strings: \"simple\"", in_: "\"simple\"", second: false, kind: language.TokenString, start: 0, end: 8, value: "simple"},
		{name: "lexes strings: \" white space \"", in_: "\" white space \"", second: false, kind: language.TokenString, start: 0, end: 15, value: " white space "},
		{name: "lexes strings: \"quote \\\\\"\"", in_: "\"quote \\\"\"", second: false, kind: language.TokenString, start: 0, end: 10, value: "quote \""},
		{name: "lexes strings: \"escaped \\\\n\\\\r\\\\b\\\\t\\\\f\"", in_: "\"escaped \\n\\r\\b\\t\\f\"", second: false, kind: language.TokenString, start: 0, end: 20, value: "escaped \n\r\u0008\t\u000C"},
		{name: "lexes strings: \"slashes \\\\\\\\ \\\\/\"", in_: "\"slashes \\\\ \\/\"", second: false, kind: language.TokenString, start: 0, end: 15, value: "slashes \\ /"},
		{name: "lexes strings: \"unescaped unicode outside BMP \U0001F600\"", in_: "\"unescaped unicode outside BMP \U0001F600\"", second: false, kind: language.TokenString, start: 0, end: 34, value: "unescaped unicode outside BMP \U0001F600"},
		{name: "lexes strings: \"unicode \\\\u1234\\\\u5678\\\\u90AB\\\\uCDEF\"", in_: "\"unicode \\u1234\\u5678\\u90AB\\uCDEF\"", second: false, kind: language.TokenString, start: 0, end: 34, value: "unicode \u1234\u5678\u90AB\uCDEF"},
		{name: "lexes strings: \"string with minimal unicode escape \\\\u{0}\"", in_: "\"string with minimal unicode escape \\u{0}\"", second: false, kind: language.TokenString, start: 0, end: 42, value: "string with minimal unicode escape \u0000"},
		{name: "lexes block strings: \"\"\"\"\"\"", in_: "\"\"\"\"\"\"", second: false, kind: language.TokenBlockString, start: 0, end: 6, line: 1, column: 1, value: ""},
		{name: "lexes block strings: \"\"\"simple\"\"\"", in_: "\"\"\"simple\"\"\"", second: false, kind: language.TokenBlockString, start: 0, end: 12, line: 1, column: 1, value: "simple"},
		{name: "lexes block strings: \"\"\" white space \"\"\"", in_: "\"\"\" white space \"\"\"", second: false, kind: language.TokenBlockString, start: 0, end: 19, line: 1, column: 1, value: " white space "},
		{name: "lexes block strings: \"\"\"contains \" quote\"\"\"", in_: "\"\"\"contains \" quote\"\"\"", second: false, kind: language.TokenBlockString, start: 0, end: 22, line: 1, column: 1, value: "contains \" quote"},
		{name: "lexes block strings: \"\"\"contains \\\\\"\"\" triple quote\"\"\"", in_: "\"\"\"contains \\\"\"\" triple quote\"\"\"", second: false, kind: language.TokenBlockString, start: 0, end: 32, line: 1, column: 1, value: "contains \"\"\" triple quote"},
		{name: "lexes block strings: \"\"\"multi\\nline\"\"\"", in_: "\"\"\"multi\nline\"\"\"", second: false, kind: language.TokenBlockString, start: 0, end: 16, line: 1, column: 1, value: "multi\nline"},
		{name: "lexes block strings: \"\"\"multi\\rline\\r\\nnormalized\"\"\"", in_: "\"\"\"multi\rline\r\nnormalized\"\"\"", second: false, kind: language.TokenBlockString, start: 0, end: 28, line: 1, column: 1, value: "multi\nline\nnormalized"},
		{name: "lexes block strings: \"\"\"unescaped \\\\n\\\\r\\\\b\\\\t\\\\f\\\\u1234\"\"\"", in_: "\"\"\"unescaped \\n\\r\\b\\t\\f\\u1234\"\"\"", second: false, kind: language.TokenBlockString, start: 0, end: 32, line: 1, column: 1, value: "unescaped \\n\\r\\b\\t\\f\\u1234"},
		{name: "lexes block strings: \"\"\"unescaped unicode outside BMP \U0001F600\"\"\"", in_: "\"\"\"unescaped unicode outside BMP \U0001F600\"\"\"", second: false, kind: language.TokenBlockString, start: 0, end: 38, line: 1, column: 1, value: "unescaped unicode outside BMP \U0001F600"},
		{name: "lexes block strings: \"\"\"slashes \\\\\\\\ \\\\/\"\"\"", in_: "\"\"\"slashes \\\\ \\/\"\"\"", second: false, kind: language.TokenBlockString, start: 0, end: 19, line: 1, column: 1, value: "slashes \\\\ \\/"},
		{name: "lexes numbers: 4", in_: "4", second: false, kind: language.TokenInt, start: 0, end: 1, value: "4"},
		{name: "lexes numbers: 4.123", in_: "4.123", second: false, kind: language.TokenFloat, start: 0, end: 5, value: "4.123"},
		{name: "lexes numbers: -4", in_: "-4", second: false, kind: language.TokenInt, start: 0, end: 2, value: "-4"},
		{name: "lexes numbers: 9", in_: "9", second: false, kind: language.TokenInt, start: 0, end: 1, value: "9"},
		{name: "lexes numbers: 0", in_: "0", second: false, kind: language.TokenInt, start: 0, end: 1, value: "0"},
		{name: "lexes numbers: -4.123", in_: "-4.123", second: false, kind: language.TokenFloat, start: 0, end: 6, value: "-4.123"},
		{name: "lexes numbers: 0.123", in_: "0.123", second: false, kind: language.TokenFloat, start: 0, end: 5, value: "0.123"},
		{name: "lexes numbers: 123e4", in_: "123e4", second: false, kind: language.TokenFloat, start: 0, end: 5, value: "123e4"},
		{name: "lexes numbers: 123E4", in_: "123E4", second: false, kind: language.TokenFloat, start: 0, end: 5, value: "123E4"},
		{name: "lexes numbers: 123e-4", in_: "123e-4", second: false, kind: language.TokenFloat, start: 0, end: 6, value: "123e-4"},
		{name: "lexes numbers: 123e+4", in_: "123e+4", second: false, kind: language.TokenFloat, start: 0, end: 6, value: "123e+4"},
		{name: "lexes numbers: -1.123e4", in_: "-1.123e4", second: false, kind: language.TokenFloat, start: 0, end: 8, value: "-1.123e4"},
		{name: "lexes numbers: -1.123E4", in_: "-1.123E4", second: false, kind: language.TokenFloat, start: 0, end: 8, value: "-1.123E4"},
		{name: "lexes numbers: -1.123e-4", in_: "-1.123e-4", second: false, kind: language.TokenFloat, start: 0, end: 9, value: "-1.123e-4"},
		{name: "lexes numbers: -1.123e+4", in_: "-1.123e+4", second: false, kind: language.TokenFloat, start: 0, end: 9, value: "-1.123e+4"},
		{name: "lexes numbers: -1.123e4567", in_: "-1.123e4567", second: false, kind: language.TokenFloat, start: 0, end: 11, value: "-1.123e4567"},
		{name: "lexes punctuation: !", in_: "!", second: false, kind: language.TokenBang, start: 0, end: 1},
		{name: "lexes punctuation: $", in_: "$", second: false, kind: language.TokenDollar, start: 0, end: 1},
		{name: "lexes punctuation: (", in_: "(", second: false, kind: language.TokenParenL, start: 0, end: 1},
		{name: "lexes punctuation: )", in_: ")", second: false, kind: language.TokenParenR, start: 0, end: 1},
		{name: "lexes punctuation: ...", in_: "...", second: false, kind: language.TokenSpread, start: 0, end: 3},
		{name: "lexes punctuation: :", in_: ":", second: false, kind: language.TokenColon, start: 0, end: 1},
		{name: "lexes punctuation: =", in_: "=", second: false, kind: language.TokenEquals, start: 0, end: 1},
		{name: "lexes punctuation: @", in_: "@", second: false, kind: language.TokenAt, start: 0, end: 1},
		{name: "lexes punctuation: [", in_: "[", second: false, kind: language.TokenBracketL, start: 0, end: 1},
		{name: "lexes punctuation: ]", in_: "]", second: false, kind: language.TokenBracketR, start: 0, end: 1},
		{name: "lexes punctuation: {", in_: "{", second: false, kind: language.TokenBraceL, start: 0, end: 1},
		{name: "lexes punctuation: |", in_: "|", second: false, kind: language.TokenPipe, start: 0, end: 1},
		{name: "lexes punctuation: }", in_: "}", second: false, kind: language.TokenBraceR, start: 0, end: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lexer := language.NewLexer(language.NewSource(tt.in_))
			token, err := lexer.Advance()
			if err == nil && tt.second {
				token, err = lexer.Advance()
			}
			if err != nil {
				t.Fatalf("lexing: %v", err)
			}
			same := token.Kind == tt.kind && token.Value == tt.value &&
				token.Start == tt.start && (tt.end == 0 || token.End == tt.end) &&
				(tt.line == 0 || token.Line == tt.line) &&
				(tt.column == 0 || token.Column == tt.column)

			if why, listed := knownLexerDivergences[tt.name]; listed {
				if same {
					t.Errorf("this case now matches graphql-js; remove it from the known divergences (%s)", why)
				} else {
					t.Logf("known divergence: %s", why)
				}
				return
			}
			if token.Kind != tt.kind {
				t.Errorf("kind = %s, want %s", token.Kind, tt.kind)
			}
			if token.Value != tt.value {
				t.Errorf("value = %q, want %q", token.Value, tt.value)
			}
			if token.Start != tt.start {
				t.Errorf("start = %d, want %d", token.Start, tt.start)
			}
			if tt.end != 0 && token.End != tt.end {
				t.Errorf("end = %d, want %d", token.End, tt.end)
			}
			if tt.line != 0 && token.Line != tt.line {
				t.Errorf("line = %d, want %d", token.Line, tt.line)
			}
			if tt.column != 0 && token.Column != tt.column {
				t.Errorf("column = %d, want %d", token.Column, tt.column)
			}
		})
	}
}

func TestPortedLexer_Errors(t *testing.T) {
	for _, tt := range []struct {
		name, in_, says string
		line, column    int
	}{
		{name: "reports unexpected characters: .", in_: ".", says: "Syntax Error: Unexpected character: \".\".", line: 1, column: 1},
		{name: "lex reports useful string errors: \"", in_: "\"", says: "Syntax Error: Unterminated string.", line: 1, column: 2},
		{name: "lex reports useful string errors: \"\"\"", in_: "\"\"\"", says: "Syntax Error: Unterminated string.", line: 1, column: 4},
		{name: "lex reports useful string errors: \"\"\"\"", in_: "\"\"\"\"", says: "Syntax Error: Unterminated string.", line: 1, column: 5},
		{name: "lex reports useful string errors: \"no end quote", in_: "\"no end quote", says: "Syntax Error: Unterminated string.", line: 1, column: 14},
		{name: "lex reports useful string errors: 'single quotes'", in_: "'single quotes'", says: "Syntax Error: Unexpected single quote character ('), did you mean to use a double quote (\")?", line: 1, column: 1},
		{name: "lex reports useful string errors: \"bad surrogate \\udead\"", in_: "\"bad surrogate \xed\xba\xad\"", says: "Syntax Error: Invalid character within String: U+DEAD.", line: 1, column: 16},
		{name: "lex reports useful string errors: \"bad high surrogate pair \\udead\\udead\"", in_: "\"bad high surrogate pair \xed\xba\xad\xed\xba\xad\"", says: "Syntax Error: Invalid character within String: U+DEAD.", line: 1, column: 26},
		{name: "lex reports useful string errors: \"bad low surrogate pair \\ud800\\ud800\"", in_: "\"bad low surrogate pair \xed\xa0\x80\xed\xa0\x80\"", says: "Syntax Error: Invalid character within String: U+D800.", line: 1, column: 25},
		{name: "lex reports useful string errors: \"multi\\nline\"", in_: "\"multi\nline\"", says: "Syntax Error: Unterminated string.", line: 1, column: 7},
		{name: "lex reports useful string errors: \"multi\\rline\"", in_: "\"multi\rline\"", says: "Syntax Error: Unterminated string.", line: 1, column: 7},
		{name: "lex reports useful string errors: \"bad \\\\z esc\"", in_: "\"bad \\z esc\"", says: "Syntax Error: Invalid character escape sequence: \"\\z\".", line: 1, column: 6},
		{name: "lex reports useful string errors: \"bad \\\\x esc\"", in_: "\"bad \\x esc\"", says: "Syntax Error: Invalid character escape sequence: \"\\x\".", line: 1, column: 6},
		{name: "lex reports useful string errors: \"bad \\\\u1 esc\"", in_: "\"bad \\u1 esc\"", says: "Syntax Error: Invalid Unicode escape sequence: \"\\u1 es\".", line: 1, column: 6},
		{name: "lex reports useful string errors: \"bad \\\\u0XX1 esc\"", in_: "\"bad \\u0XX1 esc\"", says: "Syntax Error: Invalid Unicode escape sequence: \"\\u0XX1\".", line: 1, column: 6},
		{name: "lex reports useful string errors: \"bad \\\\uXXXX esc\"", in_: "\"bad \\uXXXX esc\"", says: "Syntax Error: Invalid Unicode escape sequence: \"\\uXXXX\".", line: 1, column: 6},
		{name: "lex reports useful string errors: \"bad \\\\uFXXX esc\"", in_: "\"bad \\uFXXX esc\"", says: "Syntax Error: Invalid Unicode escape sequence: \"\\uFXXX\".", line: 1, column: 6},
		{name: "lex reports useful string errors: \"bad \\\\uXXXF esc\"", in_: "\"bad \\uXXXF esc\"", says: "Syntax Error: Invalid Unicode escape sequence: \"\\uXXXF\".", line: 1, column: 6},
		{name: "lex reports useful string errors: \"bad \\\\u{} esc\"", in_: "\"bad \\u{} esc\"", says: "Syntax Error: Invalid Unicode escape sequence: \"\\u{}\".", line: 1, column: 6},
		{name: "lex reports useful string errors: \"bad \\\\u{FXXX} esc\"", in_: "\"bad \\u{FXXX} esc\"", says: "Syntax Error: Invalid Unicode escape sequence: \"\\u{FX\".", line: 1, column: 6},
		{name: "lex reports useful string errors: \"bad \\\\u{FFFF esc\"", in_: "\"bad \\u{FFFF esc\"", says: "Syntax Error: Invalid Unicode escape sequence: \"\\u{FFFF \".", line: 1, column: 6},
		{name: "lex reports useful string errors: \"bad \\\\u{FFFF\"", in_: "\"bad \\u{FFFF\"", says: "Syntax Error: Invalid Unicode escape sequence: \"\\u{FFFF\"\".", line: 1, column: 6},
		{name: "lex reports useful string errors: \"too high \\\\u{110000} esc\"", in_: "\"too high \\u{110000} esc\"", says: "Syntax Error: Invalid Unicode escape sequence: \"\\u{110000}\".", line: 1, column: 11},
		{name: "lex reports useful string errors: \"way too high \\\\u{12345678} esc\"", in_: "\"way too high \\u{12345678} esc\"", says: "Syntax Error: Invalid Unicode escape sequence: \"\\u{12345678}\".", line: 1, column: 15},
		{name: "lex reports useful string errors: \"too long \\\\u{000000000} esc\"", in_: "\"too long \\u{000000000} esc\"", says: "Syntax Error: Invalid Unicode escape sequence: \"\\u{000000000\".", line: 1, column: 11},
		{name: `lex reports useful string errors: "bad surrogate \uDEAD esc"`, in_: `"bad surrogate \uDEAD esc"`, says: `Syntax Error: Invalid Unicode escape sequence: "\uDEAD".`, line: 1, column: 16},
		{name: `lex reports useful string errors: "bad \uD83D\not an escape"`, in_: `"bad \uD83D\not an escape"`, says: `Syntax Error: Invalid Unicode escape sequence: "\uD83D".`, line: 1, column: 6},
		{name: "lex reports useful block string errors: \"\"\"", in_: "\"\"\"", says: "Syntax Error: Unterminated string.", line: 1, column: 4},
		{name: "lex reports useful block string errors: \"\"\"no end quote", in_: "\"\"\"no end quote", says: "Syntax Error: Unterminated string.", line: 1, column: 16},
		{name: "lex reports useful block string errors: \"\"\"contains invalid surrogate \\udead\"\"\"", in_: "\"\"\"contains invalid surrogate \xed\xba\xad\"\"\"", says: "Syntax Error: Invalid character within String: U+DEAD.", line: 1, column: 31},
		{name: "lex reports useful number errors: 00", in_: "00", says: "Syntax Error: Invalid number, unexpected digit after 0: \"0\".", line: 1, column: 2},
		{name: "lex reports useful number errors: 01", in_: "01", says: "Syntax Error: Invalid number, unexpected digit after 0: \"1\".", line: 1, column: 2},
		{name: "lex reports useful number errors: 01.23", in_: "01.23", says: "Syntax Error: Invalid number, unexpected digit after 0: \"1\".", line: 1, column: 2},
		{name: "lex reports useful number errors: +1", in_: "+1", says: "Syntax Error: Unexpected character: \"+\".", line: 1, column: 1},
		{name: "lex reports useful number errors: 1.", in_: "1.", says: "Syntax Error: Invalid number, expected digit but got: <EOF>.", line: 1, column: 3},
		{name: "lex reports useful number errors: 1e", in_: "1e", says: "Syntax Error: Invalid number, expected digit but got: <EOF>.", line: 1, column: 3},
		{name: "lex reports useful number errors: 1E", in_: "1E", says: "Syntax Error: Invalid number, expected digit but got: <EOF>.", line: 1, column: 3},
		{name: "lex reports useful number errors: 1.e1", in_: "1.e1", says: "Syntax Error: Invalid number, expected digit but got: \"e\".", line: 1, column: 3},
		{name: "lex reports useful number errors: .123", in_: ".123", says: "Syntax Error: Invalid number, expected digit before \".\", did you mean \"0.123\"?", line: 1, column: 1},
		{name: "lex reports useful number errors: 1.A", in_: "1.A", says: "Syntax Error: Invalid number, expected digit but got: \"A\".", line: 1, column: 3},
		{name: "lex reports useful number errors: -A", in_: "-A", says: "Syntax Error: Invalid number, expected digit but got: \"A\".", line: 1, column: 2},
		{name: "lex reports useful number errors: 1.0e", in_: "1.0e", says: "Syntax Error: Invalid number, expected digit but got: <EOF>.", line: 1, column: 5},
		{name: "lex reports useful number errors: 1.0eA", in_: "1.0eA", says: "Syntax Error: Invalid number, expected digit but got: \"A\".", line: 1, column: 5},
		{name: "lex reports useful number errors: 1.0e\"", in_: "1.0e\"", says: "Syntax Error: Invalid number, expected digit but got: '\"'.", line: 1, column: 5},
		{name: "lex reports useful number errors: 1.2e3e", in_: "1.2e3e", says: "Syntax Error: Invalid number, expected digit but got: \"e\".", line: 1, column: 6},
		{name: "lex reports useful number errors: 1.2e3.4", in_: "1.2e3.4", says: "Syntax Error: Invalid number, expected digit but got: \".\".", line: 1, column: 6},
		{name: "lex reports useful number errors: 1.23.4", in_: "1.23.4", says: "Syntax Error: Invalid number, expected digit but got: \".\".", line: 1, column: 5},
		{name: "lex does not allow name-start after a number: 0xF1", in_: "0xF1", says: "Syntax Error: Invalid number, expected digit but got: \"x\".", line: 1, column: 2},
		{name: "lex does not allow name-start after a number: 0b10", in_: "0b10", says: "Syntax Error: Invalid number, expected digit but got: \"b\".", line: 1, column: 2},
		{name: "lex does not allow name-start after a number: 123abc", in_: "123abc", says: "Syntax Error: Invalid number, expected digit but got: \"a\".", line: 1, column: 4},
		{name: "lex does not allow name-start after a number: 1_234", in_: "1_234", says: "Syntax Error: Invalid number, expected digit but got: \"_\".", line: 1, column: 2},
		{name: "lex does not allow name-start after a number: 1\u00DF", in_: "1\u00DF", says: "Syntax Error: Unexpected character: U+00DF.", line: 1, column: 2},
		{name: "lex does not allow name-start after a number: 1.23f", in_: "1.23f", says: "Syntax Error: Invalid number, expected digit but got: \"f\".", line: 1, column: 5},
		{name: "lex does not allow name-start after a number: 1.234_5", in_: "1.234_5", says: "Syntax Error: Invalid number, expected digit but got: \"_\".", line: 1, column: 6},
		{name: "lex reports useful unknown character error: ..", in_: "..", says: "Syntax Error: Unexpected \"..\", did you mean \"...\"?", line: 1, column: 1},
		{name: "lex reports useful unknown character error: ~", in_: "~", says: "Syntax Error: Unexpected character: \"~\".", line: 1, column: 1},
		{name: "lex reports useful unknown character error: \\x00", in_: "\u0000", says: "Syntax Error: Unexpected character: U+0000.", line: 1, column: 1},
		{name: "lex reports useful unknown character error: \\x08", in_: "\u0008", says: "Syntax Error: Unexpected character: U+0008.", line: 1, column: 1},
		{name: "lex reports useful unknown character error: \u00AA", in_: "\u00AA", says: "Syntax Error: Unexpected character: U+00AA.", line: 1, column: 1},
		{name: "lex reports useful unknown character error: \u0AAA", in_: "\u0AAA", says: "Syntax Error: Unexpected character: U+0AAA.", line: 1, column: 1},
		{name: "lex reports useful unknown character error: \u203B", in_: "\u203B", says: "Syntax Error: Unexpected character: U+203B.", line: 1, column: 1},
		{name: "lex reports useful unknown character error: \U0001F600", in_: "\U0001F600", says: "Syntax Error: Unexpected character: U+1F600.", line: 1, column: 1},
		{name: "lex reports useful unknown character error: \U0001F600 (2)", in_: "\U0001F600", says: "Syntax Error: Unexpected character: U+1F600.", line: 1, column: 1},
		{name: "lex reports useful unknown character error: \U00010000", in_: "\U00010000", says: "Syntax Error: Unexpected character: U+10000.", line: 1, column: 1},
		{name: "lex reports useful unknown character error: \\U0010ffff", in_: "\U0010FFFF", says: "Syntax Error: Unexpected character: U+10FFFF.", line: 1, column: 1},
		{name: "lex reports useful unknown character error: \\udead", in_: "\xed\xba\xad", says: "Syntax Error: Invalid character: U+DEAD.", line: 1, column: 1},
		{name: "lexes comments: # Invalid surrogate \\udead", in_: "# Invalid surrogate \xed\xba\xad", says: "Syntax Error: Invalid character: U+DEAD.", line: 1, column: 21},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lexer := language.NewLexer(language.NewSource(tt.in_))
			var err error
			for i := 0; i < 2 && err == nil; i++ {
				var token *language.Token
				token, err = lexer.Advance()
				if err == nil && token.Kind == language.TokenEOF {
					break
				}
			}
			if err == nil {
				t.Fatalf("%q lexed without complaint", tt.in_)
			}
			var syntax *language.SyntaxError
			said, line, column := err.Error(), 0, 0
			if errorsAs(err, &syntax) {
				said = syntax.Error()
				line, column = syntax.Location.Line, syntax.Location.Column
			}
			same := said == tt.says && line == tt.line && column == tt.column

			if why, listed := knownLexerDivergences[tt.name]; listed {
				if same {
					t.Errorf("this case now matches graphql-js; remove it from the known divergences (%s)", why)
				} else {
					t.Logf("known divergence: %s", why)
				}
				return
			}
			if said != tt.says {
				t.Errorf("said %q, want %q", said, tt.says)
			}
			if line != tt.line || column != tt.column {
				t.Errorf("at %d:%d, want %d:%d", line, column, tt.line, tt.column)
			}
		})
	}
}

// errorsAs is errors.As, named apart so that this file reads without an import
// whose only use is one call.
func errorsAs(err error, target **language.SyntaxError) bool {
	for err != nil {
		if syntax, is := err.(*language.SyntaxError); is {
			*target = syntax
			return true
		}
		unwrapped, can := err.(interface{ Unwrap() error })
		if !can {
			return false
		}
		err = unwrapped.Unwrap()
	}
	return false
}
