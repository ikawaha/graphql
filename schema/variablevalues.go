package schema

// VariableValues are the values a request supplied for its variables, together
// with the type each was declared as.
//
// The types matter when a value has to be written back out as a literal, which
// is what a custom scalar accepting a complex literal is shown. An enum member
// and a string are the same Go value; only the declared type says that
// `$status` should appear as `ACTIVE` rather than as `"ACTIVE"`. graphql-js
// carries the same pair, as the signature beside each variable's value.
//
// The zero value is usable and holds nothing, which is what a literal outside
// a request — a default written in a schema, say — is read against.
type VariableValues struct {
	values map[string]any
	types  map[string]Type
}

// NewVariableValues pairs coerced variable values with the types they were
// declared as. A name missing from types is still supplied; it is only written
// back out less exactly.
func NewVariableValues(values map[string]any, types map[string]Type) VariableValues {
	return VariableValues{values: values, types: types}
}

// Get returns what a variable was given, and whether it was given anything. A
// variable the caller left out is absent, which is not the same as one given
// as null.
func (v VariableValues) Get(name string) (any, bool) {
	if v.values == nil {
		return nil, false
	}
	held, was := v.values[name]
	return held, was
}

// TypeOf returns the type a variable was declared as, or nil where that is not
// known.
func (v VariableValues) TypeOf(name string) Type {
	if v.types == nil {
		return nil
	}
	return v.types[name]
}

// IsSet reports whether these are a scope in their own right rather than the
// absence of one. A request with no variables still has a scope; the zero
// value stands for not having been given one.
func (v VariableValues) IsSet() bool { return v.values != nil || v.types != nil }

// Values returns the values on their own, in the form a resolver sees them.
// The map is the one held, so a caller must not write to it.
func (v VariableValues) Values() map[string]any { return v.values }

// Len returns how many variables were supplied.
func (v VariableValues) Len() int { return len(v.values) }

// Clone returns a copy that can be added to and taken from without changing
// the original, which is what a fragment scope is built from.
func (v VariableValues) Clone() VariableValues {
	out := VariableValues{
		values: make(map[string]any, len(v.values)+1),
		types:  make(map[string]Type, len(v.types)+1),
	}
	for name, held := range v.values {
		out.values[name] = held
	}
	for name, declared := range v.types {
		out.types[name] = declared
	}
	return out
}

// Set records what a variable was given and the type it was declared as. It
// writes into the maps this holds, so it is for a copy of one's own.
func (v *VariableValues) Set(name string, held any, declared Type) {
	if v.values == nil {
		v.values = map[string]any{}
	}
	v.values[name] = held
	if declared == nil {
		delete(v.types, name)
		return
	}
	if v.types == nil {
		v.types = map[string]Type{}
	}
	v.types[name] = declared
}

// Delete forgets a variable, which is how a scope of its own keeps an outer
// variable of the same name from showing through.
func (v *VariableValues) Delete(name string) {
	delete(v.values, name)
	delete(v.types, name)
}
