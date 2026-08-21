package execution

import (
	"context"
	"encoding/json"

	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/value"
)

// LegacyIncrementalResult is what running a document that uses @defer or
// @stream gives in the payload format that came before the current one.
//
// It is the same pair as [IncrementalResult] — a first response and the rest
// afterwards — in the shape clients written against the earlier draft of the
// incremental delivery specification expect.
type LegacyIncrementalResult struct {
	// Initial is the response that can be sent straight away.
	Initial LegacyInitialResult
	// Subsequent carries the rest and is closed when there is no more. It is
	// nil when the document deferred nothing after all, in which case Initial
	// is the whole response — a nil channel never yields, so a caller ranging
	// over this without looking would wait for ever.
	Subsequent <-chan LegacySubsequentResult
}

// LegacyInitialResult is the first payload of a response in the older format.
//
// It differs from [InitialResult] in having nothing to announce: the older
// format tells a client what is coming by sending it, not by naming it first.
type LegacyInitialResult struct {
	// Errors is what went wrong while the first response was built.
	Errors []*gqlerror.Error `json:"errors,omitempty"`
	// Data is the response so far.
	Data value.Maybe[*value.OrderedMap] `json:"data,omitzero"`
	// HasNext says more payloads follow.
	HasNext bool `json:"hasNext"`
}

// MarshalJSON writes the first response of a run.
//
// hasNext is left out when nothing follows, for the reason
// [InitialResult.MarshalJSON] gives: it is then not the first payload of
// anything but the whole response.
func (r LegacyInitialResult) MarshalJSON() ([]byte, error) {
	type withNext LegacyInitialResult
	if r.HasNext {
		return json.Marshal(withNext(r))
	}
	return json.Marshal(struct {
		Errors []*gqlerror.Error              `json:"errors,omitempty"`
		Data   value.Maybe[*value.OrderedMap] `json:"data,omitzero"`
	}{Errors: r.Errors, Data: r.Data})
}

// LegacySubsequentResult is a payload after the first, in the older format.
// The fields are in the order graphql-js writes them, so that a payload from
// either implementation is the same text.
type LegacySubsequentResult struct {
	// HasNext says whether anything follows this payload.
	HasNext bool `json:"hasNext"`
	// Incremental is what arrived in this payload.
	Incremental []LegacyIncrementalPayload `json:"incremental,omitempty"`
}

// LegacyIncrementalPayload is one piece of a response in the older format.
//
// A piece names where it goes by its own path rather than by an identifier the
// client was given beforehand, and carries the label the document wrote on the
// directive. A piece that could not be delivered says so with a null in place
// of what it would have held.
// The fields are in the order graphql-js writes them: what arrived, where it
// goes, and then what was said about it.
type LegacyIncrementalPayload struct {
	// Data is the fields of a deferred fragment, and is null where the
	// fragment could not be delivered. It is absent for a streamed list.
	Data value.Maybe[*value.OrderedMap] `json:"data,omitzero"`
	// Items are entries of a streamed list, and are null where the rest of
	// the list could not be delivered. They are absent for a fragment.
	Items value.Maybe[[]any] `json:"items,omitzero"`
	// Path is where in the response this piece belongs: the place the @defer
	// was written, or the place of the first entry this payload carries.
	Path []any `json:"path"`
	// Errors is what went wrong in this piece.
	Errors []*gqlerror.Error `json:"errors,omitempty"`
	// Label is what the directive was labelled, if it was.
	Label string `json:"label,omitempty"`
}

// ExecuteLegacyIncrementally runs a request in the payload format that came
// before the current one.
//
// [ExecuteIncrementally] is what a new client wants: it announces each piece
// before sending it, so a client knows what to expect and when everything has
// arrived. This is for a client written against the earlier draft, where each
// payload simply says where it goes.
//
// The two differ in more than shape. The current format works out which set of
// deferred fragments each field belongs to, so a field asked for both inside a
// fragment and outside it is sent once; this one answers each fragment on its
// own, sending the same field again for each fragment that asked. graphql-js
// keeps both for the same reason and calls this one legacy.
func ExecuteLegacyIncrementally(ctx context.Context, req Request) LegacyIncrementalResult {
	prepared, failure := prepare(req)
	if failure != nil {
		return LegacyIncrementalResult{Initial: LegacyInitialResult{Errors: failure}}
	}
	e := prepared.executor
	e.incremental = true
	e.branching = true
	rootType, missing := prepared.root()
	if missing != nil {
		return LegacyIncrementalResult{Initial: LegacyInitialResult{
			Errors: []*gqlerror.Error{missing},
			Data:   value.Just[*value.OrderedMap](nil),
		}}
	}

	fields, met := CollectFieldsIncrementally(
		e.schema, e.fragments, e.variables, rootType, e.operation.SelectionSet, true)
	if errs := fields.Errors(); len(errs) > 0 {
		return LegacyIncrementalResult{Initial: LegacyInitialResult{
			Errors: errs, Data: value.Just[*value.OrderedMap](nil),
		}}
	}
	e = e.withDeferrals(met, nil)

	col := &collector{}
	serially := e.operation.Operation == language.OperationMutation
	data, failed := e.executeSelectionSet(ctx, col, rootType, req.RootValue, fields, nil, serially)

	initial := LegacyInitialResult{Errors: col.errors}
	if failed != nil {
		col.add(failed)
		initial.Errors = col.errors
		initial.Data = value.Just[*value.OrderedMap](nil)
		return LegacyIncrementalResult{Initial: initial}
	}
	initial.Data = value.Just(data)

	col.keepDeliverable()
	work := col.pending
	if len(work) == 0 {
		return LegacyIncrementalResult{Initial: initial}
	}
	initial.HasNext = true
	return LegacyIncrementalResult{Initial: initial, Subsequent: e.publishLegacy(ctx, work)}
}

// publishLegacy runs the deferred work and sends what it produced in the older
// format.
//
// Everything ready at the same moment goes in one payload, for the reason
// [executor.publish] gives. Nothing is held back to be reported together: in
// this format a piece names where it goes, so there is no announcement for a
// later failure to contradict.
func (e *executor) publishLegacy(ctx context.Context, work []*pendingItem) <-chan LegacySubsequentResult {
	out := make(chan LegacySubsequentResult)
	go func() {
		defer close(out)

		var result LegacySubsequentResult
		for len(work) > 0 {
			item := work[0]
			work = work[1:]
			step := item.next(ctx)
			work = append(work, step.discovered...)
			result.Incremental = append(result.Incremental, item.legacyPayload(step))
		}

		select {
		case out <- result:
		case <-ctx.Done():
		}
	}()
	return out
}

// legacyPayload says what one piece of work comes to in the older format.
func (i *pendingItem) legacyPayload(step step) LegacyIncrementalPayload {
	under := i.groups[0]
	payload := LegacyIncrementalPayload{Label: under.label, Errors: step.errors}

	failed := step.payload == nil
	switch {
	case i.streamed && failed:
		// The rest of the list could not be delivered, which is said of the
		// list itself rather than of an entry of it.
		payload.Items = value.Just[[]any](nil)
		payload.Path = pathOrRoot(under.path)
	case i.streamed:
		payload.Items = value.Just(step.payload.Items)
		payload.Errors = step.payload.Errors
		// A streamed payload is named by the place of its first entry.
		payload.Path = pathOrRoot(under.path.WithIndex(i.from))
	case failed:
		payload.Data = value.Just[*value.OrderedMap](nil)
		payload.Path = pathOrRoot(under.path)
	default:
		payload.Data = value.Just(step.payload.Data)
		payload.Errors = step.payload.Errors
		payload.Path = pathOrRoot(under.path)
	}
	return payload
}

// pathOrRoot renders a path, coping with the root of the response: it is an
// empty list there, not an absent one.
func pathOrRoot(path *value.Path) []any {
	if written := path.AsSlice(); written != nil {
		return written
	}
	return []any{}
}
