package schema_test

// Ported from graphql-js src/utilities/__tests__/replaceVariables-test.ts.
//
// Upstream takes the operation's variables and a fragment's separately, and
// works the fragment's out of unresolved literals as it goes. Here a fragment
// spread settles its arguments where it is written, so what reaches this is
// one map — the scope the literal was found under — and the two halves of the
// upstream file become one. The cases about a fragment variable overriding an
// operation variable are the scope having already been merged, and are kept
// as such.

import (
	"github.com/ikawaha/graphql/value"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

func TestPortedReplaceVariables(t *testing.T) {
	for _, tt := range []struct {
		name      string
		literal   string
		variables map[string]any
		want      string
		// same says the very literal handed in comes back, there being
		// nothing to replace.
		same bool
	}{
		{name: "does not change a simple value", literal: "null", want: "null", same: true},
		{name: "does not change a value holding no variable",
			literal: `{ foo: [1, "two"] }`, want: `{ foo: [1, "two"] }`, same: true},

		{name: "replaces a simple variable", literal: "$var",
			variables: map[string]any{"var": 123}, want: "123"},
		{name: "replaces a variable that fell back to its default", literal: "$var",
			// The executor has already put the default in the map.
			variables: map[string]any{"var": 123}, want: "123"},
		{name: "replaces nested variables", literal: "{ foo: [ $var ], bar: $var }",
			variables: map[string]any{"var": 123}, want: "{ foo: [123], bar: 123 }"},

		{name: "replaces a missing variable with null", literal: "$var", want: "null"},
		{name: "replaces a variable that was never declared with null", literal: "$var",
			variables: map[string]any{}, want: "null"},
		{name: "replaces a misspelled variable with null", literal: "$var1",
			variables: map[string]any{"var2": 123}, want: "null"},
		{name: "replaces a missing variable in a list with null", literal: "[1, $var]",
			want: "[1, null]"},

		{name: "leaves a missing variable out of an object", literal: "{ foo: 1, bar: $var }",
			variables: map[string]any{"wrongVar": 123}, want: "{ foo: 1 }"},

		// The scope a fragment spread settled: what it supplied stands over
		// the request's own variable of the same name.
		{name: "the scope a fragment spread settled", literal: "$var",
			variables: map[string]any{"var": 456}, want: "456"},
		{name: "the scope a fragment spread settled, nested",
			literal:   "{ foo: [ $var ], bar: $var }",
			variables: map[string]any{"var": 456}, want: "{ foo: [456], bar: 456 }"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			literal := literalOf(t, tt.literal)
			got := schema.ReplaceVariables(literal, schema.NewVariableValues(tt.variables, nil))
			if tt.same && got != literal {
				t.Errorf("the literal was rebuilt though nothing needed replacing")
			}
			if printed := language.Print(got); printed != tt.want {
				t.Errorf("replaced to %s, want %s", printed, tt.want)
			}
			if names := variablesIn(got); len(names) > 0 {
				t.Errorf("the result still names variables: %v", names)
			}
		})
	}
}

// variablesIn collects the variables a literal still names, which after
// replacing should be none.
func variablesIn(literal language.Value) []string {
	var names []string
	language.Visit(literal, language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			if variable, isVariable := node.(*language.Variable); isVariable && variable.Name != nil {
				names = append(names, variable.Name.Value)
			}
			return language.VisitContinue
		},
	})
	return names
}

// A scalar that reads complex literals is given one with no variables left in
// it, which is what makes such a scalar writable at all: it would otherwise
// have to resolve variables itself, and to know that one the request left out
// is not the same as one given as null.
func TestReplaceVariables_ReachesACustomScalar(t *testing.T) {
	var seen string
	json := schema.NewScalar(schema.ScalarConfig{
		Name: "JSON",
		CoerceInputLiteral: func(literal language.Value) (value.Maybe[any], error) {
			seen = language.Print(literal)
			return value.Just[any](seen), nil
		},
	})

	got, ok := schema.CoerceInputLiteral(
		literalOf(t, `{ kept: $there, dropped: $missing, list: [$there, $missing] }`),
		json, schema.NewVariableValues(map[string]any{"there": 1}, nil))
	if !ok {
		t.Fatal("the literal was refused")
	}
	// The field whose variable was left out is gone; the entry of the list is
	// null, since a list has no way to leave one out.
	const want = `{ kept: 1, list: [1, null] }`
	if seen != want {
		t.Errorf("the scalar was given %s, want %s", seen, want)
	}
	if got != want {
		t.Errorf("the coercion answered %v", got)
	}
}

// TestPortedReplaceVariables_ReadsAgainstTheDeclaredType is what graphql-js's
// replaceVariables does with `valueToLiteral(value, signature.type)`: a value
// is written back out as the type it was declared as, not as whatever the Go
// value happens to look like. An enum member and a string are the same Go
// string, and only the type tells them apart.
func TestPortedReplaceVariables_ReadsAgainstTheDeclaredType(t *testing.T) {
	status := schema.NewEnum(schema.EnumConfig{
		Name:   "Status",
		Values: []*schema.EnumValue{schema.NewEnumValue("ACTIVE", schema.EnumValueConfig{})},
	})
	tests := []struct {
		name     string
		literal  string
		declared schema.Type
		want     string
	}{
		{"an enum member", "$v", status, "ACTIVE"},
		{"an enum member inside an object", "{ status: $v }", status, "{ status: ACTIVE }"},
		{"an enum member inside a list", "[$v]", status, "[ACTIVE]"},
		{"a string stays a string", "$v", schema.String, `"ACTIVE"`},
		// With no type to read it against, the value speaks for itself.
		{"no declared type", "$v", nil, `"ACTIVE"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			literal, err := language.ParseValue(language.NewSource(tt.literal))
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}
			types := map[string]schema.Type{}
			if tt.declared != nil {
				types["v"] = tt.declared
			}
			got := schema.ReplaceVariables(literal,
				schema.NewVariableValues(map[string]any{"v": "ACTIVE"}, types))
			if language.Print(got) != tt.want {
				t.Errorf("got %s, want %s", language.Print(got), tt.want)
			}
		})
	}
}
