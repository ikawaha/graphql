package validation

import "github.com/ikawaha/graphql/language"

// UniqueDirectiveNamesRule reports a directive declared twice, or declared
// where the schema already has one of that name.
func UniqueDirectiveNamesRule(ctx *Context) language.Visitor {
	known := map[string]*language.Name{}
	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			declaration, isDirective := node.(*language.DirectiveDefinition)
			if !isDirective || declaration.Name == nil {
				return language.VisitContinue
			}
			name := declaration.Name.Value
			switch {
			case ctx.Schema().Directive(name) != nil:
				ctx.Reportf([]language.Node{declaration.Name},
					"Directive %s already exists in the schema. It cannot be redefined.", quote("@"+name))
			case known[name] != nil:
				ctx.Reportf([]language.Node{known[name], declaration.Name},
					"There can be only one directive named %s.", quote("@"+name))
			default:
				known[name] = declaration.Name
			}
			return language.VisitSkip
		},
	}
}
