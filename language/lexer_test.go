package language

import (
	"errors"
	"strings"
	"testing"
)

// lexOne lexes body and returns its first non-ignored token.
func lexOne(t *testing.T, body string) *Token {
	t.Helper()
	tok, err := NewLexer(NewSource(body)).Advance()
	if err != nil {
		t.Fatalf("lexing %q: %v", body, err)
	}
	return tok
}

// lexErr lexes body and returns the syntax error it must produce.
func lexErr(t *testing.T, body string) *SyntaxError {
	t.Helper()
	lexer := NewLexer(NewSource(body))
	for {
		tok, err := lexer.Advance()
		if err != nil {
			var se *SyntaxError
			if !errors.As(err, &se) {
				t.Fatalf("lexing %q: error is %T, want *SyntaxError", body, err)
			}
			return se
		}
		if tok.Kind == TokenEOF {
			t.Fatalf("lexing %q succeeded, want a syntax error", body)
		}
	}
}

func checkToken(t *testing.T, tok *Token, kind TokenKind, start, end int, value string) {
	t.Helper()
	if tok.Kind != kind {
		t.Errorf("Kind = %v, want %v", tok.Kind, kind)
	}
	if tok.Start != start || tok.End != end {
		t.Errorf("span = [%d,%d), want [%d,%d)", tok.Start, tok.End, start, end)
	}
	if tok.Value != value {
		t.Errorf("Value = %q, want %q", tok.Value, value)
	}
}

func TestLexer_StartsWithSOF(t *testing.T) {
	lexer := NewLexer(NewSource("{ hello }"))
	if lexer.Token.Kind != TokenSOF {
		t.Errorf("initial Kind = %v, want %v", lexer.Token.Kind, TokenSOF)
	}
}

func TestLexer_Punctuation(t *testing.T) {
	tests := []struct {
		body string
		kind TokenKind
		end  int
	}{
		{"!", TokenBang, 1},
		{"$", TokenDollar, 1},
		{"&", TokenAmp, 1},
		{"(", TokenParenL, 1},
		{")", TokenParenR, 1},
		{"...", TokenSpread, 3},
		{":", TokenColon, 1},
		{"=", TokenEquals, 1},
		{"@", TokenAt, 1},
		{"[", TokenBracketL, 1},
		{"]", TokenBracketR, 1},
		{"{", TokenBraceL, 1},
		{"|", TokenPipe, 1},
		{"}", TokenBraceR, 1},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			checkToken(t, lexOne(t, tt.body), tt.kind, 0, tt.end, "")
		})
	}
}

func TestLexer_Names(t *testing.T) {
	tests := []struct {
		body  string
		value string
		end   int
	}{
		{"simple", "simple", 6},
		{"_underscore", "_underscore", 11},
		{"a1", "a1", 2},
		{"a_1_b", "a_1_b", 5},
		{"  name  ", "name", 6},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			tok := lexOne(t, tt.body)
			if tok.Kind != TokenName {
				t.Errorf("Kind = %v, want %v", tok.Kind, TokenName)
			}
			if tok.Value != tt.value {
				t.Errorf("Value = %q, want %q", tok.Value, tt.value)
			}
			if tok.End != tt.end {
				t.Errorf("End = %d, want %d", tok.End, tt.end)
			}
		})
	}
}

func TestLexer_SkipsIgnoredCharacters(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"spaces and tabs", "  \t name"},
		{"commas", ",,,name,,,"},
		{"line feeds", "\n\n name"},
		{"carriage returns", "\r\r name"},
		{"carriage return and line feed", "\r\n\r\n name"},
		{"comments", "# comment\nname"},
		{"byte order mark", "\ufeff name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := lexOne(t, tt.body)
			if tok.Kind != TokenName || tok.Value != "name" {
				t.Errorf("got %v %q, want Name \"name\"", tok.Kind, tok.Value)
			}
		})
	}
}

func TestLexer_Comment(t *testing.T) {
	lexer := NewLexer(NewSource("# a comment\nname"))
	tok, err := lexer.Advance()
	if err != nil {
		t.Fatalf("Advance() error = %v", err)
	}
	// Advance skips comments, so the comment is only reachable through the
	// token list that Lookahead builds.
	if tok.Kind != TokenName {
		t.Fatalf("Kind = %v, want %v", tok.Kind, TokenName)
	}
	comment := tok.Prev
	if comment == nil || comment.Kind != TokenComment {
		t.Fatalf("previous token = %v, want a comment", comment)
	}
	if comment.Value != " a comment" {
		t.Errorf("comment Value = %q, want %q", comment.Value, " a comment")
	}
}

func TestLexer_Numbers(t *testing.T) {
	tests := []struct {
		body  string
		kind  TokenKind
		value string
	}{
		{"0", TokenInt, "0"},
		{"1", TokenInt, "1"},
		{"-1", TokenInt, "-1"},
		{"9", TokenInt, "9"},
		{"42", TokenInt, "42"},
		{"-0", TokenInt, "-0"},
		{"4.123", TokenFloat, "4.123"},
		{"-4.123", TokenFloat, "-4.123"},
		{"0.123", TokenFloat, "0.123"},
		{"123e4", TokenFloat, "123e4"},
		{"123E4", TokenFloat, "123E4"},
		{"123e-4", TokenFloat, "123e-4"},
		{"123e+4", TokenFloat, "123e+4"},
		{"-1.123e4", TokenFloat, "-1.123e4"},
		{"-1.123e-4", TokenFloat, "-1.123e-4"},
		{"-1.123e4567", TokenFloat, "-1.123e4567"},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			tok := lexOne(t, tt.body)
			if tok.Kind != tt.kind {
				t.Errorf("Kind = %v, want %v", tok.Kind, tt.kind)
			}
			if tok.Value != tt.value {
				t.Errorf("Value = %q, want %q", tok.Value, tt.value)
			}
		})
	}
}

func TestLexer_NumberErrors(t *testing.T) {
	tests := []struct {
		body string
		want string
	}{
		{"00", "Invalid number, unexpected digit after 0"},
		{"01", "Invalid number, unexpected digit after 0"},
		{"-", "Invalid number, expected digit but got: <EOF>"},
		{"+1", "Unexpected character"},
		{"1.", "Invalid number, expected digit but got: <EOF>"},
		{".123", `Invalid number, expected digit before ".", did you mean "0.123"?`},
		{"1.A", `Invalid number, expected digit but got: "A"`},
		{"-A", `Invalid number, expected digit but got: "A"`},
		{"1.0e", "Invalid number, expected digit but got: <EOF>"},
		{"1.0eA", `Invalid number, expected digit but got: "A"`},
		{"1.2e3e", `Invalid number, expected digit but got: "e"`},
		{"1.2e3.4", `Invalid number, expected digit but got: "."`},
		{"1.23.4", `Invalid number, expected digit but got: "."`},
		{"0xF1", `Invalid number, expected digit but got: "x"`},
		{"1beta", `Invalid number, expected digit but got: "b"`},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			err := lexErr(t, tt.body)
			if !strings.Contains(err.Description, tt.want) {
				t.Errorf("description = %q, want it to contain %q", err.Description, tt.want)
			}
		})
	}
}

func TestLexer_Strings(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		value string
	}{
		{"empty", `""`, ""},
		{"simple", `"simple"`, "simple"},
		{"white space", `" white space "`, " white space "},
		{"quote escape", `"quote \""`, `quote "`},
		{"escaped characters", `"\\ \/ \b \f \n \r \t"`, "\\ / \b \f \n \r \t"},
		{"slashes", `"slashes \\ \/"`, `slashes \ /`},
		{"unicode escape", `"\u1234"`, "\u1234"},
		{"variable width unicode escape", `"\u{1F600}"`, "\U0001F600"},
		{"minimal variable width escape", `"\u{0}"`, "\x00"},
		{"maximal variable width escape", `"\u{10FFFF}"`, "\U0010FFFF"},
		{"non-ASCII passes through", `"日本語"`, "日本語"},
		{"supplementary character passes through", "\"\U0001F600\"", "\U0001F600"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := lexOne(t, tt.body)
			if tok.Kind != TokenString {
				t.Errorf("Kind = %v, want %v", tok.Kind, TokenString)
			}
			if tok.Value != tt.value {
				t.Errorf("Value = %q, want %q", tok.Value, tt.value)
			}
		})
	}
}

// Two four-digit escapes naming a surrogate pair stand for the supplementary
// character the pair encodes. JavaScript keeps the pair as two UTF-16 code
// units; Go stores the single code point, which is the same character.
func TestLexer_SurrogatePairEscapes(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		value string
	}{
		{"emoji", `"\uD83D\uDE00"`, "\U0001F600"},
		{"lowest supplementary", `"\uD800\uDC00"`, "\U00010000"},
		{"highest supplementary", `"\uDBFF\uDFFF"`, "\U0010FFFF"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := lexOne(t, tt.body)
			if tok.Value != tt.value {
				t.Errorf("Value = %q, want %q", tok.Value, tt.value)
			}
		})
	}
}

func TestLexer_StringErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"unterminated", `"`, "Unterminated string."},
		{"unterminated with text", `"no end quote`, "Unterminated string."},
		{"line feed in string", "\"multi\nline\"", "Unterminated string."},
		{"carriage return in string", "\"multi\rline\"", "Unterminated string."},
		{"bad character escape", `"bad \z esc"`, `Invalid character escape sequence: "\z".`},
		{"escaped line feed", `"bad \` + "\n" + ` esc"`, "Invalid character escape sequence"},
		{"hex escape", `"bad \x esc"`, `Invalid character escape sequence: "\x".`},
		{"too short unicode escape", `"bad \u1 esc"`, `Invalid Unicode escape sequence: "\u1 es".`},
		{"non-hex unicode escape", `"bad \u0XX1 esc"`, `Invalid Unicode escape sequence: "\u0XX1".`},
		{"empty variable width escape", `"bad \u{} esc"`, `Invalid Unicode escape sequence: "\u{}".`},
		{"unterminated variable width escape", `"bad \u{1 esc"`, "Invalid Unicode escape sequence"},
		{"out of range variable width escape", `"bad \u{110000} esc"`, "Invalid Unicode escape sequence"},
		{"single quote", `'single quotes'`, "Unexpected single quote character"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := lexErr(t, tt.body)
			if !strings.Contains(err.Description, tt.want) {
				t.Errorf("description = %q, want it to contain %q", err.Description, tt.want)
			}
		})
	}
}

// A surrogate code point is not a Unicode scalar value, so the specification
// forbids it in source text. UTF-8 cannot encode one, but a Go string is a
// byte string and can still carry the three-byte form UTF-8 reserves, which is
// how these inputs reach the lexer at all.
const (
	loneSurrogateDEAD = "\xed\xba\xad" // U+DEAD
	loneSurrogateD800 = "\xed\xa0\x80" // U+D800
)

func TestLexer_RejectsLoneSurrogatesInSource(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"bare", loneSurrogateDEAD, "Invalid character: U+DEAD."},
		{"inside a string", `"bad ` + loneSurrogateDEAD + `"`, "Invalid character within String: U+DEAD."},
		{
			name: "inside a block string",
			body: `"""bad ` + loneSurrogateDEAD + `"""`,
			want: "Invalid character within String: U+DEAD.",
		},
		{
			name: "leading surrogate inside a string",
			body: `"bad ` + loneSurrogateD800 + `"`,
			want: "Invalid character within String: U+D800.",
		},
		{
			name: "two leading surrogates are still not a pair",
			body: `"bad ` + loneSurrogateD800 + loneSurrogateD800 + `"`,
			want: "Invalid character within String: U+D800.",
		},
		{
			name: "an escaped pair split by a literal surrogate",
			body: `"bad ` + loneSurrogateD800 + `\uDE00 esc"`,
			want: "Invalid character within String: U+D800.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := lexErr(t, tt.body)
			if err.Description != tt.want {
				t.Errorf("description = %q, want %q", err.Description, tt.want)
			}
		})
	}
}

func TestLexer_RejectsSurrogateEscapes(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"lone trailing surrogate escape", `"bad \uDEAD esc"`, `Invalid Unicode escape sequence: "\uDEAD".`},
		{"lone leading surrogate escape", `"bad \uD800 esc"`, `Invalid Unicode escape sequence: "\uD800".`},
		{"surrogate in the brace form", `"bad \u{DEAD} esc"`, `Invalid Unicode escape sequence: "\u{DEAD}".`},
		{"braces cannot form a pair", `"bad \u{D83D}\u{DE00} esc"`, `Invalid Unicode escape sequence: "\u{D83D}".`},
		{"leading surrogate not followed by a trailing one", `"bad \uD800\uD800 esc"`, `Invalid Unicode escape sequence: "\uD800".`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := lexErr(t, tt.body)
			if err.Description != tt.want {
				t.Errorf("description = %q, want %q", err.Description, tt.want)
			}
		})
	}
}

// Invalid UTF-8 that is not a surrogate is rejected the same way.
func TestLexer_RejectsInvalidUTF8(t *testing.T) {
	err := lexErr(t, "\xff")
	if !strings.HasPrefix(err.Description, "Invalid character:") {
		t.Errorf("description = %q, want it to start with %q", err.Description, "Invalid character:")
	}
}

func TestLexer_BlockStrings(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		value string
	}{
		{"empty", `""""""`, ""},
		{"simple", `"""simple"""`, "simple"},
		{"white space", `""" white space """`, " white space "},
		{"contains quote", `"""contains " quote"""`, `contains " quote`},
		{"contains triple quote", `"""contains \""" triple quote"""`, `contains """ triple quote`},
		{"multiple lines", "\"\"\"multi\nline\"\"\"", "multi\nline"},
		{"normalizes carriage returns", "\"\"\"multi\r\nline\r\nnormalized\"\"\"", "multi\nline\nnormalized"},
		{"does not interpret escapes", `"""unescaped \n\r\b\t\f\u1234"""`, `unescaped \n\r\b\t\f\u1234`},
		{"dedents", "\"\"\"\n    spans\n      multiple\n        lines\n    \"\"\"", "spans\n  multiple\n    lines"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := lexOne(t, tt.body)
			if tok.Kind != TokenBlockString {
				t.Errorf("Kind = %v, want %v", tok.Kind, TokenBlockString)
			}
			if tok.Value != tt.value {
				t.Errorf("Value = %q, want %q", tok.Value, tt.value)
			}
		})
	}
}

func TestLexer_BlockStringErrors(t *testing.T) {
	for _, body := range []string{`"""`, `"""no end quote`} {
		t.Run(body, func(t *testing.T) {
			err := lexErr(t, body)
			if err.Description != "Unterminated string." {
				t.Errorf("description = %q, want %q", err.Description, "Unterminated string.")
			}
		})
	}
}

func TestLexer_LineAndColumn(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantLine   int
		wantColumn int
	}{
		{"first token", "name", 1, 1},
		{"after spaces", "   name", 1, 4},
		{"second line", "\nname", 2, 1},
		{"third line", "\n\n  name", 3, 3},
		{"after a carriage return", "\rname", 2, 1},
		{"after a carriage return and line feed", "\r\nname", 2, 1},
		{"after a comment", "# c\nname", 2, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := lexOne(t, tt.body)
			if tok.Line != tt.wantLine || tok.Column != tt.wantColumn {
				t.Errorf("line:column = %d:%d, want %d:%d",
					tok.Line, tok.Column, tt.wantLine, tt.wantColumn)
			}
		})
	}
}

// Columns count code points, so a multi-byte character before a token shifts
// the column by one rather than by its byte length.
func TestLexer_ColumnCountsCodePoints(t *testing.T) {
	// Each character of the comment body is three bytes in UTF-8.
	tok := lexOne(t, "#あいう\nname")
	if tok.Line != 2 || tok.Column != 1 {
		t.Errorf("line:column = %d:%d, want 2:1", tok.Line, tok.Column)
	}

	// A block string holding multi-byte text must still leave the following
	// token on the right line.
	lexer := NewLexer(NewSource("\"\"\"あ\nい\"\"\"\nname"))
	if _, err := lexer.Advance(); err != nil {
		t.Fatalf("Advance() error = %v", err)
	}
	next, err := lexer.Advance()
	if err != nil {
		t.Fatalf("Advance() error = %v", err)
	}
	if next.Line != 3 || next.Column != 1 {
		t.Errorf("token after a block string at %d:%d, want 3:1", next.Line, next.Column)
	}
}

func TestLexer_MultilineTokenPositions(t *testing.T) {
	lexer := NewLexer(NewSource("{\n  field\n}"))
	want := []struct {
		kind   TokenKind
		line   int
		column int
	}{
		{TokenBraceL, 1, 1},
		{TokenName, 2, 3},
		{TokenBraceR, 3, 1},
		{TokenEOF, 3, 2},
	}
	for i, w := range want {
		tok, err := lexer.Advance()
		if err != nil {
			t.Fatalf("token %d: Advance() error = %v", i, err)
		}
		if tok.Kind != w.kind || tok.Line != w.line || tok.Column != w.column {
			t.Errorf("token %d = %v at %d:%d, want %v at %d:%d",
				i, tok.Kind, tok.Line, tok.Column, w.kind, w.line, w.column)
		}
	}
}

func TestLexer_LookaheadDoesNotAdvance(t *testing.T) {
	lexer := NewLexer(NewSource("{ hello }"))
	ahead, err := lexer.Lookahead()
	if err != nil {
		t.Fatalf("Lookahead() error = %v", err)
	}
	if ahead.Kind != TokenBraceL {
		t.Errorf("Lookahead() kind = %v, want %v", ahead.Kind, TokenBraceL)
	}
	if lexer.Token.Kind != TokenSOF {
		t.Errorf("Lookahead() moved the cursor to %v", lexer.Token.Kind)
	}

	advanced, err := lexer.Advance()
	if err != nil {
		t.Fatalf("Advance() error = %v", err)
	}
	if advanced != ahead {
		t.Error("Advance() returned a different token than Lookahead(), so the lookahead was not reused")
	}
}

func TestLexer_TokenListIsLinked(t *testing.T) {
	lexer := NewLexer(NewSource("{ a }"))
	var tokens []*Token
	for {
		tok, err := lexer.Advance()
		if err != nil {
			t.Fatalf("Advance() error = %v", err)
		}
		tokens = append(tokens, tok)
		if tok.Kind == TokenEOF {
			break
		}
	}
	for i := 1; i < len(tokens); i++ {
		if tokens[i].Prev != tokens[i-1] {
			t.Errorf("token %d Prev is not the previous token", i)
		}
		if tokens[i-1].Next != tokens[i] {
			t.Errorf("token %d Next is not the following token", i-1)
		}
	}
}

func TestLexer_StaysAtEOF(t *testing.T) {
	lexer := NewLexer(NewSource(""))
	first, err := lexer.Advance()
	if err != nil {
		t.Fatalf("Advance() error = %v", err)
	}
	if first.Kind != TokenEOF {
		t.Fatalf("Kind = %v, want %v", first.Kind, TokenEOF)
	}
	for i := 0; i < 3; i++ {
		again, err := lexer.Advance()
		if err != nil {
			t.Fatalf("Advance() error = %v", err)
		}
		if again != first {
			t.Error("Advance() past the end returned a different token")
		}
	}
}

func TestSyntaxError_ReportsLocation(t *testing.T) {
	err := lexErr(t, "{\n  ?\n}")
	if err.Location.Line != 2 || err.Location.Column != 3 {
		t.Errorf("location = %v, want 2:3", err.Location)
	}
	if got, want := err.Error(), `Syntax Error: Unexpected character: "?".`; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// A source embedded in a larger file reports locations in the coordinates of
// that file. Only the first line is shifted horizontally, since later lines
// begin at column 1 wherever the document was embedded.
func TestSyntaxError_AppliesLocationOffset(t *testing.T) {
	src := NewSource("?\n?", SourceLocationOffset(10, 5))
	lexer := NewLexer(src)
	_, err := lexer.Advance()
	var se *SyntaxError
	if !errors.As(err, &se) {
		t.Fatalf("error = %v, want *SyntaxError", err)
	}
	if se.Location.Line != 10 || se.Location.Column != 5 {
		t.Errorf("first line location = %v, want 10:5", se.Location)
	}

	lexer2 := NewLexer(NewSource("a\n?", SourceLocationOffset(10, 5)))
	if _, err := lexer2.Advance(); err != nil {
		t.Fatalf("Advance() error = %v", err)
	}
	_, err = lexer2.Advance()
	if !errors.As(err, &se) {
		t.Fatalf("error = %v, want *SyntaxError", err)
	}
	if se.Location.Line != 11 || se.Location.Column != 1 {
		t.Errorf("second line location = %v, want 11:1", se.Location)
	}
}

func TestIsPunctuatorTokenKind(t *testing.T) {
	punctuation := []TokenKind{
		TokenBang, TokenDollar, TokenAmp, TokenParenL, TokenParenR, TokenDot,
		TokenSpread, TokenColon, TokenEquals, TokenAt, TokenBracketL,
		TokenBracketR, TokenBraceL, TokenPipe, TokenBraceR,
	}
	for _, k := range punctuation {
		if !IsPunctuatorToken(k) {
			t.Errorf("IsPunctuatorToken(%v) = false, want true", k)
		}
	}
	for _, k := range []TokenKind{TokenSOF, TokenEOF, TokenName, TokenInt,
		TokenFloat, TokenString, TokenBlockString, TokenComment} {
		if IsPunctuatorToken(k) {
			t.Errorf("IsPunctuatorToken(%v) = true, want false", k)
		}
	}
}

func TestLexer_SpreadErrors(t *testing.T) {
	err := lexErr(t, "..")
	if want := `Unexpected "..", did you mean "..."?`; err.Description != want {
		t.Errorf("description = %q, want %q", err.Description, want)
	}
	err = lexErr(t, ".")
	if want := `Unexpected character: ".".`; err.Description != want {
		t.Errorf("description = %q, want %q", err.Description, want)
	}
}

// The error's Location is in the coordinates of the enclosing file, while
// PrintSourceLocation works in the coordinates of the source body. Rendering
// through the error's own method applies the offset exactly once.
func TestSyntaxError_PrintLocation(t *testing.T) {
	src := NewSource("?", SourceName("f.graphql"), SourceLocationOffset(10, 5))
	_, err := NewLexer(src).Advance()
	var se *SyntaxError
	if !errors.As(err, &se) {
		t.Fatalf("error = %v, want *SyntaxError", err)
	}

	want := "f.graphql:10:5\n10 |     ?\n   |     ^"
	if got := se.PrintLocation(); got != want {
		t.Errorf("PrintLocation() = %q, want %q", got, want)
	}
	if se.Location != (SourceLocation{Line: 10, Column: 5}) {
		t.Errorf("Location = %v, want 10:5", se.Location)
	}

	var absent *SyntaxError
	if got := absent.PrintLocation(); got != "" {
		t.Errorf("PrintLocation() on nil = %q, want empty", got)
	}
}
