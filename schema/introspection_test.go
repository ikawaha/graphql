package schema

import (
	"context"
	"github.com/ikawaha/graphql/value"
	"slices"
	"testing"

	"github.com/ikawaha/graphql/language"
)

// resolveOn calls a field's resolver directly. There is no executor yet, so
// this is how the introspection resolvers are exercised.
func resolveOn(t *testing.T, owner *ObjectType, field string, source any, args map[string]any, info *ResolveInfo) any {
	t.Helper()
	f := owner.Field(field)
	if f == nil {
		t.Fatalf("%s has no field %q", owner.Name(), field)
	}
	if f.Resolve == nil {
		t.Fatalf("%s.%s has no resolver", owner.Name(), field)
	}
	got, err := f.Resolve(context.Background(), source, NewArguments(args), info)
	if err != nil {
		t.Fatalf("%s.%s: %v", owner.Name(), field, err)
	}
	return got
}

func TestKindOf(t *testing.T) {
	scalar := testScalar("Int")
	tests := []struct {
		typ  Type
		want TypeKindName
	}{
		{scalar, TypeKindScalar},
		{NewObject(ObjectConfig{Name: "User"}), TypeKindObject},
		{NewInterface(InterfaceConfig{Name: "Node"}), TypeKindInterface},
		{NewUnion(UnionConfig{Name: "Media"}), TypeKindUnion},
		{NewEnum(EnumConfig{Name: "Colour"}), TypeKindEnum},
		{NewInputObject(InputObjectConfig{Name: "Filter"}), TypeKindInputObject},
		{NewList(scalar), TypeKindList},
		{NewNonNull(scalar), TypeKindNonNull},
	}
	for _, tt := range tests {
		t.Run(string(tt.want), func(t *testing.T) {
			if got := KindOf(tt.typ); got != tt.want {
				t.Errorf("KindOf(%s) = %q, want %q", tt.typ, got, tt.want)
			}
		})
	}
	if got := KindOf(nil); got != "" {
		t.Errorf("KindOf(nil) = %q, want empty", got)
	}
}

func TestIntrospection_Enums(t *testing.T) {
	if got := len(TypeKindType.Values()); got != 8 {
		t.Errorf("__TypeKind has %d values, want 8", got)
	}
	for _, kind := range []TypeKindName{
		TypeKindScalar, TypeKindObject, TypeKindInterface, TypeKindUnion,
		TypeKindEnum, TypeKindInputObject, TypeKindList, TypeKindNonNull,
	} {
		member := TypeKindType.Value(string(kind))
		if member == nil {
			t.Errorf("__TypeKind is missing %s", kind)
			continue
		}
		if member.Value != kind {
			t.Errorf("%s internal value = %v, want the kind itself", kind, member.Value)
		}
	}

	// Every location the language recognises has to be namable.
	if got := len(DirectiveLocationType.Values()); got != 21 {
		t.Errorf("__DirectiveLocation has %d values, want 21", got)
	}
	for _, loc := range []language.DirectiveLocation{
		language.DirectiveLocationQuery,
		language.DirectiveLocationInputFieldDefinition,
	} {
		if DirectiveLocationType.Value(string(loc)) == nil {
			t.Errorf("__DirectiveLocation is missing %s", loc)
		}
	}
}

// testIntrospectionSchema builds a schema with one of everything worth asking
// about.
func testIntrospectionSchema(t *testing.T) (*Schema, *ObjectType, *InterfaceType, *EnumType, *InputObjectType) {
	t.Helper()

	colour := NewEnum(EnumConfig{
		Name:        "Colour",
		Description: value.Just("A colour."),
		Values: []*EnumValue{
			NewEnumValue("RED", EnumValueConfig{Description: value.Just("Red.")}),
			NewEnumValue("PUCE", EnumValueConfig{DeprecationReason: DeprecatedFor("Out of fashion.")}),
		},
	})
	filter := NewInputObject(InputObjectConfig{
		Name:    "Filter",
		IsOneOf: true,
		Fields: []*InputField{
			NewInputField("term", InputFieldConfig{
				Type:    String,
				Default: DefaultLiteral(&language.StringValue{Value: "all"}),
			}),
			NewInputField("legacy", InputFieldConfig{Type: String, DeprecationReason: DeprecatedFor("Gone.")}),
		},
	})
	node := NewInterface(InterfaceConfig{
		Name:   "Node",
		Fields: []*Field{NewField("id", FieldConfig{Type: NewNonNull(ID)})},
	})
	user := NewObject(ObjectConfig{
		Name:        "User",
		Description: value.Just("A person."),
		Interfaces:  Implements(node),
		Fields: []*Field{
			NewField("id", FieldConfig{Type: NewNonNull(ID)}),
			NewField("name", FieldConfig{
				Type: String,
				Args: []*Argument{
					NewArgument("upper", ArgumentConfig{Type: Boolean}),
					NewArgument("legacy", ArgumentConfig{Type: Boolean, DeprecationReason: DeprecatedFor("Gone.")}),
				},
			}),
			NewField("old", FieldConfig{Type: String, DeprecationReason: DeprecatedFor("Use name.")}),
		},
	})
	query := NewObject(ObjectConfig{
		Name: "Query",
		Fields: []*Field{
			NewField("me", FieldConfig{Type: user}),
			NewField("node", FieldConfig{Type: node}),
			NewField("colour", FieldConfig{Type: colour}),
			NewField("search", FieldConfig{
				Type: String,
				Args: []*Argument{NewArgument("filter", ArgumentConfig{Type: filter})},
			}),
		},
	})
	return New(Config{Query: query, Description: value.Just("A schema.")}), user, node, colour, filter
}

func TestIntrospection_Schema(t *testing.T) {
	s, _, _, _, _ := testIntrospectionSchema(t)

	if got := resolveOn(t, SchemaIntrospectionType, "description", s, nil, nil); got != "A schema." {
		t.Errorf("description = %v", got)
	}
	if got := resolveOn(t, SchemaIntrospectionType, "queryType", s, nil, nil); got != Type(s.QueryType()) {
		t.Errorf("queryType = %v, want the query root", got)
	}
	// A schema without a mutation reports null rather than a typed nil.
	if got := resolveOn(t, SchemaIntrospectionType, "mutationType", s, nil, nil); got != nil {
		t.Errorf("mutationType = %v, want nil", got)
	}
	if got := resolveOn(t, SchemaIntrospectionType, "subscriptionType", s, nil, nil); got != nil {
		t.Errorf("subscriptionType = %v, want nil", got)
	}

	types, ok := resolveOn(t, SchemaIntrospectionType, "types", s, nil, nil).([]any)
	if !ok || len(types) != len(s.Types()) {
		t.Errorf("types returned %d entries, want %d", len(types), len(s.Types()))
	}
	directives, ok := resolveOn(t, SchemaIntrospectionType, "directives", s, nil, nil).([]any)
	if !ok || len(directives) != len(s.Directives()) {
		t.Errorf("directives returned %d entries, want %d", len(directives), len(s.Directives()))
	}

	// A schema with no description reports null, not an empty string.
	bare := New(Config{Query: s.QueryType()})
	if got := resolveOn(t, SchemaIntrospectionType, "description", bare, nil, nil); got != nil {
		t.Errorf("description of a schema without one = %v, want nil", got)
	}
}

func TestIntrospection_Type(t *testing.T) {
	s, user, node, colour, filter := testIntrospectionSchema(t)
	info := &ResolveInfo{Schema: s}

	t.Run("an object", func(t *testing.T) {
		if got := resolveOn(t, TypeIntrospectionType, "kind", user, nil, info); got != TypeKindObject {
			t.Errorf("kind = %v, want OBJECT", got)
		}
		if got := resolveOn(t, TypeIntrospectionType, "name", user, nil, info); got != "User" {
			t.Errorf("name = %v", got)
		}
		if got := resolveOn(t, TypeIntrospectionType, "description", user, nil, info); got != "A person." {
			t.Errorf("description = %v", got)
		}
		interfaces, _ := resolveOn(t, TypeIntrospectionType, "interfaces", user, nil, info).([]any)
		if len(interfaces) != 1 || interfaces[0] != any(node) {
			t.Errorf("interfaces = %v, want [Node]", interfaces)
		}
		// Fields that apply to other kinds report null.
		for _, field := range []string{"enumValues", "inputFields", "ofType", "specifiedByURL", "isOneOf"} {
			if got := resolveOn(t, TypeIntrospectionType, field, user, nil, info); got != nil {
				t.Errorf("%s of an object = %v, want nil", field, got)
			}
		}
	})

	t.Run("a wrapper", func(t *testing.T) {
		wrapped := NewNonNull(NewList(user))
		if got := resolveOn(t, TypeIntrospectionType, "kind", wrapped, nil, info); got != TypeKindNonNull {
			t.Errorf("kind = %v, want NON_NULL", got)
		}
		// A wrapper has no name of its own.
		if got := resolveOn(t, TypeIntrospectionType, "name", wrapped, nil, info); got != nil {
			t.Errorf("name of a wrapper = %v, want nil", got)
		}
		inner := resolveOn(t, TypeIntrospectionType, "ofType", wrapped, nil, info)
		if inner != Type(wrapped.OfType) {
			t.Errorf("ofType = %v, want the list inside", inner)
		}
		if got := resolveOn(t, TypeIntrospectionType, "fields", wrapped, nil, info); got != nil {
			t.Errorf("fields of a wrapper = %v, want nil", got)
		}
	})

	t.Run("an enum", func(t *testing.T) {
		if got := resolveOn(t, TypeIntrospectionType, "kind", colour, nil, info); got != TypeKindEnum {
			t.Errorf("kind = %v, want ENUM", got)
		}
		values, _ := resolveOn(t, TypeIntrospectionType, "enumValues", colour, nil, info).([]any)
		if len(values) != 1 {
			t.Errorf("enumValues = %v, want only the one that is current", values)
		}
	})

	t.Run("an input object", func(t *testing.T) {
		if got := resolveOn(t, TypeIntrospectionType, "kind", filter, nil, info); got != TypeKindInputObject {
			t.Errorf("kind = %v, want INPUT_OBJECT", got)
		}
		if got := resolveOn(t, TypeIntrospectionType, "isOneOf", filter, nil, info); got != true {
			t.Errorf("isOneOf = %v, want true", got)
		}
		fields, _ := resolveOn(t, TypeIntrospectionType, "inputFields", filter, nil, info).([]any)
		if len(fields) != 1 {
			t.Errorf("inputFields = %v, want only the one that is current", fields)
		}
	})

	t.Run("an interface knows what could implement it", func(t *testing.T) {
		possible, _ := resolveOn(t, TypeIntrospectionType, "possibleTypes", node, nil, info).([]any)
		if len(possible) != 1 || possible[0] != any(user) {
			t.Errorf("possibleTypes = %v, want [User]", possible)
		}
		// Without a schema to consult there is nothing to say.
		if got := resolveOn(t, TypeIntrospectionType, "possibleTypes", node, nil, nil); got != nil {
			t.Errorf("possibleTypes without a schema = %v, want nil", got)
		}
		if got := resolveOn(t, TypeIntrospectionType, "possibleTypes", user, nil, info); got != nil {
			t.Errorf("possibleTypes of an object = %v, want nil", got)
		}
	})

	t.Run("a scalar carries its specification", func(t *testing.T) {
		custom := NewScalar(ScalarConfig{Name: "DateTime", SpecifiedByURL: "https://example.com"})
		if got := resolveOn(t, TypeIntrospectionType, "specifiedByURL", custom, nil, info); got != "https://example.com" {
			t.Errorf("specifiedByURL = %v", got)
		}
		if got := resolveOn(t, TypeIntrospectionType, "specifiedByURL", Int, nil, info); got != nil {
			t.Errorf("specifiedByURL of a scalar without one = %v, want nil", got)
		}
	})
}

// Deprecated members are hidden unless asked for, which is what the argument's
// default of false means.
func TestIntrospection_IncludeDeprecated(t *testing.T) {
	s, user, _, colour, filter := testIntrospectionSchema(t)
	info := &ResolveInfo{Schema: s}
	withDeprecated := map[string]any{"includeDeprecated": true}

	count := func(source any, field string, args map[string]any) int {
		t.Helper()
		got, _ := resolveOn(t, TypeIntrospectionType, field, source, args, info).([]any)
		return len(got)
	}

	if got, want := count(user, "fields", nil), 2; got != want {
		t.Errorf("fields without deprecated = %d, want %d", got, want)
	}
	if got, want := count(user, "fields", withDeprecated), 3; got != want {
		t.Errorf("fields with deprecated = %d, want %d", got, want)
	}
	if got, want := count(colour, "enumValues", withDeprecated), 2; got != want {
		t.Errorf("enumValues with deprecated = %d, want %d", got, want)
	}
	if got, want := count(filter, "inputFields", withDeprecated), 2; got != want {
		t.Errorf("inputFields with deprecated = %d, want %d", got, want)
	}

	name := user.Field("name")
	args, _ := resolveOn(t, FieldIntrospectionType, "args", name, nil, info).([]any)
	if len(args) != 1 {
		t.Errorf("args without deprecated = %d, want 1", len(args))
	}
	args, _ = resolveOn(t, FieldIntrospectionType, "args", name, withDeprecated, info).([]any)
	if len(args) != 2 {
		t.Errorf("args with deprecated = %d, want 2", len(args))
	}
}

func TestIntrospection_Field(t *testing.T) {
	s, user, _, _, _ := testIntrospectionSchema(t)
	info := &ResolveInfo{Schema: s}

	name := user.Field("name")
	if got := resolveOn(t, FieldIntrospectionType, "name", name, nil, info); got != "name" {
		t.Errorf("name = %v", got)
	}
	if got := resolveOn(t, FieldIntrospectionType, "type", name, nil, info); got != Type(String) {
		t.Errorf("type = %v, want String", got)
	}
	if got := resolveOn(t, FieldIntrospectionType, "isDeprecated", name, nil, info); got != false {
		t.Errorf("isDeprecated = %v, want false", got)
	}
	if got := resolveOn(t, FieldIntrospectionType, "deprecationReason", name, nil, info); got != nil {
		t.Errorf("deprecationReason = %v, want nil", got)
	}
	if got := resolveOn(t, FieldIntrospectionType, "description", name, nil, info); got != nil {
		t.Errorf("description of a field without one = %v, want nil", got)
	}

	old := user.Field("old")
	if got := resolveOn(t, FieldIntrospectionType, "isDeprecated", old, nil, info); got != true {
		t.Errorf("isDeprecated = %v, want true", got)
	}
	if got := resolveOn(t, FieldIntrospectionType, "deprecationReason", old, nil, info); got != "Use name." {
		t.Errorf("deprecationReason = %v", got)
	}
}

// __InputValue describes an argument and an input object field alike, since
// the two have the same shape.
func TestIntrospection_InputValue(t *testing.T) {
	s, user, _, _, filter := testIntrospectionSchema(t)
	info := &ResolveInfo{Schema: s}

	arg := user.Field("name").Arg("upper")
	if got := resolveOn(t, InputValueIntrospectionType, "name", arg, nil, info); got != "upper" {
		t.Errorf("name = %v", got)
	}
	if got := resolveOn(t, InputValueIntrospectionType, "type", arg, nil, info); got != Type(Boolean) {
		t.Errorf("type = %v, want Boolean", got)
	}
	if got := resolveOn(t, InputValueIntrospectionType, "defaultValue", arg, nil, info); got != nil {
		t.Errorf("defaultValue of an argument without one = %v, want nil", got)
	}

	// A default written in a schema is reported as it was written.
	term := filter.Field("term")
	if got := resolveOn(t, InputValueIntrospectionType, "defaultValue", term, nil, info); got != `"all"` {
		t.Errorf("defaultValue = %v, want the literal it was written as", got)
	}
	if got := resolveOn(t, InputValueIntrospectionType, "isDeprecated", term, nil, info); got != false {
		t.Errorf("isDeprecated = %v, want false", got)
	}

	legacy := filter.Field("legacy")
	if got := resolveOn(t, InputValueIntrospectionType, "isDeprecated", legacy, nil, info); got != true {
		t.Errorf("isDeprecated = %v, want true", got)
	}
	if got := resolveOn(t, InputValueIntrospectionType, "deprecationReason", legacy, nil, info); got != "Gone." {
		t.Errorf("deprecationReason = %v", got)
	}
}

func TestIntrospection_EnumValueAndDirective(t *testing.T) {
	s, _, _, colour, _ := testIntrospectionSchema(t)
	info := &ResolveInfo{Schema: s}

	red := colour.Value("RED")
	if got := resolveOn(t, EnumValueIntrospectionType, "name", red, nil, info); got != "RED" {
		t.Errorf("name = %v", got)
	}
	if got := resolveOn(t, EnumValueIntrospectionType, "description", red, nil, info); got != "Red." {
		t.Errorf("description = %v", got)
	}
	puce := colour.Value("PUCE")
	if got := resolveOn(t, EnumValueIntrospectionType, "isDeprecated", puce, nil, info); got != true {
		t.Errorf("isDeprecated = %v, want true", got)
	}

	skip := s.Directive("skip")
	if got := resolveOn(t, DirectiveIntrospectionType, "name", skip, nil, info); got != "skip" {
		t.Errorf("name = %v", got)
	}
	if got := resolveOn(t, DirectiveIntrospectionType, "isRepeatable", skip, nil, info); got != false {
		t.Errorf("isRepeatable = %v, want false", got)
	}
	locations, _ := resolveOn(t, DirectiveIntrospectionType, "locations", skip, nil, info).([]any)
	if len(locations) != 3 {
		t.Errorf("locations = %v, want three", locations)
	}
	args, _ := resolveOn(t, DirectiveIntrospectionType, "args", skip, nil, info).([]any)
	if len(args) != 1 {
		t.Errorf("args = %v, want one", args)
	}
	// A directive is never deprecated, but the fields exist so that a client
	// can ask about anything uniformly.
	if got := resolveOn(t, DirectiveIntrospectionType, "isDeprecated", skip, nil, info); got != false {
		t.Errorf("isDeprecated = %v, want false", got)
	}
	if got := resolveOn(t, DirectiveIntrospectionType, "deprecationReason", skip, nil, info); got != nil {
		t.Errorf("deprecationReason = %v, want nil", got)
	}
}

func TestIntrospection_MetaFields(t *testing.T) {
	s, user, _, _, _ := testIntrospectionSchema(t)

	if got, err := SchemaMetaField.Resolve(context.Background(), nil, Arguments{}, &ResolveInfo{Schema: s}); err != nil || got != any(s) {
		t.Errorf("__schema = %v, %v, want the schema", got, err)
	}
	if got, _ := SchemaMetaField.Resolve(context.Background(), nil, Arguments{}, nil); got != nil {
		t.Errorf("__schema without info = %v, want nil", got)
	}

	named := NewArguments(map[string]any{"name": "User"})
	if got, err := TypeMetaField.Resolve(context.Background(), nil, named, &ResolveInfo{Schema: s}); err != nil || got != any(user) {
		t.Errorf("__type(User) = %v, %v, want the type", got, err)
	}
	missing := NewArguments(map[string]any{"name": "Nope"})
	if got, _ := TypeMetaField.Resolve(context.Background(), nil, missing, &ResolveInfo{Schema: s}); got != nil {
		t.Errorf("__type of an unknown name = %v, want nil", got)
	}

	info := &ResolveInfo{Schema: s, ParentType: user}
	if got, err := TypeNameMetaField.Resolve(context.Background(), nil, Arguments{}, info); err != nil || got != "User" {
		t.Errorf("__typename = %v, %v, want User", got, err)
	}
	if got, _ := TypeNameMetaField.Resolve(context.Background(), nil, Arguments{}, nil); got != nil {
		t.Errorf("__typename without info = %v, want nil", got)
	}

	// The meta-fields are non-null where the specification says so.
	if !IsNonNullType(SchemaMetaField.Type) || !IsNonNullType(TypeNameMetaField.Type) {
		t.Error("a meta-field that must be non-null is not")
	}
	if IsNonNullType(TypeMetaField.Type) {
		t.Error("__type is non-null, but asking for an unknown type must give null")
	}
}

func TestIntrospectionTypes(t *testing.T) {
	if got := len(IntrospectionTypes); got != 8 {
		t.Fatalf("%d introspection types, want 8", got)
	}
	names := make([]string, len(IntrospectionTypes))
	for i, typ := range IntrospectionTypes {
		names[i] = typ.Name()
		if !IsIntrospectionType(typ) {
			t.Errorf("%s was not recognised as an introspection type", typ.Name())
		}
	}
	slices.Sort(names)
	want := []string{
		"__Directive", "__DirectiveLocation", "__EnumValue", "__Field",
		"__InputValue", "__Schema", "__Type", "__TypeKind",
	}
	if !slices.Equal(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}

	if IsIntrospectionType(NewObject(ObjectConfig{Name: "__NotReally"})) {
		t.Error("a type merely named like one was recognised as an introspection type")
	}
	if IsIntrospectionType(NewList(String)) {
		t.Error("a wrapper was recognised as an introspection type")
	}
}

// Every schema carries the introspection types, whether or not anything in it
// refers to them.
func TestSchema_CarriesIntrospectionTypes(t *testing.T) {
	query := NewObject(ObjectConfig{Name: "Query", Fields: []*Field{NewField("a", FieldConfig{Type: String})}})
	s := New(Config{Query: query})

	for _, typ := range IntrospectionTypes {
		if s.Type(typ.Name()) == nil {
			t.Errorf("%s is missing from the schema", typ.Name())
		}
	}
}

// A resolver handed something other than the schema element its type describes
// says so rather than returning a wrong answer.
func TestIntrospection_RejectsUnexpectedSources(t *testing.T) {
	tests := []struct {
		owner *ObjectType
		field string
	}{
		{SchemaIntrospectionType, "description"},
		{TypeIntrospectionType, "kind"},
		{FieldIntrospectionType, "name"},
		{DirectiveIntrospectionType, "name"},
		{EnumValueIntrospectionType, "name"},
		{InputValueIntrospectionType, "name"},
	}
	for _, tt := range tests {
		t.Run(tt.owner.Name()+"."+tt.field, func(t *testing.T) {
			f := tt.owner.Field(tt.field)
			if _, err := f.Resolve(context.Background(), "not a schema element", Arguments{}, nil); err == nil {
				t.Error("a resolver accepted a source it cannot describe")
			}
		})
	}

	// A type the package does not know about has no kind to report.
	f := TypeIntrospectionType.Field("kind")
	if _, err := f.Resolve(context.Background(), unknownType{}, Arguments{}, nil); err == nil {
		t.Error("kind accepted a type it does not know")
	}
}

// unknownType stands for a Type implemented outside this package.
type unknownType struct{}

func (unknownType) String() string { return "Unknown" }
func (unknownType) isType()        {}

// Every kind of named type carries a description, and a wrapper carries none
// of its own.
func TestIntrospection_DescriptionsAcrossKinds(t *testing.T) {
	const text = "Described."
	named := []NamedType{
		NewScalar(ScalarConfig{Name: "S", Description: value.Just(text)}),
		NewObject(ObjectConfig{Name: "O", Description: value.Just(text)}),
		NewInterface(InterfaceConfig{Name: "I", Description: value.Just(text)}),
		NewUnion(UnionConfig{Name: "U", Description: value.Just(text)}),
		NewEnum(EnumConfig{Name: "E", Description: value.Just(text)}),
		NewInputObject(InputObjectConfig{Name: "In", Description: value.Just(text)}),
	}
	for _, typ := range named {
		t.Run(typ.Name(), func(t *testing.T) {
			if got := resolveOn(t, TypeIntrospectionType, "description", typ, nil, nil); got != text {
				t.Errorf("description = %v, want %q", got, text)
			}
			// The same type without one reports null rather than an empty
			// string, because the field is nullable.
			bare := NewScalar(ScalarConfig{Name: "Bare"})
			if got := resolveOn(t, TypeIntrospectionType, "description", bare, nil, nil); got != nil {
				t.Errorf("description of a type without one = %v, want nil", got)
			}
		})
	}

	if got := resolveOn(t, TypeIntrospectionType, "description", NewList(String), nil, nil); got != nil {
		t.Errorf("description of a wrapper = %v, want nil", got)
	}
}

// The fields that only some kinds have report null for the rest, across every
// kind rather than only the one each was written for.
func TestIntrospection_KindSpecificFieldsReportNull(t *testing.T) {
	iface := NewInterface(InterfaceConfig{
		Name:   "Node",
		Fields: []*Field{NewField("id", FieldConfig{Type: ID})},
	})
	union := NewUnion(UnionConfig{Name: "Media"})
	enum := NewEnum(EnumConfig{Name: "Colour", Values: []*EnumValue{NewEnumValue("RED", EnumValueConfig{})}})
	input := NewInputObject(InputObjectConfig{
		Name:   "Filter",
		Fields: []*InputField{NewInputField("term", InputFieldConfig{Type: String})},
	})

	// A union has no fields and no interfaces of its own.
	for _, field := range []string{"fields", "interfaces", "enumValues", "inputFields"} {
		if got := resolveOn(t, TypeIntrospectionType, field, union, nil, nil); got != nil {
			t.Errorf("%s of a union = %v, want nil", field, got)
		}
	}
	// An enum has members but no fields.
	if got := resolveOn(t, TypeIntrospectionType, "fields", enum, nil, nil); got != nil {
		t.Errorf("fields of an enum = %v, want nil", got)
	}
	if got, _ := resolveOn(t, TypeIntrospectionType, "enumValues", enum, nil, nil).([]any); len(got) != 1 {
		t.Errorf("enumValues of an enum = %v, want one", got)
	}
	// An interface has fields, and an input object has input fields.
	if got, _ := resolveOn(t, TypeIntrospectionType, "fields", iface, nil, nil).([]any); len(got) != 1 {
		t.Errorf("fields of an interface = %v, want one", got)
	}
	if got, _ := resolveOn(t, TypeIntrospectionType, "inputFields", input, nil, nil).([]any); len(got) != 1 {
		t.Errorf("inputFields of an input object = %v, want one", got)
	}
	if got := resolveOn(t, TypeIntrospectionType, "isOneOf", enum, nil, nil); got != nil {
		t.Errorf("isOneOf of an enum = %v, want nil", got)
	}

	// __type needs a schema to look anything up in.
	if got, _ := TypeMetaField.Resolve(context.Background(), nil,
		NewArguments(map[string]any{"name": "User"}), nil); got != nil {
		t.Errorf("__type without a schema = %v, want nil", got)
	}
}
