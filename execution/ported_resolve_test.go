package execution_test

// Ported from graphql-js src/execution/__tests__/resolve-test.ts, which is
// about where a field's value comes from when the schema gives no resolver.
//
// graphql-js hands a method the arguments and the context value; here the
// context is the context.Context the request was made with, which is where a
// Go program already looks for what the caller brought.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/schema"
)

func TestPortedResolve(t *testing.T) {
	runPorted(t, nil, nil, nil, []portedCase{
		{
			name: `default function accesses properties`,
			sdl:  `type Query { test: String }`, query: `{ test }`,
			root: map[string]any{"test": "testValue"},
			want: `{"data": {"test": "testValue"}}`,
		},
		{
			name: `default function calls methods`,
			sdl:  `type Query { test: String }`, query: `{ test }`,
			root: &keeperOfASecret{secret: "secretValue"},
			want: `{"data": {"test": "secretValue"}}`,
		},
		// The rendered arguments show a Go map's keys in name order, because a
		// Go map has no order of its own; graphql-js shows them in the order
		// the field declares them.
		{
			name:  `uses provided resolve function, with no source and no arguments`,
			built: renderingSchema(t), query: `{ test }`,
			want: `{"data": {"test": "[null,{}]"}}`,
		},
		{
			name:  `uses provided resolve function, with a source`,
			built: renderingSchema(t), query: `{ test }`, root: "Source!",
			want: `{"data": {"test": "[\"Source!\",{}]"}}`,
		},
		{
			name:  `uses provided resolve function, with one argument`,
			built: renderingSchema(t), query: `{ test(aStr: "String!") }`, root: "Source!",
			want: `{"data": {"test": "[\"Source!\",{\"aStr\":\"String!\"}]"}}`,
		},
		{
			name:  `uses provided resolve function, with two arguments`,
			built: renderingSchema(t), query: `{ test(aInt: -123, aStr: "String!") }`, root: "Source!",
			want: `{"data": {"test": "[\"Source!\",{\"aInt\":-123,\"aStr\":\"String!\"}]"}}`,
		},
	})
}

// keeperOfASecret answers a field from something it does not expose.
type keeperOfASecret struct{ secret string }

func (k *keeperOfASecret) Test() string { return k.secret }

// adder answers with what it was built with, what the document asked for, and
// what the caller brought in the context.
type adder struct{ num int }

type addendKey struct{}

func (a *adder) Test(ctx context.Context, args schema.Arguments) int {
	addend1, _ := args.Get("addend1")
	addend2, _ := ctx.Value(addendKey{}).(int)
	return a.num + int(addend1.(int32)) + addend2
}

// renderingSchema answers with the source and the arguments as JSON, so that
// the response shows exactly what the resolver was handed.
func renderingSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s := buildPorted(t, `type Query { test(aStr: String, aInt: Int): String }`)
	s.QueryType().Field("test").Resolve = func(
		_ context.Context, source any, args schema.Arguments, _ *schema.ResolveInfo,
	) (any, error) {
		given := args.Raw()
		if given == nil {
			given = map[string]any{}
		}
		encoded, err := json.Marshal([]any{source, given})
		if err != nil {
			return nil, err
		}
		return string(encoded), nil
	}
	return s
}

// Ported from `default function passes args and context`. What graphql-js
// passes as a context value travels in the context.Context here, so this case
// supplies one rather than going through the shared harness.
func TestPortedResolve_ArgsAndContext(t *testing.T) {
	s := buildPorted(t, `type Query { test(addend1: Int): Int }`)
	ctx := context.WithValue(context.Background(), addendKey{}, 9)
	result := execution.Execute(ctx, execution.Request{
		Schema:    s,
		Document:  mustParse(t, `{ test(addend1: 80) }`),
		RootValue: &adder{num: 700},
	})
	if got, want := mustMarshal(t, result), `{"data":{"test":789}}`; got != want {
		t.Errorf("response = %s, want %s", got, want)
	}
}
