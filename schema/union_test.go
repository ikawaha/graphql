package schema

import (
	"github.com/ikawaha/graphql/value"
	"slices"
	"testing"
)

func typeNames(types []Declared[*ObjectType]) []string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = t.Name()
	}
	return names
}

func TestUnion_MembersKeepDeclarationOrder(t *testing.T) {
	photo := NewObject(ObjectConfig{Name: "Photo"})
	video := NewObject(ObjectConfig{Name: "Video"})

	u := NewUnion(UnionConfig{Name: "Media", Types: Members(photo, video)})

	if got := typeNames(u.Types()); !slices.Equal(got, []string{"Photo", "Video"}) {
		t.Errorf("Types() = %v, want [Photo Video]", got)
	}
}

func TestUnion_HasType(t *testing.T) {
	photo := NewObject(ObjectConfig{Name: "Photo"})
	video := NewObject(ObjectConfig{Name: "Video"})
	other := NewObject(ObjectConfig{Name: "Other"})

	u := NewUnion(UnionConfig{Name: "Media", Types: Members(photo, video)})

	if !u.HasType(photo) || !u.HasType(video) {
		t.Error("a declared member was not recognised")
	}
	if u.HasType(other) {
		t.Error("a type that is not a member was recognised")
	}
	if u.HasType(nil) {
		t.Error("HasType(nil) = true, want false")
	}

	// A different type that happens to share a name is not the same member.
	impostor := NewObject(ObjectConfig{Name: "Photo"})
	if u.HasType(impostor) {
		t.Error("a different type with the same name was recognised as a member")
	}
}

// A member whose own field points back at the union cannot be listed with a
// plain slice, which is what the thunk is for.
func TestUnion_RecursiveMembers(t *testing.T) {
	var media *UnionType
	photo := NewObject(ObjectConfig{
		Name: "Photo",
		FieldsThunk: func() []*Field {
			return []*Field{NewField("related", FieldConfig{Type: NewList(media)})}
		},
	})
	media = NewUnion(UnionConfig{
		Name:       "Media",
		TypesThunk: func() []Declared[*ObjectType] { return Members(photo) },
	})

	if got := typeNames(media.Types()); !slices.Equal(got, []string{"Photo"}) {
		t.Errorf("Types() = %v, want [Photo]", got)
	}
	related := photo.Field("related")
	if related == nil {
		t.Fatal("the recursive field is missing")
	}
	if got := related.Type.(*List).OfType; got != Type(media) {
		t.Errorf("related points at %v, want the union", got)
	}
}

func TestUnion_ThunkWinsAndNilsAreSkipped(t *testing.T) {
	ignored := NewObject(ObjectConfig{Name: "Ignored"})
	used := NewObject(ObjectConfig{Name: "Used"})

	u := NewUnion(UnionConfig{
		Name:       "Media",
		Types:      Members(ignored),
		TypesThunk: func() []Declared[*ObjectType] { return Members(nil, used) },
	})

	if !u.HasType(used) {
		t.Error("the thunk's member was not indexed past the nil entry")
	}
	if u.HasType(ignored) {
		t.Error("the plain slice was used even though a thunk was given")
	}
}

func TestUnion_Predicates(t *testing.T) {
	u := NewUnion(UnionConfig{Name: "Media", Description: value.Just("Either kind.")})

	if !IsUnionType(u) {
		t.Error("IsUnionType = false, want true")
	}
	// A selection set is written against a union, and its runtime type has to
	// be worked out during execution.
	if !IsCompositeType(u) || !IsAbstractType(u) {
		t.Errorf("union: composite=%v abstract=%v, want both true",
			IsCompositeType(u), IsAbstractType(u))
	}
	if IsInputType(u) {
		t.Error("a union was reported as an input type")
	}
	if !IsOutputType(u) {
		t.Error("a union was not reported as an output type")
	}
	if IsLeafType(u) {
		t.Error("a union was reported as a leaf")
	}

	if got := u.Name(); got != "Media" {
		t.Errorf("Name() = %q", got)
	}
	if got := u.Description(); got != "Either kind." {
		t.Errorf("Description() = %q", got)
	}
	if got := u.String(); got != "Media" {
		t.Errorf("String() = %q", got)
	}
	var absent *UnionType
	if got := absent.String(); got != "<nil>" {
		t.Errorf("String() on nil = %q", got)
	}
}

func TestUnion_ResolveThunks(t *testing.T) {
	called := false
	u := NewUnion(UnionConfig{
		Name: "Media",
		TypesThunk: func() []Declared[*ObjectType] {
			called = true
			return nil
		},
	})
	if called {
		t.Fatal("the thunk ran before anything read the members")
	}
	u.resolveThunks()
	if !called {
		t.Error("resolveThunks did not run the thunk")
	}
}
