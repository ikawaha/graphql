package schema

import (
	"github.com/ikawaha/graphql/value"
	"slices"
	"testing"
)

func inputFieldNames(fields []*InputField) []string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.Name()
	}
	return names
}

func TestInputObject_FieldsKeepDeclarationOrder(t *testing.T) {
	str := testScalar("String")
	in := NewInputObject(InputObjectConfig{
		Name: "Filter",
		Fields: []*InputField{
			NewInputField("zebra", InputFieldConfig{Type: str}),
			NewInputField("alpha", InputFieldConfig{Type: str}),
		},
	})

	if got := inputFieldNames(in.Fields()); !slices.Equal(got, []string{"zebra", "alpha"}) {
		t.Errorf("Fields() = %v, want [zebra alpha]", got)
	}
	if in.Field("alpha") == nil {
		t.Error("Field(alpha) = nil")
	}
	if in.Field("missing") != nil {
		t.Error("Field(missing) found something")
	}
}

// An input object may hold a field of its own type, which needs the thunk for
// the same reason an object's fields do.
func TestInputObject_SelfReference(t *testing.T) {
	var filter *InputObjectType
	filter = NewInputObject(InputObjectConfig{
		Name: "Filter",
		FieldsThunk: func() []*InputField {
			return []*InputField{
				NewInputField("term", InputFieldConfig{Type: testScalar("String")}),
				NewInputField("or", InputFieldConfig{Type: NewList(filter)}),
			}
		},
	})

	or := filter.Field("or")
	if or == nil {
		t.Fatal("the recursive field is missing")
	}
	if got := or.Type.(*List).OfType; got != Type(filter) {
		t.Errorf("or points at %v, want the type itself", got)
	}
}

// The three-state default carries through to whether a caller has to supply
// the field, which is the whole point of holding it that way.
func TestInputObject_RequiredFields(t *testing.T) {
	intType := testScalar("Int")
	in := NewInputObject(InputObjectConfig{
		Name: "Paging",
		Fields: []*InputField{
			NewInputField("required", InputFieldConfig{Type: NewNonNull(intType)}),
			NewInputField("defaulted", InputFieldConfig{
				Type:    NewNonNull(intType),
				Default: DefaultValue(10),
			}),
			NewInputField("nullDefault", InputFieldConfig{
				Type:    NewNonNull(intType),
				Default: DefaultValue(nil),
			}),
			NewInputField("optional", InputFieldConfig{Type: intType}),
		},
	})

	want := map[string]bool{
		"required":    true,
		"defaulted":   false,
		"nullDefault": false,
		"optional":    false,
	}
	for name, required := range want {
		if got := IsRequiredInputField(in.Field(name)); got != required {
			t.Errorf("IsRequiredInputField(%s) = %v, want %v", name, got, required)
		}
	}
}

func TestInputObject_IsOneOf(t *testing.T) {
	plain := NewInputObject(InputObjectConfig{Name: "Plain"})
	if plain.IsOneOf {
		t.Error("IsOneOf = true for an ordinary input object")
	}

	tagged := NewInputObject(InputObjectConfig{Name: "Tagged", IsOneOf: true})
	if !tagged.IsOneOf {
		t.Error("IsOneOf = false, want true")
	}
}

func TestInputObject_ThunkWinsAndNilsAreSkipped(t *testing.T) {
	str := testScalar("String")
	in := NewInputObject(InputObjectConfig{
		Name:   "Filter",
		Fields: []*InputField{NewInputField("ignored", InputFieldConfig{Type: str})},
		FieldsThunk: func() []*InputField {
			return []*InputField{nil, NewInputField("used", InputFieldConfig{Type: str})}
		},
	})

	if in.Field("used") == nil {
		t.Error("the thunk's field was not indexed past the nil entry")
	}
	if in.Field("ignored") != nil {
		t.Error("the plain slice was used even though a thunk was given")
	}
}

func TestInputObject_Predicates(t *testing.T) {
	in := NewInputObject(InputObjectConfig{Name: "Filter", Description: value.Just("Narrows a search.")})

	if !IsInputObjectType(in) {
		t.Error("IsInputObjectType = false, want true")
	}
	// An input object may only be used as input; it is not something a field
	// can return, and a selection set cannot be written against it.
	if !IsInputType(in) {
		t.Error("an input object was not reported as an input type")
	}
	if IsOutputType(in) {
		t.Error("an input object was reported as an output type")
	}
	if IsLeafType(in) || IsCompositeType(in) || IsAbstractType(in) {
		t.Error("an input object was reported as a leaf, composite or abstract type")
	}

	if got := in.Name(); got != "Filter" {
		t.Errorf("Name() = %q", got)
	}
	if got := in.Description(); got != "Narrows a search." {
		t.Errorf("Description() = %q", got)
	}
	if got := in.String(); got != "Filter" {
		t.Errorf("String() = %q", got)
	}
	var absent *InputObjectType
	if got := absent.String(); got != "<nil>" {
		t.Errorf("String() on nil = %q", got)
	}
}

// A list of an input object stays usable as input, and one of an output type
// does not, which is the recursion in IsInputType doing its job across the
// concrete types rather than the stubs.
func TestInputObject_WrappedUsability(t *testing.T) {
	in := NewInputObject(InputObjectConfig{Name: "Filter"})
	obj := NewObject(ObjectConfig{Name: "User"})

	if !IsInputType(NewNonNull(NewList(in))) {
		t.Error("a wrapped input object was not reported as an input type")
	}
	if IsInputType(NewList(obj)) {
		t.Error("a list of an object was reported as an input type")
	}
	if !IsOutputType(NewNonNull(NewList(obj))) {
		t.Error("a wrapped object was not reported as an output type")
	}
}

func TestInputObject_ResolveThunks(t *testing.T) {
	called := false
	in := NewInputObject(InputObjectConfig{
		Name: "Filter",
		FieldsThunk: func() []*InputField {
			called = true
			return nil
		},
	})
	if called {
		t.Fatal("the thunk ran before anything read the fields")
	}
	in.resolveThunks()
	if !called {
		t.Error("resolveThunks did not run the thunk")
	}
}
