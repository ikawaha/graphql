package language_test

// Ported from graphql-js src/language/__tests__/printString-test.ts: how a
// string is written back into a document.

import (
	"strings"
	"testing"

	"github.com/ikawaha/graphql/language"
)

func TestPortedPrintString(t *testing.T) {
	for _, tt := range []struct{ name, in, want string }{
		{"an empty string", "", "\"\""},
		{"a double quote", "\"", "\"\\\"\""},
		{"a backslash", "\\", "\"\\\\\""},
		{"a line feed", "\u000A", "\"\\n\""},
		{"a carriage return", "\u000D", "\"\\r\""},
		{"a tab", "\u0009", "\"\\t\""},
		{"a backspace", "\u0008", "\"\\b\""},
		{"a form feed", "\u000C", "\"\\f\""},
		{"a null", "\u0000", "\"\\u0000\""},
		{"a space, which stands as it is", " ", "\" \""},
		{"a character outside ASCII, which stands as it is", "↻", "\"↻\""},
		{"a character outside the basic plane, which stands as it is", "😀", "\"😀\""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := language.PrintString(tt.in); got != tt.want {
				t.Errorf("wrote %q, want %q", got, tt.want)
			}
		})
	}
}

// Every control character is escaped and every printable one is not, which is
// checked in one go over the whole of the first two blocks.
func TestPortedPrintString_ControlCharacters(t *testing.T) {
	var b strings.Builder
	for r := rune(0); r < 0xA0; r++ {
		b.WriteRune(r)
	}
	want := "\"\\u0000\\u0001\\u0002\\u0003\\u0004\\u0005\\u0006\\u0007\\b\\t\\n\\u000B\\f\\r\\u000E\\u000F\\u0010\\u0011\\u0012\\u0013\\u0014\\u0015\\u0016\\u0017\\u0018\\u0019\\u001A\\u001B\\u001C\\u001D\\u001E\\u001F !\\\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\\\]^_`abcdefghijklmnopqrstuvwxyz{|}~\\u007F\\u0080\\u0081\\u0082\\u0083\\u0084\\u0085\\u0086\\u0087\\u0088\\u0089\\u008A\\u008B\\u008C\\u008D\\u008E\\u008F\\u0090\\u0091\\u0092\\u0093\\u0094\\u0095\\u0096\\u0097\\u0098\\u0099\\u009A\\u009B\\u009C\\u009D\\u009E\\u009F\""
	if got := language.PrintString(b.String()); got != want {
		t.Errorf("wrote\n%q\nwant\n%q", got, want)
	}
}
