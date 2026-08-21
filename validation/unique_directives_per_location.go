package validation

import (
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// UniqueDirectivesPerLocationRule reports a directive applied twice in one
// place, unless it is declared repeatable.
//
// A directive that is not repeatable says one thing about what it is on, so
// two of them are either redundant or in disagreement, and there is no rule
// for which would win.
func UniqueDirectivesPerLocationRule(ctx *Context) language.Visitor {
	repeatable := map[string]bool{}
	known := map[string]bool{}
	directives := schema.SpecifiedDirectives
	if ctx.Schema() != nil {
		directives = ctx.Schema().Directives()
	}
	for _, d := range directives {
		if d != nil {
			known[d.Name()] = true
			repeatable[d.Name()] = d.IsRepeatable
		}
	}
	for _, def := range ctx.Document().Definitions {
		declaration, isDirective := def.(*language.DirectiveDefinition)
		if isDirective && declaration.Name != nil {
			known[declaration.Name.Value] = true
			repeatable[declaration.Name.Value] = declaration.Repeatable
		}
	}

	// A type and its extensions are one place as far as this rule is
	// concerned, so a directive on the type and on an extension of it counts
	// as twice in the same place.
	seenByType := map[string]map[string]*language.Directive{}

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			applied := directivesOn(node)
			if len(applied) == 0 {
				return language.VisitContinue
			}

			seen := map[string]*language.Directive{}
			if name := extendedTypeName(node); name != "" {
				if existing, started := seenByType[name]; started {
					seen = existing
				} else {
					seenByType[name] = seen
				}
			}

			for _, directive := range applied {
				if directive == nil || directive.Name == nil {
					continue
				}
				name := directive.Name.Value
				// An unknown directive, or a repeatable one, is not this
				// rule's business.
				if !known[name] || repeatable[name] {
					continue
				}
				if first, twice := seen[name]; twice {
					ctx.Reportf([]language.Node{first, directive},
						"The directive %s can only be used once at this location.", quote("@"+name))
				} else {
					seen[name] = directive
				}
			}
			return language.VisitContinue
		},
	}
}

// extendedTypeName names the type a definition or extension is about, so that
// the two share one tally of directives. Anything else returns the empty
// string and is counted on its own.
func extendedTypeName(node language.Node) string {
	switch n := node.(type) {
	case language.TypeDefinition:
		return "type:" + typeDefinitionName(n)
	case language.TypeExtension:
		return "type:" + typeExtensionName(n)
	case *language.SchemaDefinition, *language.SchemaExtension:
		return "schema"
	case *language.DirectiveDefinition:
		return "directive:" + nameOf(n.Name)
	case *language.DirectiveExtension:
		return "directive:" + nameOf(n.Name)
	default:
		return ""
	}
}

// directivesOn returns the directives written on a node, for the many kinds of
// node that can carry them.
func directivesOn(node language.Node) []*language.Directive {
	switch n := node.(type) {
	case *language.OperationDefinition:
		return n.Directives
	case *language.Field:
		return n.Directives
	case *language.FragmentSpread:
		return n.Directives
	case *language.InlineFragment:
		return n.Directives
	case *language.FragmentDefinition:
		return n.Directives
	case *language.VariableDefinition:
		return n.Directives
	case *language.SchemaDefinition:
		return n.Directives
	case *language.SchemaExtension:
		return n.Directives
	case *language.ScalarTypeDefinition:
		return n.Directives
	case *language.ScalarTypeExtension:
		return n.Directives
	case *language.ObjectTypeDefinition:
		return n.Directives
	case *language.ObjectTypeExtension:
		return n.Directives
	case *language.InterfaceTypeDefinition:
		return n.Directives
	case *language.InterfaceTypeExtension:
		return n.Directives
	case *language.UnionTypeDefinition:
		return n.Directives
	case *language.UnionTypeExtension:
		return n.Directives
	case *language.EnumTypeDefinition:
		return n.Directives
	case *language.EnumTypeExtension:
		return n.Directives
	case *language.InputObjectTypeDefinition:
		return n.Directives
	case *language.InputObjectTypeExtension:
		return n.Directives
	case *language.FieldDefinition:
		return n.Directives
	case *language.InputValueDefinition:
		return n.Directives
	case *language.EnumValueDefinition:
		return n.Directives
	case *language.DirectiveDefinition:
		return n.Directives
	case *language.DirectiveExtension:
		return n.Directives
	default:
		return nil
	}
}
