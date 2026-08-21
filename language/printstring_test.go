package language

import "testing"

func TestPrintString(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple string", "hello", `"hello"`},
		{"empty string", "", `""`},
		{"escapes double quotes", `"`, `"\""`},
		{"leaves single quotes alone", "'", `"'"`},
		{"escapes backslashes", `\`, `"\\"`},
		{"escapes the well known control characters", "\b\f\n\r\t", `"\b\f\n\r\t"`},
		{"escapes the zero byte", "\u0000", `"\u0000"`},
		{"leaves spaces alone", " ", `" "`},
		{"leaves non-ASCII characters alone", "\u041b", "\"\u041b\""},
		{"leaves supplementary characters alone", "\U0001F600", "\"\U0001F600\""},
		{"escapes C0 controls without a short escape", "\u000b", `"\u000B"`},
		{"escapes delete", "\u007f", `"\u007F"`},
		{"escapes the first C1 control", "\u0080", `"\u0080"`},
		{"escapes the last C1 control", "\u009f", `"\u009F"`},
		{"leaves the character just past C1 alone", "\u00a0", "\"\u00a0\""},
		{"uses uppercase hexadecimal digits", "\u001a", `"\u001A"`},
		{"escapes only what needs it", "a\"b\\c", `"a\"b\\c"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PrintString(tt.in); got != tt.want {
				t.Errorf("PrintString(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

// Every code point that must be escaped has to produce an escape sequence, and
// nothing outside those ranges may be touched.
func TestPrintString_EscapesExactlyTheRequiredRanges(t *testing.T) {
	mustEscape := func(r rune) bool {
		return r < 0x20 || r == '"' || r == '\\' || (r >= 0x7f && r <= 0x9f)
	}
	for r := rune(0); r <= 0x00ff; r++ {
		got := PrintString(string(r))
		escaped := got != `"`+string(r)+`"`
		if escaped != mustEscape(r) {
			t.Errorf("U+%04X: escaped = %v, want %v (got %s)", r, escaped, mustEscape(r), got)
		}
	}
}
