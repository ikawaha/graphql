package execution_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/validation"
)

// message is the sort of Go type a server's events already are: the subscriber
// returns a channel of these, not a channel of any.
type message struct {
	Body   string
	Sender string
}

const subscriptionSDL = `
	type Message { body: String sender: String shout: String }
	type Query { placeholder: String }
	type Subscription {
		messageAdded(inChannel: String): Message
		numbers: Int
	}`

// subscriptionSchema wires the root field to a channel the test controls.
func subscriptionSchema(t *testing.T, subscribe schema.FieldResolver) *schema.Schema {
	t.Helper()
	s := buildSchema(t, subscriptionSDL)
	s.SubscriptionType().Field("messageAdded").Subscribe = subscribe
	return s
}

// startSubscription checks the document and starts the subscription, which is
// the order a server does it in.
func startSubscription(
	ctx context.Context,
	t *testing.T,
	s *schema.Schema,
	query string,
	req execution.Request,
) execution.SubscriptionResult {
	t.Helper()
	doc, err := language.ParseString(query)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	if errs := validation.Validate(s, doc); len(errs) != 0 {
		t.Fatalf("the test document does not validate: %v", errs)
	}
	req.Schema = s
	req.Document = doc
	return execution.Subscribe(ctx, req)
}

// collect reads the responses a subscription produced, as JSON, until the
// stream closes.
func collect(t *testing.T, events <-chan execution.Result) []string {
	t.Helper()
	var out []string
	timeout := time.After(5 * time.Second)
	for {
		select {
		case result, open := <-events:
			if !open {
				return out
			}
			out = append(out, jsonOf(t, result))
		case <-timeout:
			t.Fatal("the subscription did not end within five seconds")
		}
	}
}

// Each event is answered as though the operation were a query against it,
// which is what makes a subscription's selection set mean what it looks like.
func TestSubscribe_EventsBecomeResponses(t *testing.T) {
	source := make(chan message, 3)
	s := subscriptionSchema(t, func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		return source, nil
	})
	// A resolver on the event type runs per event, like any other field.
	s.Type("Message").(*schema.ObjectType).Field("shout").Resolve =
		func(_ context.Context, src any, _ schema.Arguments, _ *schema.ResolveInfo) (any, error) {
			return strings.ToUpper(src.(message).Body), nil
		}

	source <- message{Body: "hello", Sender: "Ada"}
	source <- message{Body: "again", Sender: "Grace"}
	close(source)

	got := startSubscription(context.Background(), t, s,
		`subscription { messageAdded { body sender shout } }`, execution.Request{})
	if got.Events == nil {
		t.Fatalf("the subscription did not start: %v", got.Errors)
	}

	want := []string{
		`{"data":{"messageAdded":{"body":"hello","sender":"Ada","shout":"HELLO"}}}`,
		`{"data":{"messageAdded":{"body":"again","sender":"Grace","shout":"AGAIN"}}}`,
	}
	responses := collect(t, got.Events)
	if len(responses) != len(want) {
		t.Fatalf("%d responses, want %d:\n%s", len(responses), len(want), strings.Join(responses, "\n"))
	}
	for i := range want {
		if responses[i] != want[i] {
			t.Errorf("response %d =\n  %s\nwant\n  %s", i, responses[i], want[i])
		}
	}
}

// The subscriber is called with the arguments the document gave it, which is
// how a server knows which stream to open.
func TestSubscribe_ArgumentsReachTheSubscriber(t *testing.T) {
	var seen string
	source := make(chan message)
	close(source)
	s := subscriptionSchema(t, func(_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo) (any, error) {
		if v, given := args.Get("inChannel"); given {
			seen, _ = v.(string)
		}
		return source, nil
	})

	got := startSubscription(context.Background(), t, s,
		`subscription ($c: String) { messageAdded(inChannel: $c) { body } }`,
		execution.Request{Variables: vars(map[string]any{"c": "general"})})
	if got.Events == nil {
		t.Fatalf("the subscription did not start: %v", got.Errors)
	}
	collect(t, got.Events)

	if seen != "general" {
		t.Errorf("the subscriber was given %q, want %q", seen, "general")
	}
}

// A subscription runs for as long as the caller wants it to, and cancelling
// the context is how the caller says it wants no more.
func TestSubscribe_CancellationEndsIt(t *testing.T) {
	// A source that never ends on its own.
	source := make(chan message)
	go func() {
		for {
			select {
			case source <- message{Body: "tick"}:
			case <-time.After(2 * time.Second):
				return
			}
		}
	}()

	s := subscriptionSchema(t, func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		return source, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	got := startSubscription(ctx, t, s, `subscription { messageAdded { body } }`, execution.Request{})
	if got.Events == nil {
		t.Fatalf("the subscription did not start: %v", got.Errors)
	}

	// Read a couple, then give up.
	for range 2 {
		select {
		case <-got.Events:
		case <-time.After(5 * time.Second):
			t.Fatal("no event arrived")
		}
	}
	cancel()

	// The stream closes rather than leaving the caller waiting.
	timeout := time.After(5 * time.Second)
	for {
		select {
		case _, open := <-got.Events:
			if !open {
				return
			}
		case <-timeout:
			t.Fatal("the stream did not close after the context was cancelled")
		}
	}
}

// One bad event is not a reason to end a stream that may run for hours, so the
// failure is reported in that event's response and the next one still arrives.
func TestSubscribe_AnEventThatFails(t *testing.T) {
	source := make(chan message, 3)
	s := subscriptionSchema(t, func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		return source, nil
	})
	s.Type("Message").(*schema.ObjectType).Field("shout").Resolve =
		func(_ context.Context, src any, _ schema.Arguments, _ *schema.ResolveInfo) (any, error) {
			if src.(message).Body == "bad" {
				return nil, errors.New("cannot shout")
			}
			return strings.ToUpper(src.(message).Body), nil
		}

	source <- message{Body: "one"}
	source <- message{Body: "bad"}
	source <- message{Body: "three"}
	close(source)

	got := startSubscription(context.Background(), t, s,
		`subscription { messageAdded { shout } }`, execution.Request{})
	responses := collect(t, got.Events)

	if len(responses) != 3 {
		t.Fatalf("%d responses, want 3:\n%s", len(responses), strings.Join(responses, "\n"))
	}
	if !strings.Contains(responses[1], "cannot shout") || !strings.Contains(responses[1], `"shout":null`) {
		t.Errorf("the failing event gave %s", responses[1])
	}
	if !strings.Contains(responses[2], `"shout":"THREE"`) {
		t.Errorf("the stream did not carry on after a failing event: %s", responses[2])
	}
}

// A source that cannot go on says so, and the client is told rather than
// having the stream end in silence.
func TestSubscribe_ASourceThatReportsAnError(t *testing.T) {
	source := make(chan any, 2)
	s := subscriptionSchema(t, func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		return source, nil
	})
	source <- message{Body: "one"}
	source <- errors.New("the connection dropped")
	close(source)

	got := startSubscription(context.Background(), t, s,
		`subscription { messageAdded { body } }`, execution.Request{})
	responses := collect(t, got.Events)

	if len(responses) != 2 {
		t.Fatalf("%d responses, want 2:\n%s", len(responses), strings.Join(responses, "\n"))
	}
	if !strings.Contains(responses[1], "the connection dropped") {
		t.Errorf("the reason did not reach the client: %s", responses[1])
	}
	// It is an error with no data, since no event was answered.
	if strings.Contains(responses[1], `"data"`) {
		t.Errorf("a source error came with data: %s", responses[1])
	}
}

// Failing to start is a different thing from an event failing: there is no
// stream at all, and the reason comes back directly.
func TestSubscribe_FailingToStart(t *testing.T) {
	tests := []struct {
		name      string
		subscribe schema.FieldResolver
		want      string
	}{
		{
			name: "the subscriber returns an error",
			subscribe: func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
				return nil, errors.New("no such channel")
			},
			want: "no such channel",
		},
		{
			name: "the subscriber returns something that is not a channel",
			subscribe: func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
				return "not a channel", nil
			},
			want: "must return a channel of events",
		},
		{
			name: "the subscriber returns a send-only channel",
			subscribe: func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
				return make(chan<- message), nil
			},
			want: "send-only",
		},
		{
			name: "the subscriber panics",
			subscribe: func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
				panic("something went badly wrong")
			},
			want: "something went badly wrong",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := subscriptionSchema(t, tt.subscribe)
			got := startSubscription(context.Background(), t, s,
				`subscription { messageAdded { body } }`, execution.Request{})

			if got.Events != nil {
				t.Fatal("a stream was returned for a subscription that could not start")
			}
			if len(got.Errors) != 1 {
				t.Fatalf("%d errors, want 1: %v", len(got.Errors), got.Errors)
			}
			if !strings.Contains(got.Errors[0].Message, tt.want) {
				t.Errorf("error = %q, want it to mention %q", got.Errors[0].Message, tt.want)
			}
			// It says which field could not be subscribed to.
			if len(got.Errors[0].Path) == 0 {
				t.Error("the error does not say which field it is about")
			}
		})
	}
}

// A request that is not a subscription, or that the schema cannot answer, is
// refused before anything is opened.
func TestSubscribe_WrongSortOfRequest(t *testing.T) {
	source := make(chan message)
	close(source)
	subscriber := func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		return source, nil
	}

	t.Run("a query rather than a subscription", func(t *testing.T) {
		s := subscriptionSchema(t, subscriber)
		got := startSubscription(context.Background(), t, s, `{ placeholder }`, execution.Request{})
		if got.Events != nil {
			t.Fatal("a query was subscribed to")
		}
		if len(got.Errors) != 1 || !strings.Contains(got.Errors[0].Message, "subscription operation") {
			t.Errorf("errors = %v", got.Errors)
		}
	})

	t.Run("a schema with no subscription root", func(t *testing.T) {
		s := buildSchema(t, `type Query { a: String }`)
		doc, err := language.ParseString(`subscription { a }`)
		if err != nil {
			t.Fatal(err)
		}
		got := execution.Subscribe(context.Background(), execution.Request{Schema: s, Document: doc})
		if got.Events != nil {
			t.Fatal("a schema with no subscription root produced a stream")
		}
		if len(got.Errors) != 1 {
			t.Fatalf("%d errors, want 1", len(got.Errors))
		}
	})

	// Validation reports this, but a document that skipped it must not be
	// answered with an arbitrary one of its fields.
	t.Run("more than one root field", func(t *testing.T) {
		s := subscriptionSchema(t, subscriber)
		doc, err := language.ParseString(`subscription { messageAdded { body } numbers }`)
		if err != nil {
			t.Fatal(err)
		}
		got := execution.Subscribe(context.Background(), execution.Request{Schema: s, Document: doc})
		if got.Events != nil {
			t.Fatal("a subscription selecting two fields produced a stream")
		}
		if len(got.Errors) != 1 || !strings.Contains(got.Errors[0].Message, "exactly one field") {
			t.Errorf("errors = %v", got.Errors)
		}
	})

	// With no subscriber of its own the field falls through to the ordinary
	// resolver and then to the default one, which reads the root value — so
	// what is wrong is not that nobody answered but that the answer was not a
	// channel. graphql-js says the same of an answer that is not an async
	// iterable.
	t.Run("a field with nothing to subscribe to", func(t *testing.T) {
		s := buildSchema(t, subscriptionSDL)
		got := startSubscription(context.Background(), t, s,
			`subscription { messageAdded { body } }`, execution.Request{})
		if got.Events != nil {
			t.Fatal("a field with nothing to subscribe to produced a stream")
		}
		if len(got.Errors) != 1 || !strings.Contains(got.Errors[0].Message, "must return a channel") {
			t.Errorf("errors = %v", got.Errors)
		}
	})

	// The channel may come from the root value, with no resolver written at
	// all, which is how graphql-js takes one too.
	t.Run("a channel in the root value", func(t *testing.T) {
		s := buildSchema(t, subscriptionSDL)
		events := make(chan any, 1)
		events <- map[string]any{"body": "from the root value"}
		close(events)
		got := startSubscription(context.Background(), t, s,
			`subscription { messageAdded { body } }`, execution.Request{
				RootValue: map[string]any{"messageAdded": events},
			})
		if len(got.Errors) > 0 {
			t.Fatalf("errors = %v", got.Errors)
		}
		seen := 0
		for range got.Events {
			seen++
		}
		if seen != 1 {
			t.Errorf("%d events, want 1", seen)
		}
	})

	t.Run("variables that will not coerce", func(t *testing.T) {
		s := subscriptionSchema(t, subscriber)
		doc, err := language.ParseString(
			`subscription ($c: String!) { messageAdded(inChannel: $c) { body } }`)
		if err != nil {
			t.Fatal(err)
		}
		got := execution.Subscribe(context.Background(), execution.Request{Schema: s, Document: doc})
		if got.Events != nil {
			t.Fatal("a subscription with missing variables produced a stream")
		}
		if len(got.Errors) != 1 {
			t.Fatalf("%d errors, want 1: %v", len(got.Errors), got.Errors)
		}
	})

	t.Run("nothing to subscribe to", func(t *testing.T) {
		if got := execution.Subscribe(context.Background(), execution.Request{}); len(got.Errors) != 1 {
			t.Errorf("subscribing with no schema gave %d errors", len(got.Errors))
		}
		s := buildSchema(t, subscriptionSDL)
		if got := execution.Subscribe(context.Background(),
			execution.Request{Schema: s}); len(got.Errors) != 1 {
			t.Errorf("subscribing with no document gave %d errors", len(got.Errors))
		}
	})
}

// A subscription root field's ordinary resolver is not asked for the stream.
//
// The specification gives such a field two internal functions —
// ResolveFieldEventStream for the stream and ResolveFieldValue for each
// event's value — and both are used, at different moments. A field with only
// the second must not have it called for the first, or it would be asked for a
// channel and answer with a value. graphql-js refuses the same case with
// "Subscription field must return Async Iterable. Received: undefined."
func TestSubscribe_TheOrdinaryResolverIsNotTheSubscriber(t *testing.T) {
	source := make(chan message, 1)
	source <- message{Body: "hello"}
	close(source)

	s := buildSchema(t, subscriptionSDL)
	s.SubscriptionType().Field("messageAdded").Resolve =
		func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
			return source, nil
		}

	got := startSubscription(context.Background(), t, s,
		`subscription { messageAdded { body } }`, execution.Request{})
	if got.Events != nil {
		t.Fatal("the ordinary resolver was asked for the stream")
	}
	if len(got.Errors) != 1 ||
		!strings.Contains(got.Errors[0].Message, "must return a channel of events") {
		t.Errorf("errors were %v", got.Errors)
	}
}

// The two roles together: the channel comes from the root value and the
// ordinary resolver shapes each event, which is what a server publishing
// envelopes writes.
func TestSubscribe_TheOrdinaryResolverShapesEachEvent(t *testing.T) {
	source := make(chan any, 2)
	source <- map[string]any{"messageAdded": message{Body: "one"}}
	source <- map[string]any{"messageAdded": message{Body: "two"}}
	close(source)

	s := buildSchema(t, subscriptionSDL)
	s.SubscriptionType().Field("messageAdded").Resolve = func(
		_ context.Context, event any, _ schema.Arguments, _ *schema.ResolveInfo,
	) (any, error) {
		return event.(map[string]any)["messageAdded"], nil
	}

	got := startSubscription(context.Background(), t, s,
		`subscription { messageAdded { body } }`,
		execution.Request{RootValue: map[string]any{"messageAdded": source}})
	if got.Events == nil {
		t.Fatalf("the subscription did not start: %v", got.Errors)
	}
	want := []string{
		`{"data":{"messageAdded":{"body":"one"}}}`,
		`{"data":{"messageAdded":{"body":"two"}}}`,
	}
	if responses := collect(t, got.Events); strings.Join(responses, "\n") != strings.Join(want, "\n") {
		t.Errorf("responses =\n  %s\nwant\n  %s",
			strings.Join(responses, "\n  "), strings.Join(want, "\n  "))
	}
}

// The last place looked is the default resolver, so a channel put in the root
// value is a whole subscription with no resolver written at all.
func TestSubscribe_AChannelInTheRootValue(t *testing.T) {
	source := make(chan message, 2)
	source <- message{Body: "one"}
	source <- message{Body: "two"}
	close(source)

	s := buildSchema(t, subscriptionSDL)
	got := startSubscription(context.Background(), t, s,
		`subscription { messageAdded { body } }`,
		execution.Request{RootValue: map[string]any{"messageAdded": source}})
	if got.Events == nil {
		t.Fatalf("the subscription did not start: %v", got.Errors)
	}
	responses := collect(t, got.Events)

	want := []string{
		`{"data":{"messageAdded":{"body":"one"}}}`,
		`{"data":{"messageAdded":{"body":"two"}}}`,
	}
	if strings.Join(responses, "\n") != strings.Join(want, "\n") {
		t.Errorf("responses =\n  %s\nwant\n  %s",
			strings.Join(responses, "\n  "), strings.Join(want, "\n  "))
	}
}

// A server may want the events themselves rather than the responses.
func TestCreateSourceEventStream(t *testing.T) {
	source := make(chan message, 2)
	source <- message{Body: "one"}
	source <- message{Body: "two"}
	close(source)

	s := subscriptionSchema(t, func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		return source, nil
	})
	doc, err := language.ParseString(`subscription { messageAdded { body } }`)
	if err != nil {
		t.Fatal(err)
	}

	events, errs := execution.CreateSourceEventStream(
		context.Background(), execution.Request{Schema: s, Document: doc})
	if len(errs) != 0 {
		t.Fatalf("starting: %v", errs)
	}

	var got []string
	for event := range events {
		got = append(got, event.(message).Body)
	}
	if strings.Join(got, ",") != "one,two" {
		t.Errorf("events = %v, want [one two]", got)
	}

	t.Run("it reports a failure to start", func(t *testing.T) {
		s := buildSchema(t, subscriptionSDL)
		events, errs := execution.CreateSourceEventStream(
			context.Background(), execution.Request{Schema: s, Document: doc})
		if events != nil {
			t.Error("a stream was returned for a subscription that could not start")
		}
		if len(errs) != 1 {
			t.Errorf("%d errors, want 1", len(errs))
		}
	})
}

// A root field with a resolver of its own is given the event and says what the
// field's value should be, which is how a server whose events are envelopes
// unwraps them — and how a subscription written for graphql-js keeps working.
func TestSubscribe_ARootFieldWithItsOwnResolver(t *testing.T) {
	// The envelope idiom: the event carries the value under the field's name,
	// which is what graphql-js servers publish.
	source := make(chan map[string]any, 1)
	source <- map[string]any{"messageAdded": message{Body: "hello", Sender: "Ada"}}
	close(source)

	s := subscriptionSchema(t, func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		return source, nil
	})
	s.SubscriptionType().Field("messageAdded").Resolve =
		func(_ context.Context, event any, _ schema.Arguments, _ *schema.ResolveInfo) (any, error) {
			return event.(map[string]any)["messageAdded"], nil
		}

	got := startSubscription(context.Background(), t, s,
		`subscription { messageAdded { body sender } }`, execution.Request{})
	responses := collect(t, got.Events)

	if len(responses) != 1 {
		t.Fatalf("%d responses, want 1", len(responses))
	}
	want := `{"data":{"messageAdded":{"body":"hello","sender":"Ada"}}}`
	if responses[0] != want {
		t.Errorf("response =\n  %s\nwant\n  %s", responses[0], want)
	}

	// And a resolver that fails does so per event, with the path to the field.
	t.Run("a root resolver that fails", func(t *testing.T) {
		source := make(chan message, 1)
		source <- message{Body: "x"}
		close(source)
		s := subscriptionSchema(t, func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
			return source, nil
		})
		s.SubscriptionType().Field("messageAdded").Resolve =
			func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
				return nil, errors.New("cannot map the event")
			}
		got := startSubscription(context.Background(), t, s,
			`subscription { messageAdded { body } }`, execution.Request{})
		responses := collect(t, got.Events)
		if len(responses) != 1 {
			t.Fatalf("%d responses, want 1", len(responses))
		}
		if !strings.Contains(responses[0], "cannot map the event") ||
			!strings.Contains(responses[0], `"path":["messageAdded"]`) {
			t.Errorf("response = %s", responses[0])
		}
	})
}

// An alias names the key the response uses, for a subscription as for a query.
func TestSubscribe_Alias(t *testing.T) {
	source := make(chan message, 1)
	source <- message{Body: "hello"}
	close(source)
	s := subscriptionSchema(t, func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		return source, nil
	})

	got := startSubscription(context.Background(), t, s,
		`subscription { latest: messageAdded { body } }`, execution.Request{})
	responses := collect(t, got.Events)

	if len(responses) != 1 {
		t.Fatalf("%d responses, want 1", len(responses))
	}
	if responses[0] != `{"data":{"latest":{"body":"hello"}}}` {
		t.Errorf("response = %s", responses[0])
	}
}

// A field that may not be null has nowhere to put one, so an event that fails
// gives a response with no data rather than a null field.
func TestSubscribe_ANonNullRootField(t *testing.T) {
	s := buildSchema(t, `
		type Message { body: String }
		type Query { placeholder: String }
		type Subscription { messageAdded: Message! }
	`)
	source := make(chan message, 1)
	source <- message{Body: "x"}
	close(source)
	s.SubscriptionType().Field("messageAdded").Subscribe =
		func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
			return source, nil
		}
	s.SubscriptionType().Field("messageAdded").Resolve =
		func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
			return nil, errors.New("no message after all")
		}

	got := startSubscription(context.Background(), t, s,
		`subscription { messageAdded { body } }`, execution.Request{})
	responses := collect(t, got.Events)

	if len(responses) != 1 {
		t.Fatalf("%d responses, want 1", len(responses))
	}
	if !strings.Contains(responses[0], `"data":null`) {
		t.Errorf("response = %s, want data to be null", responses[0])
	}
}
