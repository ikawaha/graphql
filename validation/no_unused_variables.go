package validation

import "github.com/ikawaha/graphql/language"

// NoUnusedVariablesRule reports a variable an operation declares but never
// uses.
//
// A declared variable is part of the contract a client codes against, so one
// that is never read is either a leftover or a use that was meant to be there.
func NoUnusedVariablesRule(ctx *Context) language.Visitor {
	return language.Visitor{
		// Reported on the way in rather than on the way out: what a definition
		// declares is written on the definition itself, and what it uses is
		// worked out by a walk of its own, so there is nothing to wait for.
		// graphql-js reports here too, which is what puts these errors before
		// the ones found inside the definition.
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			switch n := node.(type) {
			// A fragment that declares variables of its own is checked the way
			// an operation is: what it declares, it should use.
			case *language.FragmentDefinition:
				reportUnusedIn(ctx, ctx.VariableUsages(n), n.VariableDefinitions,
					"fragment", nameOf(n.Name))
			case *language.OperationDefinition:
				reportUnusedIn(ctx, ctx.RecursiveVariableUsages(n), n.VariableDefinitions,
					"operation", nameOf(n.Name))
			}
			return language.VisitContinue
		},
	}
}

// reportUnusedIn reports the variables a definition declares but never uses.
func reportUnusedIn(
	ctx *Context,
	usages []VariableUsage,
	declarations []*language.VariableDefinition,
	kind, owner string,
) {
	used := map[string]bool{}
	for _, usage := range usages {
		if usage.Node == nil || usage.Node.Name == nil {
			continue
		}
		// A variable a fragment declares for itself is supplied by the spread,
		// so an operation does not count it as one of its own.
		if kind == "operation" && usage.FragmentVarDef != nil {
			continue
		}
		used[usage.Node.Name.Value] = true
	}
	for _, def := range declarations {
		if def == nil || def.Variable == nil || def.Variable.Name == nil {
			continue
		}
		name := def.Variable.Name.Value
		if used[name] {
			continue
		}
		if owner == "" {
			ctx.Reportf([]language.Node{def}, "Variable %s is never used.", quote("$"+name))
			continue
		}
		ctx.Reportf([]language.Node{def}, "Variable %s is never used in %s %s.",
			quote("$"+name), kind, quote(owner))
	}
}
