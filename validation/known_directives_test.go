package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestKnownDirectives(t *testing.T) {
	s := testSchema(t)
	rule := validation.KnownDirectivesRule

	t.Run("no directives", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo {
				dog { name }
			}
		`)
	})

	t.Run("standard directives", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				dog @include(if: true) { name }
				cat @skip(if: false) { name }
			}
		`)
	})

	t.Run("directives in valid locations", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo($var: Boolean @onVariableDefinition) @onQuery {
				dog @onField { name @onField }
				...Frag @onFragmentSpread
				... @onInlineFragment { cat { name } }
			}
			fragment Frag on Dog @onFragmentDefinition {
				name
			}
		`)
	})

	t.Run("an unknown directive", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				dog @unknown(arg: "value") { name }
			}
		`,
			want{Message: `Unknown directive "@unknown".`, At: []at{{2, 6}}},
		)
	})

	t.Run("directives in the wrong place", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query Foo @onField {
				name @onQuery
				...Frag @onQuery
			}
		`,
			want{Message: `Directive "@onField" may not be used on QUERY.`, At: []at{{1, 11}}},
			want{Message: `Directive "@onQuery" may not be used on FIELD.`, At: []at{{2, 7}}},
			want{Message: `Directive "@onQuery" may not be used on FRAGMENT_SPREAD.`, At: []at{{3, 10}}},
		)
	})

	// The same node is an argument in one place and an input field in another,
	// and the two locations are not interchangeable.
	t.Run("SDL locations", func(t *testing.T) {
		expectValidSDL(t, nil, rule, `
			type Query @onObject {
				field(arg: String @onArgumentDefinition): String @onFieldDefinition
			}
			input Filter @onInputObject {
				byName: String @onInputFieldDefinition
			}
			enum Colour @onEnum {
				RED @onEnumValue
			}
			interface Node @onInterface { id: ID }
			union U @onUnion = Query
			scalar S @onScalar
			schema @onSchema { query: Query }

			directive @onObject on OBJECT
			directive @onArgumentDefinition on ARGUMENT_DEFINITION
			directive @onFieldDefinition on FIELD_DEFINITION
			directive @onInputObject on INPUT_OBJECT
			directive @onInputFieldDefinition on INPUT_FIELD_DEFINITION
			directive @onEnum on ENUM
			directive @onEnumValue on ENUM_VALUE
			directive @onInterface on INTERFACE
			directive @onUnion on UNION
			directive @onScalar on SCALAR
			directive @onSchema on SCHEMA
		`)
	})

	t.Run("an argument directive used on an input field", func(t *testing.T) {
		expectSDLErrors(t, nil, rule, `
			directive @onArgumentDefinition on ARGUMENT_DEFINITION
			input Filter {
				byName: String @onArgumentDefinition
			}
		`,
			want{Message: `may not be used on INPUT_FIELD_DEFINITION`, At: []at{{3, 17}}},
		)
	})
}
