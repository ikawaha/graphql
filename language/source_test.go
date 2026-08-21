package language

import "testing"

func TestNewSource_Defaults(t *testing.T) {
	s := NewSource("{ hello }")
	if s.Body != "{ hello }" {
		t.Errorf("Body = %q, want %q", s.Body, "{ hello }")
	}
	if s.Name != "GraphQL request" {
		t.Errorf("Name = %q, want %q", s.Name, "GraphQL request")
	}
	if want := (SourceLocation{Line: 1, Column: 1}); s.LocationOffset != want {
		t.Errorf("LocationOffset = %v, want %v", s.LocationOffset, want)
	}
}

func TestNewSource_Options(t *testing.T) {
	s := NewSource("{ hello }", SourceName("schema.graphql"), SourceLocationOffset(10, 3))
	if s.Name != "schema.graphql" {
		t.Errorf("Name = %q, want %q", s.Name, "schema.graphql")
	}
	if want := (SourceLocation{Line: 10, Column: 3}); s.LocationOffset != want {
		t.Errorf("LocationOffset = %v, want %v", s.LocationOffset, want)
	}
}

func TestNewSource_RejectsNonPositiveOffsets(t *testing.T) {
	tests := []struct {
		name         string
		line, column int
	}{
		{"zero line", 0, 1},
		{"negative line", -1, 1},
		{"zero column", 1, 0},
		{"negative column", 1, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("NewSource with offset %d:%d did not panic", tt.line, tt.column)
				}
			}()
			NewSource("{}", SourceLocationOffset(tt.line, tt.column))
		})
	}
}

// A Source must be able to hold input that is not valid UTF-8. The
// specification requires rejecting an unpaired surrogate in the source text,
// and the lexer can only report that if the bytes survive this far. Here
// U+DEAD is encoded in the three-byte form that UTF-8 forbids.
func TestSource_HoldsInvalidUTF8(t *testing.T) {
	const loneSurrogate = "\xed\xba\xad"
	s := NewSource(`"bad ` + loneSurrogate + `"`)
	if len(s.Body) != 9 {
		t.Errorf("len(Body) = %d, want 9", len(s.Body))
	}
	if s.Body[5] != 0xed || s.Body[6] != 0xba || s.Body[7] != 0xad {
		t.Errorf("Body bytes = % x, want the surrogate bytes preserved", s.Body)
	}
}

func TestSourceLocation_String(t *testing.T) {
	if got := (SourceLocation{Line: 3, Column: 14}).String(); got != "3:14" {
		t.Errorf("String() = %q, want %q", got, "3:14")
	}
}
