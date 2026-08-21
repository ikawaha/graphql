package gqlerror

import (
	"errors"

	"github.com/ikawaha/graphql/language"
)

// FromSyntaxError turns a parse failure into a GraphQL error.
//
// The language package returns its own error type, because it sits below this
// one and Go does not allow two packages to import each other. This is the
// bridge between the two.
func FromSyntaxError(err *language.SyntaxError) *Error {
	if err == nil {
		return nil
	}
	return New(err.Error(),
		WithSource(err.Source),
		WithPositions(err.Position),
		WithCause(err))
}

// Ensure returns err as a GraphQL error, wrapping it if it is not one already.
//
// A syntax error is converted so that it keeps its source and position; any
// other error becomes a GraphQL error carrying its message, with the original
// reachable through errors.Unwrap.
func Ensure(err error) *Error {
	if err == nil {
		return nil
	}
	var gqlErr *Error
	if errors.As(err, &gqlErr) {
		return gqlErr
	}
	var syntaxErr *language.SyntaxError
	if errors.As(err, &syntaxErr) {
		return FromSyntaxError(syntaxErr)
	}
	return New(err.Error(), WithCause(err))
}

// Located returns err as a GraphQL error that knows which part of the document
// and which response path it came from.
//
// It is meant for errors raised while a field is being resolved: the resolver
// reports what went wrong, and the executor says where. An error that already
// carries a path is returned unchanged, so a path set deeper in the tree is
// not overwritten on the way out, and nodes the error already blamed are kept
// in preference to the ones passed in.
func Located(err error, nodes []language.Node, path []any) *Error {
	if err == nil {
		return nil
	}
	located := Ensure(err)
	if len(located.Path) > 0 {
		return located
	}

	blamed := located.Nodes
	if len(blamed) == 0 {
		blamed = nodes
	}
	opts := []Option{WithNodes(blamed...), WithPath(path...), WithCause(err)}
	if located.Source != nil {
		opts = append(opts, WithSource(located.Source), WithPositions(located.Positions...))
	}
	if located.Extensions != nil {
		opts = append(opts, WithExtensions(located.Extensions))
	}
	return New(located.Message, opts...)
}
