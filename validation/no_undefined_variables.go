package validation

import "github.com/ikawaha/graphql/language"

// NoUndefinedVariablesRule reports a variable used by an operation that the
// operation does not declare.
//
// A variable is supplied by the request against the operation's declarations,
// so one used but not declared can never be given a value. The check reaches
// through fragments, since a fragment's variables are supplied by whichever
// operation spreads it.
func NoUndefinedVariablesRule(ctx *Context) language.Visitor {
	return language.Visitor{
		// Reported on the way in, as graphql-js reports it: what the operation
		// declares is written on the operation, and what it uses is worked out
		// by a walk of its own, so there is nothing to wait for. That is also
		// what puts these errors before the ones found inside the operation.
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			operation, isOperation := node.(*language.OperationDefinition)
			if !isOperation {
				return language.VisitContinue
			}
			declared := map[string]bool{}
			for _, def := range operation.VariableDefinitions {
				if def != nil && def.Variable != nil && def.Variable.Name != nil {
					declared[def.Variable.Name.Value] = true
				}
			}
			for _, usage := range ctx.RecursiveVariableUsages(operation) {
				if usage.Node == nil || usage.Node.Name == nil {
					continue
				}
				// A variable the fragment declares for itself is supplied by
				// the spread, not by the operation.
				if usage.FragmentVarDef != nil {
					continue
				}
				name := usage.Node.Name.Value
				if declared[name] {
					continue
				}
				if operation.Name != nil {
					ctx.Reportf([]language.Node{usage.Node, operation},
						"Variable %s is not defined by operation %s.",
						quote("$"+name), quote(operation.Name.Value))
				} else {
					ctx.Reportf([]language.Node{usage.Node, operation},
						"Variable %s is not defined.", quote("$"+name))
				}
			}
			return language.VisitContinue
		},
	}
}
