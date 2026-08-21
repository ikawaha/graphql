package schema

import (
	"fmt"
	"slices"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/value"
)

// ArgumentConfig describes an argument of a field or a directive.
type ArgumentConfig struct {
	Description value.Maybe[string]
	// Type is the argument's type, which has to be an input type. It is
	// declared as the general Type because whether a list or a non-null counts
	// as input depends on what it wraps, which Go cannot express in a field
	// declaration; [ValidateSchema] checks it.
	Type Type
	// Default is the value used when the caller leaves the argument out.
	//
	// Leaving this unset means there is no default, which is different from a
	// default of null. Use [DefaultValue] or [DefaultLiteral] to supply one,
	// and [DefaultValue](nil) for a default that really is null.
	Default value.Maybe[DefaultInput]
	// DeprecationReason marks the argument deprecated when it is not empty.
	DeprecationReason value.Maybe[string]
	ASTNode           *language.InputValueDefinition
	Extensions        map[string]any
}

// Argument is an argument of a field or a directive.
type Argument struct {
	name        string
	description value.Maybe[string]
	// parent is the field or directive the argument belongs to, set when that
	// field or directive is built. It is what makes an argument name itself
	// the way a schema coordinate does.
	parent            fmt.Stringer
	Type              Type
	Default           value.Maybe[DefaultInput]
	DeprecationReason value.Maybe[string]
	ASTNode           *language.InputValueDefinition
	Extensions        map[string]any
}

// NewArgument returns an argument with the given name.
func NewArgument(name string, config ArgumentConfig) *Argument {
	return &Argument{
		name:              name,
		description:       config.Description,
		Type:              config.Type,
		Default:           config.Default,
		DeprecationReason: config.DeprecationReason,
		ASTNode:           config.ASTNode,
		Extensions:        config.Extensions,
	}
}

// Name is the name the argument is declared under.
func (a *Argument) Name() string { return a.name }

// Parent is the field or directive the argument belongs to, or nil for one
// that has not been given to either.
func (a *Argument) Parent() fmt.Stringer {
	if a == nil || a.parent == nil {
		return nil
	}
	return a.parent
}

// Description is the documentation written for the argument, if any.
func (a *Argument) Description() string { return a.description.Or("") }

// DescribedAs is the documentation written for it, telling one written
// as the empty string from none at all. graphql-js keeps the two apart
// and prints and describes them differently.
func (a *Argument) DescribedAs() value.Maybe[string] { return a.description }

// IsDeprecated reports whether the argument has been marked deprecated.
func (a *Argument) IsDeprecated() bool { return a.DeprecationReason.IsSet() }

// String names the argument the way a schema coordinate does, as in
// "Query.greeting(name:)". An argument that belongs to no field or directive
// yet is named on its own.
func (a *Argument) String() string {
	if a == nil {
		return "<nil>"
	}
	if a.parent == nil {
		return a.name
	}
	return a.parent.String() + "(" + a.name + ":)"
}

// IsRequiredArgument reports whether a caller has to supply the argument.
//
// An argument is required when its type is non-null and it has no default at
// all. An argument whose default is null is not required: leaving it out is
// allowed and yields null.
func IsRequiredArgument(a *Argument) bool {
	return a != nil && IsNonNullType(a.Type) && !hasDefault(a.Default)
}

// InputFieldConfig describes a field of an input object type. It is the same
// shape as an argument.
type InputFieldConfig = ArgumentConfig

// InputField is a field of an input object type.
type InputField struct {
	name        string
	description value.Maybe[string]
	// parent is the input object the field belongs to, set when that input
	// object is built.
	parent            *InputObjectType
	Type              Type
	Default           value.Maybe[DefaultInput]
	DeprecationReason value.Maybe[string]
	ASTNode           *language.InputValueDefinition
	Extensions        map[string]any
}

// NewInputField returns an input object field with the given name.
func NewInputField(name string, config InputFieldConfig) *InputField {
	return &InputField{
		name:              name,
		description:       config.Description,
		Type:              config.Type,
		Default:           config.Default,
		DeprecationReason: config.DeprecationReason,
		ASTNode:           config.ASTNode,
		Extensions:        config.Extensions,
	}
}

// Name is the name the field is declared under.
func (f *InputField) Name() string { return f.name }

// Parent is the input object the field belongs to, or nil for one that has
// not been given to an input object.
func (f *InputField) Parent() *InputObjectType { return f.parent }

// Description is the documentation written for the field, if any.
func (f *InputField) Description() string { return f.description.Or("") }

// DescribedAs is the documentation written for it, telling one written
// as the empty string from none at all. graphql-js keeps the two apart
// and prints and describes them differently.
func (f *InputField) DescribedAs() value.Maybe[string] { return f.description }

// IsDeprecated reports whether the field has been marked deprecated.
func (f *InputField) IsDeprecated() bool { return f.DeprecationReason.IsSet() }

// String names the field the way a schema coordinate does, as in
// "Point.x". A field that belongs to no input object yet is named on its own.
func (f *InputField) String() string {
	if f == nil {
		return "<nil>"
	}
	if f.parent == nil {
		return f.name
	}
	return f.parent.Name() + "." + f.name
}

// cloneArguments copies a list of arguments and points each copy at what owns
// it. A copy, because the same argument may be written into two fields and
// each has to be able to name itself; the caller's own arguments are left as
// they were, as they are for a field or an enum value.
func cloneArguments(args []*Argument, parent fmt.Stringer) []*Argument {
	if args == nil {
		return nil
	}
	kept := keepLastByName(args, func(a *Argument) string { return a.name })
	out := make([]*Argument, 0, len(kept))
	for _, held := range kept {
		if held == nil {
			out = append(out, nil)
			continue
		}
		a := new(Argument)
		*a = *held
		a.parent = parent
		out = append(out, a)
	}
	return out
}

// cloneArgumentList copies a list of arguments without saying what owns them,
// which is what a field built on its own has until a type takes it.
func cloneArgumentList(args []*Argument) []*Argument {
	if args == nil {
		return nil
	}
	return slices.Clone(keepLastByName(args, func(a *Argument) string { return a.name }))
}

// IsRequiredInputField reports whether a caller has to supply the field, by
// the same rule as [IsRequiredArgument].
func IsRequiredInputField(f *InputField) bool {
	return f != nil && IsNonNullType(f.Type) && !hasDefault(f.Default)
}
