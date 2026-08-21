package validation

import (
	"github.com/ikawaha/graphql/internal/typeinfo"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// VariablesAreInputTypesRule reports a variable declared as a type no value
// can be given for.
//
// A variable carries a value from the request into the document, and only a
// scalar, an enum or an input object can be written down as one. An object
// type is something the server returns, not something a client can send.
func VariablesAreInputTypesRule(ctx *Context) language.Visitor {
	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			def, isVariableDefinition := node.(*language.VariableDefinition)
			if !isVariableDefinition || def.Type == nil {
				return language.VisitContinue
			}
			// A type the schema does not have is a separate complaint.
			t, known := typeinfo.TypeFromAST(ctx.Schema(), def.Type)
			if !known || t == nil || schema.IsInputType(t) {
				return language.VisitContinue
			}
			name := ""
			if def.Variable != nil {
				name = nameOf(def.Variable.Name)
			}
			ctx.Reportf([]language.Node{def.Type}, "Variable %s cannot be non-input type %s.",
				quote("$"+name), quote(language.Print(def.Type)))
			return language.VisitContinue
		},
	}
}
