package validation

import (
	"strconv"

	"github.com/ikawaha/graphql/language"
)

// quote wraps a name for a message, the way upstream writes them.
func quote(s string) string { return strconv.Quote(s) }

// nameOf reads a name, coping with there being none.
func nameOf(n *language.Name) string {
	if n == nil {
		return ""
	}
	return n.Value
}

// typeDefinitionName reads the name a type definition declares.
func typeDefinitionName(node language.TypeDefinition) string {
	switch def := node.(type) {
	case *language.ScalarTypeDefinition:
		return nameOf(def.Name)
	case *language.ObjectTypeDefinition:
		return nameOf(def.Name)
	case *language.InterfaceTypeDefinition:
		return nameOf(def.Name)
	case *language.UnionTypeDefinition:
		return nameOf(def.Name)
	case *language.EnumTypeDefinition:
		return nameOf(def.Name)
	case *language.InputObjectTypeDefinition:
		return nameOf(def.Name)
	default:
		return ""
	}
}

// typeExtensionName reads the name a type extension applies to.
func typeExtensionName(node language.TypeExtension) string {
	switch ext := node.(type) {
	case *language.ScalarTypeExtension:
		return nameOf(ext.Name)
	case *language.ObjectTypeExtension:
		return nameOf(ext.Name)
	case *language.InterfaceTypeExtension:
		return nameOf(ext.Name)
	case *language.UnionTypeExtension:
		return nameOf(ext.Name)
	case *language.EnumTypeExtension:
		return nameOf(ext.Name)
	case *language.InputObjectTypeExtension:
		return nameOf(ext.Name)
	default:
		return ""
	}
}

// typeDefinitionNameNode returns the name node of a type definition, which is
// what an error about the name should point at.
func typeDefinitionNameNode(node language.TypeDefinition) *language.Name {
	switch def := node.(type) {
	case *language.ScalarTypeDefinition:
		return def.Name
	case *language.ObjectTypeDefinition:
		return def.Name
	case *language.InterfaceTypeDefinition:
		return def.Name
	case *language.UnionTypeDefinition:
		return def.Name
	case *language.EnumTypeDefinition:
		return def.Name
	case *language.InputObjectTypeDefinition:
		return def.Name
	default:
		return nil
	}
}

// typeExtensionNameNode returns the name node of a type extension, which is
// what an error about the name should point at.
func typeExtensionNameNode(node language.TypeExtension) *language.Name {
	switch ext := node.(type) {
	case *language.ScalarTypeExtension:
		return ext.Name
	case *language.ObjectTypeExtension:
		return ext.Name
	case *language.InterfaceTypeExtension:
		return ext.Name
	case *language.UnionTypeExtension:
		return ext.Name
	case *language.EnumTypeExtension:
		return ext.Name
	case *language.InputObjectTypeExtension:
		return ext.Name
	default:
		return nil
	}
}
