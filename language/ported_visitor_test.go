package language_test

// Ported from graphql-js src/language/__tests__/visitor-test.ts. The cases
// about rewriting nodes as they are visited are left out: this walker reads
// the tree and does not edit it, which COMPATIBILITY.md records.

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ikawaha/graphql/language"
)

// step is one call the walk made: which way it was going, what it was on, and
// where that node sat in its parent.
type step struct {
	action string
	kind   language.Kind
	key    string
	index  int
}

func (s step) String() string {
	return fmt.Sprintf("%s %s at %s[%d]", s.action, s.kind, s.key, s.index)
}

// record walks a tree and returns every step of the walk.
func record(root language.Node) []step {
	var steps []step
	language.Visit(root, language.Visitor{
		Enter: func(node language.Node, ctx language.VisitContext) language.VisitAction {
			steps = append(steps, step{"enter", node.Kind(), ctx.Key, ctx.Index})
			return language.VisitContinue
		},
		Leave: func(node language.Node, ctx language.VisitContext) language.VisitAction {
			steps = append(steps, step{"leave", node.Kind(), ctx.Key, ctx.Index})
			return language.VisitContinue
		},
	})
	return steps
}

func compareSteps(t *testing.T, got, want []step) {
	t.Helper()
	for i := range max(len(got), len(want)) {
		switch {
		case i >= len(got):
			t.Errorf("step %d: missing, want %v", i, want[i])
		case i >= len(want):
			t.Errorf("step %d: %v, want no further steps", i, got[i])
		case got[i] != want[i]:
			t.Errorf("step %d: %v, want %v", i, got[i], want[i])
		}
	}
}

func parsePorted(t *testing.T, body string, opts ...language.ParseOption) *language.Document {
	t.Helper()
	doc, err := language.ParseString(body, opts...)
	if err != nil {
		t.Fatalf("parsing %q: %v", body, err)
	}
	return doc
}

func TestPortedVisitor_WalksInOrder(t *testing.T) {
	want := []step{
		{"enter", "Document", "", -1},
		{"enter", "OperationDefinition", "Definitions", 0},
		{"enter", "SelectionSet", "SelectionSet", -1},
		{"enter", "Field", "Selections", 0},
		{"enter", "Name", "Name", -1},
		{"leave", "Name", "Name", -1},
		{"leave", "Field", "Selections", 0},
		{"leave", "SelectionSet", "SelectionSet", -1},
		{"leave", "OperationDefinition", "Definitions", 0},
		{"leave", "Document", "", -1},
	}
	compareSteps(t, record(parsePorted(t, "{ a }", language.NoLocation())), want)
}

// Every node is entered with the enclosing nodes listed from the root down,
// and the list has shrunk back by the time the node is left.
func TestPortedVisitor_Ancestors(t *testing.T) {
	doc := parsePorted(t, "{ a }", language.NoLocation())

	var open []language.Node
	language.Visit(doc, language.Visitor{
		Enter: func(node language.Node, ctx language.VisitContext) language.VisitAction {
			expectAncestors(t, "entering", node, ctx, open)
			open = append(open, node)
			return language.VisitContinue
		},
		Leave: func(node language.Node, ctx language.VisitContext) language.VisitAction {
			open = open[:len(open)-1]
			expectAncestors(t, "leaving", node, ctx, open)
			return language.VisitContinue
		},
	})
	if len(open) != 0 {
		t.Errorf("%d nodes left open after the walk", len(open))
	}
}

// Ancestors runs from the root down to and including the node's own parent.
// graphql-js stops one short and hands the parent separately; here the parent
// is both the last ancestor and the Parent field, which saves a caller that
// wants the whole chain from having to put it back together.
func expectAncestors(t *testing.T, doing string, node language.Node, ctx language.VisitContext, open []language.Node) {
	t.Helper()
	if len(open) > 0 {
		if ctx.Parent != open[len(open)-1] {
			t.Errorf("%s %s: parent is not the node it is inside", doing, node.Kind())
		}
	} else if ctx.Parent != nil {
		t.Errorf("%s %s: parent is set at the root of the walk", doing, node.Kind())
	}
	if len(ctx.Ancestors) != len(open) {
		t.Errorf("%s %s: %d ancestors, want %d", doing, node.Kind(), len(ctx.Ancestors), len(open))
		return
	}
	for i := range open {
		if ctx.Ancestors[i] != open[i] {
			t.Errorf("%s %s: ancestor %d is not the enclosing node", doing, node.Kind(), i)
		}
	}
}

func TestPortedVisitor_EmptyVisitor(t *testing.T) {
	language.Visit(parsePorted(t, "{ a }", language.NoLocation()), language.Visitor{})
}

// A fragment's own variable definitions and a spread's arguments are part of
// the tree, so a walk reaches them.
func TestPortedVisitor_FragmentArguments(t *testing.T) {
	t.Run("defined on a fragment", func(t *testing.T) {
		doc := parsePorted(t, "fragment Foo($x: Int) on Query { a }",
			language.NoLocation(), language.ExperimentalFragmentArguments())
		want := []step{
			{"enter", "Document", "", -1},
			{"enter", "FragmentDefinition", "Definitions", 0},
			{"enter", "Name", "Name", -1},
			{"leave", "Name", "Name", -1},
			{"enter", "VariableDefinition", "VariableDefinitions", 0},
			{"enter", "Variable", "Variable", -1},
			{"enter", "Name", "Name", -1},
			{"leave", "Name", "Name", -1},
			{"leave", "Variable", "Variable", -1},
			{"enter", "NamedType", "Type", -1},
			{"enter", "Name", "Name", -1},
			{"leave", "Name", "Name", -1},
			{"leave", "NamedType", "Type", -1},
			{"leave", "VariableDefinition", "VariableDefinitions", 0},
			{"enter", "NamedType", "TypeCondition", -1},
			{"enter", "Name", "Name", -1},
			{"leave", "Name", "Name", -1},
			{"leave", "NamedType", "TypeCondition", -1},
			{"enter", "SelectionSet", "SelectionSet", -1},
			{"enter", "Field", "Selections", 0},
			{"enter", "Name", "Name", -1},
			{"leave", "Name", "Name", -1},
			{"leave", "Field", "Selections", 0},
			{"leave", "SelectionSet", "SelectionSet", -1},
			{"leave", "FragmentDefinition", "Definitions", 0},
			{"leave", "Document", "", -1},
		}
		compareSteps(t, record(doc), want)
	})

	t.Run("supplied by a spread", func(t *testing.T) {
		doc := parsePorted(t, "{ ...Foo(x: 1) }",
			language.NoLocation(), language.ExperimentalFragmentArguments())
		want := []step{
			{"enter", "Document", "", -1},
			{"enter", "OperationDefinition", "Definitions", 0},
			{"enter", "SelectionSet", "SelectionSet", -1},
			{"enter", "FragmentSpread", "Selections", 0},
			{"enter", "Name", "Name", -1},
			{"leave", "Name", "Name", -1},
			{"enter", "FragmentArgument", "Arguments", 0},
			{"enter", "Name", "Name", -1},
			{"leave", "Name", "Name", -1},
			{"enter", "IntValue", "Value", -1},
			{"leave", "IntValue", "Value", -1},
			{"leave", "FragmentArgument", "Arguments", 0},
			{"leave", "FragmentSpread", "Selections", 0},
			{"leave", "SelectionSet", "SelectionSet", -1},
			{"leave", "OperationDefinition", "Definitions", 0},
			{"leave", "Document", "", -1},
		}
		compareSteps(t, record(doc), want)
	})
}

// A sub-tree that is skipped is not descended into, and the node that was
// skipped is not left either.
func TestPortedVisitor_SkipsASubTree(t *testing.T) {
	doc := parsePorted(t, "{ a, b { x }, c }", language.NoLocation())

	var steps []step
	language.Visit(doc, language.Visitor{
		Enter: func(node language.Node, ctx language.VisitContext) language.VisitAction {
			steps = append(steps, step{"enter", node.Kind(), ctx.Key, ctx.Index})
			if field, isField := node.(*language.Field); isField && field.Name.Value == "b" {
				return language.VisitSkip
			}
			return language.VisitContinue
		},
		Leave: func(node language.Node, ctx language.VisitContext) language.VisitAction {
			steps = append(steps, step{"leave", node.Kind(), ctx.Key, ctx.Index})
			return language.VisitContinue
		},
	})

	want := []step{
		{"enter", "Document", "", -1},
		{"enter", "OperationDefinition", "Definitions", 0},
		{"enter", "SelectionSet", "SelectionSet", -1},
		{"enter", "Field", "Selections", 0},
		{"enter", "Name", "Name", -1},
		{"leave", "Name", "Name", -1},
		{"leave", "Field", "Selections", 0},
		{"enter", "Field", "Selections", 1},
		{"enter", "Field", "Selections", 2},
		{"enter", "Name", "Name", -1},
		{"leave", "Name", "Name", -1},
		{"leave", "Field", "Selections", 2},
		{"leave", "SelectionSet", "SelectionSet", -1},
		{"leave", "OperationDefinition", "Definitions", 0},
		{"leave", "Document", "", -1},
	}
	compareSteps(t, steps, want)
}

func TestPortedVisitor_EndsEarly(t *testing.T) {
	doc := parsePorted(t, "{ a, b { x }, c }", language.NoLocation())

	var steps []step
	language.Visit(doc, language.Visitor{
		Enter: func(node language.Node, ctx language.VisitContext) language.VisitAction {
			steps = append(steps, step{"enter", node.Kind(), ctx.Key, ctx.Index})
			if name, isName := node.(*language.Name); isName && name.Value == "x" {
				return language.VisitBreak
			}
			return language.VisitContinue
		},
		Leave: func(node language.Node, ctx language.VisitContext) language.VisitAction {
			steps = append(steps, step{"leave", node.Kind(), ctx.Key, ctx.Index})
			return language.VisitContinue
		},
	})

	want := []step{
		{"enter", "Document", "", -1},
		{"enter", "OperationDefinition", "Definitions", 0},
		{"enter", "SelectionSet", "SelectionSet", -1},
		{"enter", "Field", "Selections", 0},
		{"enter", "Name", "Name", -1},
		{"leave", "Name", "Name", -1},
		{"leave", "Field", "Selections", 0},
		{"enter", "Field", "Selections", 1},
		{"enter", "Name", "Name", -1},
		{"leave", "Name", "Name", -1},
		{"enter", "SelectionSet", "SelectionSet", -1},
		{"enter", "Field", "Selections", 0},
		{"enter", "Name", "Name", -1},
	}
	compareSteps(t, steps, want)
}

// Each visitor of a parallel walk decides for itself: one skipping a sub-tree
// does not stop another from seeing it.
func TestPortedVisitor_InParallel(t *testing.T) {
	doc := parsePorted(t, "{ a, b { x }, c }", language.NoLocation())

	var skipped, whole []step
	skipper := language.Visitor{
		Enter: func(node language.Node, ctx language.VisitContext) language.VisitAction {
			skipped = append(skipped, step{"enter", node.Kind(), ctx.Key, ctx.Index})
			if field, isField := node.(*language.Field); isField && field.Name.Value == "b" {
				return language.VisitSkip
			}
			return language.VisitContinue
		},
		Leave: func(node language.Node, ctx language.VisitContext) language.VisitAction {
			skipped = append(skipped, step{"leave", node.Kind(), ctx.Key, ctx.Index})
			return language.VisitContinue
		},
	}
	watcher := language.Visitor{
		Enter: func(node language.Node, ctx language.VisitContext) language.VisitAction {
			whole = append(whole, step{"enter", node.Kind(), ctx.Key, ctx.Index})
			return language.VisitContinue
		},
		Leave: func(node language.Node, ctx language.VisitContext) language.VisitAction {
			whole = append(whole, step{"leave", node.Kind(), ctx.Key, ctx.Index})
			return language.VisitContinue
		},
	}
	language.Visit(doc, language.VisitInParallel(skipper, watcher))

	// The visitor that skipped nothing saw exactly what it would have seen on
	// its own. The one that skipped missed the four nodes under "b" — its
	// name, its selection set, the field inside and that field's name, entered
	// and left — and did not leave "b" either.
	compareSteps(t, whole, record(doc))
	if len(whole)-len(skipped) != 4*2+1 {
		t.Errorf("the skipping visitor saw %d steps and the other %d, a difference of %d; want 9",
			len(skipped), len(whole), len(whole)-len(skipped))
	}
}

// The whole of a large document, node by node. This is the test that says the
// table in visitChildren is right.
func TestPortedVisitor_KitchenSink(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "kitchen-sink-query.graphql"))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	doc := parsePorted(t, string(body))
	compareSteps(t, record(doc), kitchenSinkSteps)
}

var kitchenSinkSteps = []step{
	{"enter", "Document", "", -1},
	{"enter", "OperationDefinition", "Definitions", 0},
	{"enter", "StringValue", "Description", -1},
	{"leave", "StringValue", "Description", -1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "VariableDefinition", "VariableDefinitions", 0},
	{"enter", "StringValue", "Description", -1},
	{"leave", "StringValue", "Description", -1},
	{"enter", "Variable", "Variable", -1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Variable", "Variable", -1},
	{"enter", "NamedType", "Type", -1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "NamedType", "Type", -1},
	{"leave", "VariableDefinition", "VariableDefinitions", 0},
	{"enter", "VariableDefinition", "VariableDefinitions", 1},
	{"enter", "Variable", "Variable", -1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Variable", "Variable", -1},
	{"enter", "NamedType", "Type", -1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "NamedType", "Type", -1},
	{"enter", "EnumValue", "DefaultValue", -1},
	{"leave", "EnumValue", "DefaultValue", -1},
	{"leave", "VariableDefinition", "VariableDefinitions", 1},
	{"enter", "Directive", "Directives", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Directive", "Directives", 0},
	{"enter", "SelectionSet", "SelectionSet", -1},
	{"enter", "Field", "Selections", 0},
	{"enter", "Name", "Alias", -1},
	{"leave", "Name", "Alias", -1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "Argument", "Arguments", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "ListValue", "Value", -1},
	{"enter", "IntValue", "Values", 0},
	{"leave", "IntValue", "Values", 0},
	{"enter", "IntValue", "Values", 1},
	{"leave", "IntValue", "Values", 1},
	{"leave", "ListValue", "Value", -1},
	{"leave", "Argument", "Arguments", 0},
	{"enter", "SelectionSet", "SelectionSet", -1},
	{"enter", "Field", "Selections", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Field", "Selections", 0},
	{"enter", "InlineFragment", "Selections", 1},
	{"enter", "NamedType", "TypeCondition", -1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "NamedType", "TypeCondition", -1},
	{"enter", "Directive", "Directives", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Directive", "Directives", 0},
	{"enter", "SelectionSet", "SelectionSet", -1},
	{"enter", "Field", "Selections", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "SelectionSet", "SelectionSet", -1},
	{"enter", "Field", "Selections", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Field", "Selections", 0},
	{"enter", "Field", "Selections", 1},
	{"enter", "Name", "Alias", -1},
	{"leave", "Name", "Alias", -1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "Argument", "Arguments", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "IntValue", "Value", -1},
	{"leave", "IntValue", "Value", -1},
	{"leave", "Argument", "Arguments", 0},
	{"enter", "Argument", "Arguments", 1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "Variable", "Value", -1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Variable", "Value", -1},
	{"leave", "Argument", "Arguments", 1},
	{"enter", "Directive", "Directives", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "Argument", "Arguments", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "Variable", "Value", -1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Variable", "Value", -1},
	{"leave", "Argument", "Arguments", 0},
	{"leave", "Directive", "Directives", 0},
	{"enter", "SelectionSet", "SelectionSet", -1},
	{"enter", "Field", "Selections", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Field", "Selections", 0},
	{"enter", "FragmentSpread", "Selections", 1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "Directive", "Directives", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Directive", "Directives", 0},
	{"leave", "FragmentSpread", "Selections", 1},
	{"leave", "SelectionSet", "SelectionSet", -1},
	{"leave", "Field", "Selections", 1},
	{"leave", "SelectionSet", "SelectionSet", -1},
	{"leave", "Field", "Selections", 0},
	{"leave", "SelectionSet", "SelectionSet", -1},
	{"leave", "InlineFragment", "Selections", 1},
	{"enter", "InlineFragment", "Selections", 2},
	{"enter", "Directive", "Directives", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "Argument", "Arguments", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "Variable", "Value", -1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Variable", "Value", -1},
	{"leave", "Argument", "Arguments", 0},
	{"leave", "Directive", "Directives", 0},
	{"enter", "SelectionSet", "SelectionSet", -1},
	{"enter", "Field", "Selections", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Field", "Selections", 0},
	{"leave", "SelectionSet", "SelectionSet", -1},
	{"leave", "InlineFragment", "Selections", 2},
	{"enter", "InlineFragment", "Selections", 3},
	{"enter", "SelectionSet", "SelectionSet", -1},
	{"enter", "Field", "Selections", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Field", "Selections", 0},
	{"leave", "SelectionSet", "SelectionSet", -1},
	{"leave", "InlineFragment", "Selections", 3},
	{"leave", "SelectionSet", "SelectionSet", -1},
	{"leave", "Field", "Selections", 0},
	{"leave", "SelectionSet", "SelectionSet", -1},
	{"leave", "OperationDefinition", "Definitions", 0},
	{"enter", "OperationDefinition", "Definitions", 1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "Directive", "Directives", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Directive", "Directives", 0},
	{"enter", "SelectionSet", "SelectionSet", -1},
	{"enter", "Field", "Selections", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "Argument", "Arguments", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "IntValue", "Value", -1},
	{"leave", "IntValue", "Value", -1},
	{"leave", "Argument", "Arguments", 0},
	{"enter", "Directive", "Directives", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Directive", "Directives", 0},
	{"enter", "SelectionSet", "SelectionSet", -1},
	{"enter", "Field", "Selections", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "SelectionSet", "SelectionSet", -1},
	{"enter", "Field", "Selections", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "Directive", "Directives", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Directive", "Directives", 0},
	{"leave", "Field", "Selections", 0},
	{"leave", "SelectionSet", "SelectionSet", -1},
	{"leave", "Field", "Selections", 0},
	{"leave", "SelectionSet", "SelectionSet", -1},
	{"leave", "Field", "Selections", 0},
	{"leave", "SelectionSet", "SelectionSet", -1},
	{"leave", "OperationDefinition", "Definitions", 1},
	{"enter", "OperationDefinition", "Definitions", 2},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "VariableDefinition", "VariableDefinitions", 0},
	{"enter", "Variable", "Variable", -1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Variable", "Variable", -1},
	{"enter", "NamedType", "Type", -1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "NamedType", "Type", -1},
	{"enter", "Directive", "Directives", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Directive", "Directives", 0},
	{"leave", "VariableDefinition", "VariableDefinitions", 0},
	{"enter", "Directive", "Directives", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Directive", "Directives", 0},
	{"enter", "SelectionSet", "SelectionSet", -1},
	{"enter", "Field", "Selections", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "Argument", "Arguments", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "Variable", "Value", -1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Variable", "Value", -1},
	{"leave", "Argument", "Arguments", 0},
	{"enter", "SelectionSet", "SelectionSet", -1},
	{"enter", "Field", "Selections", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "SelectionSet", "SelectionSet", -1},
	{"enter", "Field", "Selections", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "SelectionSet", "SelectionSet", -1},
	{"enter", "Field", "Selections", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Field", "Selections", 0},
	{"leave", "SelectionSet", "SelectionSet", -1},
	{"leave", "Field", "Selections", 0},
	{"enter", "Field", "Selections", 1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "SelectionSet", "SelectionSet", -1},
	{"enter", "Field", "Selections", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Field", "Selections", 0},
	{"leave", "SelectionSet", "SelectionSet", -1},
	{"leave", "Field", "Selections", 1},
	{"leave", "SelectionSet", "SelectionSet", -1},
	{"leave", "Field", "Selections", 0},
	{"leave", "SelectionSet", "SelectionSet", -1},
	{"leave", "Field", "Selections", 0},
	{"leave", "SelectionSet", "SelectionSet", -1},
	{"leave", "OperationDefinition", "Definitions", 2},
	{"enter", "FragmentDefinition", "Definitions", 3},
	{"enter", "StringValue", "Description", -1},
	{"leave", "StringValue", "Description", -1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "NamedType", "TypeCondition", -1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "NamedType", "TypeCondition", -1},
	{"enter", "Directive", "Directives", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Directive", "Directives", 0},
	{"enter", "SelectionSet", "SelectionSet", -1},
	{"enter", "Field", "Selections", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "Argument", "Arguments", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "Variable", "Value", -1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Variable", "Value", -1},
	{"leave", "Argument", "Arguments", 0},
	{"enter", "Argument", "Arguments", 1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "Variable", "Value", -1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Variable", "Value", -1},
	{"leave", "Argument", "Arguments", 1},
	{"enter", "Argument", "Arguments", 2},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "ObjectValue", "Value", -1},
	{"enter", "ObjectField", "Fields", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "StringValue", "Value", -1},
	{"leave", "StringValue", "Value", -1},
	{"leave", "ObjectField", "Fields", 0},
	{"enter", "ObjectField", "Fields", 1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "StringValue", "Value", -1},
	{"leave", "StringValue", "Value", -1},
	{"leave", "ObjectField", "Fields", 1},
	{"leave", "ObjectValue", "Value", -1},
	{"leave", "Argument", "Arguments", 2},
	{"leave", "Field", "Selections", 0},
	{"leave", "SelectionSet", "SelectionSet", -1},
	{"leave", "FragmentDefinition", "Definitions", 3},
	{"enter", "OperationDefinition", "Definitions", 4},
	{"enter", "SelectionSet", "SelectionSet", -1},
	{"enter", "Field", "Selections", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "Argument", "Arguments", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "BooleanValue", "Value", -1},
	{"leave", "BooleanValue", "Value", -1},
	{"leave", "Argument", "Arguments", 0},
	{"enter", "Argument", "Arguments", 1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "BooleanValue", "Value", -1},
	{"leave", "BooleanValue", "Value", -1},
	{"leave", "Argument", "Arguments", 1},
	{"enter", "Argument", "Arguments", 2},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"enter", "NullValue", "Value", -1},
	{"leave", "NullValue", "Value", -1},
	{"leave", "Argument", "Arguments", 2},
	{"leave", "Field", "Selections", 0},
	{"enter", "Field", "Selections", 1},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Field", "Selections", 1},
	{"leave", "SelectionSet", "SelectionSet", -1},
	{"leave", "OperationDefinition", "Definitions", 4},
	{"enter", "OperationDefinition", "Definitions", 5},
	{"enter", "SelectionSet", "SelectionSet", -1},
	{"enter", "Field", "Selections", 0},
	{"enter", "Name", "Name", -1},
	{"leave", "Name", "Name", -1},
	{"leave", "Field", "Selections", 0},
	{"leave", "SelectionSet", "SelectionSet", -1},
	{"leave", "OperationDefinition", "Definitions", 5},
	{"leave", "Document", "", -1},
}

// transformed is the answer of a rewrite that was expected to succeed.
func transformed(t *testing.T, root language.Node, tr language.Transformer) language.Node {
	t.Helper()
	out, err := language.Transform(root, tr)
	if err != nil {
		t.Fatalf("rewriting: %v", err)
	}
	return out
}

// Ported from graphql-js's editing cases. The walker there both reads and
// rewrites; here rewriting is language.Transform, so these run against it.
func TestPortedTransform_Editing(t *testing.T) {
	const source = "{ a, b, c { a, b, c } }"
	const wantEdited = "{\n  a\n  c {\n    a\n    c\n  }\n}"

	dropsB := func(node language.Node, _ language.VisitContext) (language.Node, language.TransformAction) {
		if field, isField := node.(*language.Field); isField && field.Name.Value == "b" {
			return nil, language.TransformRemove
		}
		return nil, language.TransformContinue
	}

	for _, tt := range []struct {
		name string
		t    language.Transformer
	}{
		{name: "editing on enter", t: language.Transformer{Enter: dropsB}},
		{name: "editing on leave", t: language.Transformer{Leave: dropsB}},
		{
			name: "editing on enter and on leave",
			t: language.Transformer{
				Enter: func(node language.Node, _ language.VisitContext) (language.Node, language.TransformAction) {
					if field, isField := node.(*language.Field); isField && field.Name.Value == "b" {
						return nil, language.TransformRemove
					}
					return nil, language.TransformContinue
				},
				Leave: func(node language.Node, _ language.VisitContext) (language.Node, language.TransformAction) {
					return nil, language.TransformContinue
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc := parsePorted(t, source, language.NoLocation())
			out := transformed(t, doc, tt.t)
			if got := language.Print(out); got != wantEdited {
				t.Errorf("wrote\n%s\nwant\n%s", got, wantEdited)
			}
			// The tree that went in is untouched.
			if got := language.Print(doc); got != "{\n  a\n  b\n  c {\n    a\n    b\n    c\n  }\n}" {
				t.Errorf("the original was modified: %s", got)
			}
		})
	}
}

// The root itself can be replaced.
func TestPortedTransform_EditingTheRoot(t *testing.T) {
	doc := parsePorted(t, "{ a }", language.NoLocation())
	other := parsePorted(t, "{ b }", language.NoLocation())

	for _, tt := range []struct {
		name string
		t    language.Transformer
	}{
		{
			name: "on enter",
			t: language.Transformer{
				Enter: func(node language.Node, _ language.VisitContext) (language.Node, language.TransformAction) {
					if _, isDocument := node.(*language.Document); isDocument {
						return other, language.TransformSkip
					}
					return nil, language.TransformContinue
				},
			},
		},
		{
			name: "on leave",
			t: language.Transformer{
				Leave: func(node language.Node, _ language.VisitContext) (language.Node, language.TransformAction) {
					if _, isDocument := node.(*language.Document); isDocument {
						return other, language.TransformContinue
					}
					return nil, language.TransformContinue
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := language.Print(transformed(t, doc, tt.t)); got != "{\n  b\n}" {
				t.Errorf("wrote %s, want the replacement root", got)
			}
		})
	}
}

// A node put in place of another is itself walked into, so a transformer sees
// what it added.
func TestPortedTransform_VisitsTheEditedNode(t *testing.T) {
	added := &language.Field{Name: &language.Name{Value: "__typename"}}
	doc := parsePorted(t, "{ a { x } }", language.NoLocation())

	sawAdded := false
	transformed(t, doc, language.Transformer{
		Enter: func(node language.Node, _ language.VisitContext) (language.Node, language.TransformAction) {
			if node == language.Node(added) {
				sawAdded = true
			}
			field, isField := node.(*language.Field)
			if !isField || field.Name.Value != "a" {
				return nil, language.TransformContinue
			}
			grown := *field
			grown.SelectionSet = &language.SelectionSet{
				Selections: append([]language.Selection{added}, field.SelectionSet.Selections...),
			}
			return &grown, language.TransformContinue
		},
	})
	if !sawAdded {
		t.Error("the added field was not walked into")
	}
}
