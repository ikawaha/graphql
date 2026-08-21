package schema

import (
	"github.com/ikawaha/graphql/value"
	"slices"

	"github.com/ikawaha/graphql/language"
)

// UnionConfig describes a union type.
//
// Types comes in two forms for the same reason fields do on an object: a
// member may be an object whose own fields point back at the union, so the
// member list cannot always be built at the moment the union is created. When
// both are given the thunk wins.
//
// A member is a [Declared] rather than an [ObjectType] because a document may
// name something else — `union U = String` parses — and a schema holds what
// was named, leaving [ValidateSchema] to say what is wrong with it. [Members]
// is how a union written in Go gives its members, and takes object types.
type UnionConfig struct {
	Name        string
	Description value.Maybe[string]

	Types      []Declared[*ObjectType]
	TypesThunk func() []Declared[*ObjectType]

	// ResolveType decides which of the members a value belongs to. The
	// alternative is an IsTypeOf on each member.
	ResolveType TypeResolver

	ASTNode           *language.UnionTypeDefinition
	ExtensionASTNodes []*language.UnionTypeExtension
	Extensions        map[string]any
}

// UnionType is a type whose value is one of a fixed set of object types.
//
// A UnionType must not be copied once it has been used, because resolving its
// members is guarded against concurrent use.
type UnionType struct {
	name        string
	description value.Maybe[string]

	types *lazy[unionMembers]

	// ResolveType decides which of the members a value belongs to.
	ResolveType TypeResolver

	ASTNode           *language.UnionTypeDefinition
	ExtensionASTNodes []*language.UnionTypeExtension
	Extensions        map[string]any
}

// unionMembers keeps the members in declaration order while allowing a lookup
// by name.
type unionMembers struct {
	ordered []Declared[*ObjectType]
	byName  map[string]*ObjectType
}

// NewUnion returns a union type.
func NewUnion(config UnionConfig) *UnionType {
	u := &UnionType{
		name:              config.Name,
		description:       config.Description,
		ResolveType:       config.ResolveType,
		ASTNode:           config.ASTNode,
		ExtensionASTNodes: config.ExtensionASTNodes,
		Extensions:        config.Extensions,
	}
	u.types = newLazy(func() unionMembers {
		members := config.Types
		if config.TypesThunk != nil {
			members = config.TypesThunk()
		}
		// A copy, so that a caller adding to their own list afterwards does
		// not change what the type stands for.
		members = slices.Clone(members)
		// Only the members that are object types can be looked up: nothing
		// else is a type a value could turn out to be.
		byName := make(map[string]*ObjectType, len(members))
		for _, declared := range members {
			m, isObject := declared.Get()
			if !isObject {
				continue
			}
			if _, exists := byName[m.name]; !exists {
				byName[m.name] = m
			}
		}
		return unionMembers{ordered: members, byName: byName}
	})
	return u
}

// Name is the name the type is declared under.
func (u *UnionType) Name() string { return u.name }

// Description is the documentation written for the type, if any.
func (u *UnionType) Description() string { return u.description.Or("") }

// DescribedAs is the documentation written for it, telling one written
// as the empty string from none at all. graphql-js keeps the two apart
// and prints and describes them differently.
func (u *UnionType) DescribedAs() value.Maybe[string] { return u.description }

// Types returns the members in the order they were declared, whatever kind
// each turned out to be. Callers must not modify the returned slice.
//
// [Schema.PossibleTypes] is the narrower question — which of them a value
// could actually turn out to be — and answers with object types alone.
func (u *UnionType) Types() []Declared[*ObjectType] { return u.types.get().ordered }

// HasType reports whether an object type is one of the union's members.
func (u *UnionType) HasType(t *ObjectType) bool {
	if t == nil {
		return false
	}
	return u.types.get().byName[t.name] == t
}

// resolveThunks builds whatever this type had deferred.
func (u *UnionType) resolveThunks() { u.types.resolve() }

// String renders the type as it is written in a schema.
func (u *UnionType) String() string {
	if u == nil {
		return "<nil>"
	}
	return u.name
}

func (*UnionType) isType()       {}
func (*UnionType) isNamedType()  {}
func (*UnionType) isOutputType() {}
func (*UnionType) isComposite()  {}
func (*UnionType) isAbstract()   {}

// IsUnionType reports whether a type is a union.
func IsUnionType(t Type) bool {
	_, ok := t.(*UnionType)
	return ok
}
