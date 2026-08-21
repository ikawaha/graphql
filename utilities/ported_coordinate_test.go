package utilities_test

// Ported from graphql-js src/utilities/__tests__/resolveSchemaCoordinate-test.ts.
//
// A coordinate either names something, names nothing, or is not a question
// about this schema at all — a type it does not have, or a type of the wrong
// sort for what follows the dot. The third is an error rather than an empty
// answer.

import (
	"testing"

	"github.com/ikawaha/graphql/utilities"
)

func TestPortedResolveSchemaCoordinate(t *testing.T) {
	s := mustBuild(t, `
  type Query {
    searchBusiness(criteria: SearchCriteria!): [Business]
  }

  input SearchCriteria {
    name: String
    filter: SearchFilter
  }

  enum SearchFilter {
    OPEN_NOW
    DELIVERS_TAKEOUT
    VEGETARIAN_MENU
  }

  type Business {
    id: ID
    name: String
    email: String @private(scope: "loggedIn")
  }

  directive @private(scope: String!) on FIELD_DEFINITION`)

	// what says which member of the answer should be filled in, or that there
	// should be no answer at all.
	const (
		nothing      = "nothing"
		refused      = "refused"
		namedType    = "type"
		field        = "field"
		inputField   = "input field"
		enumValue    = "enum value"
		directive    = "directive"
		argument     = "argument"
		directiveArg = "directive argument"
	)

	for _, tt := range []struct{ coordinate, what, named string }{
		{"Business", namedType, "Business"},
		{"String", namedType, "String"},
		{"private", nothing, ""},
		{"Unknown", nothing, ""},

		{"Business.name", field, "name"},
		{"Business.unknown", nothing, ""},
		{"Unknown.field", refused, ""},
		{"String.field", refused, ""},

		{"SearchCriteria.filter", inputField, "filter"},
		{"SearchCriteria.unknown", nothing, ""},

		{"SearchFilter.OPEN_NOW", enumValue, "OPEN_NOW"},
		{"SearchFilter.UNKNOWN", nothing, ""},

		{"Query.searchBusiness(criteria:)", argument, "criteria"},
		{"Business.name(unknown:)", nothing, ""},
		{"Unknown.field(arg:)", refused, ""},
		{"Business.unknown(arg:)", refused, ""},
		{"SearchCriteria.name(arg:)", refused, ""},

		{"@private", directive, "private"},
		{"@deprecated", directive, "deprecated"},
		{"@unknown", nothing, ""},
		{"@Business", nothing, ""},
		{"@private(scope:)", directiveArg, "scope"},
		{"@private(unknown:)", nothing, ""},
		{"@unknown(arg:)", refused, ""},

		// Introspection is part of every schema, so it can be pointed at too.
		{"Business.__typename", field, "__typename"},
		{"Query.__type(name:)", argument, "name"},
		{"__Type", namedType, "__Type"},
		{"__Directive.name", field, "name"},
		{"__DirectiveLocation.INLINE_FRAGMENT", enumValue, "INLINE_FRAGMENT"},
	} {
		t.Run(tt.coordinate, func(t *testing.T) {
			got, err := utilities.ResolveSchemaCoordinate(s, tt.coordinate)
			if tt.what == refused {
				if err == nil {
					t.Errorf("resolved to %+v, want a refusal", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolving: %v", err)
			}
			if tt.what == nothing {
				if got != nil {
					t.Errorf("resolved to %+v, want nothing", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("resolved to nothing, want the %s %q", tt.what, tt.named)
			}
			var name string
			switch tt.what {
			case namedType:
				if got.Type != nil {
					name = got.Type.Name()
				}
			case field:
				if got.Field != nil {
					name = got.Field.Name()
				}
			case inputField:
				if got.InputField != nil {
					name = got.InputField.Name()
				}
			case enumValue:
				if got.EnumValue != nil {
					name = got.EnumValue.Name()
				}
			case directive:
				if got.Directive != nil {
					name = got.Directive.Name()
				}
			case argument, directiveArg:
				if got.Argument != nil {
					name = got.Argument.Name()
				}
			}
			if name != tt.named {
				t.Errorf("resolved to %+v, want the %s %q", got, tt.what, tt.named)
			}
		})
	}
}
