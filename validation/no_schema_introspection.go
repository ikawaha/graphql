package validation

import (
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// NoSchemaIntrospectionCustomRule reports any use of the introspection types.
//
// It is not one of the rules the specification requires, and turning
// introspection off does not make a server secure: a schema is discoverable by
// other means, and a field left unprotected is unprotected either way. It is
// here for servers that have decided not to publish their schema.
func NoSchemaIntrospectionCustomRule(ctx *Context) language.Visitor {
	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			field, isField := node.(*language.Field)
			if !isField {
				return language.VisitContinue
			}
			t := schema.NamedTypeOf(ctx.Type())
			if t == nil || !schema.IsIntrospectionType(t) {
				return language.VisitContinue
			}
			ctx.Reportf([]language.Node{field},
				"GraphQL introspection has been disabled, but the requested query contained the field %s.",
				quote(nameOf(field.Name)))
			return language.VisitContinue
		},
	}
}
