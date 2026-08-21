package schema

import (
	"encoding/json"
	"math"
	"reflect"
	"slices"
	"strconv"

	"github.com/ikawaha/graphql/language"
)

// LiteralFromValue renders a Go value as the GraphQL literal that stands for
// it, guided by the type it belongs to.
//
// It is the reverse of coercing input, and it is what lets a default value
// supplied in code be written into a schema or reported by introspection. The
// type matters: the same Go string is a quoted string for a String field and
// an unquoted member name for an enum, and only the type says which.
//
// A false ok means the value cannot be written as a literal of that type.
//
// This lives with the types rather than with the other value utilities because
// introspection needs it, and the utilities are built on top of this package
// rather than beside it.
func LiteralFromValue(v any, t Type) (language.Value, bool) {
	if nonNull, isNonNull := t.(*NonNull); isNonNull {
		if v == nil {
			return nil, false
		}
		return LiteralFromValue(v, nonNull.OfType)
	}
	if v == nil {
		return &language.NullValue{}, true
	}

	switch typ := t.(type) {
	case *List:
		return listLiteral(v, typ)
	case *InputObjectType:
		return inputObjectLiteral(v, typ)
	case *EnumType:
		// A member is written by name. What arrives may be the name already,
		// which is what a schema written as SDL holds, or the internal value a
		// schema built in Go gave the member.
		if name, isName := v.(string); isName && typ.Value(name) != nil {
			return &language.EnumValue{Value: name}, true
		}
		member := typ.ValueFor(v)
		if member == nil {
			return nil, false
		}
		return &language.EnumValue{Value: member.Name()}, true
	case *ScalarType:
		if typ.ValueToLiteral != nil {
			literal, err := typ.ValueToLiteral(v, typ)
			if err != nil || literal == nil {
				return nil, false
			}
			return literal, true
		}
		// Without a rendering of its own, the value is put through the
		// scalar's ordinary input conversion first. That is what makes the
		// result fit the type: rendering a Go value directly is type-blind and
		// would happily write a string where an Int belongs, producing a
		// literal the schema it lands in could not accept.
		coerced, err := typ.CoerceInputValue(v)
		if err != nil {
			return nil, false
		}
		held, fits := coerced.Get()
		if !fits {
			return nil, false
		}
		return LiteralFromGoValue(held)
	default:
		return nil, false
	}
}

// listLiteral renders a value as a list literal.
func listLiteral(v any, t *List) (language.Value, bool) {
	items, isList := literalItems(v)
	if !isList {
		// A lone value stands for a list of one on the way in, so on the way
		// out it is written as the bare value rather than wrapped.
		return LiteralFromValue(v, t.OfType)
	}
	values := make([]language.Value, 0, len(items))
	for _, item := range items {
		node, ok := LiteralFromValue(item, t.OfType)
		if !ok {
			return nil, false
		}
		values = append(values, node)
	}
	return &language.ListValue{Values: values}, true
}

// inputObjectLiteral renders a value as an input object literal.
func inputObjectLiteral(v any, t *InputObjectType) (language.Value, bool) {
	supplied, isObject := literalFields(v)
	if !isObject {
		return nil, false
	}

	known := make(map[string]bool, len(t.Fields()))
	for _, f := range t.Fields() {
		if f != nil {
			known[f.Name()] = true
		}
	}
	for name := range supplied {
		if !known[name] {
			return nil, false
		}
	}

	// Fields are written in the order the type declares them, so that the same
	// value always produces the same text.
	fields := make([]*language.ObjectField, 0, len(t.Fields()))
	for _, f := range t.Fields() {
		if f == nil {
			continue
		}
		fieldValue, present := supplied[f.Name()]
		if !present {
			if IsRequiredInputField(f) {
				return nil, false
			}
			continue
		}
		node, ok := LiteralFromValue(fieldValue, f.Type)
		if !ok {
			return nil, false
		}
		fields = append(fields, &language.ObjectField{
			Name:  &language.Name{Value: f.Name()},
			Value: node,
		})
	}
	return &language.ObjectValue{Fields: fields}, true
}

// LiteralFromGoValue renders a Go value as a literal without a type to guide
// it.
//
// It is what a scalar with no opinion of its own gets. Without a type there is
// no way to tell a string meant as an enum member from one meant as text, so
// everything textual becomes a string; that is right for a scalar, which is
// the only place this is reached.
func LiteralFromGoValue(v any) (language.Value, bool) {
	switch value := v.(type) {
	case nil:
		return &language.NullValue{}, true
	case bool:
		return &language.BooleanValue{Value: value}, true
	case string:
		return &language.StringValue{Value: value}, true
	case json.Number:
		return numberLiteral(value.String()), true
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &language.IntValue{Value: strconv.FormatInt(rv.Int(), 10)}, true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &language.IntValue{Value: strconv.FormatUint(rv.Uint(), 10)}, true
	case reflect.Float32, reflect.Float64:
		f := rv.Float()
		if math.IsInf(f, 0) || math.IsNaN(f) {
			// Neither can be written in a document, and null is the closest
			// thing to "no value here".
			return &language.NullValue{}, true
		}
		return numberLiteral(formatFloat(f)), true
	case reflect.Slice, reflect.Array:
		values := make([]language.Value, rv.Len())
		for i := range values {
			node, ok := LiteralFromGoValue(rv.Index(i).Interface())
			if !ok {
				return nil, false
			}
			values[i] = node
		}
		return &language.ListValue{Values: values}, true
	case reflect.Map:
		return mapLiteral(rv)
	default:
		return nil, false
	}
}

// formatFloat writes a number as a document would: plain digits, until they
// run far enough that an exponent is shorter. That is the rule JavaScript
// uses, and a schema printed here is read by the same tools.
func formatFloat(f float64) string {
	if size := math.Abs(f); f != 0 && (size < 1e-6 || size >= 1e21) {
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// numberLiteral decides whether digits name an integer or a floating point
// number, by whether writing them needed a point or an exponent.
func numberLiteral(text string) language.Value {
	if _, err := strconv.ParseInt(text, 10, 64); err == nil {
		return &language.IntValue{Value: text}
	}
	return &language.FloatValue{Value: text}
}

// mapLiteral renders a map as an object literal, with its fields in name order
// so that the same map always produces the same text.
func mapLiteral(rv reflect.Value) (language.Value, bool) {
	if rv.Type().Key().Kind() != reflect.String {
		return nil, false
	}
	names := make([]string, 0, rv.Len())
	for _, key := range rv.MapKeys() {
		names = append(names, key.String())
	}
	slices.Sort(names)

	fields := make([]*language.ObjectField, 0, len(names))
	for _, name := range names {
		node, ok := LiteralFromGoValue(rv.MapIndex(reflect.ValueOf(name)).Interface())
		if !ok {
			return nil, false
		}
		fields = append(fields, &language.ObjectField{
			Name:  &language.Name{Value: name},
			Value: node,
		})
	}
	return &language.ObjectValue{Fields: fields}, true
}

// literalItems reports whether a value is a list, and returns its items. A
// string is not a list, for the same reason it is not one on the way in.
func literalItems(v any) ([]any, bool) {
	if items, ok := v.([]any); ok {
		return items, true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		items := make([]any, rv.Len())
		for i := range items {
			items[i] = rv.Index(i).Interface()
		}
		return items, true
	default:
		return nil, false
	}
}

// literalFields reports whether a value is a map of field names to values.
func literalFields(v any) (map[string]any, bool) {
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Map || rv.Type().Key().Kind() != reflect.String {
		return nil, false
	}
	out := make(map[string]any, rv.Len())
	for _, key := range rv.MapKeys() {
		out[key.String()] = rv.MapIndex(key).Interface()
	}
	return out, true
}
