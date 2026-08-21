package value_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ikawaha/graphql/value"
)

func TestDescribe(t *testing.T) {
	nested := map[string]any{"a": map[string]any{"b": map[string]any{"c": map[string]any{"d": 1}}}}

	tests := []struct {
		name string
		in   any
		want string
	}{
		{"nothing", nil, "null"},
		{"a string is quoted", "hi", `"hi"`},
		{"a string with a quote in it", `say "hi"`, `"say \"hi\""`},
		{"a whole number keeps no point", 1.0, "1"},
		{"a fraction keeps its point", 1.5, "1.5"},
		{"an integer", 42, "42"},
		{"a truth", true, "true"},
		{"a number decoded from JSON", json.Number("12345678901234567890"), "12345678901234567890"},
		{"an empty list", []any{}, "[]"},
		{"a list", []any{1, "two", nil}, `[1, "two", null]`},
		{"an empty object", map[string]any{}, "{}"},
		{"an object, by key", map[string]any{"b": 2, "a": 1}, "{ a: 1, b: 2 }"},
		{"a list past what is shown", []any{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
			"[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, ... 1 more item]"},
		{"a list well past what is shown", []any{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
			"[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, ... 2 more items]"},
		{"deeper than is worth showing", nested, "{ a: { b: [Object] } }"},
		{"a pointer is followed", func() any { s := "hi"; return &s }(), `"hi"`},
		{"a nil pointer is null", (*string)(nil), "null"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := value.Describe(tt.in); got != tt.want {
				t.Errorf("Describe(%v) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

// A response object knows the order its keys were written in, and shows them
// that way rather than sorted as a Go map has to be.
func TestDescribe_OrderedMap(t *testing.T) {
	m := value.NewOrderedMap()
	m.Set("b", 2)
	m.Set("a", 1)
	if got, want := value.Describe(m), "{ b: 2, a: 1 }"; got != want {
		t.Errorf("Describe = %s, want %s", got, want)
	}
}

// A value that contains itself is named rather than followed for ever.
func TestDescribe_ContainsItself(t *testing.T) {
	loop := map[string]any{}
	loop["self"] = loop
	if got, want := value.Describe(loop), "{ self: [Circular] }"; got != want {
		t.Errorf("Describe = %s, want %s", got, want)
	}
}

// A Go server usually has a struct where JavaScript would have an object, so
// a struct is described the way an object is. Only the exported fields are
// shown: the rest are not part of what the value is to anyone outside.
func TestDescribe_Struct(t *testing.T) {
	type inner struct{ B int }
	type outer struct {
		Name    string
		Count   int
		Nested  inner
		Pointer *inner
		hidden  string //nolint:unused // the point of the case
	}

	tests := []struct {
		name string
		in   any
		want string
	}{
		{
			name: "a struct",
			in:   outer{Name: "a", Count: 1, Nested: inner{B: 2}},
			want: `{ Name: "a", Count: 1, Nested: { B: 2 }, Pointer: null }`,
		},
		{
			name: "a pointer to one",
			in:   &inner{B: 3},
			want: "{ B: 3 }",
		},
		{
			name: "a struct with nothing anyone outside can see",
			in:   struct{ hidden int }{hidden: 1}, //nolint:unused // the point of the case
			want: "{}",
		},
		{
			name: "a struct with no fields at all",
			in:   struct{}{},
			want: "{}",
		},
		{
			name: "a struct inside a list",
			in:   []any{inner{B: 1}, inner{B: 2}},
			want: "[{ B: 1 }, { B: 2 }]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := value.Describe(tt.in); got != tt.want {
				t.Errorf("Describe() = %s, want %s", got, tt.want)
			}
		})
	}
}

// A struct that holds itself is described as far as it can be and then stops,
// the same as any other value that does.
func TestDescribe_StructContainsItself(t *testing.T) {
	type node struct {
		Name string
		Self *node
	}
	n := &node{Name: "a"}
	n.Self = n

	const want = `{ Name: "a", Self: [Circular] }`
	if got := value.Describe(n); got != want {
		t.Errorf("Describe() = %s, want %s", got, want)
	}
}

// A struct stands in for an object, so it is held to the same depth. Without
// this a chain of structs — a linked list, a tree of nodes — would be spelled
// out whole, and a resolver that returns one would turn a one-line error
// message into a megabyte of it.
func TestDescribe_StructDepth(t *testing.T) {
	type level3 struct{ D string }
	type level2 struct{ C level3 }
	type level1 struct{ B level2 }

	got := value.Describe(level1{B: level2{C: level3{D: "deep"}}})
	if want := "{ B: { C: [Object] } }"; got != want {
		t.Errorf("Describe() = %s, want %s", got, want)
	}
}

// A long chain is named rather than followed, however long it is.
func TestDescribe_ALongChainIsNotFollowed(t *testing.T) {
	type link struct {
		Name string
		Next *link
	}
	head := &link{Name: "head"}
	at := head
	for range 100_000 {
		at.Next = &link{Name: "x"}
		at = at.Next
	}

	got := value.Describe(head)
	if want := `{ Name: "head", Next: [Object] }`; got != want {
		t.Errorf("Describe() = %s, want %s", got, want)
	}
}

// jsonString writes itself as a JSON string, which is graphql-js's "use
// toJSON if provided".
type jsonString struct{}

func (jsonString) MarshalJSON() ([]byte, error) { return []byte(`"<json value>"`), nil }

// jsonObject writes itself as a JSON object, which is graphql-js's "handles
// toJSON returning object values".
type jsonObject struct{ Hidden string }

func (o jsonObject) MarshalJSON() ([]byte, error) {
	return []byte(`{"json":"value","count":3}`), nil
}

// jsonBroken cannot write itself, and so is shown the ordinary way.
type jsonBroken struct{ Field string }

func (jsonBroken) MarshalJSON() ([]byte, error) { return nil, errors.New("no") }

func describeNamedFunc() {}

func TestDescribe_Function(t *testing.T) {
	t.Parallel()
	if got, want := value.Describe(describeNamedFunc), "[function describeNamedFunc]"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// A function declared inside another has no name in JavaScript. Go gives
	// it one, and naming it is closer to graphql-js than showing an address
	// that differs from one run to the next.
	closure := func() {}
	if got := value.Describe(closure); !strings.HasPrefix(got, "[function ") {
		t.Errorf("got %q, want a named function", got)
	}
	var absent func()
	if got, want := value.Describe(absent), "null"; got != want {
		t.Errorf("nil func: got %q, want %q", got, want)
	}
}

func TestDescribe_WritesItselfAsJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"a string is shown as itself", jsonString{}, "<json value>"},
		{"an object is shown as an object", jsonObject{}, `{ count: 3, json: "value" }`},
		{"one that fails is shown the ordinary way", jsonBroken{Field: "x"}, `{ Field: "x" }`},
		{"a time is what it writes", time.Unix(0, 0).UTC(), "1970-01-01T00:00:00Z"},
		// The value that writes itself counts as one level, as it does in
		// graphql-js, so a list two deep inside it is named rather than shown.
		{"raw JSON is what it holds", json.RawMessage(`{"a":[1,2]}`), "{ a: [Array] }"},
		{"raw JSON one level shallower", json.RawMessage(`[1,2]`), "[1, 2]"},
		{"a number keeps its digits", json.RawMessage(`900719925474099267`), "900719925474099267"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := value.Describe(tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDescribe_NothingIsNull(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   any
	}{
		{"a map that is not there", map[string]any(nil)},
		{"a list that is not there", []any(nil)},
		{"a pointer that is not there", (*jsonObject)(nil)},
		{"a channel that is not there", (chan int)(nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got, want := value.Describe(tt.in), "null"; got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
	// An empty one is still empty, which is not the same thing.
	if got, want := value.Describe(map[string]any{}), "{}"; got != want {
		t.Errorf("empty map: got %q, want %q", got, want)
	}
	if got, want := value.Describe([]any{}), "[]"; got != want {
		t.Errorf("empty list: got %q, want %q", got, want)
	}
}

func TestDescribe_Channel(t *testing.T) {
	t.Parallel()
	// The address a channel holds would differ from one run to the next.
	if got, want := value.Describe(make(chan int)), "[chan]"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
