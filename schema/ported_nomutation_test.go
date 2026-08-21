package schema_test

// Ported from graphql-js src/type/__tests__/definition-test.ts's
// "does not mutate passed field definitions".
//
// The same configuration is used to build two types, and neither the
// configuration nor anything in it may come back changed. graphql-js builds a
// field of its own for each type; so does this, for the same reason — a field
// records which type it belongs to, and would otherwise belong to whichever
// type was built second.

import (
	"testing"

	"github.com/ikawaha/graphql/schema"
)

func TestPortedDefinition_DoesNotMutateWhatItWasGiven(t *testing.T) {
	t.Run("object fields", func(t *testing.T) {
		fields := []*schema.Field{
			schema.NewField("field1", schema.FieldConfig{Type: schema.String}),
			schema.NewField("field2", schema.FieldConfig{
				Type: schema.String,
				Args: []*schema.Argument{
					schema.NewArgument("id", schema.ArgumentConfig{Type: schema.String}),
				},
			}),
		}
		first := schema.NewObject(schema.ObjectConfig{Name: "Test1", Fields: fields})
		second := schema.NewObject(schema.ObjectConfig{Name: "Test2", Fields: fields})

		for _, name := range []string{"field1", "field2"} {
			a, b := first.Field(name), second.Field(name)
			if a == b {
				t.Errorf("%s is the same field in both types", name)
			}
			if a.Parent() != schema.NamedType(first) || b.Parent() != schema.NamedType(second) {
				t.Errorf("%s belongs to %v and %v", name, a.Parent(), b.Parent())
			}
			if a.Type != b.Type || len(a.Args) != len(b.Args) {
				t.Errorf("the two copies of %s differ", name)
			}
		}
		for _, f := range fields {
			if f.Parent() != nil {
				t.Errorf("the field %s handed in was attached to %v", f.Name(), f.Parent())
			}
		}
	})

	t.Run("input object fields", func(t *testing.T) {
		fields := []*schema.InputField{
			schema.NewInputField("field1", schema.InputFieldConfig{Type: schema.String}),
			schema.NewInputField("field2", schema.InputFieldConfig{Type: schema.String}),
		}
		first := schema.NewInputObject(schema.InputObjectConfig{Name: "Test1", Fields: fields})
		second := schema.NewInputObject(schema.InputObjectConfig{Name: "Test2", Fields: fields})
		if len(first.Fields()) != 2 || len(second.Fields()) != 2 {
			t.Fatalf("%d and %d fields, want two each", len(first.Fields()), len(second.Fields()))
		}
	})

	t.Run("enum members", func(t *testing.T) {
		values := []*schema.EnumValue{
			schema.NewEnumValue("A", schema.EnumValueConfig{Value: schema.InternalValue(1)}),
			schema.NewEnumValue("B", schema.EnumValueConfig{Value: schema.InternalValue(2)}),
		}
		first := schema.NewEnum(schema.EnumConfig{Name: "First", Values: values})
		second := schema.NewEnum(schema.EnumConfig{Name: "Second", Values: values})

		if first.Value("A").Parent() != first || second.Value("A").Parent() != second {
			t.Error("a member belongs to the wrong enum")
		}
		if got := second.Value("A").String(); got != "Second.A" {
			t.Errorf("String() = %q, want Second.A", got)
		}
		for _, v := range values {
			if v.Parent() != nil {
				t.Errorf("the member %s handed in was attached to %v", v.Name(), v.Parent())
			}
		}
	})

	// The list handed in belongs to the caller. Adding to it afterwards must
	// not change what the type holds.
	t.Run("the list handed in", func(t *testing.T) {
		fields := make([]*schema.Field, 1, 4)
		fields[0] = schema.NewField("kept", schema.FieldConfig{Type: schema.String})
		object := schema.NewObject(schema.ObjectConfig{Name: "O", Fields: fields})
		_ = object.Fields()
		fields = append(fields, schema.NewField("added", schema.FieldConfig{Type: schema.String}))
		fields[0] = schema.NewField("swapped", schema.FieldConfig{Type: schema.String})

		if len(object.Fields()) != 1 || object.Field("kept") == nil {
			t.Errorf("the type followed the caller's list: %v", object.Fields())
		}

		values := make([]*schema.EnumValue, 1, 4)
		values[0] = schema.NewEnumValue("KEPT", schema.EnumValueConfig{})
		enum := schema.NewEnum(schema.EnumConfig{Name: "E", Values: values})
		values[0] = schema.NewEnumValue("SWAPPED", schema.EnumValueConfig{})
		if enum.Value("KEPT") == nil || enum.Value("SWAPPED") != nil {
			t.Error("the enum followed the caller's list")
		}

		members := make([]schema.Declared[*schema.ObjectType], 1, 4)
		members[0] = schema.Declare(object)
		union := schema.NewUnion(schema.UnionConfig{Name: "U", Types: members})
		_ = union.Types()
		members[0] = schema.Declare(schema.NewObject(schema.ObjectConfig{Name: "Other"}))
		if got := union.Types(); len(got) != 1 || got[0].Named() != schema.NamedType(object) {
			t.Error("the union followed the caller's list")
		}
	})
}
