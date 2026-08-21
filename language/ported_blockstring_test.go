package language_test

// Ported from graphql-js src/language/__tests__/blockString-test.ts: which
// strings may be written as block strings, and how one is written.

import (
	"testing"

	"github.com/ikawaha/graphql/language"
)

func TestPortedIsPrintableAsBlockString(t *testing.T) {
	for _, tt := range []struct {
		in_  string
		want bool
	}{
		{in_: "", want: true},
		{in_: " a", want: true},
		{in_: "\t\"\n\"", want: true},
		{in_: "\t\"\n \n\t\"", want: false},
		{in_: " ", want: false},
		{in_: "\t", want: false},
		{in_: "\t ", want: false},
		{in_: " \t", want: false},
		{in_: "\u0000", want: false},
		{in_: "a\u0000b", want: false},
		{in_: "\n", want: false},
		{in_: "\n\n", want: false},
		{in_: "\n\n\n", want: false},
		{in_: " \n  \n", want: false},
		{in_: "\t\n\t\t\n", want: false},
		{in_: "\r", want: false},
		{in_: "\n\r", want: false},
		{in_: "\r\n", want: false},
		{in_: "a\rb", want: false},
		{in_: "\na", want: false},
		{in_: " \na", want: false},
		{in_: "\t\na", want: false},
		{in_: "\n\na", want: false},
		{in_: "a\n", want: false},
		{in_: "a\n ", want: false},
		{in_: "a\n\t", want: false},
		{in_: "a\n\n", want: false},
	} {
		t.Run(language.PrintString(tt.in_), func(t *testing.T) {
			if got := language.IsPrintableAsBlockString(tt.in_); got != tt.want {
				t.Errorf("= %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPortedPrintBlockString(t *testing.T) {
	for _, tt := range []struct {
		name, in_, readable, minimized string
	}{
		{
			name:      "does not escape characters",
			in_:       "\" \\ / \u0008 \u000C \n \r \t",
			readable:  "\"\"\"\n\" \\ / \u0008 \u000C \n \r \t\n\"\"\"",
			minimized: "\"\"\"\n\" \\ / \u0008 \u000C \n \r \t\"\"\"",
		},
		{
			name:      "prints a one-liner on one line",
			in_:       "one liner",
			readable:  "\"\"\"one liner\"\"\"",
			minimized: "\"\"\"one liner\"\"\"",
		},
		{
			name:      "prints a string ending in three quotes across lines",
			in_:       "triple quotation \"\"\"",
			readable:  "\"\"\"\ntriple quotation \\\"\"\"\n\"\"\"",
			minimized: "\"\"\"triple quotation \\\"\"\"\"\"\"",
		},
		{
			name:      "prints a one-liner that starts with a space",
			in_:       "    space-led string",
			readable:  "\"\"\"    space-led string\"\"\"",
			minimized: "\"\"\"    space-led string\"\"\"",
		},
		{
			name:      "prints a one-liner that starts with a space and ends in a quote",
			in_:       "    space-led value \"quoted string\"",
			readable:  "\"\"\"    space-led value \"quoted string\"\n\"\"\"",
			minimized: "\"\"\"    space-led value \"quoted string\"\n\"\"\"",
		},
		{
			name:      "prints a one-liner ending in a backslash",
			in_:       "backslash \\",
			readable:  "\"\"\"\nbackslash \\\n\"\"\"",
			minimized: "\"\"\"backslash \\\n\"\"\"",
		},
		{
			name:      "prints lines that are indented differently",
			in_:       "no indent\n with indent",
			readable:  "\"\"\"\nno indent\n with indent\n\"\"\"",
			minimized: "\"\"\"\nno indent\n with indent\"\"\"",
		},
		{
			name:      "prints a string whose first line is indented",
			in_:       "    first  \n  line     \nindentation\n     string",
			readable:  "\"\"\"\n    first  \n  line     \nindentation\n     string\n\"\"\"",
			minimized: "\"\"\"    first  \n  line     \nindentation\n     string\"\"\"",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := language.PrintBlockString(tt.in_, false); got != tt.readable {
				t.Errorf("wrote %q, want %q", got, tt.readable)
			}
			if got := language.PrintBlockString(tt.in_, true); got != tt.minimized {
				t.Errorf("minimized to %q, want %q", got, tt.minimized)
			}
		})
	}
}
