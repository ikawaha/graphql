package validation

import "github.com/ikawaha/graphql/language"

// KnownOperationTypesRule reports an operation of a kind the schema has no
// root type for.
//
// A schema need not support mutations or subscriptions, and a document asking
// for one it does not support has nowhere to start.
func KnownOperationTypesRule(ctx *Context) language.Visitor {
	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			operation, isOperation := node.(*language.OperationDefinition)
			if !isOperation {
				return language.VisitContinue
			}
			if ctx.Schema().RootType(operation.Operation) == nil {
				ctx.Reportf([]language.Node{operation},
					"The %s operation is not supported by the schema.", operation.Operation)
			}
			return language.VisitContinue
		},
	}
}
