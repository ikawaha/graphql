package language

// A rewrite runs user code over every node of a tree, so the ways a caller can
// get it wrong are worth pinning: a nil where a node was meant, a tree built by
// hand with holes in it, a replacement that is not what the place holds. None
// of them may take the process down.

import "testing"

func expectNoPanic(t *testing.T, name string, f func()) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC: %v", r)
			}
		}()
		f()
	})
}

func TestTransform_SurvivesWhatACallerMayDo(t *testing.T) {
	doc := mustParse(t, "{ a { b } }")

	expectNoPanic(t, "typed-nil replacement from Enter", func() {
		transformed(t, doc, Transformer{
			Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
				if f, ok := node.(*Field); ok && f.Name.Value == "a" {
					return (*Field)(nil), TransformContinue
				}
				return nil, TransformContinue
			},
		})
	})

	expectNoPanic(t, "typed-nil replacement from Leave", func() {
		transformed(t, doc, Transformer{
			Leave: func(node Node, _ VisitContext) (Node, TransformAction) {
				if f, ok := node.(*Field); ok && f.Name.Value == "a" {
					return (*Field)(nil), TransformContinue
				}
				return nil, TransformContinue
			},
		})
	})

	expectNoPanic(t, "typed-nil root", func() {
		transformed(t, (*Document)(nil), Transformer{})
	})

	expectNoPanic(t, "nil root", func() {
		transformed(t, nil, Transformer{})
	})

	expectNoPanic(t, "replacement with nil inner fields", func() {
		transformed(t, doc, Transformer{
			Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
				if f, ok := node.(*Field); ok && f.Name.Value == "a" {
					return &Field{}, TransformContinue
				}
				return nil, TransformContinue
			},
		})
	})

	expectNoPanic(t, "removing a required single child", func() {
		out := transformed(t, doc, Transformer{
			Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
				if _, ok := node.(*Name); ok {
					return nil, TransformRemove
				}
				return nil, TransformContinue
			},
		})
		_ = Print(out)
	})

	expectNoPanic(t, "removing every selection", func() {
		out := transformed(t, doc, Transformer{
			Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
				if _, ok := node.(*Field); ok {
					return nil, TransformRemove
				}
				return nil, TransformContinue
			},
		})
		_ = Print(out)
	})
}

func TestTransform_SurvivesAMalformedTree(t *testing.T) {
	// A tree built by hand, with holes a parser would never leave.
	holed := &Document{Definitions: []Definition{
		&OperationDefinition{
			Operation: OperationQuery,
			SelectionSet: &SelectionSet{Selections: []Selection{
				&Field{},      // no name
				nil,           // a hole in the list
				(*Field)(nil), // a typed hole in the list
				&Field{Name: &Name{Value: "ok"}},
			}},
		},
		nil,
	}}

	expectNoPanic(t, "walking a holed tree", func() {
		Visit(holed, Visitor{
			Enter: func(Node, VisitContext) VisitAction { return VisitContinue },
			Leave: func(Node, VisitContext) VisitAction { return VisitContinue },
		})
	})

	expectNoPanic(t, "rewriting a holed tree", func() {
		transformed(t, holed, Transformer{
			Enter: func(Node, VisitContext) (Node, TransformAction) { return nil, TransformContinue },
			Leave: func(Node, VisitContext) (Node, TransformAction) { return nil, TransformContinue },
		})
	})

	expectNoPanic(t, "removing from a holed list", func() {
		transformed(t, holed, Transformer{
			Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
				if _, ok := node.(*Field); ok {
					return nil, TransformRemove
				}
				return nil, TransformContinue
			},
		})
	})

	expectNoPanic(t, "replacing the root with a leaf", func() {
		out := transformed(t, mustParse(t, "{ a }"), Transformer{
			Enter: func(node Node, _ VisitContext) (Node, TransformAction) {
				if _, ok := node.(*Document); ok {
					return &Name{Value: "x"}, TransformSkip
				}
				return nil, TransformContinue
			},
		})
		_ = Print(out)
	})

	expectNoPanic(t, "breaking on the root", func() {
		transformed(t, mustParse(t, "{ a }"), Transformer{
			Enter: func(Node, VisitContext) (Node, TransformAction) {
				return nil, TransformBreak
			},
		})
	})

	expectNoPanic(t, "removing on leave at the root", func() {
		out := transformed(t, mustParse(t, "{ a }"), Transformer{
			Leave: func(node Node, _ VisitContext) (Node, TransformAction) {
				if _, ok := node.(*Document); ok {
					return nil, TransformRemove
				}
				return nil, TransformContinue
			},
		})
		_ = Print(out)
	})

	expectNoPanic(t, "a transformer with no functions", func() {
		transformed(t, mustParse(t, "{ a }"), Transformer{})
	})

	expectNoPanic(t, "parallel with no transformers", func() {
		transformed(t, mustParse(t, "{ a }"), TransformInParallel())
	})

	expectNoPanic(t, "parallel with an empty transformer", func() {
		transformed(t, mustParse(t, "{ a }"), TransformInParallel(Transformer{}, Transformer{}))
	})

	expectNoPanic(t, "rewriting every kind of node", func() {
		for _, node := range allNodes() {
			transformed(t, node, Transformer{
				Enter: func(Node, VisitContext) (Node, TransformAction) { return nil, TransformContinue },
				Leave: func(Node, VisitContext) (Node, TransformAction) { return nil, TransformContinue },
			})
		}
	})

	expectNoPanic(t, "removing every child of every kind", func() {
		for _, node := range allNodes() {
			out := transformed(t, node, Transformer{
				Enter: func(child Node, ctx VisitContext) (Node, TransformAction) {
					if ctx.Parent == nil {
						return nil, TransformContinue
					}
					return nil, TransformRemove
				},
			})
			_ = Print(out)
		}
	})
}
