package validation

import (
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// UniqueEnumValueNamesRule reports an enum member declared twice.
//
// A member is identified by its name, so two of a name are either a repeat or
// a disagreement about what the name stands for.
func UniqueEnumValueNamesRule(ctx *Context) language.Visitor {
	// Members are counted per enum, and a definition and its extensions add to
	// the same tally.
	known := map[string]map[string]*language.Name{}

	check := func(typeName string, values []*language.EnumValueDefinition) {
		members, started := known[typeName]
		if !started {
			members = map[string]*language.Name{}
			known[typeName] = members
		}
		existing, _ := ctx.Schema().Type(typeName).(*schema.EnumType)
		for _, value := range values {
			if value == nil || value.Name == nil {
				continue
			}
			name := value.Name.Value
			switch {
			case existing != nil && existing.Value(name) != nil:
				ctx.Reportf([]language.Node{value.Name},
					"Enum value %s already exists in the schema. It cannot also be defined in this type extension.",
					quote(typeName+"."+name))
			case members[name] != nil:
				ctx.Reportf([]language.Node{members[name], value.Name},
					"Enum value %s can only be defined once.", quote(typeName+"."+name))
			default:
				members[name] = value.Name
			}
		}
	}

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			switch n := node.(type) {
			case *language.EnumTypeDefinition:
				check(nameOf(n.Name), n.Values)
				return language.VisitSkip
			case *language.EnumTypeExtension:
				check(nameOf(n.Name), n.Values)
				return language.VisitSkip
			}
			return language.VisitContinue
		},
	}
}
