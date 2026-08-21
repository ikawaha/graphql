package schema

import (
	"github.com/ikawaha/graphql/value"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ikawaha/graphql/language"
)

func namedTypeNames(types []NamedType) []string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = t.Name()
	}
	slices.Sort(names)
	return names
}

// A schema holds everything it can reach from its roots, following field
// types, argument types, interfaces, union members and input object fields.
func TestSchema_CollectsReachableTypes(t *testing.T) {
	filter := NewInputObject(InputObjectConfig{
		Name:   "Filter",
		Fields: []*InputField{NewInputField("term", InputFieldConfig{Type: String})},
	})
	node := NewInterface(InterfaceConfig{
		Name:   "Node",
		Fields: []*Field{NewField("id", FieldConfig{Type: NewNonNull(ID)})},
	})
	photo := NewObject(ObjectConfig{
		Name:       "Photo",
		Interfaces: Implements(node),
		Fields:     []*Field{NewField("url", FieldConfig{Type: String})},
	})
	video := NewObject(ObjectConfig{
		Name:   "Video",
		Fields: []*Field{NewField("length", FieldConfig{Type: Int})},
	})
	media := NewUnion(UnionConfig{Name: "Media", Types: Members(photo, video)})

	query := NewObject(ObjectConfig{
		Name: "Query",
		Fields: []*Field{
			NewField("search", FieldConfig{
				Type: NewList(media),
				Args: []*Argument{NewArgument("filter", ArgumentConfig{Type: filter})},
			}),
		},
	})

	s := New(Config{Query: query})

	want := []string{
		"Boolean", "Filter", "ID", "Int", "Media", "Node",
		"Photo", "Query", "String", "Video",
		// Every schema can describe itself, so it carries these too.
		"__Directive", "__DirectiveLocation", "__EnumValue", "__Field",
		"__InputValue", "__Schema", "__Type", "__TypeKind",
	}
	if got := namedTypeNames(s.Types()); !slices.Equal(got, want) {
		t.Errorf("types = %v, want %v", got, want)
	}
	// Boolean and String arrive through the built-in directives' arguments,
	// which is the only path to them here.
	if s.Type("Boolean") == nil {
		t.Error("Boolean was not reached through the directive arguments")
	}
	if s.Type("Missing") != nil {
		t.Error("Type returned something for a name the schema does not have")
	}
}

// A type only ever produced at run time is unreachable from the roots, so a
// schema has to be told about it.
func TestSchema_ExtraTypes(t *testing.T) {
	node := NewInterface(InterfaceConfig{
		Name:   "Node",
		Fields: []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	user := NewObject(ObjectConfig{
		Name:       "User",
		Interfaces: Implements(node),
		Fields:     []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	query := NewObject(ObjectConfig{
		Name:   "Query",
		Fields: []*Field{NewField("node", FieldConfig{Type: node})},
	})

	without := New(Config{Query: query})
	if without.Type("User") != nil {
		t.Error("an unreachable type was found without being listed")
	}

	with := New(Config{Query: query, Types: []NamedType{user}})
	if with.Type("User") == nil {
		t.Error("a listed type was not added to the schema")
	}
}

// This is the test decision 26 rests on. Building a schema walks every type,
// which resolves the deferred field lists; nothing is left to compute when a
// request arrives.
func TestSchema_ResolvesEveryThunkWhileBuilding(t *testing.T) {
	var queryFields, userFields, memberTypes atomic.Int32

	var user *ObjectType
	user = NewObject(ObjectConfig{
		Name: "User",
		FieldsThunk: func() []*Field {
			userFields.Add(1)
			return []*Field{
				NewField("name", FieldConfig{Type: String}),
				NewField("friends", FieldConfig{Type: NewList(user)}),
			}
		},
	})
	media := NewUnion(UnionConfig{
		Name: "Media",
		TypesThunk: func() []Declared[*ObjectType] {
			memberTypes.Add(1)
			return Members(NewObject(ObjectConfig{Name: "Photo"}))
		},
	})
	query := NewObject(ObjectConfig{
		Name: "Query",
		FieldsThunk: func() []*Field {
			queryFields.Add(1)
			return []*Field{
				NewField("me", FieldConfig{Type: user}),
				NewField("media", FieldConfig{Type: media}),
			}
		},
	})

	if got := queryFields.Load() + userFields.Load() + memberTypes.Load(); got != 0 {
		t.Fatalf("%d thunks ran before the schema was built", got)
	}

	s := New(Config{Query: query})

	for name, count := range map[string]int32{
		"Query.fields": queryFields.Load(),
		"User.fields":  userFields.Load(),
		"Media.types":  memberTypes.Load(),
	} {
		if count != 1 {
			t.Errorf("%s ran %d times during construction, want 1", name, count)
		}
	}
	if s.Type("Photo") == nil {
		t.Error("a type reached only through a thunk is missing")
	}

	// Reading the schema afterwards must not run anything again.
	_ = s.QueryType().Fields()
	_ = s.Type("User").(*ObjectType).Fields()
	if got := queryFields.Load() + userFields.Load() + memberTypes.Load(); got != 3 {
		t.Errorf("thunks ran %d times in total, want 3", got)
	}
}

// A finished schema is read by every request at once, so reading it must need
// no synchronisation. Run this with -race.
func TestSchema_ConcurrentReads(t *testing.T) {
	node := NewInterface(InterfaceConfig{
		Name:   "Node",
		Fields: []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	user := NewObject(ObjectConfig{
		Name:        "User",
		Interfaces:  Implements(node),
		FieldsThunk: func() []*Field { return []*Field{NewField("id", FieldConfig{Type: ID})} },
	})
	query := NewObject(ObjectConfig{
		Name:   "Query",
		Fields: []*Field{NewField("me", FieldConfig{Type: user})},
	})
	s := New(Config{Query: query, Types: []NamedType{user}})

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s.Type("User") == nil {
				t.Error("a reader could not find a type")
			}
			if len(s.QueryType().Fields()) != 1 {
				t.Error("a reader saw an incomplete field list")
			}
			if len(s.PossibleTypes(node)) != 1 {
				t.Error("a reader saw an incomplete implementations index")
			}
			if s.Directive("skip") == nil {
				t.Error("a reader could not find a directive")
			}
		}()
	}
	wg.Wait()
}

func TestSchema_RootTypes(t *testing.T) {
	query := NewObject(ObjectConfig{Name: "Query", Fields: []*Field{NewField("a", FieldConfig{Type: String})}})
	mutation := NewObject(ObjectConfig{Name: "Mutation", Fields: []*Field{NewField("b", FieldConfig{Type: String})}})
	subscription := NewObject(ObjectConfig{Name: "Subscription", Fields: []*Field{NewField("c", FieldConfig{Type: String})}})

	s := New(Config{Query: query, Mutation: mutation, Subscription: subscription})

	if s.QueryType() != query || s.MutationType() != mutation || s.SubscriptionType() != subscription {
		t.Error("the root types were not kept")
	}
	tests := map[language.OperationType]*ObjectType{
		language.OperationQuery:        query,
		language.OperationMutation:     mutation,
		language.OperationSubscription: subscription,
	}
	for op, want := range tests {
		if got := s.RootType(op); got != want {
			t.Errorf("RootType(%s) = %v, want %v", op, got, want)
		}
	}
	if got := s.RootType("nonsense"); got != nil {
		t.Errorf("RootType of an unknown operation = %v, want nil", got)
	}

	// A schema may have only a query.
	bare := New(Config{Query: query})
	if bare.MutationType() != nil || bare.SubscriptionType() != nil {
		t.Error("a schema with only a query reported other roots")
	}
	if got := bare.RootType(language.OperationMutation); got != nil {
		t.Errorf("RootType(mutation) = %v, want nil", got)
	}
}

func TestSchema_Directives(t *testing.T) {
	query := NewObject(ObjectConfig{Name: "Query", Fields: []*Field{NewField("a", FieldConfig{Type: String})}})

	// Left out, a schema gets the built-in directives.
	standard := New(Config{Query: query})
	if len(standard.Directives()) != len(SpecifiedDirectives) {
		t.Errorf("%d directives, want the built-in ones", len(standard.Directives()))
	}
	for _, name := range []string{"skip", "include", "deprecated", "specifiedBy", "oneOf"} {
		if standard.Directive(name) == nil {
			t.Errorf("the built-in @%s is missing", name)
		}
	}
	if standard.Directive("missing") != nil {
		t.Error("Directive returned something for a name the schema does not have")
	}

	// Supplied, they replace the built-in ones rather than adding to them.
	custom := NewDirective(DirectiveConfig{
		Name:      "auth",
		Locations: []language.DirectiveLocation{language.DirectiveLocationFieldDefinition},
	})
	replaced := New(Config{Query: query, Directives: []*Directive{custom}})
	if len(replaced.Directives()) != 1 {
		t.Errorf("%d directives, want only the one supplied", len(replaced.Directives()))
	}
	if replaced.Directive("skip") != nil {
		t.Error("a built-in directive survived being replaced")
	}
	if replaced.Directive("auth") != custom {
		t.Error("the supplied directive is missing")
	}
}

func TestSchema_Implementations(t *testing.T) {
	node := NewInterface(InterfaceConfig{
		Name:   "Node",
		Fields: []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	named := NewInterface(InterfaceConfig{
		Name:       "Named",
		Interfaces: Implements(node),
		Fields:     []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	user := NewObject(ObjectConfig{
		Name:       "User",
		Interfaces: Implements(node, named),
		Fields:     []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	query := NewObject(ObjectConfig{
		Name:   "Query",
		Fields: []*Field{NewField("node", FieldConfig{Type: node})},
	})

	s := New(Config{Query: query, Types: []NamedType{user, named}})

	nodeImpl := s.Implementations(node)
	if len(nodeImpl.Objects) != 1 || nodeImpl.Objects[0] != user {
		t.Errorf("Node objects = %v, want [User]", nodeImpl.Objects)
	}
	if len(nodeImpl.Interfaces) != 1 || nodeImpl.Interfaces[0] != named {
		t.Errorf("Node interfaces = %v, want [Named]", nodeImpl.Interfaces)
	}

	namedImpl := s.Implementations(named)
	if len(namedImpl.Objects) != 1 || namedImpl.Objects[0] != user {
		t.Errorf("Named objects = %v, want [User]", namedImpl.Objects)
	}
	if len(namedImpl.Interfaces) != 0 {
		t.Errorf("Named interfaces = %v, want none", namedImpl.Interfaces)
	}

	// An interface the schema does not know about answers emptily rather than
	// not at all.
	stranger := NewInterface(InterfaceConfig{Name: "Stranger"})
	if empty := s.Implementations(stranger); len(empty.Objects) != 0 || len(empty.Interfaces) != 0 {
		t.Errorf("Implementations of an unknown interface = %v", empty)
	}
	if empty := s.Implementations(nil); len(empty.Objects) != 0 {
		t.Errorf("Implementations(nil) = %v", empty)
	}
}

func TestSchema_PossibleTypesAndSubTypes(t *testing.T) {
	node := NewInterface(InterfaceConfig{
		Name:   "Node",
		Fields: []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	named := NewInterface(InterfaceConfig{
		Name:       "Named",
		Interfaces: Implements(node),
		Fields:     []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	photo := NewObject(ObjectConfig{
		Name:       "Photo",
		Interfaces: Implements(node),
		Fields:     []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	video := NewObject(ObjectConfig{Name: "Video", Fields: []*Field{NewField("id", FieldConfig{Type: ID})}})
	media := NewUnion(UnionConfig{Name: "Media", Types: Members(photo, video)})
	query := NewObject(ObjectConfig{
		Name: "Query",
		Fields: []*Field{
			NewField("node", FieldConfig{Type: node}),
			NewField("media", FieldConfig{Type: media}),
		},
	})

	s := New(Config{Query: query, Types: []NamedType{named, photo, video}})

	if got := s.PossibleTypes(media); len(got) != 2 {
		t.Errorf("PossibleTypes(Media) = %v, want two members", got)
	}
	if got := s.PossibleTypes(node); len(got) != 1 || got[0] != photo {
		t.Errorf("PossibleTypes(Node) = %v, want [Photo]", got)
	}
	stranger := NewInterface(InterfaceConfig{Name: "Stranger"})
	if got := s.PossibleTypes(stranger); got != nil {
		t.Errorf("PossibleTypes of an unknown interface = %v, want nil", got)
	}

	tests := []struct {
		name     string
		abstract AbstractType
		sub      NamedType
		want     bool
	}{
		{"a union member", media, photo, true},
		{"not a union member", media, NewObject(ObjectConfig{Name: "Other"}), false},
		{"an object implementing an interface", node, photo, true},
		{"an object that does not", node, video, false},
		{"an interface implementing another", node, named, true},
		{"an interface that does not", named, node, false},
		{"a union asked about an interface", media, node, false},
		{"an unknown interface", stranger, photo, false},
		{"nothing", node, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.IsSubType(tt.abstract, tt.sub); got != tt.want {
				t.Errorf("IsSubType(%v, %v) = %v, want %v", tt.abstract, tt.sub, got, tt.want)
			}
		})
	}
	if s.IsSubType(nil, photo) {
		t.Error("IsSubType(nil, ...) = true, want false")
	}
}

func TestSchema_Field(t *testing.T) {
	node := NewInterface(InterfaceConfig{
		Name:   "Node",
		Fields: []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	user := NewObject(ObjectConfig{
		Name:   "User",
		Fields: []*Field{NewField("name", FieldConfig{Type: String})},
	})
	media := NewUnion(UnionConfig{Name: "Media"})
	query := NewObject(ObjectConfig{
		Name: "Query",
		Fields: []*Field{
			NewField("me", FieldConfig{Type: user}),
			NewField("node", FieldConfig{Type: node}),
			NewField("media", FieldConfig{Type: media}),
		},
	})
	s := New(Config{Query: query})

	if got := s.Field(user, "name"); got == nil {
		t.Error("Field(User, name) = nil")
	}
	if got := s.Field(node, "id"); got == nil {
		t.Error("Field(Node, id) = nil")
	}
	if got := s.Field(user, "missing"); got != nil {
		t.Error("Field found a name the type does not have")
	}
	// A union has no fields of its own.
	if got := s.Field(media, "anything"); got != nil {
		t.Errorf("Field(Media, ...) = %v, want nil", got)
	}
}

// Building a schema does not fail. Problems found while gathering the types
// are kept for validation to report along with everything else.
func TestSchema_RecordsDuplicateNames(t *testing.T) {
	first := NewObject(ObjectConfig{Name: "User", Fields: []*Field{NewField("a", FieldConfig{Type: String})}})
	second := NewObject(ObjectConfig{Name: "User", Fields: []*Field{NewField("b", FieldConfig{Type: String})}})
	query := NewObject(ObjectConfig{
		Name: "Query",
		Fields: []*Field{
			NewField("first", FieldConfig{Type: first}),
			NewField("second", FieldConfig{Type: second}),
		},
	})

	s := New(Config{Query: query})
	if len(s.collectErrors) == 0 {
		t.Error("two types sharing a name were not recorded as a problem")
	}
	// The first one to be reached is the one the schema keeps.
	if s.Type("User") != NamedType(first) {
		t.Error("the later type of the same name replaced the earlier one")
	}

	duplicateDirective := NewDirective(DirectiveConfig{Name: "dup"})
	other := NewDirective(DirectiveConfig{Name: "dup"})
	withDup := New(Config{Query: query, Directives: []*Directive{duplicateDirective, other}})
	if len(withDup.collectErrors) == 0 {
		t.Error("two directives sharing a name were not recorded as a problem")
	}
}

func TestSchema_Description(t *testing.T) {
	query := NewObject(ObjectConfig{Name: "Query", Fields: []*Field{NewField("a", FieldConfig{Type: String})}})
	s := New(Config{Query: query, Description: value.Just("A schema.")})
	if got := s.Description(); got != "A schema." {
		t.Errorf("Description() = %q", got)
	}
}

// A schema assembled by generated code, or by hand under construction, can
// carry a nil in one of its slices. Building it must skip those rather than
// crash, so that validation gets a chance to report what is actually missing.
func TestSchema_ToleratesNilsWhileCollecting(t *testing.T) {
	iface := NewInterface(InterfaceConfig{
		Name:   "Node",
		Fields: []*Field{nil, NewField("id", FieldConfig{Type: ID})},
	})
	user := NewObject(ObjectConfig{
		Name:       "User",
		Interfaces: Implements(nil, iface),
		Fields: []*Field{
			nil,
			NewField("name", FieldConfig{
				Type: String,
				Args: []*Argument{nil, NewArgument("upper", ArgumentConfig{Type: Boolean})},
			}),
		},
	})
	input := NewInputObject(InputObjectConfig{
		Name:   "Filter",
		Fields: []*InputField{nil, NewInputField("term", InputFieldConfig{Type: String})},
	})
	query := NewObject(ObjectConfig{
		Name: "Query",
		Fields: []*Field{
			NewField("me", FieldConfig{
				Type: user,
				Args: []*Argument{NewArgument("filter", ArgumentConfig{Type: input})},
			}),
		},
	})

	s := New(Config{
		Query:      query,
		Types:      []NamedType{nil, user},
		Directives: []*Directive{nil, Skip},
	})

	for _, name := range []string{"Query", "User", "Node", "Filter", "String", "ID", "Boolean"} {
		if s.Type(name) == nil {
			t.Errorf("%s is missing from a schema that contained nils", name)
		}
	}
	if s.Directive("skip") == nil {
		t.Error("the directive after the nil one was not indexed")
	}
	if impl := s.Implementations(iface); len(impl.Objects) != 1 || impl.Objects[0] != user {
		t.Errorf("Node objects = %v, want [User]", impl.Objects)
	}
}

// The three meta-fields belong to no type in particular, so the schema answers
// for them where no type declares them. __typename may be asked of anything
// composite; the other two describe the schema itself and so belong only to
// the type a query enters through.
func TestSchema_MetaFields(t *testing.T) {
	iface := NewInterface(InterfaceConfig{
		Name:   "Node",
		Fields: []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	user := NewObject(ObjectConfig{
		Name:       "User",
		Interfaces: Implements(iface),
		Fields:     []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	union := NewUnion(UnionConfig{Name: "Media", Types: Members(user)})
	query := NewObject(ObjectConfig{
		Name: "Query",
		Fields: []*Field{
			NewField("me", FieldConfig{Type: user}),
			NewField("media", FieldConfig{Type: union}),
			NewField("node", FieldConfig{Type: iface}),
		},
	})
	s := New(Config{Query: query, Types: []NamedType{user}})

	t.Run("__typename is asked of anything composite", func(t *testing.T) {
		for _, parent := range []CompositeType{query, user, iface, union} {
			got := s.Field(parent, "__typename")
			if got != TypeNameMetaField {
				t.Errorf("__typename of %s = %v, want the meta-field", parent.Name(), got)
			}
		}
	})

	t.Run("__schema and __type belong to the query root", func(t *testing.T) {
		if got := s.Field(query, "__schema"); got != SchemaMetaField {
			t.Errorf("__schema of the root = %v, want the meta-field", got)
		}
		if got := s.Field(query, "__type"); got != TypeMetaField {
			t.Errorf("__type of the root = %v, want the meta-field", got)
		}
		for _, parent := range []CompositeType{user, iface, union} {
			if got := s.Field(parent, "__schema"); got != nil {
				t.Errorf("__schema of %s = %v, want nothing", parent.Name(), got)
			}
			if got := s.Field(parent, "__type"); got != nil {
				t.Errorf("__type of %s = %v, want nothing", parent.Name(), got)
			}
		}
	})

	t.Run("an ordinary field still resolves", func(t *testing.T) {
		if s.Field(user, "id") == nil {
			t.Error("a declared field was lost to the meta-field lookup")
		}
		if s.Field(user, "missing") != nil {
			t.Error("an undeclared field was found")
		}
		// A union has no fields of its own beyond the meta-fields.
		if s.Field(union, "id") != nil {
			t.Error("a union answered for a field of one of its members")
		}
	})

	t.Run("nothing to ask of", func(t *testing.T) {
		if got := s.Field(nil, "__typename"); got != nil {
			t.Errorf("__typename of nothing = %v, want nothing", got)
		}
		if got := s.Field(nil, "__schema"); got != nil {
			t.Errorf("__schema of nothing = %v, want nothing", got)
		}
	})
}

// SDL can be checked before there is a schema to check it against, so a nil
// schema has to answer questions rather than bring the process down. It knows
// nothing, which is the honest answer when there is no schema.
func TestSchema_NilKnowsNothing(t *testing.T) {
	var s *Schema

	if s.Description() != "" {
		t.Error("a nil schema has a description")
	}
	for name, got := range map[string]*ObjectType{
		"query":        s.QueryType(),
		"mutation":     s.MutationType(),
		"subscription": s.SubscriptionType(),
	} {
		if got != nil {
			t.Errorf("a nil schema has a %s root", name)
		}
	}
	for _, operation := range []language.OperationType{
		language.OperationQuery, language.OperationMutation, language.OperationSubscription,
	} {
		if s.RootType(operation) != nil {
			t.Errorf("a nil schema has a %s root", operation)
		}
	}
	if s.Types() != nil || s.Type("String") != nil {
		t.Error("a nil schema has types")
	}
	if s.Directives() != nil || s.Directive("skip") != nil {
		t.Error("a nil schema has directives")
	}

	iface := NewInterface(InterfaceConfig{Name: "Node", Fields: []*Field{NewField("id", FieldConfig{Type: ID})}})
	object := NewObject(ObjectConfig{Name: "User", Fields: []*Field{NewField("id", FieldConfig{Type: ID})}})
	if impl := s.Implementations(iface); len(impl.Objects) != 0 || len(impl.Interfaces) != 0 {
		t.Error("a nil schema knows of implementations")
	}
	if s.PossibleTypes(iface) != nil {
		t.Error("a nil schema has possible types")
	}
	if s.IsSubType(iface, object) {
		t.Error("a nil schema relates two types")
	}
	// A union answers from itself, so it works even with no schema.
	union := NewUnion(UnionConfig{Name: "Media", Types: Members(object)})
	if got := s.PossibleTypes(union); got != nil {
		t.Errorf("a nil schema returned %v for a union", got)
	}
	if s.Field(object, "id") == nil {
		t.Error("a nil schema cannot answer for a field of a type it was handed")
	}
	if s.Field(object, "__schema") != nil {
		t.Error("a nil schema answered for __schema")
	}
}
