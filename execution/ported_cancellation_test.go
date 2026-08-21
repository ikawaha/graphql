package execution_test

// Ported from graphql-js src/execution/__tests__/cancellation-test.ts, as far
// as it goes. Most of that file is about a promise that is still pending when
// the abort arrives — a hanging resolver, a hanging list item, a subscription
// resolver that has not returned yet. A Go resolver returns before the next
// one starts, so there is nothing to catch mid-flight; what carries over is
// where the executor looks at the context, and what a request that was given
// up on answers with.

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/schema"
)

// stopped is a resolver that gives up on the request and answers anyway.
func stopped(cancel context.CancelFunc, answer any) schema.FieldResolver {
	return func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		cancel()
		return answer, nil
	}
}

// counting is a resolver that records having been called.
func counting(ran *atomic.Int64, answer any) schema.FieldResolver {
	return func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		ran.Add(1)
		return answer, nil
	}
}

// A resolver is handed the request's context, which is how it learns that the
// caller has gone. graphql-js passes an abort signal for the same purpose.
func TestPortedCancellation_ResolversSeeIt(t *testing.T) {
	s := buildSchema(t, `type Query { a: String b: String }`)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var sawIt bool
	s.QueryType().Field("a").Resolve = stopped(cancel, "A")
	s.QueryType().Field("b").Resolve = func(
		inner context.Context, _ any, _ schema.Arguments, _ *schema.ResolveInfo,
	) (any, error) {
		sawIt = inner.Err() != nil
		return "B", nil
	}

	// The executor stops before the second field, so a resolver only sees a
	// cancelled context when it does the cancelling itself or when it is
	// running alongside one that did.
	execution.Execute(ctx, execution.Request{Schema: s, Document: mustParse(t, `{ a b }`)})
	if sawIt {
		t.Error("the second resolver ran after the request was given up on")
	}

	// Asked on its own, with the context already cancelled before the request
	// begins, a resolver is not called at all — but one that is called can
	// always read the context it was handed.
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	var handed context.Context
	s.QueryType().Field("a").Resolve = func(
		inner context.Context, _ any, _ schema.Arguments, _ *schema.ResolveInfo,
	) (any, error) {
		handed = inner
		return "A", nil
	}
	execution.Execute(ctx2, execution.Request{Schema: s, Document: mustParse(t, `{ a }`)})
	if handed == nil {
		t.Fatal("the resolver was not called")
	}
	if handed.Err() != nil {
		t.Error("the context was already cancelled when the resolver ran")
	}
	cancel2()
	if handed.Err() == nil {
		t.Error("the resolver was handed a context that does not follow the request's")
	}
}

// Giving up part way through an object leaves what was already answered in
// place and reports the rest.
func TestPortedCancellation_DuringNestedCompletion(t *testing.T) {
	s := buildSchema(t, `
		type Query { hero: Hero }
		type Hero { name: String friend: Hero }
	`)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var ran atomic.Int64
	s.Type("Hero").(*schema.ObjectType).Field("name").Resolve = stopped(cancel, "R2-D2")
	s.Type("Hero").(*schema.ObjectType).Field("friend").Resolve = counting(&ran, map[string]any{})

	result := execution.Execute(ctx, execution.Request{
		Schema:    s,
		Document:  mustParse(t, `{ hero { name friend { name } } }`),
		RootValue: map[string]any{"hero": map[string]any{}},
	})

	if got := jsonOf(t, result); !strings.Contains(got, `"name":"R2-D2"`) {
		t.Errorf("response = %s, want the work already done to be kept", got)
	}
	if ran.Load() != 0 {
		t.Error("a nested field was resolved after the request was given up on")
	}
	if len(result.Errors) != 1 || !errors.Is(result.Errors[0], context.Canceled) {
		t.Fatalf("errors = %v, want one saying the request was given up on", result.Errors)
	}
	if path := result.Errors[0].Path; len(path) != 2 || path[0] != "hero" || path[1] != "friend" {
		t.Errorf("path = %v, want [hero friend]", path)
	}
}

// A field that may not be null and was never answered nulls its parent, the
// same as any other failure.
func TestPortedCancellation_NullBubbling(t *testing.T) {
	s := buildSchema(t, `
		type Query { hero: Hero }
		type Hero { name: String alwaysThere: String! }
	`)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Type("Hero").(*schema.ObjectType).Field("name").Resolve = stopped(cancel, "R2-D2")

	result := execution.Execute(ctx, execution.Request{
		Schema:    s,
		Document:  mustParse(t, `{ hero { name alwaysThere } }`),
		RootValue: map[string]any{"hero": map[string]any{}},
	})

	if got := jsonOf(t, result); !strings.Contains(got, `"hero":null`) {
		t.Errorf("response = %s, want the object nulled", got)
	}
	if len(result.Errors) != 1 || !errors.Is(result.Errors[0], context.Canceled) {
		t.Fatalf("errors = %v, want one saying the request was given up on", result.Errors)
	}
}

// Working out which type a value has costs a call, and it is not made once the
// caller has gone.
func TestPortedCancellation_StopsResolvingAbstractTypes(t *testing.T) {
	for _, tt := range []struct{ name, sdl, query string }{
		{
			name: "an interface",
			sdl: `
				type Query { first: String second: Named }
				interface Named { name: String }
				type Person implements Named { name: String }
			`,
			query: `{ first second { name } }`,
		},
		{
			name: "a union",
			sdl: `
				type Query { first: String second: Anything }
				union Anything = Person
				type Person { name: String }
			`,
			query: `{ first second { ... on Person { name } } }`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := buildSchema(t, tt.sdl)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var asked atomic.Int64
			person := s.Type("Person").(*schema.ObjectType)
			person.IsTypeOf = func(context.Context, any, *schema.ResolveInfo) (bool, error) {
				asked.Add(1)
				return true, nil
			}
			s.QueryType().Field("first").Resolve = stopped(cancel, "one")
			s.QueryType().Field("second").Resolve = func(
				context.Context, any, schema.Arguments, *schema.ResolveInfo,
			) (any, error) {
				asked.Add(1)
				return map[string]any{"name": "Ada"}, nil
			}

			result := execution.Execute(ctx, execution.Request{
				Schema: s, Document: mustParse(t, tt.query),
			})
			if asked.Load() != 0 {
				t.Errorf("%d calls were made to work out a type after the request was given up on",
					asked.Load())
			}
			if len(result.Errors) != 1 {
				t.Fatalf("errors = %v, want one", result.Errors)
			}
		})
	}
}

// A mutation's root fields run in order, so giving up stops the ones that have
// not started.
func TestPortedCancellation_MidMutation(t *testing.T) {
	s := buildSchema(t, `
		type Query { a: String }
		type Mutation { first: String second: String }
	`)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var ran atomic.Int64
	s.MutationType().Field("first").Resolve = stopped(cancel, "one")
	s.MutationType().Field("second").Resolve = counting(&ran, "two")

	result := execution.Execute(ctx, execution.Request{
		Schema: s, Document: mustParse(t, `mutation { first second }`),
	})
	if ran.Load() != 0 {
		t.Error("the second root field ran after the request was given up on")
	}
	if got := jsonOf(t, result); !strings.Contains(got, `"first":"one"`) {
		t.Errorf("response = %s, want the first field kept", got)
	}
	if len(result.Errors) != 1 || !errors.Is(result.Errors[0], context.Canceled) {
		t.Fatalf("errors = %v, want one saying the request was given up on", result.Errors)
	}
}

// The entries of a list are answered by one resolver, so a list already
// returned is completed; what stops is the work inside each entry.
func TestPortedCancellation_DuringListCompletion(t *testing.T) {
	s := buildSchema(t, `
		type Query { first: String people: [Person] }
		type Person { name: String }
	`)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var ran atomic.Int64
	s.QueryType().Field("first").Resolve = stopped(cancel, "one")
	s.Type("Person").(*schema.ObjectType).Field("name").Resolve = counting(&ran, "Ada")

	result := execution.Execute(ctx, execution.Request{
		Schema:    s,
		Document:  mustParse(t, `{ first people { name } }`),
		RootValue: map[string]any{"people": []any{map[string]any{}, map[string]any{}}},
	})
	if ran.Load() != 0 {
		t.Error("a list entry was resolved after the request was given up on")
	}
	if len(result.Errors) != 1 || !errors.Is(result.Errors[0], context.Canceled) {
		t.Fatalf("errors = %v, want one saying the request was given up on", result.Errors)
	}
}

// A request given up on before it begins produces nothing at all.
func TestPortedCancellation_BeforeAnythingRuns(t *testing.T) {
	s := buildSchema(t, `type Query { a: String }`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var ran atomic.Int64
	s.QueryType().Field("a").Resolve = counting(&ran, "A")

	result := execution.Execute(ctx, execution.Request{
		Schema: s, Document: mustParse(t, `{ a }`),
	})
	if ran.Load() != 0 {
		t.Error("a resolver ran for a request that had already been given up on")
	}
	if len(result.Errors) != 1 || !errors.Is(result.Errors[0], context.Canceled) {
		t.Fatalf("errors = %v, want one saying the request was given up on", result.Errors)
	}
}

// A subscription whose context is already done never opens a stream.
func TestPortedCancellation_BeforeASubscriptionResolverReturns(t *testing.T) {
	s := buildSchema(t, `
		type Query { a: String }
		type Subscription { events: String }
	`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var ran atomic.Int64
	s.SubscriptionType().Field("events").Subscribe = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		ran.Add(1)
		events := make(chan string, 1)
		events <- "x"
		close(events)
		return events, nil
	}

	sub := execution.Subscribe(ctx, execution.Request{
		Schema: s, Document: mustParse(t, `subscription { events }`),
	})
	if sub.Events != nil {
		for range sub.Events { //nolint:revive // draining, if there is anything
		}
	}
	if ran.Load() != 0 {
		t.Error("the subscription was opened for a request already given up on")
	}
	if len(sub.Errors) != 1 || !errors.Is(sub.Errors[0], context.Canceled) {
		t.Fatalf("errors = %v, want one saying the request was given up on", sub.Errors)
	}
}
