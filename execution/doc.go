// Package execution answers a validated document against a schema.
//
// [Execute] takes a [Request] and returns a [Result]. The request carries the
// schema, the parsed document, which operation to run, and the variables the
// caller supplied; the result carries the response and whatever went wrong.
//
// # What a resolver receives
//
// A field's value comes from its resolver, which the schema holds:
//
//	func(ctx context.Context, source any, args schema.Arguments, info *schema.ResolveInfo) (any, error)
//
// source is what the enclosing field resolved to, args holds the arguments
// after coercion, and info says where in the request the call is. Anything a
// resolver needs to know about the caller — who they are, what they may see, a
// database handle — travels in ctx, which is where a Go program already looks
// for it; there is deliberately no second channel for it.
//
// A field with no resolver falls back to [DefaultResolver], which reads the
// source the way graphql-js reads a property: see its documentation for what
// stands in for that in Go.
//
// # Three states, not two
//
// An argument or variable the caller left out, one given as null, and one
// given a value are three different things, and they stay apart the whole way
// through. A resolver tells them apart with [schema.Arguments.Get], which
// returns whether the argument was supplied at all; the caller supplies
// variables as [value.Maybe] values for the same reason. An argument left out
// falls back to its default and one given as null does not, so collapsing the
// two would silently change what a request means.
//
// The same distinction appears in the response: [Result.Data] is absent when
// the request failed before it could begin, and present but null when a field
// that may not be null failed.
//
// # Errors
//
// A field that fails is null and its failure is reported alongside the data,
// with the path to where it happened, so a client can tell which part of the
// response is missing and why. Where the field may not be null there is
// nowhere to put the null, and the failure moves outwards until it reaches
// somewhere that can hold one — possibly the whole response.
//
// A panic in a resolver is caught and reported as that field failing. It is a
// fault in code the server author wrote, and left alone it would take down
// every request the process is serving rather than the one field it belongs
// to; the stack is kept on the [PanicError] so the cause can still be found.
//
// # Order and concurrency
//
// The keys of an object in the response follow the document, as the
// specification asks, which is why the response is built from
// [value.OrderedMap] rather than a Go map.
//
// Resolvers run one after another by default. Setting [Request.Concurrency]
// above one lets the fields of an object, and the entries of a list, be worked
// on alongside each other, which is worth a great deal when resolvers wait on
// a database, but it means resolvers must be safe to call from several
// goroutines at once — a promise this package cannot make on their behalf, so
// it is not the default. The response does not change either way. A mutation's root fields always run in order, because
// each is expected to see what the one before it did.
//
// # Validation comes first
//
// Execution assumes its document has passed the validation package. It does
// not check that a field exists, that an argument is of the right type, or
// that a fragment can apply, because a rule there has already said so. Running
// an unvalidated document may produce a response that makes no sense rather
// than an error explaining why.
package execution
