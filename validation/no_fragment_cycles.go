package validation

import (
	"strings"

	"github.com/ikawaha/graphql/language"
)

// NoFragmentCyclesRule reports a fragment that reaches itself.
//
// Spreading a fragment inlines it, so a cycle would inline for ever. This has
// to be caught before anything else follows spreads, which is why the context
// stops at a fragment it has already been through rather than relying on it.
func NoFragmentCyclesRule(ctx *Context) language.Visitor {
	visited := map[string]bool{}
	// path is the chain of spreads followed to get here, and indexByName says
	// where in that chain each fragment was entered, so a repeat gives the
	// cycle directly.
	var path []*language.FragmentSpread
	indexByName := map[string]int{}

	var walk func(fragment *language.FragmentDefinition)
	walk = func(fragment *language.FragmentDefinition) {
		if fragment == nil || fragment.Name == nil || visited[fragment.Name.Value] {
			return
		}
		name := fragment.Name.Value
		visited[name] = true

		spreads := ctx.FragmentSpreads(fragment.SelectionSet)
		if len(spreads) == 0 {
			return
		}
		indexByName[name] = len(path)
		for _, spread := range spreads {
			if spread.Name == nil {
				continue
			}
			spreadName := spread.Name.Value
			start, inPath := indexByName[spreadName]
			path = append(path, spread)
			if !inPath {
				walk(ctx.Fragment(spreadName))
			} else {
				cycle := path[start:]
				var via []string
				for _, s := range cycle[:len(cycle)-1] {
					via = append(via, quote(s.Name.Value))
				}
				blamed := make([]language.Node, len(cycle))
				for i, s := range cycle {
					blamed[i] = s
				}
				if len(via) == 0 {
					ctx.Reportf(blamed, "Cannot spread fragment %s within itself.", quote(spreadName))
				} else {
					ctx.Reportf(blamed, "Cannot spread fragment %s within itself via %s.",
						quote(spreadName), strings.Join(via, ", "))
				}
			}
			path = path[:len(path)-1]
		}
		delete(indexByName, name)
	}

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			switch n := node.(type) {
			case *language.OperationDefinition:
				return language.VisitSkip
			case *language.FragmentDefinition:
				walk(n)
				return language.VisitSkip
			}
			return language.VisitContinue
		},
	}
}
