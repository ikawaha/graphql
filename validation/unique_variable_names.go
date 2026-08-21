package validation

import "github.com/ikawaha/graphql/language"

// UniqueVariableNamesRule reports a variable declared twice by one definition.
func UniqueVariableNamesRule(ctx *Context) language.Visitor {
	check := func(definitions []*language.VariableDefinition) {
		seen := map[string][]*language.Name{}
		var order []string
		for _, def := range definitions {
			if def == nil || def.Variable == nil || def.Variable.Name == nil {
				continue
			}
			name := def.Variable.Name.Value
			if _, known := seen[name]; !known {
				order = append(order, name)
			}
			seen[name] = append(seen[name], def.Variable.Name)
		}
		for _, name := range order {
			if nodes := seen[name]; len(nodes) > 1 {
				blamed := make([]language.Node, len(nodes))
				for i, n := range nodes {
					blamed[i] = n
				}
				ctx.Reportf(blamed, "There can be only one variable named %s.", quote("$"+name))
			}
		}
	}

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			switch n := node.(type) {
			case *language.OperationDefinition:
				check(n.VariableDefinitions)
			case *language.FragmentDefinition:
				check(n.VariableDefinitions)
			}
			return language.VisitContinue
		},
	}
}
