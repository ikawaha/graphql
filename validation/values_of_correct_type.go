package validation

import (
	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// ValuesOfCorrectTypeRule reports a literal that cannot be a value of the type
// the place it is written expects.
//
// Every literal in a document sits somewhere with a declared type: an
// argument, an input field, a list element or a variable's default. Checking
// them here means execution can coerce a value knowing it will succeed.
//
// What a literal is checked against is [schema.ValidateInputLiteral], which
// is the same walk execution would do, so a type refuses a literal here for
// exactly the reason it would refuse it there — and says so in its own words.
func ValuesOfCorrectTypeRule(ctx *Context) language.Visitor {
	// Each kind of literal is checked where it sits and its children are not
	// walked again: the check descends into a list or an object itself, and
	// walking in as well would report the same problem twice.
	check := func(node language.Value, against schema.Type) language.VisitAction {
		if against == nil {
			return language.VisitSkip
		}
		for _, wrong := range schema.ValidateInputLiteral(node, against, schema.VariableValues{}, ctx.CheckOptions()...) {
			blamed := wrong.Node
			if blamed == nil {
				blamed = node
			}
			ctx.ReportError(gqlerror.New(wrong.Message, gqlerror.WithNodes(blamed)))
		}
		return language.VisitSkip
	}

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			switch n := node.(type) {
			case *language.ListValue:
				// The walk has already stepped into the element type, so it is
				// the type one level out that a list is judged against.
				return check(n, ctx.ParentInputType())
			case *language.ObjectValue:
				return check(n, ctx.InputType())
			case *language.NullValue:
				return check(n, ctx.InputType())
			case *language.EnumValue:
				return check(n, ctx.InputType())
			case *language.IntValue:
				return check(n, ctx.InputType())
			case *language.FloatValue:
				return check(n, ctx.InputType())
			case *language.BooleanValue:
				return check(n, ctx.InputType())
			case *language.StringValue:
				// A description is a string that would not pass this, and the
				// specification says descriptions must not affect validation.
				if isDescription(n, ctx) {
					return language.VisitContinue
				}
				return check(n, ctx.InputType())
			}
			return language.VisitContinue
		},
	}
}

// isDescription reports whether a string is documentation rather than a value.
//
// A description sits where no input type is expected, so having none is what
// tells them apart.
func isDescription(node *language.StringValue, ctx *Context) bool {
	return ctx.InputType() == nil
}
