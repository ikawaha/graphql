package schema_test

// What a type gives back must build the same type again, and changing what
// comes back must not reach into the type it came from.
//
// graphql-js has toConfig on every type for the same purpose; its own tests
// are "can be converted to a configuration object" in
// src/type/__tests__/definition-test.ts.

import (
	"context"
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

func TestToConfig_RoundTrips(t *testing.T) {
	const sdl = `
		"a schema" schema { query: Query }
		directive @tag(name: String! = "x") repeatable on FIELD_DEFINITION
		scalar Weekday @specifiedBy(url: "https://example.com/weekday")
		"an enum" enum Colour { RED GREEN @deprecated(reason: "faded") }
		interface Node { id: ID! }
		"an input" input Filter @oneOf { term: String other: String }
		type Photo implements Node { id: ID! url: String }
		type User implements Node { id: ID! name(upper: Boolean = false): String }
		union Media = Photo | User
		type Query { node(id: ID!): Node media: Media colour: Colour when: Weekday find(f: Filter): String }
	`
	s, err := utilities.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}

	// Every type rebuilt from what it gives back prints as it did.
	for _, name := range []string{"Weekday", "Colour", "Node", "Filter", "Photo", "User", "Media"} {
		t.Run(name, func(t *testing.T) {
			var rebuilt schema.NamedType
			switch original := s.Type(name).(type) {
			case *schema.ScalarType:
				rebuilt = schema.NewScalar(original.ToConfig())
			case *schema.EnumType:
				rebuilt = schema.NewEnum(original.ToConfig())
			case *schema.InterfaceType:
				rebuilt = schema.NewInterface(original.ToConfig())
			case *schema.InputObjectType:
				rebuilt = schema.NewInputObject(original.ToConfig())
			case *schema.ObjectType:
				rebuilt = schema.NewObject(original.ToConfig())
			case *schema.UnionType:
				rebuilt = schema.NewUnion(original.ToConfig())
			default:
				t.Fatalf("%s is a %T", name, original)
			}
			if got, want := utilities.PrintType(rebuilt), utilities.PrintType(s.Type(name)); got != want {
				t.Errorf("rebuilt as\n%s\nwant\n%s", got, want)
			}
		})
	}

	t.Run("the whole schema", func(t *testing.T) {
		rebuilt := schema.New(s.ToConfig())
		if err := schema.AssertValidSchema(rebuilt); err != nil {
			t.Fatalf("the rebuilt schema is not sound:\n%v", err)
		}
		if got, want := utilities.PrintSchema(rebuilt), utilities.PrintSchema(s); got != want {
			t.Errorf("rebuilt as\n%s\nwant\n%s", got, want)
		}
	})

	t.Run("a directive", func(t *testing.T) {
		tag := s.Directive("tag")
		rebuilt := schema.NewDirective(tag.ToConfig())
		if rebuilt.Name() != "tag" || !rebuilt.IsRepeatable || len(rebuilt.Args) != 1 {
			t.Errorf("rebuilt as %v", rebuilt)
		}
		if got := rebuilt.Args[0].Default; !got.IsSet() {
			t.Error("the argument lost its default")
		}
	})
}

// A configuration is the caller's to change. Changing it must not reach into
// the type it came from.
func TestToConfig_IsTheCallersOwn(t *testing.T) {
	s, err := utilities.BuildSchema(`type Query { a: String b: String }`)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	query := s.QueryType()

	config := query.ToConfig()
	config.Name = "Other"
	config.Fields = append(config.Fields,
		schema.NewField("c", schema.FieldConfig{Type: schema.String}))
	derived := schema.NewObject(config)

	if query.Name() != "Query" || len(query.Fields()) != 2 || query.Field("c") != nil {
		t.Errorf("the type the configuration came from was changed: %v", utilities.PrintType(query))
	}
	if derived.Name() != "Other" || len(derived.Fields()) != 3 {
		t.Errorf("the derived type is %v", utilities.PrintType(derived))
	}
	// The derived type holds fields of its own, so wiring one up leaves the
	// type it was derived from alone.
	derived.Field("a").Resolve = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		return "derived", nil
	}
	if query.Field("a").Resolve != nil {
		t.Error("wiring up the derived type reached the original")
	}
}
