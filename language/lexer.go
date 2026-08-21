package language

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Lexer turns GraphQL source text into a stream of tokens.
//
// A Lexer is a stateful cursor: each call to Advance returns the next token
// that is not ignored. Once the end of the source is reached it keeps
// returning the same token of kind [TokenEOF].
//
// # Offsets and columns
//
// Positions are byte offsets into the source body, which is the natural unit
// for a Go string. Columns count Unicode code points, so a multi-byte
// character advances the column by one. graphql-js counts UTF-16 code units
// for both; the three agree for ASCII, which is what GraphQL source almost
// always is outside string literals and comments.
type Lexer struct {
	// Source is the document being read.
	Source *Source
	// Token is the current token, initially a token of kind [TokenSOF].
	Token *Token
	// LastToken is the token before Token, which the parser uses to report
	// the end of the previous construct.
	LastToken *Token

	// line is the one-indexed line the cursor is on.
	line int
	// lineStart is the byte offset at which the current line begins.
	lineStart int
	// colAnchor and colAnchorCol memoize the column of a byte offset on the
	// current line so that columnAt does not rescan the line from its start
	// for every token. Token starts only move forward within a line, which
	// keeps the total work linear in the length of the source.
	colAnchor    int
	colAnchorCol int

	// coordinate restricts the lexer to the schema coordinate grammar, which
	// is a much smaller language: a handful of punctuation marks and names,
	// with no ignored characters at all.
	coordinate bool
}

// NewLexer returns a Lexer positioned before the first token of source.
func NewLexer(source *Source) *Lexer {
	sof := &Token{Kind: TokenSOF}
	return &Lexer{
		Source:       source,
		Token:        sof,
		LastToken:    sof,
		line:         1,
		lineStart:    0,
		colAnchor:    0,
		colAnchorCol: 1,
	}
}

// newSchemaCoordinateLexer returns a lexer for a schema coordinate such as
// "Query.field(arg:)".
//
// A coordinate is written in a restricted grammar that the document lexer
// cannot read: it needs a lone dot, which in a document is only ever part of
// the spread punctuation. It also has no ignored characters, so a space
// anywhere in a coordinate is an error rather than something to skip.
func newSchemaCoordinateLexer(source *Source) *Lexer {
	l := NewLexer(source)
	l.coordinate = true
	return l
}

// Advance moves to the next token that is not ignored and returns it.
func (l *Lexer) Advance() (*Token, error) {
	tok, err := l.Lookahead()
	if err != nil {
		return nil, err
	}
	l.LastToken = l.Token
	l.Token = tok
	return tok, nil
}

// Lookahead returns the next token that is not ignored without moving the
// cursor. Tokens it reads are linked into the token list, so looking ahead and
// then advancing does not lex the same text twice.
func (l *Lexer) Lookahead() (*Token, error) {
	tok := l.Token
	if tok.Kind == TokenEOF {
		return tok, nil
	}
	for {
		if tok.Next != nil {
			tok = tok.Next
		} else {
			next, err := l.readNextToken(tok.End)
			if err != nil {
				return nil, err
			}
			tok.Next = next
			next.Prev = tok
			tok = next
		}
		if tok.Kind != TokenComment {
			return tok, nil
		}
	}
}

// IsPunctuatorToken reports whether a token kind is one of the punctuation
// tokens, as opposed to a name, a number, a string or a comment.
//
// The distinction matters to anything writing a document back out: punctuation
// is self-delimiting, so nothing need be written between it and what follows,
// while two names run together would read as one.
func IsPunctuatorToken(kind TokenKind) bool {
	switch kind {
	case TokenBang, TokenDollar, TokenAmp, TokenParenL, TokenParenR,
		TokenDot, TokenSpread, TokenColon, TokenEquals, TokenAt,
		TokenBracketL, TokenBracketR, TokenBraceL, TokenPipe, TokenBraceR:
		return true
	default:
		return false
	}
}

// columnAt returns the one-indexed column of a byte offset on the current
// line, counting Unicode code points.
func (l *Lexer) columnAt(pos int) int {
	body := l.Source.Body
	if pos < l.colAnchor {
		// Scanning moved backwards, which only happens if a caller asks about
		// an earlier offset. Recount from the start of the line.
		return utf8.RuneCountInString(body[l.lineStart:pos]) + 1
	}
	col := l.colAnchorCol + utf8.RuneCountInString(body[l.colAnchor:pos])
	l.colAnchor, l.colAnchorCol = pos, col
	return col
}

// startLine records that a new line begins at the given byte offset.
func (l *Lexer) startLine(pos int) {
	l.line++
	l.lineStart = pos
	l.colAnchor, l.colAnchorCol = pos, 1
}

// newToken builds a token, filling in the line and column of its start.
func (l *Lexer) newToken(kind TokenKind, start, end int, value string) *Token {
	return &Token{
		Kind:   kind,
		Start:  start,
		End:    end,
		Line:   l.line,
		Column: l.columnAt(start),
		Value:  value,
	}
}

func (l *Lexer) errorf(position int, format string, args ...any) error {
	return newSyntaxError(l.Source, position, format, args...)
}

// decodeSourceChar decodes the source character at a byte offset.
//
// ok reports whether the bytes form a Unicode scalar value, which the GraphQL
// specification requires of source text. It is false both for bytes that are
// not valid UTF-8 at all and for a surrogate code point encoded in the
// three-byte form UTF-8 forbids; in the latter case r is the surrogate itself
// so that an error message can name it, matching what graphql-js reports for
// the same input. At the end of the source, size is zero.
func decodeSourceChar(body string, pos int) (r rune, size int, ok bool) {
	if pos >= len(body) {
		return 0, 0, false
	}
	b := body[pos]
	if b < utf8.RuneSelf {
		return rune(b), 1, true
	}
	// A surrogate encoded as three bytes: ED A0 80 through ED BF BF.
	if b == 0xed && pos+2 < len(body) &&
		body[pos+1] >= 0xa0 && body[pos+1] <= 0xbf &&
		body[pos+2] >= 0x80 && body[pos+2] <= 0xbf {
		r = rune(b&0x0f)<<12 | rune(body[pos+1]&0x3f)<<6 | rune(body[pos+2]&0x3f)
		return r, 3, false
	}
	r, size = utf8.DecodeRuneInString(body[pos:])
	if r == utf8.RuneError && size <= 1 {
		return rune(b), 1, false
	}
	return r, size, true
}

// describeCharAt renders the character at a byte offset for an error message.
// Printable ASCII is quoted, anything else is named by code point.
func (l *Lexer) describeCharAt(pos int) string {
	r, size, _ := decodeSourceChar(l.Source.Body, pos)
	if size == 0 {
		return string(TokenEOF)
	}
	if r >= 0x20 && r <= 0x7e {
		if r == '"' {
			return `'"'`
		}
		return `"` + string(r) + `"`
	}
	h := strings.ToUpper(strconv.FormatInt(int64(r), 16))
	for len(h) < 4 {
		h = "0" + h
	}
	return "U+" + h
}

// readNextToken reads the token beginning at or after start, skipping ignored
// characters along the way.
func (l *Lexer) readNextToken(start int) (*Token, error) {
	if l.coordinate {
		return l.readCoordinateToken(start)
	}
	body := l.Source.Body
	pos := start

	for pos < len(body) {
		c := body[pos]
		switch c {
		case '\t', ' ', ',':
			pos++
			continue
		case '\n':
			pos++
			l.startLine(pos)
			continue
		case '\r':
			if pos+1 < len(body) && body[pos+1] == '\n' {
				pos += 2
			} else {
				pos++
			}
			l.startLine(pos)
			continue
		case '#':
			return l.readComment(pos)
		case '!':
			return l.newToken(TokenBang, pos, pos+1, ""), nil
		case '$':
			return l.newToken(TokenDollar, pos, pos+1, ""), nil
		case '&':
			return l.newToken(TokenAmp, pos, pos+1, ""), nil
		case '(':
			return l.newToken(TokenParenL, pos, pos+1, ""), nil
		case ')':
			return l.newToken(TokenParenR, pos, pos+1, ""), nil
		case ':':
			return l.newToken(TokenColon, pos, pos+1, ""), nil
		case '=':
			return l.newToken(TokenEquals, pos, pos+1, ""), nil
		case '@':
			return l.newToken(TokenAt, pos, pos+1, ""), nil
		case '[':
			return l.newToken(TokenBracketL, pos, pos+1, ""), nil
		case ']':
			return l.newToken(TokenBracketR, pos, pos+1, ""), nil
		case '{':
			return l.newToken(TokenBraceL, pos, pos+1, ""), nil
		case '|':
			return l.newToken(TokenPipe, pos, pos+1, ""), nil
		case '}':
			return l.newToken(TokenBraceR, pos, pos+1, ""), nil
		case '.':
			if pos+2 < len(body) && body[pos+1] == '.' && body[pos+2] == '.' {
				return l.newToken(TokenSpread, pos, pos+3, ""), nil
			}
			if pos+1 < len(body) && body[pos+1] == '.' {
				return nil, l.errorf(pos, `Unexpected "..", did you mean "..."?`)
			}
			if pos+1 < len(body) && isDigit(body[pos+1]) {
				end, err := l.readDigits(pos + 1)
				if err != nil {
					return nil, err
				}
				return nil, l.errorf(pos,
					`Invalid number, expected digit before ".", did you mean "0.%s"?`,
					body[pos+1:end])
			}
			// Fall through to the generic unexpected-character report.
		case '"':
			if pos+2 < len(body) && body[pos+1] == '"' && body[pos+2] == '"' {
				return l.readBlockString(pos)
			}
			return l.readString(pos)
		case 0xef:
			// A byte order mark, U+FEFF, is ignored.
			if pos+2 < len(body) && body[pos+1] == 0xbb && body[pos+2] == 0xbf {
				pos += 3
				continue
			}
		}

		if isDigit(c) || c == '-' {
			return l.readNumber(pos)
		}
		if isNameStart(c) {
			return l.readName(pos)
		}

		if c == '\'' {
			return nil, l.errorf(pos,
				`Unexpected single quote character ('), did you mean to use a double quote (")?`)
		}
		if _, _, ok := decodeSourceChar(body, pos); ok {
			return nil, l.errorf(pos, "Unexpected character: %s.", l.describeCharAt(pos))
		}
		return nil, l.errorf(pos, "Invalid character: %s.", l.describeCharAt(pos))
	}

	return l.newToken(TokenEOF, len(body), len(body), ""), nil
}

// readCoordinateToken reads the next token of a schema coordinate.
func (l *Lexer) readCoordinateToken(pos int) (*Token, error) {
	body := l.Source.Body
	if pos >= len(body) {
		return l.newToken(TokenEOF, len(body), len(body), ""), nil
	}
	switch c := body[pos]; c {
	case '.':
		return l.newToken(TokenDot, pos, pos+1, ""), nil
	case '(':
		return l.newToken(TokenParenL, pos, pos+1, ""), nil
	case ')':
		return l.newToken(TokenParenR, pos, pos+1, ""), nil
	case ':':
		return l.newToken(TokenColon, pos, pos+1, ""), nil
	case '@':
		return l.newToken(TokenAt, pos, pos+1, ""), nil
	default:
		if isNameStart(c) {
			return l.readName(pos)
		}
	}
	return nil, l.errorf(pos, "Invalid character: %s.", l.describeCharAt(pos))
}

// readComment reads a comment, which runs to the end of the line.
func (l *Lexer) readComment(start int) (*Token, error) {
	body := l.Source.Body
	pos := start + 1
	for pos < len(body) {
		if body[pos] == '\n' || body[pos] == '\r' {
			break
		}
		_, size, ok := decodeSourceChar(body, pos)
		if !ok {
			break
		}
		pos += size
	}
	return l.newToken(TokenComment, start, pos, body[start+1:pos]), nil
}

// readName reads a Name token.
func (l *Lexer) readName(start int) (*Token, error) {
	body := l.Source.Body
	pos := start + 1
	for pos < len(body) && isNameContinue(body[pos]) {
		pos++
	}
	return l.newToken(TokenName, start, pos, body[start:pos]), nil
}

// byteAt returns the byte at a position, or 0 past the end of the source. Zero
// is safe as a sentinel because a NUL byte is never valid where these checks
// are made.
func (l *Lexer) byteAt(pos int) byte {
	if pos < 0 || pos >= len(l.Source.Body) {
		return 0
	}
	return l.Source.Body[pos]
}

// readDigits returns the offset just past a run of one or more digits.
func (l *Lexer) readDigits(start int) (int, error) {
	if !isDigit(l.byteAt(start)) {
		return 0, l.errorf(start, "Invalid number, expected digit but got: %s.", l.describeCharAt(start))
	}
	pos := start + 1
	for isDigit(l.byteAt(pos)) {
		pos++
	}
	return pos, nil
}

// readNumber reads an Int or a Float token, deciding between them by whether a
// fractional or exponent part is present.
func (l *Lexer) readNumber(start int) (*Token, error) {
	body := l.Source.Body
	pos := start
	isFloat := false

	if l.byteAt(pos) == '-' {
		pos++
	}

	if l.byteAt(pos) == '0' {
		pos++
		if isDigit(l.byteAt(pos)) {
			return nil, l.errorf(pos, "Invalid number, unexpected digit after 0: %s.", l.describeCharAt(pos))
		}
	} else {
		var err error
		if pos, err = l.readDigits(pos); err != nil {
			return nil, err
		}
	}

	if l.byteAt(pos) == '.' {
		isFloat = true
		var err error
		if pos, err = l.readDigits(pos + 1); err != nil {
			return nil, err
		}
	}

	if c := l.byteAt(pos); c == 'e' || c == 'E' {
		isFloat = true
		pos++
		if c := l.byteAt(pos); c == '+' || c == '-' {
			pos++
		}
		var err error
		if pos, err = l.readDigits(pos); err != nil {
			return nil, err
		}
	}

	// A number may not run straight into a full stop or the start of a name.
	if c := l.byteAt(pos); c == '.' || isNameStart(c) {
		return nil, l.errorf(pos, "Invalid number, expected digit but got: %s.", l.describeCharAt(pos))
	}

	kind := TokenInt
	if isFloat {
		kind = TokenFloat
	}
	return l.newToken(kind, start, pos, body[start:pos]), nil
}

// readString reads a single-quoted string token, resolving escape sequences.
func (l *Lexer) readString(start int) (*Token, error) {
	body := l.Source.Body
	pos := start + 1
	chunkStart := pos
	var value strings.Builder

	for pos < len(body) {
		switch body[pos] {
		case '"':
			value.WriteString(body[chunkStart:pos])
			return l.newToken(TokenString, start, pos+1, value.String()), nil
		case '\\':
			value.WriteString(body[chunkStart:pos])
			text, size, err := l.readEscape(pos)
			if err != nil {
				return nil, err
			}
			value.WriteString(text)
			pos += size
			chunkStart = pos
			continue
		case '\n', '\r':
			return nil, l.errorf(pos, "Unterminated string.")
		}
		_, size, ok := decodeSourceChar(body, pos)
		if !ok {
			return nil, l.errorf(pos, "Invalid character within String: %s.", l.describeCharAt(pos))
		}
		pos += size
	}
	return nil, l.errorf(pos, "Unterminated string.")
}

// readEscape reads one escape sequence beginning at a backslash and returns
// the text it stands for together with the number of bytes consumed.
func (l *Lexer) readEscape(pos int) (string, int, error) {
	if l.byteAt(pos+1) == 'u' {
		if l.byteAt(pos+2) == '{' {
			return l.readVariableWidthUnicodeEscape(pos)
		}
		return l.readFixedWidthUnicodeEscape(pos)
	}
	return l.readCharacterEscape(pos)
}

// readCharacterEscape reads one of the fixed single-character escapes.
func (l *Lexer) readCharacterEscape(pos int) (string, int, error) {
	switch l.byteAt(pos + 1) {
	case '"':
		return "\"", 2, nil
	case '\\':
		return "\\", 2, nil
	case '/':
		return "/", 2, nil
	case 'b':
		return "\b", 2, nil
	case 'f':
		return "\f", 2, nil
	case 'n':
		return "\n", 2, nil
	case 'r':
		return "\r", 2, nil
	case 't':
		return "\t", 2, nil
	}
	return "", 0, l.errorf(pos, `Invalid character escape sequence: "%s".`, l.slice(pos, pos+2))
}

// readVariableWidthUnicodeEscape reads the brace form of a Unicode escape,
// such as an escape naming a code point above the basic multilingual plane.
func (l *Lexer) readVariableWidthUnicodeEscape(pos int) (string, int, error) {
	point := 0
	size := 3 // past the backslash, the u and the opening brace
	// The longest valid sequence names eight hexadecimal digits.
	for size < 12 {
		c := l.byteAt(pos + size)
		size++
		if c == '}' {
			// At least one digit must be present and it must name a scalar.
			if size < 5 || !isUnicodeScalarValue(rune(point)) {
				break
			}
			return string(rune(point)), size, nil
		}
		d := hexDigitValue(c)
		if d < 0 {
			break
		}
		point = point<<4 | d
	}
	return "", 0, l.errorf(pos, `Invalid Unicode escape sequence: "%s".`, l.slice(pos, pos+size))
}

// readFixedWidthUnicodeEscape reads the four-digit form of a Unicode escape.
//
// A four-digit escape cannot name a code point above the basic multilingual
// plane, so, as JSON does, GraphQL allows two of them to name a surrogate
// pair. JavaScript stores such a pair as two UTF-16 code units; Go combines
// them into the single code point they stand for, which is the same character.
func (l *Lexer) readFixedWidthUnicodeEscape(pos int) (string, int, error) {
	code := l.read16BitHexCode(pos + 2)
	if isUnicodeScalarValue(rune(code)) {
		return string(rune(code)), 6, nil
	}
	if isLeadingSurrogate(code) && l.byteAt(pos+6) == '\\' && l.byteAt(pos+7) == 'u' {
		if trailing := l.read16BitHexCode(pos + 8); isTrailingSurrogate(trailing) {
			r := 0x10000 + (code-0xd800)<<10 + (trailing - 0xdc00)
			return string(rune(r)), 12, nil
		}
	}
	return "", 0, l.errorf(pos, `Invalid Unicode escape sequence: "%s".`, l.slice(pos, pos+6))
}

// read16BitHexCode reads four hexadecimal digits, returning -1 if any of them
// is not a hexadecimal digit.
func (l *Lexer) read16BitHexCode(pos int) int {
	code := 0
	for i := 0; i < 4; i++ {
		d := hexDigitValue(l.byteAt(pos + i))
		if d < 0 {
			return -1
		}
		code = code<<4 | d
	}
	return code
}

// slice returns a range of the source body, clamped to its bounds.
func (l *Lexer) slice(start, end int) string {
	body := l.Source.Body
	if start < 0 {
		start = 0
	}
	if end > len(body) {
		end = len(body)
	}
	if start > end {
		return ""
	}
	return body[start:end]
}

// readBlockString reads a triple-quoted string token and dedents its lines.
func (l *Lexer) readBlockString(start int) (*Token, error) {
	body := l.Source.Body
	lineStart := l.lineStart
	pos := start + 3
	chunkStart := pos

	var currentLine strings.Builder
	var blockLines []string

	for pos < len(body) {
		c := body[pos]

		if c == '"' && l.byteAt(pos+1) == '"' && l.byteAt(pos+2) == '"' {
			currentLine.WriteString(body[chunkStart:pos])
			blockLines = append(blockLines, currentLine.String())

			value := strings.Join(dedentBlockStringLines(blockLines), "\n")
			tok := l.newToken(TokenBlockString, start, pos+3, value)

			// The token records where it began, so the lexer only moves to the
			// block's last line after the token has been built.
			l.line += len(blockLines) - 1
			l.lineStart = lineStart
			l.colAnchor, l.colAnchorCol = lineStart, 1
			return tok, nil
		}

		if c == '\\' && l.byteAt(pos+1) == '"' && l.byteAt(pos+2) == '"' && l.byteAt(pos+3) == '"' {
			currentLine.WriteString(body[chunkStart:pos])
			chunkStart = pos + 1 // keep the quotes, drop the backslash
			pos += 4
			continue
		}

		if c == '\n' || c == '\r' {
			currentLine.WriteString(body[chunkStart:pos])
			blockLines = append(blockLines, currentLine.String())
			currentLine.Reset()
			if c == '\r' && l.byteAt(pos+1) == '\n' {
				pos += 2
			} else {
				pos++
			}
			chunkStart = pos
			lineStart = pos
			continue
		}

		_, size, ok := decodeSourceChar(body, pos)
		if !ok {
			return nil, l.errorf(pos, "Invalid character within String: %s.", l.describeCharAt(pos))
		}
		pos += size
	}
	return nil, l.errorf(pos, "Unterminated string.")
}

// isUnicodeScalarValue reports whether r is a Unicode scalar value, that is,
// any code point that is not a surrogate.
func isUnicodeScalarValue(r rune) bool {
	return (r >= 0 && r <= 0xd7ff) || (r >= 0xe000 && r <= 0x10ffff)
}

func isLeadingSurrogate(code int) bool  { return code >= 0xd800 && code <= 0xdbff }
func isTrailingSurrogate(code int) bool { return code >= 0xdc00 && code <= 0xdfff }
