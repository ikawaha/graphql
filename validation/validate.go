package validation

import (
	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/internal/typeinfo"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// Validate checks a document against a schema and returns what is wrong with
// it. An empty result means the document may be executed.
//
// Every rule sees the document in one walk rather than one walk each, so the
// cost is of the document's size rather than of its size times the number of
// rules.
//
// Validation is the step that lets execution assume its input is sound: a rule
// that reports nothing here is one execution never has to check for. Running a
// document that has not been validated is therefore not merely risky but
// outside what execution promises to cope with.
//
// With no rules given, [SpecifiedRules] are used, which are the rules the
// specification requires. Passing rules explicitly is how a server adds its
// own or leaves some out.
func Validate(s *schema.Schema, doc *language.Document, rules ...Rule) []*gqlerror.Error {
	if rules == nil {
		return ValidateWithOptions(s, doc)
	}
	return ValidateWithOptions(s, doc, WithRules(rules...))
}

// ValidateWithOptions is [Validate] with a say over how the check is run:
// which rules, how many problems to report, and whether messages may suggest
// what the document might have meant.
func ValidateWithOptions(
	s *schema.Schema, doc *language.Document, opts ...Option,
) []*gqlerror.Error {
	if doc == nil {
		return []*gqlerror.Error{gqlerror.New("Must provide document.")}
	}
	settings := apply(opts)
	rules := settings.rules
	if !settings.rulesGiven {
		rules = SpecifiedRules
	}
	ctx := NewContext(s, doc)
	ctx.maxErrors = settings.maxErrors
	ctx.hideSuggestions = settings.hideSuggestions
	run(ctx, doc, rules)
	return ctx.Errors()
}

// ValidateSDL checks a document of type definitions, which is the check that
// runs before a schema is built from it.
//
// The schema being extended may be nil, which is the case when the document
// stands on its own. Rules then have no types to check names against and say
// only what the document alone can settle.
func ValidateSDL(doc *language.Document, toExtend *schema.Schema, rules ...Rule) []*gqlerror.Error {
	if doc == nil {
		return []*gqlerror.Error{gqlerror.New("Must provide document.")}
	}
	if rules == nil {
		rules = SpecifiedSDLRules
	}
	ctx := NewContext(toExtend, doc)
	run(ctx, doc, rules)
	return ctx.Errors()
}

// run walks the document once, driving every rule and keeping the context's
// idea of where it is in step.
func run(ctx *Context, doc *language.Document, rules []Rule) {
	visitors := make([]language.Visitor, 0, len(rules))
	for _, rule := range rules {
		if rule == nil {
			continue
		}
		visitors = append(visitors, rule(ctx))
	}
	if len(visitors) == 0 {
		return
	}

	// TypeInfo wraps the rules rather than the other way round, so that it has
	// already moved by the time a rule asks where it is.
	walk := typeinfo.VisitWithTypeInfo(ctx.TypeInfo(), language.VisitInParallel(visitors...))

	language.Visit(doc, language.Visitor{
		Enter: func(node language.Node, vc language.VisitContext) language.VisitAction {
			action := walk.Enter(node, vc)
			if ctx.full {
				// Enough has been reported that going on would only add to a
				// list nobody will read to the end.
				return language.VisitBreak
			}
			return action
		},
		Leave: func(node language.Node, vc language.VisitContext) language.VisitAction {
			return walk.Leave(node, vc)
		},
	})
}
