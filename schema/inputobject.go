package schema

import (
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/value"
)

// InputObjectConfig describes an input object type.
//
// Fields comes in two forms for the same reason it does on an object type: an
// input object may hold a field of its own type, so the field list cannot
// always be built when the type is created. When both are given the thunk
// wins.
type InputObjectConfig struct {
	Name        string
	Description value.Maybe[string]

	Fields      []*InputField
	FieldsThunk func() []*InputField

	// IsOneOf marks the type as one where exactly one field must be supplied,
	// which is how a schema spells a tagged union of inputs.
	IsOneOf bool

	ASTNode           *language.InputObjectTypeDefinition
	ExtensionASTNodes []*language.InputObjectTypeExtension
	Extensions        map[string]any
}

// InputObjectType is a type used for structured input: the type of an
// argument or a variable, never of a field.
//
// An InputObjectType must not be copied once it has been used, because
// resolving its fields is guarded against concurrent use.
type InputObjectType struct {
	name        string
	description value.Maybe[string]

	fields *lazy[inputFieldMap]

	// IsOneOf reports whether exactly one field must be supplied.
	IsOneOf bool

	ASTNode           *language.InputObjectTypeDefinition
	ExtensionASTNodes []*language.InputObjectTypeExtension
	Extensions        map[string]any
}

// inputFieldMap keeps fields in declaration order while allowing a lookup by
// name.
type inputFieldMap struct {
	ordered []*InputField
	byName  map[string]*InputField
}

// NewInputObject returns an input object type.
func NewInputObject(config InputObjectConfig) *InputObjectType {
	o := &InputObjectType{
		name:              config.Name,
		description:       config.Description,
		IsOneOf:           config.IsOneOf,
		ASTNode:           config.ASTNode,
		ExtensionASTNodes: config.ExtensionASTNodes,
		Extensions:        config.Extensions,
	}
	o.fields = newLazy(func() inputFieldMap {
		fields := config.Fields
		if config.FieldsThunk != nil {
			fields = config.FieldsThunk()
		}
		// A copy of each field, so that a caller reusing one in two input
		// objects gets two fields that each know where they belong, and so
		// that adding to their own list afterwards does not change what the
		// type asks for.
		ordered := make([]*InputField, 0, len(fields))
		byName := make(map[string]*InputField, len(fields))
		for _, held := range keepLastByName(fields, func(f *InputField) string { return f.name }) {
			if held == nil {
				ordered = append(ordered, nil)
				continue
			}
			f := new(InputField)
			*f = *held
			f.parent = o
			ordered = append(ordered, f)
			byName[f.name] = f
		}
		return inputFieldMap{ordered: ordered, byName: byName}
	})
	return o
}

// Name is the name the type is declared under.
func (o *InputObjectType) Name() string { return o.name }

// Description is the documentation written for the type, if any.
func (o *InputObjectType) Description() string { return o.description.Or("") }

// DescribedAs is the documentation written for it, telling one written
// as the empty string from none at all. graphql-js keeps the two apart
// and prints and describes them differently.
func (o *InputObjectType) DescribedAs() value.Maybe[string] { return o.description }

// Fields returns the type's fields in the order they were declared. Callers
// must not modify the returned slice.
func (o *InputObjectType) Fields() []*InputField { return o.fields.get().ordered }

// Field returns the field with the given name, or nil if there is none.
func (o *InputObjectType) Field(name string) *InputField { return o.fields.get().byName[name] }

// resolveThunks builds whatever this type had deferred.
func (o *InputObjectType) resolveThunks() { o.fields.resolve() }

// String renders the type as it is written in a schema.
func (o *InputObjectType) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.name
}

func (*InputObjectType) isType()      {}
func (*InputObjectType) isNamedType() {}
func (*InputObjectType) isInputType() {}

// IsInputObjectType reports whether a type is an input object.
func IsInputObjectType(t Type) bool {
	_, ok := t.(*InputObjectType)
	return ok
}
