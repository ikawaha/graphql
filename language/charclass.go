package language

// The character class helpers below take a single byte rather than a rune.
// Every character they recognise is ASCII, and in UTF-8 the leading byte of a
// multi-byte sequence is always >= 0x80, so a byte test can never mistake part
// of a multi-byte character for one of these.

// isWhiteSpace reports whether c is GraphQL WhiteSpace:
// horizontal tab (U+0009) or space (U+0020).
func isWhiteSpace(c byte) bool {
	return c == '\t' || c == ' '
}

// isDigit reports whether c is one of the digits 0 through 9.
func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// isLetter reports whether c is an ASCII letter, A through Z or a through z.
func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isNameStart reports whether c may begin a Name: a letter or an underscore.
func isNameStart(c byte) bool {
	return isLetter(c) || c == '_'
}

// isNameContinue reports whether c may appear after the first character of a
// Name: a letter, a digit, or an underscore.
func isNameContinue(c byte) bool {
	return isLetter(c) || isDigit(c) || c == '_'
}

// hexDigitValue returns the numeric value of a hexadecimal digit, or -1 if c
// is not one.
func hexDigitValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}
