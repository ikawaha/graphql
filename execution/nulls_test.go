package execution_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/schema"
)

// failing makes a field's resolver return an error, for testing what happens
// when one does.
func failing(t *testing.T, s *schema.Schema, typeName, fieldName, message string) {
	t.Helper()
	object, isObject := s.Type(typeName).(*schema.ObjectType)
	if !isObject {
		t.Fatalf("%s is not an object type", typeName)
	}
	field := object.Field(fieldName)
	if field == nil {
		t.Fatalf("%s has no field %s", typeName, fieldName)
	}
	field.Resolve = func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		return nil, errors.New(message)
	}
}

// A field that may not be null cannot be left null when it fails, so the
// failure moves outwards until it reaches somewhere a null can sit. This is
// the part of execution most easily got wrong, and the part a client most
// depends on: it is what makes a non-null type a promise rather than a hint.
func TestExecute_NullPropagation(t *testing.T) {
	const sdl = `
		type Query { a: A b: String }
		type A { x: X! y: String }
		type X { deep: String }
	`

	t.Run("a nullable field stops it", func(t *testing.T) {
		s := buildSchema(t, sdl)
		failing(t, s, "X", "deep", "boom")
		// X.deep is nullable, so the failure stops there.
		expectJSONContaining(t, s, `{ a { x { deep } y } b }`,
			execution.Request{RootValue: rootWithA()},
			`"data":{"a":{"x":{"deep":null},"y":"why"},"b":"bee"}`)
	})

	t.Run("a non-null field passes it up", func(t *testing.T) {
		s := buildSchema(t, sdl)
		failing(t, s, "A", "x", "boom")
		// A.x may not be null, so A cannot stand; A itself is nullable, so the
		// failure stops there and its siblings survive.
		expectJSONContaining(t, s, `{ a { x { deep } y } b }`,
			execution.Request{RootValue: rootWithA()},
			`"data":{"a":null,"b":"bee"}`)
	})

	t.Run("to the root", func(t *testing.T) {
		s := buildSchema(t, `
			type Query { a: A! }
			type A { x: X! }
			type X { deep: String }
		`)
		failing(t, s, "A", "x", "boom")
		// Nothing between the failure and the root can hold a null, so the
		// whole response is null. It is null rather than absent: the request
		// did run.
		result := run(t, s, `{ a { x { deep } } }`, execution.Request{RootValue: rootWithA()})
		if got := jsonOf(t, result); !strings.Contains(got, `"data":null`) {
			t.Errorf("response = %s, want data to be null", got)
		}
		if len(result.Errors) != 1 {
			t.Fatalf("%d errors, want 1", len(result.Errors))
		}
		// The error still points at where it happened, not at the root.
		if got, want := pathOf(result.Errors[0]), "a.x"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	})

	// A resolver that returns nothing for a field that may not be null is the
	// same fault as one that fails, and has to be reported rather than
	// producing a response that breaks its own contract.
	t.Run("returning null where null is not allowed", func(t *testing.T) {
		s := buildSchema(t, `type Query { required: String! other: String }`)
		result := run(t, s, `{ required other }`,
			execution.Request{RootValue: map[string]any{"required": nil, "other": "here"}})

		if got := jsonOf(t, result); !strings.Contains(got, `"data":null`) {
			t.Errorf("response = %s, want data to be null", got)
		}
		if len(result.Errors) != 1 {
			t.Fatalf("%d errors, want 1", len(result.Errors))
		}
		if !strings.Contains(result.Errors[0].Message, "Cannot return null") &&
			!strings.Contains(result.Errors[0].Message, "cannot return null") {
			t.Errorf("message = %q, want it to say null is not allowed", result.Errors[0].Message)
		}
	})
}

// A list of nullable entries loses only the entry that failed; a list whose
// entries may not be null loses the whole list.
func TestExecute_NullPropagationInLists(t *testing.T) {
	t.Run("a bad entry among nullable ones", func(t *testing.T) {
		s := buildSchema(t, `
			type Query { things: [Thing] }
			type Thing { name: String }
		`)
		s.Type("Thing").(*schema.ObjectType).Field("name").Resolve =
			func(_ context.Context, source any, _ schema.Arguments, _ *schema.ResolveInfo) (any, error) {
				name := source.(map[string]any)["name"].(string)
				if name == "bad" {
					return nil, errors.New("boom")
				}
				return name, nil
			}
		expectJSONContaining(t, s, `{ things { name } }`,
			execution.Request{RootValue: threeThings()},
			`"data":{"things":[{"name":"one"},{"name":null},{"name":"three"}]}`)
	})

	t.Run("a bad entry where entries may not be null", func(t *testing.T) {
		s := buildSchema(t, `
			type Query { things: [Thing!] }
			type Thing { name: String! }
		`)
		s.Type("Thing").(*schema.ObjectType).Field("name").Resolve =
			func(_ context.Context, source any, _ schema.Arguments, _ *schema.ResolveInfo) (any, error) {
				name := source.(map[string]any)["name"].(string)
				if name == "bad" {
					return nil, errors.New("boom")
				}
				return name, nil
			}
		// One entry cannot be null, so the list cannot stand; the list itself
		// is nullable, so the failure stops there.
		expectJSONContaining(t, s, `{ things { name } }`,
			execution.Request{RootValue: threeThings()},
			`"data":{"things":null}`)
	})

	// The path says which entry it was, which is the only way to tell in a
	// list of many.
	t.Run("the error names the entry", func(t *testing.T) {
		s := buildSchema(t, `
			type Query { things: [Thing] }
			type Thing { name: String }
		`)
		s.Type("Thing").(*schema.ObjectType).Field("name").Resolve =
			func(_ context.Context, source any, _ schema.Arguments, _ *schema.ResolveInfo) (any, error) {
				if source.(map[string]any)["name"] == "bad" {
					return nil, errors.New("boom")
				}
				return source.(map[string]any)["name"], nil
			}
		result := run(t, s, `{ things { name } }`, execution.Request{RootValue: threeThings()})
		if len(result.Errors) != 1 {
			t.Fatalf("%d errors, want 1", len(result.Errors))
		}
		if got, want := pathOf(result.Errors[0]), "things.1.name"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	})
}

// A field whose type is a list but whose resolver returned something else is a
// fault in the server, and saying so beats putting the value through anyway.
func TestExecute_NotAList(t *testing.T) {
	s := buildSchema(t, `type Query { things: [String] }`)
	result := run(t, s, `{ things }`,
		execution.Request{RootValue: map[string]any{"things": "not a list"}})

	if len(result.Errors) != 1 {
		t.Fatalf("%d errors, want 1", len(result.Errors))
	}
	if !strings.Contains(result.Errors[0].Message, "Iterable") {
		t.Errorf("message = %q, want it to say a list was expected", result.Errors[0].Message)
	}
}
