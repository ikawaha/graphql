package validation

import "github.com/ikawaha/graphql/language"

// UniqueInputFieldNamesRule reports a field given twice in one input object
// literal.
func UniqueInputFieldNamesRule(ctx *Context) language.Visitor {
	// Input objects nest, so each level keeps its own names and the level
	// below is pushed on top.
	var stack []map[string]*language.Name
	known := map[string]*language.Name{}

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			switch n := node.(type) {
			case *language.ObjectValue:
				stack = append(stack, known)
				known = map[string]*language.Name{}
			case *language.ObjectField:
				if n.Name == nil {
					break
				}
				if first, taken := known[n.Name.Value]; taken {
					ctx.Reportf([]language.Node{first, n.Name},
						"There can be only one input field named %s.", quote(n.Name.Value))
				} else {
					known[n.Name.Value] = n.Name
				}
			}
			return language.VisitContinue
		},
		Leave: func(node language.Node, _ language.VisitContext) language.VisitAction {
			if _, isObject := node.(*language.ObjectValue); !isObject {
				return language.VisitContinue
			}
			known = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			return language.VisitContinue
		},
	}
}
