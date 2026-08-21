package schema

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/ikawaha/graphql/language"
)

// printLiteral renders what the conversion produced, which is how a default
// value appears in a schema or in introspection.
func printLiteral(t *testing.T, v any, typ Type) (string, bool) {
	t.Helper()
	literal, ok := LiteralFromValue(v, typ)
	if !ok {
		return "", false
	}
	return language.Print(literal), true
}

func TestLiteralFromValue_Scalars(t *testing.T) {
	tests := []struct {
		name string
		in   any
		typ  Type
		want string
	}{
		{"an Int", 1, Int, "1"},
		{"a negative Int", -1, Int, "-1"},
		{"an int32", int32(7), Int, "7"},
		{"a uint", uint(7), Int, "7"},
		{"a Float", 1.5, Float, "1.5"},
		{"a whole Float", 1.0, Float, "1"},
		{"a String", "hi", String, `"hi"`},
		{"a String needing escapes", `a"b`, String, `"a\"b"`},
		{"true", true, Boolean, "true"},
		{"false", false, Boolean, "false"},
		{"an ID", "abc", ID, `"abc"`},
		{"null", nil, String, "null"},
		// An identifier made of digits is written as an integer, which is what
		// a document would have written and reads back as the same ID. Its
		// size does not matter: nothing here turns it into a number.
		{"a large identifier", json.Number("99999999999999999999"), ID, "99999999999999999999"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := printLiteral(t, tt.in, tt.typ)
			if !ok {
				t.Fatalf("%#v could not be written as %s", tt.in, tt.typ)
			}
			if got != tt.want {
				t.Errorf("= %s, want %s", got, tt.want)
			}
		})
	}
}

// The type is what decides how a value is written. The same Go string is a
// quoted string for a String field and a bare member name for an enum, and
// nothing but the type says which.
func TestLiteralFromValue_TypeDecidesTheForm(t *testing.T) {
	colour := NewEnum(EnumConfig{
		Name: "Colour",
		Values: []*EnumValue{
			NewEnumValue("RED", EnumValueConfig{}),
			NewEnumValue("GREEN", EnumValueConfig{Value: InternalValue(2)}),
		},
	})

	if got, _ := printLiteral(t, "RED", String); got != `"RED"` {
		t.Errorf("as a String = %s, want it quoted", got)
	}
	if got, _ := printLiteral(t, "RED", colour); got != "RED" {
		t.Errorf("as an enum = %s, want the bare member name", got)
	}
	// A member with an internal value of its own is found by that value.
	if got, _ := printLiteral(t, 2, colour); got != "GREEN" {
		t.Errorf("the member with internal value 2 = %s, want GREEN", got)
	}
	// A value that is no member at all cannot be written.
	if _, ok := LiteralFromValue("BLUE", colour); ok {
		t.Error("a value that names no member was written anyway")
	}
}

func TestLiteralFromValue_ListsAndObjects(t *testing.T) {
	filter := NewInputObject(InputObjectConfig{
		Name: "Filter",
		Fields: []*InputField{
			NewInputField("term", InputFieldConfig{Type: String}),
			NewInputField("limit", InputFieldConfig{Type: Int}),
		},
	})

	tests := []struct {
		name string
		in   any
		typ  Type
		want string
	}{
		{"a list", []any{1, 2}, NewList(Int), "[1, 2]"},
		{"a typed slice", []int{1, 2}, NewList(Int), "[1, 2]"},
		{"an empty list", []any{}, NewList(Int), "[]"},
		{"a list holding null", []any{1, nil}, NewList(Int), "[1, null]"},
		// A lone value stands for a list of one on the way in, so it is
		// written bare on the way out.
		{"a lone value for a list", 1, NewList(Int), "1"},
		{
			name: "an input object",
			in:   map[string]any{"term": "x", "limit": 10},
			typ:  filter,
			want: `{ term: "x", limit: 10 }`,
		},
		{
			name: "an input object with a field left out",
			in:   map[string]any{"term": "x"},
			typ:  filter,
			want: `{ term: "x" }`,
		},
		{"an empty input object", map[string]any{}, filter, "{  }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := printLiteral(t, tt.in, tt.typ)
			if !ok {
				t.Fatalf("%#v could not be written as %s", tt.in, tt.typ)
			}
			if got != tt.want {
				t.Errorf("= %s, want %s", got, tt.want)
			}
		})
	}
}

// An input object's fields are written in the order the type declares them, so
// that the same value always produces the same text. A Go map has no order of
// its own, so without this the output would vary between runs.
func TestLiteralFromValue_FieldOrderIsStable(t *testing.T) {
	filter := NewInputObject(InputObjectConfig{
		Name: "Filter",
		Fields: []*InputField{
			NewInputField("zebra", InputFieldConfig{Type: String}),
			NewInputField("alpha", InputFieldConfig{Type: String}),
			NewInputField("middle", InputFieldConfig{Type: String}),
		},
	})
	in := map[string]any{"alpha": "a", "middle": "m", "zebra": "z"}

	want := `{ zebra: "z", alpha: "a", middle: "m" }`
	for range 20 {
		got, ok := printLiteral(t, in, filter)
		if !ok {
			t.Fatal("could not be written")
		}
		if got != want {
			t.Fatalf("= %s, want %s", got, want)
		}
	}
}

func TestLiteralFromValue_Rejections(t *testing.T) {
	filter := NewInputObject(InputObjectConfig{
		Name: "Filter",
		Fields: []*InputField{
			NewInputField("needed", InputFieldConfig{Type: NewNonNull(String)}),
		},
	})
	object := NewObject(ObjectConfig{
		Name:   "User",
		Fields: []*Field{NewField("a", FieldConfig{Type: String})},
	})

	tests := []struct {
		name string
		in   any
		typ  Type
	}{
		{"null where it is forbidden", nil, NewNonNull(String)},
		{"a required field left out", map[string]any{}, filter},
		{"a field the type does not have", map[string]any{"needed": "x", "extra": 1}, filter},
		{"not an object at all", "x", filter},
		{"a type that cannot hold input", "x", object},
		{"something with no literal form", make(chan int), String},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, ok := LiteralFromValue(tt.in, tt.typ); ok {
				t.Errorf("written as %s, want it refused", language.Print(got))
			}
		})
	}
}

// A number that cannot be written in a document has no literal form. Writing
// null instead would quietly turn a default that is wrong into a default of
// null, which means something different.
func TestLiteralFromValue_NonFiniteNumbers(t *testing.T) {
	for _, f := range []float64{math.Inf(1), math.Inf(-1), math.NaN()} {
		if got, ok := LiteralFromValue(f, Float); ok {
			t.Errorf("%v was written as %s, want it refused", f, language.Print(got))
		}
	}

	// The type-blind rendering, which has no type to refuse against, maps them
	// to null instead. Nothing reaches it for a built-in scalar; it is what a
	// scalar with no conversion of its own would get.
	for _, f := range []float64{math.Inf(1), math.NaN()} {
		got, ok := LiteralFromGoValue(f)
		if !ok || language.Print(got) != "null" {
			t.Errorf("%v = %v, %v, want null", f, got, ok)
		}
	}
}

// A scalar may render its own values, which is how a custom scalar controls
// how its defaults appear.
func TestLiteralFromValue_ScalarWithItsOwnRendering(t *testing.T) {
	custom := NewScalar(ScalarConfig{
		Name: "Upper",
		ValueToLiteral: func(v any, _ Type) (language.Value, error) {
			s, isString := v.(string)
			if !isString {
				return nil, errNotAString
			}
			return &language.StringValue{Value: "<" + s + ">"}, nil
		},
	})

	if got, _ := printLiteral(t, "hi", custom); got != `"<hi>"` {
		t.Errorf("= %s, want the scalar's own rendering", got)
	}
	if _, ok := LiteralFromValue(1, custom); ok {
		t.Error("the scalar's refusal was ignored")
	}
}

var errNotAString = errString("not a string")

type errString string

func (e errString) Error() string { return string(e) }

// A default supplied in code now appears in introspection, which it did not
// while there was no way to render a Go value as a literal.
func TestIntrospection_DefaultValueFromAGoValue(t *testing.T) {
	colour := NewEnum(EnumConfig{
		Name:   "Colour",
		Values: []*EnumValue{NewEnumValue("RED", EnumValueConfig{})},
	})

	tests := []struct {
		name string
		arg  *Argument
		want any
	}{
		{
			name: "a number",
			arg:  NewArgument("limit", ArgumentConfig{Type: Int, Default: DefaultValue(10)}),
			want: "10",
		},
		{
			name: "a string",
			arg:  NewArgument("term", ArgumentConfig{Type: String, Default: DefaultValue("all")}),
			want: `"all"`,
		},
		{
			name: "an enum member",
			arg:  NewArgument("colour", ArgumentConfig{Type: colour, Default: DefaultValue("RED")}),
			want: "RED",
		},
		{
			name: "null",
			arg:  NewArgument("maybe", ArgumentConfig{Type: String, Default: DefaultValue(nil)}),
			want: "null",
		},
		{
			name: "a literal, which is kept as written",
			arg: NewArgument("term", ArgumentConfig{
				Type:    String,
				Default: DefaultLiteral(&language.StringValue{Value: "written"}),
			}),
			want: `"written"`,
		},
		{
			name: "no default at all",
			arg:  NewArgument("plain", ArgumentConfig{Type: String}),
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveOn(t, InputValueIntrospectionType, "defaultValue", tt.arg, nil, nil)
			if got != tt.want {
				t.Errorf("defaultValue = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// A default that does not fit its own type has no literal form. Rather than
// print something wrong, introspection reports none; the schema validator is
// what complains about the schema.
func TestIntrospection_DefaultValueThatDoesNotFit(t *testing.T) {
	arg := NewArgument("bad", ArgumentConfig{Type: Int, Default: DefaultValue("not an Int")})
	if got := resolveOn(t, InputValueIntrospectionType, "defaultValue", arg, nil, nil); got != nil {
		t.Errorf("defaultValue = %#v, want nil", got)
	}
}

// LiteralFromGoValue renders without a type to consult. It is what a scalar
// that neither renders nor converts its own values falls back to, and it is
// exported for anyone writing such a scalar, so it is worth pinning on its
// own.
func TestLiteralFromGoValue(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"null", nil, "null"},
		{"true", true, "true"},
		{"a string", "hi", `"hi"`},
		{"an int", 1, "1"},
		{"an int64", int64(-9), "-9"},
		{"a uint64", uint64(9), "9"},
		{"a whole float", 2.0, "2"},
		{"a fractional float", 2.5, "2.5"},
		{"a float32", float32(0.5), "0.5"},
		{"a json.Number holding an integer", json.Number("7"), "7"},
		{"a json.Number holding a fraction", json.Number("7.5"), "7.5"},
		{"a slice", []any{1, "a"}, `[1, "a"]`},
		{"a typed slice", []int{1, 2}, "[1, 2]"},
		{"an empty slice", []any{}, "[]"},
		// A map has no order of its own, so its fields are written by name to
		// keep the text the same from one run to the next.
		{"a map", map[string]any{"b": 2, "a": 1}, "{ a: 1, b: 2 }"},
		{"a typed map", map[string]int{"z": 1, "a": 2}, "{ a: 2, z: 1 }"},
		{"a nested map", map[string]any{"a": map[string]any{"b": 1}}, "{ a: { b: 1 } }"},
		// graphql-js's own cases for the same fallback.
		{"a large whole number", 1099511627776, "1099511627776"},
		{"an integer beyond a float64's precision", int64(9007199254740993), "9007199254740993"},
		{"a number large enough to need an exponent", 2e40, "2e+40"},
		// Neither can be written in a document at all.
		{"not a number", math.NaN(), "null"},
		{"infinity", math.Inf(1), "null"},
		{"negative infinity", math.Inf(-1), "null"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			literal, ok := LiteralFromGoValue(tt.in)
			if !ok {
				t.Fatalf("%#v could not be written", tt.in)
			}
			if got := language.Print(literal); got != tt.want {
				t.Errorf("= %s, want %s", got, tt.want)
			}
		})
	}
}

func TestLiteralFromGoValue_Rejections(t *testing.T) {
	tests := []struct {
		name string
		in   any
	}{
		{"a channel", make(chan int)},
		{"a function", func() {}},
		{"a map keyed by something other than a string", map[int]any{1: 2}},
		{"a slice holding something with no form", []any{make(chan int)}},
		{"a map holding something with no form", map[string]any{"a": make(chan int)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if literal, ok := LiteralFromGoValue(tt.in); ok {
				t.Errorf("written as %s, want it refused", language.Print(literal))
			}
		})
	}
}
