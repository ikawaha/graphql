package schema

import (
	"github.com/ikawaha/graphql/value"
	"slices"
)

// The ToConfig methods give back what a type was built from, so that a type
// can be derived from one that already exists: take the configuration, change
// what should differ, and build.
//
// What comes back is settled rather than deferred — a thunk has been called
// and the list it produced is what the configuration holds — because the type
// is finished and there is nothing left to wait for.
//
// The lists are copies, so adding to one changes nothing. What they hold is
// shared, as it is when a type is built: a field in the configuration is the
// field the type holds, so changing one means putting a different field in the
// list rather than reaching into the one that is there.

// ToConfig returns what the scalar was built from.
func (s *ScalarType) ToConfig() ScalarConfig {
	return ScalarConfig{
		Name:               s.name,
		Description:        s.description,
		SpecifiedByURL:     s.SpecifiedByURL,
		CoerceOutputValue:  s.CoerceOutputValue,
		CoerceInputValue:   s.CoerceInputValue,
		CoerceInputLiteral: s.CoerceInputLiteral,
		ValueToLiteral:     s.ValueToLiteral,
		ASTNode:            s.ASTNode,
		ExtensionASTNodes:  slices.Clone(s.ExtensionASTNodes),
		Extensions:         s.Extensions,
	}
}

// ToConfig returns what the object type was built from.
func (o *ObjectType) ToConfig() ObjectConfig {
	return ObjectConfig{
		Name:              o.name,
		Description:       o.description,
		Fields:            slices.Clone(o.Fields()),
		Interfaces:        slices.Clone(o.Interfaces()),
		IsTypeOf:          o.IsTypeOf,
		ASTNode:           o.ASTNode,
		ExtensionASTNodes: slices.Clone(o.ExtensionASTNodes),
		Extensions:        o.Extensions,
	}
}

// ToConfig returns what the interface type was built from.
func (i *InterfaceType) ToConfig() InterfaceConfig {
	return InterfaceConfig{
		Name:              i.name,
		Description:       i.description,
		Fields:            slices.Clone(i.Fields()),
		Interfaces:        slices.Clone(i.Interfaces()),
		ResolveType:       i.ResolveType,
		ASTNode:           i.ASTNode,
		ExtensionASTNodes: slices.Clone(i.ExtensionASTNodes),
		Extensions:        i.Extensions,
	}
}

// ToConfig returns what the union type was built from.
func (u *UnionType) ToConfig() UnionConfig {
	return UnionConfig{
		Name:              u.name,
		Description:       u.description,
		Types:             slices.Clone(u.Types()),
		ResolveType:       u.ResolveType,
		ASTNode:           u.ASTNode,
		ExtensionASTNodes: slices.Clone(u.ExtensionASTNodes),
		Extensions:        u.Extensions,
	}
}

// ToConfig returns what the enum type was built from.
func (e *EnumType) ToConfig() EnumConfig {
	return EnumConfig{
		Name:              e.name,
		Description:       e.description,
		Values:            slices.Clone(e.Values()),
		ASTNode:           e.ASTNode,
		ExtensionASTNodes: slices.Clone(e.ExtensionASTNodes),
		Extensions:        e.Extensions,
	}
}

// ToConfig returns what the input object type was built from.
func (o *InputObjectType) ToConfig() InputObjectConfig {
	return InputObjectConfig{
		Name:              o.name,
		Description:       o.description,
		Fields:            slices.Clone(o.Fields()),
		IsOneOf:           o.IsOneOf,
		ASTNode:           o.ASTNode,
		ExtensionASTNodes: slices.Clone(o.ExtensionASTNodes),
		Extensions:        o.Extensions,
	}
}

// ToConfig returns what the field was built from.
func (f *Field) ToConfig() FieldConfig {
	return FieldConfig{
		Description:       f.description,
		Type:              f.Type,
		Args:              slices.Clone(f.Args),
		Resolve:           f.Resolve,
		Subscribe:         f.Subscribe,
		DeprecationReason: f.DeprecationReason,
		ASTNode:           f.ASTNode,
		Extensions:        f.Extensions,
	}
}

// ToConfig returns what the argument was built from.
func (a *Argument) ToConfig() ArgumentConfig {
	return ArgumentConfig{
		Description:       a.description,
		Type:              a.Type,
		Default:           a.Default,
		DeprecationReason: a.DeprecationReason,
		ASTNode:           a.ASTNode,
		Extensions:        a.Extensions,
	}
}

// ToConfig returns what the input object field was built from.
func (f *InputField) ToConfig() InputFieldConfig {
	return InputFieldConfig{
		Description:       f.description,
		Type:              f.Type,
		Default:           f.Default,
		DeprecationReason: f.DeprecationReason,
		ASTNode:           f.ASTNode,
		Extensions:        f.Extensions,
	}
}

// ToConfig returns what the enum member was built from.
func (v *EnumValue) ToConfig() EnumValueConfig {
	return EnumValueConfig{
		Description:       v.description,
		Value:             InternalValue(v.Value),
		DeprecationReason: v.DeprecationReason,
		ASTNode:           v.ASTNode,
		Extensions:        v.Extensions,
	}
}

// ToConfig returns what the directive was built from.
func (d *Directive) ToConfig() DirectiveConfig {
	return DirectiveConfig{
		Name:              d.name,
		Description:       d.description,
		Locations:         slices.Clone(d.Locations),
		Args:              slices.Clone(d.Args),
		IsRepeatable:      d.IsRepeatable,
		DeprecationReason: d.DeprecationReason,
		ASTNode:           d.ASTNode,
		Extensions:        d.Extensions,
	}
}

// ToConfig returns what the schema was built from.
//
// The types are every type the schema holds, which is more than was handed to
// it: building a schema walks everything its roots and directives can reach.
func (s *Schema) ToConfig() Config {
	return Config{
		Description:       value.Just(s.Description()),
		Query:             s.QueryType(),
		Mutation:          s.MutationType(),
		Subscription:      s.SubscriptionType(),
		Types:             slices.Clone(s.Types()),
		Directives:        slices.Clone(s.Directives()),
		ASTNode:           s.ASTNode,
		ExtensionASTNodes: slices.Clone(s.ExtensionASTNodes),
		Extensions:        s.Extensions,
	}
}
