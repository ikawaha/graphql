package validation

import (
	"github.com/ikawaha/graphql/internal/typeinfo"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// FragmentsOnCompositeTypesRule reports a fragment conditioned on a type that
// has no fields.
//
// A fragment contributes a selection set, and only an object, an interface or
// a union has fields for one to select.
func FragmentsOnCompositeTypesRule(ctx *Context) language.Visitor {
	// A type condition naming something the schema does not have is a separate
	// complaint, so nothing is said about it here.
	check := func(condition *language.NamedType) bool {
		if condition == nil {
			return true
		}
		t, known := typeinfo.TypeFromAST(ctx.Schema(), condition)
		return !known || t == nil || schema.IsCompositeType(t)
	}

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			switch n := node.(type) {
			case *language.InlineFragment:
				if !check(n.TypeCondition) {
					ctx.Reportf([]language.Node{n.TypeCondition},
						"Fragment cannot condition on non composite type %s.",
						quote(language.Print(n.TypeCondition)))
				}
			case *language.FragmentDefinition:
				if !check(n.TypeCondition) {
					ctx.Reportf([]language.Node{n.TypeCondition},
						"Fragment %s cannot condition on non composite type %s.",
						quote(nameOf(n.Name)), quote(language.Print(n.TypeCondition)))
				}
			}
			return language.VisitContinue
		},
	}
}
