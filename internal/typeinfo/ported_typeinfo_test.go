package typeinfo_test

// Ported from graphql-js src/utilities/__tests__/TypeInfo-test.ts: what the
// type stack holds at every step of a walk.
//
// A walk of a whole document touches every kind of node, so what this checks
// is the whole of TypeInfo at once: which type each node is being read
// against, which type encloses it, and which type an input value is expected
// to be.

import (
	"testing"

	"github.com/ikawaha/graphql/internal/typeinfo"
	"github.com/ikawaha/graphql/language"
)

func TestPortedTypeInfo(t *testing.T) {
	s := mustBuild(t, `
  interface Pet {
    name: String
  }

  type Dog implements Pet {
    name: String
  }

  type Cat implements Pet {
    name: String
  }

  type Human {
    name: String
    pets: [Pet]
  }

  type Alien {
    name(surname: Boolean): String
  }

  union HumanOrAlien = Human | Alien

  type QueryRoot {
    human(id: ID): Human
    alien: Alien
    humanOrAlien: HumanOrAlien
    pet: Pet
  }

  schema {
    query: QueryRoot
  }`)

	// step is one call the walk makes: whether it was on the way down or back
	// up, what kind of node it was, the name where the node is one, and what
	// the type stack held.
	type step struct {
		action, kind, name, parent, typ, input string
	}
	want := []step{
		{"enter", "Document", "", "", "", ""},
		{"enter", "OperationDefinition", "", "", "QueryRoot", ""},
		{"enter", "SelectionSet", "", "QueryRoot", "QueryRoot", ""},
		{"enter", "Field", "", "QueryRoot", "Human", ""},
		{"enter", "Name", "human", "QueryRoot", "Human", ""},
		{"leave", "Name", "human", "QueryRoot", "Human", ""},
		{"enter", "Argument", "", "QueryRoot", "Human", "ID"},
		{"enter", "Name", "id", "QueryRoot", "Human", "ID"},
		{"leave", "Name", "id", "QueryRoot", "Human", "ID"},
		{"enter", "IntValue", "", "QueryRoot", "Human", "ID"},
		{"leave", "IntValue", "", "QueryRoot", "Human", "ID"},
		{"leave", "Argument", "", "QueryRoot", "Human", "ID"},
		{"enter", "SelectionSet", "", "Human", "Human", ""},
		{"enter", "Field", "", "Human", "String", ""},
		{"enter", "Name", "name", "Human", "String", ""},
		{"leave", "Name", "name", "Human", "String", ""},
		{"leave", "Field", "", "Human", "String", ""},
		{"enter", "Field", "", "Human", "[Pet]", ""},
		{"enter", "Name", "pets", "Human", "[Pet]", ""},
		{"leave", "Name", "pets", "Human", "[Pet]", ""},
		{"enter", "SelectionSet", "", "Pet", "[Pet]", ""},
		{"enter", "InlineFragment", "", "Pet", "Pet", ""},
		{"enter", "SelectionSet", "", "Pet", "Pet", ""},
		{"enter", "Field", "", "Pet", "String", ""},
		{"enter", "Name", "name", "Pet", "String", ""},
		{"leave", "Name", "name", "Pet", "String", ""},
		{"leave", "Field", "", "Pet", "String", ""},
		{"leave", "SelectionSet", "", "Pet", "Pet", ""},
		{"leave", "InlineFragment", "", "Pet", "Pet", ""},
		{"leave", "SelectionSet", "", "Pet", "[Pet]", ""},
		{"leave", "Field", "", "Human", "[Pet]", ""},
		{"enter", "Field", "", "Human", "", ""},
		{"enter", "Name", "unknown", "Human", "", ""},
		{"leave", "Name", "unknown", "Human", "", ""},
		{"leave", "Field", "", "Human", "", ""},
		{"leave", "SelectionSet", "", "Human", "Human", ""},
		{"leave", "Field", "", "QueryRoot", "Human", ""},
		{"leave", "SelectionSet", "", "QueryRoot", "QueryRoot", ""},
		{"leave", "OperationDefinition", "", "", "QueryRoot", ""},
		{"leave", "Document", "", "", "", ""},
	}

	doc, err := language.ParseString(`{ human(id: 4) { name, pets { ... { name } }, unknown } }`)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	info := typeinfo.NewTypeInfo(s)

	named := func(t language.Node) string {
		if name, isName := t.(*language.Name); isName {
			return name.Value
		}
		return ""
	}
	describe := func(v interface{ String() string }) string {
		if v == nil {
			return ""
		}
		return v.String()
	}

	var got []step
	record := func(action string, node language.Node) {
		var parent, typ, input string
		if p := info.ParentType(); p != nil {
			parent = describe(p)
		}
		if t := info.Type(); t != nil {
			typ = describe(t)
		}
		if i := info.InputType(); i != nil {
			input = describe(i)
		}
		got = append(got, step{action, string(node.Kind()), named(node), parent, typ, input})
	}

	language.Visit(doc, language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			info.Enter(node)
			record("enter", node)
			return language.VisitContinue
		},
		Leave: func(node language.Node, _ language.VisitContext) language.VisitAction {
			record("leave", node)
			info.Leave(node)
			return language.VisitContinue
		},
	})

	if len(got) != len(want) {
		t.Fatalf("%d steps, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("step %d = %v, want %v", i, got[i], want[i])
		}
	}
}
