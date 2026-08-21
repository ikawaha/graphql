package execution

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// IncrementalResult is what running a document that uses @defer or @stream
// gives: a first response, and the rest of it afterwards.
type IncrementalResult struct {
	// Initial is the response that can be sent straight away.
	Initial InitialResult
	// Subsequent carries the rest and is closed when there is no more. It is
	// nil when the document deferred nothing after all, in which case Initial
	// is the whole response — a nil channel never yields, so a caller ranging
	// over this without looking would wait for ever.
	//
	// A caller must read from it until it closes, or cancel the context.
	Subsequent <-chan SubsequentResult
}

// InitialResult is the first payload of an incremental response.
type InitialResult struct {
	// Errors is what went wrong in the part delivered here.
	Errors []*gqlerror.Error `json:"errors,omitempty"`
	// Data is the part of the response that did not wait.
	Data value.Maybe[*value.OrderedMap] `json:"data,omitzero"`
	// Pending announces what is still to come, so that a client knows what it
	// is waiting for before any of it arrives.
	Pending []PendingResult `json:"pending,omitempty"`
	// HasNext says whether anything follows. It is written out only when it
	// does: a run that turned out to defer nothing answers with an ordinary
	// response, which has no hasNext at all.
	HasNext bool `json:"hasNext"`
}

// MarshalJSON writes the first response of a run.
//
// hasNext is left out when nothing follows, because then this is not the first
// payload of anything — it is the whole response, and an ordinary response
// does not carry the field. An initial payload that does have something
// following always says hasNext: true, so the value alone settles which of the
// two this is.
func (r InitialResult) MarshalJSON() ([]byte, error) {
	type withNext InitialResult
	if r.HasNext {
		return json.Marshal(withNext(r))
	}
	return json.Marshal(struct {
		Errors  []*gqlerror.Error              `json:"errors,omitempty"`
		Data    value.Maybe[*value.OrderedMap] `json:"data,omitzero"`
		Pending []PendingResult                `json:"pending,omitempty"`
	}{Errors: r.Errors, Data: r.Data, Pending: r.Pending})
}

// SubsequentResult is a payload after the first.
// The fields are in the order graphql-js writes them, so that a payload from
// either implementation is the same text.
type SubsequentResult struct {
	// HasNext says whether anything follows.
	HasNext bool `json:"hasNext"`
	// Pending announces work found while producing this payload.
	Pending []PendingResult `json:"pending,omitempty"`
	// Incremental carries the data itself.
	Incremental []IncrementalPayload `json:"incremental,omitempty"`
	// Completed says which announced pieces are finished, and reports any
	// error that finished one early.
	Completed []CompletedResult `json:"completed,omitempty"`
}

// PendingResult announces a piece of the response that has not arrived.
type PendingResult struct {
	// ID identifies the piece, and is what an incremental payload refers to.
	ID string `json:"id"`
	// Path is where in the response the piece belongs.
	Path []any `json:"path"`
	// Label is what the document called it, if anything.
	Label string `json:"label,omitempty"`
}

// IncrementalPayload is one piece of a response, delivered late.
type IncrementalPayload struct {
	// ID says which announced piece this belongs to.
	ID string `json:"id"`
	// Data is the fields of a deferred fragment.
	Data *value.OrderedMap `json:"data,omitempty"`
	// Items are entries of a streamed list, to be appended in order.
	Items []any `json:"items,omitempty"`
	// SubPath says how much further down than the announcement the data sits.
	// A deferred fragment's fields may be found deeper than the fragment
	// itself, and this is the way from the one to the other; it is absent
	// where they are in the same place.
	SubPath []any `json:"subPath,omitempty"`
	// Errors is what went wrong in this piece.
	Errors []*gqlerror.Error `json:"errors,omitempty"`
}

// CompletedResult says that an announced piece has finished.
type CompletedResult struct {
	// ID says which piece.
	ID string `json:"id"`
	// Errors is what stopped it, where something did. A piece that completes
	// with errors delivered no data, and a client should treat what it was
	// waiting for as missing rather than merely late.
	Errors []*gqlerror.Error `json:"errors,omitempty"`
}

// ExecuteIncrementally runs a request, delivering what @defer and @stream ask
// to be delayed after the rest.
//
// @defer on a fragment says its fields need not hold up the response: what the
// rest of the document asked for is sent first, and the fragment's fields
// follow. @stream on a list says the same of a list's entries beyond the first
// few. Both let a server answer the parts of a query it can answer quickly
// without waiting for the parts it cannot.
//
// The first payload comes back directly. The rest arrive on
// [IncrementalResult.Subsequent], and the last of them says hasNext: false. A
// document that defers nothing gets a first payload that is the whole response
// and a nil channel, which is the way to tell an ordinary response from one
// that has more to come.
//
// The schema must declare @defer and @stream for a document to use them, since
// neither is one of the directives every schema has; [schema.Defer] and
// [schema.Stream] are the definitions to add. Validation refuses a document
// using one the schema does not declare.
func ExecuteIncrementally(ctx context.Context, req Request) IncrementalResult {
	prepared, failure := prepare(req)
	if failure != nil {
		return IncrementalResult{Initial: InitialResult{Errors: failure}}
	}
	e := prepared.executor
	e.incremental = true
	rootType, missing := prepared.root()
	if missing != nil {
		return IncrementalResult{Initial: InitialResult{
			Errors: []*gqlerror.Error{missing},
			Data:   value.Just[*value.OrderedMap](nil),
		}}
	}

	fields, met := CollectFieldsIncrementally(
		e.schema, e.fragments, e.variables, rootType, e.operation.SelectionSet, true)
	// A @defer written in the operation's own selection set is announced at
	// the root of the response.
	e = e.withDeferrals(met, nil)
	// A fragment spread that got its arguments wrong is a fault in the request
	// rather than in any one field, so no field runs and nothing is announced.
	// Collecting is work the run does, so the data is null rather than absent.
	if errs := fields.Errors(); len(errs) > 0 {
		return IncrementalResult{Initial: InitialResult{
			Errors: errs, Data: value.Just[*value.OrderedMap](nil),
		}}
	}

	col := &collector{}
	serially := e.operation.Operation == language.OperationMutation
	data, failed := e.executeSelectionSet(ctx, col, rootType, req.RootValue, fields, nil, serially)

	initial := InitialResult{Errors: col.errors}
	if failed != nil {
		col.add(failed)
		initial.Errors = col.errors
		initial.Data = value.Just[*value.OrderedMap](nil)
		// Nothing that was waiting on the response can be delivered into it.
		return IncrementalResult{Initial: initial}
	}
	initial.Data = value.Just(data)

	// Work under a path a failure nulled can never be delivered, so it is not
	// announced either.
	col.keepDeliverable()

	work := col.pending
	if len(work) == 0 {
		return IncrementalResult{Initial: initial}
	}
	initial.Pending = e.announce(work)
	initial.HasNext = true

	return IncrementalResult{Initial: initial, Subsequent: e.publish(ctx, work)}
}

// publish runs the deferred work and sends what it produced.
//
// Everything ready at the same moment goes in one payload, which is what a
// client is best served by: it merges a whole payload into the response at
// once rather than a piece at a time. A resolver here answers rather than
// promising to answer, so a piece found while producing a payload is ready
// too and joins the same one; a synchronous run therefore has exactly one
// payload after the first, as graphql-js does when nothing it is waiting on
// is a promise.
func (e *executor) publish(ctx context.Context, work []*pendingItem) <-chan SubsequentResult {
	out := make(chan SubsequentResult)
	go func() {
		defer close(out)

		var result SubsequentResult
		// What a piece produced is held until the run is over rather than put
		// in the payload as it comes. A fragment's fields arrive together or
		// not at all: where one piece of it cannot be delivered, the pieces
		// that could are dropped too, and there would be no taking them back
		// once written.
		type held struct {
			item    *pendingItem
			payload IncrementalPayload
		}
		var delivered []held

		for len(work) > 0 {
			item := work[0]
			work = work[1:]
			// An announcement that has failed takes its remaining work with
			// it: there is nothing left to deliver the piece under.
			if item.dropped() {
				continue
			}
			step := item.next(ctx)

			// Work found while producing this payload is announced with it, so
			// that a client learns of it no later than the data it sits
			// beside. It is announced before anything completes, so that a
			// fragment still holding work never looks finished.
			result.Pending = append(result.Pending, e.announce(step.discovered)...)
			work = append(work, step.discovered...)

			if len(step.errors) > 0 {
				result.Completed = append(result.Completed, fail(item, step.errors)...)
				continue
			}
			if step.payload != nil {
				delivered = append(delivered, held{item: item, payload: *step.payload})
			}
			completed, pending := e.finish(item)
			result.Completed = append(result.Completed, completed...)
			result.Pending = append(result.Pending, pending...)
		}

		for _, piece := range delivered {
			if piece.item.dropped() {
				// Every fragment the piece belonged to failed while the rest
				// of the run went on, so there is nothing to deliver it under.
				continue
			}
			under, sub := piece.item.attribution()
			piece.payload.ID = under.id
			piece.payload.SubPath = sub
			result.Incremental = append(result.Incremental, piece.payload)
		}

		select {
		case out <- result:
		case <-ctx.Done():
		}
	}()
	return out
}

// finish records that a piece was delivered, and says which announcements had
// nothing further and so are complete, along with what a client is now told to
// expect because they are.
func (e *executor) finish(item *pendingItem) ([]CompletedResult, []PendingResult) {
	var completed []CompletedResult
	var pending []PendingResult
	for _, group := range item.groups {
		group.outstanding--
		if group.outstanding > 0 || group.failed {
			continue
		}
		group.done = true
		completed = append(completed, CompletedResult{ID: group.id})
		// What was written inside this fragment has somewhere to go now.
		for _, child := range group.children {
			pending = append(pending, e.tell(child))
		}
		group.children = nil
	}
	return completed, pending
}

// fail records that a piece could not be delivered.
//
// What a fragment asked for arrives together or not at all, so a piece that
// failed finishes every announcement it belonged to, whatever else they were
// still waiting on. The rest of their work is dropped rather than delivered
// into a response that will not hold it.
func fail(item *pendingItem, errs []*gqlerror.Error) []CompletedResult {
	var out []CompletedResult
	for _, group := range item.groups {
		if group.failed {
			continue
		}
		group.failed = true
		out = append(out, CompletedResult{ID: group.id, Errors: errs})
		// Whatever was written inside was never announced, so there is nothing
		// to tell the client about it; it is marked so that its own work is
		// dropped rather than delivered into a response that will not hold it.
		condemn(group.children)
		group.children = nil
	}
	return out
}

// condemn marks the fragments written inside one that failed, which can no
// more be delivered than the fragment holding them.
func condemn(children []*announcement) {
	for _, child := range children {
		child.failed = true
		condemn(child.children)
		child.children = nil
	}
}

// announce tells the client to expect the announcements a batch of work
// belongs to, and counts the work against them.
//
// The identifiers are given out here rather than where the work was found, so
// that a piece dropped for being undeliverable does not use one up and leave a
// gap in what the client is told. An announcement is made once however many
// pieces of work belong to it.
//
// A fragment written inside another is held back until the enclosing one has
// been delivered: until then there is nothing in the response for its fields
// to be merged into. A fragment that turned out to hold no work of its own —
// everything it asked for was in the first response already — is never
// announced at all, and whatever was written inside it takes its place.
func (e *executor) announce(work []*pendingItem) []PendingResult {
	// Everything the batch brings is registered before any of it is told, so
	// that a fragment and the one it is written inside are not announced the
	// wrong way round because their pieces were found in that order.
	var fresh []*announcement
	for _, item := range work {
		for _, group := range item.groups {
			group.outstanding++
			if !group.registered {
				group.registered = true
				fresh = append(fresh, group)
			}
		}
	}
	// Deepest first, and otherwise in the order the work that belongs to them
	// was found. That is the order a reader meets them too: a fragment written
	// inside another is met second and delivered second.
	//
	// The sort is stable and looks only at the depth on purpose. Entries of a
	// list may be worked on alongside one another, so which of two fragments
	// at the same depth was met first is a matter of scheduling; the order the
	// work was found in is not, since it is merged in the order the list holds
	// its entries.
	sort.SliceStable(fresh, func(a, b int) bool {
		return fresh[a].path.Len() > fresh[b].path.Len()
	})

	var out []PendingResult
	for _, group := range fresh {
		if waiting := group.waitFor(); waiting != nil {
			waiting.children = append(waiting.children, group)
			continue
		}
		out = append(out, e.tell(group))
	}
	return out
}

// waitFor returns the fragment this one has to wait for, or nil where there is
// none left to wait for.
//
// A fragment that holds no work of its own is passed over: its fields were in
// the first response, so there is nothing to wait for it to deliver.
func (a *announcement) waitFor() *announcement {
	for enclosing := a.parent; enclosing != nil; enclosing = enclosing.parent {
		if enclosing.registered && !enclosing.done {
			return enclosing
		}
	}
	return nil
}

// tell hands an announcement its identifier and says what the client is told.
func (e *executor) tell(group *announcement) PendingResult {
	group.announced = true
	group.id = e.nextID()
	// Something announced at the root of the response has an empty path, not
	// an absent one: the field is always a list on the wire.
	path := group.path.AsSlice()
	if path == nil {
		path = []any{}
	}
	return PendingResult{ID: group.id, Path: path, Label: group.label}
}

// announcement is something the client was told to expect: a deferred
// fragment, or the rest of a streamed list.
//
// It is what an identifier names. A deferred fragment is announced once, at
// the place the @defer was written, however many pieces of work its fields
// turn out to be split into: the fields of a fragment may sit deeper than the
// fragment itself, and a field written in two branches belongs to both.
type announcement struct {
	label string
	// order is where in the run the @defer was first met. It settles which of
	// two deferrals comes first wherever a set of them has to be put in order,
	// and it counts across the whole run: a deferral's place within the
	// selection set it was written in says nothing about how it stands against
	// one written elsewhere.
	order int
	// path is where the client is told to expect this. For a fragment it is
	// the object whose selection set holds the @defer, which may be shallower
	// than where its fields are delivered.
	path *value.Path
	// parent is the deferred fragment this one is written inside, once that
	// one is known to hold work of its own. A client is told about a fragment
	// only when the fragment enclosing it has been delivered, so that it hears
	// about a piece of the response in the order the pieces arrive.
	parent   *announcement
	children []*announcement
	// id is handed out when the announcement is made, and is empty until then.
	id string
	// registered says the announcement is known to hold work; announced says
	// the client has been told about it, which comes later where a fragment is
	// waiting on the one it is written inside.
	registered bool
	announced  bool
	done       bool
	// outstanding is how many pieces of work are still to be delivered. The
	// announcement completes when the count reaches zero.
	outstanding int
	// failed says a piece of this announcement's work could not be delivered,
	// which finishes the whole of it: what a fragment asked for arrives
	// together or not at all.
	failed bool
}

// pendingItem is a piece of the response that arrives after the first payload.
//
// A deferred fragment's fields may be split into several of these, one per
// depth they sit at, and a piece may belong to more than one fragment where
// the same field was written in more than one of them. A streamed list is one
// piece belonging to one announcement.
type pendingItem struct {
	// groups are the announcements this piece belongs to.
	groups []*announcement
	// path is where the piece's data belongs in the response, which may be
	// deeper than the announcement's path.
	path *value.Path
	// next produces the piece's payload. It closes over the executor as it
	// stood where the piece was found, deferrals and all: a piece found inside
	// a deferred fragment belongs to that fragment, and running it against the
	// outermost payload's view would defer its fields all over again.
	next func(ctx context.Context) step
	// streamed says the piece is the rest of a list rather than the fields of
	// a deferred fragment, and from is the index its first entry takes. Only
	// the older payload format asks: it names a streamed payload by the place
	// of its first entry, where the current one names the list and counts from
	// what was already sent.
	streamed bool
	from     int
}

// dropped reports whether nothing is left to deliver the piece under, which is
// so once every announcement it belongs to has failed.
func (i *pendingItem) dropped() bool {
	for _, group := range i.groups {
		if !group.failed {
			return false
		}
	}
	return true
}

// attribution picks the announcement a piece is delivered under, and where the
// data sits relative to it.
//
// A piece belonging to several fragments is delivered under the most specific
// of them — the one written deepest, which is the first, since the groups are
// held that way — and the others simply complete. That is what makes the path
// relative: whoever reads the payload has been told where the announcement is,
// and subPath says how much further down the data goes.
//
// A fragment that failed is still named here. What the piece holds arrived,
// and the place it belongs is the place it belongs; that the fragment also has
// something to report says nothing about where its data goes. A piece with no
// fragment left standing at all is not delivered — see [pendingItem.dropped].
func (i *pendingItem) attribution() (*announcement, []any) {
	under := i.groups[0]
	return under, i.path.AsSlice()[under.path.Len():]
}

// withDeferrals returns the executor the walk below this object should use,
// having recorded where each @defer newly met here was written.
//
// A @defer is one thing in a document and many in a response: written inside a
// list, it stands for one announcement per entry, each at that entry's path.
// The deferrals themselves are shared — the collector's answer is remembered,
// so every entry of a list is handed the same ones — and it is here that they
// are told apart, by giving each branch of the walk its own map from the
// deferral to what the client is told about it.
//
// graphql-js does the same in getNewDeliveryGroupMap, and for the same reason:
// "as defer directives may be used with operations returning lists, a
// DeferUsage object may correspond to many DeliveryGroups".
//
// Where nothing new was met the walk carries on with what it had, which is
// every object in a request that defers nothing.
func (e *executor) withDeferrals(met []*DeferUsage, path *value.Path) *executor {
	if len(met) == 0 {
		return e
	}
	branch := *e
	branch.deferred = make(map[*DeferUsage]*announcement, len(e.deferred)+len(met))
	for deferral, announced := range e.deferred {
		branch.deferred[deferral] = announced
	}
	for _, deferral := range met {
		// The enclosing fragment is looked up in the map being built: one met
		// here may be written inside another met here, and the collector hands
		// them over in the order the document wrote them, outermost first.
		branch.deferred[deferral] = &announcement{
			label:  deferral.Label,
			path:   path,
			order:  int(e.shared.deferrals.Add(1)),
			parent: branch.deferred[deferral.Parent],
		}
	}
	return &branch
}

// step is what one turn of a pending item produced.
type step struct {
	// payload is what to deliver, or nil where this turn produced nothing.
	payload *IncrementalPayload
	// discovered is work found while producing it, such as a @defer inside a
	// deferred fragment.
	discovered []*pendingItem
	// errors is what stopped the piece, where something did.
	errors []*gqlerror.Error
	// done says the piece has nothing more to deliver.
	done bool
}

// deferPlan says which of an object's fields belong to the payload being
// built, and which belong to payloads of their own.
type deferPlan struct {
	// now is the fields to resolve into the payload being built.
	now *GroupedFieldSet
	// later groups the rest by the set of deferrals they belong to, in the
	// order the document reached them.
	later []*deferGroup
}

// deferGroup is the fields of one deferred payload.
type deferGroup struct {
	deferrals []*DeferUsage
	fields    *GroupedFieldSet
}

// planDeferrals splits an object's fields into those that belong to the
// payload being built and those that belong to later ones.
//
// A field written both inside a deferred fragment and outside it is not
// deferred: the client asked for it in the first response, and asking for it
// again later would deliver it twice.
func (e *executor) planDeferrals(fields *GroupedFieldSet) deferPlan {
	if !e.incremental {
		return deferPlan{now: fields}
	}
	if e.branching {
		return e.planBranchingDeferrals(fields)
	}

	plan := deferPlan{now: &GroupedFieldSet{}}
	byKey := map[string]*deferGroup{}
	for _, key := range fields.Keys() {
		selections := fields.Fields(key)
		deferrals := e.filteredDeferrals(selections)
		if sameDeferrals(deferrals, e.within) {
			for _, selection := range selections {
				plan.now.add(key, selection)
			}
			continue
		}
		id := e.deferralsKey(deferrals)
		group, started := byKey[id]
		if !started {
			group = &deferGroup{deferrals: deferrals, fields: &GroupedFieldSet{}}
			byKey[id] = group
			plan.later = append(plan.later, group)
		}
		for _, selection := range selections {
			group.fields.add(key, selection)
		}
	}
	plan.now.seal()
	for _, group := range plan.later {
		group.fields.seal()
	}
	return plan
}

// filteredDeferrals returns the deferrals a group of selections belongs to.
//
// A selection written outside any deferred fragment settles it: the field
// belongs to the first response, and the empty set says so. Otherwise a
// deferral whose ancestor is also in the set is dropped, since delivering the
// ancestor delivers this one with it.
func (e *executor) filteredDeferrals(selections []FieldSelection) []*DeferUsage {
	set := make(map[*DeferUsage]bool, len(selections))
	for _, selection := range selections {
		if selection.Defer == nil {
			return nil
		}
		set[selection.Defer] = true
	}
	for deferral := range set {
		for parent := deferral.Parent; parent != nil; parent = parent.Parent {
			if set[parent] {
				delete(set, deferral)
				break
			}
		}
	}
	out := make([]*DeferUsage, 0, len(set))
	for deferral := range set {
		out = append(out, deferral)
	}
	// The set is built from a map, so it is put in a settled order: which
	// deferral a payload is delivered under must not vary between runs. The
	// order is the one the run met them in, which is the only one that holds
	// across deferrals written in different selection sets.
	sort.Slice(out, func(i, j int) bool {
		return e.deferred[out[i]].order < e.deferred[out[j]].order
	})
	return out
}

// sameDeferrals reports whether two sets of deferrals are the same.
func sameDeferrals(a, b []*DeferUsage) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// deferralsKey names a set of deferrals, so that fields belonging to the same
// set are gathered into one payload.
func (e *executor) deferralsKey(deferrals []*DeferUsage) string {
	parts := make([]string, len(deferrals))
	for i, deferral := range deferrals {
		parts[i] = strconv.Itoa(e.deferred[deferral].order)
	}
	return strings.Join(parts, ",")
}

// planBranchingDeferrals is [executor.planDeferrals] for the older payload
// format, which branches rather than gathers.
//
// The current format asks which *set* of deferred fragments a response key
// belongs to, so a field written in two of them is delivered once, and one
// written both inside a fragment and outside it is not deferred at all. The
// older format asks each selection which single fragment it was written in,
// and gives every fragment its own payload holding everything it asked for —
// the same field over again where two fragments both asked.
//
// graphql-js keeps the two apart the same way, as buildExecutionPlan and
// buildBranchingExecutionPlan.
func (e *executor) planBranchingDeferrals(fields *GroupedFieldSet) deferPlan {
	plan := deferPlan{now: &GroupedFieldSet{}}
	started := map[*DeferUsage]*deferGroup{}
	for _, key := range fields.Keys() {
		for _, selection := range fields.Fields(key) {
			var deferrals []*DeferUsage
			if selection.Defer != nil {
				deferrals = []*DeferUsage{selection.Defer}
			}
			if sameDeferrals(deferrals, e.within) {
				plan.now.add(key, selection)
				continue
			}
			group, going := started[selection.Defer]
			if !going {
				group = &deferGroup{deferrals: deferrals, fields: &GroupedFieldSet{}}
				started[selection.Defer] = group
				plan.later = append(plan.later, group)
			}
			group.fields.add(key, selection)
		}
	}
	plan.now.seal()
	for _, group := range plan.later {
		group.fields.seal()
	}
	return plan
}

// deferredWork makes a pending item for a group of deferred fields.
func (e *executor) deferredWork(
	objectType *schema.ObjectType,
	source any,
	group *deferGroup,
	path *value.Path,
) *pendingItem {
	item := &pendingItem{path: path}
	for _, deferral := range group.deferrals {
		item.groups = append(item.groups, e.deferred[deferral])
	}
	// The deepest fragment first: it is the one the piece is delivered under,
	// and the one a client is told about first. Two fragments sharing a field
	// both enclose the place that field sits, so one of them is always written
	// inside the other and the order is settled.
	sort.SliceStable(item.groups, func(a, b int) bool {
		return item.groups[a].path.Len() > item.groups[b].path.Len()
	})

	// The payload is built as though the deferred fields were the whole
	// selection set, with the deferrals it belongs to standing as what the
	// walk is already inside.
	inner := *e
	inner.within = group.deferrals

	item.next = func(ctx context.Context) step {
		col := &collector{}
		data, failed := inner.executeSelectionSet(ctx, col, objectType, source, group.fields, path, false)
		// Work found inside this payload that its own errors have nulled can
		// no more be delivered than work nulled by the first response.
		col.keepDeliverable()
		if failed != nil {
			col.add(failed)
			// Nothing can be delivered: the fields that may not be null failed
			// and there is nowhere in this payload to put the null.
			return step{errors: col.errors, done: true}
		}
		return step{
			payload:    &IncrementalPayload{Data: data, Errors: col.errors},
			discovered: col.pending,
			done:       true,
		}
	}
	return item
}

// nextID hands out the identifiers a client uses to match payloads to what was
// announced.
func (e *executor) nextID() string {
	id := strconv.Itoa(e.shared.ids)
	e.shared.ids++
	return id
}

// incrementalDirectiveInEffect returns the first @defer or @stream the
// operation would actually act on, or nil.
//
// A schema has to declare the two for a document to use them, and most do not,
// so the common case costs two lookups and no walk at all. A directive
// switched off with `if: false` asks for nothing, so it does not count.
func incrementalDirectiveInEffect(e *executor) *language.Directive {
	deferName, streamName := schema.Defer.Name(), schema.Stream.Name()
	if e.schema.Directive(deferName) == nil && e.schema.Directive(streamName) == nil {
		return nil
	}

	var found *language.Directive
	seen := map[string]bool{}

	var walk func(node language.Node)
	walk = func(node language.Node) {
		if found != nil || node == nil {
			return
		}
		language.Visit(node, language.Visitor{
			Enter: func(n language.Node, _ language.VisitContext) language.VisitAction {
				if found != nil {
					return language.VisitBreak
				}
				switch typed := n.(type) {
				case *language.Directive:
					name := nameOf(typed.Name)
					if name != deferName && name != streamName {
						return language.VisitContinue
					}
					def := e.schema.Directive(name)
					if args, written := directiveValues(def, []*language.Directive{typed}, e.variables); written {
						if on, given := args.Get("if"); given && on != true {
							return language.VisitContinue
						}
					}
					found = typed
					return language.VisitBreak

				case *language.FragmentSpread:
					// A fragment is followed once: a document that spreads one
					// twice, or in a cycle, must not be walked for ever.
					name := nameOf(typed.Name)
					if seen[name] {
						return language.VisitContinue
					}
					seen[name] = true
					walk(e.fragments[name])
				}
				return language.VisitContinue
			},
		})
	}
	walk(e.operation)
	return found
}
