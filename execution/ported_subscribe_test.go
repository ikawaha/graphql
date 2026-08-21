package execution_test

// Ported from graphql-js src/execution/__tests__/subscribe-test.ts.
//
// Most of that file is about JavaScript's async iterators: how a generator
// settles, when `return` is called on it, what a rejected promise does to the
// stream. None of those questions arise here, where a subscription is a Go
// channel. What is ported is what a subscription does when something is wrong,
// which is a question about GraphQL rather than about the language.
//
// One difference runs through all of it: graphql-js makes the event the root
// value and resolves the root field from it, so an event is written as an
// envelope, `{ foo: 'FooValue' }`. Here the event is the root field's value,
// so the same event is written `"FooValue"`. See COMPATIBILITY.md.

import (
	"context"
	"errors"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// TestPortedSubscribe_FailsToStart covers the cases where a subscription never
// produces a stream at all, and the response says why.
func TestPortedSubscribe_FailsToStart(t *testing.T) {
	tests := []struct {
		name string
		// build makes the schema the case runs against.
		build func(*testing.T) *schema.Schema
		query string
		// variables are the request's, where a case supplies any.
		variables map[string]value.Maybe[any]
		want      string
	}{
		{
			name:  `throws when subscribe is called with a non-subscription operation`,
			build: func(t *testing.T) *schema.Schema { return buildPorted(t, dummyQuerySDL) },
			query: `{ dummy }`,
			want: `{"errors": [{"message": "Expected subscription operation.",
				"locations": [{"line": 1, "column": 1}]}]}`,
		},
		{
			name:  `resolves to an error if schema does not support subscriptions`,
			build: func(t *testing.T) *schema.Schema { return buildPorted(t, dummyQuerySDL) },
			query: `subscription { unknownField }`,
			want: `{"errors": [{"message": "Schema is not configured to execute subscription operation.",
				"locations": [{"line": 1, "column": 1}]}]}`,
		},
		{
			name:  `resolves to an error for unknown subscription field`,
			build: func(t *testing.T) *schema.Schema { return buildPorted(t, fooSubscriptionSDL) },
			query: `subscription { unknownField }`,
			want: `{"errors": [{"message": "The subscription field \"unknownField\" is not defined.",
				"locations": [{"line": 1, "column": 16}]}]}`,
		},
		{
			name: `passes through unexpected errors thrown in subscribe`,
			build: func(t *testing.T) *schema.Schema {
				return subscriberReturning(t, func() (any, error) { return nil, errors.New("test error") })
			},
			query: `subscription { foo }`,
			want: `{"errors": [{"message": "test error",
				"locations": [{"line": 1, "column": 16}], "path": ["foo"]}]}`,
		},
		{
			// graphql-js asks for an async iterable; the Go counterpart is a
			// channel, so the message says so.
			name: `errors if subscribe does not return a stream of events`,
			build: func(t *testing.T) *schema.Schema {
				return subscriberReturning(t, func() (any, error) { return "test", nil })
			},
			query: `subscription { foo }`,
			want: `{"errors": [{"message": "Subscription field must return a channel of events. Received: \"test\".",
				"locations": [{"line": 1, "column": 16}], "path": ["foo"]}]}`,
		},
		{
			name: `resolves to an error for subscription resolver errors`,
			build: func(t *testing.T) *schema.Schema {
				return subscriberReturning(t, func() (any, error) { return nil, errors.New("test error") })
			},
			query: `subscription { foo }`,
			want: `{"errors": [{"message": "test error",
				"locations": [{"line": 1, "column": 16}], "path": ["foo"]}]}`,
		},
		{
			name: `resolves to an error if variables were wrong type`,
			build: func(t *testing.T) *schema.Schema {
				return buildPorted(t, `
					type Query { dummy: String }
					type Subscription { foo(arg: Int): String }
				`)
			},
			query: `
      subscription ($arg: Int) {
        foo(arg: $arg)
      }
    `,
			variables: map[string]value.Maybe[any]{"arg": value.Just[any]("meow")},
			want: `{"errors": [{"message": "Variable \"$arg\" has invalid value: Int cannot represent non-integer value: \"meow\"",
				"locations": [{"line": 2, "column": 21}]}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := execution.Subscribe(context.Background(), execution.Request{
				Schema:    tt.build(t),
				Document:  mustParse(t, tt.query),
				Variables: tt.variables,
			})
			if sub.Events != nil {
				t.Fatal("a stream was started, want none")
			}
			got := decodeJSON(t, mustMarshal(t, execution.Result{Errors: sub.Errors}))
			if want := decodeJSON(t, tt.want); !equalJSON(got, want) {
				t.Errorf("errors =\n%s\nwant\n%s",
					mustMarshal(t, execution.Result{Errors: sub.Errors}), tt.want)
			}
		})
	}
}

// TestPortedSubscribe_HandlesErrorDuringExecution is graphql-js's `should
// handle error during execution of source event`: one event fails to resolve
// and the stream carries on.
func TestPortedSubscribe_HandlesErrorDuringExecution(t *testing.T) {
	s := buildPorted(t, `
		type Query { dummy: String }
		type Subscription { newMessage: String }
	`)
	messages := make(chan string, 3)
	for _, message := range []string{"Hello", "Goodbye", "Bonjour"} {
		messages <- message
	}
	close(messages)

	field := s.SubscriptionType().Field("newMessage")
	field.Subscribe = func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		return messages, nil
	}
	field.Resolve = func(
		_ context.Context, source any, _ schema.Arguments, _ *schema.ResolveInfo,
	) (any, error) {
		if source == "Goodbye" {
			return nil, errors.New("Never leave.")
		}
		return source, nil
	}

	want := []string{
		`{"data": {"newMessage": "Hello"}}`,
		`{"data": {"newMessage": null}, "errors": [{"message": "Never leave.",
			"locations": [{"line": 1, "column": 16}], "path": ["newMessage"]}]}`,
		`{"data": {"newMessage": "Bonjour"}}`,
	}
	expectSubscriptionEvents(t, s, `subscription { newMessage }`, want)
}

// TestPortedSubscribe_PassesThroughSourceError is graphql-js's `should pass
// through error thrown in source event stream`. A Go channel cannot fail, so
// the way a source says something went wrong is to send the error.
func TestPortedSubscribe_PassesThroughSourceError(t *testing.T) {
	s := buildPorted(t, `
		type Query { dummy: String }
		type Subscription { newMessage: String }
	`)
	messages := make(chan any, 2)
	messages <- "Hello"
	messages <- errors.New("test error")
	close(messages)

	s.SubscriptionType().Field("newMessage").Subscribe = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		return messages, nil
	}

	want := []string{
		`{"data": {"newMessage": "Hello"}}`,
		`{"errors": [{"message": "test error"}]}`,
	}
	expectSubscriptionEvents(t, s, `subscription { newMessage }`, want)
}

// TestPortedSubscribe_OnlyTheFirstField is graphql-js's `should only resolve
// the first field of invalid multi-field`, which here is refused outright: a
// subscription selecting two fields is not answerable, and answering an
// arbitrary one of them would be worse than saying so.
func TestPortedSubscribe_OnlyTheFirstField(t *testing.T) {
	s := buildPorted(t, `
		type Query { dummy: String }
		type Subscription { foo: String bar: String }
	`)
	started := 0
	for _, name := range []string{"foo", "bar"} {
		s.SubscriptionType().Field(name).Subscribe = func(
			context.Context, any, schema.Arguments, *schema.ResolveInfo,
		) (any, error) {
			started++
			events := make(chan string, 1)
			events <- "FooValue"
			close(events)
			return events, nil
		}
	}

	sub := execution.Subscribe(context.Background(), execution.Request{
		Schema:   s,
		Document: mustParse(t, `subscription { foo bar }`),
	})
	if sub.Events != nil {
		t.Fatal("a stream was started, want none")
	}
	if started != 0 {
		t.Errorf("%d subscribers were called, want none", started)
	}
	if len(sub.Errors) != 1 {
		t.Fatalf("%d errors, want 1", len(sub.Errors))
	}
}

const (
	dummyQuerySDL      = `type Query { dummy: String }`
	fooSubscriptionSDL = `type Query { dummy: String } type Subscription { foo: String }`
)

// subscriberReturning builds the one-field subscription schema with a
// subscriber that answers however the case says.
func subscriberReturning(t *testing.T, answer func() (any, error)) *schema.Schema {
	t.Helper()
	s := buildPorted(t, fooSubscriptionSDL)
	s.SubscriptionType().Field("foo").Subscribe = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		return answer()
	}
	return s
}

// expectSubscriptionEvents runs a subscription to its end and compares each
// response to what was expected.
func expectSubscriptionEvents(t *testing.T, s *schema.Schema, query string, want []string) {
	t.Helper()
	sub := execution.Subscribe(context.Background(), execution.Request{
		Schema:   s,
		Document: mustParse(t, query),
	})
	if sub.Events == nil {
		t.Fatalf("the subscription did not start: %v", sub.Errors)
	}
	var got []execution.Result
	for result := range sub.Events {
		got = append(got, result)
	}
	if len(got) != len(want) {
		t.Fatalf("%d events, want %d", len(got), len(want))
	}
	for i := range want {
		if !equalJSON(decodeJSON(t, mustMarshal(t, got[i])), decodeJSON(t, want[i])) {
			t.Errorf("event %d =\n%s\nwant\n%s", i, mustMarshal(t, got[i]), want[i])
		}
	}
}

// Not ported, because each of these is written in a way this could not follow:
//   - throws for legacy ExecutionArgs passed to createSourceEventStream: there
//     is no legacy shape here
//   - throws when validateSubscriptionArgs is called with a non-subscription
//     operation: that check is inside Subscribe rather than a step of its own
//   - accepts multiple subscription fields defined in schema, accepts type
//     definition with sync/async subscribe function: what they check is that a
//     generator is accepted; the counterpart is Subscribe's own tests
//   - uses a custom default subscribeFieldResolver, maps a source stream to
//     response events with a custom rootSelectionSetExecutor: neither hook
//     exists here
//   - produces a payload for multiple subscribe in same subscription, produces
//     a payload when queried fields are async, produces a payload per
//     subscription event, produces a payload when there are multiple events,
//     event order is correct for multiple publishes: all about when a promise
//     settles; the Go tests in subscribe_test.go cover what remains
//   - subscribe function returns errors with @defer / @stream: incremental
//     delivery within a subscription is not answered by Subscribe
//   - should not trigger when subscription is already done / is thrown: a Go
//     channel is closed rather than returned from
