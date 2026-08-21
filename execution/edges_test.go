package execution_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// A variable that is wrong deep inside says where, because "the filter is
// wrong" is no help when the filter has a dozen fields.
func TestExecute_VariableErrorNamesThePlace(t *testing.T) {
	s := buildSchema(t, `
		input Filter { terms: [Term] }
		input Term { name: String! weight: Int }
		type Query { search(filter: Filter): String }
	`)
	result := run(t, s, `query ($f: Filter) { search(filter: $f) }`,
		execution.Request{Variables: vars(map[string]any{
			"f": map[string]any{"terms": []any{
				map[string]any{"name": "ok"},
				map[string]any{"weight": 1},
			}},
		})})

	if len(result.Errors) != 1 {
		t.Fatalf("%d errors, want 1:\n%v", len(result.Errors), result.Errors)
	}
	message := result.Errors[0].Message
	if !strings.Contains(message, "$f") {
		t.Errorf("message = %q, want it to name the variable", message)
	}
	// The second entry is the bad one, and the message has to say so.
	if !strings.Contains(message, "terms[1]") {
		t.Errorf("message = %q, want it to point at terms[1]", message)
	}
	if !strings.Contains(message, "name") {
		t.Errorf("message = %q, want it to name the missing field", message)
	}
}

// A scalar decides what it will accept out of a resolver, and a value it will
// not take is the server's fault rather than something to pass on.
func TestExecute_CustomScalarOutput(t *testing.T) {
	s := buildSchema(t, `scalar Even type Query { good: Even bad: Even }`)
	even := s.Type("Even").(*schema.ScalarType)
	even.CoerceOutputValue = func(internal any) (value.Maybe[any], error) {
		n, isInt := internal.(int)
		if !isInt {
			return value.Nothing[any](), errors.New("not a number")
		}
		if n%2 != 0 {
			return value.Nothing[any](), errors.New("not even")
		}
		return value.Just[any](n), nil
	}

	result := run(t, s, `{ good bad }`,
		execution.Request{RootValue: map[string]any{"good": 2, "bad": 3}})

	if got := jsonOf(t, result); !strings.Contains(got, `"good":2`) || !strings.Contains(got, `"bad":null`) {
		t.Errorf("response = %s", got)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("%d errors, want 1", len(result.Errors))
	}
	if !strings.Contains(result.Errors[0].Message, "not even") {
		t.Errorf("message = %q, want it to carry what the scalar said", result.Errors[0].Message)
	}

	// A scalar that returns nothing without saying why still fails the field
	// rather than producing a null the type may not allow.
	t.Run("a coercer that returns nothing", func(t *testing.T) {
		even.CoerceOutputValue = func(any) (value.Maybe[any], error) { return value.Nothing[any](), nil }
		result := run(t, s, `{ good }`, execution.Request{RootValue: map[string]any{"good": 2}})
		if len(result.Errors) != 1 {
			t.Fatalf("%d errors, want 1", len(result.Errors))
		}
	})
}

// Deciding which type a value is happens in a resolver the server wrote, and
// that can fail like any other.
func TestExecute_ResolveTypeFails(t *testing.T) {
	s := buildSchema(t, `
		type Dog { name: String }
		type Cat { name: String }
		union Pet = Dog | Cat
		type Query { pet: Pet }
	`)

	t.Run("the resolver errors", func(t *testing.T) {
		s.Type("Pet").(*schema.UnionType).ResolveType =
			func(context.Context, any, *schema.ResolveInfo) (string, error) {
				return "", errors.New("cannot tell")
			}
		result := run(t, s, `{ pet { __typename } }`,
			execution.Request{RootValue: map[string]any{"pet": map[string]any{}}})
		if len(result.Errors) != 1 {
			t.Fatalf("%d errors, want 1", len(result.Errors))
		}
		if !strings.Contains(result.Errors[0].Message, "cannot tell") {
			t.Errorf("message = %q", result.Errors[0].Message)
		}
	})

	// Naming a type the union does not include is a mistake worth reporting,
	// rather than resolving against a type the document never considered.
	t.Run("the resolver names a type that does not belong", func(t *testing.T) {
		s.Type("Pet").(*schema.UnionType).ResolveType =
			func(context.Context, any, *schema.ResolveInfo) (string, error) {
				return "Query", nil
			}
		result := run(t, s, `{ pet { __typename } }`,
			execution.Request{RootValue: map[string]any{"pet": map[string]any{}}})
		if len(result.Errors) != 1 {
			t.Fatalf("%d errors, want 1", len(result.Errors))
		}
		if !strings.Contains(result.Errors[0].Message, "Query") {
			t.Errorf("message = %q, want it to name the type that was given", result.Errors[0].Message)
		}
	})

	// A value may say what it is, which spares the schema a resolver.
	t.Run("the value says what it is", func(t *testing.T) {
		s.Type("Pet").(*schema.UnionType).ResolveType = nil
		expectJSON(t, s, `{ pet { __typename ... on Cat { name } } }`,
			execution.Request{RootValue: map[string]any{"pet": selfNaming{}}},
			`{"data":{"pet":{"__typename":"Cat","name":"Tom"}}}`)
	})
}

// selfNaming says which GraphQL type it is, rather than being named after it.
type selfNaming struct{}

func (selfNaming) GraphQLTypeName() string { return "Cat" }
func (selfNaming) Name() string            { return "Tom" }

// A source held in a typed map is read like any other map, since a server
// keeping its data in one should not have to unwrap it first.
func TestDefaultResolver_TypedMap(t *testing.T) {
	s := buildSchema(t, `type Query { a: String b: String }`)
	type name = string
	expectJSON(t, s, `{ a b }`,
		execution.Request{RootValue: map[name]string{"a": "A"}},
		`{"data":{"a":"A","b":null}}`)
}

// The set of fields a selection set asks for is a public answer, so it has to
// describe itself.
func TestCollectFields(t *testing.T) {
	s := buildSchema(t, `
		type Query { a: String b: String c: String }
	`)
	doc, err := language.ParseString(`{ a b a ...F } fragment F on Query { c }`)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	operation := doc.Definitions[0].(*language.OperationDefinition)
	fragments := map[string]*language.FragmentDefinition{
		"F": doc.Definitions[1].(*language.FragmentDefinition),
	}

	fields := execution.CollectFields(s, fragments, schema.VariableValues{}, s.QueryType(), operation.SelectionSet)

	if got, want := fields.Len(), 3; got != want {
		t.Errorf("Len = %d, want %d", got, want)
	}
	if got, want := strings.Join(fields.Keys(), ","), "a,b,c"; got != want {
		t.Errorf("Keys = %q, want %q", got, want)
	}
	// The same field asked for twice is one key with both selections.
	if got := len(fields.Fields("a")); got != 2 {
		t.Errorf("a has %d selections, want 2", got)
	}
	if got := len(fields.Fields("missing")); got != 0 {
		t.Errorf("a key that was not asked for has %d selections", got)
	}
}

// A schema with no root for the operation cannot answer it, and says so rather
// than returning an empty response.
func TestExecute_NoRootType(t *testing.T) {
	s := buildSchema(t, `type Query { a: String }`)
	result := runUnvalidated(t, s, `mutation { a }`, execution.Request{})
	if len(result.Errors) != 1 {
		t.Fatalf("%d errors, want 1", len(result.Errors))
	}
	if !strings.Contains(result.Errors[0].Message, "mutation") {
		t.Errorf("message = %q", result.Errors[0].Message)
	}
}

func TestExecute_MissingRequestParts(t *testing.T) {
	s := buildSchema(t, `type Query { a: String }`)
	if got := execution.Execute(context.Background(), execution.Request{}); len(got.Errors) != 1 {
		t.Errorf("a request with no schema gave %d errors, want 1", len(got.Errors))
	}
	if got := execution.Execute(context.Background(), execution.Request{Schema: s}); len(got.Errors) != 1 {
		t.Errorf("a request with no document gave %d errors, want 1", len(got.Errors))
	}
}

// A list may hold anything the resolver produced, including a typed slice
// rather than a slice of any.
func TestExecute_TypedSlice(t *testing.T) {
	s := buildSchema(t, `type Query { names: [String] }`)
	expectJSON(t, s, `{ names }`,
		execution.Request{RootValue: map[string]any{"names": []string{"a", "b"}}},
		`{"data":{"names":["a","b"]}}`)

	// And an entry that is a typed nil is null rather than a fault.
	t.Run("a typed nil entry", func(t *testing.T) {
		s := buildSchema(t, `type Query { people: [Person] } type Person { name: String }`)
		expectJSON(t, s, `{ people { name } }`,
			execution.Request{RootValue: map[string]any{"people": []*person{{Name: "Ada"}, nil}}},
			`{"data":{"people":[{"name":"Ada"},null]}}`)
	})
}

// The variables a request supplies are three-stated, and a Maybe that holds
// nothing means the caller did not supply it at all.
func TestExecute_UnsuppliedMaybe(t *testing.T) {
	s := echoSchema(t)
	expectJSON(t, s, `query ($v: String) { echo(arg: $v) }`,
		execution.Request{Variables: map[string]value.Maybe[any]{"v": value.Nothing[any]()}},
		`{"data":{"echo":"omitted"}}`)
}

// A oneOf input object takes exactly one of its fields, and that has to hold
// for a value arriving in a variable as much as for one written in the
// document: validation cannot see what a variable will contain.
func TestExecute_OneOfInput(t *testing.T) {
	s := buildSchema(t, `
		input Choice @oneOf { byId: ID byName: String }
		type Query { find(by: Choice!): String }
	`)
	s.QueryType().Field("find").Resolve =
		func(_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo) (any, error) {
			by, _ := args.Get("by")
			return describeValue(by), nil
		}

	t.Run("exactly one key", func(t *testing.T) {
		expectJSONContaining(t, s, `query ($c: Choice!) { find(by: $c) }`,
			execution.Request{Variables: vars(map[string]any{"c": map[string]any{"byName": "Ada"}})},
			`"find":"{byName=Ada}"`)
	})

	for _, tt := range []struct {
		name  string
		given map[string]any
	}{
		{"no keys", map[string]any{}},
		{"two keys", map[string]any{"byId": "1", "byName": "Ada"}},
		{"the one key is null", map[string]any{"byName": nil}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result := run(t, s, `query ($c: Choice!) { find(by: $c) }`,
				execution.Request{Variables: vars(map[string]any{"c": tt.given})})
			if len(result.Errors) == 0 {
				t.Fatalf("accepted %v; a oneOf input takes exactly one non-null key", tt.given)
			}
			if got := jsonOf(t, result); strings.Contains(got, `"data"`) {
				t.Errorf("response = %s, want no data: the variables never coerced", got)
			}
		})
	}
}

// What several selections of one field ask for underneath is one selection
// set, which is how `a { x } a { y }` asks for a once with both.
func TestCollectSubfields(t *testing.T) {
	s := buildSchema(t, `
		type User { name: String age: Int city: String }
		type Query { me: User }
	`)
	doc, err := language.ParseString(`
		{ me { name } me { age ...F } }
		fragment F on User { city }
	`)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	operation := doc.Definitions[0].(*language.OperationDefinition)
	fragments := map[string]*language.FragmentDefinition{
		"F": doc.Definitions[1].(*language.FragmentDefinition),
	}

	root := execution.CollectFields(s, fragments, schema.VariableValues{}, s.QueryType(), operation.SelectionSet)
	subfields := execution.CollectSubfields(
		s, fragments, schema.VariableValues{}, s.Type("User").(*schema.ObjectType), root.Nodes("me"))

	if got, want := strings.Join(subfields.Keys(), ","), "name,age,city"; got != want {
		t.Errorf("Keys = %q, want %q", got, want)
	}
	if got := len(subfields.Fields("name")); got != 1 {
		t.Errorf("name has %d selections, want 1", got)
	}
	// Nothing was deferred, so no selection is marked as belonging elsewhere.
	for _, key := range subfields.Keys() {
		for _, selection := range subfields.Fields(key) {
			if selection.Defer != nil {
				t.Errorf("%q was marked deferred by a plain collection", key)
			}
		}
	}
}
