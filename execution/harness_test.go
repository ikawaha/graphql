package execution_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/gqlerror"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
	"github.com/ikawaha/graphql/validation"
	"github.com/ikawaha/graphql/value"
)

// buildSchema reads SDL and checks it, so that a test that fails does so
// because of the executor rather than the schema it was given.
func buildSchema(t *testing.T, sdl string) *schema.Schema {
	t.Helper()
	s, err := utilities.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	if err := schema.AssertValidSchema(s); err != nil {
		t.Fatalf("the schema is not sound: %v", err)
	}
	return s
}

// run executes a query, checking first that it is one the schema can answer.
// Execution is written to assume that, so a test that skipped the check would
// be exercising undefined behaviour rather than the executor.
func run(t *testing.T, s *schema.Schema, query string, req execution.Request) execution.Result {
	t.Helper()
	doc, err := language.ParseString(query)
	if err != nil {
		t.Fatalf("parsing the query: %v\n%s", err, query)
	}
	if errs := validation.Validate(s, doc); len(errs) != 0 {
		var b strings.Builder
		for _, e := range errs {
			b.WriteString("\n  " + e.Message)
		}
		t.Fatalf("the test query does not validate:%s\n%s", b.String(), query)
	}
	req.Schema = s
	req.Document = doc
	return execution.Execute(context.Background(), req)
}

// jsonOf renders a result the way it would go over the wire, which is the form
// worth asserting on: it shows the order of the keys, tells null apart from
// absent, and reads as what a client would receive.
func jsonOf(t *testing.T, result execution.Result) string {
	t.Helper()
	out, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("rendering the result: %v", err)
	}
	return string(out)
}

// expectJSON runs a query and compares the whole response.
func expectJSON(t *testing.T, s *schema.Schema, query string, req execution.Request, want string) {
	t.Helper()
	got := jsonOf(t, run(t, s, query, req))
	if got != want {
		t.Errorf("response =\n  %s\nwant\n  %s", got, want)
	}
}

// vars is shorthand for the variables of a request, where every one named is
// supplied. A variable the caller omits is simply absent from the map.
func vars(pairs map[string]any) map[string]value.Maybe[any] {
	out := make(map[string]value.Maybe[any], len(pairs))
	for k, v := range pairs {
		out[k] = value.Just(v)
	}
	return out
}

// expectJSONContaining runs a query and checks the response contains a
// fragment of JSON, for the cases where the errors alongside it are not what
// is being tested.
func expectJSONContaining(t *testing.T, s *schema.Schema, query string, req execution.Request, want string) {
	t.Helper()
	got := jsonOf(t, run(t, s, query, req))
	if !strings.Contains(got, want) {
		t.Errorf("response =\n  %s\nwant it to contain\n  %s", got, want)
	}
}

// pathOf renders where in the response an error happened.
func pathOf(err *gqlerror.Error) string {
	parts := make([]string, len(err.Path))
	for i, step := range err.Path {
		parts[i] = fmt.Sprint(step)
	}
	return strings.Join(parts, ".")
}

// rootWithA is a source for the null-propagation tests.
func rootWithA() map[string]any {
	return map[string]any{
		"a": map[string]any{"x": map[string]any{"deep": "down"}, "y": "why"},
		"b": "bee",
	}
}

// threeThings is a list with a bad entry in the middle.
func threeThings() map[string]any {
	return map[string]any{"things": []any{
		map[string]any{"name": "one"},
		map[string]any{"name": "bad"},
		map[string]any{"name": "three"},
	}}
}

// fmtSprint renders a value for a test message.
func fmtSprint(v any) string { return fmt.Sprint(v) }

// runUnvalidated executes a query without checking it first, for the few tests
// that are about what execution does with a document validation would refuse.
func runUnvalidated(t *testing.T, s *schema.Schema, query string, req execution.Request) execution.Result {
	t.Helper()
	doc, err := language.ParseString(query)
	if err != nil {
		t.Fatalf("parsing the query: %v\n%s", err, query)
	}
	req.Schema = s
	req.Document = doc
	return execution.Execute(context.Background(), req)
}

// fullIntrospectionQuery is what a client sends to discover a schema.
const fullIntrospectionQuery = `
	query IntrospectionQuery {
		__schema {
			queryType { name }
			mutationType { name }
			subscriptionType { name }
			types { ...FullType }
			directives { name description locations args { ...InputValue } }
		}
	}
	fragment FullType on __Type {
		kind name description
		fields(includeDeprecated: true) {
			name description args { ...InputValue } type { ...TypeRef }
			isDeprecated deprecationReason
		}
		inputFields { ...InputValue }
		interfaces { ...TypeRef }
		enumValues(includeDeprecated: true) { name description isDeprecated deprecationReason }
		possibleTypes { ...TypeRef }
	}
	fragment InputValue on __InputValue { name description type { ...TypeRef } defaultValue }
	fragment TypeRef on __Type {
		kind name
		ofType { kind name ofType { kind name ofType { kind name } } }
	}`

// mustParse parses a query for the tests that drive Execute directly.
func mustParse(t *testing.T, query string) *language.Document {
	t.Helper()
	doc, err := language.ParseString(query)
	if err != nil {
		t.Fatalf("parsing the query: %v", err)
	}
	return doc
}
