package language

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// transformed is the answer of a rewrite that was expected to succeed.
func transformed(t *testing.T, root Node, tr Transformer) Node {
	t.Helper()
	out, err := Transform(root, tr)
	if err != nil {
		t.Fatalf("rewriting: %v", err)
	}
	return out
}

func mustPrint(t *testing.T, node Node) string {
	t.Helper()
	if node == nil {
		return "<removed>"
	}
	return Print(node)
}

// The example from graphql-js's own documentation: renaming a field.
func TestTransform_ReplacesANode(t *testing.T) {
	doc := mustParse(t, "{ hero { name } }")
	out := transformed(t, doc, Transformer{
		Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
			field, isField := node.(*Field)
			if !isField || field.Name.Value != "hero" {
				return nil, TransformContinue
			}
			renamed := *field
			renamed.Name = &Name{Value: "human"}
			return &renamed, TransformContinue
		},
	})

	if got, want := mustPrint(t, out), "{\n  human {\n    name\n  }\n}"; got != want {
		t.Errorf("wrote\n%s\nwant\n%s", got, want)
	}
	// The tree that went in is untouched.
	if got := Print(doc); got != "{\n  hero {\n    name\n  }\n}" {
		t.Errorf("the original was modified: %s", got)
	}
}

// Only the path from a changed node up to the root is rebuilt; everything else
// is the same object it was.
func TestTransform_SharesWhatDidNotChange(t *testing.T) {
	doc := mustParse(t, "{ a { keep } b }")
	operation := doc.Definitions[0].(*OperationDefinition)
	first := operation.SelectionSet.Selections[0].(*Field)
	second := operation.SelectionSet.Selections[1].(*Field)

	out := transformed(t, doc, Transformer{
		Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
			if field, isField := node.(*Field); isField && field.Name.Value == "b" {
				renamed := *field
				renamed.Name = &Name{Value: "c"}
				return &renamed, TransformContinue
			}
			return nil, TransformContinue
		},
	})

	newDoc, isDocument := out.(*Document)
	if !isDocument {
		t.Fatalf("the root came back as %T", out)
	}
	if newDoc == doc {
		t.Fatal("the document was not rebuilt")
	}
	newOperation := newDoc.Definitions[0].(*OperationDefinition)
	if newOperation.SelectionSet.Selections[0] != Selection(first) {
		t.Error("the field that did not change was rebuilt")
	}
	if newOperation.SelectionSet.Selections[1] == Selection(second) {
		t.Error("the field that changed was not rebuilt")
	}
}

// A node can be removed, and a list closes up behind it.
func TestTransform_RemovesANode(t *testing.T) {
	tests := []struct {
		name, query, drop, want string
	}{
		{name: "one of several", query: "{ a b c }", drop: "b", want: "{\n  a\n  c\n}"},
		{name: "the first", query: "{ a b }", drop: "a", want: "{\n  b\n}"},
		{name: "the last", query: "{ a b }", drop: "b", want: "{\n  a\n}"},
		{name: "a nested one", query: "{ a { x y } }", drop: "x", want: "{\n  a {\n    y\n  }\n}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := mustParse(t, tt.query)
			out := transformed(t, doc, Transformer{
				Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
					if field, isField := node.(*Field); isField && field.Name.Value == tt.drop {
						return nil, TransformRemove
					}
					return nil, TransformContinue
				},
			})
			if got := mustPrint(t, out); got != tt.want {
				t.Errorf("wrote\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

// Removing the root leaves nothing.
func TestTransform_RemovesTheRoot(t *testing.T) {
	doc := mustParse(t, "{ a }")
	out := transformed(t, doc, Transformer{
		Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
			if _, isDocument := node.(*Document); isDocument {
				return nil, TransformRemove
			}
			return nil, TransformContinue
		},
	})
	if out != nil {
		t.Errorf("the root came back as %v, want nothing", out)
	}
}

// A transformer that changes nothing gets back the tree it was given, object
// for object.
func TestTransform_UnchangedIsTheSameTree(t *testing.T) {
	doc := mustParse(t, "query Q($v: Int = 1) @d { alias: f(a: {b: [1, 2]}) { g } ...F } fragment F on T { h }")
	var seen int
	out := transformed(t, doc, Transformer{
		Enter: func(Node, VisitContext) (Node, TransformAction) {
			seen++
			return nil, TransformContinue
		},
		Leave: func(Node, VisitContext) (Node, TransformAction) { return nil, TransformContinue },
	})
	if out != Node(doc) {
		t.Error("a rewrite that changed nothing rebuilt the tree")
	}
	if seen == 0 {
		t.Error("nothing was visited")
	}
}

// Enter is shown the node as it was; Leave is shown the one holding whatever
// the rewrite made of its children.
func TestTransform_LeaveSeesTheRewrittenChildren(t *testing.T) {
	doc := mustParse(t, "{ a { x } }")
	var leftWith string
	out := transformed(t, doc, Transformer{
		Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
			if name, isName := node.(*Name); isName && name.Value == "x" {
				return &Name{Value: "y"}, TransformContinue
			}
			return nil, TransformContinue
		},
		Leave: func(node Node, _ VisitContext) (Node, TransformAction) {
			if field, isField := node.(*Field); isField && field.Name.Value == "a" {
				leftWith = Print(field)
			}
			return nil, TransformContinue
		},
	})
	if leftWith != "a {\n  y\n}" {
		t.Errorf("Leave saw %q, want the renamed child", leftWith)
	}
	if got := mustPrint(t, out); got != "{\n  a {\n    y\n  }\n}" {
		t.Errorf("wrote\n%s", got)
	}
}

// A replacement returned from Enter is what the rewrite goes on into, so a
// transformer sees the children of what it put there.
func TestTransform_DescendsIntoTheReplacement(t *testing.T) {
	doc := mustParse(t, "{ a }")
	var names []string
	transformed(t, doc, Transformer{
		Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
			switch n := node.(type) {
			case *Field:
				if n.Name.Value == "a" {
					replaced := mustParse(t, "{ b { c } }").
						Definitions[0].(*OperationDefinition).SelectionSet.Selections[0]
					return replaced, TransformContinue
				}
			case *Name:
				names = append(names, n.Value)
			}
			return nil, TransformContinue
		},
	})
	if len(names) != 2 || names[0] != "b" || names[1] != "c" {
		t.Errorf("names = %v, want the replacement's own children", names)
	}
}

func TestTransform_Skip(t *testing.T) {
	doc := mustParse(t, "{ a { x } b }")
	var seen []string
	out := transformed(t, doc, Transformer{
		Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
			if name, isName := node.(*Name); isName {
				seen = append(seen, name.Value)
			}
			if field, isField := node.(*Field); isField && field.Name.Value == "a" {
				return nil, TransformSkip
			}
			return nil, TransformContinue
		},
	})
	if len(seen) != 1 || seen[0] != "b" {
		t.Errorf("saw %v, want only the name outside the skipped field", seen)
	}
	if out != Node(doc) {
		t.Error("a rewrite that changed nothing rebuilt the tree")
	}
}

func TestTransform_Break(t *testing.T) {
	doc := mustParse(t, "{ a b c }")
	out := transformed(t, doc, Transformer{
		Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
			field, isField := node.(*Field)
			if !isField {
				return nil, TransformContinue
			}
			switch field.Name.Value {
			case "a":
				renamed := *field
				renamed.Name = &Name{Value: "A"}
				return &renamed, TransformContinue
			case "b":
				return nil, TransformBreak
			}
			return nil, TransformContinue
		},
	})
	// What was rewritten before the break stands; the rest is as it was.
	if got, want := mustPrint(t, out), "{\n  A\n  b\n  c\n}"; got != want {
		t.Errorf("wrote\n%s\nwant\n%s", got, want)
	}
}

// Every node type has to be in transformChildren, or a rewrite would silently
// leave part of a tree alone.
//
// The child fields are derived from the struct by reflection, the same way the
// check on visitChildren derives them, so a field added to a node type turns
// this red until the rewrite reaches it too.
func TestTransformChildren_ReachesEveryChildField(t *testing.T) {
	for _, node := range allNodes() {
		t.Run(string(node.Kind()), func(t *testing.T) {
			filled, want := fillNodeChildren(t, node)

			var got []string
			var err error
			transformChildren(filled, func(child Node, key string, index int) Node {
				if index >= 0 {
					got = append(got, fmt.Sprintf("%s[%d]", key, index))
				} else {
					got = append(got, key)
				}
				return child
			}, &err)

			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("transformChildren reached %v, but the struct has child fields %v", got, want)
			}
		})
	}
}

// A node whose children are all replaced comes back holding the replacements,
// for every node type there is. This is what says the table assigns each
// child back to the field it came from rather than to some other one.
func TestTransformChildren_PutsEveryChildBack(t *testing.T) {
	for _, node := range allNodes() {
		t.Run(string(node.Kind()), func(t *testing.T) {
			filled, want := fillNodeChildren(t, node)
			if len(want) == 0 {
				return // a leaf has nothing to put back
			}

			swapped := make(map[string]Node, len(want))
			var err error
			out := transformChildren(filled, func(child Node, key string, index int) Node {
				replacement := placeholderFor(reflect.TypeOf(child))
				at := key
				if index >= 0 {
					at = fmt.Sprintf("%s[%d]", key, index)
				}
				swapped[at] = replacement
				return replacement
			}, &err)
			if out == filled {
				t.Fatal("every child was replaced, but the node was not rebuilt")
			}

			// Reading the rebuilt node back has to find the replacements, and
			// nothing of the original.
			transformChildren(out, func(child Node, key string, index int) Node {
				at := key
				if index >= 0 {
					at = fmt.Sprintf("%s[%d]", key, index)
				}
				if child != swapped[at] {
					t.Errorf("%s holds %p, want the replacement %p", at, child, swapped[at])
				}
				return child
			}, &err)
		})
	}
}

// Transformers run over one rewrite. A node one of them changes is not shown
// to the rest, which is the rule graphql-js follows.
func TestTransformInParallel(t *testing.T) {
	doc := mustParse(t, "{ a b }")

	var secondSaw []string
	renames := Transformer{
		Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
			if name, isName := node.(*Name); isName && name.Value == "a" {
				return &Name{Value: "A"}, TransformContinue
			}
			return nil, TransformContinue
		},
	}
	watches := Transformer{
		Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
			if name, isName := node.(*Name); isName {
				secondSaw = append(secondSaw, name.Value)
			}
			return nil, TransformContinue
		},
	}

	out := transformed(t, doc, TransformInParallel(renames, watches))
	if got, want := mustPrint(t, out), "{\n  A\n  b\n}"; got != want {
		t.Errorf("wrote\n%s\nwant\n%s", got, want)
	}
	if len(secondSaw) != 1 || secondSaw[0] != "b" {
		t.Errorf("the second transformer saw %v, want only the name the first left alone", secondSaw)
	}
}

// Each transformer decides for itself when to stop, and the rewrite goes on
// until they all have.
func TestTransformInParallel_BreakIsPerTransformer(t *testing.T) {
	doc := mustParse(t, "{ a b c }")

	var quick, whole []string
	stops := Transformer{
		Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
			if name, isName := node.(*Name); isName {
				quick = append(quick, name.Value)
				if name.Value == "a" {
					return nil, TransformBreak
				}
			}
			return nil, TransformContinue
		},
	}
	watches := Transformer{
		Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
			if name, isName := node.(*Name); isName {
				whole = append(whole, name.Value)
			}
			return nil, TransformContinue
		},
	}
	transformed(t, doc, TransformInParallel(stops, watches))

	if len(quick) != 1 {
		t.Errorf("the transformer that stopped saw %v, want only the first name", quick)
	}
	if len(whole) != 3 {
		t.Errorf("the one that carried on saw %v, want all three names", whole)
	}
}

// A rewrite that changes nothing gives back the tree it was handed, and does
// not copy the lists along the way — those are allocated only once something
// in one of them actually changes.
//
// It is not free: a node with children is copied so that the rewritten ones
// can be written into it, and the copy is thrown away when they turn out to be
// the same. That is about one allocation per node with children, which this
// pins so that a change costing more than that is noticed. Walking without
// rewriting is [Visit], which allocates nothing to speak of.
func TestTransform_UnchangedCopiesNoLists(t *testing.T) {
	doc := mustParse(t, kitchenSinkForAllocs)
	tr := Transformer{
		Enter: func(Node, VisitContext) (Node, TransformAction) { return nil, TransformContinue },
		Leave: func(Node, VisitContext) (Node, TransformAction) { return nil, TransformContinue },
	}
	// The closure the rewrite installs for each node's children is one
	// allocation per node with children; nothing else should be allocated.
	var nodesWithChildren int
	Visit(doc, Visitor{
		Enter: func(node Node, _ VisitContext) VisitAction {
			visitChildren(node, func(Node, string, int) bool {
				nodesWithChildren++
				return false
			})
			return VisitContinue
		},
	})

	got := testing.AllocsPerRun(20, func() {
		out, err := Transform(doc, tr)
		if err != nil || out != Node(doc) {
			t.Fatalf("out = %v, err = %v", out, err)
		}
	})
	// Room for the ancestors slice to grow, and no more.
	if limit := nodesWithChildren + 8; int(got) > limit {
		t.Errorf("%.0f allocations for a rewrite that changed nothing, want at most %d "+
			"(one per node with children, plus room for the ancestor stack to grow) — "+
			"something is being copied when it need not be", got, limit)
	}
}

const kitchenSinkForAllocs = `
query Q($a: Int = 1, $b: [String!]) @onQuery {
  alias: field(x: 1, y: {k: [1, 2, 3]}) @skip(if: $a) {
    inner
    ... on Thing { deep }
    ...Frag
  }
}
fragment Frag on Thing { a b c }
`
