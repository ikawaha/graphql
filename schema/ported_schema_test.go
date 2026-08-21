package schema_test

// Ported from graphql-js src/type/__tests__/schema-test.ts: which types a
// schema gathers, and in what order. The rest of that file — root types,
// getField, meta-fields, duplicate names — is covered by schema_test.go.

import (
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// A type reachable only as an argument of a directive is still part of the
// schema, and so is a type nested inside it.
func TestPortedSchema_GathersTypesFromDirectives(t *testing.T) {
	foo := schema.NewInputObject(schema.InputObjectConfig{Name: "Foo"})
	bar := schema.NewInputObject(schema.InputObjectConfig{Name: "Bar"})
	s := schema.New(schema.Config{
		Directives: []*schema.Directive{
			schema.NewDirective(schema.DirectiveConfig{
				Name:      "dir",
				Locations: []language.DirectiveLocation{language.DirectiveLocationObject},
				Args: []*schema.Argument{
					schema.NewArgument("arg", schema.ArgumentConfig{Type: foo}),
					schema.NewArgument("argList", schema.ArgumentConfig{
						Type: schema.NewList(bar),
					}),
				},
			}),
		},
	})
	for _, name := range []string{"Foo", "Bar"} {
		if s.Type(name) == nil {
			t.Errorf("%s was not gathered", name)
		}
	}
}

// An input object reached through a field argument brings what it holds with
// it.
func TestPortedSchema_GathersNestedInputObjects(t *testing.T) {
	nested := schema.NewInputObject(schema.InputObjectConfig{Name: "NestedInputObject"})
	some := schema.NewInputObject(schema.InputObjectConfig{
		Name: "SomeInputObject",
		Fields: []*schema.InputField{
			schema.NewInputField("nested", schema.InputFieldConfig{Type: nested}),
		},
	})
	s := schema.New(schema.Config{
		Query: schema.NewObject(schema.ObjectConfig{
			Name: "Query",
			Fields: []*schema.Field{
				schema.NewField("something", schema.FieldConfig{
					Type: schema.String,
					Args: []*schema.Argument{
						schema.NewArgument("input", schema.ArgumentConfig{Type: some}),
					},
				}),
			},
		}),
	})
	if s.Type("SomeInputObject") != schema.Type(some) {
		t.Error("the input object was not gathered")
	}
	if s.Type("NestedInputObject") != schema.Type(nested) {
		t.Error("the nested input object was not gathered")
	}
}

// A type listed but not reachable is still in the schema, and an interface it
// implements makes it a subtype.
func TestPortedSchema_GathersListedSubtypes(t *testing.T) {
	iface := schema.NewInterface(schema.InterfaceConfig{Name: "SomeInterface"})
	subtype := schema.NewObject(schema.ObjectConfig{
		Name:       "SomeSubtype",
		Interfaces: schema.Implements(iface),
	})
	s := schema.New(schema.Config{
		Query: schema.NewObject(schema.ObjectConfig{
			Name: "Query",
			Fields: []*schema.Field{
				schema.NewField("iface", schema.FieldConfig{Type: iface}),
			},
		}),
		Types: []schema.NamedType{subtype},
	})
	if s.Type("SomeInterface") != schema.NamedType(iface) {
		t.Error("the interface was not gathered")
	}
	if s.Type("SomeSubtype") != schema.NamedType(subtype) {
		t.Error("the listed subtype was not gathered")
	}
	if !s.IsSubType(iface, subtype) {
		t.Error("the listed subtype is not recognised as one")
	}
}

// The types a caller listed come first, in the order given, each followed by
// what it reaches; then whatever is left, then the introspection types.
func TestPortedSchema_KeepsTheOrderTypesWereGivenIn(t *testing.T) {
	aType := schema.NewObject(schema.ObjectConfig{
		Name: "A",
		Fields: []*schema.Field{
			schema.NewField("sub", schema.FieldConfig{
				Type: schema.NewScalar(schema.ScalarConfig{Name: "ASub"}),
			}),
		},
	})
	zType := schema.NewObject(schema.ObjectConfig{
		Name: "Z",
		Fields: []*schema.Field{
			schema.NewField("sub", schema.FieldConfig{
				Type: schema.NewScalar(schema.ScalarConfig{Name: "ZSub"}),
			}),
		},
	})
	query := schema.NewObject(schema.ObjectConfig{
		Name: "Query",
		Fields: []*schema.Field{
			schema.NewField("a", schema.FieldConfig{Type: aType}),
			schema.NewField("z", schema.FieldConfig{Type: zType}),
			schema.NewField("sub", schema.FieldConfig{
				Type: schema.NewScalar(schema.ScalarConfig{Name: "QuerySub"}),
			}),
		},
	})
	// Boolean and String appear because the built-in directives take them,
	// and the introspection types come last because every schema can describe
	// itself whether or not anything refers to them.
	want := []string{
		"Z", "ZSub", "Query", "QuerySub", "A", "ASub", "Boolean", "String",
		"__Schema", "__Type", "__TypeKind", "__Field", "__InputValue",
		"__EnumValue", "__Directive", "__DirectiveLocation",
	}

	s := schema.New(schema.Config{
		Types: []schema.NamedType{zType, query, aType},
		Query: query,
	})
	expectTypeOrder(t, s, want)

	// The order is settled, not a product of how the schema was assembled: a
	// schema built again from the same types comes out the same way.
	again := schema.New(schema.Config{
		Types: s.Types(),
		Query: query,
	})
	expectTypeOrder(t, again, want)
}

func expectTypeOrder(t *testing.T, s *schema.Schema, want []string) {
	t.Helper()
	var got []string
	for _, named := range s.Types() {
		got = append(got, named.Name())
	}
	for i := range max(len(got), len(want)) {
		switch {
		case i >= len(got):
			t.Errorf("type %d: missing, want %q", i, want[i])
		case i >= len(want):
			t.Errorf("type %d: %q, want no further types", i, got[i])
		case got[i] != want[i]:
			t.Errorf("type %d: %q, want %q", i, got[i], want[i])
		}
	}
}
