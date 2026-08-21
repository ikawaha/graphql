package schema

import (
	"github.com/ikawaha/graphql/value"
	"slices"

	"github.com/ikawaha/graphql/language"
)

// ObjectConfig describes an object type.
//
// Fields and Interfaces each come in two forms. The plain slice is what most
// types need. The thunk exists for the case a plain slice cannot express: a
// field whose type is the type being defined, or two types that refer to each
// other. A thunk is not called until every type has been created, which closes
// that loop. When both are given the thunk wins.
type ObjectConfig struct {
	Name        string
	Description value.Maybe[string]

	Fields      []*Field
	FieldsThunk func() []*Field

	Interfaces      []Declared[*InterfaceType]
	InterfacesThunk func() []Declared[*InterfaceType]

	// IsTypeOf reports whether a runtime value belongs to this type, which is
	// one of the two ways a value's type is worked out when a field is
	// declared to return an interface or a union.
	IsTypeOf IsTypeOfFn

	ASTNode           *language.ObjectTypeDefinition
	ExtensionASTNodes []*language.ObjectTypeExtension
	Extensions        map[string]any
}

// ObjectType is a type with fields, which is what most of a schema is made of.
//
// An ObjectType must not be copied once it has been used, because resolving
// its fields is guarded against concurrent use.
type ObjectType struct {
	name        string
	description value.Maybe[string]

	fields     *lazy[fieldMap]
	interfaces *lazy[[]Declared[*InterfaceType]]

	// IsTypeOf reports whether a runtime value belongs to this type.
	IsTypeOf IsTypeOfFn

	ASTNode           *language.ObjectTypeDefinition
	ExtensionASTNodes []*language.ObjectTypeExtension
	Extensions        map[string]any
}

// NewObject returns an object type.
//
// Nothing is checked here; a schema is checked as a whole once assembled. See
// [ValidateSchema].
func NewObject(config ObjectConfig) *ObjectType {
	o := &ObjectType{
		name:              config.Name,
		description:       config.Description,
		IsTypeOf:          config.IsTypeOf,
		ASTNode:           config.ASTNode,
		ExtensionASTNodes: config.ExtensionASTNodes,
		Extensions:        config.Extensions,
	}
	o.fields = newLazy(func() fieldMap {
		return newFieldMap(o, pickFields(config.Fields, config.FieldsThunk))
	})
	o.interfaces = newLazy(func() []Declared[*InterfaceType] {
		return pickInterfaces(config.Interfaces, config.InterfacesThunk)
	})
	return o
}

// pickFields chooses between the plain slice and the thunk.
func pickFields(fields []*Field, thunk func() []*Field) []*Field {
	if thunk != nil {
		return thunk()
	}
	return fields
}

// pickInterfaces chooses between the plain slice and the thunk.
//
// A copy comes back, so that a caller adding to their own list afterwards does
// not change what the type declares. The fields need no such copy: the type
// makes one of every field it holds.
func pickInterfaces(interfaces []Declared[*InterfaceType], thunk func() []Declared[*InterfaceType]) []Declared[*InterfaceType] {
	if thunk != nil {
		return slices.Clone(thunk())
	}
	return slices.Clone(interfaces)
}

// Name is the name the type is declared under.
func (o *ObjectType) Name() string { return o.name }

// Description is the documentation written for the type, if any.
func (o *ObjectType) Description() string { return o.description.Or("") }

// DescribedAs is the documentation written for it, telling one written
// as the empty string from none at all. graphql-js keeps the two apart
// and prints and describes them differently.
func (o *ObjectType) DescribedAs() value.Maybe[string] { return o.description }

// Fields returns the type's fields in the order they were declared, which is
// the order a printed schema writes them in. Callers must not modify the
// returned slice.
func (o *ObjectType) Fields() []*Field { return o.fields.get().ordered }

// Field returns the field with the given name, or nil if there is none.
func (o *ObjectType) Field(name string) *Field { return o.fields.get().byName[name] }

// Interfaces returns the interfaces the type declares it implements. Callers
// must not modify the returned slice.
func (o *ObjectType) Interfaces() []Declared[*InterfaceType] { return o.interfaces.get() }

// resolveThunks builds whatever this type had deferred, so that a schema can
// get every thunk out of the way before it serves a request.
func (o *ObjectType) resolveThunks() {
	o.fields.resolve()
	o.interfaces.resolve()
}

// String renders the type as it is written in a schema.
func (o *ObjectType) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.name
}

func (*ObjectType) isType()       {}
func (*ObjectType) isNamedType()  {}
func (*ObjectType) isOutputType() {}
func (*ObjectType) isComposite()  {}

// IsObjectType reports whether a type is an object type.
func IsObjectType(t Type) bool {
	_, ok := t.(*ObjectType)
	return ok
}
