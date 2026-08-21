package language_test

// Ported from graphql-js src/language/__tests__/blockString-fuzz.ts, which is
// the one property test in that suite: whatever a block string is printed as
// must lex back to the value it was printed from.
//
// Every string up to a given length is tried, over the characters that make
// block strings hard — a newline, a tab, a space, a quote, an 'a' and a
// backslash. graphql-js says the same: "Testing with length >7 is taking
// exponentially more time. However it is highly recommended to test with
// increased limit if you make any change."

import (
	"testing"

	"github.com/ikawaha/graphql/language"
)

// fuzzStrings calls yield with every string of the allowed characters up to
// maxLength, the empty string included.
func fuzzStrings(allowed []string, maxLength int, yield func(string)) {
	yield("")
	var build func(prefix string, left int)
	build = func(prefix string, left int) {
		if left == 0 {
			return
		}
		for _, c := range allowed {
			s := prefix + c
			yield(s)
			build(s, left-1)
		}
	}
	build("", maxLength)
}

func TestPortedPrintBlockString_Fuzz(t *testing.T) {
	if testing.Short() {
		t.Skip("the whole space of strings takes a while")
	}
	allowed := []string{"\n", "\t", " ", `"`, "a", `\`}
	const maxLength = 7

	tried := 0
	fuzzStrings(allowed, maxLength, func(s string) {
		tried++
		if !language.IsPrintableAsBlockString(s) {
			// A value that cannot be written as a block string must not
			// survive the round trip, or nothing would have been gained by
			// saying so.
			if lexBlockValue(t, language.PrintBlockString(s, false)) == s {
				t.Fatalf("%q is said not to be printable as a block string, yet it survived", s)
			}
			return
		}
		for _, minimize := range []bool{false, true} {
			printed := language.PrintBlockString(s, minimize)
			if got := lexBlockValue(t, printed); got != s {
				t.Fatalf("printing %q as %q lexed back as %q", s, printed, got)
			}
		}
	})
	t.Logf("%d strings tried", tried)
}

// lexBlockValue reads the one string token a printed block string is, and says
// what it holds.
func lexBlockValue(t *testing.T, printed string) string {
	t.Helper()
	lexer := language.NewLexer(language.NewSource(printed))
	token, err := lexer.Advance()
	if err != nil {
		t.Fatalf("lexing %q: %v", printed, err)
	}
	if token.Kind != language.TokenBlockString {
		t.Fatalf("lexing %q gave a %v, want a block string", printed, token.Kind)
	}
	next, err := lexer.Advance()
	if err != nil {
		t.Fatalf("lexing %q: %v", printed, err)
	}
	if next.Kind != language.TokenEOF {
		t.Fatalf("lexing %q left more than one token", printed)
	}
	return token.Value
}
