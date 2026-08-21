package language

import (
	"strings"
	"testing"
)

func TestPrintSourceLocation(t *testing.T) {
	tests := []struct {
		name string
		body string
		at   SourceLocation
		want []string
	}{
		{
			name: "start of a single line",
			body: "type Query { hello: String }",
			at:   SourceLocation{Line: 1, Column: 1},
			want: []string{
				"GraphQL request:1:1",
				"1 | type Query { hello: String }",
				"  | ^",
			},
		},
		{
			name: "middle of a single line",
			body: "type Query { hello: String }",
			at:   SourceLocation{Line: 1, Column: 14},
			want: []string{
				"GraphQL request:1:14",
				"1 | type Query { hello: String }",
				"  |              ^",
			},
		},
		{
			name: "first of several lines shows the line after",
			body: "a\nb\nc",
			at:   SourceLocation{Line: 1, Column: 1},
			want: []string{
				"GraphQL request:1:1",
				"1 | a",
				"  | ^",
				"2 | b",
			},
		},
		{
			name: "a middle line shows both neighbours",
			body: "a\nb\nc",
			at:   SourceLocation{Line: 2, Column: 1},
			want: []string{
				"GraphQL request:2:1",
				"1 | a",
				"2 | b",
				"  | ^",
				"3 | c",
			},
		},
		{
			name: "the last line shows only the line before",
			body: "a\nb\nc",
			at:   SourceLocation{Line: 3, Column: 1},
			want: []string{
				"GraphQL request:3:1",
				"2 | b",
				"3 | c",
				"  | ^",
			},
		},
		{
			name: "prefixes are aligned when the numbers widen",
			body: strings.Repeat("x\n", 10) + "y",
			at:   SourceLocation{Line: 10, Column: 1},
			want: []string{
				"GraphQL request:10:1",
				" 9 | x",
				"10 | x",
				"   | ^",
				"11 | y",
			},
		},
		{
			name: "an empty neighbouring line prints as a bare prefix",
			body: "a\n\nc",
			at:   SourceLocation{Line: 1, Column: 1},
			want: []string{
				"GraphQL request:1:1",
				"1 | a",
				"  | ^",
				"2 |",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PrintSourceLocation(NewSource(tt.body), tt.at)
			want := strings.Join(tt.want, "\n")
			if got != want {
				t.Errorf("PrintSourceLocation() =\n%s\nwant\n%s", got, want)
			}
		})
	}
}

// A source embedded in a larger file reports its position in that file's
// coordinates, and its first line is indented to sit where it really does.
func TestPrintSourceLocation_WithOffset(t *testing.T) {
	source := NewSource("a\nb", SourceName("foo.graphql"), SourceLocationOffset(10, 5))

	got := PrintSourceLocation(source, SourceLocation{Line: 1, Column: 1})
	want := strings.Join([]string{
		"foo.graphql:10:5",
		"10 |     a",
		"   |     ^",
		"11 | b",
	}, "\n")
	if got != want {
		t.Errorf("first line =\n%s\nwant\n%s", got, want)
	}

	// Later lines start at column 1 wherever the document was embedded, so
	// only the first line is shifted.
	got = PrintSourceLocation(source, SourceLocation{Line: 2, Column: 1})
	want = strings.Join([]string{
		"foo.graphql:11:1",
		"10 |     a",
		"11 | b",
		"   | ^",
	}, "\n")
	if got != want {
		t.Errorf("second line =\n%s\nwant\n%s", got, want)
	}
}

// A minified document is one enormous line, so it is shown in chunks rather
// than printed whole.
func TestPrintSourceLocation_MinifiedLine(t *testing.T) {
	body := strings.Repeat("a", 200)
	got := PrintSourceLocation(NewSource(body), SourceLocation{Line: 1, Column: 100})
	lines := strings.Split(got, "\n")

	if lines[0] != "GraphQL request:1:100" {
		t.Errorf("header = %q, want %q", lines[0], "GraphQL request:1:100")
	}
	if len(lines) != 5 {
		t.Fatalf("printed %d lines, want 5:\n%s", len(lines), got)
	}
	// Every excerpt line holds at most one chunk, so none of them is as long
	// as the original.
	for _, line := range lines[1:] {
		if len(line) > 4+80 {
			t.Errorf("excerpt line is %d characters, want it chunked: %q", len(line), line)
		}
	}
	// Column 100 falls 20 characters into the second chunk, so the caret sits
	// after the prefix, its separating space, and 19 more.
	if got, want := lines[3], "  | "+strings.Repeat(" ", 19)+"^"; got != want {
		t.Errorf("caret line = %q, want %q", got, want)
	}
}

func TestPrintLocation_FromNode(t *testing.T) {
	const body = "type Query { hello: String }"
	doc := mustParse(t, body)
	got := PrintLocation(doc.Definitions[0].Location())
	want := strings.Join([]string{
		"GraphQL request:1:1",
		"1 | type Query { hello: String }",
		"  | ^",
	}, "\n")
	if got != want {
		t.Errorf("PrintLocation() =\n%s\nwant\n%s", got, want)
	}
}

func TestPrintLocation_Nil(t *testing.T) {
	if got := PrintLocation(nil); got != "" {
		t.Errorf("PrintLocation(nil) = %q, want empty", got)
	}
	if got := PrintLocation(&Location{}); got != "" {
		t.Errorf("PrintLocation with no source = %q, want empty", got)
	}
}

// The caret counts code points, so it lands under the right character even
// when the line holds multi-byte text.
func TestPrintSourceLocation_MultiByteLine(t *testing.T) {
	got := PrintSourceLocation(NewSource("あいう!"), SourceLocation{Line: 1, Column: 4})
	want := strings.Join([]string{
		"GraphQL request:1:4",
		"1 | あいう!",
		"  |    ^",
	}, "\n")
	if got != want {
		t.Errorf("PrintSourceLocation() =\n%s\nwant\n%s", got, want)
	}
}
