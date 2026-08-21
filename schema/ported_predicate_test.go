package schema_test

// Ported from graphql-js src/type/__tests__/predicate-test.ts: which of the
// questions about a type each kind of type answers yes to.
//
// A good part of that file asks what a predicate says about a class rather
// than an instance, or about a value that is not a type at all. Neither
// question can be asked here: a predicate takes a schema.Type, so anything
// else does not compile.

import (
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

func TestPortedPredicates(t *testing.T) {
	object := schema.NewObject(schema.ObjectConfig{Name: "Object"})
	iface := schema.NewInterface(schema.InterfaceConfig{Name: "Interface"})
	union := schema.NewUnion(schema.UnionConfig{Name: "Union"})
	enum := schema.NewEnum(schema.EnumConfig{Name: "Enum"})
	input := schema.NewInputObject(schema.InputObjectConfig{Name: "InputObject"})
	scalar := schema.NewScalar(schema.ScalarConfig{Name: "Scalar"})

	// yes lists the predicates a type answers to; everything else must say no.
	all := []struct {
		name string
		ask  func(schema.Type) bool
	}{
		{"type", func(t schema.Type) bool { return t != nil }},
		{"scalar", schema.IsScalarType},
		{"object", schema.IsObjectType},
		{"interface", schema.IsInterfaceType},
		{"union", schema.IsUnionType},
		{"enum", schema.IsEnumType},
		{"input object", schema.IsInputObjectType},
		{"list", schema.IsListType},
		{"non-null", schema.IsNonNullType},
		{"input", schema.IsInputType},
		{"output", schema.IsOutputType},
		{"leaf", schema.IsLeafType},
		{"composite", schema.IsCompositeType},
		{"abstract", schema.IsAbstractType},
		{"wrapping", schema.IsWrappingType},
		{"nullable", schema.IsNullableType},
		{"named", schema.IsNamedType},
	}

	for _, tt := range []struct {
		name string
		typ  schema.Type
		yes  []string
	}{
		{"a scalar", scalar,
			[]string{"type", "scalar", "input", "output", "leaf", "nullable", "named"}},
		{"an object", object,
			[]string{"type", "object", "output", "composite", "nullable", "named"}},
		{"an interface", iface,
			[]string{"type", "interface", "output", "composite", "abstract", "nullable", "named"}},
		{"a union", union,
			[]string{"type", "union", "output", "composite", "abstract", "nullable", "named"}},
		{"an enum", enum,
			[]string{"type", "enum", "input", "output", "leaf", "nullable", "named"}},
		{"an input object", input,
			[]string{"type", "input object", "input", "nullable", "named"}},
		{"a list", schema.NewList(object),
			[]string{"type", "list", "output", "wrapping", "nullable"}},
		{"a list of an input type", schema.NewList(input),
			[]string{"type", "list", "input", "wrapping", "nullable"}},
		{"a non-null", schema.NewNonNull(object),
			[]string{"type", "non-null", "output", "wrapping"}},
		{"a non-null of an input type", schema.NewNonNull(input),
			[]string{"type", "non-null", "input", "wrapping"}},
		{"a list of a scalar", schema.NewList(scalar),
			[]string{"type", "list", "input", "output", "wrapping", "nullable"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			expected := map[string]bool{}
			for _, name := range tt.yes {
				expected[name] = true
			}
			for _, ask := range all {
				if got := ask.ask(tt.typ); got != expected[ask.name] {
					t.Errorf("is %s? %v, want %v", ask.name, got, expected[ask.name])
				}
			}
		})
	}
}

// Unwrapping a type answers what it is made of.
func TestPortedPredicates_Unwrapping(t *testing.T) {
	object := schema.NewObject(schema.ObjectConfig{Name: "Object"})

	for _, tt := range []struct {
		name            string
		typ             schema.Type
		named, nullable string
	}{
		{"a named type", object, "Object", "Object"},
		{"a list", schema.NewList(object), "Object", "[Object]"},
		{"a non-null", schema.NewNonNull(object), "Object", "Object"},
		{"a non-null list", schema.NewNonNull(schema.NewList(object)), "Object", "[Object]"},
		{"a list of non-nulls", schema.NewList(schema.NewNonNull(object)), "Object", "[Object!]"},
		{"a non-null list of non-nulls",
			schema.NewNonNull(schema.NewList(schema.NewNonNull(object))), "Object", "[Object!]"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := schema.NamedTypeOf(tt.typ); got == nil || got.Name() != tt.named {
				t.Errorf("NamedTypeOf = %v, want %s", got, tt.named)
			}
			if got := schema.NullableTypeOf(tt.typ); got == nil || got.String() != tt.nullable {
				t.Errorf("NullableTypeOf = %v, want %s", got, tt.nullable)
			}
		})
	}
}

// The scalars and types every schema has are recognised as such.
func TestPortedPredicates_Specified(t *testing.T) {
	for _, s := range []*schema.ScalarType{
		schema.String, schema.Int, schema.Float, schema.Boolean, schema.ID,
	} {
		if !schema.IsSpecifiedScalarType(s) {
			t.Errorf("%s is not recognised as a built-in scalar", s.Name())
		}
		if schema.IsIntrospectionType(s) {
			t.Errorf("%s is recognised as an introspection type", s.Name())
		}
	}
	custom := schema.NewScalar(schema.ScalarConfig{Name: "Scalar"})
	if schema.IsSpecifiedScalarType(custom) {
		t.Error("a custom scalar is recognised as a built-in one")
	}
	// A custom type named after a built-in is still a custom type: what makes
	// a scalar built-in is being the one this library declares, not its name.
	sameName := schema.NewScalar(schema.ScalarConfig{Name: "String"})
	if schema.IsSpecifiedScalarType(sameName) {
		t.Error("a custom scalar named String is recognised as the built-in one")
	}
}

// Ported from predicate-test.ts's isRequiredArgument, isRequiredInputField and
// isSpecifiedDirective.
//
// The other predicates that file covers — isField, isArgument, isEnumValue,
// isInputField, isDirective, isSchema — are a type assertion here, so there is
// nothing to ask them.
func TestPortedPredicates_Required(t *testing.T) {
	t.Run("an argument", func(t *testing.T) {
		for _, tt := range []struct {
			name     string
			arg      *schema.Argument
			required bool
		}{
			{
				name:     "non-null and no default",
				arg:      schema.NewArgument("a", schema.ArgumentConfig{Type: schema.NewNonNull(schema.String)}),
				required: true,
			},
			{
				name: "non-null with a default",
				arg: schema.NewArgument("a", schema.ArgumentConfig{
					Type: schema.NewNonNull(schema.String), Default: schema.DefaultValue("x"),
				}),
			},
			{
				name: "nullable and no default",
				arg:  schema.NewArgument("a", schema.ArgumentConfig{Type: schema.String}),
			},
			{
				name: "nullable with a default",
				arg: schema.NewArgument("a", schema.ArgumentConfig{
					Type: schema.String, Default: schema.DefaultValue("x"),
				}),
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				if got := schema.IsRequiredArgument(tt.arg); got != tt.required {
					t.Errorf("IsRequiredArgument() = %v, want %v", got, tt.required)
				}
			})
		}
	})

	t.Run("an input field", func(t *testing.T) {
		for _, tt := range []struct {
			name     string
			field    *schema.InputField
			required bool
		}{
			{
				name:     "non-null and no default",
				field:    schema.NewInputField("a", schema.InputFieldConfig{Type: schema.NewNonNull(schema.String)}),
				required: true,
			},
			{
				name: "non-null with a default",
				field: schema.NewInputField("a", schema.InputFieldConfig{
					Type: schema.NewNonNull(schema.String), Default: schema.DefaultValue("x"),
				}),
			},
			{
				name:  "nullable and no default",
				field: schema.NewInputField("a", schema.InputFieldConfig{Type: schema.String}),
			},
			{
				name: "nullable with a default",
				field: schema.NewInputField("a", schema.InputFieldConfig{
					Type: schema.String, Default: schema.DefaultValue("x"),
				}),
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				if got := schema.IsRequiredInputField(tt.field); got != tt.required {
					t.Errorf("IsRequiredInputField() = %v, want %v", got, tt.required)
				}
			})
		}
	})

	t.Run("a directive", func(t *testing.T) {
		if !schema.IsSpecifiedDirective(schema.Skip) {
			t.Error("@skip is not recognised as one of the specified directives")
		}
		// @defer and @stream are experimental and deliberately not among them.
		if schema.IsSpecifiedDirective(schema.Defer) {
			t.Error("@defer is recognised as a specified directive")
		}
		own := schema.NewDirective(schema.DirectiveConfig{
			Name:      "mine",
			Locations: []language.DirectiveLocation{language.DirectiveLocationField},
		})
		if schema.IsSpecifiedDirective(own) {
			t.Error("a directive of one's own is recognised as a specified one")
		}
	})
}
