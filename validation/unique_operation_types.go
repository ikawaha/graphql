package validation

import "github.com/ikawaha/graphql/language"

// UniqueOperationTypesRule reports a root operation type named more than once.
//
// Each kind of operation enters through one type, so naming two is a
// contradiction rather than an addition.
func UniqueOperationTypesRule(ctx *Context) language.Visitor {
	s := ctx.Schema()
	existing := map[language.OperationType]bool{
		language.OperationQuery:        s.QueryType() != nil,
		language.OperationMutation:     s.MutationType() != nil,
		language.OperationSubscription: s.SubscriptionType() != nil,
	}
	defined := map[language.OperationType]*language.OperationTypeDefinition{}

	check := func(operations []*language.OperationTypeDefinition) {
		for _, op := range operations {
			if op == nil {
				continue
			}
			switch {
			case existing[op.Operation]:
				ctx.Reportf([]language.Node{op},
					"Type for %s already defined in the schema. It cannot be redefined.", op.Operation)
			case defined[op.Operation] != nil:
				ctx.Reportf([]language.Node{defined[op.Operation], op},
					"There can be only one %s type in schema.", op.Operation)
			default:
				defined[op.Operation] = op
			}
		}
	}

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			switch n := node.(type) {
			case *language.SchemaDefinition:
				check(n.OperationTypes)
				return language.VisitSkip
			case *language.SchemaExtension:
				check(n.OperationTypes)
				return language.VisitSkip
			}
			return language.VisitContinue
		},
	}
}
