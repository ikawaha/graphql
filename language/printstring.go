package language

import (
	"strconv"
	"strings"
)

// PrintString renders s as a GraphQL StringValue literal, including the
// surrounding quotes. Control characters, the quote and the backslash are
// replaced with escape sequences.
//
// Escaped are U+0000 through U+001F, the quote U+0022, the backslash U+005C,
// and U+007F through U+009F. The characters with dedicated short escapes use
// them; everything else becomes \uXXXX with uppercase hexadecimal digits.
func PrintString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\f':
			b.WriteString(`\f`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20 || (r >= 0x7f && r <= 0x9f):
			b.WriteString(`\u`)
			h := strconv.FormatInt(int64(r), 16)
			for i := len(h); i < 4; i++ {
				b.WriteByte('0')
			}
			b.WriteString(strings.ToUpper(h))
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
