package graphql_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ikawaha/graphql"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/validation"
)

func doSchema(t *testing.T) *graphql.Schema {
	t.Helper()
	s, err := graphql.BuildSchema(`
		type Query { greeting(name: String = "world"): String secret: String }
	`)
	if err != nil {
		t.Fatalf("building: %v", err)
	}
	s.QueryType().Field("greeting").Resolve =
		func(_ context.Context, _ any, args graphql.Arguments, _ *graphql.ResolveInfo) (any, error) {
			name, _ := args.Get("name")
			return "hello, " + name.(string), nil
		}
	return s
}

func TestDo(t *testing.T) {
	s := doSchema(t)

	t.Run("a query", func(t *testing.T) {
		got := jsonOf(t, graphql.Do(context.Background(),
			graphql.Params{Schema: s, Query: `{ greeting }`}))
		if want := `{"data":{"greeting":"hello, world"}}`; got != want {
			t.Errorf("= %s, want %s", got, want)
		}
	})

	t.Run("with variables", func(t *testing.T) {
		got := jsonOf(t, graphql.Do(context.Background(), graphql.Params{
			Schema:    s,
			Query:     `query ($name: String) { greeting(name: $name) }`,
			Variables: graphql.Variables(map[string]any{"name": "Ada"}),
		}))
		if want := `{"data":{"greeting":"hello, Ada"}}`; got != want {
			t.Errorf("= %s, want %s", got, want)
		}
	})

	// A variable that was not supplied is not the same as one supplied as
	// null: the first falls back to the argument's default.
	t.Run("a variable that was not supplied", func(t *testing.T) {
		got := jsonOf(t, graphql.Do(context.Background(), graphql.Params{
			Schema: s,
			Query:  `query ($name: String) { greeting(name: $name) }`,
			// The variable is named but held as nothing.
			Variables: map[string]graphql.Maybe[any]{"name": graphql.Nothing[any]()},
		}))
		if want := `{"data":{"greeting":"hello, world"}}`; got != want {
			t.Errorf("= %s, want %s", got, want)
		}
	})

	t.Run("choosing an operation", func(t *testing.T) {
		got := jsonOf(t, graphql.Do(context.Background(), graphql.Params{
			Schema:        s,
			Query:         `query A { greeting } query B { greeting(name: "B") }`,
			OperationName: "B",
		}))
		if want := `{"data":{"greeting":"hello, B"}}`; got != want {
			t.Errorf("= %s, want %s", got, want)
		}
	})

	// A server that parses once and runs many times hands over the document.
	t.Run("an already parsed document", func(t *testing.T) {
		doc, err := language.ParseString(`{ greeting }`)
		if err != nil {
			t.Fatal(err)
		}
		got := jsonOf(t, graphql.Do(context.Background(),
			graphql.Params{Schema: s, Document: doc}))
		if want := `{"data":{"greeting":"hello, world"}}`; got != want {
			t.Errorf("= %s, want %s", got, want)
		}
	})
}

// Anything that goes wrong before a request could run comes back with no data
// at all, which is how a client tells "could not be answered" from "was
// answered incompletely".
func TestDo_ThingsThatGoWrong(t *testing.T) {
	s := doSchema(t)
	tests := []struct {
		name   string
		params graphql.Params
		says   string
	}{
		{"no schema", graphql.Params{Query: `{ greeting }`}, "Must provide a schema"},
		{"no query", graphql.Params{Schema: s}, "Must provide a query"},
		{"text that will not parse", graphql.Params{Schema: s, Query: `{ greeting`}, "Syntax Error"},
		{"a field that is not there", graphql.Params{Schema: s, Query: `{ nope }`}, "Cannot query field"},
		{
			name:   "an unknown operation",
			params: graphql.Params{Schema: s, Query: `query A { greeting }`, OperationName: "B"},
			says:   "Unknown operation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jsonOf(t, graphql.Do(context.Background(), tt.params))
			if !strings.Contains(got, tt.says) {
				t.Errorf("= %s, want it to say %q", got, tt.says)
			}
			if strings.Contains(got, `"data"`) {
				t.Errorf("a request that never ran came back with data: %s", got)
			}
		})
	}
}

func TestDo_Validation(t *testing.T) {
	s := doSchema(t)
	const bad = `{ nope }`

	t.Run("checked by default", func(t *testing.T) {
		got := jsonOf(t, graphql.Do(context.Background(), graphql.Params{Schema: s, Query: bad}))
		if !strings.Contains(got, "Cannot query field") {
			t.Errorf("= %s", got)
		}
	})

	// Skipping the check is for a document already known to be sound. What
	// comes back for one that is not is undefined; it must at least not bring
	// the process down.
	t.Run("skipped", func(t *testing.T) {
		got := jsonOf(t, graphql.Do(context.Background(),
			graphql.Params{Schema: s, Query: bad, SkipValidation: true}))
		if strings.Contains(got, "Cannot query field") {
			t.Errorf("the document was checked after all: %s", got)
		}
	})

	// A rule of a server's own goes alongside the ones the specification
	// requires.
	t.Run("with a rule of its own", func(t *testing.T) {
		rules := append([]validation.Rule{}, validation.SpecifiedRules...)
		rules = append(rules, validation.NoSchemaIntrospectionCustomRule)
		got := jsonOf(t, graphql.Do(context.Background(), graphql.Params{
			Schema: s, Query: `{ __schema { queryType { name } } }`, Rules: rules,
		}))
		if !strings.Contains(got, "introspection has been disabled") {
			t.Errorf("= %s", got)
		}
		// And the specified rules still apply.
		got = jsonOf(t, graphql.Do(context.Background(),
			graphql.Params{Schema: s, Query: bad, Rules: rules}))
		if !strings.Contains(got, "Cannot query field") {
			t.Errorf("= %s", got)
		}
	})

	// An empty list means the ones the specification requires, not "check
	// nothing": leaving a document unchecked is what SkipValidation is for.
	t.Run("an empty list of rules", func(t *testing.T) {
		got := jsonOf(t, graphql.Do(context.Background(),
			graphql.Params{Schema: s, Query: bad, Rules: []validation.Rule{}}))
		if !strings.Contains(got, "Cannot query field") {
			t.Errorf("an empty rule list turned checking off: %s", got)
		}
	})
}

// A parse option bounds what a request can cost before any of it is answered,
// which is what a server exposed to the internet wants.
func TestDo_ParseOptions(t *testing.T) {
	s := doSchema(t)
	got := jsonOf(t, graphql.Do(context.Background(), graphql.Params{
		Schema: s,
		Query:  `{ greeting }`,
		// `{ greeting }` is three tokens, so two is one too few.
		ParseOptions: []language.ParseOption{language.MaxTokens(2)},
	}))
	if !strings.Contains(got, "token") {
		t.Errorf("= %s, want it to say the document was too long", got)
	}
}

func TestBuildSchema(t *testing.T) {
	if _, err := graphql.BuildSchema(`type Query { a: String }`); err != nil {
		t.Errorf("building a sound schema: %v", err)
	}
	t.Run("text that will not parse", func(t *testing.T) {
		if _, err := graphql.BuildSchema(`type Query {`); err == nil {
			t.Error("built from unparseable SDL")
		}
	})
	// A schema that is not sound is refused here rather than failing later at
	// a request that happens to touch the unsound part.
	t.Run("a schema that is not sound", func(t *testing.T) {
		if _, err := graphql.BuildSchema(`type Query { a: Missing }`); err == nil {
			t.Error("built an unsound schema")
		}
	})
}

// The other two ways in refuse a bad request the same way Do does, before
// anything is started.
func TestDoIncrementallyAndSubscribe_ThingsThatGoWrong(t *testing.T) {
	s := doSchema(t)

	t.Run("incrementally", func(t *testing.T) {
		got := graphql.DoIncrementally(context.Background(),
			graphql.Params{Schema: s, Query: `{ nope }`})
		if got.Subsequent != nil {
			t.Error("a stream was returned for a request that never ran")
		}
		if len(got.Initial.Errors) != 1 {
			t.Fatalf("%d errors, want 1", len(got.Initial.Errors))
		}
		if got.Initial.HasNext {
			t.Error("the first payload promises more")
		}
	})

	t.Run("subscribing", func(t *testing.T) {
		got := graphql.Subscribe(context.Background(),
			graphql.Params{Schema: s, Query: `{ nope }`})
		if got.Events != nil {
			t.Error("a stream was returned for a request that never ran")
		}
		if len(got.Errors) != 1 {
			t.Fatalf("%d errors, want 1", len(got.Errors))
		}
	})
}
