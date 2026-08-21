// Package schema describes what a server can answer.
//
// A schema is the types, the fields those types have, and the arguments those
// fields take. It is what validation checks a document against and what
// execution walks while answering one, and it is where a server says how each
// field's value is found.
//
// # Building one
//
// A schema can be written as SDL and read with the utilities package, or
// built here type by type — [NewObject], [NewInterface], [NewUnion],
// [NewEnum], [NewInputObject], [NewScalar] — and assembled with [New]. The two
// meet in the middle: a schema read from SDL has no resolvers, and a server
// puts them on afterwards by reaching for the field and setting
// [Field.Resolve].
//
// Building never fails. A type takes a configuration and answers with a type,
// so that describing a schema reads as a description rather than a sequence of
// error handling. The price is that nothing is verified until [ValidateSchema]
// is called, which is where a schema that cannot work says so.
//
// A type takes what the configuration holds rather than borrowing it: the
// lists are copied, and so are the fields and enum members, which record which
// type they belong to. The same field may therefore be put into two types
// without the second claiming it from the first, and adding to a list after
// the type is built changes nothing. What a copy holds is shared — the
// resolver, the type, the arguments — so reaching a field through the type and
// setting its resolver, which is how a schema read from SDL is wired up, works
// as it reads.
//
// A type may refer to itself, or to a type declared after it, which Go cannot
// express in a struct literal. Every configuration therefore has a thunk
// alongside the list — [ObjectConfig.FieldsThunk] beside
// [ObjectConfig.Fields] — evaluated once, the first time the list is asked
// for.
//
// # Three states, not two
//
// An argument the caller left out, one given as null, and one given a value
// are three different things. [Arguments.Get] says whether the argument was
// supplied at all, and a default is held as a [DefaultInput] that may be
// absent, may be null, or may be a value; [DefaultValue], [DefaultLiteral] and
// [NoDefault] are the three ways to say which.
//
// # Values in and out
//
// A value arriving from a caller is turned into the form a resolver receives
// by [CoerceInputValue], and one written in a document by
// [CoerceInputLiteral]. Both answer with whether the value fits rather than
// with why it does not: [ValidateInputValue] and [ValidateInputLiteral] walk
// the same ground more slowly to say what is wrong, and are worth running only
// once the fast answer has come back no. Going the other way,
// [LiteralFromValue] renders an internal value as a literal, which is how a
// default given in code is printed in a schema.
//
// # Asking about types
//
// The predicates — [IsObjectType], [IsInputType], [IsWrappingType] and the
// rest — answer what a type is where Go's own type assertion would be clumsy,
// such as through the [Type] interface or after unwrapping. [NamedTypeOf] and
// [NullableTypeOf] do the unwrapping. [IsTypeSubTypeOf] and [DoTypesOverlap]
// answer how two types stand to each other, which is what validation asks when
// it decides whether a fragment may apply.
package schema
