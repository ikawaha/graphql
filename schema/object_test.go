package schema

import (
	"github.com/ikawaha/graphql/value"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
)

func fieldNames(fields []*Field) []string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.Name()
	}
	return names
}

// Fields keep the order they were declared in, because printing a schema has
// to give back the order it was written in rather than an alphabetical one.
func TestObject_FieldsKeepDeclarationOrder(t *testing.T) {
	str := testScalar("String")
	obj := NewObject(ObjectConfig{
		Name: "User",
		Fields: []*Field{
			NewField("zebra", FieldConfig{Type: str}),
			NewField("alpha", FieldConfig{Type: str}),
			NewField("middle", FieldConfig{Type: str}),
		},
	})

	want := []string{"zebra", "alpha", "middle"}
	if got := fieldNames(obj.Fields()); !slices.Equal(got, want) {
		t.Errorf("Fields() = %v, want %v", got, want)
	}
}

func TestObject_FieldLookup(t *testing.T) {
	str := testScalar("String")
	obj := NewObject(ObjectConfig{
		Name:   "User",
		Fields: []*Field{NewField("name", FieldConfig{Type: str})},
	})

	field := obj.Field("name")
	if field == nil {
		t.Fatal("Field(name) = nil, want the field")
	}
	if field.Type != Type(str) {
		t.Errorf("field type = %v, want the scalar", field.Type)
	}
	if obj.Field("missing") != nil {
		t.Error("Field(missing) returned something")
	}
}

// A field learns which type it belongs to when the type is built, which is
// what lets it name itself as "Type.field".
func TestObject_FieldsLearnTheirParent(t *testing.T) {
	str := testScalar("String")
	field := NewField("name", FieldConfig{Type: str})
	if field.Parent() != nil {
		t.Error("a field knows its parent before being attached to one")
	}

	obj := NewObject(ObjectConfig{Name: "User", Fields: []*Field{field}})
	_ = obj.Fields()

	// The type holds a field of its own, so the one handed in is left as it
	// was: the same field put into two types would otherwise belong to
	// whichever was built second, and would say so in every message.
	if got := field.Parent(); got != nil {
		t.Errorf("the field handed in was attached to %v", got)
	}
	held := obj.Field("name")
	if got := held.Parent(); got != NamedType(obj) {
		t.Errorf("Parent() = %v, want the object type", got)
	}
	if got := held.String(); got != "User.name" {
		t.Errorf("String() = %q, want %q", got, "User.name")
	}
}

// A type whose field points back at itself cannot be written with a plain
// slice, because the type does not exist yet at that point. The thunk is what
// makes it expressible.
func TestObject_SelfReference(t *testing.T) {
	var user *ObjectType
	user = NewObject(ObjectConfig{
		Name: "User",
		FieldsThunk: func() []*Field {
			return []*Field{
				NewField("name", FieldConfig{Type: testScalar("String")}),
				NewField("friends", FieldConfig{Type: NewList(user)}),
			}
		},
	})

	friends := user.Field("friends")
	if friends == nil {
		t.Fatal("the recursive field is missing")
	}
	list, ok := friends.Type.(*List)
	if !ok {
		t.Fatalf("friends type is %T, want a list", friends.Type)
	}
	if list.OfType != Type(user) {
		t.Errorf("friends element type = %v, want the type itself", list.OfType)
	}
}

// Two types that refer to each other work for the same reason.
func TestObject_MutualRecursion(t *testing.T) {
	var author, book *ObjectType

	author = NewObject(ObjectConfig{
		Name: "Author",
		FieldsThunk: func() []*Field {
			return []*Field{NewField("books", FieldConfig{Type: NewList(book)})}
		},
	})
	book = NewObject(ObjectConfig{
		Name: "Book",
		FieldsThunk: func() []*Field {
			return []*Field{NewField("author", FieldConfig{Type: author})}
		},
	})

	if got := author.Field("books").Type.(*List).OfType; got != Type(book) {
		t.Errorf("Author.books points at %v, want Book", got)
	}
	if got := book.Field("author").Type; got != Type(author) {
		t.Errorf("Book.author points at %v, want Author", got)
	}
}

// The thunk runs once however many times the fields are read, so a schema is
// not rebuilt on every request.
func TestObject_ThunkRunsOnce(t *testing.T) {
	var calls atomic.Int32
	obj := NewObject(ObjectConfig{
		Name: "User",
		FieldsThunk: func() []*Field {
			calls.Add(1)
			return []*Field{NewField("name", FieldConfig{Type: testScalar("String")})}
		},
	})

	for range 5 {
		_ = obj.Fields()
		_ = obj.Field("name")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("the thunk ran %d times, want 1", got)
	}
}

// The reference implementation resolves a thunk by overwriting the field it
// was stored in, which would be a data race here because one schema serves
// many requests at once. This is the test that decision stands on: run it with
// -race.
func TestObject_ConcurrentFieldAccess(t *testing.T) {
	var calls atomic.Int32
	obj := NewObject(ObjectConfig{
		Name: "User",
		FieldsThunk: func() []*Field {
			calls.Add(1)
			return []*Field{
				NewField("name", FieldConfig{Type: testScalar("String")}),
				NewField("age", FieldConfig{Type: testScalar("Int")}),
			}
		},
		InterfacesThunk: func() []Declared[*InterfaceType] {
			return Implements(NewInterface(InterfaceConfig{Name: "Node"}))
		},
	})

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if len(obj.Fields()) != 2 {
				t.Error("a reader saw an incomplete field list")
			}
			if obj.Field("name") == nil {
				t.Error("a reader could not find a field")
			}
			if len(obj.Interfaces()) != 1 {
				t.Error("a reader saw an incomplete interface list")
			}
		}()
	}
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Errorf("the thunk ran %d times under concurrent readers, want 1", got)
	}
}

// Given both forms, the thunk is what counts, because a caller that wrote one
// meant to defer.
func TestObject_ThunkWinsOverSlice(t *testing.T) {
	str := testScalar("String")
	obj := NewObject(ObjectConfig{
		Name:        "User",
		Fields:      []*Field{NewField("ignored", FieldConfig{Type: str})},
		FieldsThunk: func() []*Field { return []*Field{NewField("used", FieldConfig{Type: str})} },
	})

	if got := fieldNames(obj.Fields()); !slices.Equal(got, []string{"used"}) {
		t.Errorf("Fields() = %v, want the thunk's fields", got)
	}
}

func TestObject_Interfaces(t *testing.T) {
	node := NewInterface(InterfaceConfig{Name: "Node"})
	named := NewInterface(InterfaceConfig{Name: "Named"})

	obj := NewObject(ObjectConfig{
		Name:       "User",
		Interfaces: Implements(node, named),
	})

	got := obj.Interfaces()
	if len(got) != 2 || got[0].Named() != NamedType(node) || got[1].Named() != NamedType(named) {
		t.Errorf("Interfaces() = %v, want [Node Named] in order", got)
	}

	empty := NewObject(ObjectConfig{Name: "Plain"})
	if len(empty.Interfaces()) != 0 {
		t.Errorf("Interfaces() = %v, want none", empty.Interfaces())
	}
}

// A name written twice is one field: the last of them, where the name first
// appeared. graphql-js builds its fields from an object literal, where writing
// a key twice is writing it once, and a Go list standing in for one answers
// the same.
func TestObject_DuplicateFieldNameKeepsTheLast(t *testing.T) {
	first := NewField("name", FieldConfig{Type: testScalar("String")})
	second := NewField("name", FieldConfig{Type: testScalar("Int")})
	other := NewField("age", FieldConfig{Type: testScalar("Int")})
	obj := NewObject(ObjectConfig{Name: "User", Fields: []*Field{first, other, second}})

	if got := obj.Field("name"); got == nil || got.Type != second.Type {
		t.Errorf("Field(\"name\") = %v, want the last field of that name", got)
	}
	var named []string
	for _, f := range obj.Fields() {
		named = append(named, f.Name())
	}
	if want := []string{"name", "age"}; !slices.Equal(named, want) {
		t.Errorf("Fields() = %v, want %v", named, want)
	}
}

func TestObject_NilFieldsAreSkipped(t *testing.T) {
	obj := NewObject(ObjectConfig{
		Name:   "User",
		Fields: []*Field{nil, NewField("name", FieldConfig{Type: testScalar("String")})},
	})
	if obj.Field("name") == nil {
		t.Error("a nil entry stopped the following field being indexed")
	}
}

func TestObject_TypePredicates(t *testing.T) {
	obj := NewObject(ObjectConfig{Name: "User"})
	iface := NewInterface(InterfaceConfig{Name: "Node"})

	if !IsObjectType(obj) || IsObjectType(iface) {
		t.Error("IsObjectType did not tell an object from an interface")
	}
	if !IsInterfaceType(iface) || IsInterfaceType(obj) {
		t.Error("IsInterfaceType did not tell an interface from an object")
	}

	// A selection set can be written against either.
	if !IsCompositeType(obj) || !IsCompositeType(iface) {
		t.Error("an object or an interface was not reported as composite")
	}
	// Only the interface needs its runtime type worked out.
	if IsAbstractType(obj) {
		t.Error("an object was reported as abstract")
	}
	if !IsAbstractType(iface) {
		t.Error("an interface was not reported as abstract")
	}

	// Neither may be used as input, and neither is a leaf.
	for _, typ := range []Type{obj, iface} {
		if IsInputType(typ) {
			t.Errorf("%s was reported as an input type", typ)
		}
		if !IsOutputType(typ) {
			t.Errorf("%s was not reported as an output type", typ)
		}
		if IsLeafType(typ) {
			t.Errorf("%s was reported as a leaf", typ)
		}
	}

	if IsCompositeType(testScalar("Int")) {
		t.Error("a scalar was reported as composite")
	}
}

func TestInterface_FieldsAndRecursion(t *testing.T) {
	var node *InterfaceType
	node = NewInterface(InterfaceConfig{
		Name: "Node",
		FieldsThunk: func() []*Field {
			return []*Field{
				NewField("id", FieldConfig{Type: NewNonNull(testScalar("ID"))}),
				NewField("parent", FieldConfig{Type: node}),
			}
		},
	})

	if got := fieldNames(node.Fields()); !slices.Equal(got, []string{"id", "parent"}) {
		t.Errorf("Fields() = %v, want [id parent]", got)
	}
	if got := node.Field("parent").Type; got != Type(node) {
		t.Errorf("parent points at %v, want the interface itself", got)
	}
	if got := node.Field("id").String(); got != "Node.id" {
		t.Errorf("String() = %q, want %q", got, "Node.id")
	}
}

func TestField_Args(t *testing.T) {
	first := NewArgument("first", ArgumentConfig{Type: testScalar("Int")})
	after := NewArgument("after", ArgumentConfig{Type: testScalar("String")})

	field := NewField("friends", FieldConfig{
		Type: testScalar("String"),
		Args: []*Argument{first, after},
	})

	if got := field.Arg("first"); got != first {
		t.Errorf("Arg(first) = %v, want the argument", got)
	}
	if field.Arg("missing") != nil {
		t.Error("Arg(missing) returned something")
	}
	var absent *Field
	if absent.Arg("first") != nil {
		t.Error("Arg on a nil field returned something")
	}
}

func TestField_Accessors(t *testing.T) {
	field := NewField("name", FieldConfig{
		Description:       value.Just("The name."),
		Type:              testScalar("String"),
		DeprecationReason: DeprecatedFor("Use fullName."),
	})

	if got := field.Description(); got != "The name." {
		t.Errorf("Description() = %q", got)
	}
	if !field.IsDeprecated() {
		t.Error("IsDeprecated() = false, want true")
	}
	// A field with no owner is graphql-js's meta-field case.
	if got := field.String(); got != "<meta>.name" {
		t.Errorf("String() before attachment = %q, want %q", got, "<meta>.name")
	}

	var absent *Field
	if got := absent.String(); got != "<nil>" {
		t.Errorf("String() on nil = %q", got)
	}
}

func TestObjectAndInterface_StringOnNil(t *testing.T) {
	var obj *ObjectType
	if got := obj.String(); got != "<nil>" {
		t.Errorf("ObjectType.String() on nil = %q", got)
	}
	var iface *InterfaceType
	if got := iface.String(); got != "<nil>" {
		t.Errorf("InterfaceType.String() on nil = %q", got)
	}
}

func TestResolveThunks(t *testing.T) {
	var built atomic.Int32
	obj := NewObject(ObjectConfig{
		Name: "User",
		FieldsThunk: func() []*Field {
			built.Add(1)
			return nil
		},
	})
	if got := built.Load(); got != 0 {
		t.Fatalf("the thunk ran %d times before anything read the fields", got)
	}

	obj.resolveThunks()
	if got := built.Load(); got != 1 {
		t.Errorf("the thunk ran %d times after resolveThunks, want 1", got)
	}

	iface := NewInterface(InterfaceConfig{Name: "Node"})
	iface.resolveThunks() // must not panic on a type with nothing deferred
}
