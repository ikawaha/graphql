package language

import "testing"

// Does a transformer that skipped a subtree get un-skipped when an earlier
// transformer edits the node the walk is leaving?
// A transformer that skipped a subtree starts seeing nodes again when the
// rewrite leaves it, even if another transformer edits the node on the way out.
//
// graphql-js leaves it stuck: its parallel visitor returns as soon as one
// visitor answers, so the ones after it never learn that their skip is over.
// That makes the answer depend on the order the visitors were given in, which
// COMPATIBILITY.md records as a difference.
func TestTransformInParallel_SkipEndsEvenWhenAnotherEdits(t *testing.T) {
	doc := mustParse(t, "{ a { x } b }")

	var editorSaw, skipperSaw []string
	editor := Transformer{
		Leave: func(node Node, _ VisitContext) (Node, TransformAction) {
			if f, ok := node.(*Field); ok && f.Name.Value == "a" {
				editorSaw = append(editorSaw, "edited a")
				renamed := *f
				renamed.Name = &Name{Value: "A"}
				return &renamed, TransformContinue
			}
			return nil, TransformContinue
		},
	}
	skipper := Transformer{
		Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
			if f, ok := node.(*Field); ok && f.Name.Value == "a" {
				skipperSaw = append(skipperSaw, "skip a")
				return nil, TransformSkip
			}
			if f, ok := node.(*Field); ok {
				skipperSaw = append(skipperSaw, "enter "+f.Name.Value)
			}
			return nil, TransformContinue
		},
	}
	out, err := Transform(doc, TransformInParallel(editor, skipper))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := Print(out), "{\n  A {\n    x\n  }\n  b\n}"; got != want {
		t.Errorf("wrote\n%s\nwant\n%s", got, want)
	}
	if len(editorSaw) != 1 {
		t.Errorf("the editor ran %d times, want once", len(editorSaw))
	}
	if len(skipperSaw) != 2 || skipperSaw[1] != "enter b" {
		t.Errorf("the skipper saw %v, want it to come back for the field after the one it skipped",
			skipperSaw)
	}
}
