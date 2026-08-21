package validation

import "github.com/ikawaha/graphql/language"

// UniqueFragmentNamesRule reports two fragments of the same name.
//
// A spread names the fragment to include, so two of a name leave no way to say
// which was meant.
func UniqueFragmentNamesRule(ctx *Context) language.Visitor {
	known := map[string]*language.Name{}
	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			switch n := node.(type) {
			case *language.OperationDefinition:
				return language.VisitSkip
			case *language.FragmentDefinition:
				if n.Name != nil {
					if first, taken := known[n.Name.Value]; taken {
						ctx.Reportf([]language.Node{first, n.Name},
							"There can be only one fragment named %s.", quote(n.Name.Value))
					} else {
						known[n.Name.Value] = n.Name
					}
				}
				return language.VisitSkip
			}
			return language.VisitContinue
		},
	}
}
