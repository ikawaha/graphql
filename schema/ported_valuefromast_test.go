package schema_test

// Ported from graphql-js src/utilities/__tests__/valueFromASTUntyped-test.ts:
// a literal reduced to a plain Go value without consulting a type.

import (
	"math"
	"reflect"
	"testing"

	"github.com/ikawaha/graphql/schema"
)

func TestPortedValueFromASTUntyped(t *testing.T) {
	held := func(name string, v any) map[string]any { return map[string]any{name: v} }

	for _, tt := range []struct {
		name      string
		literal   string
		variables map[string]any
		want      any
		// fits says whether a value came back at all. A variable nothing was
		// supplied for gives none.
		fits bool
	}{
		{name: "null", literal: "null", fits: true},
		{name: "true", literal: "true", want: true, fits: true},
		{name: "false", literal: "false", want: false, fits: true},
		{name: "an integer", literal: "123", want: int64(123), fits: true},
		{name: "a number with a point", literal: "123.456", want: 123.456, fits: true},
		{name: "a string", literal: `"abc123"`, want: "abc123", fits: true},

		{name: "a list", literal: "[true, false]", want: []any{true, false}, fits: true},
		{name: "a list of mixed values", literal: "[true, 123.45]",
			want: []any{true, 123.45}, fits: true},
		{name: "a list holding null", literal: "[true, null]", want: []any{true, nil}, fits: true},
		{name: "a list holding a list", literal: `[true, ["foo", 1.2]]`,
			want: []any{true, []any{"foo", 1.2}}, fits: true},

		{name: "an object", literal: "{ int: 123, bool: false }",
			want: map[string]any{"int": int64(123), "bool": false}, fits: true},
		{name: "an object holding a list of objects", literal: `{ foo: [ { bar: "baz"} ] }`,
			want: map[string]any{"foo": []any{map[string]any{"bar": "baz"}}}, fits: true},

		// With no type to read them against, enum members are plain strings.
		{name: "a member name", literal: "TEST_ENUM_VALUE", want: "TEST_ENUM_VALUE", fits: true},
		{name: "a list of member names", literal: "[TEST_ENUM_VALUE]",
			want: []any{"TEST_ENUM_VALUE"}, fits: true},

		{name: "a variable", literal: "$testVariable",
			variables: held("testVariable", "foo"), want: "foo", fits: true},
		{name: "a variable in a list", literal: "[$testVariable]",
			variables: held("testVariable", "foo"), want: []any{"foo"}, fits: true},
		{name: "a variable in an object", literal: "{a:[$testVariable]}",
			variables: held("testVariable", "foo"),
			want:      map[string]any{"a": []any{"foo"}}, fits: true},
		{name: "a variable holding null", literal: "$testVariable",
			variables: held("testVariable", nil), fits: true},
		{name: "a variable holding a number that is not one", literal: "$testVariable",
			variables: held("testVariable", math.NaN()), want: math.NaN(), fits: true},
		{name: "a variable nothing was supplied for", literal: "$testVariable",
			variables: map[string]any{}},
		{name: "a variable with no variables at all", literal: "$testVariable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, fits := schema.ValueFromASTUntyped(literalOf(t, tt.literal), schema.NewVariableValues(tt.variables, nil))
			if fits != tt.fits {
				t.Fatalf("fits = %v, want %v (value %#v)", fits, tt.fits, got)
			}
			if !fits {
				return
			}
			if left, isFloat := got.(float64); isFloat && math.IsNaN(left) {
				if right, isFloat := tt.want.(float64); !isFloat || !math.IsNaN(right) {
					t.Errorf("read as %#v, want %#v", got, tt.want)
				}
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("read as %#v, want %#v", got, tt.want)
			}
		})
	}
}
