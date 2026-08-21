package utilities_test

import (
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

func coordinateSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := utilities.BuildSchema(`
		scalar DateTime
		enum Colour { RED GREEN }
		interface Node { id: ID! }
		input Filter { term: String }
		type User implements Node {
			id: ID!
			name: String
			friends(first: Int, filter: Filter): [User!]
		}
		union Media = User
		directive @auth(role: String!) on FIELD_DEFINITION
		type Query { me: User }
	`)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	return s
}

// A coordinate is how tooling refers to one element of a schema, so every form
// the grammar allows has to reach the thing it names.
func TestResolveSchemaCoordinate(t *testing.T) {
	s := coordinateSchema(t)

	tests := []struct {
		coordinate string
		// what says which member the answer should have set, so that a
		// coordinate reaching the right name on the wrong kind of thing is
		// still a failure.
		what func(*utilities.ResolvedCoordinate) bool
	}{
		{"User", func(r *utilities.ResolvedCoordinate) bool {
			return r.Type != nil && r.Type.Name() == "User" && r.Field == nil
		}},
		{"DateTime", func(r *utilities.ResolvedCoordinate) bool { return r.Type != nil }},
		{"User.name", func(r *utilities.ResolvedCoordinate) bool {
			return r.Field != nil && r.Field.Name() == "name"
		}},
		{"Node.id", func(r *utilities.ResolvedCoordinate) bool {
			return r.Field != nil && r.Field.Name() == "id"
		}},
		{"User.friends(first:)", func(r *utilities.ResolvedCoordinate) bool {
			return r.Argument != nil && r.Argument.Name() == "first" && r.Field != nil
		}},
		{"Colour.RED", func(r *utilities.ResolvedCoordinate) bool {
			return r.EnumValue != nil && r.EnumValue.Name() == "RED"
		}},
		{"Filter.term", func(r *utilities.ResolvedCoordinate) bool {
			return r.InputField != nil && r.InputField.Name() == "term"
		}},
		{"@auth", func(r *utilities.ResolvedCoordinate) bool {
			return r.Directive != nil && r.Directive.Name() == "auth" && r.Argument == nil
		}},
		{"@auth(role:)", func(r *utilities.ResolvedCoordinate) bool {
			return r.Directive != nil && r.Argument != nil && r.Argument.Name() == "role"
		}},
		{"@deprecated(reason:)", func(r *utilities.ResolvedCoordinate) bool {
			return r.Directive != nil && r.Argument != nil
		}},
	}
	for _, tt := range tests {
		t.Run(tt.coordinate, func(t *testing.T) {
			got, err := utilities.ResolveSchemaCoordinate(s, tt.coordinate)
			if err != nil {
				t.Fatalf("resolving: %v", err)
			}
			if got == nil {
				t.Fatal("resolved to nothing")
			}
			if !tt.what(got) {
				t.Errorf("resolved to the wrong thing: %s", got)
			}
			// What was resolved describes itself as what was asked for.
			if got.String() != tt.coordinate {
				t.Errorf("String() = %q, want %q", got.String(), tt.coordinate)
			}
		})
	}
}

// A coordinate written about a schema that has since changed names nothing,
// which is an answer rather than a fault.
func TestResolveSchemaCoordinate_NamesNothing(t *testing.T) {
	s := coordinateSchema(t)
	for _, coordinate := range []string{
		"Missing",
		"User.missing",
		"User.name(missing:)",
		"Colour.PUCE",
		"Filter.missing",
		"@missing",
		"@auth(missing:)",
	} {
		t.Run(coordinate, func(t *testing.T) {
			got, err := utilities.ResolveSchemaCoordinate(s, coordinate)
			if err != nil {
				t.Fatalf("resolving: %v", err)
			}
			if got != nil {
				t.Errorf("resolved to %s, want nothing", got)
			}
		})
	}
}

// A coordinate that is not a question about this schema is reported rather
// than answered with nothing: naming a type the schema does not have, or
// putting a member after a type that has none, is a mistake in the coordinate
// rather than a gap in the schema.
func TestResolveSchemaCoordinate_NotAboutThisSchema(t *testing.T) {
	s := coordinateSchema(t)
	for _, coordinate := range []string{
		"Missing.field",
		"User.missing(first:)",
		"@missing(arg:)",
		// A scalar and a union have no members.
		"DateTime.anything",
		"Media.anything",
	} {
		if _, err := utilities.ResolveSchemaCoordinate(s, coordinate); err == nil {
			t.Errorf("%q was answered rather than refused", coordinate)
		}
	}
}

// Text that is not a coordinate at all is reported, rather than quietly
// naming nothing: the two are different problems for a caller.
func TestResolveSchemaCoordinate_NotACoordinate(t *testing.T) {
	s := coordinateSchema(t)
	for _, coordinate := range []string{"", "User.", ".name", "User..name", "@", "User name", "User(x:)"} {
		if _, err := utilities.ResolveSchemaCoordinate(s, coordinate); err == nil {
			t.Errorf("%q was accepted as a coordinate", coordinate)
		}
	}
}

// A nil schema knows nothing, and asking it names nothing rather than
// crashing.
func TestResolveSchemaCoordinate_NilSchema(t *testing.T) {
	got, err := utilities.ResolveSchemaCoordinate(nil, "User.name")
	if err != nil {
		t.Fatalf("resolving: %v", err)
	}
	if got != nil {
		t.Errorf("a nil schema resolved %s", got)
	}
	if (*utilities.ResolvedCoordinate)(nil).String() != "" {
		t.Error("nothing does not describe itself as nothing")
	}
}
