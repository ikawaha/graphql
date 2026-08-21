package schema

import (
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/value"
)

// FieldConfig describes a field of an object or interface type.
type FieldConfig struct {
	Description value.Maybe[string]
	// Type is the field's type, which has to be an output type. As with an
	// argument, whether a wrapper qualifies depends on what it wraps, so
	// [ValidateSchema] checks it rather than the type system.
	Type Type
	// Args are the field's arguments, in the order they should be written.
	Args []*Argument
	// Resolve produces the field's value. A field with no resolver falls back
	// to the schema's default.
	Resolve FieldResolver
	// Subscribe produces the event stream for a field of the subscription
	// type. It is ignored elsewhere.
	Subscribe FieldResolver
	// DeprecationReason marks the field deprecated when it is not empty.
	DeprecationReason value.Maybe[string]
	ASTNode           *language.FieldDefinition
	Extensions        map[string]any
}

// Field is a field of an object or interface type.
type Field struct {
	name              string
	description       value.Maybe[string]
	Type              Type
	Args              []*Argument
	Resolve           FieldResolver
	Subscribe         FieldResolver
	DeprecationReason value.Maybe[string]
	ASTNode           *language.FieldDefinition
	Extensions        map[string]any

	parent NamedType
}

// NewField returns a field with the given name.
func NewField(name string, config FieldConfig) *Field {
	return &Field{
		name:              name,
		description:       config.Description,
		Type:              config.Type,
		Args:              cloneArgumentList(config.Args),
		Resolve:           config.Resolve,
		Subscribe:         config.Subscribe,
		DeprecationReason: config.DeprecationReason,
		ASTNode:           config.ASTNode,
		Extensions:        config.Extensions,
	}
}

// Name is the name the field is declared under.
func (f *Field) Name() string { return f.name }

// Description is the documentation written for the field, if any.
func (f *Field) Description() string { return f.description.Or("") }

// DescribedAs is the documentation written for it, telling one written
// as the empty string from none at all. graphql-js keeps the two apart
// and prints and describes them differently.
func (f *Field) DescribedAs() value.Maybe[string] { return f.description }

// IsDeprecated reports whether the field has been marked deprecated.
func (f *Field) IsDeprecated() bool { return f.DeprecationReason.IsSet() }

// Parent is the type the field belongs to, or nil for a field that has not
// been attached to one.
func (f *Field) Parent() NamedType { return f.parent }

// Arg returns the argument with the given name, or nil if the field has none.
func (f *Field) Arg(name string) *Argument {
	if f == nil {
		return nil
	}
	for _, arg := range f.Args {
		if arg.name == name {
			return arg
		}
	}
	return nil
}

// String renders the field as "Type.field", which is how a schema coordinate
// names it.
func (f *Field) String() string {
	if f == nil {
		return "<nil>"
	}
	if f.parent == nil {
		// An introspection meta-field belongs to no type in particular, and
		// graphql-js names it this way rather than leaving the owner blank.
		return "<meta>." + f.name
	}
	return f.parent.Name() + "." + f.name
}

// fieldMap indexes fields by name while keeping the order they were declared
// in, because printing a schema has to give back the order it was written in.
type fieldMap struct {
	ordered []*Field
	byName  map[string]*Field
}

// newFieldMap indexes fields and records which type they belong to.
//
// A field is copied rather than taken as it stands, because a type records
// itself on the fields it holds: the same field put into two types would
// otherwise end up belonging to whichever was built second, and would say so
// in every message naming it. graphql-js builds a field of its own for each
// type for the same reason.
//
// What a copy holds is shared — the resolver, the type, the arguments — so
// reaching a field through the type it belongs to and setting its resolver,
// which is how a schema read from SDL is wired up, works as it did.
func newFieldMap(parent NamedType, fields []*Field) fieldMap {
	m := fieldMap{
		ordered: make([]*Field, 0, len(fields)),
		byName:  make(map[string]*Field, len(fields)),
	}
	for _, held := range keepLastByName(fields, func(f *Field) string { return f.name }) {
		if held == nil {
			m.ordered = append(m.ordered, nil)
			continue
		}
		f := new(Field)
		*f = *held
		f.parent = parent
		f.Args = cloneArguments(held.Args, f)
		m.ordered = append(m.ordered, f)
		m.byName[f.name] = f
	}
	return m
}
