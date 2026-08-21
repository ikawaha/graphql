package execution_test

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// echoSchema reports exactly what its resolver was given, which is how the
// three states an argument can be in are told apart.
func echoSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s := buildSchema(t, `
		input Filter { term: String limit: Int = 5 }
		type Query {
			echo(arg: String): String
			echoWithDefault(arg: String = "fallback"): String
			echoInt(arg: Int!): String
			echoFilter(arg: Filter): String
		}
	`)
	describe := func(_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo) (any, error) {
		v, given := args.Get("arg")
		if !given {
			return "omitted", nil
		}
		if v == nil {
			return "null", nil
		}
		return "value:" + describeValue(v), nil
	}
	query := s.QueryType()
	for _, name := range []string{"echo", "echoWithDefault", "echoInt", "echoFilter"} {
		query.Field(name).Resolve = describe
	}
	return s
}

// describeValue renders what a resolver received, so a test can assert on it.
func describeValue(v any) string {
	switch typed := v.(type) {
	case string:
		return typed
	case *value.OrderedMap:
		var parts []string
		for k, entry := range typed.All() {
			parts = append(parts, k+"="+describeValue(entry))
		}
		return "{" + strings.Join(parts, ",") + "}"
	case map[string]any:
		// The fields of an input object carry no order, so they are sorted
		// here rather than left to the map's iteration order, which would make
		// the test flaky rather than the implementation wrong.
		keys := slices.Sorted(maps.Keys(typed))
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = k + "=" + describeValue(typed[k])
		}
		return "{" + strings.Join(parts, ",") + "}"
	case nil:
		return "null"
	default:
		return strings.TrimSpace(strings.ReplaceAll(fmtSprint(typed), "\n", " "))
	}
}

// An argument left out, one given as null, and one given a value are three
// different things, and a resolver has to be able to tell them apart. This is
// the distinction the whole port was shaped around.
func TestExecute_ArgumentsAreThreeStated(t *testing.T) {
	s := echoSchema(t)

	t.Run("omitted", func(t *testing.T) {
		expectJSON(t, s, `{ echo }`, execution.Request{},
			`{"data":{"echo":"omitted"}}`)
	})

	t.Run("given as null", func(t *testing.T) {
		expectJSON(t, s, `{ echo(arg: null) }`, execution.Request{},
			`{"data":{"echo":"null"}}`)
	})

	t.Run("given a value", func(t *testing.T) {
		expectJSON(t, s, `{ echo(arg: "hi") }`, execution.Request{},
			`{"data":{"echo":"value:hi"}}`)
	})

	// An argument left out falls back to its default; one given as null does
	// not, because null is a value the caller chose.
	t.Run("a default stands in only for an omission", func(t *testing.T) {
		expectJSON(t, s, `{ echoWithDefault }`, execution.Request{},
			`{"data":{"echoWithDefault":"value:fallback"}}`)
		expectJSON(t, s, `{ echoWithDefault(arg: null) }`, execution.Request{},
			`{"data":{"echoWithDefault":"null"}}`)
	})
}

// The same three states apply to variables, and they have to carry through to
// the argument the variable feeds.
func TestExecute_VariablesAreThreeStated(t *testing.T) {
	s := echoSchema(t)
	const query = `query ($v: String) { echo(arg: $v) }`

	t.Run("the variable was not supplied", func(t *testing.T) {
		// Writing `arg: $v` and supplying nothing asks for whatever the field
		// does without the argument, which is not the same as asking for null.
		expectJSON(t, s, query, execution.Request{},
			`{"data":{"echo":"omitted"}}`)
	})

	t.Run("the variable was supplied as null", func(t *testing.T) {
		expectJSON(t, s, query,
			execution.Request{Variables: map[string]value.Maybe[any]{"v": value.Just[any](nil)}},
			`{"data":{"echo":"null"}}`)
	})

	t.Run("the variable was supplied a value", func(t *testing.T) {
		expectJSON(t, s, query, execution.Request{Variables: vars(map[string]any{"v": "hi"})},
			`{"data":{"echo":"value:hi"}}`)
	})

	// A variable left out falls back to its own default before the argument's.
	t.Run("the variable has a default", func(t *testing.T) {
		expectJSON(t, s, `query ($v: String = "from the variable") { echo(arg: $v) }`,
			execution.Request{},
			`{"data":{"echo":"value:from the variable"}}`)
	})
}

func TestExecute_VariableErrors(t *testing.T) {
	s := echoSchema(t)

	// A request whose variables will not coerce never began, so the response
	// has no data at all rather than null data.
	t.Run("a required variable not supplied", func(t *testing.T) {
		result := run(t, s, `query ($v: Int!) { echoInt(arg: $v) }`, execution.Request{})
		if got := jsonOf(t, result); strings.Contains(got, `"data"`) {
			t.Errorf("response = %s, want no data at all", got)
		}
		if len(result.Errors) != 1 {
			t.Fatalf("%d errors, want 1", len(result.Errors))
		}
		if !strings.Contains(result.Errors[0].Message, `non-null type "Int!" to be provided`) {
			t.Errorf("message = %q", result.Errors[0].Message)
		}
	})

	t.Run("a variable of the wrong type", func(t *testing.T) {
		result := run(t, s, `query ($v: Int!) { echoInt(arg: $v) }`,
			execution.Request{Variables: vars(map[string]any{"v": "not a number"})})
		if len(result.Errors) != 1 {
			t.Fatalf("%d errors, want 1:\n%v", len(result.Errors), result.Errors)
		}
		if !strings.Contains(result.Errors[0].Message, "$v") {
			t.Errorf("message = %q, want it to name the variable", result.Errors[0].Message)
		}
	})

	// Every variable is reported on, so a caller fixing a request sees all of
	// it at once.
	t.Run("several bad variables at once", func(t *testing.T) {
		result := run(t, s, `query ($a: Int!, $b: Int!) { one: echoInt(arg: $a) two: echoInt(arg: $b) }`,
			execution.Request{})
		if len(result.Errors) != 2 {
			t.Fatalf("%d errors, want 2:\n%v", len(result.Errors), result.Errors)
		}
	})

	// An input object arrives with its fields coerced, and a field it left out
	// takes the default the schema gave it.
	t.Run("an input object with a default field", func(t *testing.T) {
		expectJSONContaining(t, s, `{ echoFilter(arg: { term: "x" }) }`, execution.Request{},
			`"echoFilter":"value:{limit=5,term=x}"`)
	})
}
