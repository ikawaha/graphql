package graphql_test

import (
	"context"
	"slices"
	"testing"

	"github.com/ikawaha/graphql"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/validation"
)

// TestParams_Rules covers the three things a caller can say about which rules
// a document is checked against.
//
// Nil is "the ones the specification requires" and an empty list is "none",
// which is the distinction [schema.Config.Directives] and
// [validation.WithRules] already draw, and the one graphql-js draws with its
// own rules argument.
func TestParams_Rules(t *testing.T) {
	s, err := graphql.BuildSchema(`type Query { greeting: String }`)
	if err != nil {
		t.Fatal(err)
	}
	s.QueryType().Field("greeting").Resolve = func(
		context.Context, any, graphql.Arguments, *graphql.ResolveInfo,
	) (any, error) {
		return "hello", nil
	}

	// A field the schema does not have, which the specified rules refuse.
	const query = `{ nope }`

	tests := []struct {
		name       string
		rules      []validation.Rule
		wantErrors int
	}{
		{"nil: the specified rules", nil, 1},
		{"empty but not nil: none", []validation.Rule{}, 0},
		{"one rule of its own", []validation.Rule{validation.FieldsOnCorrectTypeRule}, 1},
		{
			"the specified rules and one more",
			append(slices.Clone(validation.SpecifiedRules), validation.NoDeprecatedCustomRule),
			1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := graphql.Do(context.Background(), graphql.Params{
				Schema: s, Query: query, Rules: test.rules,
			})
			if len(result.Errors) != test.wantErrors {
				t.Errorf("%d errors, wanted %d: %v",
					len(result.Errors), test.wantErrors, result.Errors)
			}
		})
	}
}

// TestParams_RulesMatchesTheOtherTwoPlaces pins that the three places a caller
// meets this distinction agree, since disagreeing is the trap.
func TestParams_RulesMatchesTheOtherTwoPlaces(t *testing.T) {
	s, err := graphql.BuildSchema(`type Query { greeting: String }`)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := language.ParseString(`{ nope }`)
	if err != nil {
		t.Fatal(err)
	}

	if got := len(validation.ValidateWithOptions(s, doc)); got != 1 {
		t.Errorf("no options gave %d errors, wanted 1", got)
	}
	if got := len(validation.ValidateWithOptions(s, doc, validation.WithRules())); got != 0 {
		t.Errorf("an empty WithRules gave %d errors, wanted 0", got)
	}
	if got := len(schema.New(schema.Config{}).Directives()); got == 0 {
		t.Error("nil directives gave none, wanted the specified ones")
	}
	if got := len(schema.New(schema.Config{Directives: []*schema.Directive{}}).Directives()); got != 0 {
		t.Errorf("an empty directive list gave %d, wanted none", got)
	}
}
