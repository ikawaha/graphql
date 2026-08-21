package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
	"github.com/ikawaha/graphql/validation"
)

func deprecatedSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := utilities.BuildSchema(`
		enum Colour {
			CURRENT
			OLD @deprecated(reason: "Use CURRENT.")
		}
		input Filter {
			current: String
			old: String @deprecated(reason: "Use current.")
		}
		type Query {
			current: String
			old: String @deprecated(reason: "Use current.")
			withArgs(
				current: String
				old: String @deprecated(reason: "Use current.")
			): String
			byColour(colour: Colour): String
			byFilter(filter: Filter): String
		}
		directive @tag(old: String @deprecated(reason: "Use nothing.")) on FIELD
	`)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	if err := schema.AssertValidSchema(s); err != nil {
		t.Fatalf("the schema is not sound: %v", err)
	}
	return s
}

func TestNoDeprecatedCustom(t *testing.T) {
	s := deprecatedSchema(t)
	rule := validation.NoDeprecatedCustomRule

	t.Run("nothing deprecated", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				current
				withArgs(current: "a")
				byColour(colour: CURRENT)
				byFilter(filter: { current: "a" })
			}
		`)
	})

	t.Run("a deprecated field", func(t *testing.T) {
		expectErrors(t, s, rule, `{ old }`,
			want{Message: "The field Query.old is deprecated. Use current.", At: []at{{1, 3}}},
		)
	})

	t.Run("a deprecated argument", func(t *testing.T) {
		expectErrors(t, s, rule, `{ withArgs(old: "a") }`,
			want{Message: `The argument "Query.withArgs(old:)" is deprecated. Use current.`, At: []at{{1, 12}}},
		)
	})

	t.Run("a deprecated directive argument", func(t *testing.T) {
		expectErrors(t, s, rule, `{ current @tag(old: "a") }`,
			want{Message: `The argument "@tag(old:)" is deprecated. Use nothing.`, At: []at{{1, 16}}},
		)
	})

	t.Run("a deprecated enum member", func(t *testing.T) {
		expectErrors(t, s, rule, `{ byColour(colour: OLD) }`,
			want{Message: `The enum value "Colour.OLD" is deprecated. Use CURRENT.`, At: []at{{1, 20}}},
		)
	})

	t.Run("a deprecated input field", func(t *testing.T) {
		expectErrors(t, s, rule, `{ byFilter(filter: { old: "a" }) }`,
			want{Message: "The input field Filter.old is deprecated. Use current.", At: []at{{1, 22}}},
		)
	})
}

func TestNoSchemaIntrospectionCustom(t *testing.T) {
	s := testSchema(t)
	rule := validation.NoSchemaIntrospectionCustomRule

	t.Run("an ordinary query", func(t *testing.T) {
		expectValid(t, s, rule, `{ dog { name } }`)
	})

	// __typename is not one of the introspection types: it returns a String.
	t.Run("__typename is allowed", func(t *testing.T) {
		expectValid(t, s, rule, `{ dog { __typename } }`)
	})

	// Every field whose type is an introspection type is reported, not just
	// the one that got there: each is a way of reading the schema.
	t.Run("a schema introspection query", func(t *testing.T) {
		expectErrors(t, s, rule, `{ __schema { queryType { name } } }`,
			want{Message: `the requested query contained the field "__schema"`, At: []at{{1, 3}}},
			want{Message: `the requested query contained the field "queryType"`, At: []at{{1, 14}}},
		)
	})

	t.Run("a type introspection query", func(t *testing.T) {
		expectErrors(t, s, rule, `{ __type(name: "Dog") { name } }`,
			want{Message: `contained the field "__type"`, At: []at{{1, 3}}},
		)
	})
}
