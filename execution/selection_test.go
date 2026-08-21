package execution_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

func TestExecute_SkipAndInclude(t *testing.T) {
	s := buildSchema(t, `type Query { a: String b: String }`)
	root := map[string]any{"a": "A", "b": "B"}

	t.Run("skip", func(t *testing.T) {
		expectJSON(t, s, `{ a @skip(if: true) b }`, execution.Request{RootValue: root},
			`{"data":{"b":"B"}}`)
		expectJSON(t, s, `{ a @skip(if: false) b }`, execution.Request{RootValue: root},
			`{"data":{"a":"A","b":"B"}}`)
	})

	t.Run("include", func(t *testing.T) {
		expectJSON(t, s, `{ a @include(if: false) b }`, execution.Request{RootValue: root},
			`{"data":{"b":"B"}}`)
		expectJSON(t, s, `{ a @include(if: true) b }`, execution.Request{RootValue: root},
			`{"data":{"a":"A","b":"B"}}`)
	})

	// Asking for something to be left out is the more specific instruction, so
	// it wins where the two disagree.
	t.Run("skip wins over include", func(t *testing.T) {
		expectJSON(t, s, `{ a @skip(if: true) @include(if: true) b }`,
			execution.Request{RootValue: root}, `{"data":{"b":"B"}}`)
	})

	t.Run("driven by a variable", func(t *testing.T) {
		expectJSON(t, s, `query ($skip: Boolean!) { a @skip(if: $skip) b }`,
			execution.Request{RootValue: root, Variables: vars(map[string]any{"skip": true})},
			`{"data":{"b":"B"}}`)
	})

	t.Run("on a fragment spread", func(t *testing.T) {
		expectJSON(t, s, `{ ...F @skip(if: true) b } fragment F on Query { a }`,
			execution.Request{RootValue: root}, `{"data":{"b":"B"}}`)
	})

	t.Run("on an inline fragment", func(t *testing.T) {
		expectJSON(t, s, `{ ... @skip(if: true) { a } b }`,
			execution.Request{RootValue: root}, `{"data":{"b":"B"}}`)
	})
}

func TestExecute_Fragments(t *testing.T) {
	s := buildSchema(t, `
		interface Named { name: String }
		type Dog implements Named { name: String barks: Boolean }
		type Cat implements Named { name: String meows: Boolean }
		union Pet = Dog | Cat
		type Query { pet: Pet named: Named }
	`)
	// Which object a value is, is decided by its __typename where the source
	// is a map, which is what a resolver returning loose data would provide.
	resolvePet := func(_ context.Context, v any, _ *schema.ResolveInfo) (string, error) {
		return v.(map[string]any)["__typename"].(string), nil
	}
	s.Type("Pet").(*schema.UnionType).ResolveType = resolvePet
	s.Type("Named").(*schema.InterfaceType).ResolveType = resolvePet

	dog := map[string]any{"__typename": "Dog", "name": "Rex", "barks": true}

	t.Run("an inline fragment that applies", func(t *testing.T) {
		expectJSON(t, s, `{ pet { ... on Dog { name barks } } }`,
			execution.Request{RootValue: map[string]any{"pet": dog}},
			`{"data":{"pet":{"name":"Rex","barks":true}}}`)
	})

	// A fragment on a type the value is not contributes nothing, rather than
	// nulls.
	t.Run("an inline fragment that does not apply", func(t *testing.T) {
		expectJSON(t, s, `{ pet { ... on Cat { name meows } ... on Dog { barks } } }`,
			execution.Request{RootValue: map[string]any{"pet": dog}},
			`{"data":{"pet":{"barks":true}}}`)
	})

	t.Run("a named fragment", func(t *testing.T) {
		expectJSON(t, s, `{ pet { ...DogFields } } fragment DogFields on Dog { name barks }`,
			execution.Request{RootValue: map[string]any{"pet": dog}},
			`{"data":{"pet":{"name":"Rex","barks":true}}}`)
	})

	// An interface's own fields are asked of whatever the value turns out to
	// be, alongside the fields of a fragment narrowing it.
	t.Run("through an interface", func(t *testing.T) {
		expectJSON(t, s, `{ named { name ... on Dog { barks } } }`,
			execution.Request{RootValue: map[string]any{"named": dog}},
			`{"data":{"named":{"name":"Rex","barks":true}}}`)
	})

	t.Run("__typename", func(t *testing.T) {
		expectJSON(t, s, `{ pet { __typename } }`,
			execution.Request{RootValue: map[string]any{"pet": dog}},
			`{"data":{"pet":{"__typename":"Dog"}}}`)
	})

	// A fragment that spreads itself would inline for ever; execution has to
	// survive one even though validation reports it.
	t.Run("a fragment cycle terminates", func(t *testing.T) {
		doc := `{ pet { ...A } } fragment A on Dog { name ...A }`
		result := runUnvalidated(t, s, doc, execution.Request{RootValue: map[string]any{"pet": dog}})
		if got := jsonOf(t, result); !strings.Contains(got, `"name":"Rex"`) {
			t.Errorf("response = %s", got)
		}
	})
}

// A value that says which type it is, is believed; failing that each candidate
// is asked; failing that the Go type's own name is taken as the answer.
func TestExecute_ResolvingAbstractTypes(t *testing.T) {
	const sdl = `
		type Dog { name: String }
		type Cat { name: String }
		union Pet = Dog | Cat
		type Query { pet: Pet }
	`

	t.Run("by the type's own resolver", func(t *testing.T) {
		s := buildSchema(t, sdl)
		s.Type("Pet").(*schema.UnionType).ResolveType =
			func(context.Context, any, *schema.ResolveInfo) (string, error) { return "Cat", nil }
		expectJSON(t, s, `{ pet { __typename ... on Cat { name } } }`,
			execution.Request{RootValue: map[string]any{"pet": map[string]any{"name": "Tom"}}},
			`{"data":{"pet":{"__typename":"Cat","name":"Tom"}}}`)
	})

	t.Run("by asking each candidate", func(t *testing.T) {
		s := buildSchema(t, sdl)
		s.Type("Cat").(*schema.ObjectType).IsTypeOf =
			func(_ context.Context, v any, _ *schema.ResolveInfo) (bool, error) {
				_, isCat := v.(catValue)
				return isCat, nil
			}
		expectJSON(t, s, `{ pet { __typename ... on Cat { name } } }`,
			execution.Request{RootValue: map[string]any{"pet": catValue{Name: "Tom"}}},
			`{"data":{"pet":{"__typename":"Cat","name":"Tom"}}}`)
	})

	// A Go type named after the GraphQL one is the common case, and asking
	// costs nothing when the schema has said nothing.
	t.Run("by the Go type's name", func(t *testing.T) {
		s := buildSchema(t, sdl)
		expectJSON(t, s, `{ pet { __typename ... on Dog { name } } }`,
			execution.Request{RootValue: map[string]any{"pet": &Dog{Name: "Rex"}}},
			`{"data":{"pet":{"__typename":"Dog","name":"Rex"}}}`)
	})

	t.Run("when it cannot be worked out", func(t *testing.T) {
		s := buildSchema(t, sdl)
		result := run(t, s, `{ pet { __typename } }`,
			execution.Request{RootValue: map[string]any{"pet": map[string]any{"name": "?"}}})
		if len(result.Errors) != 1 {
			t.Fatalf("%d errors, want 1", len(result.Errors))
		}
		if !strings.Contains(result.Errors[0].Message, "Pet") {
			t.Errorf("message = %q, want it to name the abstract type", result.Errors[0].Message)
		}
	})
}

// Dog is a source value whose Go name stands for its GraphQL type.
type Dog struct{ Name string }

// catValue is recognised by an IsTypeOf rather than by its name.
type catValue struct{ Name string }

func TestExecute_Enums(t *testing.T) {
	s := buildSchema(t, `
		enum Colour { RED GREEN }
		type Query { colour: Colour byName: Colour bad: Colour }
	`)
	// A schema built from SDL gives each member its own name as its value, so
	// returning either the name or the value works.
	root := map[string]any{"colour": "RED", "byName": "GREEN", "bad": "PUCE"}

	result := run(t, s, `{ colour byName bad }`, execution.Request{RootValue: root})
	if got := jsonOf(t, result); !strings.Contains(got, `"colour":"RED"`) ||
		!strings.Contains(got, `"byName":"GREEN"`) || !strings.Contains(got, `"bad":null`) {
		t.Errorf("response = %s", got)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("%d errors, want 1 for the value that is not a member", len(result.Errors))
	}
}

// Whatever a client uses to discover a schema has to work, or no tooling does.
func TestExecute_Introspection(t *testing.T) {
	s := buildSchema(t, `
		"A person."
		type User { name: String }
		type Query { me: User }
	`)

	t.Run("__schema", func(t *testing.T) {
		expectJSON(t, s, `{ __schema { queryType { name } } }`, execution.Request{},
			`{"data":{"__schema":{"queryType":{"name":"Query"}}}}`)
	})

	t.Run("__type", func(t *testing.T) {
		expectJSON(t, s, `{ __type(name: "User") { name kind description fields { name } } }`,
			execution.Request{},
			`{"data":{"__type":{"name":"User","kind":"OBJECT","description":"A person.",`+
				`"fields":[{"name":"name"}]}}}`)
	})

	t.Run("the whole introspection query", func(t *testing.T) {
		result := run(t, s, fullIntrospectionQuery, execution.Request{})
		if len(result.Errors) != 0 {
			t.Fatalf("the introspection query failed:\n%v", result.Errors)
		}
		data, present := result.Data.Get()
		if !present || data == nil {
			t.Fatal("no data")
		}
		got := jsonOf(t, result)
		for _, want := range []string{`"queryType":{"name":"Query"}`, `"name":"User"`, `"name":"String"`} {
			if !strings.Contains(got, want) {
				t.Errorf("the response does not contain %s", want)
			}
		}
	})
}

// A resolver may return an ordered map, which is what a server assembling a
// response by hand would produce.
func TestExecute_OrderedMapSource(t *testing.T) {
	s := buildSchema(t, `type Query { me: User } type User { name: String }`)
	user := value.NewOrderedMap()
	user.Set("name", "Ada")
	expectJSON(t, s, `{ me { name } }`,
		execution.Request{RootValue: map[string]any{"me": user}},
		`{"data":{"me":{"name":"Ada"}}}`)
}
