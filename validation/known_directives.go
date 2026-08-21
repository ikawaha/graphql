package validation

import (
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// KnownDirectivesRule reports a directive the schema does not define, and one
// used somewhere its definition does not allow.
//
// Where a directive may appear is part of what it means: @skip on a type
// definition would be read by nothing, so accepting it would only let a
// mistake through quietly.
func KnownDirectivesRule(ctx *Context) language.Visitor {
	allowed := map[string][]language.DirectiveLocation{}
	// With no schema to read them from, only the built-in directives are
	// known, which is the state SDL is checked in.
	directives := schema.SpecifiedDirectives
	if ctx.Schema() != nil {
		directives = ctx.Schema().Directives()
	}
	for _, d := range directives {
		if d != nil {
			allowed[d.Name()] = d.Locations
		}
	}
	// A document declaring a directive says where it may be used.
	for _, def := range ctx.Document().Definitions {
		declaration, isDirective := def.(*language.DirectiveDefinition)
		if !isDirective || declaration.Name == nil {
			continue
		}
		locations := make([]language.DirectiveLocation, 0, len(declaration.Locations))
		for _, loc := range declaration.Locations {
			if loc != nil {
				locations = append(locations, language.DirectiveLocation(loc.Value))
			}
		}
		allowed[declaration.Name.Value] = locations
	}

	return language.Visitor{
		Enter: func(node language.Node, vc language.VisitContext) language.VisitAction {
			directive, isDirective := node.(*language.Directive)
			if !isDirective || directive.Name == nil {
				return language.VisitContinue
			}
			name := directive.Name.Value
			locations, known := allowed[name]
			if !known {
				ctx.Reportf([]language.Node{directive}, "Unknown directive %s.", quote("@"+name))
				return language.VisitContinue
			}
			// The node the walk came from is the one the directive is on.
			here, determined := directiveLocationOf(vc)
			if !determined {
				return language.VisitContinue
			}
			for _, allowedHere := range locations {
				if allowedHere == here {
					return language.VisitContinue
				}
			}
			ctx.Reportf([]language.Node{directive}, "Directive %s may not be used on %s.",
				quote("@"+name), here)
			return language.VisitContinue
		},
	}
}

// directiveLocationOf works out which location a directive is written at, from
// the node it is attached to.
func directiveLocationOf(vc language.VisitContext) (language.DirectiveLocation, bool) {
	switch parent := vc.Parent.(type) {
	case *language.OperationDefinition:
		switch parent.Operation {
		case language.OperationQuery:
			return language.DirectiveLocationQuery, true
		case language.OperationMutation:
			return language.DirectiveLocationMutation, true
		case language.OperationSubscription:
			return language.DirectiveLocationSubscription, true
		}
		return "", false
	case *language.Field:
		return language.DirectiveLocationField, true
	case *language.FragmentSpread:
		return language.DirectiveLocationFragmentSpread, true
	case *language.InlineFragment:
		return language.DirectiveLocationInlineFragment, true
	case *language.FragmentDefinition:
		return language.DirectiveLocationFragmentDefinition, true
	case *language.VariableDefinition:
		return language.DirectiveLocationVariableDefinition, true
	case *language.SchemaDefinition, *language.SchemaExtension:
		return language.DirectiveLocationSchema, true
	case *language.ScalarTypeDefinition, *language.ScalarTypeExtension:
		return language.DirectiveLocationScalar, true
	case *language.ObjectTypeDefinition, *language.ObjectTypeExtension:
		return language.DirectiveLocationObject, true
	case *language.FieldDefinition:
		return language.DirectiveLocationFieldDefinition, true
	case *language.InterfaceTypeDefinition, *language.InterfaceTypeExtension:
		return language.DirectiveLocationInterface, true
	case *language.UnionTypeDefinition, *language.UnionTypeExtension:
		return language.DirectiveLocationUnion, true
	case *language.EnumTypeDefinition, *language.EnumTypeExtension:
		return language.DirectiveLocationEnum, true
	case *language.EnumValueDefinition:
		return language.DirectiveLocationEnumValue, true
	case *language.InputObjectTypeDefinition, *language.InputObjectTypeExtension:
		return language.DirectiveLocationInputObject, true
	case *language.DirectiveDefinition:
		return language.DirectiveLocationDirectiveDefinition, true
	case *language.InputValueDefinition:
		// The same node is an argument in one place and an input field in
		// another, so what encloses it decides which.
		if len(vc.Ancestors) >= 2 {
			if _, inInputObject := vc.Ancestors[len(vc.Ancestors)-2].(*language.InputObjectTypeDefinition); inInputObject {
				return language.DirectiveLocationInputFieldDefinition, true
			}
			if _, inInputObject := vc.Ancestors[len(vc.Ancestors)-2].(*language.InputObjectTypeExtension); inInputObject {
				return language.DirectiveLocationInputFieldDefinition, true
			}
		}
		return language.DirectiveLocationArgumentDefinition, true
	default:
		return "", false
	}
}
