package utilities_test

import (
	"context"
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

// Ported from the "throws when …" cases of
// src/utilities/__tests__/buildClientSchema-test.ts: an answer that does not
// hold together is refused rather than half-built, as graphql-js refuses it.
//
// A list left out of an answer is not a smaller answer, it is a broken one:
// every introspection query this library can produce asks for all of them, so
// a missing one means the answer cannot be rebuilt faithfully, and a schema
// quietly missing its fields is worse than being told why.
//
// The one exception is graphql-js's own: an interface with nothing where its
// interfaces should be is taken as having none, because a server built before
// interfaces could implement interfaces answers that way.
func TestPortedBuildClientSchema_MalformedAnswer(t *testing.T) {
	build := func() *utilities.IntrospectionQueryResult {
		s, err := utilities.BuildSchema(`
			type Query { foo(arg: In): Out }
			type Out implements Iface { a: String }
			interface Iface { a: String }
			union U = Out
			enum E { A }
			input In { a: String }
		`)
		if err != nil {
			t.Fatal(err)
		}
		answer, err := utilities.IntrospectionFromSchema(context.Background(), s)
		if err != nil {
			t.Fatal(err)
		}
		return answer
	}
	find := func(r *utilities.IntrospectionQueryResult, name string) *utilities.IntrospectionType {
		for _, ty := range r.Schema.Types {
			if ty.Name == name {
				return ty
			}
		}
		t.Fatalf("no type %s", name)
		return nil
	}

	// refused says the answer is inconsistent rather than merely incomplete,
	// and building from it fails.
	refused := func(name string, mutate func(*utilities.IntrospectionQueryResult)) {
		t.Run(name, func(t *testing.T) {
			r := build()
			mutate(r)
			built, err := utilities.BuildClientSchema(r)
			if err == nil {
				t.Fatalf("built a schema from it:\n%s", utilities.PrintSchema(built))
			}
			if built != nil {
				t.Error("a schema came back as well as the error")
			}
		})
	}

	// tolerated says the answer is one graphql-js still takes. The schema is
	// built and is sound.
	tolerated := func(name string, mutate func(*utilities.IntrospectionQueryResult)) {
		t.Run(name, func(t *testing.T) {
			r := build()
			mutate(r)
			built, err := utilities.BuildClientSchema(r)
			if err != nil {
				t.Fatalf("refused it: %v", err)
			}
			if errs := schema.ValidateSchema(built); len(errs) > 0 {
				t.Errorf("the schema is not sound: %v", errs)
			}
		})
	}

	refused("no schema at all", func(r *utilities.IntrospectionQueryResult) { r.Schema = utilities.IntrospectionSchema{} })
	refused("a type with no kind", func(r *utilities.IntrospectionQueryResult) { find(r, "Query").Kind = "" })
	refused("an object with no interfaces list", func(r *utilities.IntrospectionQueryResult) { find(r, "Out").Interfaces = nil })
	refused("an object with no fields list", func(r *utilities.IntrospectionQueryResult) { find(r, "Query").Fields = nil })
	refused("an interface with no fields list", func(r *utilities.IntrospectionQueryResult) { find(r, "Iface").Fields = nil })
	refused("a field with no arguments list", func(r *utilities.IntrospectionQueryResult) { find(r, "Query").Fields[0].Args = nil })
	refused("a union with no possibleTypes list", func(r *utilities.IntrospectionQueryResult) { find(r, "U").PossibleTypes = nil })
	refused("an enum with no values list", func(r *utilities.IntrospectionQueryResult) { find(r, "E").EnumValues = nil })
	refused("an input object with no fields list", func(r *utilities.IntrospectionQueryResult) { find(r, "In").InputFields = nil })
	refused("a named type reference with no name", func(r *utilities.IntrospectionQueryResult) {
		find(r, "Query").Fields[0].Type.Name = ""
	})
	refused("a reference to a type the answer does not describe", func(r *utilities.IntrospectionQueryResult) {
		find(r, "Query").Fields[0].Type.Name = "Nope"
	})
	refused("a directive with no locations list", func(r *utilities.IntrospectionQueryResult) {
		r.Schema.Directives[0].Locations = nil
	})
	refused("a directive with no arguments list", func(r *utilities.IntrospectionQueryResult) {
		r.Schema.Directives[0].Args = nil
	})
	refused("an output type used where an input type belongs", func(r *utilities.IntrospectionQueryResult) {
		find(r, "Query").Fields[0].Args[0].Type = &utilities.IntrospectionTypeRef{Kind: "OBJECT", Name: "Out"}
	})
	refused("an input type used where an output type belongs", func(r *utilities.IntrospectionQueryResult) {
		find(r, "Query").Fields[0].Type = &utilities.IntrospectionTypeRef{Kind: "INPUT_OBJECT", Name: "In"}
	})
	refused("a standard scalar the answer does not describe", func(r *utilities.IntrospectionQueryResult) {
		kept := r.Schema.Types[:0]
		for _, ty := range r.Schema.Types {
			if ty.Name != "String" {
				kept = append(kept, ty)
			}
		}
		r.Schema.Types = kept
	})

	// graphql-js's "Legacy support for interfaces with null as interfaces
	// field": an interface, unlike an object, may have nothing there.
	tolerated("an interface with no interfaces list", func(r *utilities.IntrospectionQueryResult) {
		find(r, "Iface").Interfaces = nil
	})
}
