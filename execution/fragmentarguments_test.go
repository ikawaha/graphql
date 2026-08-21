package execution_test

// graphql-js's own cases for fragment arguments are in
// ported_fragmentarguments_test.go. What is here is the one thing its harness
// cannot show: an argument left out, one given as null, and one given a value
// are three different things, and a fragment variable has to keep them apart
// on the way through.

import (
	"context"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

func TestFragmentArguments_KeepOmittedApartFromNull(t *testing.T) {
	s := buildSchema(t, `type Query { echo(in: String): String }`)
	s.QueryType().Field("echo").Resolve = func(
		_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo,
	) (any, error) {
		v, supplied := args.Get("in")
		switch {
		case !supplied:
			return "(omitted)", nil
		case v == nil:
			return "(null)", nil
		default:
			return value.Describe(v), nil
		}
	}

	for _, tt := range []struct {
		name      string
		query     string
		variables map[string]value.Maybe[any]
		want      string
	}{
		{
			name:  "a value the spread supplied",
			query: `{ ...F(x: "hi") } fragment F($x: String) on Query { echo(in: $x) }`,
			want:  `"hi"`,
		},
		{
			name:  "null, supplied on purpose",
			query: `{ ...F(x: null) } fragment F($x: String) on Query { echo(in: $x) }`,
			want:  "(null)",
		},
		{
			name:  "nothing supplied, and the fragment has no default",
			query: `{ ...F } fragment F($x: String) on Query { echo(in: $x) }`,
			want:  "(omitted)",
		},
		{
			name:  "nothing supplied, so the fragment's own default stands in",
			query: `{ ...F } fragment F($x: String = "fallback") on Query { echo(in: $x) }`,
			want:  `"fallback"`,
		},
		{
			name:      "a request variable the caller gave as null",
			query:     `query ($v: String) { ...F(x: $v) } fragment F($x: String = "fallback") on Query { echo(in: $x) }`,
			variables: map[string]value.Maybe[any]{"v": value.Just[any](nil)},
			want:      "(null)",
		},
		{
			name:      "a request variable the caller left out, so the fragment's default stands in",
			query:     `query ($v: String) { ...F(x: $v) } fragment F($x: String = "fallback") on Query { echo(in: $x) }`,
			variables: nil,
			want:      `"fallback"`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := language.ParseString(tt.query, language.ExperimentalFragmentArguments())
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}
			result := execution.Execute(context.Background(), execution.Request{
				Schema: s, Document: doc, RootValue: map[string]any{}, Variables: tt.variables,
			})
			data, ran := result.Data.Get()
			if !ran {
				t.Fatalf("the request did not run: %v", result.Errors)
			}
			got, _ := data.Get("echo")
			if got != tt.want {
				t.Errorf("echo = %v, want %v", got, tt.want)
			}
		})
	}
}
