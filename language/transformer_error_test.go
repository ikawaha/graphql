package language

import "testing"

// A replacement that cannot go where it was offered fails the rewrite. It is
// reported at the place the transformer returned it rather than left to break
// something further away, and the message names both what the place holds and
// what was offered.
func TestTransform_RefusesAReplacementOfTheWrongType(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		on      Kind
		replace Node
		says    string
	}{
		{
			name: "a single child", query: "{ a }",
			on: KindName, replace: &IntValue{Value: "1"},
			says: "graphql/language: Name must be a *language.Name, not a *language.IntValue",
		},
		{
			name: "an entry of a list", query: "{ a }",
			on: KindField, replace: &IntValue{Value: "1"},
			says: "graphql/language: Selections must hold a language.Selection, not a *language.IntValue",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := mustParse(t, tt.query)
			out, err := Transform(doc, Transformer{
				Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
					if node.Kind() == tt.on {
						return tt.replace, TransformSkip
					}
					return nil, TransformContinue
				},
			})
			if err == nil {
				t.Fatal("the replacement was accepted")
			}
			if err.Error() != tt.says {
				t.Errorf("says %q\nwant %q", err, tt.says)
			}
			// A half-rewritten tree is no use to anyone, so none is returned.
			if out != nil {
				t.Errorf("a tree came back as well: %s", Print(out))
			}
			// What went in is untouched.
			if got := Print(doc); got != "{\n  a\n}" {
				t.Errorf("the original was modified: %s", got)
			}
		})
	}
}

// The first mismatch ends the rewrite: nothing after it is asked for.
func TestTransform_StopsAtTheFirstMismatch(t *testing.T) {
	doc := mustParse(t, "{ a b c }")
	asked := 0
	_, err := Transform(doc, Transformer{
		Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
			if _, isName := node.(*Name); isName {
				asked++
				return &IntValue{Value: "1"}, TransformSkip
			}
			return nil, TransformContinue
		},
	})
	if err == nil {
		t.Fatal("the replacement was accepted")
	}
	if asked != 1 {
		t.Errorf("%d replacements were asked for, want the rewrite to stop at the first", asked)
	}
}
