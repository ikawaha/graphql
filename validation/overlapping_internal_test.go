package validation

import (
	"testing"

	"github.com/ikawaha/graphql/language"
)

// Whether two selections were given the same arguments decides whether they
// can share a place in the response, so what counts as "the same" has to be
// exactly right: order must not matter where it carries no meaning, and must
// matter where it does.
func TestSameArguments(t *testing.T) {
	parseArgs := func(t *testing.T, field string) []*language.Argument {
		t.Helper()
		doc, err := language.ParseString("{ " + field + " }")
		if err != nil {
			t.Fatalf("parsing %q: %v", field, err)
		}
		set := doc.Definitions[0].(*language.OperationDefinition).SelectionSet
		return set.Selections[0].(*language.Field).Arguments
	}

	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"neither has any", "f", "f", true},
		{"one has none", "f", "f(a: 1)", false},
		{"the same argument", "f(a: 1)", "f(a: 1)", true},
		{"different values", "f(a: 1)", "f(a: 2)", false},
		{"different names", "f(a: 1)", "f(b: 1)", false},
		{"different counts", "f(a: 1)", "f(a: 1, b: 2)", false},
		// An argument list is unordered, so writing it another way round is
		// the same list.
		{"written in another order", "f(a: 1, b: 2)", "f(b: 2, a: 1)", true},
		// So are the fields of an input object.
		{"object fields in another order", "f(a: { x: 1, y: 2 })", "f(a: { y: 2, x: 1 })", true},
		{"object fields nested", "f(a: { x: { p: 1, q: 2 } })", "f(a: { x: { q: 2, p: 1 } })", true},
		{"objects that differ", "f(a: { x: 1 })", "f(a: { x: 2 })", false},
		{"an object missing a field", "f(a: { x: 1, y: 2 })", "f(a: { x: 1 })", false},
		// A list is ordered, so the order is part of the value.
		{"a list in another order", "f(a: [1, 2])", "f(a: [2, 1])", false},
		{"the same list", "f(a: [1, 2])", "f(a: [1, 2])", true},
		{"lists of objects", "f(a: [{ x: 1, y: 2 }])", "f(a: [{ y: 2, x: 1 }])", true},
		{"variables of the same name", "f(a: $v)", "f(a: $v)", true},
		{"variables of different names", "f(a: $v)", "f(a: $w)", false},
		// A variable and the value it might hold are not the same argument:
		// what it holds is not known when this is decided.
		{"a variable against a literal", "f(a: $v)", "f(a: 1)", false},
		{"null against nothing", "f(a: null)", "f", false},
		{"enum against string", "f(a: X)", `f(a: "X")`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sameArguments(parseArgs(t, tt.a), scope{}, parseArgs(t, tt.b), scope{})
			if got != tt.want {
				t.Errorf("sameArguments(%s, %s) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// The memo of compared fragment pairs has to be asymmetric in one direction: a
// pair compared without assuming the two can never both apply has been
// compared more thoroughly, so that answer covers the weaker question too, but
// not the other way round. Getting this backwards would silently skip
// comparisons and let conflicts through.
func TestPairSet(t *testing.T) {
	t.Run("nothing is remembered to begin with", func(t *testing.T) {
		p := pairSet{}
		if p.has("a", "b", true) || p.has("a", "b", false) {
			t.Error("an empty set claims to have seen a pair")
		}
	})

	t.Run("order does not matter", func(t *testing.T) {
		p := pairSet{}
		p.add("a", "b", false)
		if !p.has("b", "a", false) {
			t.Error("a pair added one way round was not found the other")
		}
	})

	t.Run("the thorough comparison covers the weaker question", func(t *testing.T) {
		p := pairSet{}
		p.add("a", "b", false)
		if !p.has("a", "b", false) {
			t.Error("the pair was not remembered")
		}
		if !p.has("a", "b", true) {
			t.Error("a pair compared thoroughly does not answer the weaker question")
		}
	})

	t.Run("the weaker comparison does not cover the thorough one", func(t *testing.T) {
		p := pairSet{}
		p.add("a", "b", true)
		if !p.has("a", "b", true) {
			t.Error("the pair was not remembered")
		}
		if p.has("a", "b", false) {
			t.Error("a pair compared only under exclusivity was taken to answer the fuller question")
		}
	})
}
