package schema

import (
	"reflect"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/value"
)

// EnumValueConfig describes one member of an enum type.
type EnumValueConfig struct {
	Description value.Maybe[string]
	// Value is what a resolver sees and returns for this member. Leaving it
	// unset makes the member's own name its value, which is what a schema
	// built from SDL gets.
	//
	// It is a [value.Maybe] rather than a plain any so that a member whose
	// value really is null can say so: nil on its own would be indefinite
	// between "null" and "nothing said", and graphql-js tells the two apart.
	// [InternalValue] is the shorthand for setting it.
	Value value.Maybe[any]
	// DeprecationReason marks the member deprecated when it is not empty.
	DeprecationReason value.Maybe[string]
	ASTNode           *language.EnumValueDefinition
	Extensions        map[string]any
}

// EnumValue is one member of an enum type.
type EnumValue struct {
	name        string
	description value.Maybe[string]
	// Value is the member's internal value, which is what crosses into and out
	// of resolvers. Documents only ever see the name.
	Value             any
	DeprecationReason value.Maybe[string]
	ASTNode           *language.EnumValueDefinition
	Extensions        map[string]any

	parent *EnumType
}

// NewEnumValue returns an enum member with the given name.
func NewEnumValue(name string, config EnumValueConfig) *EnumValue {
	return &EnumValue{
		name:        name,
		description: config.Description,
		// A member that says nothing about its value is its own name.
		Value:             config.Value.Or(name),
		DeprecationReason: config.DeprecationReason,
		ASTNode:           config.ASTNode,
		Extensions:        config.Extensions,
	}
}

// InternalValue is what a member's value is, said in the form
// [EnumValueConfig.Value] takes. Use it to give a member a value of its own,
// including one that is null; leave the field unset for a member whose value
// is its name.
func InternalValue(v any) value.Maybe[any] { return value.Just(v) }

// Name is the name the member is declared under, which is how a document
// refers to it.
func (v *EnumValue) Name() string { return v.name }

// Description is the documentation written for the member, if any.
func (v *EnumValue) Description() string { return v.description.Or("") }

// DescribedAs is the documentation written for it, telling one written
// as the empty string from none at all. graphql-js keeps the two apart
// and prints and describes them differently.
func (v *EnumValue) DescribedAs() value.Maybe[string] { return v.description }

// IsDeprecated reports whether the member has been marked deprecated.
func (v *EnumValue) IsDeprecated() bool { return v.DeprecationReason.IsSet() }

// Parent is the enum type the member belongs to, or nil if it has not been
// attached to one.
func (v *EnumValue) Parent() *EnumType { return v.parent }

// String renders the member as "Enum.MEMBER".
func (v *EnumValue) String() string {
	if v == nil {
		return "<nil>"
	}
	if v.parent == nil {
		return v.name
	}
	return v.parent.Name() + "." + v.name
}

// EnumConfig describes an enum type.
type EnumConfig struct {
	Name        string
	Description value.Maybe[string]
	// Values are the type's members, in the order they should be written.
	Values            []*EnumValue
	ASTNode           *language.EnumTypeDefinition
	ExtensionASTNodes []*language.EnumTypeExtension
	Extensions        map[string]any
}

// EnumType is a leaf type whose values are one of a fixed set of members.
type EnumType struct {
	name        string
	description value.Maybe[string]

	values       []*EnumValue
	byName       map[string]*EnumValue
	byValue      map[any]*EnumValue
	uncomparable []*EnumValue

	ASTNode           *language.EnumTypeDefinition
	ExtensionASTNodes []*language.EnumTypeExtension
	Extensions        map[string]any
}

// NewEnum returns an enum type.
//
// Members do not refer to other types, so there is nothing here to defer and
// no thunk to supply.
func NewEnum(config EnumConfig) *EnumType {
	e := &EnumType{
		name:              config.Name,
		description:       config.Description,
		values:            make([]*EnumValue, 0, len(config.Values)),
		byName:            make(map[string]*EnumValue, len(config.Values)),
		byValue:           make(map[any]*EnumValue, len(config.Values)),
		ASTNode:           config.ASTNode,
		ExtensionASTNodes: config.ExtensionASTNodes,
		Extensions:        config.Extensions,
	}
	// A member is copied rather than taken as it stands, for the reason
	// [newFieldMap] gives: a member records which type it belongs to, and the
	// same member put into two types would otherwise belong to the second.
	for _, held := range keepLastByName(config.Values, func(v *EnumValue) string { return v.name }) {
		if held == nil {
			e.values = append(e.values, nil)
			continue
		}
		v := new(EnumValue)
		*v = *held
		v.parent = e
		e.values = append(e.values, v)
		e.byName[v.name] = v
		// A Go map key has to be comparable, and an internal value is whatever
		// the caller chose, so anything else is kept aside and found by
		// scanning. That is rare enough not to matter and beats panicking.
		if isComparableValue(v.Value) {
			if _, exists := e.byValue[v.Value]; !exists {
				e.byValue[v.Value] = v
			}
		} else {
			e.uncomparable = append(e.uncomparable, v)
		}
	}
	return e
}

// isComparableValue reports whether a value may be used as a map key.
func isComparableValue(v any) bool {
	if v == nil {
		return false
	}
	return reflect.TypeOf(v).Comparable()
}

// Name is the name the type is declared under.
func (e *EnumType) Name() string { return e.name }

// Description is the documentation written for the type, if any.
func (e *EnumType) Description() string { return e.description.Or("") }

// DescribedAs is the documentation written for it, telling one written
// as the empty string from none at all. graphql-js keeps the two apart
// and prints and describes them differently.
func (e *EnumType) DescribedAs() value.Maybe[string] { return e.description }

// Values returns the members in the order they were declared. Callers must not
// modify the returned slice.
func (e *EnumType) Values() []*EnumValue { return e.values }

// Value returns the member with the given name, or nil if there is none. This
// is the lookup a document's enum literal goes through.
func (e *EnumType) Value(name string) *EnumValue { return e.byName[name] }

// ValueFor returns the member whose internal value equals v, or nil if there
// is none. This is the lookup a resolver's return value goes through on its
// way into a response.
func (e *EnumType) ValueFor(v any) *EnumValue {
	if isComparableValue(v) {
		if found, ok := e.byValue[v]; ok {
			return found
		}
	}
	for _, candidate := range e.uncomparable {
		if reflect.DeepEqual(candidate.Value, v) {
			return candidate
		}
	}
	// A member whose value is a number is found by what the number is rather
	// than by which Go type holds it: a member declared with an int and a
	// resolver answering with an int32 mean the same member.
	if sought, isNumber := asNumber(v); isNumber {
		for _, candidate := range e.Values() {
			if candidate == nil {
				continue
			}
			if held, isNumber := asNumber(candidate.Value); isNumber && sameNumber(held, sought) {
				return candidate
			}
		}
	}
	return nil
}

// sameNumber reports whether two numbers are the same number, whatever Go type
// each arrived in.
func sameNumber(a, b number) bool {
	if a.isInteger && b.isInteger && a.exact == "" && b.exact == "" {
		return a.i == b.i
	}
	if a.exact != "" || b.exact != "" {
		return a.String() == b.String()
	}
	return a.float() == b.float()
}

// String renders the type as it is written in a schema.
func (e *EnumType) String() string {
	if e == nil {
		return "<nil>"
	}
	return e.name
}

func (*EnumType) isType()       {}
func (*EnumType) isNamedType()  {}
func (*EnumType) isInputType()  {}
func (*EnumType) isOutputType() {}
func (*EnumType) isLeafType()   {}

// IsEnumType reports whether a type is an enum.
func IsEnumType(t Type) bool {
	_, ok := t.(*EnumType)
	return ok
}
