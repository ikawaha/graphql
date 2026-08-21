package value_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/ikawaha/graphql/value"
)

func newFrom(t *testing.T, pairs ...any) *value.OrderedMap {
	t.Helper()
	if len(pairs)%2 != 0 {
		t.Fatalf("newFrom: got %d arguments, want an even number", len(pairs))
	}
	om := value.NewOrderedMap()
	for i := 0; i < len(pairs); i += 2 {
		k, ok := pairs[i].(string)
		if !ok {
			t.Fatalf("newFrom: key at %d is %T, want string", i, pairs[i])
		}
		om.Set(k, pairs[i+1])
	}
	return om
}

func TestOrderedMap_ZeroValueIsUsable(t *testing.T) {
	var om value.OrderedMap
	om.Set("a", 1)
	if got := om.Len(); got != 1 {
		t.Errorf("Len() = %d, want 1", got)
	}
	if v, ok := om.Get("a"); !ok || v != 1 {
		t.Errorf("Get(a) = %v, %v, want 1, true", v, ok)
	}
}

// Preserving insertion order is the whole reason this type exists: the
// specification requires response keys to follow the order of the query, and a
// plain Go map cannot express that.
func TestOrderedMap_PreservesInsertionOrder(t *testing.T) {
	// Deliberately not alphabetical, so a plain map would reorder these.
	keys := []string{"zebra", "alpha", "middle", "beta"}
	om := value.NewOrderedMap()
	for i, k := range keys {
		om.Set(k, i)
	}
	if got := om.Keys(); !slices.Equal(got, keys) {
		t.Errorf("Keys() = %v, want %v", got, keys)
	}

	var iterated []string
	for k := range om.All() {
		iterated = append(iterated, k)
	}
	if !slices.Equal(iterated, keys) {
		t.Errorf("All() yielded %v, want %v", iterated, keys)
	}
}

func TestOrderedMap_SetExistingKeyKeepsPosition(t *testing.T) {
	om := newFrom(t, "a", 1, "b", 2, "c", 3)
	om.Set("a", 99)

	if got, want := om.Keys(), []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Errorf("Keys() = %v, want %v", got, want)
	}
	if v, _ := om.Get("a"); v != 99 {
		t.Errorf("Get(a) = %v, want 99", v)
	}
	if got := om.Len(); got != 3 {
		t.Errorf("Len() = %d, want 3", got)
	}
}

// Absent key means omitted, present key holding nil means GraphQL null. This
// mirrors the rule Maybe follows.
func TestOrderedMap_OmittedVersusNull(t *testing.T) {
	om := newFrom(t, "explicitNull", nil)

	if v, ok := om.Get("explicitNull"); !ok || v != nil {
		t.Errorf("Get(explicitNull) = %v, %v, want nil, true", v, ok)
	}
	if !om.Has("explicitNull") {
		t.Error("Has(explicitNull) = false, want true for a null value")
	}
	if _, ok := om.Get("absent"); ok {
		t.Error("Get(absent) reported the key as present")
	}
	if om.Has("absent") {
		t.Error("Has(absent) = true, want false")
	}
}

func TestOrderedMap_Delete(t *testing.T) {
	om := newFrom(t, "a", 1, "b", 2, "c", 3)
	om.Delete("b")

	if got, want := om.Keys(), []string{"a", "c"}; !slices.Equal(got, want) {
		t.Errorf("Keys() = %v, want %v", got, want)
	}
	if om.Has("b") {
		t.Error("deleted key is still present")
	}

	om.Delete("missing") // must be a no-op
	if got := om.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}

	// Re-adding a deleted key appends it at the end.
	om.Set("b", 9)
	if got, want := om.Keys(), []string{"a", "c", "b"}; !slices.Equal(got, want) {
		t.Errorf("Keys() = %v, want %v", got, want)
	}
}

func TestOrderedMap_KeysReturnsCopy(t *testing.T) {
	om := newFrom(t, "a", 1, "b", 2)
	keys := om.Keys()
	keys[0] = "mutated"
	if got := om.Keys()[0]; got != "a" {
		t.Errorf("mutating the returned slice changed the map: got %q, want %q", got, "a")
	}
}

func TestOrderedMap_AllStopsOnBreak(t *testing.T) {
	om := newFrom(t, "a", 1, "b", 2, "c", 3)
	var seen []string
	for k := range om.All() {
		seen = append(seen, k)
		if k == "b" {
			break
		}
	}
	if want := []string{"a", "b"}; !slices.Equal(seen, want) {
		t.Errorf("All() yielded %v, want %v", seen, want)
	}
}

func TestOrderedMap_NilReceiver(t *testing.T) {
	var om *value.OrderedMap
	if got := om.Len(); got != 0 {
		t.Errorf("Len() = %d, want 0", got)
	}
	if om.Keys() != nil {
		t.Error("Keys() on a nil map is not nil")
	}
	if _, ok := om.Get("a"); ok {
		t.Error("Get() on a nil map reported a key as present")
	}
	if om.Has("a") {
		t.Error("Has() on a nil map returned true")
	}
	om.Delete("a") // must not panic
	for range om.All() {
		t.Error("All() on a nil map yielded an entry")
	}
	b, err := json.Marshal(om)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if string(b) != "null" {
		t.Errorf("Marshal() = %s, want null", b)
	}
}

func TestOrderedMap_MarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		om   *value.OrderedMap
		want string
	}{
		{"empty", value.NewOrderedMap(), `{}`},
		{
			name: "keys keep query order rather than being sorted",
			om:   newFrom(t, "zebra", 1, "alpha", 2),
			want: `{"zebra":1,"alpha":2}`,
		},
		{
			name: "null value",
			om:   newFrom(t, "a", nil),
			want: `{"a":null}`,
		},
		{
			name: "mixed value types",
			om:   newFrom(t, "s", "x", "n", 1, "b", true, "l", []any{1, 2}),
			want: `{"s":"x","n":1,"b":true,"l":[1,2]}`,
		},
		{
			name: "keys needing escapes",
			om:   newFrom(t, `a"b`, 1),
			want: `{"a\"b":1}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.om)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Marshal() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestOrderedMap_MarshalJSON_Nested(t *testing.T) {
	inner := newFrom(t, "second", 2, "first", 1)
	outer := newFrom(t, "hero", inner, "name", "Luke")

	got, err := json.Marshal(outer)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	want := `{"hero":{"second":2,"first":1},"name":"Luke"}`
	if string(got) != want {
		t.Errorf("Marshal() = %s, want %s", got, want)
	}
}

func TestOrderedMap_UnmarshalJSON_PreservesOrder(t *testing.T) {
	in := `{"zebra":1,"alpha":2,"middle":3}`
	var om value.OrderedMap
	if err := json.Unmarshal([]byte(in), &om); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got, want := om.Keys(), []string{"zebra", "alpha", "middle"}; !slices.Equal(got, want) {
		t.Errorf("Keys() = %v, want %v", got, want)
	}
}

func TestOrderedMap_UnmarshalJSON_NestedAndTypes(t *testing.T) {
	in := `{"o":{"b":1,"a":2},"arr":[1,{"y":1,"x":2}],"nul":null,"str":"s","bool":true}`
	var om value.OrderedMap
	if err := json.Unmarshal([]byte(in), &om); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	nested, ok := mustGet[*value.OrderedMap](t, &om, "o")
	if ok {
		if got, want := nested.Keys(), []string{"b", "a"}; !slices.Equal(got, want) {
			t.Errorf("nested Keys() = %v, want %v", got, want)
		}
	}

	arr, _ := mustGet[[]any](t, &om, "arr")
	if len(arr) != 2 {
		t.Fatalf("arr length = %d, want 2", len(arr))
	}
	if inner, ok := arr[1].(*value.OrderedMap); !ok {
		t.Errorf("arr[1] is %T, want *value.OrderedMap", arr[1])
	} else if got, want := inner.Keys(), []string{"y", "x"}; !slices.Equal(got, want) {
		t.Errorf("arr[1] Keys() = %v, want %v", got, want)
	}

	if v, ok := om.Get("nul"); !ok || v != nil {
		t.Errorf("Get(nul) = %v, %v, want nil, true", v, ok)
	}
}

// Numbers decode as json.Number so that large integer IDs keep full precision
// instead of being widened to float64.
func TestOrderedMap_UnmarshalJSON_KeepsNumberPrecision(t *testing.T) {
	const big = "9007199254740993" // 2^53 + 1, not representable as a float64
	var om value.OrderedMap
	if err := json.Unmarshal([]byte(`{"id":`+big+`}`), &om); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	n, ok := mustGet[json.Number](t, &om, "id")
	if !ok {
		return
	}
	if n.String() != big {
		t.Errorf("id = %s, want %s", n, big)
	}
}

func TestOrderedMap_UnmarshalJSON_Null(t *testing.T) {
	om := newFrom(t, "stale", 1)
	if err := json.Unmarshal([]byte("null"), om); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got := om.Len(); got != 0 {
		t.Errorf("Len() = %d, want 0", got)
	}
}

func TestOrderedMap_UnmarshalJSON_Errors(t *testing.T) {
	for _, in := range []string{`[1,2]`, `"str"`, `123`, `{`, `{"a":}`} {
		t.Run(in, func(t *testing.T) {
			var om value.OrderedMap
			if err := json.Unmarshal([]byte(in), &om); err == nil {
				t.Errorf("Unmarshal(%s) succeeded, want an error", in)
			}
		})
	}
}

func TestOrderedMap_JSONRoundTrip(t *testing.T) {
	for _, in := range []string{
		`{}`,
		`{"zebra":1,"alpha":2}`,
		`{"a":null,"b":"s","c":true}`,
		`{"hero":{"second":2,"first":1},"name":"Luke"}`,
		`{"list":[1,2,{"b":1,"a":2}]}`,
	} {
		t.Run(in, func(t *testing.T) {
			var om value.OrderedMap
			if err := json.Unmarshal([]byte(in), &om); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			out, err := json.Marshal(&om)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(out) != in {
				t.Errorf("round trip = %s, want %s", out, in)
			}
		})
	}
}

func TestOrderedMap_Equal(t *testing.T) {
	tests := []struct {
		name string
		a, b *value.OrderedMap
		want bool
	}{
		{"same keys and order", newFrom(t, "a", 1, "b", 2), newFrom(t, "a", 1, "b", 2), true},
		{"different order", newFrom(t, "a", 1, "b", 2), newFrom(t, "b", 2, "a", 1), false},
		{"different values", newFrom(t, "a", 1), newFrom(t, "a", 2), false},
		{"different lengths", newFrom(t, "a", 1), newFrom(t, "a", 1, "b", 2), false},
		{"both empty", value.NewOrderedMap(), value.NewOrderedMap(), true},
		{"nil equals empty", nil, value.NewOrderedMap(), true},
		{"nil differs from non-empty", nil, newFrom(t, "a", 1), false},
		{"uncomparable values do not panic", newFrom(t, "a", []int{1}), newFrom(t, "a", []int{1}), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equal(tt.b); got != tt.want {
				t.Errorf("Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}

// An OrderedMap nested inside a Maybe must still marshal in order, since that
// is exactly the shape of an execution result.
func TestOrderedMap_InsideMaybe(t *testing.T) {
	r := struct {
		Data value.Maybe[*value.OrderedMap] `json:"data,omitzero"`
	}{
		Data: value.Just(newFrom(t, "zebra", 1, "alpha", 2)),
	}
	got, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	want := `{"data":{"zebra":1,"alpha":2}}`
	if string(got) != want {
		t.Errorf("Marshal() = %s, want %s", got, want)
	}
}

func TestOrderedMap_NewOrderedMapSize(t *testing.T) {
	om := value.NewOrderedMapSize(4)
	if got := om.Len(); got != 0 {
		t.Errorf("Len() = %d, want 0", got)
	}
	om.Set("a", 1)
	if got := om.Len(); got != 1 {
		t.Errorf("Len() = %d, want 1", got)
	}
}

func mustGet[T any](t *testing.T, om *value.OrderedMap, key string) (T, bool) {
	t.Helper()
	var zero T
	v, ok := om.Get(key)
	if !ok {
		t.Errorf("key %q is missing; present keys: %v", key, om.Keys())
		return zero, false
	}
	typed, ok := v.(T)
	if !ok {
		t.Errorf("key %q holds %T, want %T", key, v, zero)
		return zero, false
	}
	return typed, true
}
