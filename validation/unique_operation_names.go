package validation

import "github.com/ikawaha/graphql/language"

// UniqueOperationNamesRule reports two operations of the same name.
//
// A request names the operation to run, so two of a name leave no way to say
// which was meant.
func UniqueOperationNamesRule(ctx *Context) language.Visitor {
	known := map[string]*language.Name{}
	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			switch n := node.(type) {
			case *language.OperationDefinition:
				if n.Name != nil {
					if first, taken := known[n.Name.Value]; taken {
						ctx.Reportf([]language.Node{first, n.Name},
							"There can be only one operation named %s.", quote(n.Name.Value))
					} else {
						known[n.Name.Value] = n.Name
					}
				}
				return language.VisitSkip
			case *language.FragmentDefinition:
				return language.VisitSkip
			}
			return language.VisitContinue
		},
	}
}
