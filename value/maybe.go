package value

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
)

// Maybe represents the three states a GraphQL input value can take:
//
//	omitted (undefined) : the zero value, equivalent to Nothing[T]()
//	null                : Just[any](nil)
//	a value             : Just(v)
//
// Because the zero value means "omitted", a field left out of a struct literal
// defaults to undefined, which matches the JavaScript behaviour this port
// mirrors.
//
// # Relationship to JSON
//
// UnmarshalJSON is only called when the key is present in the JSON input, so
// key presence maps directly onto whether the value was omitted. On the
// marshalling side, tag the field with omitzero to express omission (requires
// Go 1.24 or later; omitempty collapses null and omitted into one state).
//
//	type ExecutionResult struct {
//		Data   value.Maybe[any]  `json:"data,omitzero"`
//		Errors []*gqlerror.Error `json:"errors,omitzero"`
//	}
//
// Marshalling that struct renders the three states as:
//
//	Nothing        -> {}                 (the data key disappears entirely)
//	Just[any](nil) -> {"data":null}
//	Just[any](1)   -> {"data":1}
type Maybe[T any] struct {
	v   T
	set bool
}

// Just returns a Maybe holding a value. To represent GraphQL null, use the any
// type argument and pass nil, as in Just[any](nil).
func Just[T any](v T) Maybe[T] {
	return Maybe[T]{v: v, set: true}
}

// Nothing returns a Maybe representing an omitted (undefined) value. It is
// equivalent to the zero value of Maybe[T].
func Nothing[T any]() Maybe[T] {
	return Maybe[T]{}
}

// Get returns the value and whether it is present. A false ok means the value
// was omitted. Note that ok may be true while the value itself is nil, which
// represents GraphQL null.
func (m Maybe[T]) Get() (T, bool) {
	return m.v, m.set
}

// IsSet reports whether a value is present, that is, not omitted.
func (m Maybe[T]) IsSet() bool {
	return m.set
}

// IsZero reports whether the value was omitted. The encoding/json omitzero
// option consults this method.
func (m Maybe[T]) IsZero() bool {
	return !m.set
}

// Or returns the value if present, and def if it was omitted.
func (m Maybe[T]) Or(def T) T {
	if !m.set {
		return def
	}
	return m.v
}

// MustGet returns the value, panicking if it was omitted.
func (m Maybe[T]) MustGet() T {
	if !m.set {
		panic("graphql/value: MustGet called on an omitted Maybe")
	}
	return m.v
}

// Equal reports whether two Maybe values are equal, comparing the held values
// with reflect.DeepEqual. It exists so that go-cmp, which prefers an Equal
// method, can compare a Maybe despite its unexported fields.
func (m Maybe[T]) Equal(other Maybe[T]) bool {
	if m.set != other.set {
		return false
	}
	if !m.set {
		return true
	}
	return reflect.DeepEqual(m.v, other.v)
}

// String renders the value in a form that distinguishes all three states. It
// is intended for debugging.
func (m Maybe[T]) String() string {
	if !m.set {
		return "undefined"
	}
	if isNil(m.v) {
		return "null"
	}
	return fmt.Sprintf("%v", m.v)
}

// MarshalJSON encodes the held value as JSON.
//
// An omitted Maybe should not appear in JSON output at all, but that is the
// job of the omitzero struct tag. In contexts where omitzero does not apply,
// such as marshalling a Maybe on its own, this writes null, because Go's JSON
// encoder offers no way to emit nothing.
func (m Maybe[T]) MarshalJSON() ([]byte, error) {
	if !m.set {
		return []byte("null"), nil
	}
	return json.Marshal(m.v)
}

// UnmarshalJSON decodes JSON into the value.
//
// This method is only called when the corresponding key is present in the
// JSON, so being called at all establishes that the value was not omitted. A
// JSON null also reaches this method, leaving set true and the value at its
// zero value, which is nil when T is any.
//
// A number is read as a [encoding/json.Number] rather than as a float64, so
// that an integer larger than a float64 can hold arrives with its digits
// intact. That is what a request body decodes into — the variables of a
// GraphQL request are a map of these — and a scalar takes a json.Number
// wherever it takes an integer.
//
// Where T is any, an object is read as an [OrderedMap] rather than as a Go
// map, so that the order the request wrote its keys in survives. A message
// naming the value writes them back in that order, which is what a JavaScript
// object does for nothing and what a Go map cannot do at all.
func (m *Maybe[T]) UnmarshalJSON(data []byte) error {
	m.set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		var zero T
		m.v = zero
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if held, isAny := any(&m.v).(*any); isAny {
		decoded, err := decodeValue(decoder)
		if err != nil {
			return err
		}
		*held = decoded
		return nil
	}
	return decoder.Decode(&m.v)
}

// isNil reports whether a value stored in an any is nil, including typed nils.
func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface,
		reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
