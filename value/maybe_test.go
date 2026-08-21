package value_test

import (
	"encoding/json"
	"testing"

	"github.com/ikawaha/graphql/value"
)

// state names the three states a Maybe can take. It exists only to make the
// test tables read clearly.
type state string

const (
	undefined state = "undefined"
	null      state = "null"
	hasValue  state = "value"
)

func stateOf[T any](m value.Maybe[T]) state {
	v, ok := m.Get()
	if !ok {
		return undefined
	}
	if any(v) == nil {
		return null
	}
	return hasValue
}

func TestMaybe_ThreeStates(t *testing.T) {
	tests := []struct {
		name  string
		m     value.Maybe[any]
		want  state
		isSet bool
	}{
		{"zero value is omitted", value.Maybe[any]{}, undefined, false},
		{"Nothing is omitted", value.Nothing[any](), undefined, false},
		{"Just(nil) is null", value.Just[any](nil), null, true},
		{"Just(value) is a value", value.Just[any](42), hasValue, true},
		{"Just(false) is a value", value.Just[any](false), hasValue, true},
		{"Just(empty string) is a value", value.Just[any](""), hasValue, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stateOf(tt.m); got != tt.want {
				t.Errorf("state = %v, want %v", got, tt.want)
			}
			if got := tt.m.IsSet(); got != tt.isSet {
				t.Errorf("IsSet() = %v, want %v", got, tt.isSet)
			}
			if got := tt.m.IsZero(); got != !tt.isSet {
				t.Errorf("IsZero() = %v, want %v", got, !tt.isSet)
			}
		})
	}
}

// The zero value meaning "omitted" is load bearing: a field left out of a
// struct literal must default to undefined, matching JavaScript.
func TestMaybe_ZeroValueIsOmitted(t *testing.T) {
	var args struct {
		A value.Maybe[any]
		B value.Maybe[any]
	}
	args.A = value.Just[any](1)
	// B is deliberately left unset.

	if !args.A.IsSet() {
		t.Error("explicitly set field A reports as omitted")
	}
	if args.B.IsSet() {
		t.Error("unset field B does not report as omitted")
	}
}

func TestMaybe_Or(t *testing.T) {
	if got := value.Nothing[int]().Or(7); got != 7 {
		t.Errorf("Or() = %v, want 7", got)
	}
	if got := value.Just(1).Or(7); got != 1 {
		t.Errorf("Or() = %v, want 1", got)
	}
	// null counts as present, so it must not be replaced by the default.
	if got := value.Just[any](nil).Or("default"); got != nil {
		t.Errorf("Or() on null = %v, want nil", got)
	}
}

func TestMaybe_MustGet(t *testing.T) {
	if got := value.Just(3).MustGet(); got != 3 {
		t.Errorf("MustGet() = %v, want 3", got)
	}
	defer func() {
		if recover() == nil {
			t.Error("MustGet() on an omitted Maybe did not panic")
		}
	}()
	_ = value.Nothing[int]().MustGet()
}

func TestMaybe_String(t *testing.T) {
	tests := []struct {
		m    value.Maybe[any]
		want string
	}{
		{value.Nothing[any](), "undefined"},
		{value.Just[any](nil), "null"},
		{value.Just[any](42), "42"},
		{value.Just[any]((*int)(nil)), "null"}, // a typed nil is still null
	}
	for _, tt := range tests {
		if got := tt.m.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}

func TestMaybe_Equal(t *testing.T) {
	tests := []struct {
		name string
		a, b value.Maybe[any]
		want bool
	}{
		{"omitted equals omitted", value.Nothing[any](), value.Nothing[any](), true},
		{"null equals null", value.Just[any](nil), value.Just[any](nil), true},
		{"value equals same value", value.Just[any](1), value.Just[any](1), true},
		{"omitted differs from null", value.Nothing[any](), value.Just[any](nil), false},
		{"null differs from zero value", value.Just[any](nil), value.Just[any](0), false},
		{"value differs from other value", value.Just[any](1), value.Just[any](2), false},
		{"uncomparable types do not panic", value.Just[any]([]int{1}), value.Just[any]([]int{1}), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equal(tt.b); got != tt.want {
				t.Errorf("Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}

// result mirrors an execution result. Per the specification data takes all
// three states: omitted for a request error, null when a field error
// propagates to the root, and a value on success.
type result struct {
	Data   value.Maybe[any] `json:"data,omitzero"`
	Errors []string         `json:"errors,omitzero"`
}

func TestMaybe_MarshalJSON_ThreeStates(t *testing.T) {
	tests := []struct {
		name string
		in   result
		want string
	}{
		{
			name: "data omitted, a request error",
			in:   result{Errors: []string{"boom"}},
			want: `{"errors":["boom"]}`,
		},
		{
			name: "data null, error propagated to the root",
			in:   result{Data: value.Just[any](nil), Errors: []string{"boom"}},
			want: `{"data":null,"errors":["boom"]}`,
		},
		{
			name: "data holds a value",
			in:   result{Data: value.Just[any](map[string]any{"x": 1})},
			want: `{"data":{"x":1}}`,
		},
		{
			name: "data holds a value and errors is omitted",
			in:   result{Data: value.Just[any]("ok")},
			want: `{"data":"ok"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Marshal() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestMaybe_UnmarshalJSON_ThreeStates(t *testing.T) {
	type vars struct {
		A value.Maybe[any] `json:"a,omitzero"`
		B value.Maybe[any] `json:"b,omitzero"`
		C value.Maybe[any] `json:"c,omitzero"`
	}
	var got vars
	// The key c is absent from the input entirely.
	if err := json.Unmarshal([]byte(`{"a":1,"b":null}`), &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if s := stateOf(got.A); s != hasValue {
		t.Errorf("a: state = %v, want %v", s, hasValue)
	}
	if s := stateOf(got.B); s != null {
		t.Errorf("b: state = %v, want %v", s, null)
	}
	if s := stateOf(got.C); s != undefined {
		t.Errorf("c: state = %v, want %v", s, undefined)
	}
}

func TestMaybe_JSONRoundTrip(t *testing.T) {
	for _, in := range []string{
		`{}`,
		`{"data":null}`,
		`{"data":"ok"}`,
		`{"data":{"x":1}}`,
		`{"data":null,"errors":["boom"]}`,
	} {
		t.Run(in, func(t *testing.T) {
			var r result
			if err := json.Unmarshal([]byte(in), &r); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			out, err := json.Marshal(r)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(out) != in {
				t.Errorf("round trip = %s, want %s", out, in)
			}
		})
	}
}

// When T is not any, a JSON null cannot be represented as a value and lands on
// the zero value. set stays true, so it is still distinguishable from omitted.
func TestMaybe_UnmarshalNull_NonAnyType(t *testing.T) {
	var s struct {
		N value.Maybe[int] `json:"n,omitzero"`
	}
	if err := json.Unmarshal([]byte(`{"n":null}`), &s); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	v, ok := s.N.Get()
	if !ok {
		t.Error("a Maybe that received null reports as omitted")
	}
	if v != 0 {
		t.Errorf("value = %v, want 0", v)
	}
}

func TestMaybe_UnmarshalJSON_Error(t *testing.T) {
	var s struct {
		N value.Maybe[int] `json:"n,omitzero"`
	}
	if err := json.Unmarshal([]byte(`{"n":"not a number"}`), &s); err == nil {
		t.Error("a type mismatch did not produce an error")
	}
}

// TestMaybe_DecodesNumbersExactly pins that a number arriving in a request
// body keeps its digits. The variables of a GraphQL request decode into a map
// of these, so this is the only place a caller could lose an integer larger
// than a float64 can hold, and a scalar takes a json.Number wherever it takes
// an integer.
func TestMaybe_DecodesNumbersExactly(t *testing.T) {
	var body struct {
		Variables map[string]value.Maybe[any] `json:"variables"`
	}
	const text = `{"variables":{"big":9007199254740993,"small":1,"float":1.5,` +
		`"nested":{"big":9007199254740993}}}`
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		t.Fatalf("reading the body: %v", err)
	}

	held, was := body.Variables["big"].Get()
	if !was {
		t.Fatal("the variable was read as omitted")
	}
	number, isNumber := held.(json.Number)
	if !isNumber {
		t.Fatalf("read as %T, want a json.Number", held)
	}
	if number.String() != "9007199254740993" {
		t.Errorf("read as %s, want 9007199254740993", number)
	}

	// Inside a value too. An object arrives as an ordered map, so that a
	// message naming it writes its keys back in the order the request did.
	nested, _ := body.Variables["nested"].Get()
	object, isOrdered := nested.(*value.OrderedMap)
	if !isOrdered {
		t.Fatalf("an object read as %T, want an *OrderedMap", nested)
	}
	inner, _ := object.Get("big")
	if got, isNumber := inner.(json.Number); !isNumber || got.String() != "9007199254740993" {
		t.Errorf("nested: read as %#v, want 9007199254740993", inner)
	}

	// A float is a number too, and reads back as it was written.
	held, _ = body.Variables["float"].Get()
	if got, isNumber := held.(json.Number); !isNumber || got.String() != "1.5" {
		t.Errorf("float: read as %#v, want 1.5", held)
	}
}
