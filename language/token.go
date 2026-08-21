package language

// TokenKind names the kinds of token the lexer emits. Punctuation kinds are
// spelled as the punctuation itself so that error messages can quote them
// directly.
type TokenKind string

// The kinds of token. The two in angle brackets stand for the start and end of
// the document, which are not written but are what the parser begins and ends
// on; the rest are the punctuation and the four kinds of thing that carry a
// value.
const (
	TokenSOF         TokenKind = "<SOF>"
	TokenEOF         TokenKind = "<EOF>"
	TokenBang        TokenKind = "!"
	TokenDollar      TokenKind = "$"
	TokenAmp         TokenKind = "&"
	TokenParenL      TokenKind = "("
	TokenParenR      TokenKind = ")"
	TokenDot         TokenKind = "."
	TokenSpread      TokenKind = "..."
	TokenColon       TokenKind = ":"
	TokenEquals      TokenKind = "="
	TokenAt          TokenKind = "@"
	TokenBracketL    TokenKind = "["
	TokenBracketR    TokenKind = "]"
	TokenBraceL      TokenKind = "{"
	TokenPipe        TokenKind = "|"
	TokenBraceR      TokenKind = "}"
	TokenName        TokenKind = "Name"
	TokenInt         TokenKind = "Int"
	TokenFloat       TokenKind = "Float"
	TokenString      TokenKind = "String"
	TokenBlockString TokenKind = "BlockString"
	TokenComment     TokenKind = "Comment"
)

// String returns the token kind as it appears in error messages.
func (k TokenKind) String() string { return string(k) }

// Token is a single lexical token.
//
// Tokens form a doubly linked list covering the whole document, including the
// ignored tokens the parser skips. The list always begins with a token of kind
// [TokenSOF] and ends with one of kind [TokenEOF].
type Token struct {
	// Kind is the kind of token.
	Kind TokenKind
	// Start is the byte offset in the source body at which the token begins.
	Start int
	// End is the byte offset in the source body just past the token.
	End int
	// Line is the one-indexed line on which the token begins.
	Line int
	// Column is the one-indexed column, counted in Unicode code points, at
	// which the token begins.
	Column int
	// Value is the interpreted value for tokens that carry one: names,
	// numbers, strings and comments. It is empty for punctuation.
	Value string

	// Prev and Next link every token in the document, ignored tokens included.
	Prev *Token
	Next *Token
}

// String describes the token for use in error messages, quoting the value when
// the token carries one.
func (t *Token) String() string {
	if t == nil {
		return "<nil>"
	}
	if t.Value != "" {
		return string(t.Kind) + ` "` + t.Value + `"`
	}
	return string(t.Kind)
}
