package validation

import "github.com/ikawaha/graphql/language"

// ExecutableDefinitionsRule reports type definitions in a document meant for
// execution.
//
// A request carries operations and fragments. A type definition in one is
// either a mistake or an attempt to change the schema through the request
// endpoint, and neither should be carried out.
func ExecutableDefinitionsRule(ctx *Context) language.Visitor {
	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			doc, isDocument := node.(*language.Document)
			if !isDocument {
				return language.VisitContinue
			}
			for _, def := range doc.Definitions {
				if _, executable := def.(language.ExecutableDefinition); executable {
					continue
				}
				ctx.Reportf([]language.Node{def}, "The %s definition is not executable.", definitionLabel(def))
			}
			// Nothing below the document concerns this rule.
			return language.VisitSkip
		},
	}
}

// definitionLabel names a definition for a message, in the way upstream does:
// a schema definition has no name of its own, and everything else is quoted.
func definitionLabel(def language.Definition) string {
	switch node := def.(type) {
	case *language.SchemaDefinition, *language.SchemaExtension:
		return "schema"
	case language.TypeDefinition:
		return quote(typeDefinitionName(node))
	case language.TypeExtension:
		return quote(typeExtensionName(node))
	case *language.DirectiveDefinition:
		return quote(nameOf(node.Name))
	default:
		return "unknown"
	}
}
