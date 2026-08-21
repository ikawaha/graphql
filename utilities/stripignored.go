package utilities

import (
	"strings"

	"github.com/ikawaha/graphql/language"
)

// StripIgnoredCharacters removes everything from a document that carries no
// meaning: whitespace, line breaks, commas and comments.
//
// What comes back parses to the same thing and is a good deal shorter, which
// is worth having when a document is sent over a network or used as a cache
// key. Two documents that differ only in layout strip to the same text, so
// this is also how to tell whether they are the same request.
//
// A block string is rewritten to the shortest form with the same value, which
// is the one place the text inside a token changes.
//
// The document is lexed rather than parsed, so this works on anything that is
// made of valid tokens, including a fragment of a document.
func StripIgnoredCharacters(body string) (string, error) {
	source := language.NewSource(body)
	lexer := language.NewLexer(source)

	var b strings.Builder
	b.Grow(len(body))

	var previous *language.Token
	for {
		token, err := lexer.Advance()
		if err != nil {
			return "", err
		}
		if token.Kind == language.TokenEOF {
			break
		}

		text := tokenText(source, token)
		if token.Kind == language.TokenBlockString {
			text = language.PrintBlockString(token.Value, true)
		}

		// Two tokens need something between them only where running them
		// together would lex as one.
		if previous != nil && needsSpace(previous, token) {
			b.WriteString(" ")
		}
		b.WriteString(text)
		previous = token
	}
	return b.String(), nil
}

// tokenText returns a token as it was written.
func tokenText(source *language.Source, token *language.Token) string {
	return source.Body[token.Start:token.End]
}

// needsSpace reports whether two tokens would read as something else if
// written together.
//
// Punctuation is self-delimiting, so nothing is needed after it — except
// before a spread, since a name or a number running into one would be read as
// a single token. Anything that is not punctuation does need a space before
// another such token: two names would run together, and two strings would
// begin a block string.
func needsSpace(previous, next *language.Token) bool {
	if language.IsPunctuatorToken(previous.Kind) {
		return false
	}
	return !language.IsPunctuatorToken(next.Kind) || next.Kind == language.TokenSpread
}
