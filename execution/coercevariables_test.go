package execution_test

import (
	"strings"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
	"github.com/ikawaha/graphql/value"
)

// TestCoerceVariableValues covers the exported way of turning a request's
// variables into the form resolvers see, which a caller doing the executor's
// own job reaches for. The executor uses the bounded form beside it, so
// nothing else here had run this.
func TestCoerceVariableValues(t *testing.T) {
	s, err := utilities.BuildSchema(`
		input Filter { since: Int hidden: Boolean = false }
		type Query { f(a: Int): String }
	`)
	if err != nil {
		t.Fatal(err)
	}
	definitions := func(t *testing.T, operation string) []*language.VariableDefinition {
		t.Helper()
		doc, err := language.ParseString(operation)
		if err != nil {
			t.Fatal(err)
		}
		return doc.Definitions[0].(*language.OperationDefinition).VariableDefinitions
	}

	t.Run("what was supplied, and what was defaulted", func(t *testing.T) {
		got, errs := execution.CoerceVariableValues(s,
			definitions(t, `query ($a: Int, $b: Int = 7, $c: Boolean) { f }`),
			map[string]value.Maybe[any]{"a": value.Just[any](1)})
		if len(errs) != 0 {
			t.Fatalf("errors: %v", errs)
		}
		for _, want := range []struct {
			name     string
			value    any
			supplied bool
		}{
			{"a", int32(1), true},
			{"b", int32(7), true}, // its default
			{"c", nil, false},     // left out, and no default to stand in
		} {
			held, supplied := got.Get(want.name)
			if supplied != want.supplied || (supplied && held != want.value) {
				t.Errorf("%s = (%#v, %v), want (%#v, %v)",
					want.name, held, supplied, want.value, want.supplied)
			}
		}
		// The type each was declared as travels with it.
		if declared := got.TypeOf("a"); declared != schema.Int {
			t.Errorf("the type of a = %v", declared)
		}
	})

	t.Run("null is not absence", func(t *testing.T) {
		got, errs := execution.CoerceVariableValues(s,
			definitions(t, `query ($a: Int = 7) { f }`),
			map[string]value.Maybe[any]{"a": value.Just[any](nil)})
		if len(errs) != 0 {
			t.Fatalf("errors: %v", errs)
		}
		if held, supplied := got.Get("a"); !supplied || held != nil {
			t.Errorf("a = (%#v, %v), want (nil, true)", held, supplied)
		}
	})

	t.Run("every variable is reported on", func(t *testing.T) {
		_, errs := execution.CoerceVariableValues(s,
			definitions(t, `query ($a: Int, $b: Int, $c: Int) { f }`),
			map[string]value.Maybe[any]{
				"a": value.Just[any]("x"),
				"b": value.Just[any]("y"),
				"c": value.Just[any]("z"),
			})
		if len(errs) != 3 {
			t.Fatalf("%d errors, want one for each variable: %v", len(errs), errs)
		}
	})

	t.Run("a type the document names but the schema has not", func(t *testing.T) {
		_, errs := execution.CoerceVariableValues(s,
			definitions(t, `query ($a: Nope) { f }`), nil)
		if len(errs) != 1 {
			t.Fatalf("%d errors, want 1: %v", len(errs), errs)
		}
	})

	t.Run("suggestions can be left out", func(t *testing.T) {
		_, errs := execution.CoerceVariableValues(s,
			definitions(t, `query ($a: Filter) { f }`),
			map[string]value.Maybe[any]{
				"a": value.Just[any](map[string]any{"sinse": 1}),
			},
			schema.WithoutSuggestions())
		if len(errs) != 1 {
			t.Fatalf("%d errors, want 1: %v", len(errs), errs)
		}
		if strings.Contains(errs[0].Message, "Did you mean") {
			t.Errorf("a suggestion was given after all: %s", errs[0].Message)
		}
	})
}
