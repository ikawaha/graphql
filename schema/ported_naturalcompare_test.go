package schema_test

// Ported from graphql-js src/jsutils/__tests__/naturalCompare-test.ts.

import (
	"testing"

	"github.com/ikawaha/graphql/schema"
)

func TestPortedNaturalCompare(t *testing.T) {
	for _, tt := range []struct {
		group string
		a, b  string
		want  int
	}{
		{"empty strings", "", "", 0},
		{"empty strings", "", "a", -1},
		{"empty strings", "", "1", -1},
		{"empty strings", "a", "", 1},
		{"empty strings", "1", "", 1},

		{"different lengths", "A", "A", 0},
		{"different lengths", "A1", "A1", 0},
		{"different lengths", "A", "AA", -1},
		{"different lengths", "A1", "A1A", -1},
		{"different lengths", "AA", "A", 1},
		{"different lengths", "A1A", "A1", 1},

		{"numbers", "0", "0", 0},
		{"numbers", "1", "1", 0},
		{"numbers", "1", "2", -1},
		{"numbers", "2", "1", 1},
		{"numbers", "2", "11", -1},
		{"numbers", "11", "2", 1},

		{"leading zeros", "00", "00", 0},
		{"leading zeros", "0", "00", -1},
		{"leading zeros", "00", "0", 1},
		{"leading zeros", "02", "11", -1},
		{"leading zeros", "11", "02", 1},
		{"leading zeros", "011", "200", -1},
		{"leading zeros", "200", "011", 1},

		{"mixed", "a0a", "a0a", 0},
		{"mixed", "a0a", "a9a", -1},
		{"mixed", "a9a", "a0a", 1},
		{"mixed", "a00a", "a00a", 0},
		{"mixed", "a00a", "a09a", -1},
		{"mixed", "a09a", "a00a", 1},
		{"mixed", "a0a1", "a0a1", 0},
		{"mixed", "a0a1", "a0a9", -1},
		{"mixed", "a0a9", "a0a1", 1},
		{"mixed", "a10a11a", "a10a11a", 0},
		{"mixed", "a10a11a", "a10a19a", -1},
		{"mixed", "a10a19a", "a10a11a", 1},
		{"mixed", "a10a11a", "a10a11b", -1},
		{"mixed", "a10a11b", "a10a11a", 1},
	} {
		t.Run(tt.group+": "+tt.a+" vs "+tt.b, func(t *testing.T) {
			got := schema.NaturalCompare(tt.a, tt.b)
			if sign(got) != tt.want {
				t.Errorf("NaturalCompare(%q, %q) = %d, want %d", tt.a, tt.b, sign(got), tt.want)
			}
		})
	}
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}
