package schema

import (
	"slices"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/value"
)

// messagesFor validates a schema and returns what it complained about.
func messagesFor(s *Schema) []string {
	errs := ValidateSchema(s)
	out := make([]string, len(errs))
	for i, err := range errs {
		out[i] = err.Message
	}
	return out
}

// wantMessage fails unless exactly one complaint mentions the given text.
func wantMessage(t *testing.T, s *Schema, want string) {
	t.Helper()
	got := messagesFor(s)
	found := 0
	for _, m := range got {
		if strings.Contains(m, want) {
			found++
		}
	}
	if found != 1 {
		t.Errorf("%d complaints mention %q, want 1. All of them:\n\t%s",
			found, want, strings.Join(got, "\n\t"))
	}
}

// wantValid fails if a schema is reported as anything but sound.
func wantValid(t *testing.T, s *Schema) {
	t.Helper()
	if got := messagesFor(s); len(got) != 0 {
		t.Errorf("a sound schema was reported as invalid:\n\t%s", strings.Join(got, "\n\t"))
	}
}

// simpleQuery is a query root with one field, enough to make a schema sound.
func simpleQuery() *ObjectType {
	return NewObject(ObjectConfig{
		Name:   "Query",
		Fields: []*Field{NewField("ok", FieldConfig{Type: String})},
	})
}

func TestValidateSchema_Valid(t *testing.T) {
	node := NewInterface(InterfaceConfig{
		Name:   "Node",
		Fields: []*Field{NewField("id", FieldConfig{Type: NewNonNull(ID)})},
	})
	user := NewObject(ObjectConfig{
		Name:       "User",
		Interfaces: Implements(node),
		Fields: []*Field{
			NewField("id", FieldConfig{Type: NewNonNull(ID)}),
			NewField("name", FieldConfig{
				Type: String,
				Args: []*Argument{NewArgument("upper", ArgumentConfig{Type: Boolean})},
			}),
		},
	})
	colour := NewEnum(EnumConfig{
		Name:   "Colour",
		Values: []*EnumValue{NewEnumValue("RED", EnumValueConfig{})},
	})
	filter := NewInputObject(InputObjectConfig{
		Name:   "Filter",
		Fields: []*InputField{NewInputField("term", InputFieldConfig{Type: String})},
	})
	media := NewUnion(UnionConfig{Name: "Media", Types: Members(user)})
	query := NewObject(ObjectConfig{
		Name: "Query",
		Fields: []*Field{
			NewField("me", FieldConfig{Type: user}),
			NewField("colour", FieldConfig{Type: colour}),
			NewField("media", FieldConfig{Type: media}),
			NewField("search", FieldConfig{
				Type: String,
				Args: []*Argument{NewArgument("filter", ArgumentConfig{Type: filter})},
			}),
		},
	})

	s := New(Config{Query: query})
	wantValid(t, s)
	if err := AssertValidSchema(s); err != nil {
		t.Errorf("AssertValidSchema = %v, want nil", err)
	}
}

func TestValidateSchema_RootTypes(t *testing.T) {
	t.Run("a schema needs a query root", func(t *testing.T) {
		wantMessage(t, New(Config{}), "Query root type must be provided")
	})

	t.Run("two operations cannot share a root", func(t *testing.T) {
		query := simpleQuery()
		s := New(Config{Query: query, Mutation: query})
		wantMessage(t, s, "All root types must be different")
	})

	// A request enters through an object and nothing else. The schema holds
	// whatever it was given — graphql-js does too — and this is what says so.
	t.Run("a root that is not an object", func(t *testing.T) {
		notAnObject := NewInputObject(InputObjectConfig{
			Name:   "In",
			Fields: []*InputField{NewInputField("f", InputFieldConfig{Type: Int})},
		})
		for _, tt := range []struct {
			name   string
			config Config
			says   string
		}{
			{
				"as the query root",
				Config{Query: notAnObject},
				`Query root type must be Object type, it cannot be In.`,
			},
			{
				"as the mutation root",
				Config{Query: simpleQuery(), Mutation: notAnObject},
				`Mutation root type must be Object type if provided, it cannot be In.`,
			},
			{
				"as the subscription root",
				Config{Query: simpleQuery(), Subscription: notAnObject},
				`Subscription root type must be Object type if provided, it cannot be In.`,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				s := New(tt.config)
				wantMessage(t, s, tt.says)
				// What a request could enter through is nothing, and what the
				// schema named is still there to be read.
				if s.RootType(language.OperationQuery) != nil && tt.name == "as the query root" {
					t.Error("an input object was answered as the type a query enters through")
				}
				if named := s.DeclaredRootType(language.OperationQuery); named == nil {
					t.Error("the schema forgot which type it named")
				}
			})
		}
	})

	// Nothing else is said about a root that is not an object: it is left out
	// of the check that two operations do not share one.
	t.Run("a root that is not an object is not also reported as shared", func(t *testing.T) {
		notAnObject := NewInputObject(InputObjectConfig{
			Name:   "In",
			Fields: []*InputField{NewInputField("f", InputFieldConfig{Type: Int})},
		})
		s := New(Config{Query: simpleQuery(), Mutation: notAnObject, Subscription: notAnObject})
		for _, m := range messagesFor(s) {
			if strings.Contains(m, "All root types must be different") {
				t.Errorf("said %q about a root nothing can enter through", m)
			}
		}
	})
}

func TestValidateSchema_ReservedNames(t *testing.T) {
	t.Run("a type", func(t *testing.T) {
		reserved := NewObject(ObjectConfig{
			Name:   "__Secret",
			Fields: []*Field{NewField("a", FieldConfig{Type: String})},
		})
		query := NewObject(ObjectConfig{
			Name:   "Query",
			Fields: []*Field{NewField("secret", FieldConfig{Type: reserved})},
		})
		wantMessage(t, New(Config{Query: query}), `Name "__Secret" must not begin with`)
	})

	t.Run("a field", func(t *testing.T) {
		query := NewObject(ObjectConfig{
			Name:   "Query",
			Fields: []*Field{NewField("__secret", FieldConfig{Type: String})},
		})
		wantMessage(t, New(Config{Query: query}), `Name "__secret" must not begin with`)
	})

	t.Run("an invalid name", func(t *testing.T) {
		query := NewObject(ObjectConfig{
			Name:   "Query",
			Fields: []*Field{NewField("has space", FieldConfig{Type: String})},
		})
		wantMessage(t, New(Config{Query: query}), "must only contain")
	})

	// The introspection types are the one place the reserved prefix belongs,
	// so they must not be reported.
	t.Run("the introspection types are exempt", func(t *testing.T) {
		wantValid(t, New(Config{Query: simpleQuery()}))
	})
}

func TestValidateSchema_Fields(t *testing.T) {
	t.Run("a type needs a field", func(t *testing.T) {
		empty := NewObject(ObjectConfig{Name: "Empty"})
		query := NewObject(ObjectConfig{
			Name:   "Query",
			Fields: []*Field{NewField("empty", FieldConfig{Type: empty})},
		})
		wantMessage(t, New(Config{Query: query}), "Type Empty must define one or more fields")
	})

	t.Run("a field type must be an output type", func(t *testing.T) {
		filter := NewInputObject(InputObjectConfig{
			Name:   "Filter",
			Fields: []*InputField{NewInputField("term", InputFieldConfig{Type: String})},
		})
		query := NewObject(ObjectConfig{
			Name:   "Query",
			Fields: []*Field{NewField("bad", FieldConfig{Type: filter})},
		})
		wantMessage(t, New(Config{Query: query}), "must be Output Type")
	})

	t.Run("an argument type must be an input type", func(t *testing.T) {
		out := NewObject(ObjectConfig{
			Name:   "Out",
			Fields: []*Field{NewField("a", FieldConfig{Type: String})},
		})
		query := NewObject(ObjectConfig{
			Name: "Query",
			Fields: []*Field{NewField("bad", FieldConfig{
				Type: String,
				Args: []*Argument{NewArgument("arg", ArgumentConfig{Type: out})},
			})},
		})
		wantMessage(t, New(Config{Query: query}), "must be Input Type")
	})

	// A name written twice is one field by the time the type holds it, so the
	// schema check has nothing to say about it. graphql-js is the same, and
	// what reports a document that writes one twice is the SDL rule
	// UniqueFieldDefinitionNames, which the schema builder runs.
	t.Run("a field name written twice is one field, not a complaint", func(t *testing.T) {
		query := NewObject(ObjectConfig{
			Name: "Query",
			Fields: []*Field{
				NewField("dup", FieldConfig{Type: String}),
				NewField("dup", FieldConfig{Type: Int}),
			},
		})
		s := New(Config{Query: query})
		wantValid(t, s)
		if got := query.Field("dup"); got == nil || got.Type != Int {
			t.Errorf("Field(\"dup\") = %v, want the last of the two", got)
		}
	})

	// A caller must supply a required argument, so deprecating it leaves them
	// nowhere to go.
	t.Run("a required argument cannot be deprecated", func(t *testing.T) {
		query := NewObject(ObjectConfig{
			Name: "Query",
			Fields: []*Field{NewField("f", FieldConfig{
				Type: String,
				Args: []*Argument{NewArgument("arg", ArgumentConfig{
					Type:              NewNonNull(String),
					DeprecationReason: DeprecatedFor("Gone."),
				})},
			})},
		})
		wantMessage(t, New(Config{Query: query}), "Required argument")
	})

	// An optional one may be, since it can simply be left out.
	t.Run("an optional argument may be deprecated", func(t *testing.T) {
		query := NewObject(ObjectConfig{
			Name: "Query",
			Fields: []*Field{NewField("f", FieldConfig{
				Type: String,
				Args: []*Argument{NewArgument("arg", ArgumentConfig{
					Type:              String,
					DeprecationReason: DeprecatedFor("Gone."),
				})},
			})},
		})
		wantValid(t, New(Config{Query: query}))
	})
}

func TestValidateSchema_Interfaces(t *testing.T) {
	idField := func() *Field { return NewField("id", FieldConfig{Type: NewNonNull(ID)}) }

	t.Run("an interface cannot implement itself", func(t *testing.T) {
		var self *InterfaceType
		self = NewInterface(InterfaceConfig{
			Name:            "Self",
			Fields:          []*Field{idField()},
			InterfacesThunk: func() []Declared[*InterfaceType] { return Implements(self) },
		})
		query := NewObject(ObjectConfig{
			Name:   "Query",
			Fields: []*Field{NewField("s", FieldConfig{Type: self})},
		})
		wantMessage(t, New(Config{Query: query}), "cannot implement itself")
	})

	t.Run("an interface is implemented once", func(t *testing.T) {
		node := NewInterface(InterfaceConfig{Name: "Node", Fields: []*Field{idField()}})
		user := NewObject(ObjectConfig{
			Name:       "User",
			Interfaces: Implements(node, node),
			Fields:     []*Field{idField()},
		})
		query := NewObject(ObjectConfig{
			Name:   "Query",
			Fields: []*Field{NewField("u", FieldConfig{Type: user})},
		})
		wantMessage(t, New(Config{Query: query}), "can only implement Node once")
	})

	t.Run("an implementation must provide the promised field", func(t *testing.T) {
		node := NewInterface(InterfaceConfig{Name: "Node", Fields: []*Field{idField()}})
		user := NewObject(ObjectConfig{
			Name:       "User",
			Interfaces: Implements(node),
			Fields:     []*Field{NewField("name", FieldConfig{Type: String})},
		})
		query := NewObject(ObjectConfig{
			Name:   "Query",
			Fields: []*Field{NewField("u", FieldConfig{Type: user})},
		})
		wantMessage(t, New(Config{Query: query}), "does not provide it")
	})

	t.Run("a field may narrow the promised type but not widen it", func(t *testing.T) {
		node := NewInterface(InterfaceConfig{
			Name:   "Node",
			Fields: []*Field{NewField("id", FieldConfig{Type: ID})},
		})
		// Narrowing a nullable to a non-null is allowed.
		narrower := NewObject(ObjectConfig{
			Name:       "Narrower",
			Interfaces: Implements(node),
			Fields:     []*Field{NewField("id", FieldConfig{Type: NewNonNull(ID)})},
		})
		query := NewObject(ObjectConfig{
			Name:   "Query",
			Fields: []*Field{NewField("n", FieldConfig{Type: narrower})},
		})
		wantValid(t, New(Config{Query: query}))

		// Changing it to something unrelated is not.
		wrong := NewObject(ObjectConfig{
			Name:       "Wrong",
			Interfaces: Implements(node),
			Fields:     []*Field{NewField("id", FieldConfig{Type: Int})},
		})
		query2 := NewObject(ObjectConfig{
			Name:   "Query",
			Fields: []*Field{NewField("w", FieldConfig{Type: wrong})},
		})
		wantMessage(t, New(Config{Query: query2}), "expects type ID but")
	})

	t.Run("an extra argument must be optional", func(t *testing.T) {
		node := NewInterface(InterfaceConfig{Name: "Node", Fields: []*Field{idField()}})
		user := NewObject(ObjectConfig{
			Name:       "User",
			Interfaces: Implements(node),
			Fields: []*Field{NewField("id", FieldConfig{
				Type: NewNonNull(ID),
				Args: []*Argument{NewArgument("extra", ArgumentConfig{Type: NewNonNull(String)})},
			})},
		})
		query := NewObject(ObjectConfig{
			Name:   "Query",
			Fields: []*Field{NewField("u", FieldConfig{Type: user})},
		})
		wantMessage(t, New(Config{Query: query}), "must not be required type")
	})

	t.Run("an implementation must not deprecate what the interface recommends", func(t *testing.T) {
		node := NewInterface(InterfaceConfig{Name: "Node", Fields: []*Field{idField()}})
		user := NewObject(ObjectConfig{
			Name:       "User",
			Interfaces: Implements(node),
			Fields: []*Field{NewField("id", FieldConfig{
				Type:              NewNonNull(ID),
				DeprecationReason: DeprecatedFor("Gone."),
			})},
		})
		query := NewObject(ObjectConfig{
			Name:   "Query",
			Fields: []*Field{NewField("u", FieldConfig{Type: user})},
		})
		wantMessage(t, New(Config{Query: query}), "must not be deprecated")
	})

	// Implementing an interface means implementing what it implements too.
	t.Run("an ancestor must be implemented as well", func(t *testing.T) {
		node := NewInterface(InterfaceConfig{Name: "Node", Fields: []*Field{idField()}})
		named := NewInterface(InterfaceConfig{
			Name:       "Named",
			Interfaces: Implements(node),
			Fields:     []*Field{idField()},
		})
		user := NewObject(ObjectConfig{
			Name:       "User",
			Interfaces: Implements(named),
			Fields:     []*Field{idField()},
		})
		query := NewObject(ObjectConfig{
			Name:   "Query",
			Fields: []*Field{NewField("u", FieldConfig{Type: user})},
		})
		wantMessage(t, New(Config{Query: query}), "must implement Node because it is implemented by Named")

		// Declaring both is sound.
		complete := NewObject(ObjectConfig{
			Name:       "Complete",
			Interfaces: Implements(named, node),
			Fields:     []*Field{idField()},
		})
		query2 := NewObject(ObjectConfig{
			Name:   "Query",
			Fields: []*Field{NewField("c", FieldConfig{Type: complete})},
		})
		wantValid(t, New(Config{Query: query2}))
	})
}

func TestValidateSchema_UnionsAndEnums(t *testing.T) {
	t.Run("a union needs a member", func(t *testing.T) {
		empty := NewUnion(UnionConfig{Name: "Empty"})
		query := NewObject(ObjectConfig{
			Name:   "Query",
			Fields: []*Field{NewField("e", FieldConfig{Type: empty})},
		})
		wantMessage(t, New(Config{Query: query}), "must define one or more member types")
	})

	t.Run("a member appears once", func(t *testing.T) {
		photo := NewObject(ObjectConfig{
			Name:   "Photo",
			Fields: []*Field{NewField("a", FieldConfig{Type: String})},
		})
		media := NewUnion(UnionConfig{Name: "Media", Types: Members(photo, photo)})
		query := NewObject(ObjectConfig{
			Name:   "Query",
			Fields: []*Field{NewField("m", FieldConfig{Type: media})},
		})
		wantMessage(t, New(Config{Query: query}), "can only include type Photo once")
	})

	t.Run("an enum needs a value", func(t *testing.T) {
		empty := NewEnum(EnumConfig{Name: "Empty"})
		query := NewObject(ObjectConfig{
			Name:   "Query",
			Fields: []*Field{NewField("e", FieldConfig{Type: empty})},
		})
		wantMessage(t, New(Config{Query: query}), "must define one or more values")
	})

	t.Run("an enum value cannot be named after a literal", func(t *testing.T) {
		bad := NewEnum(EnumConfig{
			Name:   "Bad",
			Values: []*EnumValue{NewEnumValue("true", EnumValueConfig{})},
		})
		query := NewObject(ObjectConfig{
			Name:   "Query",
			Fields: []*Field{NewField("b", FieldConfig{Type: bad})},
		})
		wantMessage(t, New(Config{Query: query}), "cannot be named")
	})
}

func TestValidateSchema_InputObjects(t *testing.T) {
	t.Run("an input object needs a field", func(t *testing.T) {
		empty := NewInputObject(InputObjectConfig{Name: "Empty"})
		query := NewObject(ObjectConfig{
			Name: "Query",
			Fields: []*Field{NewField("f", FieldConfig{
				Type: String,
				Args: []*Argument{NewArgument("in", ArgumentConfig{Type: empty})},
			})},
		})
		wantMessage(t, New(Config{Query: query}), "must define one or more fields")
	})

	t.Run("a field type must be an input type", func(t *testing.T) {
		out := NewObject(ObjectConfig{
			Name:   "Out",
			Fields: []*Field{NewField("a", FieldConfig{Type: String})},
		})
		in := NewInputObject(InputObjectConfig{
			Name:   "In",
			Fields: []*InputField{NewInputField("bad", InputFieldConfig{Type: out})},
		})
		query := NewObject(ObjectConfig{
			Name: "Query",
			Fields: []*Field{NewField("f", FieldConfig{
				Type: String,
				Args: []*Argument{NewArgument("in", ArgumentConfig{Type: in})},
			})},
		})
		wantMessage(t, New(Config{Query: query}), "must be Input Type")
	})

	// Exactly one field of a oneOf input object is supplied, so a field that
	// must be given, or one that is given for you, contradicts that.
	t.Run("a oneOf field must be nullable and have no default", func(t *testing.T) {
		in := NewInputObject(InputObjectConfig{
			Name:    "OneOf",
			IsOneOf: true,
			Fields: []*InputField{
				NewInputField("a", InputFieldConfig{Type: NewNonNull(String)}),
				NewInputField("b", InputFieldConfig{Type: String, Default: DefaultValue("x")}),
			},
		})
		query := NewObject(ObjectConfig{
			Name: "Query",
			Fields: []*Field{NewField("f", FieldConfig{
				Type: String,
				Args: []*Argument{NewArgument("in", ArgumentConfig{Type: in})},
			})},
		})
		wantMessage(t, New(Config{Query: query}), "must be nullable")
		wantMessage(t, New(Config{Query: query}), "cannot have a default value")
	})

	// A type that contains itself through fields that cannot be left out can
	// never be given a value.
	t.Run("a cycle through non-null fields is impossible", func(t *testing.T) {
		var in *InputObjectType
		in = NewInputObject(InputObjectConfig{
			Name: "Loop",
			FieldsThunk: func() []*InputField {
				return []*InputField{NewInputField("self", InputFieldConfig{Type: NewNonNull(in)})}
			},
		})
		query := NewObject(ObjectConfig{
			Name: "Query",
			Fields: []*Field{NewField("f", FieldConfig{
				Type: String,
				Args: []*Argument{NewArgument("in", ArgumentConfig{Type: in})},
			})},
		})
		wantMessage(t, New(Config{Query: query}), "cannot be provided a finite value because it references itself through fields")
	})

	// The same shape is fine when the cycle can be ended, by writing null.
	t.Run("a cycle through a nullable field is fine", func(t *testing.T) {
		var in *InputObjectType
		in = NewInputObject(InputObjectConfig{
			Name: "Loop",
			FieldsThunk: func() []*InputField {
				return []*InputField{NewInputField("self", InputFieldConfig{Type: in})}
			},
		})
		query := NewObject(ObjectConfig{
			Name: "Query",
			Fields: []*Field{NewField("f", FieldConfig{
				Type: String,
				Args: []*Argument{NewArgument("in", ArgumentConfig{Type: in})},
			})},
		})
		wantValid(t, New(Config{Query: query}))
	})
}

// Problems noticed while a schema was being assembled surface here rather than
// at the point they were found, which is what lets building never fail.
func TestValidateSchema_ReportsCollectedProblems(t *testing.T) {
	first := NewObject(ObjectConfig{
		Name:   "User",
		Fields: []*Field{NewField("a", FieldConfig{Type: String})},
	})
	second := NewObject(ObjectConfig{
		Name:   "User",
		Fields: []*Field{NewField("b", FieldConfig{Type: String})},
	})
	query := NewObject(ObjectConfig{
		Name: "Query",
		Fields: []*Field{
			NewField("first", FieldConfig{Type: first}),
			NewField("second", FieldConfig{Type: second}),
		},
	})
	wantMessage(t, New(Config{Query: query}), "more than one type named")
}

func TestValidateSchema_Directives(t *testing.T) {
	query := simpleQuery()

	t.Run("a directive needs a location", func(t *testing.T) {
		nowhere := NewDirective(DirectiveConfig{Name: "nowhere"})
		s := New(Config{Query: query, Directives: []*Directive{nowhere}})
		wantMessage(t, s, "must include 1 or more locations")
	})

	t.Run("an argument type must be an input type", func(t *testing.T) {
		out := NewObject(ObjectConfig{
			Name:   "Out",
			Fields: []*Field{NewField("a", FieldConfig{Type: String})},
		})
		bad := NewDirective(DirectiveConfig{
			Name:      "bad",
			Locations: []language.DirectiveLocation{language.DirectiveLocationField},
			Args:      []*Argument{NewArgument("arg", ArgumentConfig{Type: out})},
		})
		s := New(Config{Query: query, Directives: []*Directive{bad}})
		wantMessage(t, s, "must be Input Type")
	})
}

func TestAssertValidSchema(t *testing.T) {
	if err := AssertValidSchema(nil); err == nil {
		t.Error("AssertValidSchema(nil) = nil, want an error")
	}
	if errs := ValidateSchema(nil); len(errs) != 1 {
		t.Errorf("ValidateSchema(nil) gave %d complaints, want 1", len(errs))
	}

	err := AssertValidSchema(New(Config{}))
	if err == nil {
		t.Fatal("a schema with no query root was reported as sound")
	}
	if !strings.Contains(err.Error(), "Query root type must be provided") {
		t.Errorf("error = %v, want it to name the problem", err)
	}
}

func TestIsTypeSubTypeOf(t *testing.T) {
	node := NewInterface(InterfaceConfig{
		Name:   "Node",
		Fields: []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	user := NewObject(ObjectConfig{
		Name:       "User",
		Interfaces: Implements(node),
		Fields:     []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	other := NewObject(ObjectConfig{
		Name:   "Other",
		Fields: []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	query := NewObject(ObjectConfig{
		Name:   "Query",
		Fields: []*Field{NewField("n", FieldConfig{Type: node})},
	})
	s := New(Config{Query: query, Types: []NamedType{user, other}})

	tests := []struct {
		name       string
		sub, super Type
		want       bool
	}{
		{"the same type", String, String, true},
		{"a non-null where a nullable is wanted", NewNonNull(String), String, true},
		{"a nullable where a non-null is wanted", String, NewNonNull(String), false},
		{"a list of the same", NewList(String), NewList(String), true},
		{"a list of a non-null", NewList(NewNonNull(String)), NewList(String), true},
		{"a plain value where a list is wanted", String, NewList(String), false},
		{"a list where a plain value is wanted", NewList(String), String, false},
		{"an implementation where an interface is wanted", user, node, true},
		{"an unrelated type", other, node, false},
		{"a list of an implementation", NewList(user), NewList(node), true},
		{"nothing", nil, String, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTypeSubTypeOf(s, tt.sub, tt.super); got != tt.want {
				t.Errorf("IsTypeSubTypeOf(%v, %v) = %v, want %v", tt.sub, tt.super, got, tt.want)
			}
		})
	}
}

func TestDoTypesOverlap(t *testing.T) {
	node := NewInterface(InterfaceConfig{
		Name:   "Node",
		Fields: []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	named := NewInterface(InterfaceConfig{
		Name:   "Named",
		Fields: []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	user := NewObject(ObjectConfig{
		Name:       "User",
		Interfaces: Implements(node, named),
		Fields:     []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	lonely := NewObject(ObjectConfig{
		Name:   "Lonely",
		Fields: []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	media := NewUnion(UnionConfig{Name: "Media", Types: Members(user)})
	query := NewObject(ObjectConfig{
		Name:   "Query",
		Fields: []*Field{NewField("n", FieldConfig{Type: node})},
	})
	s := New(Config{Query: query, Types: []NamedType{user, lonely, named, media}})

	tests := []struct {
		name string
		a, b CompositeType
		want bool
	}{
		{"the same type", user, user, true},
		{"two interfaces sharing an implementation", node, named, true},
		{"an interface and one of its implementations", node, user, true},
		{"an interface and an unrelated object", node, lonely, false},
		{"a union and one of its members", media, user, true},
		{"a union and a type outside it", media, lonely, false},
		{"two different object types", user, lonely, false},
		{"nothing", nil, user, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DoTypesOverlap(s, tt.a, tt.b); got != tt.want {
				t.Errorf("DoTypesOverlap(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// Validation runs over whatever it is given, including a schema with holes in
// it, and reports them rather than crashing. That is the point of building
// never failing: the checker has to cope with anything.
func TestValidateSchema_ReportsMissingPieces(t *testing.T) {
	query := NewObject(ObjectConfig{
		Name:   "Query",
		Fields: []*Field{nil, NewField("ok", FieldConfig{Type: String})},
	})
	iface := NewInterface(InterfaceConfig{
		Name:   "Node",
		Fields: []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	holed := NewObject(ObjectConfig{
		Name:       "Holed",
		Interfaces: Implements(nil, iface),
		Fields:     []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	union := NewUnion(UnionConfig{Name: "Media", Types: Members(nil, holed)})
	enum := NewEnum(EnumConfig{Name: "Colour", Values: []*EnumValue{nil, NewEnumValue("RED", EnumValueConfig{})}})
	input := NewInputObject(InputObjectConfig{
		Name:   "Filter",
		Fields: []*InputField{nil, NewInputField("term", InputFieldConfig{Type: String})},
	})
	full := NewObject(ObjectConfig{
		Name: "Query",
		Fields: []*Field{
			NewField("m", FieldConfig{
				Type: union,
				Args: []*Argument{NewArgument("in", ArgumentConfig{Type: input})},
			}),
			NewField("c", FieldConfig{Type: enum}),
		},
	})

	for _, s := range []*Schema{
		New(Config{Query: query}),
		New(Config{Query: full, Directives: []*Directive{nil, Skip}}),
	} {
		messages := messagesFor(s)
		if len(messages) == 0 {
			t.Error("a schema with missing pieces was reported as sound")
		}
		for _, m := range messages {
			if strings.Contains(m, "%!") {
				t.Errorf("a message was formatted wrongly: %q", m)
			}
		}
	}
}

// An argument declared twice on the same field is one argument, the last of
// them, as it is in graphql-js. A document that writes one twice is reported
// by the SDL rule UniqueArgumentDefinitionNames before anything is built.
func TestValidateSchema_DuplicateArgument(t *testing.T) {
	field := NewField("f", FieldConfig{
		Type: String,
		Args: []*Argument{
			NewArgument("dup", ArgumentConfig{Type: String}),
			NewArgument("other", ArgumentConfig{Type: String}),
			NewArgument("dup", ArgumentConfig{Type: Int}),
		},
	})
	query := NewObject(ObjectConfig{Name: "Query", Fields: []*Field{field}})
	s := New(Config{Query: query})
	wantValid(t, s)

	held := query.Field("f")
	var named []string
	for _, a := range held.Args {
		named = append(named, a.Name())
	}
	if want := []string{"dup", "other"}; !slices.Equal(named, want) {
		t.Errorf("Args = %v, want %v", named, want)
	}
	if got := held.Arg("dup"); got == nil || got.Type != Int {
		t.Errorf("Arg(\"dup\") = %v, want the last of the two", got)
	}
}

// A directive whose name is reserved is caught the same as a type's.
func TestValidateSchema_ReservedDirectiveName(t *testing.T) {
	reserved := NewDirective(DirectiveConfig{
		Name:      "__secret",
		Locations: []language.DirectiveLocation{language.DirectiveLocationField},
	})
	s := New(Config{Query: simpleQuery(), Directives: []*Directive{reserved}})
	wantMessage(t, s, `Name "__secret" must not begin with`)
}

// A default has to be a value the type would accept, whether it was written in
// SDL as a literal or given in Go as a value.
func TestValidateSchema_DefaultValues(t *testing.T) {
	// A default given in Go is checked the same way, which is the half of the
	// checking the ported cases never reach: they are all SDL.
	t.Run("a value that does not fit", func(t *testing.T) {
		query := NewObject(ObjectConfig{
			Name: "Query",
			Fields: []*Field{NewField("f", FieldConfig{
				Type: String,
				Args: []*Argument{NewArgument("n", ArgumentConfig{
					Type:    Int,
					Default: value.Just(DefaultInput{Value: "not a number"}),
				})},
			})},
		})
		wantMessage(t, New(Config{Query: query}),
			`Query.f(n:) has invalid default value: Int cannot represent non-integer value: "not a number"`)
	})

	// Where the trouble is inside the value, the message says where.
	//
	// The item is one no type could have produced, so that the complaint is
	// the one about the item rather than the suggested fix a value merely
	// written in the internal form gets; see
	// TestUncoerceDefaultValue_Corners for that half.
	t.Run("a value that does not fit deep inside", func(t *testing.T) {
		in := NewInputObject(InputObjectConfig{
			Name: "Filter",
			Fields: []*InputField{
				NewInputField("tags", InputFieldConfig{Type: NewList(String)}),
			},
		})
		query := NewObject(ObjectConfig{
			Name: "Query",
			Fields: []*Field{NewField("f", FieldConfig{
				Type: String,
				Args: []*Argument{NewArgument("by", ArgumentConfig{
					Type: in,
					Default: value.Just(DefaultInput{
						Value: map[string]any{"tags": []any{"ok", []any{"nested"}}},
					}),
				})},
			})},
		})
		wantMessage(t, New(Config{Query: query}), "has invalid default value at .tags[1]:")
	})

	// A default that fits is not reported.
	t.Run("a value that fits", func(t *testing.T) {
		query := NewObject(ObjectConfig{
			Name: "Query",
			Fields: []*Field{NewField("f", FieldConfig{
				Type: String,
				Args: []*Argument{NewArgument("n", ArgumentConfig{
					Type:    Int,
					Default: value.Just(DefaultInput{Value: 3}),
				})},
			})},
		})
		if errs := ValidateSchema(New(Config{Query: query})); len(errs) != 0 {
			t.Errorf("a sound default was reported: %v", errs)
		}
	})
}

// A default that fills its gaps from defaults which fill their gaps from it
// describes no value at all.
func TestValidateSchema_DefaultValueCycles(t *testing.T) {
	var in *InputObjectType
	in = NewInputObject(InputObjectConfig{
		Name: "Loop",
		FieldsThunk: func() []*InputField {
			return []*InputField{NewInputField("self", InputFieldConfig{
				Type:    in,
				Default: value.Just(DefaultInput{Value: map[string]any{}}),
			})}
		},
	})
	query := NewObject(ObjectConfig{
		Name: "Query",
		Fields: []*Field{NewField("f", FieldConfig{
			Type: String,
			Args: []*Argument{NewArgument("in", ArgumentConfig{Type: in})},
		})},
	})
	wantMessage(t, New(Config{Query: query}),
		"The default value of Input Object field Loop.self references itself")

	// The same shape is fine when the default fills the gap with something
	// that ends: null is a value, and it holds no fields to fill in turn.
	var ended *InputObjectType
	ended = NewInputObject(InputObjectConfig{
		Name: "Ended",
		FieldsThunk: func() []*InputField {
			return []*InputField{NewInputField("self", InputFieldConfig{
				Type:    ended,
				Default: value.Just(DefaultInput{Value: nil}),
			})}
		},
	})
	sound := NewObject(ObjectConfig{
		Name: "Query",
		Fields: []*Field{NewField("f", FieldConfig{
			Type: String,
			Args: []*Argument{NewArgument("in", ArgumentConfig{Type: ended})},
		})},
	})
	if errs := ValidateSchema(New(Config{Query: sound})); len(errs) != 0 {
		t.Errorf("a default that ends was reported: %v", errs)
	}
}
