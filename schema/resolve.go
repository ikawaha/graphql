package schema

import (
	"context"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/value"
)

// Arguments are the arguments a field was called with.
//
// An argument the caller left out is absent from the map, while one given as
// null is present and holds nil. Reach for Get rather than indexing, so that
// the two stay apart.
type Arguments struct {
	values map[string]any
}

// NewArguments wraps a map of coerced argument values.
func NewArguments(values map[string]any) Arguments {
	return Arguments{values: values}
}

// Get returns an argument's value and whether it was supplied at all. A false
// ok means the argument was omitted; a nil value with ok true means it was
// given as null.
func (a Arguments) Get(name string) (any, bool) {
	v, ok := a.values[name]
	return v, ok
}

// Has reports whether an argument was supplied, including as null.
func (a Arguments) Has(name string) bool {
	_, ok := a.values[name]
	return ok
}

// Len returns how many arguments were supplied.
func (a Arguments) Len() int { return len(a.values) }

// Raw returns the underlying map. Callers must not modify it: a field's
// arguments are worked out once and the same map is handed to every call for
// that field, once per object it is resolved against.
func (a Arguments) Raw() map[string]any { return a.values }

// ResolveInfo tells a resolver where in the request it is being called.
//
// It gains more members as the executor is built; what is here now is what the
// type system itself can describe. The introspection resolvers read Schema
// from it, which is how a document asking __schema reaches the schema it is
// being answered against.
type ResolveInfo struct {
	// Schema is the schema the request is being answered against.
	Schema *Schema
	// FieldName is the name of the field being resolved.
	FieldName string
	// FieldNodes are the selections in the document that asked for this field.
	// There is more than one when the same field was requested several times
	// and the results are merged.
	//
	// The same slice is handed to every call for the field, once per object it
	// is resolved against, so a resolver must not modify it.
	FieldNodes []*language.Field
	// ReturnType is the declared type of the field.
	ReturnType Type
	// ParentType is the object type the field belongs to.
	ParentType *ObjectType
	// Path is where in the response this field's value goes.
	Path *value.Path
	// RootValue is the value the request was started with.
	RootValue any
	// Operation is the operation being executed.
	Operation *language.OperationDefinition
	// Fragments are the fragments the document defined, by name.
	Fragments map[string]*language.FragmentDefinition
	// VariableValues are the coerced variables of the request. A variable the
	// caller omitted is absent from the map.
	VariableValues map[string]any
}

// FieldResolver produces the value of a field.
//
// source is the value the parent field resolved to, and args holds the
// arguments after coercion. Returning an error makes the field fail, which the
// executor turns into an entry in the response's errors.
type FieldResolver func(ctx context.Context, source any, args Arguments, info *ResolveInfo) (any, error)

// TypeResolver decides which object type a value belongs to, for a field whose
// declared type is an interface or a union.
//
// It returns the name of the type. Returning an empty name means the type
// could not be determined, which fails the field.
type TypeResolver func(ctx context.Context, v any, info *ResolveInfo) (string, error)

// IsTypeOfFn reports whether a value belongs to a particular object type. It
// is an alternative to a [TypeResolver]: each candidate type answers for
// itself.
type IsTypeOfFn func(ctx context.Context, v any, info *ResolveInfo) (bool, error)
