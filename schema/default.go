package schema

import (
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/value"
)

// DefaultInput is the default value of an argument or an input object field,
// held either as a Go value or as a GraphQL literal.
//
// A schema built in code supplies a Go value; one built from SDL supplies the
// literal it was written as, which is kept rather than converted so that
// printing the schema gives back what was written.
type DefaultInput struct {
	// Value is the default as a Go value. It is used when Literal is nil.
	Value any
	// Literal is the default as it was written in a schema. It takes
	// precedence over Value.
	Literal language.Value
}

// DefaultValue returns a default holding a Go value.
//
// Passing nil makes the default an explicit null, which is a different thing
// from having no default at all; leave the field unset for that.
func DefaultValue(v any) value.Maybe[DefaultInput] {
	return value.Just(DefaultInput{Value: v})
}

// DefaultLiteral returns a default holding a GraphQL literal.
func DefaultLiteral(literal language.Value) value.Maybe[DefaultInput] {
	return value.Just(DefaultInput{Literal: literal})
}

// NoDefault returns the absence of a default value. It is the zero value, so
// leaving the field out of a struct literal means the same thing.
func NoDefault() value.Maybe[DefaultInput] {
	return value.Nothing[DefaultInput]()
}

// hasDefault reports whether a default was given at all.
//
// This is the distinction the specification rests on: an argument with no
// default and a non-null type must be supplied by the caller, while one whose
// default is null may be left out and comes through as null.
func hasDefault(d value.Maybe[DefaultInput]) bool {
	return d.IsSet()
}

// DeprecatedFor marks an element deprecated for the reason given, which may be
// empty: an element is deprecated because somebody said so, not because they
// had something to say about it.
//
// It is named for the reason rather than plainly Deprecated, which is the
// @deprecated directive itself.
//
// A GraphQL schema tells three states apart here — nothing said, deprecated
// with a reason, and deprecated with none — so this is a [value.Maybe] rather
// than a string, as a default value is.
func DeprecatedFor(reason string) value.Maybe[string] {
	return value.Just(reason)
}

// NotDeprecated returns the absence of a deprecation. It is the zero value, so
// leaving the field out says the same thing.
func NotDeprecated() value.Maybe[string] {
	return value.Nothing[string]()
}
