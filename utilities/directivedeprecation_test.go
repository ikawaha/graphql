package utilities_test

// A directive can be deprecated, which is v17's addition to @deprecated. The
// reason has to survive every place a schema is taken apart and put back
// together.

import (
	"context"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

const deprecatedDirectiveSDL = `directive @old @deprecated(reason: "Use @new.") on FIELD

directive @new on FIELD

type Query {
  f: String
}`

func TestDirectiveDeprecation_SurvivesEveryRebuild(t *testing.T) {
	original, err := utilities.BuildSchema(deprecatedDirectiveSDL)
	if err != nil {
		t.Fatalf("building: %v", err)
	}
	if got := utilities.PrintSchema(original); got != deprecatedDirectiveSDL {
		t.Errorf("printed\n%s\nwant\n%s", got, deprecatedDirectiveSDL)
	}
	expectDeprecatedDirective(t, "as built", original)

	extended, err := utilities.ExtendSchemaSource(original, "extend type Query { g: String }")
	if err != nil {
		t.Fatalf("extending: %v", err)
	}
	expectDeprecatedDirective(t, "after extending", extended)

	expectDeprecatedDirective(t, "after sorting", utilities.LexicographicSortSchema(original))

	answer, err := utilities.IntrospectionFromSchema(context.Background(), original)
	if err != nil {
		t.Fatalf("introspecting: %v", err)
	}
	rebuilt, err := utilities.BuildClientSchema(answer)
	if err != nil {
		t.Fatalf("rebuilding: %v", err)
	}
	expectDeprecatedDirective(t, "after a round trip through introspection", rebuilt)
}

func expectDeprecatedDirective(t *testing.T, after string, s *schema.Schema) {
	t.Helper()
	old := s.Directive("old")
	if old == nil {
		t.Fatalf("%s: the directive is gone", after)
	}
	if !old.IsDeprecated() || old.DeprecationReason.Or("") != "Use @new." {
		t.Errorf("%s: @old reports %q, want %q", after, old.DeprecationReason, "Use @new.")
	}
	if current := s.Directive("new"); current == nil {
		t.Errorf("%s: @new is gone", after)
	} else if current.IsDeprecated() {
		t.Errorf("%s: @new reports itself deprecated", after)
	}
}

// Asking about a directive's deprecation is opt-in, because a server built
// against an older specification cannot answer the question at all. A query
// that does not ask does not hear about the deprecated directive either.
func TestDirectiveDeprecation_IsOptInForIntrospection(t *testing.T) {
	asked := utilities.IntrospectionQuery()
	if strings.Contains(asked, "deprecationReason\n      locations") {
		t.Error("the default query asks whether a directive is deprecated")
	}
	if !strings.Contains(utilities.IntrospectionQuery(utilities.WithDirectiveDeprecation()),
		"directives(includeDeprecated: true)") {
		t.Error("the option does not ask for the deprecated directives")
	}
}
