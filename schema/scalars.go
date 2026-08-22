package schema

import (
	"encoding/json"
	"fmt"
	"github.com/ikawaha/graphql/gqlerror"
	"math"
	"strconv"
	"strings"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/value"
)

// The range of the Int type, which the specification fixes at a 32-bit signed
// integer regardless of what the host language can hold.
const (
	MaxInt = math.MaxInt32
	MinInt = math.MinInt32
)

// The two directions a scalar converts in are deliberately not symmetric.
//
// Coercing a value out of a resolver is lenient: a resolver may hand back
// whatever its data source gave it, and the scalar converts where the meaning
// is unambiguous. Coercing a value in from a caller is strict: a request that
// sends a string where an Int belongs is a mistake worth reporting rather than
// guessing at.

// Int is a 32-bit signed integer. Its internal representation is int32.
var Int = NewScalar(ScalarConfig{
	Name: "Int",
	Description: value.Just("The `Int` scalar type represents non-fractional signed whole " +
		"numeric values. Int can represent values between -(2^31) and 2^31 - 1."),
	CoerceOutputValue: func(v any) (value.Maybe[any], error) {
		switch held := v.(type) {
		case bool:
			if held {
				return value.Just[any](int32(1)), nil
			}
			return value.Just[any](int32(0)), nil
		case string:
			n, err := strconv.ParseFloat(strings.TrimSpace(held), 64)
			if err != nil || held == "" {
				return value.Nothing[any](), refuse("Int cannot represent non-integer value: %s", inspect(v))
			}
			return int32FromFloat(n, v)
		}
		num, ok := asNumber(v)
		if !ok {
			return value.Nothing[any](), refuse("Int cannot represent non-integer value: %s", inspect(v))
		}
		return num.toInt32()
	},
	CoerceInputValue: func(v any) (value.Maybe[any], error) {
		num, ok := asNumber(v)
		if !ok {
			return value.Nothing[any](), refuse("Int cannot represent non-integer value: %s", inspect(v))
		}
		return num.toInt32()
	},
	CoerceInputLiteral: func(literal language.Value) (value.Maybe[any], error) {
		lit, ok := literal.(*language.IntValue)
		if !ok {
			return value.Nothing[any](), refuseLiteral(literal, "Int cannot represent non-integer value: %s", language.Print(literal))
		}
		n, err := strconv.ParseInt(lit.Value, 10, 64)
		if err != nil || n > MaxInt || n < MinInt {
			return value.Nothing[any](), refuseLiteral(literal, "Int cannot represent non 32-bit signed integer value: %s", lit.Value)
		}
		return value.Just[any](int32(n)), nil
	},
	// Only a whole number that an Int can hold is written as one. A string of
	// digits is not: the input coercer would take it, but a literal has to say
	// what it is, and `"5"` in a document is a String.
	ValueToLiteral: func(external any, _ Type) (language.Value, error) {
		num, ok := asNumber(external)
		if !ok || !num.isInteger {
			return nil, refuse("Int cannot represent non-integer value: %s", inspect(external))
		}
		if num.exact != "" || num.i > MaxInt || num.i < MinInt {
			return nil, refuse("Int cannot represent non 32-bit signed integer value: %s", num.String())
		}
		return &language.IntValue{Value: num.String()}, nil
	},
})

// Float is a double-precision number. Its internal representation is float64.
var Float = NewScalar(ScalarConfig{
	Name: "Float",
	Description: value.Just("The `Float` scalar type represents signed double-precision " +
		"fractional values as specified by " +
		"[IEEE 754](https://en.wikipedia.org/wiki/IEEE_floating_point)."),
	CoerceOutputValue: func(v any) (value.Maybe[any], error) {
		switch held := v.(type) {
		case bool:
			if held {
				return value.Just[any](float64(1)), nil
			}
			return value.Just[any](float64(0)), nil
		case string:
			n, err := strconv.ParseFloat(strings.TrimSpace(held), 64)
			if err != nil || held == "" {
				return value.Nothing[any](), refuse("Float cannot represent non numeric value: %s", inspect(v))
			}
			return finiteFloat(n, v)
		}
		num, ok := asNumber(v)
		if !ok {
			return value.Nothing[any](), refuse("Float cannot represent non numeric value: %s", inspect(v))
		}
		return exactFloat(num, v)
	},
	CoerceInputValue: func(v any) (value.Maybe[any], error) {
		num, ok := asNumber(v)
		if !ok {
			return value.Nothing[any](), refuse("Float cannot represent non numeric value: %s", inspect(v))
		}
		return exactFloat(num, v)
	},
	CoerceInputLiteral: func(literal language.Value) (value.Maybe[any], error) {
		// An integer literal is a valid Float, which is why 1 may be written
		// where 1.0 is meant.
		var text string
		switch lit := literal.(type) {
		case *language.FloatValue:
			text = lit.Value
		case *language.IntValue:
			text = lit.Value
		default:
			return value.Nothing[any](), refuseLiteral(literal, "Float cannot represent non numeric value: %s", language.Print(literal))
		}
		n, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return value.Nothing[any](), refuseLiteral(literal, "Float cannot represent non numeric value: %s", text)
		}
		return value.Just[any](n), nil
	},
	// A whole number is written as one, as a document may: 1 is a valid Float
	// literal. What is refused is anything that is not a number at all.
	ValueToLiteral: keepKinds("Float", func(literal language.Value) bool {
		switch literal.(type) {
		case *language.FloatValue, *language.IntValue:
			return true
		}
		return false
	}),
})

// String is a sequence of characters. Its internal representation is string.
var String = NewScalar(ScalarConfig{
	Name: "String",
	Description: value.Just("The `String` scalar type represents textual data, represented " +
		"as UTF-8 character sequences. The String type is most often used by " +
		"GraphQL to represent free-form human-readable text."),
	CoerceOutputValue: func(v any) (value.Maybe[any], error) {
		switch held := v.(type) {
		case string:
			return value.Just[any](held), nil
		case bool:
			if held {
				return value.Just[any]("true"), nil
			}
			return value.Just[any]("false"), nil
		}
		if num, ok := asNumber(v); ok && !num.isNotFinite() {
			return value.Just[any](num.String()), nil
		}
		return value.Nothing[any](), refuse("String cannot represent value: %s", inspect(v))
	},
	CoerceInputValue: func(v any) (value.Maybe[any], error) {
		s, ok := v.(string)
		if !ok {
			return value.Nothing[any](), refuse("String cannot represent a non string value: %s", inspect(v))
		}
		return value.Just[any](s), nil
	},
	CoerceInputLiteral: func(literal language.Value) (value.Maybe[any], error) {
		lit, ok := literal.(*language.StringValue)
		if !ok {
			return value.Nothing[any](), refuseLiteral(literal, "String cannot represent a non string value: %s", language.Print(literal))
		}
		return value.Just[any](lit.Value), nil
	},
	ValueToLiteral: keepKinds("String", func(literal language.Value) bool {
		_, ok := literal.(*language.StringValue)
		return ok
	}),
})

// Boolean is true or false. Its internal representation is bool.
var Boolean = NewScalar(ScalarConfig{
	Name:        "Boolean",
	Description: value.Just("The `Boolean` scalar type represents `true` or `false`."),
	CoerceOutputValue: func(v any) (value.Maybe[any], error) {
		if b, ok := v.(bool); ok {
			return value.Just[any](b), nil
		}
		if num, ok := asNumber(v); ok && !num.isNotFinite() {
			return value.Just[any](num.float() != 0), nil
		}
		return value.Nothing[any](), refuse("Boolean cannot represent a non boolean value: %s", inspect(v))
	},
	CoerceInputValue: func(v any) (value.Maybe[any], error) {
		b, ok := v.(bool)
		if !ok {
			return value.Nothing[any](), refuse("Boolean cannot represent a non boolean value: %s", inspect(v))
		}
		return value.Just[any](b), nil
	},
	CoerceInputLiteral: func(literal language.Value) (value.Maybe[any], error) {
		lit, ok := literal.(*language.BooleanValue)
		if !ok {
			return value.Nothing[any](), refuseLiteral(literal, "Boolean cannot represent a non boolean value: %s", language.Print(literal))
		}
		return value.Just[any](lit.Value), nil
	},
	ValueToLiteral: keepKinds("Boolean", func(literal language.Value) bool {
		_, ok := literal.(*language.BooleanValue)
		return ok
	}),
})

// ID is a unique identifier. Its internal representation is string, even when
// it was supplied as a number, so that an identifier keeps whatever precision
// it arrived with.
var ID = NewScalar(ScalarConfig{
	Name: "ID",
	Description: value.Just("The `ID` scalar type represents a unique identifier, often " +
		"used to refetch an object or as key for a cache. The ID type appears " +
		"in a JSON response as a String; however, it is not intended to be " +
		"human-readable. When expected as an input type, any string (such as " +
		"`\"4\"`) or integer (such as `4`) input value will be accepted as an ID."),
	CoerceOutputValue: func(v any) (value.Maybe[any], error) {
		if s, ok := v.(string); ok {
			return value.Just[any](s), nil
		}
		if num, ok := asNumber(v); ok && num.isInteger {
			return value.Just[any](num.String()), nil
		}
		return value.Nothing[any](), refuse("ID cannot represent value: %s", inspect(v))
	},
	CoerceInputValue: func(v any) (value.Maybe[any], error) {
		if s, ok := v.(string); ok {
			return value.Just[any](s), nil
		}
		if num, ok := asNumber(v); ok && num.isInteger {
			return value.Just[any](num.String()), nil
		}
		return value.Nothing[any](), refuse("ID cannot represent value: %s", inspect(v))
	},
	CoerceInputLiteral: func(literal language.Value) (value.Maybe[any], error) {
		switch lit := literal.(type) {
		case *language.StringValue:
			return value.Just[any](lit.Value), nil
		case *language.IntValue:
			return value.Just[any](lit.Value), nil
		}
		return value.Nothing[any](), refuseLiteral(literal, "ID cannot represent a non-string and non-integer value: %s",
			language.Print(literal))
	},
	// An identifier made of digits is written as an integer, which is what a
	// document would have written and what reads back as the same ID.
	ValueToLiteral: func(external any, _ Type) (language.Value, error) {
		text, isText := external.(string)
		if !isText {
			num, isNumber := asNumber(external)
			if !isNumber || !num.isInteger {
				return nil, refuse("ID cannot represent value: %s", inspect(external))
			}
			text = num.String()
		}
		if isDigits(text) {
			return &language.IntValue{Value: text}, nil
		}
		return &language.StringValue{Value: text}, nil
	},
})

// keepKinds builds the [ValueToLiteral] the specified scalars other than Int
// and ID share: render the value by its Go shape, then keep the result only if
// it is the kind of literal this type accepts.
//
// It is what makes rendering type-aware. Rendering alone is type-blind — a Go
// string becomes a string literal whatever type it is being written for — so
// without the second half a String default would be accepted for a Boolean
// field and written into the schema as one. graphql-js draws the line in the
// same place, and for the same reason.
func keepKinds(name string, accepts func(language.Value) bool) ValueToLiteral {
	return func(external any, _ Type) (language.Value, error) {
		literal, ok := LiteralFromGoValue(external)
		if !ok || !accepts(literal) {
			return nil, refuse("%s cannot represent value: %s", name, inspect(external))
		}
		return literal, nil
	}
}

// isDigits reports whether a string is a whole number written out, which is
// what may be written as an integer rather than as a string.
func isDigits(text string) bool {
	digits := strings.TrimPrefix(text, "-")
	if digits == "" || (len(digits) > 1 && digits[0] == '0') {
		return false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// SpecifiedScalars are the scalars every schema has, in the order the
// specification introduces them.
var SpecifiedScalars = []*ScalarType{String, Int, Float, Boolean, ID}

// IsSpecifiedScalarType reports whether a type is one of the built-in scalars.
func IsSpecifiedScalarType(t Type) bool {
	s, ok := t.(*ScalarType)
	if !ok {
		return false
	}
	for _, specified := range SpecifiedScalars {
		if s == specified {
			return true
		}
	}
	return false
}

// number is a Go value known to hold a number, normalised so that the scalars
// above do not each have to enumerate every numeric type Go has.
//
// JavaScript has one number type, so the reference implementation can test for
// it in a line. Go has a dozen, plus json.Number, which is what a JSON decoder
// produces when asked to preserve the digits it was given. An identifier too
// large for a float64 arrives that way, and turning it into one would silently
// lose the end of it.
type number struct {
	i         int64
	f         float64
	isInteger bool
	// exact holds the digits as they arrived, when they came from a
	// json.Number that no Go integer type can hold.
	exact string
}

// isNotFinite reports whether the number is one IEEE 754 allows but a
// response cannot carry.
func (n number) isNotFinite() bool {
	return math.IsInf(n.f, 0) || math.IsNaN(n.f)
}

// float returns the value as a float64.
func (n number) float() float64 {
	if n.isInteger && n.exact == "" {
		return float64(n.i)
	}
	return n.f
}

// String renders the number without the trailing zeros a float would print.
func (n number) String() string {
	switch {
	case n.exact != "":
		return n.exact
	case n.isInteger:
		return strconv.FormatInt(n.i, 10)
	default:
		return value.Describe(n.f)
	}
}

// toInt32 narrows the number to the range the Int type allows.
func (n number) toInt32() (value.Maybe[any], error) {
	if !n.isInteger {
		// A whole number too large for an int64 is still a whole number: what
		// is wrong with it as an Int is its size, not its shape.
		if n.exact == "" && !n.isNotFinite() && n.f == math.Trunc(n.f) {
			return value.Nothing[any](), refuse(
				"Int cannot represent non 32-bit signed integer value: %s", n.String())
		}
		return value.Nothing[any](), refuse("Int cannot represent non-integer value: %s", n.String())
	}
	if n.exact != "" || n.i > MaxInt || n.i < MinInt {
		return value.Nothing[any](), refuse("Int cannot represent non 32-bit signed integer value: %s", n.String())
	}
	return value.Just[any](int32(n.i)), nil
}

// asNumber recognises any Go value that holds a number.
func asNumber(v any) (number, bool) {
	switch held := v.(type) {
	case int:
		return number{i: int64(held), isInteger: true}, true
	case int8:
		return number{i: int64(held), isInteger: true}, true
	case int16:
		return number{i: int64(held), isInteger: true}, true
	case int32:
		return number{i: int64(held), isInteger: true}, true
	case int64:
		return number{i: held, isInteger: true}, true
	case uint:
		return fromUint(uint64(held))
	case uint8:
		return number{i: int64(held), isInteger: true}, true
	case uint16:
		return number{i: int64(held), isInteger: true}, true
	case uint32:
		return number{i: int64(held), isInteger: true}, true
	case uint64:
		return fromUint(held)
	case float32:
		return fromFloat(float64(held))
	case float64:
		return fromFloat(held)
	case json.Number:
		return fromJSONNumber(held)
	default:
		return number{}, false
	}
}

// fromUint keeps an unsigned value that no int64 can hold as digits rather
// than losing its top bit.
func fromUint(v uint64) (number, bool) {
	if v > math.MaxInt64 {
		return number{isInteger: true, exact: strconv.FormatUint(v, 10), f: float64(v)}, true
	}
	return number{i: int64(v), isInteger: true}, true
}

// fromFloat treats a float with no fractional part as an integer, which is why
// a resolver may return 1.0 for an Int field.
func fromFloat(v float64) (number, bool) {
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return number{f: v}, true
	}
	if v == math.Trunc(v) && v >= math.MinInt64 && v <= math.MaxInt64 {
		return number{i: int64(v), f: v, isInteger: true}, true
	}
	return number{f: v}, true
}

// fromJSONNumber reads the digits a JSON decoder preserved, keeping them
// verbatim when they name an integer too large for an int64.
func fromJSONNumber(v json.Number) (number, bool) {
	text := v.String()
	if i, err := strconv.ParseInt(text, 10, 64); err == nil {
		return number{i: i, isInteger: true}, true
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return number{}, false
	}
	// The digits parse as a float but not as an int64, so an integer here is
	// one that overflowed.
	if !strings.ContainsAny(text, ".eE") {
		return number{isInteger: true, exact: text, f: f}, true
	}
	return number{f: f}, true
}

// int32FromFloat narrows a float that came from text.
func int32FromFloat(n float64, original any) (value.Maybe[any], error) {
	if n != math.Trunc(n) {
		return value.Nothing[any](), refuse("Int cannot represent non-integer value: %s", inspect(original))
	}
	if n > MaxInt || n < MinInt {
		return value.Nothing[any](), refuse("Int cannot represent non 32-bit signed integer value: %s", inspect(original))
	}
	return value.Just[any](int32(n)), nil
}

// finiteFloat rejects the values IEEE 754 allows but a response cannot carry.
func finiteFloat(n float64, original any) (value.Maybe[any], error) {
	if math.IsInf(n, 0) || math.IsNaN(n) {
		return value.Nothing[any](), refuse("Float cannot represent non numeric value: %s", inspect(original))
	}
	return value.Just[any](n), nil
}

// exactFloat is [finiteFloat] for a whole number, which may be too large to
// hold as a float64 without losing digits. Rounding it would answer with a
// number nobody asked for, so it is refused instead.
func exactFloat(num number, original any) (value.Maybe[any], error) {
	f := num.float()
	if num.isInteger && num.exact == "" && int64(f) != num.i {
		return value.Nothing[any](), refuse(
			"Float cannot represent non numeric value: %s (value would lose precision)",
			num.String())
	}
	if num.exact != "" {
		if back, err := strconv.ParseFloat(num.exact, 64); err != nil ||
			strconv.FormatFloat(back, 'f', -1, 64) != num.exact {
			return value.Nothing[any](), refuse(
				"Float cannot represent non numeric value: %s (value would lose precision)",
				num.String())
		}
	}
	return finiteFloat(f, original)
}

// inspect renders a value for a message, which is what a scalar refusing one
// has to do to say which value it refused.
func inspect(v any) string { return value.Describe(v) }

// refuse says a value does not fit, in the form a type's own complaint takes.
//
// A coercer's complaint is a GraphQL error rather than a plain one, and that
// is what says the type meant it: an error of any other kind is one the type
// did not intend as a message, so what reports it wraps it in one naming the
// type and the value. graphql-js draws the same line, between a thrown
// GraphQLError and a thrown Error.
func refuse(format string, args ...any) error {
	return gqlerror.Newf(format, args...)
}

// refuseLiteral is [refuse] for a literal, which unlike a value has a place in
// the document for the complaint to point at. graphql-js attaches the node the
// same way, and only where it is reading a literal.
func refuseLiteral(literal language.Value, format string, args ...any) error {
	return gqlerror.New(fmt.Sprintf(format, args...), gqlerror.WithNodes(literal))
}
