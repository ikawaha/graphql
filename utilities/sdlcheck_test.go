package utilities_test

import (
	"strings"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

// A document is checked against the rules a schema definition must follow
// before anything is built from it, which is what graphql-js does and what
// this package has no way of doing for itself: the rules live in the
// validation package, and every one of them was reachable only by a caller who
// knew to ask. One case per rule, since the check is wired in once and a rule
// left out of the set would show up nowhere else.
func TestBuildSchema_ChecksTheDocumentFirst(t *testing.T) {
	tests := []struct {
		rule string
		sdl  string
		says string
	}{
		{
			"LoneSchemaDefinition",
			"schema { query: Query }\nschema { query: Query }\ntype Query { f: Int }",
			"Must provide only one schema definition.",
		},
		{
			"UniqueOperationTypes",
			"schema { query: Query query: Query }\ntype Query { f: Int }",
			"There can be only one query type in schema.",
		},
		{
			"UniqueTypeNames",
			"type Foo { x: Int }\ntype Foo { y: Int }\ntype Query { f: Foo }",
			`There can be only one type named "Foo".`,
		},
		{
			// A built-in is a type like any other here: the builder drops a
			// redefinition of one, so nothing else would report a document
			// that holds two.
			"UniqueTypeNames, for a built-in",
			"scalar ID\nscalar ID\ntype Query { f: ID }",
			`There can be only one type named "ID".`,
		},
		{
			"UniqueEnumValueNames",
			"enum E { A A }\ntype Query { f: E }",
			`Enum value "E.A" can only be defined once.`,
		},
		{
			"UniqueFieldDefinitionNames",
			"type Query { f: Int f: Int }",
			`Field "Query.f" can only be defined once.`,
		},
		{
			"UniqueArgumentDefinitionNames",
			"type Query { f(a: Int, a: Int): Int }",
			`Argument "Query.f(a:)" can only be defined once.`,
		},
		{
			"UniqueDirectiveNames",
			"directive @tag on OBJECT\ndirective @tag on OBJECT\ntype Query { f: Int }",
			`There can be only one directive named "@tag".`,
		},
		{
			"KnownTypeNames",
			"type Query { f: Missing }",
			`Unknown type "Missing".`,
		},
		{
			"KnownDirectives",
			"type Query { f: Int @nope }",
			`Unknown directive "@nope".`,
		},
		{
			"UniqueDirectivesPerLocation",
			"directive @tag on FIELD_DEFINITION\ntype Query { f: Int @tag @tag }",
			`The directive "@tag" can only be used once at this location.`,
		},
		{
			"PossibleTypeExtensions",
			"extend type Missing { x: Int }\ntype Query { f: Int }",
			`Cannot extend type "Missing" because it is not defined.`,
		},
		{
			"KnownArgumentNamesOnDirectives",
			"directive @tag(a: Int) on FIELD_DEFINITION\ntype Query { f: Int @tag(bogus: 1) }",
			`Unknown argument "bogus" on directive "@tag".`,
		},
		{
			"UniqueArgumentNames",
			"directive @tag(a: Int) on FIELD_DEFINITION\ntype Query { f: Int @tag(a: 1, a: 2) }",
			`There can be only one argument named "a".`,
		},
		{
			"UniqueInputFieldNames",
			"input In { x: Int }\ndirective @tag(a: In) on FIELD_DEFINITION\ntype Query { f: Int @tag(a: {x: 1, x: 2}) }",
			`There can be only one input field named "x".`,
		},
		{
			"ProvidedRequiredArgumentsOnDirectives",
			"directive @need(a: Int!) on FIELD_DEFINITION\ntype Query { f: Int @need }",
			`Argument "@need(a:)" of type "Int!" is required, but it was not provided.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.rule, func(t *testing.T) {
			_, err := utilities.BuildSchema(tt.sdl)
			if err == nil {
				t.Fatal("built without complaint")
			}
			if !strings.Contains(err.Error(), tt.says) {
				t.Errorf("error = %v\nwant it to say %q", err, tt.says)
			}

			// The same document with the check waived is the builder's own
			// problem, and whatever it makes of it, it is not this.
			t.Run("waived", func(t *testing.T) {
				_, err := utilities.BuildSchema(tt.sdl, utilities.AssumeValidSDL())
				if err != nil && strings.Contains(err.Error(), tt.says) {
					t.Errorf("the check ran anyway: %v", err)
				}
			})
		})
	}
}

// The same check runs over a document applied to a schema that already exists,
// where what may be defined depends on what is already there.
func TestExtendSchema_ChecksTheDocumentFirst(t *testing.T) {
	// The schema must name ID for ID to be a type it has: what may not be
	// redefined is what is in the schema, not what is built in.
	base := mustBuild(t, "directive @tag on OBJECT\ntype Foo { x: ID }\ntype Query { f: Foo }")
	for _, tt := range []struct{ name, sdl, says string }{
		{"a type the schema has", "type Foo { y: Int }",
			`Type "Foo" already exists in the schema. It cannot also be defined in this type definition.`},
		{"a built-in the schema has", "scalar ID",
			`Type "ID" already exists in the schema. It cannot also be defined in this type definition.`},
		{"a directive the schema has", "directive @tag on OBJECT",
			`Directive "@tag" already exists in the schema. It cannot be redefined.`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := utilities.ExtendSchemaSource(base, tt.sdl)
			if err == nil {
				t.Fatal("extended without complaint")
			}
			if !strings.Contains(err.Error(), tt.says) {
				t.Errorf("error = %v\nwant it to say %q", err, tt.says)
			}
			if _, err := utilities.ExtendSchemaSource(base, tt.sdl, utilities.AssumeValidSDL()); err != nil {
				if strings.Contains(err.Error(), tt.says) {
					t.Errorf("the check ran anyway: %v", err)
				}
			}
		})
	}
}

// AssumeValid waives the check of the document as well as the check of the
// schema it produces, which is how graphql-js reads the two together.
func TestBuildSchema_AssumeValid(t *testing.T) {
	// Sound as a document, unsound as a schema: an object with no fields.
	const sdl = "type Query"

	s, err := utilities.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("building: %v", err)
	}
	if errs := schema.ValidateSchema(s); len(errs) == 0 {
		t.Fatal("the schema was found sound, so there is nothing for AssumeValid to waive")
	}

	taken, err := utilities.BuildSchema(sdl, utilities.AssumeValid())
	if err != nil {
		t.Fatalf("building with AssumeValid: %v", err)
	}
	if errs := schema.ValidateSchema(taken); len(errs) != 0 {
		t.Errorf("ValidateSchema = %v, want nothing said about a schema taken on trust", errs)
	}

	t.Run("and the document check with it", func(t *testing.T) {
		// A field defined twice: refused by the document check alone.
		const twice = "type Query { f: Int f: Int }"
		if _, err := utilities.BuildSchema(twice); err == nil {
			t.Fatal("the document check did not run")
		}
		if _, err := utilities.BuildSchema(twice, utilities.AssumeValid()); err != nil {
			t.Errorf("AssumeValid did not waive the document check: %v", err)
		}
	})
}

// Options for the parser and options for the builder arrive in the same bag,
// as they do in graphql-js.
func TestBuildSchema_WithParseOptions(t *testing.T) {
	const sdl = "type Query { f: Int }"

	if _, err := utilities.BuildSchema(sdl, utilities.WithParseOptions(language.MaxTokens(3))); err == nil {
		t.Error("a limit the document goes past was not applied")
	}
	if _, err := utilities.BuildSchema(sdl, utilities.WithParseOptions(language.MaxTokens(1000))); err != nil {
		t.Errorf("a limit the document stays within refused it: %v", err)
	}
}
