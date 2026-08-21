package validation

import (
	"github.com/ikawaha/graphql/internal/typeinfo"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// VariablesInAllowedPositionRule reports a variable used where its declared
// type does not fit.
//
// The request supplies a variable once and the document may use it in several
// places, so each use has to accept whatever the declaration allows. A
// nullable variable used where a non-null value is wanted is the usual case,
// and it is allowed only when a default stands in for null.
func VariablesInAllowedPositionRule(ctx *Context) language.Visitor {
	declarations := map[string]*language.VariableDefinition{}

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			switch n := node.(type) {
			case *language.OperationDefinition:
				declarations = map[string]*language.VariableDefinition{}
			case *language.VariableDefinition:
				if n.Variable != nil && n.Variable.Name != nil {
					declarations[n.Variable.Name.Value] = n
				}
			}
			return language.VisitContinue
		},
		// Uses are gathered for the whole operation, including through
		// fragments, so the check waits until the operation has been read.
		Leave: func(node language.Node, _ language.VisitContext) language.VisitAction {
			operation, isOperation := node.(*language.OperationDefinition)
			if !isOperation {
				return language.VisitContinue
			}
			for _, usage := range ctx.RecursiveVariableUsages(operation) {
				if usage.Node == nil || usage.Node.Name == nil || usage.Type == nil {
					continue
				}
				name := usage.Node.Name.Value
				declared := declarations[name]
				if usage.FragmentVarDef != nil {
					declared = usage.FragmentVarDef
				}
				// A variable that is never declared is a separate complaint.
				if declared == nil || declared.Type == nil {
					continue
				}
				declaredType, known := typeinfo.TypeFromAST(ctx.Schema(), declared.Type)
				if !known || declaredType == nil {
					continue
				}
				if !allowedIn(ctx.Schema(), declaredType, declared.DefaultValue, usage) {
					ctx.Reportf([]language.Node{declared, usage.Node},
						"Variable %s of type %s used in position expecting type %s.",
						quote("$"+name), quote(declaredType.String()), quote(usage.Type.String()))
					continue
				}
				// A field of an input object that takes exactly one key cannot
				// be fed something that might turn out null: there would then
				// be no key at all, and nothing says which was meant.
				if parent, isInputObject := usage.ParentType.(*schema.InputObjectType); isInputObject &&
					parent.IsOneOf && !schema.IsNonNullType(declaredType) {
					ctx.Reportf([]language.Node{declared, usage.Node},
						"Variable %s is of type %s but must be non-nullable to be used for OneOf Input Object %s.",
						quote("$"+name), quote(declaredType.String()), quote(parent.Name()))
				}
			}
			return language.VisitContinue
		},
	}
}

// allowedIn reports whether a variable of the declared type may be used where
// the document uses it.
func allowedIn(
	s *schema.Schema,
	declaredType schema.Type,
	declaredDefault language.Value,
	usage VariableUsage,
) bool {
	locationType := usage.Type
	if !schema.IsNonNullType(locationType) || schema.IsNonNullType(declaredType) {
		return schema.IsTypeSubTypeOf(s, declaredType, locationType)
	}

	// A nullable variable can stand in a non-null position when null can never
	// reach it: either the variable has a default of its own, or the place it
	// is used has one.
	hasVariableDefault := declaredDefault != nil
	if _, isNull := declaredDefault.(*language.NullValue); isNull {
		// A default of null is a default that supplies null, which is exactly
		// what the position cannot take.
		hasVariableDefault = false
	}
	_, hasLocationDefault := usage.Default.Get()
	if !hasVariableDefault && !hasLocationDefault {
		return false
	}
	return schema.IsTypeSubTypeOf(s, declaredType, schema.NullableTypeOf(locationType))
}
