package validation

import (
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// KnownArgumentNamesRule reports an argument a field or directive does not
// take.
//
// An argument the server does not know is silently ignored at execution, so a
// misspelt one would change nothing and give no sign of why.
func KnownArgumentNamesRule(ctx *Context) language.Visitor {
	onDirectives := KnownArgumentNamesOnDirectivesRule(ctx)
	return language.Visitor{
		Enter: func(node language.Node, vc language.VisitContext) language.VisitAction {
			// Directive arguments are checked against the directive, which is
			// the same check SDL needs on its own.
			if action := onDirectives.Enter(node, vc); action != language.VisitContinue {
				return action
			}
			if spread, isSpread := node.(*language.FragmentSpread); isSpread {
				checkFragmentArguments(ctx, spread)
				return language.VisitContinue
			}
			arg, isArgument := node.(*language.Argument)
			if !isArgument || arg.Name == nil {
				return language.VisitContinue
			}
			// An argument inside a directive belongs to the directive.
			if _, inDirective := vc.Parent.(*language.Directive); inDirective {
				return language.VisitContinue
			}
			field, parent := ctx.FieldDef(), ctx.ParentType()
			if field == nil || parent == nil {
				return language.VisitContinue
			}
			name := arg.Name.Value
			if field.Arg(name) != nil {
				return language.VisitContinue
			}
			options := make([]string, 0, len(field.Args))
			for _, known := range field.Args {
				if known != nil {
					options = append(options, known.Name())
				}
			}
			ctx.Reportf([]language.Node{arg}, "Unknown argument %s on field %s.%s",
				quote(name), quote(parent.Name()+"."+field.Name()),
				ctx.DidYouMean("", schema.SuggestionList(name, options)))
			return language.VisitContinue
		},
	}
}

// KnownArgumentNamesOnDirectivesRule reports an argument a directive does not
// take. It is the part of [KnownArgumentNamesRule] that SDL needs as well,
// where there are no fields to check.
func KnownArgumentNamesOnDirectivesRule(ctx *Context) language.Visitor {
	args := map[string][]string{}
	directives := schema.SpecifiedDirectives
	if ctx.Schema() != nil {
		directives = ctx.Schema().Directives()
	}
	for _, d := range directives {
		if d == nil {
			continue
		}
		names := make([]string, 0, len(d.Args))
		for _, a := range d.Args {
			if a != nil {
				names = append(names, a.Name())
			}
		}
		args[d.Name()] = names
	}
	for _, def := range ctx.Document().Definitions {
		declaration, isDirective := def.(*language.DirectiveDefinition)
		if !isDirective || declaration.Name == nil {
			continue
		}
		names := make([]string, 0, len(declaration.Arguments))
		for _, a := range declaration.Arguments {
			if a != nil && a.Name != nil {
				names = append(names, a.Name.Value)
			}
		}
		args[declaration.Name.Value] = names
	}

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			directive, isDirective := node.(*language.Directive)
			if !isDirective || directive.Name == nil {
				return language.VisitContinue
			}
			// An unknown directive is a separate complaint.
			known, isKnown := args[directive.Name.Value]
			if !isKnown {
				return language.VisitContinue
			}
			takes := map[string]bool{}
			for _, name := range known {
				takes[name] = true
			}
			for _, arg := range directive.Arguments {
				if arg == nil || arg.Name == nil || takes[arg.Name.Value] {
					continue
				}
				ctx.Reportf([]language.Node{arg}, "Unknown argument %s on directive %s.%s",
					quote(arg.Name.Value), quote("@"+directive.Name.Value),
					ctx.DidYouMean("", schema.SuggestionList(arg.Name.Value, known)))
			}
			return language.VisitContinue
		},
	}
}

// checkFragmentArguments reports an argument a fragment does not declare.
//
// A fragment that takes arguments declares them the way an operation declares
// variables, and a spread supplies them the way a field supplies arguments.
// One the fragment never declared would be ignored, so a spread giving it is
// asking for something that will not happen.
func checkFragmentArguments(ctx *Context, spread *language.FragmentSpread) {
	if spread.Name == nil || len(spread.Arguments) == 0 {
		return
	}
	fragment := ctx.Fragment(spread.Name.Value)
	// A spread of a fragment the document does not define is a separate
	// complaint.
	if fragment == nil {
		return
	}

	declared := make(map[string]bool, len(fragment.VariableDefinitions))
	options := make([]string, 0, len(fragment.VariableDefinitions))
	for _, def := range fragment.VariableDefinitions {
		if def != nil && def.Variable != nil && def.Variable.Name != nil {
			declared[def.Variable.Name.Value] = true
			options = append(options, def.Variable.Name.Value)
		}
	}

	for _, arg := range spread.Arguments {
		if arg == nil || arg.Name == nil || declared[arg.Name.Value] {
			continue
		}
		ctx.Reportf([]language.Node{arg}, "Unknown argument %s on fragment %s.%s",
			quote(arg.Name.Value), quote(fragment.Name.Value),
			ctx.DidYouMean("", schema.SuggestionList(arg.Name.Value, options)))
	}
}
