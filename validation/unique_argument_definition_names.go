package validation

import "github.com/ikawaha/graphql/language"

// UniqueArgumentDefinitionNamesRule reports an argument declared twice on one
// field or directive.
func UniqueArgumentDefinitionNamesRule(ctx *Context) language.Visitor {
	check := func(owner string, args []*language.InputValueDefinition) {
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
			nodes := seen[name]
			if len(nodes) < 2 {
				continue
			}
			blamed := make([]language.Node, len(nodes))
			for i, n := range nodes {
				blamed[i] = n
			}
			ctx.Reportf(blamed, "Argument %s can only be defined once.", quote(owner+"("+name+":)"))
		}
	}

	// A field's arguments belong to the type that declares it, so the message
	// names both.
	checkFields := func(typeName string, fields []*language.FieldDefinition) {
		for _, field := range fields {
			if field != nil && field.Name != nil {
				check(typeName+"."+field.Name.Value, field.Arguments)
			}
		}
	}

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			switch n := node.(type) {
			case *language.DirectiveDefinition:
				check("@"+nameOf(n.Name), n.Arguments)
			case *language.ObjectTypeDefinition:
				checkFields(nameOf(n.Name), n.Fields)
			case *language.ObjectTypeExtension:
				checkFields(nameOf(n.Name), n.Fields)
			case *language.InterfaceTypeDefinition:
				checkFields(nameOf(n.Name), n.Fields)
			case *language.InterfaceTypeExtension:
				checkFields(nameOf(n.Name), n.Fields)
			default:
				return language.VisitContinue
			}
			return language.VisitSkip
		},
	}
}
