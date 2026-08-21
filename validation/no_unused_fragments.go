package validation

import "github.com/ikawaha/graphql/language"

// NoUnusedFragmentsRule reports a fragment no operation reaches.
//
// An unused fragment is still parsed and validated, and it says something the
// request never asked for, so it is more likely a leftover than intended.
func NoUnusedFragmentsRule(ctx *Context) language.Visitor {
	var operations []*language.OperationDefinition
	var fragments []*language.FragmentDefinition

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			switch n := node.(type) {
			case *language.OperationDefinition:
				operations = append(operations, n)
				return language.VisitSkip
			case *language.FragmentDefinition:
				fragments = append(fragments, n)
				return language.VisitSkip
			}
			return language.VisitContinue
		},
		// The check runs on the way out, when every definition has been seen:
		// a fragment may be spread by an operation written below it.
		Leave: func(node language.Node, _ language.VisitContext) language.VisitAction {
			if _, isDocument := node.(*language.Document); !isDocument {
				return language.VisitContinue
			}
			used := map[string]bool{}
			for _, operation := range operations {
				for _, fragment := range ctx.RecursivelyReferencedFragments(operation) {
					if fragment.Name != nil {
						used[fragment.Name.Value] = true
					}
				}
			}
			for _, fragment := range fragments {
				if fragment.Name != nil && !used[fragment.Name.Value] {
					ctx.Reportf([]language.Node{fragment}, "Fragment %s is never used.", quote(fragment.Name.Value))
				}
			}
			return language.VisitContinue
		},
	}
}
