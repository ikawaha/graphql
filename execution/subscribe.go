package execution

import (
	"context"
	"fmt"
	"reflect"

	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// SubscriptionResult is what starting a subscription gives.
//
// Either the subscription started, and Events carries a response per event, or
// it did not, and Errors says why. A subscription that starts and then fails
// on a particular event reports that in the response for the event, the same
// way a query reports a field that failed.
type SubscriptionResult struct {
	// Events carries one response per event, in the order the source produced
	// them. It is closed when the source ends, when the context is cancelled,
	// or when the source reports an error; it is nil when the subscription
	// could not be started at all.
	//
	// A caller must read from it until it closes, or cancel the context.
	// Abandoning it without either would leave the goroutine feeding it
	// waiting for a reader that never comes.
	Events <-chan Result
	// Errors is why the subscription could not be started. It is empty when
	// Events is not nil.
	Errors []*gqlerror.Error
}

// Subscribe starts a subscription and returns the responses it produces.
//
// A subscription has two halves. The first runs once: the root field's
// Subscribe resolver is called and returns a channel of events, which is
// whatever the server's own machinery produces — a message from a queue, a row
// from a database, anything. Failing here means the subscription never
// started, and the reason comes back in [SubscriptionResult.Errors].
//
// The second half runs once per event: the operation's selection set is
// executed against the event as though it were a query, and the response is
// sent on [SubscriptionResult.Events]. An event whose fields fail produces a
// response with errors in it, and the subscription carries on: one bad event
// is not a reason to end a stream that may run for hours.
//
// The subscription ends when the source channel closes, when ctx is cancelled,
// or when the source produces an error. Cancelling ctx is how a caller ends
// one; the events channel is closed once the caller has been told.
//
// As with [Execute], the document is expected to have passed validation, which
// is what settles that the operation is a subscription selecting exactly one
// field.
func Subscribe(ctx context.Context, req Request) SubscriptionResult {
	started, errs := createSourceEventStream(ctx, req)
	if len(errs) > 0 {
		return SubscriptionResult{Errors: errs}
	}

	events := make(chan Result)
	go func() {
		defer close(events)
		for event := range receiveFrom(ctx, started.source) {
			select {
			case events <- started.answer(ctx, event):
			case <-ctx.Done():
				return
			}
		}
	}()

	return SubscriptionResult{Events: events}
}

// CreateSourceEventStream runs the first half of a subscription and returns
// the events themselves, before any of them is turned into a response.
//
// This is for a server that wants the events for something other than
// answering the document that asked for them — recording them, fanning them
// out, deciding whether to answer at all. Most callers want [Subscribe].
func CreateSourceEventStream(ctx context.Context, req Request) (<-chan any, []*gqlerror.Error) {
	started, errs := createSourceEventStream(ctx, req)
	if len(errs) > 0 {
		return nil, errs
	}
	out := make(chan any)
	go func() {
		defer close(out)
		for event := range receiveFrom(ctx, started.source) {
			select {
			case out <- event:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// startedSubscription is a subscription that has opened its source and knows
// how to answer for each event it produces.
type startedSubscription struct {
	executor   *executor
	source     reflect.Value
	rootType   *schema.ObjectType
	key        string
	selections []FieldSelection
	def        *schema.Field
	path       *value.Path
}

// createSourceEventStream calls the root field's Subscribe resolver and
// returns whatever channel it produced, along with what is needed to answer
// for each event.
func createSourceEventStream(ctx context.Context, req Request) (*startedSubscription, []*gqlerror.Error) {
	var none *startedSubscription

	if req.Schema == nil {
		return none, []*gqlerror.Error{gqlerror.New("Must provide a schema.")}
	}
	if req.Document == nil {
		return none, []*gqlerror.Error{gqlerror.New("Must provide a document.")}
	}
	operation, opErr := operationToRun(req.Document, req.OperationName)
	if opErr != nil {
		return none, []*gqlerror.Error{opErr}
	}
	if operation.Operation != language.OperationSubscription {
		return none, []*gqlerror.Error{gqlerror.New(
			"Expected subscription operation.", gqlerror.WithNodes(operation))}
	}
	rootType := req.Schema.SubscriptionType()
	if rootType == nil {
		return none, []*gqlerror.Error{gqlerror.New(
			"Schema is not configured to execute subscription operation.",
			gqlerror.WithNodes(operation))}
	}

	variables, varErrs := coerceVariableValuesUpTo(req.maxCoercionErrors(),
		req.Schema, operation.VariableDefinitions, req.Variables,
		req.checkOptions()...)
	if len(varErrs) > 0 {
		return none, varErrs
	}

	fragments := fragmentsOf(req.Document)
	fields := CollectFields(req.Schema, fragments, variables, rootType, operation.SelectionSet)
	// A fragment spread that got its arguments wrong means the subscription
	// never starts, the same as any other fault in the request.
	if errs := fields.Errors(); len(errs) > 0 {
		return none, errs
	}
	if fields.Len() != 1 {
		// Validation reports this, but a document that skipped it must not be
		// answered with an arbitrary one of the fields.
		return none, []*gqlerror.Error{gqlerror.New(
			fmt.Sprintf("A subscription must select exactly one field, this one selects %d.", fields.Len()),
			gqlerror.WithNodes(operation))}
	}

	key := fields.Keys()[0]
	selections := fields.Fields(key)
	nodes := fields.Nodes(key)
	field := nodes[0]
	name := nameOf(field.Name)
	def := req.Schema.Field(rootType, name)
	if def == nil {
		return none, []*gqlerror.Error{gqlerror.New(
			fmt.Sprintf("The subscription field %q is not defined.", name),
			gqlerror.WithNodes(field))}
	}

	path := (*value.Path)(nil).WithField(key, rootType.Name())
	info := &schema.ResolveInfo{
		Schema:         req.Schema,
		FieldName:      name,
		FieldNodes:     nodes,
		ReturnType:     def.Type,
		ParentType:     rootType,
		Path:           path,
		RootValue:      req.RootValue,
		Operation:      operation,
		Fragments:      fragments,
		VariableValues: variables.Values(),
	}

	// Where the source comes from: the field's own subscriber, then the one
	// the request supplied, then the default resolver — which reads the root
	// value, so a server can put the channel there and write no subscriber at
	// all.
	//
	// The field's ordinary resolver is not in this list, and neither is
	// [Request.FieldResolver]. The specification gives a subscription root
	// field two internal functions, ResolveFieldEventStream for the stream and
	// ResolveFieldValue for each event's value, and says the first is
	// "intentionally similar" to the second rather than the same. Both are
	// used, at different moments, so a field that has only a value resolver
	// must not have it called here: it would be asked for a channel and answer
	// with a value. graphql-js looks in the same three places.
	resolver := def.Subscribe
	if resolver == nil {
		resolver = req.SubscribeResolver
	}
	if resolver == nil {
		resolver = DefaultResolver
	}

	// A caller who has already given up must not have a stream opened for
	// them: a subscriber usually takes something out — a queue, a connection —
	// that would then have nobody to close it.
	if err := ctx.Err(); err != nil {
		return none, []*gqlerror.Error{locate(err, nodes, path)}
	}

	produced, err := callSubscriber(ctx, resolver, def, field, req.RootValue, variables, info, req.checkOptions())
	if err != nil {
		return none, []*gqlerror.Error{locate(err, nodes, path)}
	}

	source := reflect.ValueOf(produced)
	if !source.IsValid() || source.Kind() != reflect.Chan {
		// graphql-js asks for an async iterable; the Go counterpart is a
		// channel, and saying so is more use than repeating a name for
		// something the language does not have.
		return none, []*gqlerror.Error{locate(gqlerror.Newf(
			"Subscription field must return a channel of events. Received: %s.",
			value.Describe(produced)), nodes, path)}
	}
	if source.Type().ChanDir() == reflect.SendDir {
		return none, []*gqlerror.Error{locate(gqlerror.New(
			"Subscription field must return a channel it can receive from. "+
				"Received: a send-only channel."), nodes, path)}
	}
	// One executor answers every event, so what it remembers about the
	// document — which fields an object type asks for — is worked out once for
	// the subscription rather than once per event.
	return &startedSubscription{
		executor: &executor{
			schema:        req.Schema,
			fragments:     fragments,
			variables:     variables,
			rootValue:     req.RootValue,
			operation:     operation,
			concurrency:   req.Concurrency,
			fieldResolver: req.FieldResolver,
			typeResolver:  req.TypeResolver,
			propagating:   !errorPropagationDisabled(req.Schema, operation),
			checks:        req.checkOptions(),
			shared:        &runState{},
		},
		source:     source,
		rootType:   rootType,
		key:        key,
		selections: selections,
		def:        def,
		path:       path,
	}, nil
}

// callSubscriber calls a subscribe resolver, coercing its arguments first and
// catching a panic the way an ordinary field's resolver is caught.
func callSubscriber(
	ctx context.Context,
	resolver schema.FieldResolver,
	def *schema.Field,
	field *language.Field,
	source any,
	variables schema.VariableValues,
	info *schema.ResolveInfo,
	checks []schema.CheckOption,
) (result any, err error) {
	args, argErr := coerceArgumentValues(argumentOwner{field: def}, def.Args, field.Arguments, variables, field,
		checks...)
	if argErr != nil {
		return nil, argErr
	}
	defer func() {
		if r := recover(); r != nil {
			result = nil
			err = &PanicError{Value: r, Stack: captureStack()}
		}
	}()
	return resolver(ctx, source, args, info)
}

// receiveFrom turns a channel of unknown element type into one of any,
// stopping when the context is cancelled.
//
// A server's events are its own types, so the channel a subscriber returns is
// whatever it already had — `<-chan Message` rather than `<-chan any`. Reading
// it means reflection, and reflecting means the cancellation has to be part of
// the same select rather than a separate one.
func receiveFrom(ctx context.Context, source reflect.Value) <-chan any {
	out := make(chan any)
	go func() {
		defer close(out)
		cases := []reflect.SelectCase{
			{Dir: reflect.SelectRecv, Chan: source},
			{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())},
		}
		for {
			chosen, received, ok := reflect.Select(cases)
			if chosen == 1 || !ok {
				// The context was cancelled, or the source has ended.
				return
			}
			select {
			case out <- received.Interface():
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// answer turns one event into a response.
//
// The event is the value of the root field, and the selection set beneath it
// is resolved against that value the way any other object's is. A root field
// that has a resolver of its own is given the event and says what the value
// should be instead, which is how a server whose events are envelopes unwraps
// them.
//
// graphql-js works the other way round: the event becomes the root value and
// the root field is resolved from it, so a server there must write a resolver
// even when the event is already the value. That reads as a mistake in Go,
// where a subscriber naturally returns a channel of the type the field
// returns.
func (s *startedSubscription) answer(ctx context.Context, event any) Result {
	// A source that produces an error is telling the subscription it cannot go
	// on — a connection dropped, a queue closed — which is worth passing to
	// the client rather than ending the stream in silence.
	if err, isError := event.(error); isError {
		return Result{Errors: []*gqlerror.Error{gqlerror.Ensure(err)}}
	}

	e := s.executor
	nodes := nodesOf(s.selections)
	info := &schema.ResolveInfo{
		Schema:         e.schema,
		FieldName:      s.def.Name(),
		FieldNodes:     nodes,
		ReturnType:     s.def.Type,
		ParentType:     s.rootType,
		Path:           s.path,
		RootValue:      event,
		Operation:      e.operation,
		Fragments:      e.fragments,
		VariableValues: e.variables.Values(),
	}

	col := &collector{}
	resolved := event
	var err error
	if s.def.Resolve != nil {
		args, argErr := coerceArgumentValues(argumentOwner{field: s.def}, s.def.Args,
			nodes[0].Arguments, scopeOf(s.selections, e.variables), nodes[0], e.checks...)
		if argErr != nil {
			err = argErr
		} else {
			resolved, err = e.resolve(ctx, s.def, event, args, info)
		}
	}
	if err == nil {
		resolved, err = e.completeValue(ctx, col, s.def.Type, s.selections, info, s.path, resolved)
	}
	if err != nil {
		located := locate(err, nodes, s.path)
		if e.propagates(s.def.Type) {
			// Nothing can hold the null, so the response for this event has no
			// data at all.
			col.add(located)
			return Result{Errors: col.errors, Data: value.Just[*value.OrderedMap](nil)}
		}
		col.add(located)
		resolved = nil
	}

	data := value.NewOrderedMapSize(1)
	data.Set(s.key, resolved)
	return Result{Errors: col.errors, Data: value.Just(data)}
}
