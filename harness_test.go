package graphql_test

import (
	"context"
	"slices"
	"testing"

	"github.com/ikawaha/graphql"
	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/validation"
)

func harnessSchema(t *testing.T) *graphql.Schema {
	t.Helper()
	s, err := graphql.BuildSchema(`
		directive @defer(if: Boolean, label: String) on FRAGMENT_SPREAD | INLINE_FRAGMENT
		type Query { greeting: String }
		type Subscription { ticks: String }
	`)
	if err != nil {
		t.Fatal(err)
	}
	s.QueryType().Field("greeting").Resolve = func(
		context.Context, any, graphql.Arguments, *graphql.ResolveInfo,
	) (any, error) {
		return "hello", nil
	}
	return s
}

// TestHarness_EachStageIsReachable covers a harness standing in for every
// stage in turn, which is what a server timing them one at a time does.
func TestHarness_EachStageIsReachable(t *testing.T) {
	s := harnessSchema(t)
	ctx := context.Background()

	var seen []string
	note := func(stage string) { seen = append(seen, stage) }
	h := &graphql.Harness{
		Parse: func(query string, opts ...language.ParseOption) (*language.Document, error) {
			note("parse")
			return language.ParseString(query, opts...)
		},
		Validate: func(
			s *schema.Schema, doc *language.Document, opts ...validation.Option,
		) []*gqlerror.Error {
			note("validate")
			return validation.ValidateWithOptions(s, doc, opts...)
		},
		Execute: func(ctx context.Context, req execution.Request) execution.Result {
			note("execute")
			return execution.Execute(ctx, req)
		},
		ExecuteIncrementally: func(
			ctx context.Context, req execution.Request,
		) execution.IncrementalResult {
			note("incremental")
			return execution.ExecuteIncrementally(ctx, req)
		},
		ExecuteLegacyIncrementally: func(
			ctx context.Context, req execution.Request,
		) execution.LegacyIncrementalResult {
			note("legacy")
			return execution.ExecuteLegacyIncrementally(ctx, req)
		},
		Subscribe: func(
			ctx context.Context, req execution.Request,
		) execution.SubscriptionResult {
			note("subscribe")
			return execution.Subscribe(ctx, req)
		},
	}

	tests := []struct {
		name  string
		query string
		run   func(graphql.Params)
		want  []string
	}{
		{"a query", `{ greeting }`, func(p graphql.Params) {
			if r := graphql.Do(ctx, p); len(r.Errors) != 0 {
				t.Fatalf("errors: %v", r.Errors)
			}
		}, []string{"parse", "validate", "execute"}},
		{"a deferred query", `{ ... @defer { greeting } }`, func(p graphql.Params) {
			r := graphql.DoIncrementally(ctx, p)
			for range r.Subsequent { //nolint:revive // drain
			}
		}, []string{"parse", "validate", "incremental"}},
		{"the earlier format", `{ ... @defer { greeting } }`, func(p graphql.Params) {
			r := graphql.DoLegacyIncrementally(ctx, p)
			for range r.Subsequent { //nolint:revive // drain
			}
		}, []string{"parse", "validate", "legacy"}},
		{"a subscription", `subscription { ticks }`, func(p graphql.Params) {
			graphql.Subscribe(ctx, p)
		}, []string{"parse", "validate", "subscribe"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			seen = nil
			test.run(graphql.Params{Schema: s, Query: test.query, Harness: h})
			if !slices.Equal(seen, test.want) {
				t.Errorf("ran %v, wanted %v", seen, test.want)
			}
		})
	}
}

// TestHarness_UnsetStagesFallBack covers a harness that names one stage,
// which is the common case: the rest are the standard ones.
func TestHarness_UnsetStagesFallBack(t *testing.T) {
	s := harnessSchema(t)
	parsed := 0
	result := graphql.Do(context.Background(), graphql.Params{
		Schema: s,
		Query:  `{ greeting }`,
		Harness: &graphql.Harness{
			Parse: func(query string, opts ...language.ParseOption) (*language.Document, error) {
				parsed++
				return language.ParseString(query, opts...)
			},
		},
	})
	if len(result.Errors) != 0 {
		t.Fatalf("errors: %v", result.Errors)
	}
	if parsed != 1 {
		t.Errorf("parsed %d times, wanted once", parsed)
	}
	if data, ok := result.Data.Get(); !ok {
		t.Error("no data")
	} else if v, _ := data.Get("greeting"); v != "hello" {
		t.Errorf("greeting was %v", v)
	}
}

// TestHarness_ParseIsSkippedForAParsedDocument covers a server that parses
// once and runs many times: there is nothing left for a Parse stage to do.
func TestHarness_ParseIsSkippedForAParsedDocument(t *testing.T) {
	s := harnessSchema(t)
	doc, err := language.ParseString(`{ greeting }`)
	if err != nil {
		t.Fatal(err)
	}
	parsed := false
	graphql.Do(context.Background(), graphql.Params{
		Schema:   s,
		Document: doc,
		Harness: &graphql.Harness{
			Parse: func(string, ...language.ParseOption) (*language.Document, error) {
				parsed = true
				return nil, nil
			},
		},
	})
	if parsed {
		t.Error("parsed a document that had already been parsed")
	}
}

// TestHarness_AStageMayRefuse covers a stage of a server's own that turns a
// request away, such as one enforcing a list of documents it will answer.
func TestHarness_AStageMayRefuse(t *testing.T) {
	s := harnessSchema(t)
	result := graphql.Do(context.Background(), graphql.Params{
		Schema: s,
		Query:  `{ greeting }`,
		Harness: &graphql.Harness{
			Validate: func(
				*schema.Schema, *language.Document, ...validation.Option,
			) []*gqlerror.Error {
				return []*gqlerror.Error{gqlerror.New("this document is not on the list.")}
			},
		},
	})
	if len(result.Errors) != 1 || result.Errors[0].Message != "this document is not on the list." {
		t.Fatalf("errors were %v", result.Errors)
	}
	if _, ok := result.Data.Get(); ok {
		t.Error("data came back for a request that was turned away")
	}
}
