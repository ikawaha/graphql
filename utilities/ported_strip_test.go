package utilities_test

// Ported from graphql-js src/utilities/__tests__/stripIgnoredCharacters-test.ts.
//
// Stripping is checked twice over: the result is what was expected, and
// stripping it again leaves it alone, since there is nothing left to remove.

import (
	"testing"

	"github.com/ikawaha/graphql/utilities"
)

func TestPortedStripIgnoredCharacters(t *testing.T) {
	for _, tt := range []struct{ name, in, want string }{
		{
			name: "{ foo(arg: \"str\"",
			in:   "{ foo(arg: \"str\"",
			want: "{foo(arg:\"str\"",
		},
		{
			name: "\\n",
			in:   "\n",
			want: "",
		},
		{
			name: ",",
			in:   ",",
			want: "",
		},
		{
			name: ",,",
			in:   ",,",
			want: "",
		},
		{
			name: "#comment\\n, \\n",
			in:   "#comment\n, \n",
			want: "",
		},
		{
			name: "\\n1",
			in:   "\n1",
			want: "1",
		},
		{
			name: ",1",
			in:   ",1",
			want: "1",
		},
		{
			name: ",,1",
			in:   ",,1",
			want: "1",
		},
		{
			name: "#comment\\n, \\n1",
			in:   "#comment\n, \n1",
			want: "1",
		},
		{
			name: "1\\n",
			in:   "1\n",
			want: "1",
		},
		{
			name: "1,",
			in:   "1,",
			want: "1",
		},
		{
			name: "1,,",
			in:   "1,,",
			want: "1",
		},
		{
			name: "1#comment\\n, \\n",
			in:   "1#comment\n, \n",
			want: "1",
		},
		{
			name: "[,)",
			in:   "[,)",
			want: "[)",
		},
		{
			name: "[\\r)",
			in:   "[\r)",
			want: "[)",
		},
		{
			name: "[\\r\\r)",
			in:   "[\r\r)",
			want: "[)",
		},
		{
			name: "[\\r,)",
			in:   "[\r,)",
			want: "[)",
		},
		{
			name: "[,\\n)",
			in:   "[,\n)",
			want: "[)",
		},
		{
			name: "[,1",
			in:   "[,1",
			want: "[1",
		},
		{
			name: "[\\r1",
			in:   "[\r1",
			want: "[1",
		},
		{
			name: "[\\r\\r1",
			in:   "[\r\r1",
			want: "[1",
		},
		{
			name: "[\\r,1",
			in:   "[\r,1",
			want: "[1",
		},
		{
			name: "[,\\n1",
			in:   "[,\n1",
			want: "[1",
		},
		{
			name: "1,[",
			in:   "1,[",
			want: "1[",
		},
		{
			name: "1\\r[",
			in:   "1\r[",
			want: "1[",
		},
		{
			name: "1\\r\\r[",
			in:   "1\r\r[",
			want: "1[",
		},
		{
			name: "1\\r,[",
			in:   "1\r,[",
			want: "1[",
		},
		{
			name: "1,\\n[",
			in:   "1,\n[",
			want: "1[",
		},
		{
			name: "a ...",
			in:   "a ...",
			want: "a ...",
		},
		{
			name: "1 ...",
			in:   "1 ...",
			want: "1 ...",
		},
		{
			name: "1 ... ...",
			in:   "1 ... ...",
			want: "1 ......",
		},
		{
			name: "1 2",
			in:   "1 2",
			want: "1 2",
		},
		{
			name: "\"\" \"\"",
			in:   "\"\" \"\"",
			want: "\"\" \"\"",
		},
		{
			name: "a b",
			in:   "a b",
			want: "a b",
		},
		{
			name: "a,1",
			in:   "a,1",
			want: "a 1",
		},
		{
			name: "a,,1",
			in:   "a,,1",
			want: "a 1",
		},
		{
			name: "a 1",
			in:   "a  1",
			want: "a 1",
		},
		{
			name: "a \\t 1",
			in:   "a \t 1",
			want: "a 1",
		},
		{
			name: "\" \"",
			in:   "\" \"",
			want: "\" \"",
		},
		{
			name: "\",\"",
			in:   "\",\"",
			want: "\",\"",
		},
		{
			name: "\",,\"",
			in:   "\",,\"",
			want: "\",,\"",
		},
		{
			name: "\",|\"",
			in:   "\",|\"",
			want: "\",|\"",
		},
		{
			name: "\"\"\",\"\"\"",
			in:   "\"\"\",\"\"\"",
			want: "\"\"\",\"\"\"",
		},
		{
			name: "\"\"\",,\"\"\"",
			in:   "\"\"\",,\"\"\"",
			want: "\"\"\",,\"\"\"",
		},
		{
			name: "\"\"\",|\"\"\"",
			in:   "\"\"\",|\"\"\"",
			want: "\"\"\",|\"\"\"",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := utilities.StripIgnoredCharacters(tt.in)
			if err != nil {
				t.Fatalf("stripping: %v", err)
			}
			if got != tt.want {
				t.Errorf("stripped to %q, want %q", got, tt.want)
			}
			again, err := utilities.StripIgnoredCharacters(got)
			if err != nil {
				t.Fatalf("stripping what was already stripped: %v", err)
			}
			if again != tt.want {
				t.Errorf("stripping again gave %q, want %q", again, tt.want)
			}
		})
	}
}
