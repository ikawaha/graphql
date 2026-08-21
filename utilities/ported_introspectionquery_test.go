package utilities_test

// Ported from graphql-js src/utilities/__tests__/getIntrospectionQuery-test.ts.
//
// Two things are checked of every query this can produce: that it is a valid
// document against a schema, which is the point of the whole exercise, and
// that each option puts in or leaves out the field it is about. graphql-js
// also counts how many times each field appears; that is a fact about the
// shape of its own query text rather than about the option, so what is checked
// here is presence.

import (
	"strings"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/utilities"
	"github.com/ikawaha/graphql/validation"
)

func TestPortedIntrospectionQuery(t *testing.T) {
	for _, tt := range []struct {
		name string
		opts []utilities.IntrospectionOption
		// present and absent are what the query should and should not mention.
		present, absent []string
	}{
		{
			name:    "by default",
			present: []string{"description", "isDeprecated", "deprecationReason"},
			absent:  []string{"isRepeatable", "specifiedByURL", "isOneOf"},
		},
		{
			name:   "without descriptions",
			opts:   []utilities.IntrospectionOption{utilities.WithoutDescriptions()},
			absent: []string{"description"},
		},
		{
			name:    "asking whether a directive may be repeated",
			opts:    []utilities.IntrospectionOption{utilities.WithDirectiveIsRepeatable()},
			present: []string{"isRepeatable"},
		},
		{
			name:    "asking for the schema's own description",
			opts:    []utilities.IntrospectionOption{utilities.WithSchemaDescription()},
			present: []string{"description"},
		},
		{
			name:   "asking for the schema's own description with no descriptions",
			opts:   []utilities.IntrospectionOption{utilities.WithSchemaDescription(), utilities.WithoutDescriptions()},
			absent: []string{"description"},
		},
		{
			name:    "asking where a scalar is specified",
			opts:    []utilities.IntrospectionOption{utilities.WithSpecifiedByURL()},
			present: []string{"specifiedByURL"},
		},
		{
			name:    "asking about deprecated arguments and input fields",
			opts:    []utilities.IntrospectionOption{utilities.WithInputValueDeprecation()},
			present: []string{"isDeprecated", "deprecationReason", "includeDeprecated: true"},
		},
		{
			name:    "asking which input objects take one field",
			opts:    []utilities.IntrospectionOption{utilities.WithOneOf()},
			present: []string{"isOneOf"},
		},
		{
			name: "asking for everything",
			opts: []utilities.IntrospectionOption{utilities.WithEverything()},
			present: []string{
				"description", "isRepeatable", "specifiedByURL", "isOneOf",
				"isDeprecated", "deprecationReason", "includeDeprecated: true",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			query := utilities.IntrospectionQuery(tt.opts...)

			// Whatever it asks for, it has to be a document the schema can
			// answer: a query that does not validate would fail at the one
			// moment it is needed.
			doc, err := language.ParseString(query)
			if err != nil {
				t.Fatalf("parsing the introspection query: %v\n%s", err, query)
			}
			s := mustBuild(t, `type Query { dummy: String }`)
			if errs := validation.Validate(s, doc, validation.SpecifiedRules...); len(errs) > 0 {
				t.Fatalf("the introspection query does not validate: %v\n%s", errs, query)
			}

			for _, text := range tt.present {
				if !strings.Contains(query, text) {
					t.Errorf("the query does not mention %q", text)
				}
			}
			for _, text := range tt.absent {
				if strings.Contains(query, text) {
					t.Errorf("the query mentions %q", text)
				}
			}
		})
	}
}

// TestPortedIntrospectionQuery_TypeDepth is graphql-js's typeDepth option:
// how many wrappers a type reference is asked to unfold. graphql-js throws
// above a hundred; an option cannot fail here, so the depth is left at the
// default instead.
func TestPortedIntrospectionQuery_TypeDepth(t *testing.T) {
	count := func(q string) int { return strings.Count(q, "ofType {") }

	if got, want := count(utilities.IntrospectionQuery()), 9; got != want {
		t.Errorf("by default unfolds %d levels, want %d", got, want)
	}
	for _, depth := range []int{0, 1, 3, 100} {
		if got := count(utilities.IntrospectionQuery(utilities.WithTypeDepth(depth))); got != depth {
			t.Errorf("WithTypeDepth(%d) unfolds %d levels", depth, got)
		}
	}
	for _, depth := range []int{101, -1} {
		if got, want := count(utilities.IntrospectionQuery(utilities.WithTypeDepth(depth))), 9; got != want {
			t.Errorf("WithTypeDepth(%d) unfolds %d levels, want the default %d", depth, got, want)
		}
	}
	// Asking for everything says nothing about how deep to unfold.
	q := utilities.IntrospectionQuery(utilities.WithTypeDepth(2), utilities.WithEverything())
	if got, want := count(q), 2; got != want {
		t.Errorf("with everything asked for: unfolds %d levels, want %d", got, want)
	}
}
