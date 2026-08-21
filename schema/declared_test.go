package schema

import (
	"slices"
	"testing"
)

// A schema names a type where a kind belongs, and what it named may not be of
// that kind. The three answers are: nothing named at all, something of the
// kind, and something else.
func TestDeclared_ThreeStates(t *testing.T) {
	object := NewObject(ObjectConfig{Name: "Photo"})
	other := NewScalar(ScalarConfig{Name: "String"})

	for _, tt := range []struct {
		name     string
		declared Declared[*ObjectType]
		set      bool
		named    NamedType
		typed    string // the name Get answers with, or "" where it answers nothing
		printed  string
	}{
		{
			name:     "nothing named",
			declared: Declared[*ObjectType]{},
			printed:  "<nil>",
		},
		{
			name:     "the kind that belongs",
			declared: Declare(object),
			set:      true,
			named:    object,
			typed:    "Photo",
			printed:  "Photo",
		},
		{
			name:     "something else",
			declared: DeclareNamed[*ObjectType](other),
			set:      true,
			named:    other,
			printed:  "String",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.declared.IsSet(); got != tt.set {
				t.Errorf("IsSet() = %v, want %v", got, tt.set)
			}
			if got := tt.declared.Named(); got != tt.named {
				t.Errorf("Named() = %v, want %v", got, tt.named)
			}
			want := ""
			if tt.named != nil {
				want = tt.named.Name()
			}
			if got := tt.declared.Name(); got != want {
				t.Errorf("Name() = %q, want %q", got, want)
			}
			if got := tt.declared.String(); got != tt.printed {
				t.Errorf("String() = %q, want %q", got, tt.printed)
			}

			held, isKind := tt.declared.Get()
			if isKind != (tt.typed != "") {
				t.Fatalf("Get() said %v, want %v", isKind, tt.typed != "")
			}
			if isKind && held.Name() != tt.typed {
				t.Errorf("Get() = %v, want %s", held, tt.typed)
			}
			if !isKind && held != nil {
				t.Errorf("Get() answered %v as well as false", held)
			}
		})
	}
}

// A type that is not there is not something named: a list assembled in Go may
// have a hole in it, and the schema check is what reports the hole rather than
// this quietly closing it.
func TestDeclared_ATypeThatIsNotThere(t *testing.T) {
	var absent *ObjectType
	for _, tt := range []struct {
		name     string
		declared Declared[*ObjectType]
	}{
		{"a nil of the kind that belongs", Declare(absent)},
		{"a nil named type", DeclareNamed[*ObjectType](nil)},
		{"a typed nil named type", DeclareNamed[*ObjectType](absent)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.declared.IsSet() {
				t.Error("IsSet() = true for a type that is not there")
			}
			if got := tt.declared.Named(); got != nil {
				t.Errorf("Named() = %v, want nothing", got)
			}
			if got := tt.declared.Name(); got != "" {
				t.Errorf("Name() = %q, want empty", got)
			}
		})
	}
}

// Members and Implements are how a schema written in Go gives these, and they
// take the kind itself: naming the wrong one does not compile, which is the
// point of the type parameter.
func TestDeclared_FromGo(t *testing.T) {
	photo := NewObject(ObjectConfig{Name: "Photo"})
	video := NewObject(ObjectConfig{Name: "Video"})
	node := NewInterface(InterfaceConfig{Name: "Node"})

	t.Run("members", func(t *testing.T) {
		got := Members(photo, video)
		if want := []string{"Photo", "Video"}; !slices.Equal(declaredNames(got), want) {
			t.Errorf("Members() = %v, want %v", declaredNames(got), want)
		}
		for i, m := range got {
			if _, isObject := m.Get(); !isObject {
				t.Errorf("member %d is not the kind it was given as", i)
			}
		}
	})

	t.Run("interfaces", func(t *testing.T) {
		got := Implements(node)
		if want := []string{"Node"}; !slices.Equal(declaredNames(got), want) {
			t.Errorf("Implements() = %v, want %v", declaredNames(got), want)
		}
	})

	t.Run("nothing given is nothing held", func(t *testing.T) {
		if got := Members(); got != nil {
			t.Errorf("Members() = %v, want nil", got)
		}
		if got := Implements(); got != nil {
			t.Errorf("Implements() = %v, want nil", got)
		}
	})

	t.Run("a hole is kept where it was", func(t *testing.T) {
		got := Members(photo, nil, video)
		if len(got) != 3 {
			t.Fatalf("%d members, want the hole kept", len(got))
		}
		if got[1].IsSet() {
			t.Error("the hole was closed")
		}
	})
}

// What a document named comes back out, of the right kind or not, which is
// what lets a broken schema be printed and described.
func TestDeclared_FromADocument(t *testing.T) {
	input := NewInputObject(InputObjectConfig{Name: "In"})
	union := NewUnion(UnionConfig{
		Name:  "U",
		Types: []Declared[*ObjectType]{DeclareNamed[*ObjectType](input)},
	})

	members := union.Types()
	if len(members) != 1 {
		t.Fatalf("%d members, want the one the document named", len(members))
	}
	if got := members[0].Named(); got != NamedType(input) {
		t.Errorf("Named() = %v, want the input object", got)
	}
	if _, isObject := members[0].Get(); isObject {
		t.Error("an input object was answered as an object type")
	}

	// And nothing a request could land on, which is the narrower question.
	s := New(Config{Query: NewObject(ObjectConfig{Name: "Query"}), Types: []NamedType{union}})
	if got := s.PossibleTypes(union); len(got) != 0 {
		t.Errorf("PossibleTypes = %v, want none", got)
	}

	// The schema is unsound, and this is what says so.
	if errs := ValidateSchema(s); len(errs) == 0 {
		t.Error("a union of an input object was found sound")
	}
}

// declaredNames reads the names out of a list of declarations.
func declaredNames[T NamedType](declared []Declared[T]) []string {
	names := make([]string, 0, len(declared))
	for _, d := range declared {
		names = append(names, d.Name())
	}
	return names
}
