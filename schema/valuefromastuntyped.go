package schema

import (
	"encoding/json"
	"strconv"

	"github.com/ikawaha/graphql/language"
)

// ValueFromASTUntyped turns a literal into a plain Go value without consulting
// a type.
//
// It is what a scalar that has no opinion about literals receives: the literal
// is reduced to the obvious Go value and handed to the scalar's ordinary input
// conversion. Because no type is involved, an enum member and a string both
// come out as strings, and telling them apart is left to whatever knows the
// type.
//
// A false ok means the literal refers to a variable that was not supplied.
func ValueFromASTUntyped(literal language.Value, variables VariableValues) (any, bool) {
	switch node := literal.(type) {
	case nil:
		return nil, false

	case *language.NullValue:
		return nil, true
	case *language.BooleanValue:
		return node.Value, true
	case *language.StringValue:
		return node.Value, true
	case *language.EnumValue:
		return node.Value, true
	case *language.IntValue:
		return integerFromText(node.Value), true
	case *language.FloatValue:
		f, err := strconv.ParseFloat(node.Value, 64)
		if err != nil {
			return nil, false
		}
		return f, true

	case *language.ListValue:
		items := make([]any, 0, len(node.Values))
		for _, item := range node.Values {
			value, ok := ValueFromASTUntyped(item, variables)
			if !ok {
				return nil, false
			}
			items = append(items, value)
		}
		return items, true

	case *language.ObjectValue:
		fields := make(map[string]any, len(node.Fields))
		for _, f := range node.Fields {
			if f == nil || f.Name == nil {
				return nil, false
			}
			value, ok := ValueFromASTUntyped(f.Value, variables)
			if !ok {
				return nil, false
			}
			fields[f.Name.Value] = value
		}
		return fields, true

	case *language.Variable:
		if node.Name == nil {
			return nil, false
		}
		supplied, wasSupplied := variables.Get(node.Name.Value)
		return supplied, wasSupplied

	default:
		return nil, false
	}
}

// integerFromText reads the digits of an integer literal.
//
// Digits that no int64 can hold are kept as a json.Number rather than being
// widened to a float64, so that an identifier written out in full survives to
// whichever scalar receives it.
func integerFromText(text string) any {
	if n, err := strconv.ParseInt(text, 10, 64); err == nil {
		return n
	}
	return json.Number(text)
}
