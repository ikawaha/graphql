package validation

import (
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// The four rules below govern @defer and @stream, which ask for parts of a
// response to arrive after the rest. They are experimental: a schema has them
// only if it opts in, and a document that never uses them is never touched by
// these rules.

// DeferStreamDirectiveOnRootFieldRule reports @defer or @stream at the root of
// a mutation or a subscription.
//
// A mutation's fields run in order and a subscription delivers one event at a
// time, so there is nothing for either to defer to.
//
// What counts as the root is settled by walking from the operation through
// fragments, not by reading the type a fragment is conditioned on: a fragment
// on an interface the root implements is still at the root.
func DeferStreamDirectiveOnRootFieldRule(ctx *Context) language.Visitor {
	fragments := map[string]*language.FragmentDefinition{}
	for _, def := range ctx.Document().Definitions {
		if fragment, isFragment := def.(*language.FragmentDefinition); isFragment && fragment.Name != nil {
			fragments[fragment.Name.Value] = fragment
		}
	}

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			operation, isOperation := node.(*language.OperationDefinition)
			if !isOperation {
				return language.VisitContinue
			}
			if operation.Operation != language.OperationMutation &&
				operation.Operation != language.OperationSubscription {
				return language.VisitContinue
			}
			root := ctx.Schema().RootType(operation.Operation)
			if root == nil {
				return language.VisitContinue
			}

			var walk func(set *language.SelectionSet, visited map[string]bool)
			walk = func(set *language.SelectionSet, visited map[string]bool) {
				if set == nil {
					return
				}
				for _, selection := range set.Selections {
					switch n := selection.(type) {
					case *language.Field:
						// A field below the root is not at the root, so only
						// the field itself is looked at.
						if stream := findDirective(n.Directives, schema.Stream.Name()); stream != nil {
							ctx.Reportf([]language.Node{stream},
								"Stream directive cannot be used on root %s type %s.",
								operation.Operation, quote(root.Name()))
						}
					case *language.FragmentSpread:
						name := nameOf(n.Name)
						if visited[name] {
							continue
						}
						visited[name] = true
						fragment := fragments[name]
						if fragment == nil {
							continue
						}
						if deferred := findDirective(n.Directives, schema.Defer.Name()); deferred != nil {
							ctx.Reportf([]language.Node{deferred},
								"Defer directive cannot be used on root %s type %s.",
								operation.Operation, quote(root.Name()))
						}
						walk(fragment.SelectionSet, visited)
					case *language.InlineFragment:
						if deferred := findDirective(n.Directives, schema.Defer.Name()); deferred != nil {
							ctx.Reportf([]language.Node{deferred},
								"Defer directive cannot be used on root %s type %s.",
								operation.Operation, quote(root.Name()))
						}
						walk(n.SelectionSet, visited)
					}
				}
			}
			walk(operation.SelectionSet, map[string]bool{})
			return language.VisitContinue
		},
	}
}

// StreamDirectiveOnListFieldRule reports @stream on a field that returns one
// value.
//
// Streaming delivers a list in pieces, so there has to be a list to break up.
func StreamDirectiveOnListFieldRule(ctx *Context) language.Visitor {
	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			directive, isDirective := node.(*language.Directive)
			if !isDirective || directive.Name == nil || directive.Name.Value != schema.Stream.Name() {
				return language.VisitContinue
			}
			field, parent := ctx.FieldDef(), ctx.ParentType()
			if field == nil || parent == nil {
				return language.VisitContinue
			}
			if schema.IsListType(schema.NullableTypeOf(field.Type)) {
				return language.VisitContinue
			}
			ctx.Reportf([]language.Node{directive},
				"Directive \"@stream\" cannot be used on non-list field %s.",
				quote(parent.Name()+"."+field.Name()))
			return language.VisitContinue
		},
	}
}

// DeferStreamDirectiveLabelRule reports a label that is not a plain string, or
// one used twice.
//
// A label names a piece of the response so the client can tell which arrived,
// which only works if it is settled before the request runs and picks out one
// piece.
func DeferStreamDirectiveLabelRule(ctx *Context) language.Visitor {
	known := map[string]*language.Directive{}
	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			directive, isDirective := node.(*language.Directive)
			if !isDirective || directive.Name == nil || deferOrStreamLabel(directive.Name.Value) == "" {
				return language.VisitContinue
			}
			var label language.Value
			for _, arg := range directive.Arguments {
				if arg != nil && arg.Name != nil && arg.Name.Value == "label" {
					label = arg.Value
				}
			}
			if label == nil {
				return language.VisitContinue
			}
			if _, isNull := label.(*language.NullValue); isNull {
				// Written as null, which is the same as not naming it.
				return language.VisitContinue
			}
			text, isString := label.(*language.StringValue)
			if !isString {
				// A variable would not be known until the request ran, by
				// which time the label is already needed.
				ctx.Reportf([]language.Node{directive}, "Argument %s must be a static string.",
					quote("@"+directive.Name.Value+"(label:)"))
				return language.VisitContinue
			}
			if first, taken := known[text.Value]; taken {
				ctx.Report(`Value for arguments "defer(label:)" and "stream(label:)" must be unique across all Defer/Stream directive usages.`,
					first, directive)
				return language.VisitContinue
			}
			known[text.Value] = directive
			return language.VisitContinue
		},
	}
}

// DeferStreamDirectiveOnValidOperationsRule reports @defer or @stream inside a
// subscription.
//
// A subscription delivers one event at a time and each has to be complete, so
// there is nothing to send afterwards. What is refused is a deferral that will
// certainly happen: one written `if: false`, or one whose `if` is a variable
// that might turn out false, is allowed, since a document may then serve as
// both a query and a subscription.
//
// A selection that might be skipped altogether is passed over: whatever it
// defers may never be asked for, so there is nothing to refuse.
func DeferStreamDirectiveOnValidOperationsRule(ctx *Context) language.Visitor {
	fragments := map[string]*language.FragmentDefinition{}
	for _, def := range ctx.Document().Definitions {
		if fragment, isFragment := def.(*language.FragmentDefinition); isFragment && fragment.Name != nil {
			fragments[fragment.Name.Value] = fragment
		}
	}

	// walk follows a subscription's selections, carrying the spreads it came
	// through so that an error can point at the whole route.
	var walk func(set *language.SelectionSet, through []language.Node, visited map[string]bool)
	walk = func(set *language.SelectionSet, through []language.Node, visited map[string]bool) {
		if set == nil {
			return
		}
		for _, selection := range set.Selections {
			directives := directivesOn(selection)

			// A selection that might be skipped defers nothing for certain.
			if skip := findDirective(directives, schema.Skip.Name()); skip != nil && mightSkip(skip) {
				continue
			}
			if include := findDirective(directives, schema.Include.Name()); include != nil && mightExclude(include) {
				continue
			}

			for _, directive := range directives {
				label := deferOrStreamLabel(nameOf(directive.Name))
				if label == "" || mightBeOff(directive) {
					continue
				}
				ctx.Reportf(append([]language.Node{directive}, through...),
					"%s directive not supported on subscription operations. Disable `@%s` by setting the `if` argument to `false`.",
					label, nameOf(directive.Name))
			}

			switch node := selection.(type) {
			case *language.FragmentSpread:
				name := nameOf(node.Name)
				// A fragment is followed once: a cycle is a separate
				// complaint, and following one here would not end.
				if visited[name] {
					continue
				}
				visited[name] = true
				if fragment := fragments[name]; fragment != nil {
					walk(fragment.SelectionSet, append([]language.Node{node}, through...), visited)
				}
			case *language.InlineFragment:
				walk(node.SelectionSet, through, visited)
			case *language.Field:
				walk(node.SelectionSet, through, visited)
			}
		}
	}

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			operation, isOperation := node.(*language.OperationDefinition)
			if !isOperation || operation.Operation != language.OperationSubscription {
				return language.VisitContinue
			}
			walk(operation.SelectionSet, nil, map[string]bool{})
			return language.VisitContinue
		},
	}
}

// mightSkip reports whether a @skip might leave its selection out.
//
// Only `if: false` says for certain that it will not; a variable might be
// anything, and a @skip with no `if` at all is a separate complaint, taken
// here as possibly skipping so that one mistake is not reported twice.
func mightSkip(skip *language.Directive) bool {
	given := findArgument(skip.Arguments, "if")
	if given == nil {
		return true
	}
	flag, isBoolean := given.Value.(*language.BooleanValue)
	if !isBoolean {
		return true
	}
	return flag.Value
}

// mightExclude reports whether an @include might leave its selection out.
//
// One with no `if` is a separate complaint, taken here as including, so that
// what it holds is still checked.
func mightExclude(include *language.Directive) bool {
	given := findArgument(include.Arguments, "if")
	if given == nil {
		return false
	}
	flag, isBoolean := given.Value.(*language.BooleanValue)
	if !isBoolean {
		return true
	}
	return !flag.Value
}

// mightBeOff reports whether a @defer or @stream might do nothing: written
// `if: false`, or given a variable that could turn out false.
func mightBeOff(directive *language.Directive) bool {
	given := findArgument(directive.Arguments, "if")
	if given == nil {
		return false
	}
	switch value := given.Value.(type) {
	case *language.BooleanValue:
		return !value.Value
	case *language.Variable:
		return true
	default:
		return false
	}
}

// findArgument returns an argument of the given name, or nil.
func findArgument(args []*language.Argument, name string) *language.Argument {
	for _, arg := range args {
		if arg != nil && arg.Name != nil && arg.Name.Value == name {
			return arg
		}
	}
	return nil
}

// findDirective returns a directive of the given name, or nil.
func findDirective(directives []*language.Directive, name string) *language.Directive {
	for _, d := range directives {
		if d != nil && d.Name != nil && d.Name.Value == name {
			return d
		}
	}
	return nil
}

// deferOrStreamLabel names the directive for a message, or returns the empty
// string for anything else.
func deferOrStreamLabel(name string) string {
	switch name {
	case schema.Defer.Name():
		return "Defer"
	case schema.Stream.Name():
		return "Stream"
	default:
		return ""
	}
}
