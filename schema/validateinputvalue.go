package schema

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/value"
)

// InputValueError describes one thing wrong with an input value.
type InputValueError struct {
	// Message says what is wrong.
	Message string
	// Path locates the problem inside the value, as field names and list
	// indices. It is empty when the value as a whole is at fault.
	Path []any
	// Cause is what a scalar or an enum said when it refused the value, where
	// one of them is what refused it. A caller reporting this can carry over
	// whatever the type attached to it.
	Cause error
}

// Error implements the error interface, naming the place along with the
// problem.
// Error says where the value went wrong and what is wrong with it.
//
// The message is worded as a GraphQL response words one, so it is given a line
// of its own rather than run on from the path: a sentence does not read well
// inside another.
func (e InputValueError) Error() string {
	if len(e.Path) == 0 {
		return e.Message
	}
	return "at " + formatPath(e.Path) + ":\n\t" + e.Message
}

// formatPath renders a path the way a person would write it.
func formatPath(path []any) string {
	var b strings.Builder
	for _, step := range path {
		switch v := step.(type) {
		case int:
			b.WriteByte('[')
			b.WriteString(strconv.Itoa(v))
			b.WriteByte(']')
		default:
			if b.Len() > 0 {
				b.WriteByte('.')
			}
			fmt.Fprint(&b, v)
		}
	}
	return b.String()
}

// ValidateInputValue explains why a value does not fit a type.
//
// It is the second half of a pair. [CoerceInputValue] answers quickly whether
// a value fits and produces the converted form; this walks the same ground
// more slowly to say what is wrong, and is only worth running once the fast
// answer has come back no. Splitting them keeps the cost of the common case
// down, since a request that is fine never needs the explanation.
//
// An empty result means the value fits.
func ValidateInputValue(input any, t Type, opts ...CheckOption) []InputValueError {
	return ValidateSuppliedInputValue(value.Just(input), t, opts...)
}

// ValidateSuppliedInputValue is [ValidateInputValue] for a value that may not
// have been supplied at all.
//
// A non-null type minds the difference: a value that was left out and one that
// was given as null are both refused, but for different reasons, and saying
// which helps whoever has to fix it.
func ValidateSuppliedInputValue(
	input value.Maybe[any], t Type, opts ...CheckOption,
) []InputValueError {
	v := &inputValidator{options: applyCheckOptions(opts)}
	v.walk(input, t, nil)
	return v.errors
}

// inputValidator gathers what is wrong with one value.
type inputValidator struct {
	errors  []InputValueError
	options checkOptions
}

func (v *inputValidator) report(path []any, format string, args ...any) {
	message := format
	if len(args) > 0 {
		message = fmt.Sprintf(format, args...)
	}
	v.errors = append(v.errors, InputValueError{
		Message: message,
		Path:    slices.Clone(path),
	})
}

// reportCause records what a type itself said about a value. What the type
// said is the message, since "Int cannot represent non-integer value: 1.5"
// tells whoever wrote it more than anything said from the outside.
func (v *inputValidator) reportCause(path []any, err error) {
	// A GraphQL error's own message, not its rendering: Error adds the
	// excerpt of the source it points at, which is how a complaint is shown
	// rather than what it says.
	said := err.Error()
	var raised *gqlerror.Error
	if errors.As(err, &raised) {
		said = raised.Message
	}
	v.errors = append(v.errors, InputValueError{
		Message: said,
		Path:    slices.Clone(path),
		Cause:   err,
	})
}

// walk checks a value against a type, recording anything wrong.
func (v *inputValidator) walk(input value.Maybe[any], t Type, path []any) {
	supplied, wasSupplied := input.Get()

	if nonNull, isNonNull := t.(*NonNull); isNonNull {
		switch {
		case !wasSupplied:
			v.report(path, "Expected a value of non-null type %s to be provided.", quote(nonNull.String()))
		case supplied == nil:
			v.report(path, "Expected value of non-null type %s not to be null.", quote(nonNull.String()))
		default:
			v.walk(input, nonNull.OfType, path)
		}
		return
	}

	// Anything nullable accepts both being left out and being null.
	if !wasSupplied || supplied == nil {
		return
	}

	switch typ := t.(type) {
	case *List:
		v.walkList(supplied, typ, path)
	case *InputObjectType:
		v.walkInputObject(supplied, typ, path)
	case *ScalarType:
		v.walkLeaf(supplied, typ, typ.CoerceInputValue, path)
	case *EnumType:
		v.walkLeaf(supplied, typ, enumValueCoercer(typ, v.options), path)
	default:
		v.report(path, "Expected %s to be an input type, which it is not.", describeTypeName(t))
	}
}

// walkList checks a value against a list type.
func (v *inputValidator) walkList(supplied any, t *List, path []any) {
	items, isList := asList(supplied)
	if !isList {
		// A lone value stands for a list of one, so it is checked against the
		// element type at the same place rather than under an index.
		v.walk(value.Just(supplied), t.OfType, path)
		return
	}
	for i, item := range items {
		v.walk(value.Just(item), t.OfType, append(path, i))
	}
}

// walkInputObject checks a value against an input object type.
func (v *inputValidator) walkInputObject(supplied any, t *InputObjectType, path []any) {
	fields, isObject := asObject(supplied)
	if !isObject {
		v.report(path, "Expected value of type %s to be an object, found: %s.",
			quote(t.Name()), value.Describe(supplied))
		return
	}

	known := make(map[string]bool, len(t.Fields()))
	for _, f := range t.Fields() {
		if f == nil {
			continue
		}
		known[f.Name()] = true

		fieldValue, present := fields[f.Name()]
		if !present {
			if IsRequiredInputField(f) {
				v.report(path, "Expected value of type %s to include required field %s, found: %s.",
					quote(t.Name()), quote(f.Name()), value.Describe(supplied))
			}
			continue
		}
		v.walk(value.Just(fieldValue), f.Type, append(path, f.Name()))
	}

	given := make([]string, 0, len(fields))
	for _, name := range fieldOrder(supplied, fields) {
		if known[name] {
			given = append(given, name)
			continue
		}
		options := make([]string, 0, len(t.Fields()))
		for _, f := range t.Fields() {
			if f != nil {
				options = append(options, f.Name())
			}
		}
		v.report(path, "Expected value of type %s not to include unknown field %s%s: %s.",
			quote(t.Name()), quote(name),
			foundAfter(v.options.didYouMean("", SuggestionList(name, options))),
			value.Describe(supplied))
	}

	if t.IsOneOf {
		v.checkOneOf(given, fields, t, path)
	}
}

// checkOneOf enforces that exactly one field is given and is not null.
func (v *inputValidator) checkOneOf(
	given []string,
	fields map[string]any,
	t *InputObjectType,
	path []any,
) {
	if len(given) != 1 {
		v.report(path, "%s", oneOfMessage(t))
		return
	}
	if name := given[0]; fields[name] == nil {
		v.report(append(path, name), "%s", oneOfMessage(t))
	}
}

// walkLeaf checks a value against a scalar or an enum, which is the one place
// the type itself decides.
func (v *inputValidator) walkLeaf(
	supplied any,
	t Type,
	coerce InputValueCoercer,
	path []any,
) {
	if coerce == nil {
		// A scalar with nothing to say accepts whatever the built-in coercion
		// makes of the value.
		if _, ok := CoerceInputValue(supplied, t); !ok {
			v.report(path, "Expected value of type %s, found: %s.", quote(t.String()), value.Describe(supplied))
		}
		return
	}
	// A type says no by returning an error. What it returns otherwise is the
	// value, and nil among those is GraphQL null, which is a value like any
	// other — the same reading [CoerceInputValue] takes.
	coerced, err := coerce(supplied)
	if err != nil {
		// A type that answered with a GraphQL error said what it meant, and
		// it carries its own extensions; what it said is passed on. Any other
		// error is one the type did not mean to make a message out of, so it
		// is wrapped in one naming the type and the value — which is exactly
		// how graphql-js tells a thrown GraphQLError from a thrown Error.
		var deliberate *gqlerror.Error
		if errors.As(err, &deliberate) {
			v.reportCause(path, err)
			return
		}
		v.errors = append(v.errors, InputValueError{
			Message: fmt.Sprintf("Expected value of type %s, but encountered error %s; found: %s.",
				quote(t.String()), strconv.Quote(err.Error()), value.Describe(supplied)),
			Path:  slices.Clone(path),
			Cause: err,
		})
		return
	}
	if !coerced.IsSet() {
		// The type answered with nothing, which says the value does not fit
		// without saying more. graphql-js reads a coercer's undefined the same
		// way, and makes the same complaint.
		v.report(path, "Expected value of type %s, found: %s.",
			quote(t.String()), value.Describe(supplied))
	}
}

// enumValueCoercer turns an enum into something that answers for a value the
// way a scalar does, so that both are checked the same way.
func enumValueCoercer(t *EnumType, o checkOptions) InputValueCoercer {
	return func(supplied any) (value.Maybe[any], error) {
		name, isString := supplied.(string)
		if !isString {
			return value.Nothing[any](), refuse("Enum %s cannot represent non-string value: %s.%s",
				quote(t.Name()), value.Describe(supplied),
				o.didYouMeanEnumValue(t, value.Describe(supplied)))
		}
		member := t.Value(name)
		if member == nil {
			return value.Nothing[any](), refuse("Value %s does not exist in %s enum.%s",
				quote(name), quote(t.Name()), o.didYouMeanEnumValue(t, name))
		}
		return value.Just(member.Value), nil
	}
}

// didYouMeanEnumValue suggests the members closest to what was written, or
// nothing where the caller asked for no suggestions.
func (o checkOptions) didYouMeanEnumValue(t *EnumType, written string) string {
	if o.hideSuggestions {
		return ""
	}
	return didYouMeanEnumValue(t, written)
}

// didYouMeanEnumValue suggests the members closest to what was written.
func didYouMeanEnumValue(t *EnumType, written string) string {
	options := make([]string, 0, len(t.Values()))
	for _, member := range t.Values() {
		if member != nil {
			options = append(options, member.Name())
		}
	}
	return DidYouMean("the enum value", SuggestionList(written, options))
}

// fieldOrder returns the field names of a supplied object in the order to read
// them in.
//
// A value that came from a request as JSON knows the order it was written in,
// and graphql-js reports what is wrong with an object in that order. A value
// assembled in Go as a map does not, so its names are read in name order:
// which key a Go map hands back first is not settled, and a message that
// differed from one run to the next would be worse than one that differs from
// graphql-js.
func fieldOrder(supplied any, fields map[string]any) []string {
	if ordered, isOrdered := supplied.(*value.OrderedMap); isOrdered && ordered != nil {
		return ordered.Keys()
	}
	return sortedKeys(fields)
}

// sortedKeys returns a map's keys in name order.
func sortedKeys(fields map[string]any) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// describeTypeName names a type for a message, coping with there being none.
func describeTypeName(t Type) string {
	if t == nil {
		return "nothing"
	}
	return t.String()
}
