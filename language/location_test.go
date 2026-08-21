package language

import "testing"

func TestGetLocation(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		position int
		want     SourceLocation
	}{
		{"start of an empty body", "", 0, SourceLocation{1, 1}},
		{"start of the first line", "abc", 0, SourceLocation{1, 1}},
		{"middle of the first line", "abc", 1, SourceLocation{1, 2}},
		{"end of the first line", "abc", 3, SourceLocation{1, 4}},
		{"just before a line feed", "ab\ncd", 2, SourceLocation{1, 3}},
		{"just after a line feed", "ab\ncd", 3, SourceLocation{2, 1}},
		{"second line", "ab\ncd", 4, SourceLocation{2, 2}},
		{"third line", "a\nb\nc", 4, SourceLocation{3, 1}},
		{"after a carriage return", "ab\rcd", 3, SourceLocation{2, 1}},
		{"after a carriage return and line feed", "ab\r\ncd", 4, SourceLocation{2, 1}},
		{"blank line", "a\n\nb", 3, SourceLocation{3, 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetLocation(NewSource(tt.body), tt.position)
			if got != tt.want {
				t.Errorf("GetLocation(%q, %d) = %v, want %v", tt.body, tt.position, got, tt.want)
			}
		})
	}
}

// A carriage return and line feed pair is one terminator, so the position
// between the two bytes must not be reported as the start of a new line.
func TestGetLocation_CRLFCountsOnce(t *testing.T) {
	src := NewSource("ab\r\ncd")
	for pos, want := range map[int]SourceLocation{
		2: {1, 3},
		4: {2, 1},
		5: {2, 2},
	} {
		if got := GetLocation(src, pos); got != want {
			t.Errorf("GetLocation(pos=%d) = %v, want %v", pos, got, want)
		}
	}
}

// Column counts Unicode code points, not bytes. This is a deliberate
// divergence from graphql-js, which counts UTF-16 code units; the two agree
// for any line made only of ASCII.
func TestGetLocation_ColumnCountsCodePoints(t *testing.T) {
	// Each of these characters is three bytes in UTF-8.
	const body = "あいう"
	tests := []struct {
		position int
		want     SourceLocation
	}{
		{0, SourceLocation{1, 1}},
		{3, SourceLocation{1, 2}},
		{6, SourceLocation{1, 3}},
		{9, SourceLocation{1, 4}},
	}
	for _, tt := range tests {
		if got := GetLocation(NewSource(body), tt.position); got != tt.want {
			t.Errorf("GetLocation(byte %d) = %v, want %v", tt.position, got, tt.want)
		}
	}
}

func TestGetLocation_ClampsOutOfRangePositions(t *testing.T) {
	src := NewSource("ab\ncd")
	if got, want := GetLocation(src, -1), (SourceLocation{1, 1}); got != want {
		t.Errorf("GetLocation(-1) = %v, want %v", got, want)
	}
	if got, want := GetLocation(src, 999), (SourceLocation{2, 3}); got != want {
		t.Errorf("GetLocation(999) = %v, want %v", got, want)
	}
}
