package utilities

import "github.com/ikawaha/graphql/language"

// SeparateOperations splits a document that defines several operations into
// one document per operation, each carrying only the fragments its operation
// reaches.
//
// A client that keeps all its queries in one file sends the whole file with
// every request, and the server parses and validates all of it each time.
// Splitting it once lets each request carry only what it needs.
//
// The operations are keyed by name; an unnamed one takes the empty string.
// Nothing is copied: each document points at the same definitions, so the
// result must be treated as read-only.
func SeparateOperations(doc *language.Document) map[string]*language.Document {
	if doc == nil {
		return nil
	}

	// A spread names a fragment, and that fragment may spread others, so what
	// each definition reaches directly is worked out once and then followed.
	spreads := map[language.Node][]string{}
	fragments := map[string]*language.FragmentDefinition{}
	var operations []*language.OperationDefinition

	for _, def := range doc.Definitions {
		switch node := def.(type) {
		case *language.OperationDefinition:
			operations = append(operations, node)
			spreads[node] = spreadsWithin(node)
		case *language.FragmentDefinition:
			if node.Name != nil {
				fragments[node.Name.Value] = node
				spreads[node] = spreadsWithin(node)
			}
		}
	}

	separated := make(map[string]*language.Document, len(operations))
	for _, operation := range operations {
		// The operation and the fragments it reaches keep the order the
		// original document wrote them in, so that each piece reads like the
		// part of the document it came from.
		reached := map[string]bool{}
		collectReachable(operation, spreads, fragments, reached)

		var definitions []language.Definition
		for _, def := range doc.Definitions {
			switch node := def.(type) {
			case *language.OperationDefinition:
				if node == operation {
					definitions = append(definitions, node)
				}
			case *language.FragmentDefinition:
				if node.Name != nil && reached[node.Name.Value] {
					definitions = append(definitions, node)
				}
			}
		}

		name := ""
		if operation.Name != nil {
			name = operation.Name.Value
		}
		separated[name] = &language.Document{Definitions: definitions}
	}
	return separated
}

// collectReachable records every fragment a definition can reach, following
// spreads as far as they go.
func collectReachable(
	from language.Node,
	spreads map[language.Node][]string,
	fragments map[string]*language.FragmentDefinition,
	into map[string]bool,
) {
	for _, name := range spreads[from] {
		// A fragment already reached needs no second visit, which is also what
		// stops a cycle from spinning here.
		if into[name] {
			continue
		}
		into[name] = true
		if fragment := fragments[name]; fragment != nil {
			collectReachable(fragment, spreads, fragments, into)
		}
	}
}

// spreadsWithin returns the names of the fragments a definition spreads
// directly.
func spreadsWithin(node language.Node) []string {
	var names []string
	seen := map[string]bool{}
	language.Visit(node, language.Visitor{
		Enter: func(n language.Node, _ language.VisitContext) language.VisitAction {
			spread, isSpread := n.(*language.FragmentSpread)
			if !isSpread || spread.Name == nil || seen[spread.Name.Value] {
				return language.VisitContinue
			}
			seen[spread.Name.Value] = true
			names = append(names, spread.Name.Value)
			return language.VisitContinue
		},
	})
	return names
}
