package execution

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/internal/typeinfo"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// Request is one GraphQL request to run.
type Request struct {
	// Schema answers the request.
	Schema *schema.Schema
	// Document holds the operations and fragments, already parsed and, unless
	// the caller has a reason otherwise, already validated.
	Document *language.Document
	// OperationName picks the operation to run. It may be empty when the
	// document defines exactly one.
	OperationName string
	// Variables are the values supplied for the operation's variables. A
	// variable the caller omitted is absent from the map, which is a different
	// thing from one supplied as null: the first falls back to a default and
	// the second does not.
	Variables map[string]value.Maybe[any]
	// RootValue is passed to the resolvers of the root fields as their source.
	RootValue any
	// ContextValue is not here on purpose: whatever a resolver needs to know
	// about the caller travels in the context.Context passed to Execute, which
	// is where a Go program already looks for it.

	// Concurrency bounds how many fields of an object, and how many entries
	// of a list, are worked on at once. Zero or one does them one after
	// another, which is the default; see [Execute].
	//
	// The bound is per object and per list rather than over the request as a
	// whole, so a list of objects reaches it at both levels.
	Concurrency int

	// FieldResolver stands in for [DefaultResolver] where a field has no
	// resolver of its own. A server whose values all come from one place —
	// a database row, a map keyed some other way — can say so once here
	// instead of on every field.
	FieldResolver schema.FieldResolver
	// TypeResolver decides which object type a value is where an abstract
	// type does not say. It replaces asking each candidate whether the value
	// is one of it, which is what happens when neither is given.
	TypeResolver schema.TypeResolver
	// SubscribeResolver stands in for a subscription root field that has no
	// Subscribe of its own, and answers with the channel the events arrive
	// on. Leaving it unset falls through to the field's ordinary resolver and
	// then to [DefaultResolver], which reads the root value.
	SubscribeResolver schema.FieldResolver

	// HideSuggestions leaves the "Did you mean …?" out of every message.
	//
	// A suggestion is worked out from the schema, so it names input fields and
	// enum members that the request got close to. A server that does not
	// answer introspection is hiding those names on purpose, and a message
	// that offers the nearest one hands them over anyway.
	HideSuggestions bool

	// MaxCoercionErrors bounds how many problems with the request's variables
	// are reported before the rest are given up on. Zero means the default,
	// which is fifty; a negative number means no bound at all.
	//
	// A request can name a variable of a deeply nested input type and supply
	// something wrong at every leaf, and working all of them out costs the
	// server something for an answer nobody reads to the end.
	MaxCoercionErrors int
}

// defaultMaxCoercionErrors is how many problems with a request's variables
// are reported when the request does not say.
const defaultMaxCoercionErrors = 50

// maxCoercionErrors is what a request's MaxCoercionErrors comes to.
func (r Request) maxCoercionErrors() int {
	if r.MaxCoercionErrors == 0 {
		return defaultMaxCoercionErrors
	}
	return r.MaxCoercionErrors
}

// checkOptions is what a request's HideSuggestions comes to when the schema
// package is asked to check a value.
func (r Request) checkOptions() []schema.CheckOption {
	if r.HideSuggestions {
		return []schema.CheckOption{schema.WithoutSuggestions()}
	}
	return nil
}

// Result is a GraphQL response.
//
// Data is absent when the request failed before execution could begin, and
// present but null when a field that may not be null failed. The two are
// different answers and a client can tell them apart, which is why this is a
// [value.Maybe] rather than a pointer that might be nil.
type Result struct {
	// Errors is what went wrong. The specification asks for it first.
	Errors []*gqlerror.Error `json:"errors,omitempty"`
	// Data is the response, keyed in the order the document asked.
	Data value.Maybe[*value.OrderedMap] `json:"data,omitzero"`
}

// Execute runs a request and returns the response.
//
// Resolvers run one after another by default. A Go resolver that talks to a
// database would often rather run alongside its siblings, and setting
// [Request.Concurrency] above one asks for that — for the fields of an object
// and for the entries of a list alike, which is where a request that fetches a
// list of things gains most. The cost is that resolvers must then be safe to
// call from several goroutines at once, which is a promise this package cannot
// make on their behalf. The response is the same either way: fields keep the
// order the document asked for them in, entries keep theirs, and so do the
// errors — including which of them are reported, since an entry that may not
// be null ends the list there whichever way it was scheduled.
//
// A mutation's root fields always run one after another however Concurrency is
// set, because the specification requires it: each is expected to see what the
// one before it did.
//
// The document is expected to have passed validation. Execution does not check
// that a field exists or that an argument is of the right type, because a rule
// in the validation package has already said so; running an unvalidated
// document may produce a response that makes no sense rather than an error
// that explains why.
func Execute(ctx context.Context, req Request) Result {
	prepared, failure := prepare(req)
	if failure != nil {
		return Result{Errors: failure}
	}
	e := prepared.executor
	rootType, missing := prepared.root()
	if missing != nil {
		return startedAndFailed(missing)
	}

	// A document asking for parts of the response to arrive later cannot be
	// answered with one response, and answering it with everything at once
	// would not be what the client is waiting for.
	if node := incrementalDirectiveInEffect(e); node != nil {
		return failed(gqlerror.New(
			"This operation would produce more than one payload, because it uses "+
				quote("@"+nameOf(node.Name))+". Use ExecuteIncrementally to run it.",
			gqlerror.WithNodes(node)))
	}

	fields := CollectFields(e.schema, e.fragments, e.variables, rootType, e.operation.SelectionSet)
	// A fragment spread that got its arguments wrong is a fault in the request
	// rather than in any one field, so no field runs. Validation reports this
	// too; the check is here because the executor may be handed a document
	// that was never validated. Collecting is work the run does, so what comes
	// back is a null response rather than none at all.
	if errs := fields.Errors(); len(errs) > 0 {
		return startedAndFailed(errs...)
	}

	// The fields of a mutation are expected to happen in order, each seeing
	// what the one before it did.
	serially := e.operation.Operation == language.OperationMutation

	col := &collector{}
	data, failure2 := e.executeSelectionSet(ctx, col, rootType, req.RootValue, fields, nil, serially)
	if failure2 != nil {
		// A root field that may not be null failed, so there is no object to
		// return; the response says so with a null rather than by leaving data
		// out, which would mean the request never ran.
		col.add(failure2)
		return Result{Errors: col.errors, Data: value.Just[*value.OrderedMap](nil)}
	}
	return Result{Errors: col.errors, Data: value.Just(data)}
}

// preparedRequest is a request that has been checked over and turned into what
// running it needs.
type preparedRequest struct {
	executor *executor
	rootType *schema.ObjectType
}

// prepare works out what a request asks for, before any of it is run. Both
// [Execute] and [ExecuteIncrementally] begin here, so that they agree about
// which operation is being run and what its variables are.
func prepare(req Request) (*preparedRequest, []*gqlerror.Error) {
	if req.Schema == nil {
		return nil, []*gqlerror.Error{gqlerror.New("Must provide a schema.")}
	}
	if req.Document == nil {
		return nil, []*gqlerror.Error{gqlerror.New("Must provide a document.")}
	}
	operation, err := operationToRun(req.Document, req.OperationName)
	if err != nil {
		return nil, []*gqlerror.Error{err}
	}
	// A schema that is not sound cannot be executed against, as graphql-js
	// refuses to. The answer is worked out once per schema, so a request pays
	// nothing after the first; a schema built with AssumeValid pays nothing at
	// all.
	if err := schema.AssertValidSchema(req.Schema); err != nil {
		return nil, []*gqlerror.Error{gqlerror.Ensure(err)}
	}
	// A fragment declaring a variable of a type no value can have is a fault
	// in the document rather than in anything the request supplied, so it is
	// reported before the request begins — which is where graphql-js reports
	// it, and why the response leaves the data out rather than nulling it.
	if errs := fragmentSignatureErrors(req.Schema, req.Document); len(errs) > 0 {
		return nil, errs
	}
	variables, varErrs := coerceVariableValuesUpTo(req.maxCoercionErrors(),
		req.Schema, operation.VariableDefinitions, req.Variables, req.checkOptions()...)
	if len(varErrs) > 0 {
		// A request whose variables will not coerce never began, so it has no
		// data at all rather than null data.
		return nil, varErrs
	}

	return &preparedRequest{
		executor: &executor{
			schema:        req.Schema,
			fragments:     fragmentsOf(req.Document),
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
		rootType: req.Schema.RootType(operation.Operation),
	}, nil
}

// fragmentSignatureErrors reports every fragment variable declared with a type
// that cannot hold an input value.
//
// Every fragment in the document is read, not only the ones the operation
// spreads, because graphql-js builds each fragment's signatures as it scans
// the document and reports what it finds there.
func fragmentSignatureErrors(s *schema.Schema, doc *language.Document) []*gqlerror.Error {
	var errs []*gqlerror.Error
	for _, def := range doc.Definitions {
		fragment, isFragment := def.(*language.FragmentDefinition)
		if !isFragment {
			continue
		}
		for _, declaration := range fragment.VariableDefinitions {
			if declaration == nil || declaration.Variable == nil || declaration.Variable.Name == nil {
				continue
			}
			declared, known := typeinfo.TypeFromAST(s, declaration.Type)
			if known && schema.IsInputType(declared) {
				continue
			}
			errs = append(errs, gqlerror.New(
				"Variable "+quote("$"+declaration.Variable.Name.Value)+
					" expected value of type "+quote(language.Print(declaration.Type))+
					" which cannot be used as an input type.",
				gqlerror.WithNodes(declaration.Type)))
		}
	}
	return errs
}

// root is the type the operation runs against, and the complaint when the
// schema has none.
//
// graphql-js looks this up after execution has begun — inside the same guard
// that catches a root field failing — so its response says the data is null
// rather than leaving the data out, which would mean the request never ran.
// This is asked at the same point for the same reason.
func (p *preparedRequest) root() (*schema.ObjectType, *gqlerror.Error) {
	if p.rootType != nil {
		return p.rootType, nil
	}
	operation := p.executor.operation
	return nil, gqlerror.New(
		fmt.Sprintf("Schema is not configured to execute %s operation.", operation.Operation),
		gqlerror.WithNodes(operation))
}

// executor holds what stays the same for the whole of one request.
type executor struct {
	schema      *schema.Schema
	fragments   map[string]*language.FragmentDefinition
	variables   schema.VariableValues
	rootValue   any
	operation   *language.OperationDefinition
	concurrency int
	// fieldResolver and typeResolver are the request's own, standing in for
	// the defaults where a field or an abstract type does not say.
	fieldResolver schema.FieldResolver
	typeResolver  schema.TypeResolver
	// checks are what a value or a literal is checked with, which carries the
	// request's say over whether a message may suggest what it might have
	// meant.
	checks []schema.CheckOption

	// propagating says whether a field that may not be null carries its
	// failure up to the enclosing object. It is true unless the operation
	// asked otherwise with @experimental_disableErrorPropagation.
	propagating bool

	// incremental says whether @defer and @stream are being honoured.
	incremental bool
	// branching says the older payload format is being produced, where every
	// deferred fragment is answered on its own rather than the fields being
	// gathered by which set of fragments asked for them.
	branching bool
	// within is the set of deferrals the payload being built belongs to, which
	// is empty for the first one. A copy of the executor is made for each
	// deferred payload, with this set to that payload's deferrals.
	within []*DeferUsage
	// shared is what belongs to the request rather than to a branch of the
	// walk. Every copy of the executor points at the same one.
	shared *runState
	// deferred says what the client is told about each deferred fragment the
	// walk is inside. A @defer written once in a document stands for one
	// announcement per place it is reached — a fragment inside a list is
	// announced once per entry — so this belongs to the branch rather than to
	// the run: it is copied and added to wherever a @defer is met, and the
	// copy is what the walk below that point sees.
	deferred map[*DeferUsage]*announcement
}

// runState is the state one request has, as against the state a branch of the
// walk has. The executor is copied — for a deferred payload, and wherever a
// @defer changes what the walk below is inside — and every copy points at the
// same one of these.
type runState struct {
	// ids hands out the identifiers a client matches payloads by, so that no
	// two pieces take one. Only the publisher hands them out, one at a time.
	ids int
	// deferrals counts the deferred fragments the run has met, which settles
	// the order a set of them is put in. Sibling fields resolving alongside
	// each other may both meet one, so it is counted atomically.
	deferrals atomic.Int64
	// subfields remembers what each object type asks for beneath a group of
	// selections. The same question is asked once per object in a list, and
	// once per object in a list of lists, so a schema-sized answer would
	// otherwise be worked out tens of thousands of times over.
	subfields map[subfieldKey]subfieldEntry
	// mu guards subfields, which sibling fields may reach at the same time.
	mu sync.Mutex
}

// subfieldKey identifies a question already asked of the collector: what does
// this object type ask for beneath these selections?
//
// The selections stand for themselves: the same group of selections is reached
// again and again as a list is completed, and it is the same slice each time,
// handed out by the grouped field set that holds it. Its first element and its
// length name it. Nothing subslices a grouped field set's entries, so two
// slices that agree on both are the same slice, and holding the pointer keeps
// what it points at alive for as long as the answer is remembered.
type subfieldKey struct {
	objectType *schema.ObjectType
	selections *FieldSelection
	count      int
}

// subfieldEntry is one remembered answer.
type subfieldEntry struct {
	fields *GroupedFieldSet
	met    []*DeferUsage
}

// subfieldsOf is [collectSubfields] with the answer remembered.
//
// graphql-js remembers the same thing, keyed the same way, and for the same
// reason: "memoizing ensures the subfields are not repeatedly calculated,
// which saves overhead when resolving lists of values". There it is a
// process-wide weak map keyed on the request as well; here the table belongs
// to the request, so there is nothing to key on and nothing to expire.
func (e *executor) subfieldsOf(
	objectType *schema.ObjectType, selections []FieldSelection,
) (*GroupedFieldSet, []*DeferUsage) {
	if len(selections) == 0 {
		return collectSubfields(
			e.schema, e.fragments, e.variables, objectType, selections, e.incremental)
	}
	key := subfieldKey{objectType: objectType, selections: &selections[0], count: len(selections)}

	// Sibling fields may be resolving alongside each other, and each of them
	// may ask. The answer is worked out under the lock rather than racing to
	// produce two of them: two answers would be equal but not the same, and
	// what the table is for is that they are the same.
	e.shared.mu.Lock()
	defer e.shared.mu.Unlock()
	if entry, remembered := e.shared.subfields[key]; remembered {
		return entry.fields, entry.met
	}
	fields, met := collectSubfields(
		e.schema, e.fragments, e.variables, objectType, selections, e.incremental)
	e.coerceArguments(objectType, fields)
	if e.shared.subfields == nil {
		e.shared.subfields = make(map[subfieldKey]subfieldEntry, 16)
	}
	e.shared.subfields[key] = subfieldEntry{fields: fields, met: met}
	return fields, met
}

// collector gathers the errors of one part of the response.
//
// Each field gets its own, and a parent merges its children's in the order the
// document wrote them, so that what a caller sees does not depend on which
// resolver happened to finish first.
type collector struct {
	errors []*gqlerror.Error
	// pending is the work found while building this part of the response,
	// which the publisher delivers afterwards.
	pending []*pendingItem
	// nulled are the places a failure came to rest, so that a null now stands
	// there. Deferred work at or below one of them can never be delivered —
	// there is no object left to merge it into — so it is dropped rather than
	// announced. graphql-js calls this filtering.
	nulled []*value.Path
}

func (c *collector) add(errs ...*gqlerror.Error) {
	for _, err := range errs {
		if err != nil {
			c.errors = append(c.errors, err)
		}
	}
}

// addAt records a failure together with the place it came to rest.
func (c *collector) addAt(at *value.Path, errs ...*gqlerror.Error) {
	c.add(errs...)
	c.nulled = append(c.nulled, at)
}

// isNulled reports whether a path sits at or below somewhere a null came to
// rest.
func (c *collector) isNulled(at *value.Path) bool {
	for _, gone := range c.nulled {
		if under(at, gone) {
			return true
		}
	}
	return false
}

// under reports whether at is gone, or somewhere inside it. A nil gone is the
// root, which everything is inside.
func under(at, gone *value.Path) bool {
	if gone == nil {
		return true
	}
	for cur := at; cur != nil; cur = cur.Prev {
		if cur == gone {
			return true
		}
	}
	return false
}

// keepDeliverable drops the work that can no longer be delivered.
func (c *collector) keepDeliverable() {
	if len(c.nulled) == 0 || len(c.pending) == 0 {
		return
	}
	kept := c.pending[:0]
	for _, item := range c.pending {
		if !c.isNulled(item.path) {
			kept = append(kept, item)
		}
	}
	c.pending = kept
}

// executeSelectionSet resolves the fields of one object.
//
// It returns an error only when the object itself cannot stand: a field that
// may not be null failed, and there is nothing to put in its place. Anything
// else is recorded and the field is left null.
func (e *executor) executeSelectionSet(
	ctx context.Context,
	col *collector,
	objectType *schema.ObjectType,
	source any,
	fields *GroupedFieldSet,
	path *value.Path,
	serially bool,
) (*value.OrderedMap, *gqlerror.Error) {
	// Fields a deferred fragment asked for are not part of this payload; they
	// become pieces of their own, announced now and delivered later.
	plan := e.planDeferrals(fields)
	fields = plan.now

	keys := e.answerable(objectType, fields)
	data := value.NewOrderedMapSize(len(keys))

	// Every field is resolved even once one of them has failed in a way that
	// will null this object. Stopping would save the work, but what it would
	// really save is telling the caller about the rest of what went wrong, and
	// a response with no data is exactly when they want all of it. The first
	// such failure in the order the document wrote the fields is the one the
	// object comes down by.
	var failure *gqlerror.Error

	if serially || e.concurrency <= 1 || len(keys) < 2 {
		// One at a time, a field reports straight into the object's own
		// collector: it is already reporting in the order the document wrote
		// the fields, which is the order the response has to keep.
		for _, key := range keys {
			result, failed := e.executeField(
				ctx, col, objectType, source, fields, key, path.WithField(key, objectType.Name()))
			if failure == nil {
				failure = failed
			}
			data.Set(key, result)
		}
	} else {
		// Alongside one another, each field needs somewhere of its own to
		// write, and what they found is merged afterwards in the order the
		// document wrote them — so that the response does not depend on which
		// resolver finished first.
		slots := make([]fieldSlot, len(keys))
		e.inParallel(len(keys), func(i int) {
			key := keys[i]
			slots[i].result, slots[i].failure = e.executeField(
				ctx, &slots[i].collector, objectType, source, fields, key,
				path.WithField(key, objectType.Name()))
		})
		for i, key := range keys {
			col.add(slots[i].errors...)
			col.pending = append(col.pending, slots[i].pending...)
			col.nulled = append(col.nulled, slots[i].nulled...)
			if failure == nil {
				failure = slots[i].failure
			}
			data.Set(key, slots[i].result)
		}
	}

	if failure != nil {
		return nil, failure
	}

	// The pieces this level defers are recorded after the level's own fields
	// have run, so that anything found deeper is announced first. A fragment
	// written outside is met later than one written inside the thing it
	// encloses, and the client is told about them in that order.
	for _, group := range plan.later {
		col.pending = append(col.pending, e.deferredWork(objectType, source, group, path))
	}

	return data, nil
}

// fieldSlot is where one field of an object writes what it came to, where the
// fields were resolved alongside one another and each therefore needs
// somewhere of its own.
type fieldSlot struct {
	collector
	// result is the field's value, which stands in the response.
	result any
	// failure is set where the field may not be null and could not be
	// resolved, which is what brings the object down.
	failure *gqlerror.Error
}

// answerable returns the keys of the fields the type actually has.
//
// A field it does not have has nothing to resolve, and leaving the key out of
// the response is closer to what the document asked for than inventing a null
// for it. Validation reports the field separately; the executor may be handed
// a document that was never validated.
func (e *executor) answerable(objectType *schema.ObjectType, fields *GroupedFieldSet) []string {
	keys := fields.Keys()
	// Every key is answerable in all but one case — a document asks an object
	// type only for fields it has, and validation has already said so — and
	// the exception is an abstract type, where a fragment on a sibling type
	// contributes keys this one does not answer. The keys are therefore handed
	// back as they are until something has to be left out, and only then is a
	// list of what is left made.
	var kept []string
	for i, key := range keys {
		if e.answers(objectType, fields.Fields(key)) {
			if kept != nil {
				kept = append(kept, key)
			}
			continue
		}
		if kept == nil {
			kept = append(make([]string, 0, len(keys)-1), keys[:i]...)
		}
	}
	if kept == nil {
		return keys
	}
	return kept
}

// answers reports whether the object type has the field a group of selections
// asks for.
func (e *executor) answers(objectType *schema.ObjectType, selections []FieldSelection) bool {
	if len(selections) == 0 {
		return false
	}
	return e.schema.Field(objectType, nameOf(selections[0].Node.Name)) != nil
}

// inParallel runs n pieces of work at once, up to the configured limit.
//
// The work is a range of integers, so there is nothing to hand out: each
// goroutine takes the next index until there are none left. That is the bound
// as well — as many goroutines as the limit allows, no more — and it is why a
// wide selection set cannot spawn a goroutine per field.
//
// The caller takes a share rather than standing and waiting, which is one
// goroutine fewer every time and none at all where the limit is one.
//
// A goroutine is cheap but not free, and a channel handshake is dearer than
// either: handing out the work over a channel, or starting a goroutine per
// piece of it, both cost several times this. BenchmarkParallelScheduling
// measures the alternatives side by side.
func (e *executor) inParallel(n int, run func(int)) {
	limit := e.concurrency
	if limit > n {
		limit = n
	}
	var next atomic.Int64
	take := func() {
		for {
			i := int(next.Add(1)) - 1
			if i >= n {
				return
			}
			run(i)
		}
	}

	var wg sync.WaitGroup
	wg.Add(limit - 1)
	for range limit - 1 {
		go func() {
			defer wg.Done()
			take()
		}()
	}
	take()
	wg.Wait()
}

// executeField resolves one field and turns what comes back into its place in
// the response.
func (e *executor) executeField(
	ctx context.Context,
	col *collector,
	objectType *schema.ObjectType,
	source any,
	fields *GroupedFieldSet,
	key string,
	path *value.Path,
) (any, *gqlerror.Error) {
	// The set is asked for both, rather than one being read out of the other,
	// because it has read the nodes out already: a resolver is given them and
	// so is every error reported against the field, and the same set answers
	// for every object it is used for.
	selections := fields.Fields(key)
	nodes := fields.Nodes(key)
	field := nodes[0]
	name := nameOf(field.Name)
	def := e.schema.Field(objectType, name)
	if def == nil {
		// Validation reports this; with no definition there is nothing to
		// resolve, and leaving the key out is closer to the document than
		// inventing a null for it.
		return nil, nil
	}

	// A caller who has given up — a client that hung up, a deadline that
	// passed — should not be made to wait for the rest of the request. The
	// check happens per field so that whatever was already resolved is still
	// returned alongside the reason the rest is missing.
	if err := ctx.Err(); err != nil {
		located := locate(err, nodes, path)
		if e.propagates(def.Type) {
			return nil, located
		}
		col.add(located)
		return nil, nil
	}

	info := &schema.ResolveInfo{
		Schema:         e.schema,
		FieldName:      name,
		FieldNodes:     nodes,
		ReturnType:     def.Type,
		ParentType:     objectType,
		Path:           path,
		RootValue:      e.rootValue,
		Operation:      e.operation,
		Fragments:      e.fragments,
		VariableValues: e.variables.Values(),
	}

	args, argErr := e.argumentsFor(fields, key, def, field, selections)
	if argErr != nil {
		return e.fieldFailed(col, def, nodes, path, argErr)
	}

	resolved, err := e.resolve(ctx, def, source, args, info)
	if err == nil {
		resolved, err = e.completeValue(ctx, col, def.Type, selections, info, path, resolved)
	}
	if err == nil {
		return resolved, nil
	}
	return e.fieldFailed(col, def, nodes, path, err)
}

// argumentsFor returns a field's arguments, working them out where they were
// not worked out when the set was collected.
func (e *executor) argumentsFor(
	fields *GroupedFieldSet,
	key string,
	def *schema.Field,
	field *language.Field,
	selections []FieldSelection,
) (schema.Arguments, *gqlerror.Error) {
	if args, err, coerced := fields.arguments(key); coerced {
		return args, err
	}
	return coerceArgumentValues(argumentOwner{field: def}, def.Args, field.Arguments,
		scopeOf(selections, e.variables), field, e.checks...)
}

// fieldFailed says what a field that could not be resolved comes to.
//
// A field that may not be null cannot be left null, so its failure becomes the
// enclosing object's failure and travels up until it reaches somewhere that
// can hold a null — unless the operation asked for it not to.
func (e *executor) fieldFailed(
	col *collector,
	def *schema.Field,
	nodes []*language.Field,
	path *value.Path,
	err error,
) (any, *gqlerror.Error) {
	located := locate(err, nodes, path)
	if e.propagates(def.Type) {
		return nil, located
	}
	col.addAt(path, located)
	return nil, nil
}

// resolve calls a field's resolver, or the default one where it has none.
//
// A resolver is ordinary code that a server author wrote, and a panic in one
// would otherwise take down every request the process is serving, not just
// this field. It is turned into a failure of the field, which is what any
// other fault in a resolver produces.
func (e *executor) resolve(
	ctx context.Context,
	def *schema.Field,
	source any,
	args schema.Arguments,
	info *schema.ResolveInfo,
) (result any, err error) {
	resolver := def.Resolve
	if resolver == nil {
		resolver = e.fieldResolver
	}
	if resolver == nil {
		resolver = DefaultResolver
	}

	defer func() {
		if r := recover(); r != nil {
			result = nil
			err = &PanicError{Value: r, Stack: captureStack()}
		}
	}()
	return resolver(ctx, source, args, info)
}

// completeValue turns what a resolver returned into the shape the field's type
// calls for.
func (e *executor) completeValue(
	ctx context.Context,
	col *collector,
	fieldType schema.Type,
	selections []FieldSelection,
	info *schema.ResolveInfo,
	path *value.Path,
	resolved any,
) (any, error) {
	// A value that is one of this library's errors is a failure rather than
	// something to put in the response. It is the only way to fail one entry
	// of a list, since a list comes back from a single resolver and there is
	// no second return value per entry. Only [gqlerror.Error] counts: any Go
	// value may happen to have an Error method, and a server returning one as
	// data should get it back as data.
	if failed, isFailure := resolved.(*gqlerror.Error); isFailure {
		return nil, failed
	}

	if nonNull, isNonNull := fieldType.(*schema.NonNull); isNonNull {
		completed, err := e.completeValue(ctx, col, nonNull.OfType, selections, info, path, resolved)
		if err != nil {
			return nil, err
		}
		if completed == nil {
			return nil, gqlerror.Newf("Cannot return null for non-nullable field %s.%s.",
				info.ParentType.Name(), info.FieldName)
		}
		return completed, nil
	}

	if isNothing(resolved) {
		return nil, nil
	}

	switch t := fieldType.(type) {
	case *schema.List:
		return e.completeList(ctx, col, t, selections, info, path, resolved)
	case *schema.ScalarType:
		return coerceOutput(t.CoerceOutputValue, atLeaf(resolved), t.Name())
	case *schema.EnumType:
		return completeEnum(t, resolved)
	case *schema.ObjectType:
		return e.completeObject(ctx, col, t, selections, info, path, resolved)
	case *schema.InterfaceType:
		return e.completeAbstract(ctx, col, t, selections, info, path, resolved)
	case *schema.UnionType:
		return e.completeAbstract(ctx, col, t, selections, info, path, resolved)
	default:
		return nil, fmt.Errorf("cannot complete value of unexpected type %q", fieldType.String())
	}
}

// completeList turns what a resolver returned into a list, one entry at a
// time.
//
// An entry that fails is nulled where the element type allows it, so one bad
// entry does not cost the whole list; where it does not, the list fails.
func (e *executor) completeList(
	ctx context.Context,
	col *collector,
	listType *schema.List,
	selections []FieldSelection,
	info *schema.ResolveInfo,
	path *value.Path,
	resolved any,
) (any, error) {
	items := reflect.ValueOf(resolved)
	switch items.Kind() {
	case reflect.Slice, reflect.Array:
	default:
		return nil, gqlerror.Newf("Expected Iterable, but did not find one for field %q.",
			info.ParentType.Name()+"."+info.FieldName)
	}

	// @stream asks for the entries past the first few to arrive later, so only
	// those first few are completed now.
	count := items.Len()
	streaming, from, label, err := e.streamUsage(selections, path, count)
	if err != nil {
		return nil, err
	}
	if streaming {
		count = from
	}

	out, failure := e.completeEntries(
		ctx, col, listType.OfType, selections, info, path, items, 0, count)
	if failure != nil {
		return nil, failure
	}

	if streaming {
		col.pending = append(col.pending,
			e.streamedWork(listType, selections, info, path, items, from, label))
	}
	return out, nil
}

// streamUsage reads a @stream written on a field, saying whether the entries
// past the first few are to arrive later and how many the first few are.
//
// A list short enough to fit in what was asked for is not streamed at all:
// announcing a piece that arrives empty tells a client nothing that hasNext
// does not already.
func (e *executor) streamUsage(
	selections []FieldSelection, path *value.Path, count int,
) (bool, int, string, error) {
	if !e.incremental || len(selections) == 0 {
		return false, 0, "", nil
	}
	// @stream is written on a field, so it is the field's own list it asks to
	// be delivered in pieces. A list inside that list is not what was asked
	// about, and is completed whole.
	if path.IsIndex() {
		return false, 0, "", nil
	}
	stream := declaredDirective(e.schema, schema.Stream)
	args, written := directiveValues(stream, selections[0].Node.Directives, scopeOf(selections, e.variables))
	if !written {
		return false, 0, "", nil
	}
	if on, given := args.Get("if"); given && on != true {
		return false, 0, "", nil
	}

	from := 0
	if v, given := args.Get("initialCount"); given {
		if n, isInt := v.(int32); isInt {
			from = int(n)
		}
	}
	// Asking for a negative number of entries first is not a smaller request,
	// it is a nonsensical one, and answering it some other way would hide the
	// mistake.
	if from < 0 {
		return false, 0, "", errors.New("initialCount must be a positive integer")
	}
	if from >= count {
		return false, 0, "", nil
	}
	label, _ := args.Get("label")
	text, _ := label.(string)
	return true, from, text, nil
}

// streamedWork makes a pending item for the entries of a list that are to
// arrive after the first payload.
//
// The entries a server can produce without waiting go out together, which is
// the point: a client sees them without waiting for the ones the server
// cannot produce quickly.
func (e *executor) streamedWork(
	listType *schema.List,
	selections []FieldSelection,
	info *schema.ResolveInfo,
	path *value.Path,
	items reflect.Value,
	from int,
	label string,
) *pendingItem {
	// The identifier is handed out when the piece is announced, not here, so
	// that a piece dropped for being undeliverable does not use one up.
	item := &pendingItem{
		groups:   []*announcement{{label: label, path: path}},
		path:     path,
		streamed: true,
		from:     from,
	}

	// The executor is taken as it stands here, so that an entry completed later
	// is completed in the same place the list was found.
	inner := *e

	item.next = func(ctx context.Context) step {
		col := &collector{}
		// The entries are completed the same way as those of a list that went
		// out with the first payload, which is what keeps the two from coming
		// to differ.
		entries, failure := inner.completeEntries(
			ctx, col, listType.OfType, selections, info, path, items, from, items.Len())
		if failure != nil {
			// The entry may not be null and there is nowhere to put one, so
			// nothing of the rest of the list can be delivered — not even the
			// entries before it, which have no list left to be appended to.
			col.add(failure)
			return step{errors: col.errors, done: true}
		}
		return step{
			payload:    &IncrementalPayload{Items: entries, Errors: col.errors},
			discovered: col.pending,
			done:       true,
		}
	}
	return item
}

// entry is what completing one entry of a list came to, where the entries were
// completed alongside one another and each therefore needs somewhere of its
// own to report into.
type entry struct {
	collector
	// failure is set where the entry may not be null and could not be
	// completed, which is what brings the list down.
	failure *gqlerror.Error
}

// completeEntries completes the entries of a list between two indices and
// merges what they found, in the order the list holds them.
//
// It is the one place the entries of a list are completed, so that a list
// completed now and the rest of one delivered later by @stream cannot come to
// differ. What it returns is the entries and, where one of them may not be
// null and could not be completed, that failure; what the caller does with the
// failure is where the two differ.
//
// Nothing an entry after the failing one found is merged. Completed one at a
// time, the entries after it are never asked for at all — which is what
// graphql-js does with a list already in hand — so reporting what they found
// would make the answer depend on how the work was scheduled.
func (e *executor) completeEntries(
	ctx context.Context,
	col *collector,
	itemType schema.Type,
	selections []FieldSelection,
	info *schema.ResolveInfo,
	path *value.Path,
	items reflect.Value,
	from, to int,
) ([]any, *gqlerror.Error) {
	count := to - from
	out := make([]any, count)

	// complete does one entry, reporting into wherever it was given. An entry
	// that may not be null and could not be completed is answered for rather
	// than reported, since it is the list that has to be nulled and not the
	// entry.
	complete := func(i int, into *collector) *gqlerror.Error {
		at := path.WithIndex(from + i)
		completed, err := e.completeValue(
			ctx, into, itemType, selections, info, at, items.Index(from+i).Interface())
		if err == nil {
			out[i] = completed
			return nil
		}
		located := locate(err, nodesOf(selections), at)
		if e.propagates(itemType) {
			return located
		}
		into.add(located)
		return nil
	}

	// One at a time, the entries report straight into the field's own
	// collector and the walk stops where the list comes down.
	if e.concurrency <= 1 || count < 2 {
		for i := range count {
			if failure := complete(i, col); failure != nil {
				return out, failure
			}
		}
		return out, nil
	}

	// Alongside one another, each entry needs somewhere of its own to report
	// into, and what they found is merged afterwards in the order the list
	// holds them.
	entries := make([]entry, count)
	e.inParallel(count, func(i int) {
		entries[i].failure = complete(i, &entries[i].collector)
	})
	for i := range entries {
		col.add(entries[i].errors...)
		col.pending = append(col.pending, entries[i].pending...)
		col.nulled = append(col.nulled, entries[i].nulled...)
		if entries[i].failure != nil {
			return out, entries[i].failure
		}
	}
	return out, nil
}

// coerceArguments works out the arguments of every field the set asks of the
// object type, before any object is resolved against it.
//
// A field written once asks for the same arguments however many objects it is
// resolved against, and the set is remembered for all of them, so this is the
// place the work belongs. It is done while the set is still being made and
// before anything else can see it, which is what keeps it from having to be
// guarded.
//
// What is wrong with the arguments is kept rather than reported: it belongs to
// the field being resolved, at the path it is resolved at, and neither is
// known yet.
func (e *executor) coerceArguments(objectType *schema.ObjectType, fields *GroupedFieldSet) {
	for _, key := range fields.Keys() {
		nodes := fields.Nodes(key)
		if len(nodes) == 0 {
			continue
		}
		def := e.schema.Field(objectType, nameOf(nodes[0].Name))
		if def == nil {
			// The type has no such field. Validation reports that; there is
			// nothing here to work out.
			continue
		}
		args, err := coerceArgumentValues(argumentOwner{field: def}, def.Args, nodes[0].Arguments,
			scopeOf(fields.Fields(key), e.variables), nodes[0], e.checks...)
		fields.coerce(key, args, err)
	}
}

// completeObject resolves the selections asked of an object.
func (e *executor) completeObject(
	ctx context.Context,
	col *collector,
	objectType *schema.ObjectType,
	selections []FieldSelection,
	info *schema.ResolveInfo,
	path *value.Path,
	resolved any,
) (any, error) {
	// A type may say which values are one of it, and a value that is not is a
	// fault in the server rather than something to put in the response.
	if objectType.IsTypeOf != nil {
		is, err := objectType.IsTypeOf(ctx, resolved, info)
		if err != nil {
			return nil, err
		}
		if !is {
			return nil, gqlerror.Newf("Expected value of type %q but got: %s.",
				objectType.Name(), value.Describe(resolved))
		}
	}

	subfields, met := e.subfieldsOf(objectType, selections)
	// A @defer is announced at the place it was written, which is here: the
	// object whose selection set holds it. Where its fields turn out to sit
	// deeper, they are still delivered under this announcement.
	inner := e.withDeferrals(met, path)
	if errs := subfields.Errors(); len(errs) > 0 {
		// Deeper down, a spread that got its arguments wrong fails the field
		// it was written under, which is where a reader is looking.
		return nil, errs[0]
	}
	data, failure := inner.executeSelectionSet(ctx, col, objectType, resolved, subfields, path, false)
	if failure != nil {
		return nil, failure
	}
	return data, nil
}

// completeAbstract works out which object type a value is before resolving the
// selections asked of it.
func (e *executor) completeAbstract(
	ctx context.Context,
	col *collector,
	abstract schema.AbstractType,
	selections []FieldSelection,
	info *schema.ResolveInfo,
	path *value.Path,
	resolved any,
) (any, error) {
	objectType, err := e.resolveType(ctx, abstract, resolved, info)
	if err != nil {
		return nil, err
	}
	return e.completeObject(ctx, col, objectType, selections, info, path, resolved)
}

// resolveType decides which of an abstract type's possible types a value is.
//
// The schema may say how, either with a resolver on the abstract type or with
// one on each candidate. Failing both, the value is asked what it is, which
// covers the common case of a Go type named after the GraphQL one.
func (e *executor) resolveType(
	ctx context.Context,
	abstract schema.AbstractType,
	resolved any,
	info *schema.ResolveInfo,
) (*schema.ObjectType, error) {
	var byName string
	switch t := abstract.(type) {
	case *schema.InterfaceType:
		if t.ResolveType != nil {
			name, err := t.ResolveType(ctx, resolved, info)
			if err != nil {
				return nil, err
			}
			byName = name
		}
	case *schema.UnionType:
		if t.ResolveType != nil {
			name, err := t.ResolveType(ctx, resolved, info)
			if err != nil {
				return nil, err
			}
			byName = name
		}
	}

	possible := e.schema.PossibleTypes(abstract)
	if byName == "" && e.typeResolver != nil {
		name, err := e.typeResolver(ctx, resolved, info)
		if err != nil {
			return nil, err
		}
		byName = name
	}
	if byName == "" {
		byName = declaredTypeName(resolved)
	}
	if byName == "" {
		// Failing all of that, each candidate is asked whether the value is
		// one of it, and failing that the value's own Go type name is taken as
		// the answer.
		for _, candidate := range possible {
			if candidate.IsTypeOf == nil {
				continue
			}
			is, err := candidate.IsTypeOf(ctx, resolved, info)
			if err != nil {
				return nil, err
			}
			if is {
				return candidate, nil
			}
		}
		byName = goTypeName(resolved)
	}
	if byName == "" {
		return nil, gqlerror.Newf(
			"Abstract type %q must resolve to an Object type at runtime for field %q. "+
				"Either the %q type should provide a \"resolveType\" function "+
				"or each possible type should provide an \"isTypeOf\" function.",
			abstract.Name(), info.ParentType.Name()+"."+info.FieldName, abstract.Name())
	}

	// Saying which of the four mistakes it is saves the reader from working
	// out for themselves whether the name is unknown, known but not an object,
	// or an object the abstract type does not cover.
	named := e.schema.Type(byName)
	if named == nil {
		return nil, gqlerror.Newf(
			"Abstract type %q was resolved to a type %q that does not exist inside the schema.",
			abstract.Name(), byName)
	}
	objectType, isObject := named.(*schema.ObjectType)
	if !isObject {
		return nil, gqlerror.Newf("Abstract type %q was resolved to a non-object type %q.",
			abstract.Name(), byName)
	}
	for _, candidate := range possible {
		if candidate == objectType {
			return candidate, nil
		}
	}
	return nil, gqlerror.Newf("Runtime Object type %q is not a possible type for %q.",
		byName, abstract.Name())
}

// atLeaf follows a pointer to the value it points at.
//
// A pointer is how a Go type says a value may be absent, so a field that can
// be null is naturally held as one. A nil pointer has already been read as
// null by the time this is reached; what is left is a pointer to the value the
// scalar or enum should be given, rather than the pointer itself.
//
// Only a leaf's value is followed. An object's is left as it is, because a
// method standing in for a field is often declared on the pointer type, and
// following it would put that method out of reach.
func atLeaf(v any) any {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return v
	}
	return rv.Interface()
}

// completeEnum turns a resolver's value into the name of an enum member.
func completeEnum(t *schema.EnumType, resolved any) (any, error) {
	// The value is looked for as it arrived and again with any pointer
	// followed. A member's value may itself be a pointer, and following that
	// first would lose the identity that says which member it is.
	//
	// Whether the value changed is decided by whether it was a pointer, not by
	// comparing the two: a resolver may hand back a map or a slice, and
	// comparing one of those with == takes the process down.
	if member := t.ValueFor(resolved); member != nil {
		return member.Name(), nil
	}
	if reflect.ValueOf(resolved).Kind() == reflect.Pointer {
		if member := t.ValueFor(atLeaf(resolved)); member != nil {
			return member.Name(), nil
		}
	}
	return nil, gqlerror.Newf("Enum %q cannot represent value: %s",
		t.Name(), value.Describe(resolved))
}

// coerceOutput puts a value through a scalar's output coercion.
func coerceOutput(coerce schema.OutputValueCoercer, resolved any, typeName string) (any, error) {
	if coerce == nil {
		return resolved, nil
	}
	out, err := coerce(resolved)
	if err != nil {
		// The type said why it would not represent the value, and what it said
		// is more use than anything said from the outside.
		return nil, err
	}
	// A response carries neither nothing nor null here: a field that may be
	// null is answered with null by its resolver, not by its type's coercion.
	// Which of the two the coercion answered with is said, as graphql-js says
	// it.
	held, represented := out.Get()
	switch {
	case !represented:
		return nil, gqlerror.Newf(
			"Expected `%s.CoerceOutputValue(%s)` to return non-nullable value, returned: undefined",
			typeName, value.Describe(resolved))
	case held == nil:
		return nil, gqlerror.Newf(
			"Expected `%s.CoerceOutputValue(%s)` to return non-nullable value, returned: null",
			typeName, value.Describe(resolved))
	}
	return held, nil
}

// operationToRun picks the operation a request asked for.
func operationToRun(doc *language.Document, name string) (*language.OperationDefinition, *gqlerror.Error) {
	var found *language.OperationDefinition
	count := 0
	for _, def := range doc.Definitions {
		operation, isOperation := def.(*language.OperationDefinition)
		if !isOperation {
			continue
		}
		count++
		if name == "" {
			found = operation
			continue
		}
		if operation.Name != nil && operation.Name.Value == name {
			return operation, nil
		}
	}
	if name != "" {
		return nil, gqlerror.New("Unknown operation named " + quote(name) + ".")
	}
	switch count {
	case 0:
		return nil, gqlerror.New("Must provide an operation.")
	case 1:
		return found, nil
	default:
		return nil, gqlerror.New("Must provide operation name if query contains multiple operations.")
	}
}

// fragmentsOf indexes a document's fragments by name.
func fragmentsOf(doc *language.Document) map[string]*language.FragmentDefinition {
	fragments := map[string]*language.FragmentDefinition{}
	for _, def := range doc.Definitions {
		if fragment, isFragment := def.(*language.FragmentDefinition); isFragment && fragment.Name != nil {
			if _, taken := fragments[fragment.Name.Value]; !taken {
				fragments[fragment.Name.Value] = fragment
			}
		}
	}
	return fragments
}

// failed returns a response for a request that never ran, which has no data at
// all rather than null data.
func failed(errs ...*gqlerror.Error) Result {
	return Result{Errors: errs}
}

// startedAndFailed is the response to a run that began and then could not go
// on. The data is null rather than absent: absent says the request never ran,
// and a client tells the two apart.
func startedAndFailed(errs ...*gqlerror.Error) Result {
	return Result{Errors: errs, Data: value.Just[*value.OrderedMap](nil)}
}

// locate attaches the field and the response path to an error, so that a
// caller can see which part of their request it is about.
func locate(err error, selections []*language.Field, path *value.Path) *gqlerror.Error {
	nodes := make([]language.Node, len(selections))
	for i, field := range selections {
		nodes[i] = field
	}
	return gqlerror.Located(err, nodes, path.AsSlice())
}

// scopeOf is the variable scope a field's selections were found under, which
// is the request's own unless the field was reached through a fragment that
// declares arguments of its own.
//
// Selections that share a response key have been checked to agree about their
// arguments, so the first of them settles the scope, exactly as it settles
// which node the arguments are read from.
func scopeOf(selections []FieldSelection, fallback schema.VariableValues) schema.VariableValues {
	if len(selections) > 0 && selections[0].Variables.IsSet() {
		return selections[0].Variables
	}
	return fallback
}

// errorPropagationDisabled reports whether the operation asked that a field
// which may not be null be answered with null when it fails, rather than the
// failure travelling up.
//
// The schema has to declare the directive for it to mean anything. graphql-js
// reads the name off the operation alone; here, as with @defer and @stream, a
// directive a schema never offered is not one a request can invoke. Validation
// refuses such a document anyway, so the two only differ for a document that
// skipped it.
func errorPropagationDisabled(s *schema.Schema, operation *language.OperationDefinition) bool {
	name := schema.DisableErrorPropagation.Name()
	if s.Directive(name) == nil {
		return false
	}
	return findDirectiveNamed(operation.Directives, name) != nil
}

// propagates reports whether a failure of a field of this type has to travel
// up to the enclosing object rather than being answered with null.
//
// A field that may not be null cannot be left null, so normally it does, and
// the failure comes to rest at the nearest place that can hold a null. An
// operation written with @experimental_disableErrorPropagation asks for the
// other thing: the response keeps whatever did resolve, and the promise the
// schema made about that one field is broken instead.
func (e *executor) propagates(t schema.Type) bool {
	return e.propagating && schema.IsNonNullType(t)
}
