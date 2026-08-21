package gqlerror_test

// Ported from graphql-js src/error/__tests__/GraphQLError-test.ts: where an
// error points, how it reads, and how it is written into a response.

import (
	"encoding/json"
	"testing"

	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/language"
)

// portedErrorSource is graphql-js's own three-line document.
func portedErrorSource(t *testing.T) (*language.Source, *language.OperationDefinition, *language.Field) {
	t.Helper()
	source := language.NewSource("{\n  field\n}\n")
	doc, err := language.Parse(source)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	operation, isOperation := doc.Definitions[0].(*language.OperationDefinition)
	if !isOperation {
		t.Fatal("the first definition is not an operation")
	}
	field, isField := operation.SelectionSet.Selections[0].(*language.Field)
	if !isField {
		t.Fatal("the first selection is not a field")
	}
	return source, operation, field
}

func TestPortedGraphQLError_Locations(t *testing.T) {
	source, operation, field := portedErrorSource(t)

	t.Run("a node becomes a position and a location", func(t *testing.T) {
		e := gqlerror.New("msg", gqlerror.WithNodes(field))
		expectAt(t, e, source, 4, 2, 3)
	})

	t.Run("a node that begins at nothing", func(t *testing.T) {
		e := gqlerror.New("msg", gqlerror.WithNodes(operation))
		expectAt(t, e, source, 0, 1, 1)
	})

	t.Run("a node with nowhere to point", func(t *testing.T) {
		e := gqlerror.New("msg", gqlerror.WithNodes(&language.Field{Name: &language.Name{Value: "x"}}))
		if e.Source != nil {
			t.Error("a source was found for a node that has none")
		}
		if len(e.Positions) != 0 || len(e.Locations) != 0 {
			t.Errorf("positions %v and locations %v, want neither", e.Positions, e.Locations)
		}
	})

	t.Run("a source and a position become a location", func(t *testing.T) {
		e := gqlerror.New("msg", gqlerror.WithSource(source), gqlerror.WithPositions(6))
		if len(e.Nodes) != 0 {
			t.Errorf("nodes = %v, want none", e.Nodes)
		}
		expectAt(t, e, source, 6, 2, 5)
	})
}

func expectAt(t *testing.T, e *gqlerror.Error, source *language.Source, position, line, column int) {
	t.Helper()
	if e.Source != source {
		t.Errorf("source = %v, want the one the node came from", e.Source)
	}
	if len(e.Positions) != 1 || e.Positions[0] != position {
		t.Errorf("positions = %v, want [%d]", e.Positions, position)
	}
	if len(e.Locations) != 1 || e.Locations[0].Line != line || e.Locations[0].Column != column {
		t.Errorf("locations = %v, want [%d:%d]", e.Locations, line, column)
	}
}

func TestPortedGraphQLError_JSON(t *testing.T) {
	_, _, field := portedErrorSource(t)

	t.Run("only what there is", func(t *testing.T) {
		expectJSONIs(t, gqlerror.New("msg"), `{"message":"msg"}`)
	})

	t.Run("every field, in the order a response writes them", func(t *testing.T) {
		e := gqlerror.New("msg",
			gqlerror.WithNodes(field),
			gqlerror.WithPath("path", 2, "field"),
			gqlerror.WithExtensions(map[string]any{"foo": "bar"}))
		expectJSONIs(t, e,
			`{"message":"msg","locations":[{"line":2,"column":3}],`+
				`"path":["path",2,"field"],"extensions":{"foo":"bar"}}`)
	})

	t.Run("a path", func(t *testing.T) {
		expectJSONIs(t, gqlerror.New("msg", gqlerror.WithPath("path", 3, "to", "field")),
			`{"message":"msg","path":["path",3,"to","field"]}`)
	})

	t.Run("extensions", func(t *testing.T) {
		expectJSONIs(t, gqlerror.New("msg", gqlerror.WithExtensions(map[string]any{"foo": "bar"})),
			`{"message":"msg","extensions":{"foo":"bar"}}`)
	})
}

func expectJSONIs(t *testing.T, e *gqlerror.Error, want string) {
	t.Helper()
	encoded, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("writing the error: %v", err)
	}
	if string(encoded) != want {
		t.Errorf("wrote %s\nwant %s", encoded, want)
	}
}

func TestPortedGraphQLError_Reads(t *testing.T) {
	t.Run("an error with nowhere to point reads as its message", func(t *testing.T) {
		e := gqlerror.New("Error without location")
		if got := e.Error(); got != "Error without location" {
			t.Errorf("reads as %q, want the message", got)
		}
	})

	t.Run("an error on a node with no location reads as its message", func(t *testing.T) {
		doc, err := language.ParseString("{ foo }", language.NoLocation())
		if err != nil {
			t.Fatalf("parsing: %v", err)
		}
		e := gqlerror.New("Error attached to node without location",
			gqlerror.WithNodes(doc))
		if got := e.Error(); got != "Error attached to node without location" {
			t.Errorf("reads as %q, want the message", got)
		}
	})
}

// TestPortedGraphQLError_NodesFromDifferentSources is graphql-js's "prints an
// error with nodes from different sources": an error blaming a type in one
// document and the same type in another points into both.
func TestPortedGraphQLError_NodesFromDifferentSources(t *testing.T) {
	fieldType := func(name, sdl string) language.Node {
		t.Helper()
		doc, err := language.Parse(language.NewSource(sdl, language.SourceName(name)))
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		definition, isObject := doc.Definitions[0].(*language.ObjectTypeDefinition)
		if !isObject {
			t.Fatalf("%s: the first definition is not an object type", name)
		}
		return definition.Fields[0].Type
	}
	a := fieldType("SourceA", "type Foo {\n  field: String\n}\n")
	b := fieldType("SourceB", "type Foo {\n  field: Int\n}\n")

	e := gqlerror.New("Example error with two nodes", gqlerror.WithNodes(a, b))

	want := "Example error with two nodes\n\n" +
		"SourceA:2:10\n1 | type Foo {\n2 |   field: String\n  |          ^\n3 | }\n\n" +
		"SourceB:2:10\n1 | type Foo {\n2 |   field: Int\n  |          ^\n3 | }"
	if got := e.Error(); got != want {
		t.Errorf("Error() =\n%s\n\nwant\n%s", got, want)
	}

	// Both are reported, each read against the document it came from, rather
	// than the second being dropped for belonging to another source.
	if len(e.Locations) != 2 {
		t.Fatalf("locations = %v, want one for each node", e.Locations)
	}
	for i, loc := range e.Locations {
		if loc.Line != 2 || loc.Column != 10 {
			t.Errorf("location %d = %d:%d, want 2:10", i, loc.Line, loc.Column)
		}
	}
}
