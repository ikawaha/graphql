package execution_test

import (
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

// firstField returns the one field a document selects, for a test that wants
// the node the executor would be looking at.
func firstField(t *testing.T, query string) *language.Field {
	t.Helper()
	doc, err := language.ParseString(query)
	if err != nil {
		t.Fatal(err)
	}
	op, ok := doc.Definitions[0].(*language.OperationDefinition)
	if !ok {
		t.Fatalf("first definition is %T", doc.Definitions[0])
	}
	field, ok := op.SelectionSet.Selections[0].(*language.Field)
	if !ok {
		t.Fatalf("first selection is %T", op.SelectionSet.Selections[0])
	}
	return field
}

func TestArgumentValues(t *testing.T) {
	s, err := utilities.BuildSchema(`
		type Query { f(a: Int, b: Int = 7, c: String!): String }
	`)
	if err != nil {
		t.Fatal(err)
	}
	def := s.QueryType().Field("f")

	t.Run("what was written, and what was defaulted", func(t *testing.T) {
		args, argErr := execution.ArgumentValues(
			def, firstField(t, `{ f(a: 1, c: "x") }`), schema.VariableValues{})
		if argErr != nil {
			t.Fatal(argErr)
		}
		for _, want := range []struct {
			name     string
			value    any
			supplied bool
		}{
			{"a", int32(1), true},
			{"b", int32(7), true}, // its default
			{"c", "x", true},
		} {
			got, supplied := args.Get(want.name)
			if supplied != want.supplied || got != want.value {
				t.Errorf("%s was (%#v, %v), wanted (%#v, %v)",
					want.name, got, supplied, want.value, want.supplied)
			}
		}
	})

	t.Run("an argument that will not coerce", func(t *testing.T) {
		_, argErr := execution.ArgumentValues(
			def, firstField(t, `{ f(a: "not a number", c: "x") }`), schema.VariableValues{})
		if argErr == nil {
			t.Fatal("no error")
		}
	})

	t.Run("nothing to work from", func(t *testing.T) {
		args, argErr := execution.ArgumentValues(nil, nil, schema.VariableValues{})
		if argErr != nil || args.Len() != 0 {
			t.Errorf("got (%v, %v)", args, argErr)
		}
	})
}

func TestDirectiveValues(t *testing.T) {
	field := firstField(t, `{ f @include(if: true) @deprecated }`)

	t.Run("written", func(t *testing.T) {
		args, written := execution.DirectiveValues(schema.Include, field.Directives, schema.VariableValues{})
		if !written {
			t.Fatal("reported as not written")
		}
		if v, ok := args.Get("if"); !ok || v != true {
			t.Errorf("if was (%#v, %v)", v, ok)
		}
	})

	t.Run("not written", func(t *testing.T) {
		if _, written := execution.DirectiveValues(schema.Skip, field.Directives, schema.VariableValues{}); written {
			t.Error("reported as written")
		}
	})

	t.Run("no definition", func(t *testing.T) {
		if _, written := execution.DirectiveValues(nil, field.Directives, schema.VariableValues{}); written {
			t.Error("reported as written")
		}
	})
}
