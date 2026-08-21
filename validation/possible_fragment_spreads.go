package validation

import (
	"github.com/ikawaha/graphql/internal/typeinfo"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// PossibleFragmentSpreadsRule reports a fragment spread that can never apply.
//
// A fragment applies where its type condition and the type being selected from
// have some type in common. Where they have none, nothing the fragment selects
// could ever be returned, so the spread is dead rather than merely unusual.
func PossibleFragmentSpreadsRule(ctx *Context) language.Visitor {
	// fragmentType resolves the type a named fragment applies to.
	fragmentType := func(name string) schema.CompositeType {
		fragment := ctx.Fragment(name)
		if fragment == nil || fragment.TypeCondition == nil {
			return nil
		}
		t, known := typeinfo.TypeFromAST(ctx.Schema(), fragment.TypeCondition)
		if !known {
			return nil
		}
		composite, isComposite := t.(schema.CompositeType)
		if !isComposite {
			return nil
		}
		return composite
	}

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			parent := ctx.ParentType()
			if parent == nil {
				return language.VisitContinue
			}
			switch n := node.(type) {
			case *language.InlineFragment:
				inner, isComposite := ctx.Type().(schema.CompositeType)
				if !isComposite || schema.DoTypesOverlap(ctx.Schema(), inner, parent) {
					return language.VisitContinue
				}
				ctx.Reportf([]language.Node{n},
					"Fragment cannot be spread here as objects of type %s can never be of type %s.",
					quote(parent.Name()), quote(inner.Name()))

			case *language.FragmentSpread:
				if n.Name == nil {
					return language.VisitContinue
				}
				inner := fragmentType(n.Name.Value)
				if inner == nil || schema.DoTypesOverlap(ctx.Schema(), inner, parent) {
					return language.VisitContinue
				}
				ctx.Reportf([]language.Node{n},
					"Fragment %s cannot be spread here as objects of type %s can never be of type %s.",
					quote(n.Name.Value), quote(parent.Name()), quote(inner.Name()))
			}
			return language.VisitContinue
		},
	}
}
