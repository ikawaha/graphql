package utilities_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

const mapSDL = `
	directive @internal on FIELD_DEFINITION
	scalar DateTime
	enum Colour { RED GREEN INTERNAL }
	interface Node { id: ID! }
	input Filter { term: String secret: String }
	type User implements Node {
		id: ID!
		name: String
		secret: String
		friends(first: Int, secret: String): [User!]
	}
	type Photo implements Node { id: ID! url: String }
	union Media = User | Photo
	type Query { me: User media: Media when: DateTime search(f: Filter): String }`

// A mapper with nothing to say gives back the same schema, which is the case
// everything else is a variation on.
func TestMapSchema_Unchanged(t *testing.T) {
	s := mustBuild(t, mapSDL)
	mapped := utilities.MapSchema(s, utilities.SchemaMapper{})

	if err := schema.AssertValidSchema(mapped); err != nil {
		t.Fatalf("the mapped schema is not sound:\n%v", err)
	}
	if got, want := utilities.PrintSchema(mapped), utilities.PrintSchema(s); got != want {
		t.Errorf("an empty mapper changed the schema:\ngot:\n%s\n\nwant:\n%s", got, want)
	}
	if utilities.MapSchema(nil, utilities.SchemaMapper{}) != nil {
		t.Error("mapping nothing gave something")
	}
}

// Hiding what a public API should not show is the use this exists for.
func TestMapSchema_LeavingThingsOut(t *testing.T) {
	s := mustBuild(t, mapSDL)
	mapped := utilities.MapSchema(s, utilities.SchemaMapper{
		Fields: func(_ schema.NamedType, fields []*schema.Field) []*schema.Field {
			return keepIf(fields, func(f *schema.Field) bool { return f.Name() != "secret" })
		},
		Arguments: func(_ string, args []*schema.Argument) []*schema.Argument {
			return keepIf(args, func(a *schema.Argument) bool { return a.Name() != "secret" })
		},
		InputFields: func(_ schema.NamedType, fields []*schema.InputField) []*schema.InputField {
			return keepIf(fields, func(f *schema.InputField) bool { return f.Name() != "secret" })
		},
		EnumValues: func(_ schema.NamedType, values []*schema.EnumValue) []*schema.EnumValue {
			return keepIf(values, func(v *schema.EnumValue) bool { return v.Name() != "INTERNAL" })
		},
	})

	if err := schema.AssertValidSchema(mapped); err != nil {
		t.Fatalf("the mapped schema is not sound:\n%v", err)
	}
	text := utilities.PrintSchema(mapped)
	if strings.Contains(text, "secret") || strings.Contains(text, "INTERNAL") {
		t.Errorf("something that should have been left out is still there:\n%s", text)
	}
	// What was not asked about is untouched.
	for _, wanted := range []string{"name: String", "term: String", "RED", "friends(first: Int)"} {
		if !strings.Contains(text, wanted) {
			t.Errorf("the output does not contain %q:\n%s", wanted, text)
		}
	}
	// The original is left alone.
	if !strings.Contains(utilities.PrintSchema(s), "secret") {
		t.Error("the original schema lost something")
	}
}

// A hook may add as well as remove, and what it adds is wired into the new
// schema like everything else.
func TestMapSchema_AddingThings(t *testing.T) {
	s := mustBuild(t, mapSDL)
	mapped := utilities.MapSchema(s, utilities.SchemaMapper{
		Fields: func(owner schema.NamedType, fields []*schema.Field) []*schema.Field {
			if owner.Name() != "User" {
				return fields
			}
			// The field's type is taken from the original schema; the
			// rebuilder points it at the new one.
			return append(append([]*schema.Field{}, fields...),
				schema.NewField("nickname", schema.FieldConfig{Type: schema.String}),
				schema.NewField("bestFriend", schema.FieldConfig{Type: s.Type("User")}))
		},
	})

	if err := schema.AssertValidSchema(mapped); err != nil {
		t.Fatalf("the mapped schema is not sound:\n%v", err)
	}
	user, _ := mapped.Type("User").(*schema.ObjectType)
	if user == nil {
		t.Fatal("User was lost")
	}
	if user.Field("nickname") == nil {
		t.Error("the added field is not there")
	}
	// A field added with a type from the original schema points at the new one.
	best := user.Field("bestFriend")
	if best == nil {
		t.Fatal("bestFriend is not there")
	}
	if best.Type != schema.Type(user) {
		t.Error("the added field points at the old User")
	}
}

// This is the property the whole rebuild turns on, and it is the same one
// ExtendSchema depends on: everything that mentioned a type must see the new
// one.
func TestMapSchema_ReferencesPointAtTheNewTypes(t *testing.T) {
	s := mustBuild(t, mapSDL)
	mapped := utilities.MapSchema(s, utilities.SchemaMapper{})

	user, _ := mapped.Type("User").(*schema.ObjectType)
	if user == nil {
		t.Fatal("User was lost")
	}
	if user == s.Type("User") {
		t.Fatal("the type was shared rather than rebuilt; the test proves nothing")
	}

	if mapped.QueryType().Field("me").Type != schema.Type(user) {
		t.Error("Query.me points at the old User")
	}
	// Through a wrapper, and through a self-reference.
	friends := user.Field("friends")
	if schema.NamedTypeOf(friends.Type) != schema.NamedType(user) {
		t.Error("User.friends points at the old User")
	}
	if got, want := friends.Type.String(), "[User!]"; got != want {
		t.Errorf("User.friends is %s, want %s", got, want)
	}
	// Through an interface and a union.
	if user.Interfaces()[0].Named() != mapped.Type("Node") {
		t.Error("User implements the old Node")
	}
	media, _ := mapped.Type("Media").(*schema.UnionType)
	if media.Types()[0].Named() != mapped.Type("User") {
		t.Error("Media holds the old User")
	}
	// And the schema knows the new types implement the new interface.
	var found bool
	for _, impl := range mapped.PossibleTypes(mapped.Type("Node").(schema.AbstractType)) {
		if impl == user {
			found = true
		}
	}
	if !found {
		t.Error("the new User was not indexed as an implementation of the new Node")
	}
}

// The types every schema has are shared rather than rebuilt, so a mapped
// schema still compares equal to schema.Int and the like.
func TestMapSchema_BuiltInsAreShared(t *testing.T) {
	s := mustBuild(t, `type Query { a: Int b: String }`)
	mapped := utilities.MapSchema(s, utilities.SchemaMapper{
		Arguments: func(_ string, args []*schema.Argument) []*schema.Argument { return args },
	})

	if mapped.Type("Int") != schema.NamedType(schema.Int) {
		t.Error("Int was rebuilt rather than shared")
	}
	if mapped.QueryType().Field("a").Type != schema.Type(schema.Int) {
		t.Error("a field of a built-in type no longer points at it")
	}
	// The built-in directives too, which is also what keeps them out of a
	// printed schema.
	if mapped.Directive("skip") != schema.Skip {
		t.Error("@skip was rebuilt rather than shared")
	}
	if strings.Contains(utilities.PrintSchema(mapped), "directive @skip") {
		t.Error("a printed schema listed a directive every schema has")
	}
}

// A resolver is part of what a schema is, and losing one in a rebuild would
// leave a schema that looks right and returns nothing.
func TestMapSchema_KeepsResolvers(t *testing.T) {
	s := mustBuild(t, `type Query { greeting: String }`)
	s.QueryType().Field("greeting").Resolve =
		func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
			return "hello", nil
		}

	mapped := utilities.MapSchema(s, utilities.SchemaMapper{})
	field := mapped.QueryType().Field("greeting")
	if field == nil || field.Resolve == nil {
		t.Fatal("the resolver was lost")
	}
	got, err := field.Resolve(context.Background(), nil, schema.Arguments{}, nil)
	if err != nil || got != "hello" {
		t.Errorf("the resolver returned (%v, %v), want (hello, nil)", got, err)
	}
}

// A scalar rebuilt from an introspection answer has no coercers, and this hook
// is how they are put back.
func TestMapSchema_ScalarHook(t *testing.T) {
	s := mustBuild(t, `scalar Odd type Query { a: Odd }`)
	mapped := utilities.MapSchema(s, utilities.SchemaMapper{
		Scalar: func(c schema.ScalarConfig) schema.ScalarConfig {
			if c.Name == "Odd" {
				c.SpecifiedByURL = "https://example.com/odd"
			}
			return c
		},
	})
	odd, _ := mapped.Type("Odd").(*schema.ScalarType)
	if odd == nil {
		t.Fatal("Odd was lost")
	}
	if odd.SpecifiedByURL != "https://example.com/odd" {
		t.Errorf("SpecifiedByURL = %q, want the one the hook set", odd.SpecifiedByURL)
	}
}

// keepIf returns the entries a test wants to keep.
func keepIf[T any](items []T, keep func(T) bool) []T {
	out := make([]T, 0, len(items))
	for _, item := range items {
		if keep(item) {
			out = append(out, item)
		}
	}
	return out
}

// Rebuilding a schema must carry over everything a type holds, including what
// printing does not show. The parts are taken from the type itself rather than
// listed out by hand, so that one nobody thought of cannot be dropped — which
// is how the extension AST nodes went missing once.
func TestMapSchema_CarriesOverWhatPrintingDoesNotShow(t *testing.T) {
	s := mustBuild(t, `
		scalar Weekday
		enum Colour { RED }
		input Filter { term: String }
		interface Node { id: ID! }
		type User implements Node { id: ID! }
		union Media = User
		type Query { me: User colour: Colour when: Weekday find(f: Filter): String node: Node media: Media }
	`)
	mark := map[string]any{"marked": true}
	s.Type("Weekday").(*schema.ScalarType).Extensions = mark
	s.Type("Colour").(*schema.EnumType).Extensions = mark
	s.Type("Filter").(*schema.InputObjectType).Extensions = mark
	s.Type("Node").(*schema.InterfaceType).Extensions = mark
	s.Type("Media").(*schema.UnionType).Extensions = mark
	user := s.Type("User").(*schema.ObjectType)
	user.Extensions = mark
	user.IsTypeOf = func(context.Context, any, *schema.ResolveInfo) (bool, error) { return true, nil }
	s.Type("Weekday").(*schema.ScalarType).SpecifiedByURL = "https://example.com"

	mapped := utilities.MapSchema(s, utilities.SchemaMapper{})

	for _, name := range []string{"Weekday", "Colour", "Filter", "Node", "Media", "User"} {
		var extensions map[string]any
		switch typ := mapped.Type(name).(type) {
		case *schema.ScalarType:
			extensions = typ.Extensions
			if typ.SpecifiedByURL != "https://example.com" {
				t.Errorf("%s lost where it is specified", name)
			}
		case *schema.EnumType:
			extensions = typ.Extensions
		case *schema.InputObjectType:
			extensions = typ.Extensions
		case *schema.InterfaceType:
			extensions = typ.Extensions
		case *schema.UnionType:
			extensions = typ.Extensions
		case *schema.ObjectType:
			extensions = typ.Extensions
			if typ.IsTypeOf == nil {
				t.Errorf("%s lost the way it says which values are one of it", name)
			}
		}
		if extensions == nil || extensions["marked"] != true {
			t.Errorf("%s lost its extensions", name)
		}
	}
}
