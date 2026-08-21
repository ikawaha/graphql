package validation

import "github.com/ikawaha/graphql/language"

// UniqueTypeNamesRule reports a type defined twice, or defined where the
// schema already has one of that name.
//
// A name identifies a type throughout a schema, so a second definition of one
// leaves no way to say which is meant.
func UniqueTypeNamesRule(ctx *Context) language.Visitor {
	known := map[string]*language.Name{}
	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			definition, isType := node.(language.TypeDefinition)
			if !isType {
				return language.VisitContinue
			}
			name := typeDefinitionName(definition)
			if name == "" {
				return language.VisitSkip
			}
			switch {
			case ctx.Schema().Type(name) != nil:
				ctx.Reportf([]language.Node{typeDefinitionNameNode(definition)},
					"Type %s already exists in the schema. It cannot also be defined in this type definition.",
					quote(name))
			case known[name] != nil:
				ctx.Reportf([]language.Node{known[name], typeDefinitionNameNode(definition)},
					"There can be only one type named %s.", quote(name))
			default:
				known[name] = typeDefinitionNameNode(definition)
			}
			// Nothing inside a definition names a type at this level.
			return language.VisitSkip
		},
	}
}
