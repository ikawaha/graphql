package schema

import (
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/value"
)

// InterfaceConfig describes an interface type.
//
// Fields and Interfaces come in the same two forms as on an object type, and
// for the same reason; see [ObjectConfig].
type InterfaceConfig struct {
	Name        string
	Description value.Maybe[string]

	Fields      []*Field
	FieldsThunk func() []*Field

	Interfaces      []Declared[*InterfaceType]
	InterfacesThunk func() []Declared[*InterfaceType]

	// ResolveType decides which object type a value belongs to. It is the
	// other way of answering that question, the alternative being an IsTypeOf
	// on each candidate object type.
	ResolveType TypeResolver

	ASTNode           *language.InterfaceTypeDefinition
	ExtensionASTNodes []*language.InterfaceTypeExtension
	Extensions        map[string]any
}

// InterfaceType is a set of fields that several object types can share.
//
// An InterfaceType must not be copied once it has been used, because resolving
// its fields is guarded against concurrent use.
type InterfaceType struct {
	name        string
	description value.Maybe[string]

	fields     *lazy[fieldMap]
	interfaces *lazy[[]Declared[*InterfaceType]]

	// ResolveType decides which object type a value belongs to.
	ResolveType TypeResolver

	ASTNode           *language.InterfaceTypeDefinition
	ExtensionASTNodes []*language.InterfaceTypeExtension
	Extensions        map[string]any
}

// NewInterface returns an interface type.
func NewInterface(config InterfaceConfig) *InterfaceType {
	i := &InterfaceType{
		name:              config.Name,
		description:       config.Description,
		ResolveType:       config.ResolveType,
		ASTNode:           config.ASTNode,
		ExtensionASTNodes: config.ExtensionASTNodes,
		Extensions:        config.Extensions,
	}
	i.fields = newLazy(func() fieldMap {
		return newFieldMap(i, pickFields(config.Fields, config.FieldsThunk))
	})
	i.interfaces = newLazy(func() []Declared[*InterfaceType] {
		return pickInterfaces(config.Interfaces, config.InterfacesThunk)
	})
	return i
}

// Name is the name the type is declared under.
func (i *InterfaceType) Name() string { return i.name }

// Description is the documentation written for the type, if any.
func (i *InterfaceType) Description() string { return i.description.Or("") }

// DescribedAs is the documentation written for it, telling one written
// as the empty string from none at all. graphql-js keeps the two apart
// and prints and describes them differently.
func (i *InterfaceType) DescribedAs() value.Maybe[string] { return i.description }

// Fields returns the type's fields in the order they were declared. Callers
// must not modify the returned slice.
func (i *InterfaceType) Fields() []*Field { return i.fields.get().ordered }

// Field returns the field with the given name, or nil if there is none.
func (i *InterfaceType) Field(name string) *Field { return i.fields.get().byName[name] }

// Interfaces returns the interfaces this one declares it implements. Callers
// must not modify the returned slice.
func (i *InterfaceType) Interfaces() []Declared[*InterfaceType] { return i.interfaces.get() }

// resolveThunks builds whatever this type had deferred.
func (i *InterfaceType) resolveThunks() {
	i.fields.resolve()
	i.interfaces.resolve()
}

// String renders the type as it is written in a schema.
func (i *InterfaceType) String() string {
	if i == nil {
		return "<nil>"
	}
	return i.name
}

func (*InterfaceType) isType()       {}
func (*InterfaceType) isNamedType()  {}
func (*InterfaceType) isOutputType() {}
func (*InterfaceType) isComposite()  {}
func (*InterfaceType) isAbstract()   {}

// IsInterfaceType reports whether a type is an interface type.
func IsInterfaceType(t Type) bool {
	_, ok := t.(*InterfaceType)
	return ok
}

// CompositeType is a type that has a selection set written against it: an
// object, an interface or a union.
type CompositeType interface {
	NamedType
	isComposite()
}

// IsCompositeType reports whether a selection set may be written against a
// type.
func IsCompositeType(t Type) bool {
	_, ok := t.(CompositeType)
	return ok
}

// AbstractType is a type whose runtime type has to be worked out during
// execution: an interface or a union.
type AbstractType interface {
	CompositeType
	isAbstract()
}

// IsAbstractType reports whether a type's runtime type has to be resolved.
func IsAbstractType(t Type) bool {
	_, ok := t.(AbstractType)
	return ok
}
