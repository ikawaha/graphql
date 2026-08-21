package execution_test

// Ported from graphql-js src/execution/__tests__/errorPropagation-test.ts, and
// extended: an operation may ask that a field which cannot be null be answered
// with null when it fails, rather than the failure travelling up.

import (
	"context"
	"errors"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

// errorPropagationSDL is graphql-js's own schema, with more to fail in it.
const errorPropagationSDL = `
	directive @experimental_disableErrorPropagation on QUERY | MUTATION | SUBSCRIPTION

	type Query {
		foo: Int!
		ok: Int
		nested: Nested
		list: [Int!]
	}
	type Nested {
		inner: Int!
		beside: Int
	}
	type Mutation {
		change: Int!
	}
	type Subscription {
		events: Int!
	}`

// failing answers every field with the same error, so that what differs
// between the cases is only where the failure comes to rest.
func failingErrorPropagationSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s := buildSchema(t, errorPropagationSDL)
	bar := func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		return nil, errors.New("bar")
	}
	s.QueryType().Field("foo").Resolve = bar
	s.QueryType().Field("ok").Resolve = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		return 1, nil
	}
	s.QueryType().Field("nested").Resolve = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		return map[string]any{}, nil
	}
	s.QueryType().Field("list").Resolve = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		return []any{1, nil, 3}, nil
	}
	nested := s.Type("Nested").(*schema.ObjectType)
	nested.Field("inner").Resolve = bar
	nested.Field("beside").Resolve = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		return 2, nil
	}
	s.MutationType().Field("change").Resolve = bar
	return s
}

func TestPortedErrorPropagation(t *testing.T) {
	s := failingErrorPropagationSchema(t)
	for _, tt := range []struct{ name, query, want string }{
		{
			name: "without the directive the error propagates",
			query: `
      query getFoo {
        foo
      }
    `,
			want: `{"errors":[{"message":"bar","locations":[{"line":3,"column":9}],` +
				`"path":["foo"]}],"data":null}`,
		},
		{
			name: "with the directive the field is null",
			query: `
      query getFoo @experimental_disableErrorPropagation {
        foo
      }
    `,
			want: `{"errors":[{"message":"bar","locations":[{"line":3,"column":9}],` +
				`"path":["foo"]}],"data":{"foo":null}}`,
		},
		{
			name:  "what did resolve is kept beside what did not",
			query: `query @experimental_disableErrorPropagation { ok foo }`,
			want: `{"errors":[{"message":"bar","locations":[{"line":1,"column":50}],` +
				`"path":["foo"]}],"data":{"ok":1,"foo":null}}`,
		},
		{
			name:  "an object is not nulled by what is inside it",
			query: `query @experimental_disableErrorPropagation { nested { inner beside } }`,
			want: `{"errors":[{"message":"bar","locations":[{"line":1,"column":56}],` +
				`"path":["nested","inner"]}],"data":{"nested":{"inner":null,"beside":2}}}`,
		},
		{
			name:  "an entry of a list of non-nulls",
			query: `query @experimental_disableErrorPropagation { list }`,
			want: `{"errors":[{"message":"Cannot return null for non-nullable field Query.list.",` +
				`"locations":[{"line":1,"column":47}],"path":["list",1]}],` +
				`"data":{"list":[1,null,3]}}`,
		},
		{
			name:  "the same list without the directive",
			query: `query { list }`,
			want: `{"errors":[{"message":"Cannot return null for non-nullable field Query.list.",` +
				`"locations":[{"line":1,"column":9}],"path":["list",1]}],"data":{"list":null}}`,
		},
		{
			name:  "a mutation may ask for it too",
			query: `mutation @experimental_disableErrorPropagation { change }`,
			want: `{"errors":[{"message":"bar","locations":[{"line":1,"column":50}],` +
				`"path":["change"]}],"data":{"change":null}}`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result := execution.Execute(context.Background(), execution.Request{
				Schema: s, Document: mustParse(t, tt.query),
			})
			if got := jsonOf(t, result); got != tt.want {
				t.Errorf("response = %s\nwant      %s", got, tt.want)
			}
		})
	}
}

// A directive the schema never declared is not one a request can invoke.
// graphql-js reads the name off the operation without asking the schema;
// validation refuses such a document either way, so the two only differ for a
// document that skipped it. COMPATIBILITY.md records this.
func TestErrorPropagation_NeedsTheSchemaToDeclareIt(t *testing.T) {
	s := buildSchema(t, `type Query { foo: Int! }`)
	s.QueryType().Field("foo").Resolve = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		return nil, errors.New("bar")
	}

	result := execution.Execute(context.Background(), execution.Request{
		Schema:   s,
		Document: mustParse(t, `query @experimental_disableErrorPropagation { foo }`),
	})
	if got := jsonOf(t, result); got != `{"errors":[{"message":"bar",`+
		`"locations":[{"line":1,"column":47}],"path":["foo"]}],"data":null}` {
		t.Errorf("response = %s, want the error to have propagated", got)
	}
}

// A subscription may ask for it, and then an event whose field fails still
// produces a response with the rest of the data in it.
func TestErrorPropagation_Subscription(t *testing.T) {
	for _, tt := range []struct{ name, query, want string }{
		{
			name:  "asked for",
			query: `subscription @experimental_disableErrorPropagation { events }`,
			want: `{"errors":[{"message":"bar","locations":[{"line":1,"column":54}],` +
				`"path":["events"]}],"data":{"events":null}}`,
		},
		{
			name:  "not asked for",
			query: `subscription { events }`,
			want: `{"errors":[{"message":"bar","locations":[{"line":1,"column":16}],` +
				`"path":["events"]}],"data":null}`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := failingErrorPropagationSchema(t)
			s.SubscriptionType().Field("events").Subscribe = func(
				context.Context, any, schema.Arguments, *schema.ResolveInfo,
			) (any, error) {
				events := make(chan int, 1)
				events <- 1
				close(events)
				return events, nil
			}
			s.SubscriptionType().Field("events").Resolve = func(
				context.Context, any, schema.Arguments, *schema.ResolveInfo,
			) (any, error) {
				return nil, errors.New("bar")
			}

			sub := execution.Subscribe(context.Background(), execution.Request{
				Schema: s, Document: mustParse(t, tt.query),
			})
			if sub.Events == nil {
				t.Fatalf("the subscription did not start: %v", sub.Errors)
			}
			seen := 0
			for result := range sub.Events {
				seen++
				if got := jsonOf(t, result); got != tt.want {
					t.Errorf("response = %s\nwant      %s", got, tt.want)
				}
			}
			if seen != 1 {
				t.Errorf("%d responses, want one", seen)
			}
		})
	}
}

// A deferred payload honours it too: the piece is delivered with a null in it
// rather than being reported as a piece that could not be delivered at all.
func TestErrorPropagation_DeferredPayload(t *testing.T) {
	for _, tt := range []struct{ name, query, want string }{
		{
			name:  "asked for",
			query: `query @experimental_disableErrorPropagation { ok ... @defer { foo } }`,
			want:  `{"hasNext":false,"incremental":[{"id":"0","data":{"foo":null},"errors":[{"message":"bar","locations":[{"line":1,"column":63}],"path":["foo"]}]}],"completed":[{"id":"0"}]}`,
		},
		{
			name:  "not asked for",
			query: `query { ok ... @defer { foo } }`,
			want:  `{"hasNext":false,"completed":[{"id":"0","errors":[{"message":"bar","locations":[{"line":1,"column":25}],"path":["foo"]}]}]}`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := failingErrorPropagationSchema(t)
			extended, err := utilities.ExtendSchemaSource(s,
				`directive @defer(if: Boolean! = true, label: String) on FRAGMENT_SPREAD | INLINE_FRAGMENT`)
			if err != nil {
				t.Fatalf("declaring @defer: %v", err)
			}
			result := execution.ExecuteIncrementally(context.Background(), execution.Request{
				Schema: extended, Document: mustParse(t, tt.query),
			})
			payloads := 0
			for payload := range result.Subsequent {
				payloads++
				if got := string(mustMarshal(t, payload)); got != tt.want {
					t.Errorf("payload = %s\nwant     %s", got, tt.want)
				}
			}
			if payloads != 1 {
				t.Errorf("%d payloads, want one", payloads)
			}
		})
	}
}

// A schema written in Go opts in by listing the directive, the same as it does
// for @defer and @stream.
func TestErrorPropagation_ASchemaWrittenInGo(t *testing.T) {
	s := schema.New(schema.Config{
		Query: schema.NewObject(schema.ObjectConfig{
			Name: "Query",
			Fields: []*schema.Field{
				schema.NewField("foo", schema.FieldConfig{
					Type: schema.NewNonNull(schema.Int),
					Resolve: func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
						return nil, errors.New("bar")
					},
				}),
			},
		}),
		Directives: append(append([]*schema.Directive{}, schema.SpecifiedDirectives...),
			schema.DisableErrorPropagation),
	})
	if errs := schema.ValidateSchema(s); len(errs) != 0 {
		t.Fatalf("the schema is not sound: %v", errs)
	}

	result := execution.Execute(context.Background(), execution.Request{
		Schema:   s,
		Document: mustParse(t, `query @experimental_disableErrorPropagation { foo }`),
	})
	const want = `{"errors":[{"message":"bar","locations":[{"line":1,"column":47}],` +
		`"path":["foo"]}],"data":{"foo":null}}`
	if got := jsonOf(t, result); got != want {
		t.Errorf("response = %s\nwant      %s", got, want)
	}
}
