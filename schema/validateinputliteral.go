package schema

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/value"
)

// LiteralError describes one thing wrong with a literal written in a document.
//
// It carries the node at fault as well as the path, so that a caller reporting
// it can point at the part of the document responsible rather than at the
// whole value.
type LiteralError struct {
	// Message says what is wrong.
	Message string
	// Node is the part of the document at fault: the literal itself, or the
	// field of an input object where the field is what is wrong.
	Node language.Node
	// Path locates the problem inside the value, as field names and list
	// indices. It is empty when the value as a whole is at fault.
	Path []any
	// Cause is what a scalar or an enum said when it refused the literal,
	// where one of them is what refused it. A caller reporting this can carry
	// over whatever the type attached to it.
	Cause error
}

// Error implements the error interface.
// Error says where the literal went wrong and what is wrong with it, worded as
// [InputValueError.Error] words it.
func (e LiteralError) Error() string {
	if len(e.Path) == 0 {
		return e.Message
	}
	return "at " + formatPath(e.Path) + ":\n\t" + e.Message
}

// ValidateInputLiteral explains why a literal written in a document does not
// fit a type.
//
// It is to [CoerceInputLiteral] what [ValidateInputValue] is to
// [CoerceInputValue]: the coercer answers quickly whether the literal fits,
// and this walks the same ground more slowly to say what is wrong. Where a
// scalar refuses a literal, the reason the scalar gave is what comes back,
// since "Int cannot represent non-integer value: 1.5" says more than "the
// value does not fit Int".
//
// A variable is not followed. What a variable holds is not known while a
// document is being checked, and whether it may stand where it is written is a
// separate question, settled by the rule for it. Passing variables lets the
// ones that are known be checked anyway, which is what a server does once a
// request has arrived.
//
// An empty result means the literal fits.
func ValidateInputLiteral(
	literal language.Value, t Type, variables VariableValues, opts ...CheckOption,
) []LiteralError {
	v := &literalValidator{
		variables:      variables,
		checkVariables: variables.IsSet(),
		options:        applyCheckOptions(opts),
	}
	v.walk(literal, t, nil)
	return v.errors
}

// literalValidator gathers what is wrong with one literal.
type literalValidator struct {
	errors    []LiteralError
	variables VariableValues
	// checkVariables says whether what a variable holds is known. While a
	// document is merely being checked it is not, and a variable is passed
	// over.
	checkVariables bool
	options        checkOptions
}

func (v *literalValidator) report(node language.Node, path []any, format string, args ...any) {
	v.errors = append(v.errors, LiteralError{
		Message: fmt.Sprintf(format, args...),
		Node:    node,
		Path:    append([]any(nil), path...),
	})
}

// walk checks a literal against a type.
func (v *literalValidator) walk(literal language.Value, t Type, path []any) {
	if literal == nil || t == nil {
		return
	}

	if variable, isVariable := literal.(*language.Variable); isVariable {
		v.checkVariable(variable, t, path)
		return
	}

	if nonNull, isNonNull := t.(*NonNull); isNonNull {
		if _, isNull := literal.(*language.NullValue); isNull {
			v.report(literal, path, "Expected value of non-null type %s not to be null.",
				quote(t.String()))
			return
		}
		v.walk(literal, nonNull.OfType, path)
		return
	}

	// Beneath a nullable type, null is always acceptable.
	if _, isNull := literal.(*language.NullValue); isNull {
		return
	}

	switch typed := t.(type) {
	case *List:
		v.walkList(literal, typed, path)
	case *InputObjectType:
		v.walkInputObject(literal, typed, path)
	default:
		v.walkLeaf(literal, t, path)
	}
}

// checkVariable reports a variable that cannot hold what the place it is
// written requires.
func (v *literalValidator) checkVariable(variable *language.Variable, t Type, path []any) {
	// Nothing can be said about a variable while the document is only being
	// checked; whether it may stand here at all is a separate question.
	if !v.checkVariables || !IsNonNullType(t) {
		return
	}
	name := ""
	if variable.Name != nil {
		name = variable.Name.Value
	}
	held, supplied := v.variables.Values()[name]
	switch {
	case !supplied:
		v.report(variable, path,
			"Expected variable %s provided to type %s to provide a runtime value.",
			quote("$"+name), quote(t.String()))
	case held == nil:
		v.report(variable, path,
			"Expected variable %s provided to non-null type %s not to be null.",
			quote("$"+name), quote(t.String()))
	}
}

// walkList checks the entries of a list.
//
// A single value stands for a list of one, which is what the specification
// calls list input coercion.
func (v *literalValidator) walkList(literal language.Value, t *List, path []any) {
	list, isList := literal.(*language.ListValue)
	if !isList {
		v.walk(literal, t.OfType, path)
		return
	}
	for i, entry := range list.Values {
		v.walk(entry, t.OfType, append(path, i))
	}
}

// walkInputObject checks the fields of an input object.
func (v *literalValidator) walkInputObject(literal language.Value, t *InputObjectType, path []any) {
	object, isObject := literal.(*language.ObjectValue)
	if !isObject {
		v.report(literal, path, "Expected value of type %s to be an object, found: %s.",
			quote(t.Name()), language.Print(literal))
		return
	}

	written := make(map[string]*language.ObjectField, len(object.Fields))
	for _, field := range object.Fields {
		if field != nil && field.Name != nil {
			written[field.Name.Value] = field
		}
	}

	for _, declared := range t.Fields() {
		if declared == nil {
			continue
		}
		field, given := written[declared.Name()]
		if !given {
			if IsRequiredInputField(declared) {
				v.report(literal, path,
					"Expected value of type %s to include required field %s, found: %s.",
					quote(t.Name()), quote(declared.Name()), language.Print(literal))
			}
			continue
		}
		if variable, isVariable := field.Value.(*language.Variable); isVariable && v.checkVariables {
			name := nameOf(variable.Name)
			held, supplied := v.variables.Values()[name]
			switch {
			case t.IsOneOf:
				// The one field a OneOf input object is given has to arrive
				// with a value, so a variable that brings none is a problem
				// here even though it would simply be an omission elsewhere.
				if !supplied {
					v.report(literal, path,
						"Expected variable %s provided to field %s for OneOf Input Object type %s to provide a runtime value.",
						quote("$"+name), quote(declared.Name()), quote(t.Name()))
				} else if held == nil {
					v.report(literal, path,
						"Expected variable %s provided to field %s for OneOf Input Object type %s not to be null.",
						quote("$"+name), quote(declared.Name()), quote(t.Name()))
				}
			case !supplied && !IsRequiredInputField(declared):
				// A field given a variable that was not supplied is the same
				// as one left out.
				continue
			}
		}
		v.walk(field.Value, declared.Type, append(path, declared.Name()))
	}

	// A field the type does not have would be ignored, so writing one is
	// asking for something that will not happen.
	var known []*language.ObjectField
	for _, field := range object.Fields {
		if field == nil || field.Name == nil {
			continue
		}
		if t.Field(field.Name.Value) != nil {
			known = append(known, field)
			continue
		}
		options := make([]string, 0, len(t.Fields()))
		for _, declared := range t.Fields() {
			if declared != nil {
				options = append(options, declared.Name())
			}
		}
		v.report(field, path,
			"Expected value of type %s not to include unknown field %s%s: %s.",
			quote(t.Name()), quote(field.Name.Value),
			foundAfter(v.options.didYouMean("", SuggestionList(field.Name.Value, options))),
			language.Print(literal))
	}

	if t.IsOneOf {
		v.checkOneOf(literal, t, known, path)
	}
}

// checkOneOf reports an input object that takes exactly one field being given
// some other number, or being given one written as null.
//
// A field written as a variable is settled while the fields are walked, since
// what the variable holds is what decides it.
func (v *literalValidator) checkOneOf(
	literal language.Value,
	t *InputObjectType,
	known []*language.ObjectField,
	path []any,
) {
	if len(known) != 1 {
		v.report(literal, path, "%s", oneOfMessage(t))
		return
	}
	if _, isNull := known[0].Value.(*language.NullValue); isNull {
		v.report(literal, append(path, nameOf(known[0].Name)), "%s", oneOfMessage(t))
	}
}

// oneOfMessage says what an input object that takes exactly one field asks for.
func oneOfMessage(t *InputObjectType) string {
	return fmt.Sprintf(
		"Within OneOf Input Object type %s, exactly one field must be specified, and the value for that field must be non-null.",
		quote(t.Name()))
}

// walkLeaf checks a literal against a scalar or an enum.
//
// The type itself decides what it accepts, and the reason it gives is what is
// reported: a scalar knows why 1.5 is not an Int in a way nothing outside it
// does.
func (v *literalValidator) walkLeaf(literal language.Value, t Type, path []any) {
	var coerce InputLiteralCoercer
	switch typed := t.(type) {
	case *ScalarType:
		coerce = typed.CoerceInputLiteral
	case *EnumType:
		coerce = enumLiteralCoercer(typed, v.options)
	default:
		v.report(literal, path, "Expected value of type %s, found: %s.",
			quote(t.String()), language.Print(literal))
		return
	}

	if coerce == nil {
		// A scalar with nothing to say accepts whatever the built-in coercion
		// makes of the literal.
		if _, ok := CoerceInputLiteral(literal, t, v.variables); !ok {
			v.report(literal, path, "Expected value of type %s, found: %s.",
				quote(t.String()), language.Print(literal))
		}
		return
	}

	// A type says no by returning an error, and what it said is more use than
	// saying the literal does not fit. What it returns otherwise is the value,
	// and nil among those is GraphQL null.
	coerced, err := coerce(literal)
	if err == nil {
		if !coerced.IsSet() {
			// Nothing said: the literal does not fit, and there is no more to
			// say about it.
			v.report(literal, path, "Expected value of type %s, found: %s.",
				quote(t.String()), language.Print(literal))
		}
		return
	}
	// As for a value: a GraphQL error is one the type meant, and anything
	// else is wrapped in a complaint naming the type and the literal.
	// A GraphQL error's own message is what is said; Error would add the
	// excerpt of the source it points at, which belongs to how a complaint is
	// shown rather than to what it says.
	said := err.Error()
	var deliberate *gqlerror.Error
	if errors.As(err, &deliberate) {
		said = deliberate.Message
	} else {
		said = fmt.Sprintf("Expected value of type %s, but encountered error %s; found: %s.",
			quote(t.String()), strconv.Quote(said), language.Print(literal))
	}
	v.errors = append(v.errors, LiteralError{
		Message: said,
		Node:    literal,
		Path:    append([]any(nil), path...),
		Cause:   err,
	})
}

// foundAfter joins a suggestion onto the tail of a message, which reads
// differently depending on whether there is one.
func foundAfter(suggestion string) string {
	if suggestion == "" {
		return ", found"
	}
	return "." + suggestion + " Found"
}

// quote wraps a name for a message.
func quote(s string) string { return strconv.Quote(s) }

// enumLiteralCoercer turns an enum into something that answers for a literal
// the way a scalar does, so that both are checked the same way.
func enumLiteralCoercer(t *EnumType, o checkOptions) InputLiteralCoercer {
	return func(literal language.Value) (value.Maybe[any], error) {
		written := language.Print(literal)
		named, isEnumValue := literal.(*language.EnumValue)
		if !isEnumValue {
			return value.Nothing[any](), refuseLiteral(literal, "Enum %s cannot represent non-enum value: %s.%s",
				quote(t.Name()), written, o.didYouMeanEnumValue(t, written))
		}
		member := t.Value(named.Value)
		if member == nil {
			return value.Nothing[any](), refuseLiteral(literal, "Value %s does not exist in %s enum.%s",
				quote(written), quote(t.Name()), o.didYouMeanEnumValue(t, written))
		}
		return value.Just(member.Value), nil
	}
}

// nameOf reads a name that may not be there, which a malformed document can
// leave out.
func nameOf(n *language.Name) string {
	if n == nil {
		return ""
	}
	return n.Value
}
