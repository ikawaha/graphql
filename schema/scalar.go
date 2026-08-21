package schema

import (
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/value"
)

// OutputValueCoercer turns an internal value into the form that goes into a
// response.
//
// Answering with nothing says the value cannot be represented, which is what
// graphql-js's returning undefined says; answering with nil says it came to
// null, which a response cannot carry either. Both are reported, and the
// complaint says which it was.
type OutputValueCoercer func(internal any) (value.Maybe[any], error)

// InputValueCoercer turns a value supplied by a caller, such as a variable,
// into the internal form a resolver receives.
//
// There are three answers, as there are in graphql-js. Nothing says the value
// does not fit, and the complaint names the type and the value. A value —
// including nil, which is GraphQL null — is what the caller meant. An error
// says the value does not fit and why, in the coercer's own words, which is
// usually more use than the complaint made for nothing.
type InputValueCoercer func(external any) (value.Maybe[any], error)

// InputLiteralCoercer turns a literal written in a document into the internal
// form.
//
// The literal names no variables: [ReplaceVariables] has already put what the
// request supplied in their place, read against the type each was declared as,
// and left out any input object field whose variable the request omitted. A
// coercer therefore sees a constant, which is what graphql-js hands its own.
type InputLiteralCoercer func(literal language.Value) (value.Maybe[any], error)

// ValueToLiteral turns an internal value back into a literal, which is how a
// default value supplied in code gets printed in a schema.
type ValueToLiteral func(internal any, t Type) (language.Value, error)

// ScalarConfig describes a scalar type.
type ScalarConfig struct {
	Name        string
	Description value.Maybe[string]
	// SpecifiedByURL points at the specification a custom scalar follows.
	SpecifiedByURL string

	// CoerceOutputValue prepares a value for a response. It defaults to
	// passing the value through unchanged.
	CoerceOutputValue OutputValueCoercer
	// CoerceInputValue accepts a value from a caller. It defaults to passing
	// the value through unchanged.
	CoerceInputValue InputValueCoercer
	// CoerceInputLiteral accepts a literal from a document. When it is nil the
	// literal is first turned into a plain Go value and then passed to
	// CoerceInputValue, which is right for most scalars.
	CoerceInputLiteral InputLiteralCoercer
	// ValueToLiteral renders an internal value as a literal. When it is nil a
	// generic conversion is used.
	ValueToLiteral ValueToLiteral

	ASTNode           *language.ScalarTypeDefinition
	ExtensionASTNodes []*language.ScalarTypeExtension
	Extensions        map[string]any
}

// ScalarType is a leaf type whose values are converted to and from a single
// primitive, such as Int or String.
type ScalarType struct {
	name        string
	description value.Maybe[string]
	// SpecifiedByURL points at the specification a custom scalar follows.
	SpecifiedByURL string

	// CoerceOutputValue prepares a value for a response.
	CoerceOutputValue OutputValueCoercer
	// CoerceInputValue accepts a value from a caller.
	CoerceInputValue InputValueCoercer
	// CoerceInputLiteral accepts a literal from a document, or is nil when the
	// scalar has no special handling for literals.
	CoerceInputLiteral InputLiteralCoercer
	// ValueToLiteral renders an internal value as a literal, or is nil.
	ValueToLiteral ValueToLiteral

	ASTNode           *language.ScalarTypeDefinition
	ExtensionASTNodes []*language.ScalarTypeExtension
	Extensions        map[string]any
}

// NewScalar returns a scalar type.
//
// It does not check the name, because a schema is checked as a whole once it
// has been assembled; see [ValidateSchema].
func NewScalar(config ScalarConfig) *ScalarType {
	s := &ScalarType{
		name:               config.Name,
		description:        config.Description,
		SpecifiedByURL:     config.SpecifiedByURL,
		CoerceOutputValue:  config.CoerceOutputValue,
		CoerceInputValue:   config.CoerceInputValue,
		CoerceInputLiteral: config.CoerceInputLiteral,
		ValueToLiteral:     config.ValueToLiteral,
		ASTNode:            config.ASTNode,
		ExtensionASTNodes:  config.ExtensionASTNodes,
		Extensions:         config.Extensions,
	}
	if s.CoerceOutputValue == nil {
		s.CoerceOutputValue = identityCoercer
	}
	if s.CoerceInputValue == nil {
		s.CoerceInputValue = identityCoercer
	}
	return s
}

// identityCoercer passes a value through unchanged, which is what a scalar
// does when it has nothing to convert.
func identityCoercer(v any) (value.Maybe[any], error) { return value.Just(v), nil }

// Name is the name the type is declared under.
func (s *ScalarType) Name() string { return s.name }

// Description is the documentation written for the type, if any.
func (s *ScalarType) Description() string { return s.description.Or("") }

// DescribedAs is the documentation written for it, telling one written
// as the empty string from none at all. graphql-js keeps the two apart
// and prints and describes them differently.
func (s *ScalarType) DescribedAs() value.Maybe[string] { return s.description }

// String renders the type as it is written in a schema.
func (s *ScalarType) String() string {
	if s == nil {
		return "<nil>"
	}
	return s.name
}

func (*ScalarType) isType()       {}
func (*ScalarType) isNamedType()  {}
func (*ScalarType) isInputType()  {}
func (*ScalarType) isOutputType() {}
func (*ScalarType) isLeafType()   {}

// IsScalarType reports whether a type is a scalar.
func IsScalarType(t Type) bool {
	_, ok := t.(*ScalarType)
	return ok
}
