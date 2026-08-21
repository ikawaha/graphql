package validation

import "github.com/ikawaha/graphql/language"

// UniqueArgumentNamesRule reports an argument given twice to one field or
// directive.
func UniqueArgumentNamesRule(ctx *Context) language.Visitor {
	check := func(args []*language.Argument) {
		seen := map[string][]*language.Name{}
		var order []string
		for _, arg := range args {
			if arg == nil || arg.Name == nil {
				continue
			}
			name := arg.Name.Value
			if _, known := seen[name]; !known {
				order = append(order, name)
			}
			seen[name] = append(seen[name], arg.Name)
		}
		for _, name := range order {
			if nodes := seen[name]; len(nodes) > 1 {
				blamed := make([]language.Node, len(nodes))
				for i, n := range nodes {
					blamed[i] = n
				}
				ctx.Reportf(blamed, "There can be only one argument named %s.", quote(name))
			}
		}
	}

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			switch n := node.(type) {
			case *language.Field:
				check(n.Arguments)
			case *language.Directive:
				check(n.Arguments)
			}
			return language.VisitContinue
		},
	}
}
