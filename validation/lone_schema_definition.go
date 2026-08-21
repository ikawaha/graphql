package validation

import "github.com/ikawaha/graphql/language"

// LoneSchemaDefinitionRule reports a second schema definition, and one written
// where a schema already exists.
//
// A schema definition says which types the operations enter through, and there
// is only one answer to that; a second would either repeat it or contradict it.
func LoneSchemaDefinitionRule(ctx *Context) language.Visitor {
	s := ctx.Schema()
	// A schema being extended already says where its operations start.
	alreadyDefined := s != nil &&
		(s.QueryType() != nil || s.MutationType() != nil || s.SubscriptionType() != nil)
	seen := 0

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			definition, isSchema := node.(*language.SchemaDefinition)
			if !isSchema {
				return language.VisitContinue
			}
			switch {
			case alreadyDefined:
				ctx.Report("Cannot define a new schema within a schema extension.", definition)
			case seen > 0:
				ctx.Report("Must provide only one schema definition.", definition)
			}
			seen++
			return language.VisitContinue
		},
	}
}
