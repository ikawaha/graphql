package schema_test

// Ported from graphql-js src/type/__tests__/scalars-test.ts: what each of the
// scalars every schema has will and will not accept, coming in as a value,
// coming in as a literal, and going back out.
//
// A `fails` message is what the scalar should say. Where Go cannot hold the
// value graphql-js passes — a bigint past 2^63, an object with a valueOf
// method — the case is left out.

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// scalarCase is one assertion: a value read as a scalar, and what should come
// of it.
type scalarCase struct {
	as *schema.ScalarType
	in any
	// want is what should come back; fails is what should be said instead.
	want  any
	fails string
}

// knownScalarDivergences are the cases this implementation does not match, and
// why. Each is asserted to *still* diverge, so that closing one cannot go
// unnoticed.
var knownScalarDivergences = map[string]string{}

func TestPortedScalars_InputValue(t *testing.T) {
	runScalarCases(t, "input value", []scalarCase{
		{as: schema.Int, in: 1, want: int32(1)},
		{as: schema.Int, in: 0, want: int32(0)},
		{as: schema.Int, in: -1, want: int32(-1)},
		{as: schema.Int, in: int64(1), want: int32(1)},
		{as: schema.Int, in: int64(0), want: int32(0)},
		{as: schema.Int, in: int64(-1), want: int32(-1)},
		{as: schema.Int, in: 9876504321, fails: "Int cannot represent non 32-bit signed integer value: 9876504321"},
		{as: schema.Int, in: -9876504321, fails: "Int cannot represent non 32-bit signed integer value: -9876504321"},
		{as: schema.Int, in: int64(2147483648), fails: "Int cannot represent non 32-bit signed integer value: 2147483648"},
		{as: schema.Int, in: int64(-2147483649), fails: "Int cannot represent non 32-bit signed integer value: -2147483649"},
		{as: schema.Int, in: 0.1, fails: "Int cannot represent non-integer value: 0.1"},
		{as: schema.Int, in: math.NaN(), fails: "Int cannot represent non-integer value: NaN"},
		{as: schema.Int, in: math.Inf(1), fails: "Int cannot represent non-integer value: Infinity"},
		{as: schema.Int, in: nil, fails: "Int cannot represent non-integer value: null"},
		{as: schema.Int, in: "", fails: "Int cannot represent non-integer value: \"\""},
		{as: schema.Int, in: "123", fails: "Int cannot represent non-integer value: \"123\""},
		{as: schema.Int, in: false, fails: "Int cannot represent non-integer value: false"},
		{as: schema.Int, in: true, fails: "Int cannot represent non-integer value: true"},
		{as: schema.Int, in: []any{1}, fails: "Int cannot represent non-integer value: [1]"},
		{as: schema.Int, in: map[string]any{"value": 1}, fails: "Int cannot represent non-integer value: { value: 1 }"},
		{as: schema.Float, in: 1, want: float64(1)},
		{as: schema.Float, in: 0, want: float64(0)},
		{as: schema.Float, in: -1, want: float64(-1)},
		{as: schema.Float, in: 0.1, want: 0.1},
		{as: schema.Float, in: math.Pi, want: math.Pi},
		{as: schema.Float, in: int64(1), want: float64(1)},
		{as: schema.Float, in: int64(0), want: float64(0)},
		{as: schema.Float, in: int64(-1), want: float64(-1)},
		{as: schema.Float, in: int64(9007199254740992), want: float64(9007199254740992)},
		{as: schema.Float, in: math.NaN(), fails: "Float cannot represent non numeric value: NaN"},
		{as: schema.Float, in: math.Inf(1), fails: "Float cannot represent non numeric value: Infinity"},
		{as: schema.Float, in: int64(9007199254740993), fails: "Float cannot represent non numeric value: 9007199254740993 (value would lose precision)"},
		{as: schema.Float, in: nil, fails: "Float cannot represent non numeric value: null"},
		{as: schema.Float, in: "", fails: "Float cannot represent non numeric value: \"\""},
		{as: schema.Float, in: "123", fails: "Float cannot represent non numeric value: \"123\""},
		{as: schema.Float, in: "123.5", fails: "Float cannot represent non numeric value: \"123.5\""},
		{as: schema.Float, in: false, fails: "Float cannot represent non numeric value: false"},
		{as: schema.Float, in: true, fails: "Float cannot represent non numeric value: true"},
		{as: schema.Float, in: []any{0.1}, fails: "Float cannot represent non numeric value: [0.1]"},
		{as: schema.Float, in: map[string]any{"value": 0.1}, fails: "Float cannot represent non numeric value: { value: 0.1 }"},
		{as: schema.String, in: "foo", want: "foo"},
		{as: schema.String, in: nil, fails: "String cannot represent a non string value: null"},
		{as: schema.String, in: 1, fails: "String cannot represent a non string value: 1"},
		{as: schema.String, in: math.NaN(), fails: "String cannot represent a non string value: NaN"},
		{as: schema.String, in: false, fails: "String cannot represent a non string value: false"},
		{as: schema.String, in: []any{"foo"}, fails: "String cannot represent a non string value: [\"foo\"]"},
		{as: schema.String, in: map[string]any{"value": "foo"}, fails: "String cannot represent a non string value: { value: \"foo\" }"},
		{as: schema.Boolean, in: true, want: true},
		{as: schema.Boolean, in: false, want: false},
		{as: schema.Boolean, in: nil, fails: "Boolean cannot represent a non boolean value: null"},
		{as: schema.Boolean, in: 0, fails: "Boolean cannot represent a non boolean value: 0"},
		{as: schema.Boolean, in: 1, fails: "Boolean cannot represent a non boolean value: 1"},
		{as: schema.Boolean, in: math.NaN(), fails: "Boolean cannot represent a non boolean value: NaN"},
		{as: schema.Boolean, in: "", fails: "Boolean cannot represent a non boolean value: \"\""},
		{as: schema.Boolean, in: "false", fails: "Boolean cannot represent a non boolean value: \"false\""},
		{as: schema.Boolean, in: []any{false}, fails: "Boolean cannot represent a non boolean value: [false]"},
		{as: schema.Boolean, in: map[string]any{"value": false}, fails: "Boolean cannot represent a non boolean value: { value: false }"},
		{as: schema.ID, in: "", want: ""},
		{as: schema.ID, in: "1", want: "1"},
		{as: schema.ID, in: "foo", want: "foo"},
		{as: schema.ID, in: 1, want: "1"},
		{as: schema.ID, in: 0, want: "0"},
		{as: schema.ID, in: -1, want: "-1"},
		{as: schema.ID, in: int64(1), want: "1"},
		{as: schema.ID, in: int64(0), want: "0"},
		{as: schema.ID, in: int64(-1), want: "-1"},
		{as: schema.ID, in: 9007199254740991, want: "9007199254740991"},
		{as: schema.ID, in: -9007199254740991, want: "-9007199254740991"},
		{as: schema.ID, in: nil, fails: "ID cannot represent value: null"},
		{as: schema.ID, in: 0.1, fails: "ID cannot represent value: 0.1"},
		{as: schema.ID, in: math.NaN(), fails: "ID cannot represent value: NaN"},
		{as: schema.ID, in: math.Inf(1), fails: "ID cannot represent value: Inf"},
		{as: schema.ID, in: false, fails: "ID cannot represent value: false"},
	}, func(s *schema.ScalarType, in any) (any, error) {
		coerced, err := s.CoerceInputValue(in)
		return coerced.Or(nil), err
	})
}

func TestPortedScalars_InputLiteral(t *testing.T) {
	runScalarCases(t, "input literal", []scalarCase{
		{as: schema.Int, in: "1", want: int32(1)},
		{as: schema.Int, in: "0", want: int32(0)},
		{as: schema.Int, in: "-1", want: int32(-1)},
		{as: schema.Int, in: "9876504321", fails: "Int cannot represent non 32-bit signed integer value: 9876504321"},
		{as: schema.Int, in: "-9876504321", fails: "Int cannot represent non 32-bit signed integer value: -9876504321"},
		{as: schema.Int, in: "1.0", fails: "Int cannot represent non-integer value: 1.0"},
		{as: schema.Int, in: "null", fails: "Int cannot represent non-integer value: null"},
		{as: schema.Int, in: "\"\"", fails: "Int cannot represent non-integer value: \"\""},
		{as: schema.Int, in: "\"123\"", fails: "Int cannot represent non-integer value: \"123\""},
		{as: schema.Int, in: "false", fails: "Int cannot represent non-integer value: false"},
		{as: schema.Int, in: "[1]", fails: "Int cannot represent non-integer value: [1]"},
		{as: schema.Int, in: "{ value: 1 }", fails: "Int cannot represent non-integer value: { value: 1 }"},
		{as: schema.Int, in: "ENUM_VALUE", fails: "Int cannot represent non-integer value: ENUM_VALUE"},
		{as: schema.Float, in: "1", want: float64(1)},
		{as: schema.Float, in: "0", want: float64(0)},
		{as: schema.Float, in: "-1", want: float64(-1)},
		{as: schema.Float, in: "0.1", want: 0.1},
		{as: schema.Float, in: strconv.FormatFloat(math.Pi, 'g', -1, 64), want: math.Pi},
		{as: schema.Float, in: "null", fails: "Float cannot represent non numeric value: null"},
		{as: schema.Float, in: "\"\"", fails: "Float cannot represent non numeric value: \"\""},
		{as: schema.Float, in: "\"123\"", fails: "Float cannot represent non numeric value: \"123\""},
		{as: schema.Float, in: "\"123.5\"", fails: "Float cannot represent non numeric value: \"123.5\""},
		{as: schema.Float, in: "false", fails: "Float cannot represent non numeric value: false"},
		{as: schema.Float, in: "[0.1]", fails: "Float cannot represent non numeric value: [0.1]"},
		{as: schema.Float, in: "{ value: 0.1 }", fails: "Float cannot represent non numeric value: { value: 0.1 }"},
		{as: schema.Float, in: "ENUM_VALUE", fails: "Float cannot represent non numeric value: ENUM_VALUE"},
		{as: schema.String, in: "\"foo\"", want: "foo"},
		{as: schema.String, in: "\"\"\"bar\"\"\"", want: "bar"},
		{as: schema.String, in: "null", fails: "String cannot represent a non string value: null"},
		{as: schema.String, in: "1", fails: "String cannot represent a non string value: 1"},
		{as: schema.String, in: "0.1", fails: "String cannot represent a non string value: 0.1"},
		{as: schema.String, in: "false", fails: "String cannot represent a non string value: false"},
		{as: schema.String, in: "[\"foo\"]", fails: "String cannot represent a non string value: [\"foo\"]"},
		{as: schema.String, in: "{ value: \"foo\" }", fails: "String cannot represent a non string value: { value: \"foo\" }"},
		{as: schema.String, in: "ENUM_VALUE", fails: "String cannot represent a non string value: ENUM_VALUE"},
		{as: schema.Boolean, in: "true", want: true},
		{as: schema.Boolean, in: "false", want: false},
		{as: schema.Boolean, in: "null", fails: "Boolean cannot represent a non boolean value: null"},
		{as: schema.Boolean, in: "0", fails: "Boolean cannot represent a non boolean value: 0"},
		{as: schema.Boolean, in: "1", fails: "Boolean cannot represent a non boolean value: 1"},
		{as: schema.Boolean, in: "0.1", fails: "Boolean cannot represent a non boolean value: 0.1"},
		{as: schema.Boolean, in: "\"\"", fails: "Boolean cannot represent a non boolean value: \"\""},
		{as: schema.Boolean, in: "\"false\"", fails: "Boolean cannot represent a non boolean value: \"false\""},
		{as: schema.Boolean, in: "[false]", fails: "Boolean cannot represent a non boolean value: [false]"},
		{as: schema.Boolean, in: "{ value: false }", fails: "Boolean cannot represent a non boolean value: { value: false }"},
		{as: schema.Boolean, in: "ENUM_VALUE", fails: "Boolean cannot represent a non boolean value: ENUM_VALUE"},
		{as: schema.ID, in: "\"\"", want: ""},
		{as: schema.ID, in: "\"1\"", want: "1"},
		{as: schema.ID, in: "\"foo\"", want: "foo"},
		{as: schema.ID, in: "\"\"\"foo\"\"\"", want: "foo"},
		{as: schema.ID, in: "1", want: "1"},
		{as: schema.ID, in: "0", want: "0"},
		{as: schema.ID, in: "-1", want: "-1"},
		{as: schema.ID, in: "90071992547409910", want: "90071992547409910"},
		{as: schema.ID, in: "-90071992547409910", want: "-90071992547409910"},
		{as: schema.ID, in: "null", fails: "ID cannot represent a non-string and non-integer value: null"},
		{as: schema.ID, in: "0.1", fails: "ID cannot represent a non-string and non-integer value: 0.1"},
		{as: schema.ID, in: "false", fails: "ID cannot represent a non-string and non-integer value: false"},
		{as: schema.ID, in: "[\"1\"]", fails: "ID cannot represent a non-string and non-integer value: [\"1\"]"},
		{as: schema.ID, in: "{ value: \"1\" }", fails: "ID cannot represent a non-string and non-integer value: { value: \"1\" }"},
		{as: schema.ID, in: "ENUM_VALUE", fails: "ID cannot represent a non-string and non-integer value: ENUM_VALUE"},
	}, func(s *schema.ScalarType, in any) (any, error) {
		literal, err := language.ParseValue(language.NewSource(in.(string)))
		if err != nil {
			return nil, err
		}
		if s.CoerceInputLiteral != nil {
			coerced, err := s.CoerceInputLiteral(literal)
			return coerced.Or(nil), err
		}
		plain, ok := schema.ValueFromASTUntyped(literal, schema.VariableValues{})
		if !ok {
			return nil, errNotALiteral
		}
		return s.CoerceInputValue(plain)
	})
}

func TestPortedScalars_OutputValue(t *testing.T) {
	runScalarCases(t, "output value", []scalarCase{
		{as: schema.Int, in: 1, want: int32(1)},
		{as: schema.Int, in: "123", want: int32(123)},
		{as: schema.Int, in: 0, want: int32(0)},
		{as: schema.Int, in: -1, want: int32(-1)},
		{as: schema.Int, in: 1e5, want: int32(100000)},
		{as: schema.Int, in: false, want: int32(0)},
		{as: schema.Int, in: true, want: int32(1)},
		{as: schema.Int, in: int64(1), want: int32(1)},
		{as: schema.Int, in: int64(0), want: int32(0)},
		{as: schema.Int, in: int64(-1), want: int32(-1)},
		{as: schema.Int, in: 0.1, fails: "Int cannot represent non-integer value: 0.1"},
		{as: schema.Int, in: 1.1, fails: "Int cannot represent non-integer value: 1.1"},
		{as: schema.Int, in: -1.1, fails: "Int cannot represent non-integer value: -1.1"},
		{as: schema.Int, in: "-1.1", fails: "Int cannot represent non-integer value: \"-1.1\""},
		{as: schema.Int, in: 9876504321, fails: "Int cannot represent non 32-bit signed integer value: 9876504321"},
		{as: schema.Int, in: -9876504321, fails: "Int cannot represent non 32-bit signed integer value: -9876504321"},
		{as: schema.Int, in: "9876504321", fails: "Int cannot represent non 32-bit signed integer value: \"9876504321\""},
		{as: schema.Int, in: "-9876504321", fails: "Int cannot represent non 32-bit signed integer value: \"-9876504321\""},
		{as: schema.Int, in: int64(2147483648), fails: "Int cannot represent non 32-bit signed integer value: 2147483648"},
		{as: schema.Int, in: int64(-2147483649), fails: "Int cannot represent non 32-bit signed integer value: -2147483649"},
		{as: schema.Int, in: 1e100, fails: "Int cannot represent non 32-bit signed integer value: 1e+100"},
		{as: schema.Int, in: -1e100, fails: "Int cannot represent non 32-bit signed integer value: -1e+100"},
		{as: schema.Int, in: "one", fails: "Int cannot represent non-integer value: \"one\""},
		{as: schema.Int, in: "", fails: "Int cannot represent non-integer value: \"\""},
		{as: schema.Int, in: math.NaN(), fails: "Int cannot represent non-integer value: NaN"},
		{as: schema.Int, in: math.Inf(1), fails: "Int cannot represent non-integer value: Infinity"},
		{as: schema.Int, in: []any{5}, fails: "Int cannot represent non-integer value: [5]"},
		{as: schema.Float, in: 1, want: 1.0},
		{as: schema.Float, in: 0, want: 0.0},
		{as: schema.Float, in: "123.5", want: 123.5},
		{as: schema.Float, in: -1, want: -1.0},
		{as: schema.Float, in: 0.1, want: 0.1},
		{as: schema.Float, in: 1.1, want: 1.1},
		{as: schema.Float, in: -1.1, want: -1.1},
		{as: schema.Float, in: "-1.1", want: -1.1},
		{as: schema.Float, in: false, want: 0.0},
		{as: schema.Float, in: true, want: 1.0},
		{as: schema.Float, in: int64(1), want: 1.0},
		{as: schema.Float, in: int64(0), want: 0.0},
		{as: schema.Float, in: int64(-1), want: -1.0},
		{as: schema.Float, in: int64(9007199254740992), want: float64(9007199254740992)},
		{as: schema.Float, in: math.NaN(), fails: "Float cannot represent non numeric value: NaN"},
		{as: schema.Float, in: math.Inf(1), fails: "Float cannot represent non numeric value: Infinity"},
		{as: schema.Float, in: int64(9007199254740993), fails: "Float cannot represent non numeric value: 9007199254740993 (value would lose precision)"},
		{as: schema.Float, in: "one", fails: "Float cannot represent non numeric value: \"one\""},
		{as: schema.Float, in: "", fails: "Float cannot represent non numeric value: \"\""},
		{as: schema.Float, in: []any{5}, fails: "Float cannot represent non numeric value: [5]"},
		{as: schema.String, in: "string", want: "string"},
		{as: schema.String, in: 1, want: "1"},
		{as: schema.String, in: -1.1, want: "-1.1"},
		{as: schema.String, in: true, want: "true"},
		{as: schema.String, in: false, want: "false"},
		{as: schema.String, in: int64(123), want: "123"},
		{as: schema.String, in: math.NaN(), fails: "String cannot represent value: NaN"},
		{as: schema.String, in: []any{1}, fails: "String cannot represent value: [1]"},
		{as: schema.Boolean, in: 1, want: true},
		{as: schema.Boolean, in: 0, want: false},
		{as: schema.Boolean, in: true, want: true},
		{as: schema.Boolean, in: false, want: false},
		{as: schema.Boolean, in: int64(1), want: true},
		{as: schema.Boolean, in: int64(0), want: false},
		{as: schema.Boolean, in: math.NaN(), fails: "Boolean cannot represent a non boolean value: NaN"},
		{as: schema.Boolean, in: "", fails: "Boolean cannot represent a non boolean value: \"\""},
		{as: schema.Boolean, in: "true", fails: "Boolean cannot represent a non boolean value: \"true\""},
		{as: schema.Boolean, in: []any{false}, fails: "Boolean cannot represent a non boolean value: [false]"},
		{as: schema.Boolean, in: map[string]any{}, fails: "Boolean cannot represent a non boolean value: {}"},
		{as: schema.ID, in: "string", want: "string"},
		{as: schema.ID, in: "false", want: "false"},
		{as: schema.ID, in: "", want: ""},
		{as: schema.ID, in: 123, want: "123"},
		{as: schema.ID, in: 0, want: "0"},
		{as: schema.ID, in: -1, want: "-1"},
		{as: schema.ID, in: int64(123), want: "123"},
		{as: schema.ID, in: int64(0), want: "0"},
		{as: schema.ID, in: int64(-1), want: "-1"},
		{as: schema.ID, in: true, fails: "ID cannot represent value: true"},
		{as: schema.ID, in: 3.14, fails: "ID cannot represent value: 3.14"},
		{as: schema.ID, in: map[string]any{}, fails: "ID cannot represent value: {}"},
		{as: schema.ID, in: []any{"abc"}, fails: "ID cannot represent value: [\"abc\"]"},
	}, func(s *schema.ScalarType, in any) (any, error) {
		coerced, err := s.CoerceOutputValue(in)
		return coerced.Or(nil), err
	})
}

var errNotALiteral = errorString("the literal cannot be read without a type")

type errorString string

func (e errorString) Error() string { return string(e) }

func runScalarCases(
	t *testing.T,
	what string,
	cases []scalarCase,
	coerce func(*schema.ScalarType, any) (any, error),
) {
	t.Helper()
	seen := map[string]int{}
	for _, tt := range cases {
		name := tt.as.Name() + " " + what + ": " + describeScalarInput(tt.in)
		seen[name]++
		if seen[name] > 1 {
			name += " (" + strconv.Itoa(seen[name]) + ")"
		}
		t.Run(name, func(t *testing.T) {
			got, err := coerce(tt.as, tt.in)
			said := ""
			if err != nil {
				said = err.Error()
			}
			same := strings.Contains(said, tt.fails) &&
				(tt.fails != "" || (err == nil && sameScalarValue(got, tt.want)))

			if why, listed := knownScalarDivergences[name]; listed {
				if same {
					t.Errorf("this case now matches graphql-js; remove it from the known divergences (%s)", why)
				} else {
					t.Logf("known divergence: %s", why)
				}
				return
			}
			switch {
			case tt.fails != "" && !strings.Contains(said, tt.fails):
				t.Errorf("said %q, want %q", said, tt.fails)
			case tt.fails == "" && err != nil:
				t.Errorf("said %q, want %#v", said, tt.want)
			case tt.fails == "" && !sameScalarValue(got, tt.want):
				t.Errorf("came back as %#v, want %#v", got, tt.want)
			}
		})
	}
}

// describeScalarInput names a case by what was passed in.
func describeScalarInput(in any) string {
	if held, isFloat := in.(float64); isFloat && math.IsNaN(held) {
		return "NaN"
	}
	return value.Describe(in)
}

func sameScalarValue(got, want any) bool {
	if left, isFloat := got.(float64); isFloat && math.IsNaN(left) {
		right, isFloat := want.(float64)
		return isFloat && math.IsNaN(right)
	}
	return got == want
}
