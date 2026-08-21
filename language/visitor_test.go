package language

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// visitLog records the walk as a list of lines so a test can compare the whole
// sequence at once.
func visitLog(t *testing.T, body string) []string {
	t.Helper()
	doc := mustParse(t, body)
	var log []string
	Visit(doc, Visitor{
		Enter: func(node Node, _ VisitContext) VisitAction {
			log = append(log, "enter "+string(node.Kind()))
			return VisitContinue
		},
		Leave: func(node Node, _ VisitContext) VisitAction {
			log = append(log, "leave "+string(node.Kind()))
			return VisitContinue
		},
	})
	return log
}

func TestVisit_EnterAndLeaveInOrder(t *testing.T) {
	got := visitLog(t, "{ a }")
	want := []string{
		"enter Document",
		"enter OperationDefinition",
		"enter SelectionSet",
		"enter Field",
		"enter Name",
		"leave Name",
		"leave Field",
		"leave SelectionSet",
		"leave OperationDefinition",
		"leave Document",
	}
	if !slices.Equal(got, want) {
		t.Errorf("walk =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Children are visited in the order the grammar writes them, so a walk sees
// the document in source order.
func TestVisit_ChildrenInGrammarOrder(t *testing.T) {
	doc := mustParse(t, "query Q($v: Int) @d { alias: f(a: 1) { g } }")
	var names []string
	Visit(doc, Visitor{
		Enter: func(node Node, _ VisitContext) VisitAction {
			if n, ok := node.(*Name); ok {
				names = append(names, n.Value)
			}
			return VisitContinue
		},
	})
	want := []string{"Q", "v", "Int", "d", "alias", "f", "a", "g"}
	if !slices.Equal(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
}

func TestVisit_Skip(t *testing.T) {
	doc := mustParse(t, "{ a { b c } d }")
	var fields []string
	Visit(doc, Visitor{
		Enter: func(node Node, _ VisitContext) VisitAction {
			f, ok := node.(*Field)
			if !ok {
				return VisitContinue
			}
			fields = append(fields, f.Name.Value)
			if f.Name.Value == "a" {
				return VisitSkip
			}
			return VisitContinue
		},
	})
	if want := []string{"a", "d"}; !slices.Equal(fields, want) {
		t.Errorf("fields = %v, want %v", fields, want)
	}
}

// A node that was skipped is never left, because the walk never went into it.
func TestVisit_SkipDoesNotLeave(t *testing.T) {
	doc := mustParse(t, "{ a }")
	var left []string
	Visit(doc, Visitor{
		Enter: func(node Node, _ VisitContext) VisitAction {
			if node.Kind() == KindField {
				return VisitSkip
			}
			return VisitContinue
		},
		Leave: func(node Node, _ VisitContext) VisitAction {
			left = append(left, string(node.Kind()))
			return VisitContinue
		},
	})
	if slices.Contains(left, string(KindField)) {
		t.Errorf("a skipped field was left: %v", left)
	}
	if !slices.Contains(left, string(KindDocument)) {
		t.Errorf("the document was not left: %v", left)
	}
}

func TestVisit_Break(t *testing.T) {
	doc := mustParse(t, "{ a { b } c }")
	var seen []string
	Visit(doc, Visitor{
		Enter: func(node Node, _ VisitContext) VisitAction {
			seen = append(seen, "enter "+string(node.Kind()))
			if f, ok := node.(*Field); ok && f.Name.Value == "a" {
				return VisitBreak
			}
			return VisitContinue
		},
		Leave: func(node Node, _ VisitContext) VisitAction {
			seen = append(seen, "leave "+string(node.Kind()))
			return VisitContinue
		},
	})
	want := []string{
		"enter Document",
		"enter OperationDefinition",
		"enter SelectionSet",
		"enter Field",
	}
	if !slices.Equal(seen, want) {
		t.Errorf("walk = %v, want %v", seen, want)
	}
}

func TestVisit_Context(t *testing.T) {
	doc := mustParse(t, "{ a b }")

	type record struct {
		kind      Kind
		parent    Kind
		key       string
		index     int
		ancestors int
	}
	var records []record
	Visit(doc, Visitor{
		Enter: func(node Node, ctx VisitContext) VisitAction {
			r := record{kind: node.Kind(), key: ctx.Key, index: ctx.Index, ancestors: len(ctx.Ancestors)}
			if ctx.Parent != nil {
				r.parent = ctx.Parent.Kind()
			}
			records = append(records, r)
			return VisitContinue
		},
	})

	want := []record{
		{kind: KindDocument, parent: "", key: "", index: -1, ancestors: 0},
		{kind: KindOperationDefinition, parent: KindDocument, key: "Definitions", index: 0, ancestors: 1},
		{kind: KindSelectionSet, parent: KindOperationDefinition, key: "SelectionSet", index: -1, ancestors: 2},
		{kind: KindField, parent: KindSelectionSet, key: "Selections", index: 0, ancestors: 3},
		{kind: KindName, parent: KindField, key: "Name", index: -1, ancestors: 4},
		{kind: KindField, parent: KindSelectionSet, key: "Selections", index: 1, ancestors: 3},
		{kind: KindName, parent: KindField, key: "Name", index: -1, ancestors: 4},
	}
	if !slices.Equal(records, want) {
		t.Errorf("records =\n%+v\nwant\n%+v", records, want)
	}
}

// Ancestors run from the root down to the parent.
func TestVisit_Ancestors(t *testing.T) {
	doc := mustParse(t, "{ a { b } }")
	var got []Kind
	Visit(doc, Visitor{
		Enter: func(node Node, ctx VisitContext) VisitAction {
			if f, ok := node.(*Field); ok && f.Name.Value == "b" {
				for _, a := range ctx.Ancestors {
					got = append(got, a.Kind())
				}
			}
			return VisitContinue
		},
	})
	want := []Kind{KindDocument, KindOperationDefinition, KindSelectionSet, KindField, KindSelectionSet}
	if !slices.Equal(got, want) {
		t.Errorf("ancestors = %v, want %v", got, want)
	}
}

func TestVisit_NilRoot(t *testing.T) {
	called := false
	Visit(nil, Visitor{Enter: func(Node, VisitContext) VisitAction {
		called = true
		return VisitContinue
	}})
	if called {
		t.Error("the visitor was called for a nil root")
	}

	// A typed nil must be treated the same way.
	var absent *Field
	Visit(absent, Visitor{Enter: func(Node, VisitContext) VisitAction {
		called = true
		return VisitContinue
	}})
	if called {
		t.Error("the visitor was called for an absent node")
	}
}

func TestVisit_EmptyVisitor(t *testing.T) {
	// Neither function set: the walk must still finish without panicking.
	Visit(mustParse(t, "{ a }"), Visitor{})
}

func TestVisitInParallel(t *testing.T) {
	doc := mustParse(t, "{ a b }")
	var first, second []string
	Visit(doc, VisitInParallel(
		Visitor{Enter: func(n Node, _ VisitContext) VisitAction {
			first = append(first, string(n.Kind()))
			return VisitContinue
		}},
		Visitor{Enter: func(n Node, _ VisitContext) VisitAction {
			second = append(second, string(n.Kind()))
			return VisitContinue
		}},
	))
	if !slices.Equal(first, second) {
		t.Errorf("the two visitors saw different walks:\n%v\n%v", first, second)
	}
	if len(first) == 0 {
		t.Fatal("no nodes visited")
	}
}

// One visitor skipping a subtree must not hide it from the others.
func TestVisitInParallel_SkipIsPerVisitor(t *testing.T) {
	doc := mustParse(t, "{ a { b } c }")
	var skipper, watcher []string

	collect := func(into *[]string, skipName string) Visitor {
		return Visitor{Enter: func(n Node, _ VisitContext) VisitAction {
			f, ok := n.(*Field)
			if !ok {
				return VisitContinue
			}
			*into = append(*into, f.Name.Value)
			if skipName != "" && f.Name.Value == skipName {
				return VisitSkip
			}
			return VisitContinue
		}}
	}
	Visit(doc, VisitInParallel(
		collect(&skipper, "a"),
		collect(&watcher, ""),
	))

	if want := []string{"a", "c"}; !slices.Equal(skipper, want) {
		t.Errorf("skipping visitor saw %v, want %v", skipper, want)
	}
	if want := []string{"a", "b", "c"}; !slices.Equal(watcher, want) {
		t.Errorf("watching visitor saw %v, want %v", watcher, want)
	}
}

// One visitor breaking must not stop the others, but the walk ends once they
// have all broken.
func TestVisitInParallel_BreakIsPerVisitor(t *testing.T) {
	doc := mustParse(t, "{ a b c }")
	var quitter, watcher []string

	Visit(doc, VisitInParallel(
		Visitor{Enter: func(n Node, _ VisitContext) VisitAction {
			if f, ok := n.(*Field); ok {
				quitter = append(quitter, f.Name.Value)
				return VisitBreak
			}
			return VisitContinue
		}},
		Visitor{Enter: func(n Node, _ VisitContext) VisitAction {
			if f, ok := n.(*Field); ok {
				watcher = append(watcher, f.Name.Value)
			}
			return VisitContinue
		}},
	))

	if want := []string{"a"}; !slices.Equal(quitter, want) {
		t.Errorf("breaking visitor saw %v, want %v", quitter, want)
	}
	if want := []string{"a", "b", "c"}; !slices.Equal(watcher, want) {
		t.Errorf("watching visitor saw %v, want %v", watcher, want)
	}
}

func TestVisitInParallel_AllBreakEndsTheWalk(t *testing.T) {
	doc := mustParse(t, "{ a b c }")
	count := 0
	breaker := Visitor{Enter: func(n Node, _ VisitContext) VisitAction {
		if n.Kind() == KindSelectionSet {
			return VisitBreak
		}
		return VisitContinue
	}}
	Visit(doc, VisitInParallel(breaker, breaker, Visitor{
		Enter: func(Node, VisitContext) VisitAction {
			count++
			return VisitContinue
		},
	}))
	// The third visitor keeps going, so the walk continues past the point
	// where the other two stopped.
	if count <= 3 {
		t.Errorf("the walk visited %d nodes, want it to continue past the break", count)
	}
}

// The child table in visitChildren is the only place that knows the shape of
// the tree, and a node type added without an entry would be walked as a leaf
// with no error. This compares the table against the struct fields themselves.
func TestVisitChildren_CoversEveryNodeField(t *testing.T) {
	for _, node := range allNodes() {
		t.Run(string(node.Kind()), func(t *testing.T) {
			filled, want := fillNodeChildren(t, node)

			var got []string
			visitChildren(filled, func(_ Node, key string, index int) bool {
				if index >= 0 {
					got = append(got, fmt.Sprintf("%s[%d]", key, index))
				} else {
					got = append(got, key)
				}
				return true
			})

			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("visitChildren yielded %v, but the struct has child fields %v", got, want)
			}
		})
	}
}

// fillNodeChildren populates every child field of a node with a placeholder and
// returns the node together with the keys those children should be yielded
// under.
func fillNodeChildren(t *testing.T, node Node) (Node, []string) {
	t.Helper()
	nodeType := reflect.TypeOf((*Node)(nil)).Elem()

	// Work on a fresh value so the shared instances from allNodes stay clean.
	v := reflect.New(reflect.TypeOf(node).Elem())
	elem := v.Elem()

	var want []string
	for i := range elem.NumField() {
		field := elem.Type().Field(i)
		if field.Name == "Loc" || !field.IsExported() {
			continue
		}
		switch {
		case field.Type.Implements(nodeType):
			elem.Field(i).Set(reflect.ValueOf(placeholderFor(field.Type)))
			want = append(want, field.Name)
		case field.Type.Kind() == reflect.Slice && field.Type.Elem().Implements(nodeType):
			slice := reflect.MakeSlice(field.Type, 1, 1)
			slice.Index(0).Set(reflect.ValueOf(placeholderFor(field.Type.Elem())))
			elem.Field(i).Set(slice)
			want = append(want, field.Name+"[0]")
		}
	}
	return v.Interface().(Node), want
}

// placeholderFor returns a value that can stand in for a child of the given
// type, whether that type is a concrete node or one of the AST interfaces.
func placeholderFor(t reflect.Type) Node {
	if t.Kind() == reflect.Pointer {
		return reflect.New(t.Elem()).Interface().(Node)
	}
	// An interface field: pick a concrete node that satisfies it.
	for _, candidate := range []Node{
		&Name{}, &NamedType{}, &IntValue{}, &Field{}, &ScalarTypeDefinition{},
		&ScalarTypeExtension{}, &TypeCoordinate{},
	} {
		if reflect.TypeOf(candidate).Implements(t) {
			return candidate
		}
	}
	panic("graphql/language: no placeholder node satisfies " + t.String())
}

func BenchmarkVisit_GitHubSchema(b *testing.B) {
	body, err := readFixtureBytes("github-schema.graphql")
	if err != nil {
		b.Fatal(err)
	}
	doc, err := Parse(NewSource(string(body)))
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	for b.Loop() {
		count := 0
		Visit(doc, Visitor{Enter: func(Node, VisitContext) VisitAction {
			count++
			return VisitContinue
		}})
	}
}

// A walk can be ended on the way back up as well as on the way down, which is
// what graphql-js's BREAK from leave does.
func TestVisit_BreakOnLeave(t *testing.T) {
	doc := mustParse(t, "{ a { b } c }")

	var seen []string
	Visit(doc, Visitor{
		Enter: func(node Node, _ VisitContext) VisitAction {
			seen = append(seen, "enter "+string(node.Kind()))
			return VisitContinue
		},
		Leave: func(node Node, _ VisitContext) VisitAction {
			seen = append(seen, "leave "+string(node.Kind()))
			if f, isField := node.(*Field); isField && f.Name.Value == "b" {
				return VisitBreak
			}
			return VisitContinue
		},
	})

	want := []string{
		"enter Document",
		"enter OperationDefinition",
		"enter SelectionSet",
		"enter Field", // a
		"enter Name",
		"leave Name",
		"enter SelectionSet",
		"enter Field", // b
		"enter Name",
		"leave Name",
		"leave Field", // b — and the walk ends here
	}
	if !slices.Equal(seen, want) {
		t.Errorf("saw\n%v\nwant\n%v", seen, want)
	}
}

// VisitSkip means nothing on the way back up: the children have already been
// walked, so there is nothing left to skip.
func TestVisit_SkipOnLeaveIsIgnored(t *testing.T) {
	doc := mustParse(t, "{ a }")

	var left []string
	Visit(doc, Visitor{
		Leave: func(node Node, _ VisitContext) VisitAction {
			left = append(left, string(node.Kind()))
			return VisitSkip
		},
	})
	if len(left) != 5 {
		t.Errorf("left %v, want every node to have been left", left)
	}
}

// Each visitor of a parallel walk decides for itself when to stop, on the way
// up as well as down, and the walk goes on until they all have.
func TestVisitInParallel_BreakOnLeaveIsPerVisitor(t *testing.T) {
	doc := mustParse(t, "{ a { b } c }")

	var quick, whole []string
	stops := Visitor{
		Leave: func(node Node, _ VisitContext) VisitAction {
			quick = append(quick, string(node.Kind()))
			if f, isField := node.(*Field); isField && f.Name.Value == "b" {
				return VisitBreak
			}
			return VisitContinue
		},
	}
	watches := Visitor{
		Leave: func(node Node, _ VisitContext) VisitAction {
			whole = append(whole, string(node.Kind()))
			return VisitContinue
		},
	}
	Visit(doc, VisitInParallel(stops, watches))

	if len(quick) >= len(whole) {
		t.Errorf("the visitor that stopped saw %d nodes and the other %d; it should have seen fewer",
			len(quick), len(whole))
	}
	if got := whole[len(whole)-1]; got != string(KindDocument) {
		t.Errorf("the visitor that carried on last left %s, want the whole document", got)
	}
}
