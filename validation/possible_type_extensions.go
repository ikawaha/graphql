package validation

import (
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// PossibleTypeExtensionsRule reports an extension of a type that does not
// exist, and one of the wrong kind.
//
// Extending is adding to something, so there has to be something to add to,
// and what is added has to be the same kind of thing: fields cannot be added
// to an enum.
func PossibleTypeExtensionsRule(ctx *Context) language.Visitor {
	defined := map[string]language.TypeDefinition{}
	for _, def := range ctx.Document().Definitions {
		if typeDef, isType := def.(language.TypeDefinition); isType {
			if name := typeDefinitionName(typeDef); name != "" {
				defined[name] = typeDef
			}
		}
	}

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			extension, isExtension := node.(language.TypeExtension)
			if !isExtension {
				return language.VisitContinue
			}
			named := typeExtensionNameNode(extension)
			if named == nil {
				return language.VisitSkip
			}
			name := named.Value

			// What kind the type is comes from the document if it defines it,
			// and otherwise from the schema being extended.
			var expected string
			var definition language.TypeDefinition
			if def, inDocument := defined[name]; inDocument {
				definition = def
				expected = kindOfDefinition(def)
			} else if existing := ctx.Schema().Type(name); existing != nil {
				expected = kindOfType(existing)
			}

			if expected == "" {
				options := make([]string, 0, len(defined))
				for known := range defined {
					options = append(options, known)
				}
				for _, t := range ctx.Schema().Types() {
					if t != nil {
						options = append(options, t.Name())
					}
				}
				ctx.Reportf([]language.Node{named},
					"Cannot extend type %s because it is not defined.%s",
					// This rule reads a document of type definitions rather
					// than a request, so there is nobody to hide the schema
					// from: the names it might suggest are the author's own.
					// graphql-js leaves the suggestions on here too.
					quote(name), schema.DidYouMean("", schema.SuggestionList(name, options)))
				return language.VisitSkip
			}

			if got := kindOfExtension(extension); got != expected {
				blamed := []language.Node{extension}
				if definition != nil {
					blamed = []language.Node{definition, extension}
				}
				ctx.Reportf(blamed, "Cannot extend non-%s type %s.", expected, quote(name))
			}
			return language.VisitSkip
		},
	}
}

// The three functions below name what kind of thing a type is, so that a
// definition, an extension and a built type can be compared.

func kindOfDefinition(node language.TypeDefinition) string {
	switch node.(type) {
	case *language.ScalarTypeDefinition:
		return "scalar"
	case *language.ObjectTypeDefinition:
		return "object"
	case *language.InterfaceTypeDefinition:
		return "interface"
	case *language.UnionTypeDefinition:
		return "union"
	case *language.EnumTypeDefinition:
		return "enum"
	case *language.InputObjectTypeDefinition:
		return "input object"
	default:
		return ""
	}
}

func kindOfExtension(node language.TypeExtension) string {
	switch node.(type) {
	case *language.ScalarTypeExtension:
		return "scalar"
	case *language.ObjectTypeExtension:
		return "object"
	case *language.InterfaceTypeExtension:
		return "interface"
	case *language.UnionTypeExtension:
		return "union"
	case *language.EnumTypeExtension:
		return "enum"
	case *language.InputObjectTypeExtension:
		return "input object"
	default:
		return ""
	}
}

func kindOfType(t schema.NamedType) string {
	switch t.(type) {
	case *schema.ScalarType:
		return "scalar"
	case *schema.ObjectType:
		return "object"
	case *schema.InterfaceType:
		return "interface"
	case *schema.UnionType:
		return "union"
	case *schema.EnumType:
		return "enum"
	case *schema.InputObjectType:
		return "input object"
	default:
		return ""
	}
}
