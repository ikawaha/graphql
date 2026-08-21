package schema_test

// Ported from graphql-js src/type/__tests__/assertName-test.ts: which names a
// schema may use, and which an enum member may.

import (
	"strings"
	"testing"
	"unicode"

	"github.com/ikawaha/graphql/schema"
)

// saysLike compares what a Go error says with what graphql-js throws.
//
// These are errors returned from a function rather than messages put straight
// into a response: whatever reports one wraps it, and Go writes such an error
// starting in lower case with no full stop so that it reads well inside the
// sentence around it. What is compared is therefore the sentence, not its
// capitalisation or its punctuation.
func saysLike(err error, want string) bool {
	if err == nil {
		return false
	}
	normalise := func(text string) string {
		text = strings.TrimSuffix(strings.TrimSpace(text), ".")
		if text == "" {
			return text
		}
		return string(unicode.ToLower(rune(text[0]))) + text[1:]
	}
	return strings.Contains(normalise(err.Error()), normalise(want))
}

func TestPortedAssertName(t *testing.T) {
	for _, tt := range []struct{ name, in, fails string }{
		{"a valid name", "_ValidName123", ""},
		{"nothing at all", "", "Expected name to be a non-empty string."},
		{"characters a name may not hold", ">--()-->",
			`Names must only contain [_a-zA-Z0-9] but ">--()-->" does not.`},
		{"a character a name may not start with", "42MeaningsOfLife",
			`Names must start with [_a-zA-Z] but "42MeaningsOfLife" does not.`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := schema.ValidateName(tt.in)
			switch {
			case tt.fails == "" && err != nil:
				t.Errorf("%q was refused: %v", tt.in, err)
			case tt.fails != "" && err == nil:
				t.Errorf("%q was accepted", tt.in)
			case tt.fails != "" && !saysLike(err, tt.fails):
				t.Errorf("said %q, want %q", err, tt.fails)
			}
		})
	}
}

func TestPortedAssertEnumValueName(t *testing.T) {
	for _, tt := range []struct{ name, in, fails string }{
		{"a valid name", "_ValidName123", ""},
		{"nothing at all", "", "Expected name to be a non-empty string."},
		{"characters a name may not hold", ">--()-->",
			`Names must only contain [_a-zA-Z0-9] but ">--()-->" does not.`},
		{"a character a name may not start with", "42MeaningsOfLife",
			`Names must start with [_a-zA-Z] but "42MeaningsOfLife" does not.`},
		{"true", "true", "Enum values cannot be named: true"},
		{"false", "false", "Enum values cannot be named: false"},
		{"null", "null", "Enum values cannot be named: null"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := schema.ValidateEnumValueName(tt.in)
			switch {
			case tt.fails == "" && err != nil:
				t.Errorf("%q was refused: %v", tt.in, err)
			case tt.fails != "" && err == nil:
				t.Errorf("%q was accepted", tt.in)
			case tt.fails != "" && !saysLike(err, tt.fails):
				t.Errorf("said %q, want %q", err, tt.fails)
			}
		})
	}
}
