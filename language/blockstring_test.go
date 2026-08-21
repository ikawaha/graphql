package language

import (
	"slices"
	"strings"
	"testing"
)

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty string is one empty line", "", []string{""}},
		{"no terminator", "a", []string{"a"}},
		{"line feed", "a\nb", []string{"a", "b"}},
		{"carriage return", "a\rb", []string{"a", "b"}},
		{"carriage return and line feed count once", "a\r\nb", []string{"a", "b"}},
		{"trailing terminator yields a trailing empty line", "a\n", []string{"a", ""}},
		{"leading terminator yields a leading empty line", "\na", []string{"", "a"}},
		{"mixed terminators", "a\nb\rc\r\nd", []string{"a", "b", "c", "d"}},
		{"consecutive terminators", "a\n\nb", []string{"a", "", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitLines(tt.in); !slices.Equal(got, tt.want) {
				t.Errorf("splitLines(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDedentBlockStringLines(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"empty string", []string{""}, []string{}},
		{"only whitespace", []string{"  ", "\t"}, []string{}},
		{"does not dedent the first line", []string{"  a", "  b"}, []string{"  a", "b"}},
		{
			name: "removes the smallest indentation",
			in:   []string{"", "  a", "    b"},
			want: []string{"a", "  b"},
		},
		{
			name: "counts a tab and a space as one character each",
			in:   []string{"", "\t\ta", "  b"},
			want: []string{"a", "b"},
		},
		{
			name: "ignores blank lines when computing the indent",
			in:   []string{"", "  a", "", "      ", "  b"},
			want: []string{"a", "", "    ", "b"},
		},
		{
			name: "removes leading and trailing blank lines",
			in:   []string{"", "", "  a", "  b", "", ""},
			want: []string{"a", "b"},
		},
		{
			name: "removes leading and trailing whitespace-only lines",
			in:   []string{"  ", "  ", "  a", "  b", "  ", "  "},
			want: []string{"a", "b"},
		},
		{
			name: "retains the indentation of the first line",
			in:   []string{"    a", "  b"},
			want: []string{"    a", "b"},
		},
		{
			name: "does not alter trailing spaces",
			in:   []string{"", "  a  ", "  b  "},
			want: []string{"a  ", "b  "},
		},
		{
			name: "handles a line shorter than the common indent",
			in:   []string{"", "    a", "  "},
			want: []string{"a"},
		},
		{"single line is untouched", []string{"  a"}, []string{"  a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dedentBlockStringLines(tt.in)
			if !slices.Equal(got, tt.want) {
				t.Errorf("dedentBlockStringLines(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsPrintableAsBlockString(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"empty string", "", true},
		{"simple text", "hello", true},
		{"multiple lines", "hello\nworld", true},
		{"blank line in the middle", "hello\n\nworld", true},
		{"trailing spaces on a line", "hello  \nworld", true},
		{"non-ASCII text", "こんにちは", true},
		{"quotes", `has "quotes"`, true},
		{"first line unindented, rest indented", "hello\n  world", true},
		{"only whitespace", "  ", false},
		{"only a newline", "\n", false},
		{"only blank lines", "\n\n", false},
		{"leading blank line", "\nhello", false},
		{"trailing blank line", "hello\n", false},
		{"carriage return", "hello\rworld", false},
		{"carriage return and line feed", "hello\r\nworld", false},
		{"null byte", "hello\x00world", false},
		{"vertical tab", "hello\vworld", false},
		{"form feed", "hello\fworld", false},
		{"every line shares an indent", "  hello\n  world", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPrintableAsBlockString(tt.in); got != tt.want {
				t.Errorf("isPrintableAsBlockString(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestPrintBlockString(t *testing.T) {
	const q3 = `"""`
	tests := []struct {
		name         string
		in           string
		want         string
		wantMinimize string
	}{
		{
			name:         "single line stays on one line",
			in:           "string",
			want:         q3 + "string" + q3,
			wantMinimize: q3 + "string" + q3,
		},
		{
			name:         "does not escape characters other than triple quotes",
			in:           `\ / \\ '`,
			want:         q3 + `\ / \\ '` + q3,
			wantMinimize: q3 + `\ / \\ '` + q3,
		},
		{
			name:         "escapes triple quotes",
			in:           `has """ inside`,
			want:         q3 + `has \""" inside` + q3,
			wantMinimize: q3 + `has \""" inside` + q3,
		},
		{
			name:         "a value ending in triple quotes goes multi-line",
			in:           `string"""`,
			want:         q3 + "\n" + `string\"""` + "\n" + q3,
			wantMinimize: q3 + `string\"""` + q3,
		},
		{
			name:         "keeps a leading space on a single line",
			in:           " string",
			want:         q3 + " string" + q3,
			wantMinimize: q3 + " string" + q3,
		},
		{
			name:         "a leading space with a trailing quote forces only a trailing newline",
			in:           ` string"`,
			want:         q3 + ` string"` + "\n" + q3,
			wantMinimize: q3 + ` string"` + "\n" + q3,
		},
		{
			name:         "a trailing backslash forces newlines",
			in:           `string\`,
			want:         q3 + "\n" + `string\` + "\n" + q3,
			wantMinimize: q3 + `string\` + "\n" + q3,
		},
		{
			name:         "multiple lines are surrounded by blank lines",
			in:           "start\nend",
			want:         q3 + "\nstart\nend\n" + q3,
			wantMinimize: q3 + "start\nend" + q3,
		},
		{
			name:         "an indented continuation forces a leading newline",
			in:           "start\n  indented",
			want:         q3 + "\nstart\n  indented\n" + q3,
			wantMinimize: q3 + "\nstart\n  indented" + q3,
		},
		{
			name:         "a long single line goes multi-line",
			in:           strings.Repeat("a", 71),
			want:         q3 + "\n" + strings.Repeat("a", 71) + "\n" + q3,
			wantMinimize: q3 + strings.Repeat("a", 71) + q3,
		},
		{
			name:         "a single line at the length limit stays on one line",
			in:           strings.Repeat("a", 70),
			want:         q3 + strings.Repeat("a", 70) + q3,
			wantMinimize: q3 + strings.Repeat("a", 70) + q3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := printBlockString(tt.in, false); got != tt.want {
				t.Errorf("printBlockString(%q, false) = %q, want %q", tt.in, got, tt.want)
			}
			if got := printBlockString(tt.in, true); got != tt.wantMinimize {
				t.Errorf("printBlockString(%q, true) = %q, want %q", tt.in, got, tt.wantMinimize)
			}
		})
	}
}

// Whatever printBlockString emits has to read back as the value it was given.
// The lexer does not exist yet, so this checks the part the printer owns: the
// printed body dedents back to the original value.
func TestPrintBlockString_RoundTripsThroughDedent(t *testing.T) {
	values := []string{
		"string",
		" string",
		"start\nend",
		"start\n  indented",
		`string\`,
		`string"`,
		"hello\n\nworld",
		"こんにちは\n世界",
		`has """ inside`,
	}
	for _, v := range values {
		if !isPrintableAsBlockString(v) {
			continue
		}
		for _, minimize := range []bool{false, true} {
			printed := printBlockString(v, minimize)
			raw := strings.TrimSuffix(strings.TrimPrefix(printed, `"""`), `"""`)
			raw = strings.ReplaceAll(raw, `\"""`, `"""`)
			got := strings.Join(dedentBlockStringLines(splitLines(raw)), "\n")
			if got != v {
				t.Errorf("round trip of %q (minimize=%v) = %q, want %q", v, minimize, got, v)
			}
		}
	}
}
