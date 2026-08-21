package validation

import (
	"strings"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// SingleFieldSubscriptionsRule reports a subscription that selects more than
// one field at its root, or an introspection field there.
//
// A subscription is a stream of events, and each event is one thing that
// happened. Two root fields would mean two streams with no way to say how
// their events line up, and an introspection field describes the schema, which
// does not change from one event to the next.
//
// What counts as one field is settled the way execution settles it: fragments
// are followed and @skip and @include applied, so `{ a @skip(if: true) b }`
// selects one field rather than two.
func SingleFieldSubscriptionsRule(ctx *Context) language.Visitor {
	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			operation, isOperation := node.(*language.OperationDefinition)
			if !isOperation || operation.Operation != language.OperationSubscription {
				return language.VisitContinue
			}
			root := ctx.Schema().SubscriptionType()
			if root == nil {
				return language.VisitContinue
			}

			fragments := fragmentsOf(ctx.Document())

			// Whether a top level field is selected must not depend on a value
			// the request has not sent yet, so neither directive may be used
			// there at all.
			if forbidden := conditionalsAtTopLevel(operation, fragments); len(forbidden) > 0 {
				ctx.Reportf(forbidden,
					"%s must not use `@skip` or `@include` directives in the top level selection.",
					subscriptionLabel(operation))
			}

			// The document has not run, so no variables are known; a condition
			// that cannot be settled is read as not true.
			fields := execution.CollectFields(
				ctx.Schema(), fragments, schema.VariableValues{}, root, operation.SelectionSet)

			keys := fields.Keys()
			if len(keys) > 1 {
				// The first field is the one that could have been meant, so
				// the rest are what is blamed.
				var extra []language.Node
				for _, key := range keys[1:] {
					for _, field := range fields.Nodes(key) {
						extra = append(extra, field)
					}
				}
				ctx.Reportf(extra, "%s must select only one top level field.",
					subscriptionLabel(operation))
			}
			for _, key := range keys {
				selections := fields.Nodes(key)
				if !strings.HasPrefix(nameOf(selections[0].Name), "__") {
					continue
				}
				blamed := make([]language.Node, len(selections))
				for i, field := range selections {
					blamed[i] = field
				}
				ctx.Reportf(blamed, "%s must not select an introspection top level field.",
					subscriptionLabel(operation))
			}
			return language.VisitContinue
		},
	}
}

// conditionalsAtTopLevel returns the @skip and @include directives written on
// a subscription's top level selections, following fragments the way field
// collection does.
func conditionalsAtTopLevel(
	operation *language.OperationDefinition,
	fragments map[string]*language.FragmentDefinition,
) []language.Node {
	var found []language.Node
	visited := map[string]bool{}

	var walk func(set *language.SelectionSet)
	walk = func(set *language.SelectionSet) {
		if set == nil {
			return
		}
		for _, selection := range set.Selections {
			var directives []*language.Directive
			switch node := selection.(type) {
			case *language.Field:
				directives = node.Directives
			case *language.FragmentSpread:
				directives = node.Directives
			case *language.InlineFragment:
				directives = node.Directives
			}
			for _, d := range directives {
				if name := nameOf(d.Name); name == schema.Skip.Name() || name == schema.Include.Name() {
					found = append(found, d)
				}
			}

			// Only the top level is looked at, so a field's own selections are
			// not followed; a fragment spread there is still the top level.
			switch node := selection.(type) {
			case *language.FragmentSpread:
				name := nameOf(node.Name)
				if visited[name] {
					continue
				}
				visited[name] = true
				if fragment := fragments[name]; fragment != nil {
					walk(fragment.SelectionSet)
				}
			case *language.InlineFragment:
				walk(node.SelectionSet)
			}
		}
	}
	walk(operation.SelectionSet)
	return found
}

// subscriptionLabel names the operation for a message.
func subscriptionLabel(operation *language.OperationDefinition) string {
	if operation.Name == nil {
		return "Anonymous Subscription"
	}
	return "Subscription " + quote(operation.Name.Value)
}

// fragmentsOf indexes a document's fragments by name, which is the form field
// collection takes them in.
func fragmentsOf(doc *language.Document) map[string]*language.FragmentDefinition {
	fragments := map[string]*language.FragmentDefinition{}
	for _, def := range doc.Definitions {
		if fragment, isFragment := def.(*language.FragmentDefinition); isFragment && fragment.Name != nil {
			if _, taken := fragments[fragment.Name.Value]; !taken {
				fragments[fragment.Name.Value] = fragment
			}
		}
	}
	return fragments
}
