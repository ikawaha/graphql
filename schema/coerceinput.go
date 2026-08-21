package schema

import (
	"reflect"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/value"
)

// CoerceInputValue converts a value supplied by a caller into the internal
// form a resolver receives.
//
// A false ok means the input does not fit the type. The reference
// implementation signals that by returning undefined, which is also how it
// spells "no value"; Go has no such value, so the two are separated here and
// the failure is returned alongside the result. Nothing is said about *why*
// the input did not fit: that costs more to work out and is only wanted when
// something has already gone wrong, so it is left to [ValidateInputValue].
//
// A nil result with ok true is GraphQL null, which is a value like any other.
func CoerceInputValue(input any, t Type) (result any, ok bool) {
	if nonNull, isNonNull := t.(*NonNull); isNonNull {
		if input == nil {
			// A null where the type forbids one.
			return nil, false
		}
		return CoerceInputValue(input, nonNull.OfType)
	}

	if input == nil {
		return nil, true
	}

	switch typ := t.(type) {
	case *List:
		return coerceList(input, typ)
	case *InputObjectType:
		return coerceInputObject(input, typ)
	case *ScalarType:
		coerced, err := typ.CoerceInputValue(input)
		if err != nil {
			return nil, false
		}
		// Nothing said is the type saying the value does not fit, which is
		// what a false ok says here.
		return coerced.Get()
	case *EnumType:
		return coerceEnum(input, typ)
	default:
		// Anything else cannot appear in an input position; the schema
		// validator reports that separately.
		return nil, false
	}
}

// coerceList converts a value for a list type.
func coerceList(input any, t *List) (any, bool) {
	items, isList := asList(input)
	if !isList {
		// A lone value is accepted where a list is wanted and becomes a list
		// of one, which is what lets a caller write a single item without
		// brackets.
		item, ok := CoerceInputValue(input, t.OfType)
		if !ok {
			return nil, false
		}
		return []any{item}, true
	}

	coerced := make([]any, 0, len(items))
	for _, item := range items {
		value, ok := CoerceInputValue(item, t.OfType)
		if !ok {
			return nil, false
		}
		coerced = append(coerced, value)
	}
	return coerced, true
}

// coerceInputObject converts a value for an input object type.
func coerceInputObject(input any, t *InputObjectType) (any, bool) {
	supplied, isObject := asObject(input)
	if !isObject {
		return nil, false
	}

	// A field the caller did not mention is absent from the map; one they set
	// to null is present holding nil. Only the first counts as "not supplied",
	// which is the distinction that decides whether a default applies.
	fields := t.Fields()
	known := make(map[string]bool, len(fields))
	for _, f := range fields {
		if f != nil {
			known[f.Name()] = true
		}
	}
	for name := range supplied {
		if !known[name] {
			// A field the type does not have.
			return nil, false
		}
	}

	coerced := make(map[string]any, len(fields))
	for _, f := range fields {
		if f == nil {
			continue
		}
		value, wasSupplied := supplied[f.Name()]
		if !wasSupplied {
			if IsRequiredInputField(f) {
				return nil, false
			}
			def, hasDef := CoerceDefaultInput(f.Default, f.Type)
			switch {
			case hasDef:
				coerced[f.Name()] = def
			case hasDefault(f.Default):
				// A default the field's own type will not take leaves no
				// value to stand in, so there is nothing to answer with. The
				// schema validator reports it; a schema that skipped the
				// validator finds out here.
				return nil, false
			}
			// With no default the field stays absent, which is different from
			// being present and null.
			continue
		}
		fieldValue, ok := CoerceInputValue(value, f.Type)
		if !ok {
			return nil, false
		}
		coerced[f.Name()] = fieldValue
	}

	if t.IsOneOf && !isValidOneOf(supplied, coerced) {
		return nil, false
	}
	return coerced, true
}

// isValidOneOf reports whether a value for a oneOf input object names exactly
// one field and gives it something other than null.
func isValidOneOf(supplied, coerced map[string]any) bool {
	if len(supplied) != 1 || len(coerced) != 1 {
		return false
	}
	for _, v := range coerced {
		if v == nil {
			return false
		}
	}
	return true
}

// coerceEnum converts a value for an enum type. A caller names a member; the
// resolver sees the member's internal value.
func coerceEnum(input any, t *EnumType) (any, bool) {
	name, isString := input.(string)
	if !isString {
		return nil, false
	}
	member := t.Value(name)
	if member == nil {
		return nil, false
	}
	return member.Value, true
}

// CoerceDefaultInput converts a default value into the internal form.
//
// A false ok means there was no default, or that the one there could not be
// converted. A schema whose default does not fit its own type is reported by
// the schema validator; here it simply does not apply.
func CoerceDefaultInput(def value.Maybe[DefaultInput], t Type) (any, bool) {
	input, has := def.Get()
	if !has {
		return nil, false
	}
	if input.Literal != nil {
		return CoerceInputLiteral(input.Literal, t, VariableValues{})
	}
	return CoerceInputValue(input.Value, t)
}

// asList reports whether a value is a list, and returns its items.
//
// A string is not a list, even though it is a sequence: writing a string where
// a list of strings is wanted means a list of one, not a list of characters.
func asList(v any) ([]any, bool) {
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

// asObject reports whether a value is a map of field names to values.
//
// A value that came from a request as JSON is an [value.OrderedMap], which
// knows the order its keys were written in. Coercion does not care about that
// order — it reads the fields the type declares — but a message naming the
// value does, so the ordered form is what the caller keeps hold of and this
// only borrows the fields from it.
func asObject(v any) (map[string]any, bool) {
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	if m, ok := v.(*value.OrderedMap); ok {
		if m == nil {
			return nil, false
		}
		out := make(map[string]any, m.Len())
		for k, held := range m.All() {
			out[k] = held
		}
		return out, true
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

// CoerceInputLiteral converts a literal written in a document into the
// internal form.
//
// variables supplies values for any variable the literal refers to. A variable
// that was not supplied makes the literal as a whole unusable, which is how a
// field left out of a request stays left out rather than becoming null.
func CoerceInputLiteral(literal language.Value, t Type, variables VariableValues) (any, bool) {
	if literal == nil {
		return nil, false
	}

	// A variable stands for whatever it was given, which has already been
	// converted, so it is substituted rather than converted again.
	if variable, isVariable := literal.(*language.Variable); isVariable {
		return resolveVariable(variable, t, variables)
	}

	if nonNull, isNonNull := t.(*NonNull); isNonNull {
		if _, isNull := literal.(*language.NullValue); isNull {
			return nil, false
		}
		return CoerceInputLiteral(literal, nonNull.OfType, variables)
	}

	if _, isNull := literal.(*language.NullValue); isNull {
		return nil, true
	}

	switch typ := t.(type) {
	case *List:
		return coerceListLiteral(literal, typ, variables)
	case *InputObjectType:
		return coerceInputObjectLiteral(literal, typ, variables)
	case *ScalarType:
		return coerceScalarLiteral(literal, typ, variables)
	case *EnumType:
		enumValue, isEnum := literal.(*language.EnumValue)
		if !isEnum {
			return nil, false
		}
		member := typ.Value(enumValue.Value)
		if member == nil {
			return nil, false
		}
		return member.Value, true
	default:
		return nil, false
	}
}

// resolveVariable substitutes a variable's value, which has already been
// converted for its declared type.
func resolveVariable(variable *language.Variable, t Type, variables VariableValues) (any, bool) {
	if variable.Name == nil {
		return nil, false
	}
	supplied, wasSupplied := variables.Get(variable.Name.Value)
	if !wasSupplied {
		// The caller left the variable out, so whatever holds this literal
		// must be left out too.
		return nil, false
	}
	if supplied == nil && IsNonNullType(t) {
		return nil, false
	}
	return supplied, true
}

// coerceListLiteral converts a literal for a list type.
func coerceListLiteral(literal language.Value, t *List, variables VariableValues) (any, bool) {
	list, isList := literal.(*language.ListValue)
	if !isList {
		// As with a runtime value, a lone item becomes a list of one.
		item, ok := CoerceInputLiteral(literal, t.OfType, variables)
		if !ok {
			return nil, false
		}
		return []any{item}, true
	}

	coerced := make([]any, 0, len(list.Values))
	for _, item := range list.Values {
		// An entry written as a variable the caller did not supply is a null
		// in the list rather than a hole: a list may hold null. Where its
		// entries may not be null there is nothing to put there, and the list
		// as a whole is unusable.
		if isMissingVariable(item, variables) {
			if IsNonNullType(t.OfType) {
				return nil, false
			}
			coerced = append(coerced, nil)
			continue
		}
		value, ok := CoerceInputLiteral(item, t.OfType, variables)
		if !ok {
			return nil, false
		}
		coerced = append(coerced, value)
	}
	return coerced, true
}

// coerceInputObjectLiteral converts a literal for an input object type.
func coerceInputObjectLiteral(literal language.Value, t *InputObjectType, variables VariableValues) (any, bool) {
	object, isObject := literal.(*language.ObjectValue)
	if !isObject {
		return nil, false
	}

	written := make(map[string]language.Value, len(object.Fields))
	for _, f := range object.Fields {
		if f == nil || f.Name == nil {
			return nil, false
		}
		written[f.Name.Value] = f.Value
	}

	fields := t.Fields()
	known := make(map[string]bool, len(fields))
	for _, f := range fields {
		if f != nil {
			known[f.Name()] = true
		}
	}
	for name := range written {
		if !known[name] {
			return nil, false
		}
	}

	coerced := make(map[string]any, len(fields))
	for _, f := range fields {
		if f == nil {
			continue
		}
		node, wasWritten := written[f.Name()]
		if !wasWritten {
			if IsRequiredInputField(f) {
				return nil, false
			}
			def, hasDef := CoerceDefaultInput(f.Default, f.Type)
			switch {
			case hasDef:
				coerced[f.Name()] = def
			case hasDefault(f.Default):
				return nil, false
			}
			continue
		}
		// A field written as a variable the caller left out counts as not
		// written at all, so its default applies.
		if isMissingVariable(node, variables) {
			if IsRequiredInputField(f) {
				return nil, false
			}
			def, hasDef := CoerceDefaultInput(f.Default, f.Type)
			switch {
			case hasDef:
				coerced[f.Name()] = def
			case hasDefault(f.Default):
				return nil, false
			}
			continue
		}
		fieldValue, ok := CoerceInputLiteral(node, f.Type, variables)
		if !ok {
			return nil, false
		}
		coerced[f.Name()] = fieldValue
	}

	if t.IsOneOf {
		// Both counts matter: writing a second field is asking for two values
		// even where one of them turns out to have no value to give.
		if len(written) != 1 || len(coerced) != 1 {
			return nil, false
		}
		for _, v := range coerced {
			if v == nil {
				return nil, false
			}
		}
	}
	return coerced, true
}

// isMissingVariable reports whether a literal is a variable the caller did not
// supply.
func isMissingVariable(literal language.Value, variables VariableValues) bool {
	variable, isVariable := literal.(*language.Variable)
	if !isVariable || variable.Name == nil {
		return false
	}
	_, wasSupplied := variables.Get(variable.Name.Value)
	return !wasSupplied
}

// coerceScalarLiteral converts a literal for a scalar type.
func coerceScalarLiteral(literal language.Value, t *ScalarType, variables VariableValues) (any, bool) {
	// A scalar that reads literals itself is given the node, so that a custom
	// scalar can accept a syntax the generic conversion would not.
	//
	// What it is given names no variables: they are replaced first, so that a
	// scalar accepting a complex literal does not have to resolve them — nor
	// to know that a variable the request left out is not the same as one
	// given as null. graphql-js does the same, and for the same reason.
	if t.CoerceInputLiteral != nil {
		coerced, err := t.CoerceInputLiteral(ReplaceVariables(literal, variables))
		if err != nil {
			return nil, false
		}
		// Nothing said is the scalar saying the literal does not fit.
		return coerced.Get()
	}
	// A scalar with nothing of its own is read the same way, variables
	// replaced first: what reaches the generic conversion is a constant, so a
	// literal naming a variable the request did not supply is read as that
	// variable being absent rather than being refused outright. graphql-js
	// reaches the same answer through its own default parseLiteral.
	plain, ok := ValueFromASTUntyped(ReplaceVariables(literal, variables), VariableValues{})
	if !ok {
		return nil, false
	}
	coerced, err := t.CoerceInputValue(plain)
	if err != nil {
		return nil, false
	}
	return coerced.Get()
}

// NestedDefaultFailure explains why a default written on a field of an input
// object inside t will not coerce, or answers with nothing when every default
// inside t does.
//
// It is the message graphql-js gives when applying such a default: the schema
// validator reports the same fault against the schema, and this is what an
// executor says about it when the schema was never validated.
func NestedDefaultFailure(t Type) string {
	return nestedDefaultFailure(t, map[*InputObjectType]bool{})
}

func nestedDefaultFailure(t Type, walked map[*InputObjectType]bool) string {
	in, isInputObject := NamedTypeOf(t).(*InputObjectType)
	if !isInputObject || walked[in] {
		return ""
	}
	walked[in] = true
	for _, f := range in.Fields() {
		if f == nil {
			continue
		}
		if _, hasDef := CoerceDefaultInput(f.Default, f.Type); !hasDef && hasDefault(f.Default) {
			written, _ := f.Default.Get()
			held := written.Value
			if written.Literal != nil {
				held = language.Print(written.Literal)
			}
			return "Expected value of type " + quote(f.Type.String()) +
				" to be valid, found: " + value.Describe(held) + "."
		}
		if why := nestedDefaultFailure(f.Type, walked); why != "" {
			return why
		}
	}
	return ""
}
