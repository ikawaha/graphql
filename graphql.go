// Package graphql answers GraphQL requests.
//
// A request is text, a schema, and whatever variables came with it. Answering
// one means parsing the text, checking that the schema can answer it, and
// running it. [Do] is all three:
//
//	result := graphql.Do(ctx, graphql.Params{
//		Schema:    s,
//		Query:     `{ user(id: "1") { name } }`,
//		Variables: graphql.Variables(map[string]any{"id": "1"}),
//	})
//
// The result marshals to the response the specification describes, so a server
// can write it out as it is.
//
// # The packages underneath
//
// This package is the way in. Everything it does is done by packages that can
// also be used on their own, which is what a server wants once it does more
// than answer one request at a time:
//
//   - [github.com/ikawaha/graphql/schema] describes a schema, in Go or from SDL.
//   - [github.com/ikawaha/graphql/language] parses and prints documents.
//   - [github.com/ikawaha/graphql/validation] checks a document against a schema.
//   - [github.com/ikawaha/graphql/execution] runs one, and holds the resolver API.
//   - [github.com/ikawaha/graphql/utilities] builds, prints and compares schemas.
//   - [github.com/ikawaha/graphql/gqlerror] is the error a response carries.
//   - [github.com/ikawaha/graphql/value] holds [Maybe], which is how a value
//     that was not supplied is told apart from one supplied as null.
//
// A server with persisted queries parses and validates once and executes many
// times; it should call those packages directly rather than [Do].
//
// # Omitted, null, and a value
//
// GraphQL distinguishes a variable that was not supplied from one supplied as
// null: the first falls back to a default and the second does not. Go's nil
// cannot tell them apart, so a request's variables are [Maybe] values. A map
// decoded from a request body already has the distinction right, since a key
// that was not sent is not in the map:
//
//	var body struct {
//		Query     string                     `json:"query"`
//		Variables map[string]graphql.Maybe[any] `json:"variables"`
//	}
//
// [Variables] is the shorthand for the other case, where a Go caller is
// supplying all of them.
package graphql

import (
	"context"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
	"github.com/ikawaha/graphql/validation"
	"github.com/ikawaha/graphql/value"
)

// The types below are the ones a caller meets without reaching for a
// subpackage. They are aliases rather than definitions, so a value of one is
// the same value the subpackage produces.
type (
	// Schema is what a request is answered against.
	Schema = schema.Schema
	// Result is a response: the data, and whatever went wrong.
	Result = execution.Result
	// Error is one thing that went wrong.
	Error = gqlerror.Error
	// Maybe holds a value that may not have been supplied, which is not the
	// same as one supplied as null.
	Maybe[T any] = value.Maybe[T]
	// OrderedMap is an object in a response, keyed in the order the document
	// asked for the fields.
	OrderedMap = value.OrderedMap

	// IncrementalResult is what a request using @defer or @stream gives.
	IncrementalResult = execution.IncrementalResult
	// LegacyIncrementalResult is the same in the payload format that came
	// before the current one.
	LegacyIncrementalResult = execution.LegacyIncrementalResult
	// SubscriptionResult is what starting a subscription gives.
	SubscriptionResult = execution.SubscriptionResult
	// ResolveInfo tells a resolver where in a request it is being called.
	ResolveInfo = schema.ResolveInfo
	// Arguments are the arguments a field was called with.
	Arguments = schema.Arguments
)

// Just says a value was supplied.
func Just[T any](v T) Maybe[T] { return value.Just(v) }

// Nothing says a value was not supplied.
func Nothing[T any]() Maybe[T] { return value.Nothing[T]() }

// Variables is shorthand for a set of variables that were all supplied.
//
// A variable meant to be null is supplied as a nil entry in the map, which is
// a different thing from leaving it out. Where the distinction has to be made
// entry by entry, build the map with [Just] and [Nothing] instead.
func Variables(supplied map[string]any) map[string]Maybe[any] {
	out := make(map[string]Maybe[any], len(supplied))
	for name, v := range supplied {
		out[name] = value.Just(v)
	}
	return out
}

// Params is a request to answer.
type Params struct {
	// Schema answers the request.
	Schema *Schema

	// Query is the document, as text. It is ignored when Document is set.
	Query string
	// Document is an already parsed document, for a server that parses once
	// and runs many times.
	Document *language.Document
	// ParseOptions configure the parse, and are ignored when Document is set.
	// [language.MaxTokens] is the one a server exposed to the internet wants,
	// since it bounds what a request can cost before any of it is answered.
	ParseOptions []language.ParseOption

	// OperationName picks the operation to run. It may be empty when the
	// document defines exactly one.
	OperationName string
	// Variables are the values supplied for the operation's variables. One
	// that was not supplied is absent from the map, which is a different thing
	// from one supplied as null.
	Variables map[string]Maybe[any]
	// RootValue is passed to the resolvers of the root fields as their source.
	RootValue any

	// Rules replaces the rules the document is checked against. Nil means the
	// ones the specification requires, which is what almost every server
	// wants; a rule of a server's own goes alongside them:
	//
	//	Rules: append(slices.Clone(validation.SpecifiedRules), myRule)
	//
	// A list that is empty but not nil checks nothing, which is not the same
	// as nil. Say [Params.SkipValidation] instead where that is what is meant:
	// it says so in its name, and execution is written to assume its input has
	// been checked.
	Rules []validation.Rule
	// SkipValidation runs the document without checking it first.
	//
	// Execution is written to assume its input has been checked: it does not
	// verify that a field exists or that an argument is of the right type,
	// because a rule has already said so. Set this only for a document that
	// was validated earlier — a persisted query whose text cannot change —
	// and never for one that arrived with the request.
	SkipValidation bool

	// Concurrency bounds how many fields of an object, and how many entries
	// of a list, are worked on at once. Zero or one does them one after
	// another, which is the default; above one, resolvers must be safe to call
	// from several goroutines at once.
	Concurrency int

	// FieldResolver stands in for the default way of reading a field's value
	// where the schema gave the field no resolver of its own. A server whose
	// values all come from one place can say so once here rather than on
	// every field.
	FieldResolver schema.FieldResolver
	// TypeResolver decides which object type a value is where an interface or
	// a union does not say. It replaces asking each candidate type whether the
	// value is one of it.
	TypeResolver schema.TypeResolver

	// Harness replaces the stages the request goes through — parsing,
	// checking, running — with a server's own. Nil runs the standard ones,
	// which is what almost every request wants; see [Harness].
	Harness *Harness

	// MaxErrors bounds how many problems checking the document reports before
	// it gives up.
	//
	// Unset means the default, which is 100. Zero is a bound like any other:
	// the first problem is already one too many. A negative number means no
	// bound at all.
	MaxErrors Maybe[int]
	// MaxCoercionErrors bounds how many problems with the request's variables
	// are reported before the rest are given up on, on the same terms as
	// MaxErrors. Unset means the default, which is fifty.
	MaxCoercionErrors Maybe[int]
	// HideSuggestions leaves the "Did you mean …?" out of every message, both
	// while the document is being checked and while it runs.
	//
	// A suggestion is worked out from the schema, so it names types, fields
	// and enum members that the request got close to. A server that does not
	// answer introspection is hiding those names on purpose, and a message
	// that offers the nearest one hands them over anyway. Turning the
	// suggestions off keeps what is wrong being said without saying what
	// would have been right.
	HideSuggestions bool
}

// Do parses, checks and runs a request.
//
// Anything that goes wrong before the request could run — text that will not
// parse, a document the schema cannot answer — comes back as errors with no
// data at all. A field that fails once the request is running comes back as a
// null in the data alongside the reason, which is the difference between a
// request that could not be answered and one that was answered incompletely.
//
// A document using @defer or @stream cannot be answered with one response; use
// [DoIncrementally] for those, and [Subscribe] for a subscription.
func Do(ctx context.Context, params Params) Result {
	req, h, errs := params.request()
	if len(errs) > 0 {
		return Result{Errors: errs}
	}
	return h.Execute(ctx, req)
}

// DoIncrementally parses, checks and runs a request, delivering what @defer
// and @stream ask to be delayed after the rest.
//
// The first payload comes back directly and the rest are ranged over. Nothing
// deferred runs until they are, so a caller that sends the first response and
// stops has not paid for the rest; see [execution.ExecuteIncrementally].
func DoIncrementally(ctx context.Context, params Params) IncrementalResult {
	req, h, errs := params.request()
	if len(errs) > 0 {
		return IncrementalResult{Initial: execution.InitialResult{Errors: errs}}
	}
	return h.ExecuteIncrementally(ctx, req)
}

// DoLegacyIncrementally is [DoIncrementally] in the payload format that came
// before the current one, for a client written against the earlier draft of
// the incremental delivery specification.
//
// See [execution.ExecuteLegacyIncrementally] for how the two differ, which is
// in more than the shape of a payload.
func DoLegacyIncrementally(ctx context.Context, params Params) LegacyIncrementalResult {
	req, h, errs := params.request()
	if len(errs) > 0 {
		return LegacyIncrementalResult{Initial: execution.LegacyInitialResult{Errors: errs}}
	}
	return h.ExecuteLegacyIncrementally(ctx, req)
}

// Subscribe parses, checks and starts a subscription.
//
// The responses arrive on a channel, one per event; see
// [execution.Subscribe] for what a caller owes that channel and for how a
// server produces the events.
func Subscribe(ctx context.Context, params Params) SubscriptionResult {
	req, h, errs := params.request()
	if len(errs) > 0 {
		return SubscriptionResult{Errors: errs}
	}
	return h.Subscribe(ctx, req)
}

// request turns the parameters into something to run, parsing and checking the
// document on the way.
func (p Params) request() (execution.Request, Harness, []*Error) {
	h := p.harness()
	if p.Schema == nil {
		return execution.Request{}, h, []*Error{gqlerror.New("Must provide a schema.")}
	}
	// What is wrong with the schema is answered before the document is looked
	// at, and each thing separately, as graphql-js answers it. A schema the
	// server never checked is caught here rather than part way through a
	// response. The answer is worked out once per schema.
	if problems := schema.ValidateSchema(p.Schema); len(problems) > 0 {
		return execution.Request{}, h, problems
	}

	doc := p.Document
	if doc == nil {
		if p.Query == "" {
			return execution.Request{}, h, []*Error{gqlerror.New("Must provide a query.")}
		}
		parsed, err := h.Parse(p.Query, p.ParseOptions...)
		if err != nil {
			// A syntax error already knows where it is; this keeps that.
			return execution.Request{}, h, []*Error{gqlerror.Ensure(err)}
		}
		doc = parsed
	}

	if !p.SkipValidation {
		// Nil means the specified rules; a list that is empty but not nil
		// means no rules, as it does for [schema.Config.Directives] and for
		// [validation.WithRules]. The specification has nothing to say about
		// this — it describes no way of choosing rules at all — so what
		// settles it is that a caller who wrote an empty list wrote it on
		// purpose.
		rules := p.Rules
		if rules == nil {
			rules = validation.SpecifiedRules
		}
		opts := []validation.Option{validation.WithRules(rules...)}
		if asked, given := p.MaxErrors.Get(); given {
			opts = append(opts, validation.WithMaxErrors(asked))
		}
		if p.HideSuggestions {
			opts = append(opts, validation.WithoutSuggestions())
		}
		if errs := h.Validate(p.Schema, doc, opts...); len(errs) > 0 {
			return execution.Request{}, h, errs
		}
	}

	return execution.Request{
		Schema:        p.Schema,
		Document:      doc,
		OperationName: p.OperationName,
		Variables:     p.Variables,
		RootValue:     p.RootValue,
		Concurrency:   p.Concurrency,
		FieldResolver: p.FieldResolver,
		TypeResolver:  p.TypeResolver,

		HideSuggestions:   p.HideSuggestions,
		MaxCoercionErrors: p.MaxCoercionErrors,
	}, h, nil
}

// BuildSchema reads SDL and returns the schema it describes, checking that it
// is sound.
//
// The schema has no resolvers: SDL says what a schema offers, not how the
// values are produced. A server fills them in afterwards, or builds its schema
// in Go with [schema.New].
func BuildSchema(sdl string) (*Schema, error) {
	s, err := utilities.BuildSchema(sdl)
	if err != nil {
		return nil, err
	}
	if err := schema.AssertValidSchema(s); err != nil {
		return nil, err
	}
	return s, nil
}

// Introspect asks a schema to describe itself, in the form a client receives
// when it sends an introspection query.
//
// The answer is complete: everything a schema can say about itself is asked
// for, so [utilities.BuildClientSchema] can rebuild the schema from it. Ask for
// less with [utilities.IntrospectionFromSchema], which this is shorthand for.
func Introspect(ctx context.Context, s *Schema) (*utilities.IntrospectionQueryResult, error) {
	return utilities.IntrospectionFromSchema(ctx, s, utilities.WithEverything())
}
