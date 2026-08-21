package schema

import (
	"encoding/json"
	"github.com/ikawaha/graphql/value"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/language"
)

// coerceCase describes one conversion a scalar has to get right.
type coerceCase struct {
	name    string
	in      any
	want    any
	wantErr string // a substring of the message, when the conversion must fail
}

func runCoerce(t *testing.T, coerce func(any) (value.Maybe[any], error), cases []coerceCase) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			answered, err := coerce(tt.in)
			got, _ := answered.Get()
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("coercing %#v gave %#v, want an error", tt.in, got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("coercing %#v: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("coercing %#v = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

// Coercing out of a resolver is lenient, because a resolver hands back
// whatever its data source held.
func TestInt_CoerceOutputValue(t *testing.T) {
	runCoerce(t, Int.CoerceOutputValue, []coerceCase{
		{name: "int", in: 1, want: int32(1)},
		{name: "negative", in: -1, want: int32(-1)},
		{name: "zero", in: 0, want: int32(0)},
		{name: "int64", in: int64(7), want: int32(7)},
		{name: "uint8", in: uint8(200), want: int32(200)},
		{name: "an integral float", in: 1.0, want: int32(1)},
		{name: "true", in: true, want: int32(1)},
		{name: "false", in: false, want: int32(0)},
		{name: "a numeric string", in: "42", want: int32(42)},
		{name: "the largest Int", in: MaxInt, want: int32(MaxInt)},
		{name: "the smallest Int", in: MinInt, want: int32(MinInt)},

		{name: "a fractional float", in: 1.5, wantErr: "non-integer value"},
		{name: "past the top of the range", in: MaxInt + 1, wantErr: "non 32-bit"},
		{name: "past the bottom of the range", in: MinInt - 1, wantErr: "non 32-bit"},
		{name: "a word", in: "one", wantErr: "non-integer value"},
		{name: "the empty string", in: "", wantErr: "non-integer value"},
		{name: "null", in: nil, wantErr: "non-integer value"},
		{name: "a list", in: []int{1}, wantErr: "non-integer value"},
	})
}

// Coercing in from a caller is strict: a request that sends the wrong kind of
// value is a mistake to report, not one to guess at.
func TestInt_CoerceInputValue(t *testing.T) {
	runCoerce(t, Int.CoerceInputValue, []coerceCase{
		{name: "int", in: 1, want: int32(1)},
		{name: "an integral float", in: 1.0, want: int32(1)},

		{name: "a numeric string is not accepted", in: "42", wantErr: "non-integer value"},
		{name: "a boolean is not accepted", in: true, wantErr: "non-integer value"},
		{name: "a fractional float", in: 1.5, wantErr: "non-integer value"},
		{name: "out of range", in: MaxInt + 1, wantErr: "non 32-bit"},
	})
}

func TestInt_CoerceInputLiteral(t *testing.T) {
	tests := []struct {
		name    string
		literal language.Value
		want    any
		wantErr string
	}{
		{name: "an integer", literal: &language.IntValue{Value: "42"}, want: int32(42)},
		{name: "negative", literal: &language.IntValue{Value: "-42"}, want: int32(-42)},
		{
			name:    "out of range",
			literal: &language.IntValue{Value: "2147483648"},
			wantErr: "non 32-bit",
		},
		{
			name:    "a float literal",
			literal: &language.FloatValue{Value: "1.5"},
			wantErr: "non-integer value",
		},
		{
			name:    "a string literal",
			literal: &language.StringValue{Value: "42"},
			wantErr: "non-integer value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answered, err := Int.CoerceInputLiteral(tt.literal)
			got, _ := answered.Get()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("= %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestFloat_Coercion(t *testing.T) {
	runCoerce(t, Float.CoerceOutputValue, []coerceCase{
		{name: "a float", in: 1.5, want: 1.5},
		{name: "an int", in: 1, want: float64(1)},
		{name: "true", in: true, want: float64(1)},
		{name: "a numeric string", in: "1.5", want: 1.5},

		{name: "not a number", in: math.NaN(), wantErr: "non numeric value"},
		{name: "infinity", in: math.Inf(1), wantErr: "non numeric value"},
		{name: "a word", in: "one", wantErr: "non numeric value"},
	})

	runCoerce(t, Float.CoerceInputValue, []coerceCase{
		{name: "a float", in: 1.5, want: 1.5},
		{name: "an int", in: 1, want: float64(1)},

		{name: "a string is not accepted", in: "1.5", wantErr: "non numeric value"},
		{name: "a boolean is not accepted", in: true, wantErr: "non numeric value"},
	})

	// An integer literal is a valid Float, which is why 1 may be written where
	// 1.0 is meant.
	for _, literal := range []language.Value{
		&language.FloatValue{Value: "1.5"},
		&language.IntValue{Value: "1"},
	} {
		if _, err := Float.CoerceInputLiteral(literal); err != nil {
			t.Errorf("Float rejected %s: %v", language.Print(literal), err)
		}
	}
	if _, err := Float.CoerceInputLiteral(&language.StringValue{Value: "1.5"}); err == nil {
		t.Error("Float accepted a string literal")
	}
}

func TestString_Coercion(t *testing.T) {
	runCoerce(t, String.CoerceOutputValue, []coerceCase{
		{name: "a string", in: "hello", want: "hello"},
		{name: "the empty string", in: "", want: ""},
		{name: "true", in: true, want: "true"},
		{name: "false", in: false, want: "false"},
		{name: "an int", in: 1, want: "1"},
		{name: "a float", in: 1.5, want: "1.5"},

		{name: "null", in: nil, wantErr: "cannot represent value"},
		{name: "a list", in: []string{"a"}, wantErr: "cannot represent value"},
	})

	runCoerce(t, String.CoerceInputValue, []coerceCase{
		{name: "a string", in: "hello", want: "hello"},

		{name: "an int is not accepted", in: 1, wantErr: "non string value"},
		{name: "a boolean is not accepted", in: true, wantErr: "non string value"},
	})
}

func TestBoolean_Coercion(t *testing.T) {
	runCoerce(t, Boolean.CoerceOutputValue, []coerceCase{
		{name: "true", in: true, want: true},
		{name: "false", in: false, want: false},
		{name: "one", in: 1, want: true},
		{name: "zero", in: 0, want: false},

		{name: "a string", in: "true", wantErr: "non boolean value"},
		{name: "null", in: nil, wantErr: "non boolean value"},
	})

	runCoerce(t, Boolean.CoerceInputValue, []coerceCase{
		{name: "true", in: true, want: true},

		{name: "a number is not accepted", in: 1, wantErr: "non boolean value"},
		{name: "a string is not accepted", in: "true", wantErr: "non boolean value"},
	})
}

func TestID_Coercion(t *testing.T) {
	for _, coerce := range []func(any) (value.Maybe[any], error){ID.CoerceOutputValue, ID.CoerceInputValue} {
		runCoerce(t, coerce, []coerceCase{
			{name: "a string", in: "abc", want: "abc"},
			{name: "an int", in: 42, want: "42"},
			{name: "an integral float", in: 42.0, want: "42"},

			{name: "a fractional number", in: 1.5, wantErr: "cannot represent value"},
			{name: "a boolean", in: true, wantErr: "cannot represent value"},
			{name: "null", in: nil, wantErr: "cannot represent value"},
		})
	}

	for _, literal := range []language.Value{
		&language.StringValue{Value: "abc"},
		&language.IntValue{Value: "42"},
	} {
		if _, err := ID.CoerceInputLiteral(literal); err != nil {
			t.Errorf("ID rejected %s: %v", language.Print(literal), err)
		}
	}
	if _, err := ID.CoerceInputLiteral(&language.FloatValue{Value: "1.5"}); err == nil {
		t.Error("ID accepted a float literal")
	}
}

// An identifier larger than a float64 can hold has to survive intact.
// A JSON decoder set to preserve digits hands one over as a json.Number, and
// widening that to a float64 on the way through would quietly change it.
func TestID_KeepsPrecisionOfLargeIdentifiers(t *testing.T) {
	const huge = "9007199254740993" // 2^53 + 1

	var decoded struct {
		ID json.Number `json:"id"`
	}
	dec := json.NewDecoder(strings.NewReader(`{"id":` + huge + `}`))
	dec.UseNumber()
	if err := dec.Decode(&decoded); err != nil {
		t.Fatalf("decoding: %v", err)
	}

	answered, err := ID.CoerceInputValue(decoded.ID)
	got, _ := answered.Get()
	if err != nil {
		t.Fatalf("coercing: %v", err)
	}
	if got != huge {
		t.Errorf("ID = %v, want %s", got, huge)
	}

	// Going through a float64 is what this avoids. Widening the same digits
	// shows why: the value comes back changed.
	widened, err := decoded.ID.Float64()
	if err != nil {
		t.Fatalf("widening: %v", err)
	}
	if strconv.FormatFloat(widened, 'f', -1, 64) == huge {
		t.Error("a float64 held these digits exactly, so this test is no longer pinning anything")
	}
}

// A numeric string reaching Int goes down its own path, because it was parsed
// as a float and has to be narrowed from there.
func TestInt_FromNumericString(t *testing.T) {
	runCoerce(t, Int.CoerceOutputValue, []coerceCase{
		{name: "an integral string", in: "42", want: int32(42)},
		{name: "a negative string", in: "-42", want: int32(-42)},
		{name: "a string with spaces", in: " 42 ", want: int32(42)},

		{name: "a fractional string", in: "1.5", wantErr: "non-integer value"},
		{name: "a string out of range", in: "99999999999", wantErr: "non 32-bit"},
	})
}

// A JSON decoder set to preserve digits hands over a json.Number whatever the
// value was, so all three shapes have to be read correctly.
func TestScalars_JSONNumberShapes(t *testing.T) {
	t.Run("an integer", func(t *testing.T) {
		answered, err := Int.CoerceInputValue(json.Number("42"))
		got, _ := answered.Get()
		if err != nil || got != int32(42) {
			t.Errorf("= %#v, %v, want int32(42), nil", got, err)
		}
	})

	t.Run("a fraction", func(t *testing.T) {
		answered, err := Float.CoerceInputValue(json.Number("1.5"))
		got, _ := answered.Get()
		if err != nil || got != 1.5 {
			t.Errorf("= %#v, %v, want 1.5, nil", got, err)
		}
		// The same digits are not an Int.
		if _, err := Int.CoerceInputValue(json.Number("1.5")); err == nil {
			t.Error("Int accepted a fractional json.Number")
		}
	})

	t.Run("an integer too large for an int64", func(t *testing.T) {
		const huge = "99999999999999999999"
		answered, err := ID.CoerceInputValue(json.Number(huge))
		got, _ := answered.Get()
		if err != nil {
			t.Fatalf("ID rejected a large identifier: %v", err)
		}
		if got != huge {
			t.Errorf("ID = %v, want the digits unchanged", got)
		}
		// It is still far outside the range of an Int.
		if _, err := Int.CoerceInputValue(json.Number(huge)); err == nil {
			t.Error("Int accepted a value far outside its range")
		}
	})

	t.Run("digits that are not a number at all", func(t *testing.T) {
		if _, err := Int.CoerceInputValue(json.Number("not a number")); err == nil {
			t.Error("Int accepted a json.Number holding nonsense")
		}
		if _, err := Float.CoerceInputValue(json.Number("")); err == nil {
			t.Error("Float accepted an empty json.Number")
		}
	})
}

// Every numeric type Go has must be recognised, because JavaScript has one and
// the reference implementation only ever tests for that one.
func TestScalars_AcceptEveryGoNumericType(t *testing.T) {
	values := []any{
		int(1), int8(1), int16(1), int32(1), int64(1),
		uint(1), uint8(1), uint16(1), uint32(1), uint64(1),
		float32(1), float64(1), json.Number("1"),
	}
	for _, v := range values {
		answered, err := Int.CoerceOutputValue(v)
		got, _ := answered.Get()
		if err != nil {
			t.Errorf("Int rejected %T(%v): %v", v, v, err)
			continue
		}
		if got != int32(1) {
			t.Errorf("Int coerced %T(%v) to %#v, want int32(1)", v, v, got)
		}
	}
}

// An unsigned value too large for an int64 must not wrap around into a
// negative one.
func TestScalars_LargeUnsignedValues(t *testing.T) {
	const huge = uint64(math.MaxUint64)

	if _, err := Int.CoerceOutputValue(huge); err == nil {
		t.Error("Int accepted a value far outside its range")
	}
	answered, err := ID.CoerceOutputValue(huge)
	got, _ := answered.Get()
	if err != nil {
		t.Fatalf("ID rejected a large identifier: %v", err)
	}
	if got != "18446744073709551615" {
		t.Errorf("ID = %v, want the digits unchanged", got)
	}
}

func TestSpecifiedScalars(t *testing.T) {
	if len(SpecifiedScalars) != 5 {
		t.Fatalf("%d specified scalars, want 5", len(SpecifiedScalars))
	}
	names := map[string]bool{}
	for _, s := range SpecifiedScalars {
		names[s.Name()] = true
		if !IsSpecifiedScalarType(s) {
			t.Errorf("%s was not recognised as a specified scalar", s.Name())
		}
	}
	for _, want := range []string{"String", "Int", "Float", "Boolean", "ID"} {
		if !names[want] {
			t.Errorf("%s is missing from SpecifiedScalars", want)
		}
	}

	custom := NewScalar(ScalarConfig{Name: "DateTime"})
	if IsSpecifiedScalarType(custom) {
		t.Error("a custom scalar was recognised as a specified one")
	}
	// A different scalar that shares a built-in's name is still not the same
	// type.
	impostor := NewScalar(ScalarConfig{Name: "Int"})
	if IsSpecifiedScalarType(impostor) {
		t.Error("a custom scalar named Int was recognised as the built-in")
	}
	if IsSpecifiedScalarType(NewObject(ObjectConfig{Name: "User"})) {
		t.Error("an object type was recognised as a specified scalar")
	}
}
