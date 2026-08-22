package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/utilities"
	"github.com/ikawaha/graphql/validation"
)

// TestContext_Directive covers what a rule is told about the directive the
// walk is inside, which nothing else here had asked for.
//
// It holds for the whole of a directive, not only its own node: a rule
// looking at an argument or at the value inside one is still inside the
// directive that argument belongs to. graphql-js's TypeInfo keeps it the same
// way, setting it on entering the directive and clearing it on leaving.
func TestContext_Directive(t *testing.T) {
	s, err := utilities.BuildSchema(`type Query { f: String }`)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := language.ParseString(`{ f @include(if: true) }`)
	if err != nil {
		t.Fatal(err)
	}

	// Where the walk says it is, for each node it passes.
	inside := map[string]string{}
	rule := func(ctx *validation.Context) language.Visitor {
		return language.Visitor{
			Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
				where := "(none)"
				if d := ctx.Directive(); d != nil {
					where = d.Name()
				}
				switch n := node.(type) {
				case *language.Directive:
					inside["the directive"] = where
				case *language.Argument:
					inside["its argument"] = where
				case *language.BooleanValue:
					inside["the value inside it"] = where
				case *language.Field:
					inside["the field it is on"] = where
				case *language.Document:
					inside["the document"] = where
				default:
					_ = n
				}
				return language.VisitContinue
			},
		}
	}
	if errs := validation.ValidateWithOptions(s, doc, validation.WithRules(rule)); len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}

	for _, want := range []struct{ at, directive string }{
		{"the document", "(none)"},
		{"the field it is on", "(none)"},
		{"the directive", "include"},
		{"its argument", "include"},
		{"the value inside it", "include"},
	} {
		if got := inside[want.at]; got != want.directive {
			t.Errorf("at %s the walk says @%s, want @%s", want.at, got, want.directive)
		}
	}
}
