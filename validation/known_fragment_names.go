package validation

import "github.com/ikawaha/graphql/language"

// KnownFragmentNamesRule reports a spread of a fragment the document does not
// define.
func KnownFragmentNamesRule(ctx *Context) language.Visitor {
	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			spread, isSpread := node.(*language.FragmentSpread)
			if !isSpread || spread.Name == nil {
				return language.VisitContinue
			}
			if ctx.Fragment(spread.Name.Value) == nil {
				ctx.Reportf([]language.Node{spread.Name}, "Unknown fragment %s.", quote(spread.Name.Value))
			}
			return language.VisitContinue
		},
	}
}
