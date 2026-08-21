package validation

import (
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// NoDeprecatedCustomRule reports the use of anything the schema marks
// deprecated.
//
// It is not one of the rules the specification requires, because a deprecated
// field still works and refusing it would break clients that have not caught
// up. It is here for the servers that want to find out who still asks.
func NoDeprecatedCustomRule(ctx *Context) language.Visitor {
	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			switch n := node.(type) {
			case *language.Field:
				field, parent := ctx.FieldDef(), ctx.ParentType()
				if field == nil || parent == nil || !field.IsDeprecated() {
					break
				}
				ctx.Reportf([]language.Node{n}, "The field %s is deprecated. %s",
					parent.Name()+"."+field.Name(), field.DeprecationReason.Or(""))

			case *language.Argument:
				arg := ctx.Argument()
				if arg == nil || !arg.IsDeprecated() {
					break
				}
				// An argument names the field or directive it belongs to, so
				// one message covers both.
				ctx.Reportf([]language.Node{n}, "The argument %s is deprecated. %s",
					quote(arg.String()), arg.DeprecationReason.Or(""))

			case *language.ObjectField:
				input, isInputObject := schema.NamedTypeOf(ctx.ParentInputType()).(*schema.InputObjectType)
				if !isInputObject || n.Name == nil {
					break
				}
				field := input.Field(n.Name.Value)
				if field == nil || !field.IsDeprecated() {
					break
				}
				ctx.Reportf([]language.Node{n}, "The input field %s is deprecated. %s",
					input.Name()+"."+field.Name(), field.DeprecationReason.Or(""))

			case *language.EnumValue:
				member := ctx.EnumValue()
				if member == nil || !member.IsDeprecated() {
					break
				}
				enum := schema.NamedTypeOf(ctx.InputType())
				if enum == nil {
					break
				}
				ctx.Reportf([]language.Node{n}, "The enum value %s is deprecated. %s",
					quote(enum.Name()+"."+member.Name()), member.DeprecationReason.Or(""))
			}
			return language.VisitContinue
		},
	}
}
