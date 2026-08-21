package schema

import "github.com/ikawaha/graphql/language"

// ReplaceVariables returns the literal with every variable in it replaced by
// what the request supplied, so that what comes back names no variables.
//
// It is what a scalar reading a literal of its own is given. A scalar that
// accepts a complex literal — a JSON scalar, a geometry, a filter language —
// would otherwise have to resolve variables itself, and would have to know
// that a variable the request left out is not the same as one given as null.
//
// A variable the request did not supply becomes null, except as the value of
// an input object's field, where the field is left out instead. That is the
// same distinction the rest of input coercion makes: a field that was not
// supplied falls back to its default, and one supplied as null does not.
//
// The variables are the coerced values, keyed by name, as the executor holds
// them: a variable the caller omitted is absent from the map, and one that
// had a default has already been given it. Where a fragment supplied
// arguments, the map is that fragment's scope.
//
// graphql-js keeps this in utilities. Here it belongs with the coercion it
// serves, for the reason [ValueFromASTUntyped] does: a type is what decides
// which values it accepts, and utilities may depend on types but not the
// other way about.
func ReplaceVariables(literal language.Value, variables VariableValues) language.Value {
	switch node := literal.(type) {
	case *language.Variable:
		supplied, was := variableValue(node, variables)
		if !was {
			return &language.NullValue{Loc: node.Loc}
		}
		// The type the variable was declared as decides how the value is
		// written, as it does in graphql-js: an enum member and a string are
		// the same Go value, and only the type says that $status belongs in
		// the literal as ACTIVE rather than as "ACTIVE".
		replaced, ok := literalFor(supplied, variables.TypeOf(node.Name.Value))
		if !ok {
			return &language.NullValue{Loc: node.Loc}
		}
		return replaced

	case *language.ListValue:
		var items []language.Value
		for i, item := range node.Values {
			replaced := ReplaceVariables(item, variables)
			if replaced == item && items == nil {
				continue
			}
			if items == nil {
				items = make([]language.Value, i, len(node.Values))
				copy(items, node.Values)
			}
			items = append(items, replaced)
		}
		if items == nil {
			return node
		}
		return &language.ListValue{Values: items, Loc: node.Loc}

	case *language.ObjectValue:
		fields := make([]*language.ObjectField, 0, len(node.Fields))
		changed := false
		for _, field := range node.Fields {
			if field == nil {
				changed = true
				continue
			}
			// A field whose value is a variable the request left out is left
			// out too, so that the field falls back to its default rather
			// than being read as null.
			if variable, isVariable := field.Value.(*language.Variable); isVariable {
				if _, was := variableValue(variable, variables); !was {
					changed = true
					continue
				}
			}
			replaced := ReplaceVariables(field.Value, variables)
			if replaced == field.Value {
				fields = append(fields, field)
				continue
			}
			changed = true
			fields = append(fields, &language.ObjectField{
				Name: field.Name, Value: replaced, Loc: field.Loc,
			})
		}
		if !changed {
			return node
		}
		return &language.ObjectValue{Fields: fields, Loc: node.Loc}

	default:
		return literal
	}
}

// variableValue reads what a variable stands for, and says whether the
// request supplied it at all.
func variableValue(node *language.Variable, variables VariableValues) (any, bool) {
	if node.Name == nil {
		return nil, false
	}
	supplied, was := variables.Get(node.Name.Value)
	return supplied, was
}

// literalFor writes a value back out as a literal, reading it against the type
// it was declared as where that is known and by what the value is where it is
// not.
func literalFor(v any, declared Type) (language.Value, bool) {
	if declared == nil {
		return LiteralFromGoValue(v)
	}
	return LiteralFromValue(v, declared)
}
