package utilities_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

func mustBuild(t *testing.T, sdl string) *schema.Schema {
	t.Helper()
	s, err := utilities.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("building:\n%s\n%v", sdl, err)
	}
	return s
}

func fieldNamesOf(fields []*schema.Field) []string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.Name()
	}
	return names
}

func TestBuildSchema_Simple(t *testing.T) {
	s := mustBuild(t, `
		type Query {
			hello: String
			count: Int!
		}
	`)

	query := s.QueryType()
	if query == nil {
		t.Fatal("no query root")
	}
	if got := query.Name(); got != "Query" {
		t.Errorf("query root = %q, want Query", got)
	}
	if got := fieldNamesOf(query.Fields()); !slices.Equal(got, []string{"hello", "count"}) {
		t.Errorf("fields = %v, want them in the order they were written", got)
	}
	if got := query.Field("hello").Type; got != schema.Type(schema.String) {
		t.Errorf("hello: %v, want String", got)
	}
	if got := query.Field("count").Type.String(); got != "Int!" {
		t.Errorf("count: %s, want Int!", got)
	}
	if err := schema.AssertValidSchema(s); err != nil {
		t.Errorf("the schema is not sound: %v", err)
	}
}

// Types name one another in any order, and a document is read once through, so
// a reference is resolved only after every type exists.
func TestBuildSchema_ReferencesInAnyOrder(t *testing.T) {
	s := mustBuild(t, `
		type Query { me: User }
		type User { friends: [User!]! manager: Employee }
		type Employee { reports: [User!] }
	`)

	user, isObject := s.Type("User").(*schema.ObjectType)
	if !isObject {
		t.Fatalf("User is %T, want an object type", s.Type("User"))
	}
	if got := user.Field("friends").Type.String(); got != "[User!]!" {
		t.Errorf("User.friends: %s, want [User!]!", got)
	}
	// Employee is written after the field that names it.
	if got := user.Field("manager").Type; got != schema.Type(s.Type("Employee")) {
		t.Errorf("User.manager: %v, want the Employee type", got)
	}
	if err := schema.AssertValidSchema(s); err != nil {
		t.Errorf("the schema is not sound: %v", err)
	}
}

func TestBuildSchema_AllKindsOfType(t *testing.T) {
	s := mustBuild(t, `
		"A scalar."
		scalar DateTime @specifiedBy(url: "https://example.com/datetime")

		interface Node { id: ID! }

		"A person."
		type User implements Node {
			id: ID!
			name(upper: Boolean = false): String
			old: String @deprecated(reason: "Use name.")
			gone: String @deprecated
		}

		type Photo { url: String }
		union Media = User | Photo

		enum Colour {
			RED
			"Out of fashion."
			PUCE @deprecated(reason: "Nobody likes it.")
		}

		input Filter @oneOf {
			byId: ID
			byName: String
		}

		type Query {
			node: Node
			media: Media
			colour: Colour
			search(filter: Filter): [User!]
			when: DateTime
		}
	`)

	t.Run("a scalar keeps its specification", func(t *testing.T) {
		when, isScalar := s.Type("DateTime").(*schema.ScalarType)
		if !isScalar {
			t.Fatalf("DateTime is %T", s.Type("DateTime"))
		}
		if when.SpecifiedByURL != "https://example.com/datetime" {
			t.Errorf("SpecifiedByURL = %q", when.SpecifiedByURL)
		}
		if when.Description() != "A scalar." {
			t.Errorf("Description() = %q", when.Description())
		}
	})

	t.Run("an object implements an interface", func(t *testing.T) {
		user := s.Type("User").(*schema.ObjectType)
		if got := len(user.Interfaces()); got != 1 {
			t.Fatalf("%d interfaces, want 1", got)
		}
		if user.Interfaces()[0].Named() != s.Type("Node") {
			t.Error("User does not implement the Node that the schema holds")
		}
		if user.Description() != "A person." {
			t.Errorf("Description() = %q", user.Description())
		}
	})

	// A reason given is kept; one left out falls back to the directive's own
	// default, which is what makes a bare @deprecated mean something.
	t.Run("deprecation", func(t *testing.T) {
		user := s.Type("User").(*schema.ObjectType)
		if got := user.Field("old").DeprecationReason.Or(""); got != "Use name." {
			t.Errorf("old: %q, want the reason given", got)
		}
		if got := user.Field("gone").DeprecationReason.Or(""); got != schema.DefaultDeprecationReason {
			t.Errorf("gone: %q, want the default reason", got)
		}
		if user.Field("name").IsDeprecated() {
			t.Error("a field with no @deprecated was marked deprecated")
		}

		colour := s.Type("Colour").(*schema.EnumType)
		if got := colour.Value("PUCE").DeprecationReason.Or(""); got != "Nobody likes it." {
			t.Errorf("PUCE: %q", got)
		}
		if got := colour.Value("PUCE").Description(); got != "Out of fashion." {
			t.Errorf("PUCE description = %q", got)
		}
	})

	t.Run("a union keeps its members in order", func(t *testing.T) {
		media := s.Type("Media").(*schema.UnionType)
		names := make([]string, len(media.Types()))
		for i, m := range media.Types() {
			names[i] = m.Name()
		}
		if !slices.Equal(names, []string{"User", "Photo"}) {
			t.Errorf("members = %v, want [User Photo]", names)
		}
	})

	t.Run("oneOf is carried across", func(t *testing.T) {
		filter := s.Type("Filter").(*schema.InputObjectType)
		if !filter.IsOneOf {
			t.Error("IsOneOf = false, want true")
		}
	})

	if err := schema.AssertValidSchema(s); err != nil {
		t.Errorf("the schema is not sound: %v", err)
	}
}

// A default is kept as the literal it was written as, so that printing the
// schema gives back the same text rather than a rendering of a converted
// value.
func TestBuildSchema_DefaultsKeepTheirLiteral(t *testing.T) {
	s := mustBuild(t, `
		enum Colour { RED }
		input Filter { limit: Int = 10 }
		type Query {
			f(
				n: Int = 1
				s: String = "x"
				c: Colour = RED
				maybe: String = null
				plain: String
				list: [Int] = [1, 2]
			): String
		}
	`)

	field := s.QueryType().Field("f")
	want := map[string]string{
		"n":     "1",
		"s":     `"x"`,
		"c":     "RED",
		"maybe": "null",
		"list":  "[1, 2]",
	}
	for name, text := range want {
		arg := field.Arg(name)
		if arg == nil {
			t.Errorf("no argument %q", name)
			continue
		}
		def, has := arg.Default.Get()
		if !has {
			t.Errorf("%s has no default", name)
			continue
		}
		if def.Literal == nil {
			t.Errorf("%s kept a value rather than the literal it was written as", name)
			continue
		}
		if got := language.Print(def.Literal); got != text {
			t.Errorf("%s = %s, want %s", name, got, text)
		}
	}

	// An argument with no default has none, which is different from a default
	// of null.
	if field.Arg("plain").Default.IsSet() {
		t.Error("an argument written without a default has one")
	}
	if !field.Arg("maybe").Default.IsSet() {
		t.Error("an argument whose default is null has none")
	}
	// The one with a default of null is therefore optional even though the
	// other is too; what matters is that they are told apart.
	if schema.IsRequiredArgument(field.Arg("maybe")) {
		t.Error("an argument defaulting to null was reported as required")
	}
}

func TestBuildSchema_RootTypes(t *testing.T) {
	t.Run("found by their conventional names", func(t *testing.T) {
		s := mustBuild(t, `
			type Query { a: String }
			type Mutation { b: String }
			type Subscription { c: String }
		`)
		if s.QueryType() == nil || s.MutationType() == nil || s.SubscriptionType() == nil {
			t.Error("a root was not found by its name")
		}
	})

	t.Run("named by a schema definition", func(t *testing.T) {
		s := mustBuild(t, `
			"The schema."
			schema {
				query: Root
				mutation: Change
			}
			type Root { a: String }
			type Change { b: String }
		`)
		if got := s.QueryType(); got == nil || got.Name() != "Root" {
			t.Errorf("query root = %v, want Root", got)
		}
		if got := s.MutationType(); got == nil || got.Name() != "Change" {
			t.Errorf("mutation root = %v, want Change", got)
		}
		if s.SubscriptionType() != nil {
			t.Error("a subscription root appeared from nowhere")
		}
		if got := s.Description(); got != "The schema." {
			t.Errorf("Description() = %q", got)
		}
	})

	// A type named Query is not the root when a schema definition says
	// otherwise.
	t.Run("a schema definition overrides the names", func(t *testing.T) {
		s := mustBuild(t, `
			schema { query: Root }
			type Root { a: String }
			type Query { b: String }
		`)
		if got := s.QueryType().Name(); got != "Root" {
			t.Errorf("query root = %s, want Root", got)
		}
	})
}

func TestBuildSchema_Directives(t *testing.T) {
	s := mustBuild(t, `
		"Restricts a field."
		directive @auth(role: String! = "user") repeatable on FIELD_DEFINITION | OBJECT
		type Query { a: String }
	`)

	auth := s.Directive("auth")
	if auth == nil {
		t.Fatal("the directive is missing")
	}
	if !auth.IsRepeatable {
		t.Error("IsRepeatable = false, want true")
	}
	if got := len(auth.Locations); got != 2 {
		t.Errorf("%d locations, want 2", got)
	}
	if got := auth.Description(); got != "Restricts a field." {
		t.Errorf("Description() = %q", got)
	}

	// A directive's arguments name types, so they are only buildable once
	// every type exists; this checks they were not dropped.
	role := auth.Arg("role")
	if role == nil {
		t.Fatal("the directive's argument is missing")
	}
	if got := role.Type.String(); got != "String!" {
		t.Errorf("role: %s, want String!", got)
	}
	if _, has := role.Default.Get(); !has {
		t.Error("the argument's default was dropped")
	}

	// The built-in directives are still there alongside it.
	for _, name := range []string{"skip", "include", "deprecated"} {
		if s.Directive(name) == nil {
			t.Errorf("the built-in @%s is missing", name)
		}
	}
}

// A directive of a built-in name replaces it rather than sitting beside it.
func TestBuildSchema_RedefinedDirective(t *testing.T) {
	s := mustBuild(t, `
		directive @skip(unless: Boolean!) on FIELD
		type Query { a: String }
	`)
	skip := s.Directive("skip")
	if skip == nil {
		t.Fatal("@skip is missing")
	}
	if skip.Arg("unless") == nil {
		t.Error("the redefined @skip does not have the argument the document gave it")
	}
	count := 0
	for _, d := range s.Directives() {
		if d.Name() == "skip" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("@skip appears %d times, want once", count)
	}
}

func TestBuildSchema_Errors(t *testing.T) {
	tests := []struct {
		name string
		sdl  string
		want string
	}{
		{"nothing at all", "", "Syntax Error"},
		{"an unparseable document", "type {", "Syntax Error"},
		{"two types of the same name", "type Query { a: String }\ntype Query { b: String }", `only one type named "Query"`},
		{"two schema definitions", "schema { query: Q }\nschema { query: Q }\ntype Q { a: String }", "only one schema definition"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := utilities.BuildSchema(tt.sdl)
			if err == nil {
				t.Fatal("built without complaint")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to mention %q", err, tt.want)
			}
		})
	}

	if _, err := utilities.BuildASTSchema(nil); err == nil {
		t.Error("building from nothing succeeded")
	}
}

// A schema may name something a request cannot enter through. graphql-js
// builds it and validateSchema says what is wrong; so does this, since a
// schema that could not be held could not be described to a client either.
func TestBuildSchema_ARootThatIsNotAnObject(t *testing.T) {
	for _, tt := range []struct{ name, sdl, says string }{
		{
			"named in a schema definition",
			"schema { query: C }\nscalar C",
			`Query root type must be Object type, it cannot be C.`,
		},
		{
			"taken up by its conventional name",
			"input Query { f: Int }",
			`Query root type must be Object type, it cannot be Query.`,
		},
		{
			"a mutation root",
			"type Query { a: String }\ninput In { f: Int }\n" +
				"schema { query: Query mutation: In }",
			`Mutation root type must be Object type if provided, it cannot be In.`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, err := utilities.BuildSchema(tt.sdl)
			if err != nil {
				t.Fatalf("building: %v", err)
			}
			errs := schema.ValidateSchema(s)
			var said []string
			for _, e := range errs {
				said = append(said, e.Message)
			}
			if !slices.Contains(said, tt.says) {
				t.Errorf("ValidateSchema said %q\nwant it to include %q", said, tt.says)
			}
			// And the schema still describes itself, which is what a client
			// asking a broken schema what it is receives.
			if got := utilities.PrintSchema(s); !strings.Contains(got, "C") && !strings.Contains(got, "Query") && !strings.Contains(got, "In") {
				t.Errorf("PrintSchema = %q, want it to hold the type it named", got)
			}
		})
	}
}

// A document may extend what it defines, so a schema split across definitions
// and extensions reads as one schema.
func TestBuildSchema_AppliesItsOwnExtensions(t *testing.T) {
	s := mustBuild(t, `
		directive @somewhere on SCHEMA
		type Query { a: String }
		extend type Query { b: Int }
		extend schema @somewhere
	`)

	query := s.QueryType()
	if query == nil {
		t.Fatal("the query root was lost")
	}
	for _, name := range []string{"a", "b"} {
		if query.Field(name) == nil {
			t.Errorf("Query has no field %q; fields = %v", name, fieldNamesOf(query.Fields()))
		}
	}
}

// Extending something no one defines is a mistake rather than a way to declare
// it, and saying so beats building a schema that quietly lacks the fields.
func TestBuildSchema_RefusesToExtendWhatIsNotThere(t *testing.T) {
	_, err := utilities.BuildSchema("type Query { a: String }\nextend type Missing { b: Int }")
	if err == nil {
		t.Fatal("extending an undefined type succeeded")
	}
	if !strings.Contains(err.Error(), "Missing") {
		t.Errorf("error = %v, want it to name the type", err)
	}
}

// A reference to a type the document never defines is refused. Leaving the
// field out would build a schema quietly missing what was asked for, and
// nothing afterwards could report the gap: a field that is not there is not
// something the validator can complain about.
func TestBuildSchema_RefusesAnUnknownType(t *testing.T) {
	for _, sdl := range []string{
		`type Query { a: Missing b: String }`,
		`type Query { a(arg: Missing): String }`,
		`type Query { a: String } type Other implements Missing { a: String }`,
	} {
		if _, err := utilities.BuildSchema(sdl); err == nil {
			t.Errorf("a schema naming an undefined type was built: %s", sdl)
		} else if !strings.Contains(err.Error(), "Missing") {
			t.Errorf("error = %v, want it to name the type", err)
		}
	}
}

func TestTypeFromAST(t *testing.T) {
	s := mustBuild(t, `type Query { a: String }`)

	tests := []struct {
		text string
		want string
	}{
		{"String", "String"},
		{"[String]", "[String]"},
		{"String!", "String!"},
		{"[String!]!", "[String!]!"},
		{"Query", "Query"},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			node, err := language.ParseType(language.NewSource(tt.text))
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}
			got, ok := utilities.TypeFromAST(s, node)
			if !ok {
				t.Fatalf("%s was not found", tt.text)
			}
			if got.String() != tt.want {
				t.Errorf("= %s, want %s", got, tt.want)
			}
		})
	}

	unknown, err := language.ParseType(language.NewSource("[Missing!]"))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	if _, ok := utilities.TypeFromAST(s, unknown); ok {
		t.Error("an unknown type was found")
	}
	if _, ok := utilities.TypeFromAST(nil, unknown); ok {
		t.Error("a type was found in no schema at all")
	}
}

// The real test of the builder is a schema written by someone else. This one
// is large, uses every kind of type, and is not tailored to what the builder
// happens to handle.
func TestBuildSchema_GitHubSchema(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "language", "testdata", "github-schema.graphql"))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}

	s, err := utilities.BuildSchema(string(body))
	if err != nil {
		t.Fatalf("building: %v", err)
	}
	if s.QueryType() == nil {
		t.Error("no query root")
	}
	if got := len(s.Types()); got < 500 {
		t.Errorf("%d types, want a large schema", got)
	}
	if err := schema.AssertValidSchema(s); err != nil {
		t.Errorf("the schema built from a real document is not sound:\n%v", err)
	}
}

// Building a schema from a document already parsed, which is the second half
// of what a server does at startup; the first half is BenchmarkParse_GitHubSchema
// in the language package.
func BenchmarkBuildASTSchema_GitHubSchema(b *testing.B) {
	body, err := os.ReadFile(filepath.Join("..", "language", "testdata", "github-schema.graphql"))
	if err != nil {
		b.Fatal(err)
	}
	doc, err := language.ParseString(string(body))
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(body)))

	// The document is checked against the rules a schema definition must
	// follow before anything is built from it. A caller who already knows a
	// document is sound can say so, and what that saves is the whole of the
	// difference between these two.
	for _, c := range []struct {
		name string
		opts []utilities.BuildOption
	}{
		{"checked", nil},
		{"AssumeValidSDL", []utilities.BuildOption{utilities.AssumeValidSDL()}},
	} {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := utilities.BuildASTSchema(doc, c.opts...); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// A document assembled in code, rather than parsed, can have holes in it. The
// builder skips those rather than crashing, leaving the schema validator to
// report what is actually missing.
func TestBuildASTSchema_ToleratesMalformedNodes(t *testing.T) {
	str := &language.NamedType{Name: &language.Name{Value: "String"}}
	name := func(s string) *language.Name { return &language.Name{Value: s} }

	doc := &language.Document{Definitions: []language.Definition{
		&language.ObjectTypeDefinition{
			Name: name("Query"),
			Fields: []*language.FieldDefinition{
				nil,
				{Name: nil, Type: str},
				{Name: name("ok"), Type: str, Arguments: []*language.InputValueDefinition{
					nil,
					{Name: nil, Type: str},
					{Name: name("arg"), Type: str},
				}},
			},
			Interfaces: []*language.NamedType{nil, {Name: nil}},
		},
		&language.InputObjectTypeDefinition{
			Name:   name("Filter"),
			Fields: []*language.InputValueDefinition{nil, {Name: nil, Type: str}, {Name: name("term"), Type: str}},
		},
		&language.UnionTypeDefinition{
			Name:  name("Media"),
			Types: []*language.NamedType{nil, {Name: nil}},
		},
		&language.EnumTypeDefinition{
			Name:   name("Colour"),
			Values: []*language.EnumValueDefinition{nil, {Name: nil}, {Name: name("RED")}},
		},
	}}

	s, err := utilities.BuildASTSchema(doc)
	if err != nil {
		t.Fatalf("building: %v", err)
	}
	if s.QueryType().Field("ok") == nil {
		t.Error("the sound field beside the malformed ones was dropped")
	}
	if s.QueryType().Field("ok").Arg("arg") == nil {
		t.Error("the sound argument beside the malformed ones was dropped")
	}
	if s.Type("Filter").(*schema.InputObjectType).Field("term") == nil {
		t.Error("the sound input field was dropped")
	}
	if s.Type("Colour").(*schema.EnumType).Value("RED") == nil {
		t.Error("the sound enum member was dropped")
	}
}

// A definition with no name cannot be indexed, so it is refused rather than
// silently skipped.
func TestBuildASTSchema_UnnamedDefinitions(t *testing.T) {
	for _, def := range []language.Definition{
		&language.ObjectTypeDefinition{},
		&language.DirectiveDefinition{},
	} {
		doc := &language.Document{Definitions: []language.Definition{def}}
		if _, err := utilities.BuildASTSchema(doc); err == nil {
			t.Errorf("%T with no name was accepted", def)
		}
	}
}
