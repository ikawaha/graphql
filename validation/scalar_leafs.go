package validation

import (
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// ScalarLeafsRule reports a selection set where the field's type has no fields
// to select, and the absence of one where the type does.
//
// Every leaf of a query has to be a scalar or an enum, because that is where
// the response stops being a shape and starts being a value.
func ScalarLeafsRule(ctx *Context) language.Visitor {
	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			field, isField := node.(*language.Field)
			if !isField {
				return language.VisitContinue
			}
			// A field the schema does not have is a separate complaint.
			t := ctx.Type()
			if t == nil {
				return language.VisitContinue
			}
			if schema.IsLeafType(schema.NamedTypeOf(t)) {
				if field.SelectionSet != nil {
					ctx.Reportf([]language.Node{field.SelectionSet},
						"Field %s must not have a selection since type %s has no subfields.",
						quote(nameOf(field.Name)), quote(t.String()))
				}
				return language.VisitContinue
			}
			switch name := nameOf(field.Name); {
			case field.SelectionSet == nil:
				ctx.Reportf([]language.Node{field},
					"Field %s of type %s must have a selection of subfields. Did you mean %s?",
					quote(name), quote(t.String()), quote(name+" { ... }"))
			case len(field.SelectionSet.Selections) == 0:
				// A document cannot be written this way — the grammar wants at
				// least one selection between the braces — but an AST built by
				// hand can be, and it is a different thing to be told.
				ctx.Reportf([]language.Node{field},
					"Field %s of type %s must have at least one field selected.",
					quote(name), quote(t.String()))
			}
			return language.VisitContinue
		},
	}
}
