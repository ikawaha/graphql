package value

import (
	"bytes"
	"encoding/json"
	"fmt"
	"iter"
	"reflect"
)

// OrderedMap is a JSON object that preserves the insertion order of its keys.
//
// The GraphQL specification requires the keys of the response data map to
// appear in the order the fields were requested. Go maps are unordered and
// encoding/json sorts keys alphabetically, so a plain map[string]any cannot
// satisfy the specification. Always use this type for objects in an execution
// result.
//
// The distinction between "omitted" and "null" follows the same rule as
// [Maybe]: an absent key means omitted, and a present key holding nil means
// GraphQL null.
//
//	v, ok := om.Get("x")
//	// ok == false          -> omitted
//	// ok == true, v == nil -> null
//	// ok == true, v != nil -> a value
//
// The zero value of OrderedMap is an empty map and is ready to use.
type OrderedMap struct {
	keys []string
	m    map[string]any
}

// NewOrderedMap returns an empty OrderedMap.
func NewOrderedMap() *OrderedMap {
	return &OrderedMap{}
}

// NewOrderedMapSize returns an empty OrderedMap with capacity for n entries.
// Use it when the number of collected fields is known in advance.
func NewOrderedMapSize(n int) *OrderedMap {
	return &OrderedMap{
		keys: make([]string, 0, n),
		m:    make(map[string]any, n),
	}
}

// Set assigns a value to a key. Setting an existing key updates the value in
// place without changing its position, matching the last-one-wins behaviour of
// duplicate keys in JSON.
func (o *OrderedMap) Set(key string, v any) {
	if o.m == nil {
		o.m = make(map[string]any)
	}
	if _, exists := o.m[key]; !exists {
		o.keys = append(o.keys, key)
	}
	o.m[key] = v
}

// Get returns the value and whether the key is present. A false ok means the
// key was omitted.
func (o *OrderedMap) Get(key string) (any, bool) {
	if o == nil || o.m == nil {
		return nil, false
	}
	v, ok := o.m[key]
	return v, ok
}

// Has reports whether the key is present. It returns true for a key whose
// value is nil, that is, GraphQL null.
func (o *OrderedMap) Has(key string) bool {
	_, ok := o.Get(key)
	return ok
}

// Delete removes a key. Deleting a key that is not present does nothing.
func (o *OrderedMap) Delete(key string) {
	if o == nil || o.m == nil {
		return
	}
	if _, ok := o.m[key]; !ok {
		return
	}
	delete(o.m, key)
	for i, k := range o.keys {
		if k == key {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			break
		}
	}
}

// Len returns the number of keys.
func (o *OrderedMap) Len() int {
	if o == nil {
		return 0
	}
	return len(o.keys)
}

// Keys returns the keys in insertion order. The returned slice is a copy, so
// modifying it does not affect the OrderedMap.
func (o *OrderedMap) Keys() []string {
	if o == nil {
		return nil
	}
	return append([]string(nil), o.keys...)
}

// All returns an iterator over the keys and values in insertion order.
//
//	for k, v := range om.All() {
//		...
//	}
func (o *OrderedMap) All() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		if o == nil {
			return
		}
		for _, k := range o.keys {
			if !yield(k, o.m[k]) {
				return
			}
		}
	}
}

// Equal reports whether two OrderedMaps are equal, including key order. It
// exists so that go-cmp can compare an OrderedMap despite its unexported
// fields.
func (o *OrderedMap) Equal(other *OrderedMap) bool {
	if o == nil || other == nil {
		return o.Len() == 0 && other.Len() == 0
	}
	if len(o.keys) != len(other.keys) {
		return false
	}
	for i, k := range o.keys {
		if other.keys[i] != k {
			return false
		}
		if !reflect.DeepEqual(o.m[k], other.m[k]) {
			return false
		}
	}
	return true
}

// MarshalJSON encodes the map as a JSON object, preserving insertion order.
func (o *OrderedMap) MarshalJSON() ([]byte, error) {
	if o == nil {
		return []byte("null"), nil
	}
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, fmt.Errorf("graphql/value: marshal key %q: %w", k, err)
		}
		buf.Write(kb)
		buf.WriteByte(':')
		vb, err := json.Marshal(o.m[k])
		if err != nil {
			return nil, fmt.Errorf("graphql/value: marshal value of %q: %w", k, err)
		}
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// UnmarshalJSON decodes a JSON object while preserving key order.
//
// Nested objects are restored as *OrderedMap and numbers are kept as
// json.Number so that precision is not lost. This is what lets differential
// tests against graphql-js compare responses including key order.
func (o *OrderedMap) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok == nil { // JSON null
		*o = OrderedMap{}
		return nil
	}
	d, ok := tok.(json.Delim)
	if !ok || d != '{' {
		return fmt.Errorf("graphql/value: cannot unmarshal %s into OrderedMap", data)
	}
	decoded, err := decodeObject(dec)
	if err != nil {
		return err
	}
	*o = *decoded
	return nil
}

// decodeObject reads from just past an opening brace through its matching
// closing brace.
func decodeObject(dec *json.Decoder) (*OrderedMap, error) {
	o := NewOrderedMap()
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, err
		}
		k, ok := kt.(string)
		if !ok {
			return nil, fmt.Errorf("graphql/value: object key is not a string: %v", kt)
		}
		v, err := decodeValue(dec)
		if err != nil {
			return nil, err
		}
		o.Set(k, v)
	}
	if _, err := dec.Token(); err != nil { // closing brace
		return nil, err
	}
	return o, nil
}

// decodeValue reads the next single value. Objects come back as *OrderedMap,
// arrays as []any, and numbers as json.Number.
func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return tok, nil // nil, bool, string or json.Number
	}
	switch d {
	case '{':
		return decodeObject(dec)
	case '[':
		arr := []any{}
		for dec.More() {
			v, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		if _, err := dec.Token(); err != nil { // closing bracket
			return nil, err
		}
		return arr, nil
	default:
		return nil, fmt.Errorf("graphql/value: unexpected delimiter %v", d)
	}
}
