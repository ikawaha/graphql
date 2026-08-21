package validation

import "github.com/ikawaha/graphql/language"

// LoneAnonymousOperationRule reports an unnamed operation in a document that
// defines more than one operation.
//
// An unnamed operation is the one to run when the request names none, so it
// only makes sense where it is the only one.
func LoneAnonymousOperationRule(ctx *Context) language.Visitor {
	operations := 0
	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			switch n := node.(type) {
			case *language.Document:
				operations = 0
				for _, def := range n.Definitions {
					if _, isOperation := def.(*language.OperationDefinition); isOperation {
						operations++
					}
				}
			case *language.OperationDefinition:
				if n.Name == nil && operations > 1 {
					ctx.Report("This anonymous operation must be the only defined operation.", n)
				}
			}
			return language.VisitContinue
		},
	}
}
