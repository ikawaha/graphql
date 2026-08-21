package schema_test

// Ported from graphql-js src/utilities/__tests__/typeComparators-test.ts: when
// one type may stand where another is wanted.

import (
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

func TestPortedTypeComparators(t *testing.T) {
	// The schema the sub-type cases are asked against, holding a union and two
	// interfaces so that every way of being a sub-type is covered.
	s := buildForComparators(t, `
		type Query { field: String }
		interface Iface { field: String }
		interface Iface2 implements Iface { field: String }
		type Impl implements Iface2 & Iface { field: String }
		union Union = Impl
	`)
	named := func(name string) schema.Type { return s.Type(name) }

	t.Run("the same type is the same", func(t *testing.T) {
		expectSame(t, schema.String, schema.String, true)
	})
	t.Run("Int and Float are not the same", func(t *testing.T) {
		expectSame(t, schema.Int, schema.Float, false)
	})
	t.Run("lists of the same type are the same", func(t *testing.T) {
		expectSame(t, schema.NewList(schema.Int), schema.NewList(schema.Int), true)
	})
	t.Run("a list is not the same as what it holds", func(t *testing.T) {
		expectSame(t, schema.NewList(schema.Int), schema.Int, false)
	})
	t.Run("non-nulls of the same type are the same", func(t *testing.T) {
		expectSame(t, schema.NewNonNull(schema.Int), schema.NewNonNull(schema.Int), true)
	})
	t.Run("a non-null is not the same as the nullable", func(t *testing.T) {
		expectSame(t, schema.NewNonNull(schema.Int), schema.Int, false)
	})

	for _, tt := range []struct {
		name       string
		sub, super schema.Type
		want       bool
	}{
		{"the same type stands for itself", schema.String, schema.String, true},
		{"Int does not stand for Float", schema.Int, schema.Float, false},
		{"a non-null stands where the nullable is wanted",
			schema.NewNonNull(schema.Int), schema.Int, true},
		{"a nullable does not stand where a non-null is wanted",
			schema.Int, schema.NewNonNull(schema.Int), false},
		{"an item does not stand where a list is wanted",
			schema.Int, schema.NewList(schema.Int), false},
		{"a list does not stand where an item is wanted",
			schema.NewList(schema.Int), schema.Int, false},
		{"a member stands where its union is wanted", named("Impl"), named("Union"), true},
		{"an object stands where an interface it implements is wanted",
			named("Impl"), named("Iface"), true},
		{"an interface stands where an interface it implements is wanted",
			named("Iface2"), named("Iface"), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := schema.IsTypeSubTypeOf(s, tt.sub, tt.super); got != tt.want {
				t.Errorf("IsTypeSubTypeOf(%s, %s) = %v, want %v", tt.sub, tt.super, got, tt.want)
			}
		})
	}
}

func expectSame(t *testing.T, a, b schema.Type, want bool) {
	t.Helper()
	if got := schema.IsEqualType(a, b); got != want {
		t.Errorf("IsEqualType(%s, %s) = %v, want %v", a, b, got, want)
	}
}

func buildForComparators(t *testing.T, sdl string) *schema.Schema {
	t.Helper()
	s, err := utilities.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	return s
}
