package execution

import (
	"errors"

	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/internal/typeinfo"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// CoerceVariableValues turns the variables a request supplied into the form
// resolvers see, checking each against the type the document declared for it.
//
// A variable the caller left out is absent from the result, which is not the
// same as one given as null: the first falls back to a default where there is
// one, and the second does not. That distinction is the whole reason the
// request's variables arrive as a [value.Maybe] rather than a plain map.
//
// Every variable is reported on rather than stopping at the first, since a
// caller fixing a request would rather see all of it at once.
func CoerceVariableValues(
	s *schema.Schema,
	definitions []*language.VariableDefinition,
	supplied map[string]value.Maybe[any],
	opts ...schema.CheckOption,
) (schema.VariableValues, []*gqlerror.Error) {
	return coerceVariableValuesUpTo(-1, s, definitions, supplied, opts...)
}

// coerceVariableValuesUpTo is [CoerceVariableValues] with a bound on how many
// problems are reported. A bound of zero or less means there is none.
//
// Reaching the bound replaces what has been found with a single error saying
// so, as graphql-js does: a caller who asked for at most a few problems is
// being told the request has more than that, not handed the few that happened
// to come first.
func coerceVariableValuesUpTo(
	maxErrors int,
	s *schema.Schema,
	definitions []*language.VariableDefinition,
	supplied map[string]value.Maybe[any],
	opts ...schema.CheckOption,
) (schema.VariableValues, []*gqlerror.Error) {
	coerced := map[string]any{}
	// The type each variable was declared as travels with its value: writing
	// one back out as a literal, which is what a custom scalar taking a
	// complex literal is shown, needs to know an enum member from a string.
	declaredTypes := map[string]schema.Type{}
	var errs []*gqlerror.Error

	// add records problems one at a time, as graphql-js does, and answers
	// whether the bound has been reached. What was found before it was
	// reached is kept, with one last error saying there was more.
	add := func(found ...*gqlerror.Error) bool {
		for _, one := range found {
			// A negative bound is no bound, which is how a caller asks for
			// all of them; graphql-js passes Infinity.
			if maxErrors >= 0 && len(errs) >= maxErrors {
				errs = append(errs, gqlerror.New(
					"Too many errors processing variables, error limit reached. Execution aborted."))
				return true
			}
			errs = append(errs, one)
		}
		return false
	}

	for _, def := range definitions {
		if def == nil || def.Variable == nil || def.Variable.Name == nil {
			continue
		}
		name := def.Variable.Name.Value

		declared, known := typeinfo.TypeFromAST(s, def.Type)
		if !known || !schema.IsInputType(declared) {
			// Validation reports this too, but the executor may be handed a
			// document that was never validated.
			if add(gqlerror.New(
				"Variable "+quote("$"+name)+" expected value of type "+
					quote(language.Print(def.Type))+" which cannot be used as an input type.",
				gqlerror.WithNodes(def.Type))) {
				return schema.VariableValues{}, errs
			}
			continue
		}

		blamed := "Variable " + quote("$"+name)
		given, hasValue := supplied[name].Get()
		if !hasValue {
			// A default stands in for a variable the caller did not supply.
			if def.DefaultValue != nil {
				fallback, ok := schema.CoerceInputLiteral(def.DefaultValue, declared, schema.VariableValues{})
				if !ok {
					found := explain(blamed+" has invalid default value",
						literalReasons(def.DefaultValue, declared, schema.VariableValues{}, opts...), def)
					// The default itself may be sound, in which case what
					// would not coerce is a default written on a field of an
					// input object it holds.
					if len(found) == 0 {
						if why := schema.NestedDefaultFailure(declared); why != "" {
							found = []*gqlerror.Error{gqlerror.New(
								blamed+" has invalid default value: "+why, gqlerror.WithNodes(def))}
						}
					}
					if add(found...) {
						return schema.VariableValues{}, errs
					}
					continue
				}
				coerced[name] = fallback
				declaredTypes[name] = declared
				continue
			}
			// A nullable variable with no default and no value stays absent,
			// so that an argument it feeds falls back to its own default. A
			// non-null one cannot be left out, which is said the same way as
			// any other value that does not fit.
			if schema.IsNonNullType(declared) {
				if add(explain(blamed+" has invalid value",
					valueReasons(value.Nothing[any](), declared, opts...), def)...) {
					return schema.VariableValues{}, errs
				}
			}
			continue
		}

		converted, ok := schema.CoerceInputValue(given, declared)
		if !ok {
			// The fast path only says yes or no, so the slower one is run to
			// find out what to tell the caller.
			if add(explain(blamed+" has invalid value",
				valueReasons(value.Just(given), declared, opts...), def)...) {
				return schema.VariableValues{}, errs
			}
			continue
		}
		coerced[name] = converted
		declaredTypes[name] = declared
	}

	if len(errs) > 0 {
		return schema.VariableValues{}, errs
	}
	return schema.NewVariableValues(coerced, declaredTypes), nil
}

// argumentOwner names the field or directive an argument belongs to.
//
// It is held as what it is rather than as the finished piece of a message
// because the message is only ever needed where something is wrong. Written
// out on the way past, it would be built once per argument of every field of
// every object of every list, and thrown away every time.
type argumentOwner struct {
	field     *schema.Field
	directive *schema.Directive
}

// blame names one argument the way a message about it should.
func (o argumentOwner) blame(name string) string {
	owner := ""
	switch {
	case o.field != nil:
		owner = o.field.String()
	case o.directive != nil:
		owner = "@" + o.directive.Name()
	}
	return "Argument " + quote(owner+"("+name+":)")
}

// ArgumentValues returns the arguments a field was written with, coerced into
// the form a resolver receives.
//
// The executor does this for every field it resolves, so a server answering
// requests has no reason to call it. It is here for a caller doing the
// executor's job itself — walking a document against a schema, or working out
// what a field would be called with without running it.
//
// An argument left out falls back to its default, and so does one whose value
// is a variable the request omitted. A value that will not coerce is returned
// as an error rather than left out, which is the difference between this and
// what validation would have said about the same document.
//
// graphql-js calls this getArgumentValues, and throws where this returns.
func ArgumentValues(
	def *schema.Field,
	node *language.Field,
	variables schema.VariableValues,
) (schema.Arguments, *gqlerror.Error) {
	if def == nil || node == nil {
		return schema.Arguments{}, nil
	}
	return coerceArgumentValues(
		argumentOwner{field: def}, def.Args, node.Arguments, variables, node)
}

// DirectiveValues returns the arguments a directive was written with, and
// whether it was written at all.
//
// The directives are those of one node — a field's, a fragment spread's — and
// the definition says which of them is being asked about. Nothing written
// means no arguments and false, which is how @skip and @include tell "not
// there" from "there and false".
//
// A directive whose arguments will not coerce is reported as not written.
// Validation has already refused such a document, and for one that skipped
// validation this is the reading that leaves the response closest to what was
// asked for.
//
// graphql-js calls this getDirectiveValues.
func DirectiveValues(
	def *schema.Directive,
	directives []*language.Directive,
	variables schema.VariableValues,
) (schema.Arguments, bool) {
	return directiveValues(def, directives, variables)
}

// coerceArgumentValues turns the arguments written on a field or directive
// into the form a resolver sees.
//
// An argument left out falls back to its default, and one whose value is a
// variable the request omitted does the same: writing `f(arg: $x)` and not
// supplying `$x` asks for whatever `f` does without the argument.
func coerceArgumentValues(
	owner argumentOwner,
	definitions []*schema.Argument,
	written []*language.Argument,
	variables schema.VariableValues,
	blame language.Node,
	opts ...schema.CheckOption,
) (schema.Arguments, *gqlerror.Error) {
	if len(definitions) == 0 {
		return schema.Arguments{}, nil
	}
	byName := make(map[string]*language.Argument, len(written))
	for _, arg := range written {
		if arg != nil && arg.Name != nil {
			byName[arg.Name.Value] = arg
		}
	}

	coerced := make(map[string]any, len(definitions))
	for _, def := range definitions {
		if def == nil {
			continue
		}
		name := def.Name()
		node := byName[name]

		// A variable stands for whatever the request supplied, so an argument
		// written as one the request left out is the same as an argument that
		// was not written at all — unless it had to be there, in which case
		// what is wrong is said below along with everything else.
		omitted := node == nil
		if node != nil && !schema.IsRequiredArgument(def) {
			if variable, isVariable := node.Value.(*language.Variable); isVariable {
				_, supplied := variables.Get(nameOf(variable.Name))
				omitted = !supplied
			}
		}

		if omitted {
			if node == nil && schema.IsRequiredArgument(def) {
				return schema.Arguments{}, gqlerror.New(
					owner.blame(name)+" of required type "+quote(def.Type.String())+" was not provided.",
					gqlerror.WithNodes(blame))
			}
			fallback, ok := schema.CoerceDefaultInput(def.Default, def.Type)
			switch {
			case ok:
				coerced[name] = fallback
			case hasDefaultInput(def.Default):
				blamed := owner.blame(name) + " has invalid default value"
				if errs := explain(blamed, defaultReasons(def.Default, def.Type, opts...), blame); len(errs) > 0 {
					return schema.Arguments{}, errs[0]
				}
				// The default itself is sound, so what would not coerce is a
				// default written on a field of an input object it holds.
				if why := schema.NestedDefaultFailure(def.Type); why != "" {
					return schema.Arguments{}, gqlerror.New(blamed+": "+why, gqlerror.WithNodes(blame))
				}
			}
			continue
		}

		converted, ok := schema.CoerceInputLiteral(node.Value, def.Type, variables)
		if !ok {
			errs := explainWritten(owner.blame(name)+" has invalid value",
				literalReasons(node.Value, def.Type, variables, opts...))
			if len(errs) > 0 {
				return schema.Arguments{}, errs[0]
			}
			return schema.Arguments{}, gqlerror.New(
				owner.blame(name)+" has invalid value "+language.Print(node.Value)+".",
				gqlerror.WithNodes(node))
		}
		coerced[name] = converted
	}
	return schema.NewArguments(coerced), nil
}

// reason is one explanation of why a value does not fit a type, in the single
// shape the two halves of the checking are read through: one walks a value the
// caller supplied, the other a literal written in the document.
type reason struct {
	path    []any
	message string
	cause   error
	// node is the part of the document at fault, where the reason came from
	// reading one. A reason a type gave has none: a scalar refusing a value
	// says so in its own words and names no part of the document.
	node language.Node
}

// valueReasons explains why a value the caller supplied does not fit.
func valueReasons(supplied value.Maybe[any], t schema.Type, opts ...schema.CheckOption) []reason {
	found := schema.ValidateSuppliedInputValue(supplied, t, opts...)
	reasons := make([]reason, len(found))
	for i, why := range found {
		reasons[i] = reason{path: why.Path, message: why.Message, cause: why.Cause}
	}
	return reasons
}

// literalReasons explains why a literal written in the document does not fit.
func literalReasons(
	literal language.Value, t schema.Type, variables schema.VariableValues, opts ...schema.CheckOption,
) []reason {
	found := schema.ValidateInputLiteral(literal, t, variables, opts...)
	reasons := make([]reason, len(found))
	for i, why := range found {
		reasons[i] = reason{path: why.Path, message: why.Message, cause: why.Cause, node: why.Node}

		// A type that raised a GraphQL error has already had its say about
		// where it points, and where it named nowhere it is left that way, so
		// that the complaint comes to rest on the field. Any other error is
		// simply what a type returned while the literal was being read, and
		// the literal is what to point at.
		var raised *gqlerror.Error
		if why.Cause != nil && errors.As(why.Cause, &raised) {
			reasons[i].node = nil
			if len(raised.Nodes) > 0 {
				reasons[i].node = raised.Nodes[0]
			}
		}
	}
	return reasons
}

// defaultReasons explains why a default declared in the schema cannot be used.
// A default is either a literal or a value, and is checked as whichever it is.
func defaultReasons(
	def value.Maybe[schema.DefaultInput], t schema.Type, opts ...schema.CheckOption,
) []reason {
	input, has := def.Get()
	switch {
	case !has:
		return nil
	case input.Literal != nil:
		return literalReasons(input.Literal, t, schema.VariableValues{}, opts...)
	default:
		return valueReasons(value.Just(input.Value), t, opts...)
	}
}

// hasDefaultInput reports whether a default was declared at all, which is not
// the same as one that was declared and will not coerce.
func hasDefaultInput(def value.Maybe[schema.DefaultInput]) bool {
	_, has := def.Get()
	return has
}

// explain turns the reasons a value does not fit into errors, each saying who
// the value belongs to and where inside it the trouble is.
//
// Whatever a scalar attached to its refusal is carried over, so that a server
// naming its own error codes still sees them here.
func explain(blamed string, reasons []reason, blame language.Node) []*gqlerror.Error {
	return explainAt(blamed, reasons, func(reason) language.Node { return blame })
}

// explainWritten is [explain] for a literal written in the document, where the
// part of the literal at fault is a better place to point than the whole of
// it. A reason with nowhere to point is left without one, and the field it
// happened under stands in, which is where a reader is looking anyway.
func explainWritten(blamed string, reasons []reason) []*gqlerror.Error {
	return explainAt(blamed, reasons, func(why reason) language.Node { return why.node })
}

func explainAt(blamed string, reasons []reason, blame func(reason) language.Node) []*gqlerror.Error {
	errs := make([]*gqlerror.Error, 0, len(reasons))
	for _, why := range reasons {
		var opts []gqlerror.Option
		if at := blame(why); at != nil {
			opts = append(opts, gqlerror.WithNodes(at))
		}
		if why.cause != nil {
			opts = append(opts, gqlerror.WithCause(why.cause))
			var carried *gqlerror.Error
			if errors.As(why.cause, &carried) && carried.Extensions != nil {
				opts = append(opts, gqlerror.WithExtensions(carried.Extensions))
			}
		}
		errs = append(errs, gqlerror.New(blamed+where(why.path)+": "+why.message, opts...))
	}
	return errs
}

// directiveValues returns the arguments a directive was written with, and
// whether it was written at all.
func directiveValues(
	def *schema.Directive,
	directives []*language.Directive,
	variables schema.VariableValues,
) (schema.Arguments, bool) {
	if def == nil {
		return schema.Arguments{}, false
	}
	for _, written := range directives {
		if written == nil || written.Name == nil || written.Name.Value != def.Name() {
			continue
		}
		// A directive whose arguments will not coerce is reported by
		// validation; here it is treated as not written, which is the reading
		// that leaves the response closest to what was asked for.
		args, err := coerceArgumentValues(
			argumentOwner{directive: def}, def.Args, written.Arguments, variables, written)
		if err != nil {
			return schema.Arguments{}, false
		}
		return args, true
	}
	return schema.Arguments{}, false
}

// declaredDirective returns a schema's own definition of a directive.
//
// @defer and @stream are declared by the schema rather than being ones every
// schema has, so the schema's definition is the authority on what their
// arguments mean and what they default to. Falling back to the library's
// definition keeps a caller working against a schema that has none.
func declaredDirective(s *schema.Schema, fallback *schema.Directive) *schema.Directive {
	if declared := s.Directive(fallback.Name()); declared != nil {
		return declared
	}
	return fallback
}

// where renders the path inside a value that a complaint is about, or nothing
// when it is about the value as a whole.
func where(path []any) string {
	if len(path) == 0 {
		return ""
	}
	return " at " + formatValuePath(path)
}
