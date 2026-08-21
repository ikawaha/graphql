package validation

import "github.com/ikawaha/graphql/language"

// maxIntrospectionDepth is how deeply the introspection types may be nested
// before a document is refused.
const maxIntrospectionDepth = 3

// MaxIntrospectionDepthRule reports an introspection query nested deeply
// enough to be expensive out of proportion to its size.
//
// The introspection types refer to one another, so a short query can ask for a
// response that grows exponentially: a type's fields' types' fields, and so
// on. The response would be enormous while the request stayed small, which is
// why this is checked before anything else runs.
func MaxIntrospectionDepthRule(ctx *Context) language.Visitor {
	// tooDeep counts nesting through fields, inline fragments and spreads.
	var tooDeep func(node language.Node, depth int, visiting map[*language.FragmentDefinition]bool) bool
	tooDeep = func(node language.Node, depth int, visiting map[*language.FragmentDefinition]bool) bool {
		switch n := node.(type) {
		case *language.FragmentSpread:
			if n.Name == nil {
				return false
			}
			fragment := ctx.Fragment(n.Name.Value)
			// A cycle is a separate complaint, and following it here would not
			// end.
			if fragment == nil || visiting[fragment] {
				return false
			}
			visiting[fragment] = true
			defer delete(visiting, fragment)
			return anyTooDeep(selectionsOf(fragment.SelectionSet), depth, visiting, tooDeep)

		case *language.Field:
			// Only the fields that lead from a type back to more types are
			// counted; an ordinary field costs what its resolver costs, which
			// is a different concern.
			if name := nameOf(n.Name); name == "fields" || name == "interfaces" ||
				name == "possibleTypes" || name == "inputFields" {
				depth++
				if depth >= maxIntrospectionDepth {
					return true
				}
			}
			return anyTooDeep(selectionsOf(n.SelectionSet), depth, visiting, tooDeep)

		case *language.InlineFragment:
			return anyTooDeep(selectionsOf(n.SelectionSet), depth, visiting, tooDeep)
		}
		return false
	}

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			field, isField := node.(*language.Field)
			if !isField {
				return language.VisitContinue
			}
			// The count starts at the two fields that reach the introspection
			// types in the first place.
			if name := nameOf(field.Name); name != "__schema" && name != "__type" {
				return language.VisitContinue
			}
			if tooDeep(field, 0, map[*language.FragmentDefinition]bool{}) {
				ctx.Report("Maximum introspection depth exceeded", field)
			}
			// Nothing under a field already measured needs measuring again;
			// another introspection root elsewhere in the document is measured
			// on its own, since each costs what it costs.
			return language.VisitSkip
		},
	}
}

// anyTooDeep reports whether any of the given nodes goes too deep.
func anyTooDeep(
	nodes []language.Selection,
	depth int,
	visiting map[*language.FragmentDefinition]bool,
	tooDeep func(language.Node, int, map[*language.FragmentDefinition]bool) bool,
) bool {
	for _, n := range nodes {
		if tooDeep(n, depth, visiting) {
			return true
		}
	}
	return false
}

// selectionsOf reads a selection set, coping with there being none.
func selectionsOf(set *language.SelectionSet) []language.Selection {
	if set == nil {
		return nil
	}
	return set.Selections
}
