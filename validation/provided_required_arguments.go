package validation

import (
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// ProvidedRequiredArgumentsRule reports a required argument left out.
//
// An argument is required when its type is non-null and it has no default:
// there is then no value the server could use in its place.
func ProvidedRequiredArgumentsRule(ctx *Context) language.Visitor {
	onDirectives := ProvidedRequiredArgumentsOnDirectivesRule(ctx)
	return language.Visitor{
		Enter: func(node language.Node, vc language.VisitContext) language.VisitAction {
			return onDirectives.Enter(node, vc)
		},
		// The check runs on the way out, when every argument written on the
		// field has been seen.
		Leave: func(node language.Node, vc language.VisitContext) language.VisitAction {
			onDirectives.Leave(node, vc)

			if spread, isSpread := node.(*language.FragmentSpread); isSpread {
				checkFragmentSpreadArguments(ctx, spread)
				return language.VisitContinue
			}
			field, isField := node.(*language.Field)
			if !isField {
				return language.VisitContinue
			}
			def := ctx.FieldDef()
			if def == nil {
				return language.VisitContinue
			}
			given := map[string]bool{}
			for _, arg := range field.Arguments {
				if arg != nil && arg.Name != nil {
					given[arg.Name.Value] = true
				}
			}
			for _, arg := range def.Args {
				if arg == nil || given[arg.Name()] || !schema.IsRequiredArgument(arg) {
					continue
				}
				// The argument names the field it belongs to, as a schema
				// coordinate does.
				ctx.Reportf([]language.Node{field},
					"Argument %s of type %s is required, but it was not provided.",
					quote(arg.String()), quote(arg.Type.String()))
			}
			return language.VisitContinue
		},
	}
}

// ProvidedRequiredArgumentsOnDirectivesRule reports a required directive
// argument left out. It is the part of [ProvidedRequiredArgumentsRule] that
// SDL needs as well.
func ProvidedRequiredArgumentsOnDirectivesRule(ctx *Context) language.Visitor {
	// required maps a directive name to the arguments that must be given, and
	// how each is written, so that the message can name the type.
	type requirement struct{ name, typeName string }
	required := map[string][]requirement{}

	directives := schema.SpecifiedDirectives
	if ctx.Schema() != nil {
		directives = ctx.Schema().Directives()
	}
	for _, d := range directives {
		if d == nil {
			continue
		}
		var reqs []requirement
		for _, a := range d.Args {
			if a != nil && schema.IsRequiredArgument(a) {
				reqs = append(reqs, requirement{a.Name(), a.Type.String()})
			}
		}
		required[d.Name()] = reqs
	}
	for _, def := range ctx.Document().Definitions {
		declaration, isDirective := def.(*language.DirectiveDefinition)
		if !isDirective || declaration.Name == nil {
			continue
		}
		var reqs []requirement
		for _, a := range declaration.Arguments {
			// Written down, an argument is required when its type is non-null
			// and no default stands in for it.
			if a == nil || a.Name == nil || a.DefaultValue != nil {
				continue
			}
			if _, nonNull := a.Type.(*language.NonNullType); nonNull {
				reqs = append(reqs, requirement{a.Name.Value, language.Print(a.Type)})
			}
		}
		required[declaration.Name.Value] = reqs
	}

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			return language.VisitContinue
		},
		Leave: func(node language.Node, _ language.VisitContext) language.VisitAction {
			directive, isDirective := node.(*language.Directive)
			if !isDirective || directive.Name == nil {
				return language.VisitContinue
			}
			reqs, known := required[directive.Name.Value]
			if !known {
				return language.VisitContinue
			}
			given := map[string]bool{}
			for _, arg := range directive.Arguments {
				if arg != nil && arg.Name != nil {
					given[arg.Name.Value] = true
				}
			}
			for _, req := range reqs {
				if given[req.name] {
					continue
				}
				ctx.Reportf([]language.Node{directive},
					"Argument %s of type %s is required, but it was not provided.",
					quote("@"+directive.Name.Value+"("+req.name+":)"), quote(req.typeName))
			}
			return language.VisitContinue
		},
	}
}

// checkFragmentSpreadArguments reports a variable a fragment must be given
// that a spread of it leaves out.
//
// A fragment takes arguments the way an operation takes variables, and one
// whose type is non-null with no default has to be supplied: there is nothing
// the fragment could use in its place.
func checkFragmentSpreadArguments(ctx *Context, spread *language.FragmentSpread) {
	if spread.Name == nil {
		return
	}
	fragment := ctx.Fragment(spread.Name.Value)
	// A spread of a fragment the document does not define is a separate
	// complaint.
	if fragment == nil {
		return
	}

	given := make(map[string]bool, len(spread.Arguments))
	for _, arg := range spread.Arguments {
		if arg != nil && arg.Name != nil {
			given[arg.Name.Value] = true
		}
	}
	for _, declared := range fragment.VariableDefinitions {
		if declared == nil || declared.Variable == nil || declared.Variable.Name == nil {
			continue
		}
		name := declared.Variable.Name.Value
		if given[name] || declared.DefaultValue != nil {
			continue
		}
		if _, nonNull := declared.Type.(*language.NonNullType); !nonNull {
			continue
		}
		ctx.Reportf([]language.Node{spread},
			"Fragment %s argument %s of type %s is required, but it was not provided.",
			quote(fragment.Name.Value), quote(name), quote(language.Print(declared.Type)))
	}
}
