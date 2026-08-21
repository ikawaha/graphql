package utilities_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

// mustExtend applies SDL to a schema, failing the test if it cannot.
func mustExtend(t *testing.T, s *schema.Schema, sdl string) *schema.Schema {
	t.Helper()
	extended, err := utilities.ExtendSchemaSource(s, sdl)
	if err != nil {
		t.Fatalf("extending: %v\n%s", err, sdl)
	}
	return extended
}

// fieldNamesOfType lists a composite type's field names, for reporting.
func fieldNamesOfType(t schema.NamedType) []string {
	switch typ := t.(type) {
	case *schema.ObjectType:
		return fieldNamesOf(typ.Fields())
	case *schema.InterfaceType:
		return fieldNamesOf(typ.Fields())
	default:
		return nil
	}
}

// hasField reports whether a type has a field of the given name.
func hasField(t schema.NamedType, name string) bool {
	switch typ := t.(type) {
	case *schema.ObjectType:
		return typ.Field(name) != nil
	case *schema.InterfaceType:
		return typ.Field(name) != nil
	case *schema.InputObjectType:
		return typ.Field(name) != nil
	default:
		return false
	}
}

// Each kind of type can be extended, and what an extension adds joins what was
// already there rather than replacing it.
func TestExtendSchema_EveryKindOfType(t *testing.T) {
	base := mustBuild(t, `
		scalar DateTime
		interface Node { id: ID! }
		type User implements Node { id: ID! name: String }
		type Photo { url: String }
		union Media = User
		enum Colour { RED }
		input Filter { byId: ID }
		type Query { me: User media: Media colour: Colour }
	`)

	got := mustExtend(t, base, `
		extend scalar DateTime @specifiedBy(url: "https://example.com")
		extend interface Node { createdAt: DateTime }
		extend type User { email: String createdAt: DateTime }
		extend union Media = Photo
		extend enum Colour { GREEN }
		extend input Filter { byName: String }
	`)

	t.Run("scalar", func(t *testing.T) {
		scalar, _ := got.Type("DateTime").(*schema.ScalarType)
		if scalar == nil {
			t.Fatal("DateTime was lost")
		}
		if scalar.SpecifiedByURL != "https://example.com" {
			t.Errorf("SpecifiedByURL = %q, want the one the extension gave", scalar.SpecifiedByURL)
		}
	})

	t.Run("interface", func(t *testing.T) {
		iface := got.Type("Node")
		for _, name := range []string{"id", "createdAt"} {
			if !hasField(iface, name) {
				t.Errorf("Node has no field %q; fields = %v", name, fieldNamesOfType(iface))
			}
		}
	})

	t.Run("object", func(t *testing.T) {
		user := got.Type("User")
		for _, name := range []string{"id", "name", "email", "createdAt"} {
			if !hasField(user, name) {
				t.Errorf("User has no field %q; fields = %v", name, fieldNamesOfType(user))
			}
		}
		object := user.(*schema.ObjectType)
		if n := len(object.Interfaces()); n != 1 {
			t.Errorf("User implements %d interfaces, want 1", n)
		}
	})

	t.Run("union", func(t *testing.T) {
		union, _ := got.Type("Media").(*schema.UnionType)
		if union == nil {
			t.Fatal("Media was lost")
		}
		var names []string
		for _, m := range union.Types() {
			names = append(names, m.Name())
		}
		if len(names) != 2 || names[0] != "User" || names[1] != "Photo" {
			t.Errorf("Media = %v, want [User Photo]", names)
		}
	})

	t.Run("enum", func(t *testing.T) {
		enum, _ := got.Type("Colour").(*schema.EnumType)
		if enum == nil {
			t.Fatal("Colour was lost")
		}
		for _, name := range []string{"RED", "GREEN"} {
			if enum.Value(name) == nil {
				t.Errorf("Colour has no member %q", name)
			}
		}
	})

	t.Run("input object", func(t *testing.T) {
		input := got.Type("Filter")
		for _, name := range []string{"byId", "byName"} {
			if !hasField(input, name) {
				t.Errorf("Filter has no field %q", name)
			}
		}
	})

	if err := schema.AssertValidSchema(got); err != nil {
		t.Errorf("the extended schema is not sound:\n%v", err)
	}
}

// This is the property the whole design turns on. Extending one type means
// every type that mentions it must see the new version; a schema where Query
// still points at the type as it was before would be quietly wrong, and the
// mistake would only show up at execution.
func TestExtendSchema_ReferencesPointAtTheNewTypes(t *testing.T) {
	base := mustBuild(t, `
		type User { name: String friend: User }
		type Query { me: User users(filter: Filter): [User!]! }
		input Filter { byName: String }
	`)
	got := mustExtend(t, base, `extend type User { email: String }`)

	user, _ := got.Type("User").(*schema.ObjectType)
	if user == nil {
		t.Fatal("User was lost")
	}
	if user.Field("email") == nil {
		t.Fatal("the extension did not take effect")
	}

	t.Run("through a field", func(t *testing.T) {
		me := got.QueryType().Field("me")
		if me == nil {
			t.Fatal("Query.me was lost")
		}
		if me.Type != schema.Type(user) {
			t.Error("Query.me points at the old User")
		}
	})

	t.Run("through a wrapped field", func(t *testing.T) {
		// [User!]! has to come back out with its wrappers intact and the new
		// type inside.
		users := got.QueryType().Field("users")
		if users == nil {
			t.Fatal("Query.users was lost")
		}
		if got, want := users.Type.String(), "[User!]!"; got != want {
			t.Errorf("Query.users: %s, want %s", got, want)
		}
		if schema.NamedTypeOf(users.Type) != schema.NamedType(user) {
			t.Error("Query.users points at the old User")
		}
	})

	t.Run("through itself", func(t *testing.T) {
		// A type that refers to itself must come out of the rebuild pointing
		// at its new self, not at the version it was rebuilt from.
		friend := user.Field("friend")
		if friend == nil {
			t.Fatal("User.friend was lost")
		}
		if friend.Type != schema.Type(user) {
			t.Error("User.friend points at the old User")
		}
	})

	t.Run("through an argument", func(t *testing.T) {
		arg := got.QueryType().Field("users").Arg("filter")
		if arg == nil {
			t.Fatal("the filter argument was lost")
		}
		if arg.Type != schema.Type(got.Type("Filter")) {
			t.Error("the argument points at the old Filter")
		}
	})
}

// Extending returns a new schema and leaves the old one alone, so a caller
// holding the original keeps the schema they had.
func TestExtendSchema_LeavesTheOriginalAlone(t *testing.T) {
	base := mustBuild(t, `type Query { a: String }`)
	before := utilities.PrintSchema(base)

	got := mustExtend(t, base, `extend type Query { b: Int }`)

	if base.QueryType().Field("b") != nil {
		t.Error("the original schema gained a field")
	}
	if after := utilities.PrintSchema(base); after != before {
		t.Errorf("the original changed:\nbefore:\n%s\n\nafter:\n%s", before, after)
	}
	if got.QueryType().Field("b") == nil {
		t.Error("the extension did not take effect on the result")
	}
	if got.QueryType() == base.QueryType() {
		t.Error("the extended schema shares its root with the original")
	}
}

// A schema written in Go has no AST behind it, so extending one exercises the
// path that rebuilds a type from the type itself.
func TestExtendSchema_ASchemaBuiltInGo(t *testing.T) {
	resolved := false
	user := schema.NewObject(schema.ObjectConfig{
		Name: "User",
		Fields: []*schema.Field{
			schema.NewField("name", schema.FieldConfig{
				Type: schema.String,
				Resolve: func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
					resolved = true
					return "Ada", nil
				},
			}),
		},
	})
	base := schema.New(schema.Config{
		Query: schema.NewObject(schema.ObjectConfig{
			Name:   "Query",
			Fields: []*schema.Field{schema.NewField("me", schema.FieldConfig{Type: user})},
		}),
	})

	got := mustExtend(t, base, `extend type User { email: String }`)

	extended, _ := got.Type("User").(*schema.ObjectType)
	if extended == nil {
		t.Fatal("User was lost")
	}
	if extended.Field("email") == nil {
		t.Error("the extension did not take effect")
	}

	// A resolver is part of what a type is, and losing one in the rebuild
	// would leave a schema that looks right and returns nothing.
	name := extended.Field("name")
	if name == nil {
		t.Fatal("User.name was lost")
	}
	if name.Resolve == nil {
		t.Fatal("User.name lost its resolver")
	}
	if _, err := name.Resolve(context.Background(), nil, schema.Arguments{}, nil); err != nil {
		t.Fatalf("calling the resolver: %v", err)
	}
	if !resolved {
		t.Error("the field carries a different resolver than the one it was given")
	}
}

// A document applied to a schema may bring types of its own, and they can
// refer to what the schema already had and the other way round.
func TestExtendSchema_AddsNewTypes(t *testing.T) {
	base := mustBuild(t, `type Query { me: User } type User { name: String }`)
	got := mustExtend(t, base, `
		type Post { title: String author: User }
		extend type User { posts: [Post!] }
	`)

	post, _ := got.Type("Post").(*schema.ObjectType)
	if post == nil {
		t.Fatal("Post was not added")
	}
	if post.Field("author").Type != schema.Type(got.Type("User")) {
		t.Error("Post.author points at the old User")
	}
	posts := got.Type("User").(*schema.ObjectType).Field("posts")
	if posts == nil {
		t.Fatal("User.posts was not added")
	}
	if schema.NamedTypeOf(posts.Type) != schema.NamedType(post) {
		t.Error("User.posts does not point at the new Post")
	}
	if err := schema.AssertValidSchema(got); err != nil {
		t.Errorf("the extended schema is not sound:\n%v", err)
	}
}

// The roots can be changed by a schema extension, which is how a schema that
// began with only a query gains a mutation.
func TestExtendSchema_Roots(t *testing.T) {
	base := mustBuild(t, `type Query { a: String }`)

	t.Run("a root is added", func(t *testing.T) {
		got := mustExtend(t, base, `
			type Mutation { b: String }
			extend schema { mutation: Mutation }
		`)
		if got.MutationType() == nil {
			t.Fatal("no mutation root")
		}
		if got.MutationType().Name() != "Mutation" {
			t.Errorf("the mutation root is %s", got.MutationType().Name())
		}
		if got.QueryType() == nil || got.QueryType().Name() != "Query" {
			t.Error("the query root did not carry over")
		}
	})

	t.Run("the roots carry over untouched", func(t *testing.T) {
		got := mustExtend(t, base, `extend type Query { b: String }`)
		if got.QueryType() == nil {
			t.Fatal("the query root was lost")
		}
		if got.QueryType() != got.Type("Query") {
			t.Error("the root is not the type of that name in the new schema")
		}
	})

	// A root a request cannot enter through is held rather than refused, as
	// graphql-js holds it, and reported by the schema check.
	t.Run("a root named as something that is not an object", func(t *testing.T) {
		got, err := utilities.ExtendSchemaSource(base, `
			scalar Thing
			extend schema { mutation: Thing }
		`)
		if err != nil {
			t.Fatalf("extending: %v", err)
		}
		if named := got.DeclaredRootType(language.OperationMutation); named == nil || named.Name() != "Thing" {
			t.Fatalf("the mutation root is %v, want Thing", named)
		}
		if got.MutationType() != nil {
			t.Error("a scalar was answered as the type a mutation enters through")
		}
		want := `Mutation root type must be Object type if provided, it cannot be Thing.`
		var said []string
		for _, e := range schema.ValidateSchema(got) {
			said = append(said, e.Message)
		}
		if !slices.Contains(said, want) {
			t.Errorf("ValidateSchema said %q\nwant it to include %q", said, want)
		}
	})

	// A root type named by a schema definition rather than by convention must
	// survive being rebuilt.
	t.Run("an unconventionally named root", func(t *testing.T) {
		odd := mustBuild(t, "schema { query: Root }\ntype Root { a: String }")
		got := mustExtend(t, odd, `extend type Root { b: String }`)
		if got.QueryType() == nil {
			t.Fatal("the query root was lost")
		}
		if got.QueryType().Name() != "Root" {
			t.Errorf("the query root is %s, want Root", got.QueryType().Name())
		}
		if got.QueryType().Field("b") == nil {
			t.Error("the extension did not reach the root")
		}
	})
}

// Directives declared by the schema carry over, and a document can add more.
func TestExtendSchema_Directives(t *testing.T) {
	base := mustBuild(t, `
		directive @auth(role: Role!) on FIELD_DEFINITION
		enum Role { ADMIN }
		type Query { a: String }
	`)
	got := mustExtend(t, base, `directive @tag(name: String!) on OBJECT`)

	for _, name := range []string{"auth", "tag", "skip", "include", "deprecated"} {
		if got.Directive(name) == nil {
			t.Errorf("@%s is missing", name)
		}
	}

	// A directive's arguments name types too, so they need pointing at the new
	// ones just as a field's do.
	auth := got.Directive("auth")
	arg := auth.Arg("role")
	if arg == nil {
		t.Fatal("@auth lost its argument")
	}
	if schema.NamedTypeOf(arg.Type) != schema.NamedType(got.Type("Role")) {
		t.Error("@auth's argument points at the old Role")
	}
}

// Extending is refused where carrying on would produce a schema that quietly
// differs from what the document asked for.
func TestExtendSchema_Errors(t *testing.T) {
	base := mustBuild(t, `type Query { a: String } type User { name: String }`)

	tests := []struct {
		name string
		sdl  string
		want string
	}{
		{
			name: "extending what is not there",
			sdl:  `extend type Missing { a: String }`,
			want: "Missing",
		},
		{
			name: "redefining what the schema already has",
			sdl:  `type User { other: String }`,
			want: "User",
		},
		{
			name: "two definitions of one name",
			sdl:  "type Post { a: String }\ntype Post { b: String }",
			want: "Post",
		},
		{
			name: "more than one schema definition",
			sdl:  "schema { query: Query }\nschema { query: Query }",
			want: "Cannot define a new schema",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := utilities.ExtendSchemaSource(base, tt.sdl)
			if err == nil {
				t.Fatal("extended without complaint")
			}
			// The message is the one graphql-js gives, and nothing else:
			// the check that produced it is the same check, so a caller
			// comparing against graphql-js sees the same string.
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to mention %q", err, tt.want)
			}
		})
	}

	t.Run("nothing to apply", func(t *testing.T) {
		if _, err := utilities.ExtendSchema(base, nil); err == nil {
			t.Error("extending with no document succeeded")
		}
	})

	t.Run("SDL that does not parse", func(t *testing.T) {
		if _, err := utilities.ExtendSchemaSource(base, "extend type {"); err == nil {
			t.Error("unparseable SDL was accepted")
		}
	})
}

// The types every schema has are the same objects in the extended schema, not
// copies: a rebuilt Int would no longer be the Int a schema written in Go
// refers to, and the two would stop comparing equal.
func TestExtendSchema_BuiltInsAreShared(t *testing.T) {
	base := mustBuild(t, `type Query { a: Int }`)
	got := mustExtend(t, base, `extend type Query { b: String }`)

	if got.Type("Int") != schema.NamedType(schema.Int) {
		t.Error("Int was rebuilt rather than shared")
	}
	if got.Type("__Schema") != base.Type("__Schema") {
		t.Error("an introspection type was rebuilt rather than shared")
	}
	if got.QueryType().Field("a").Type != schema.Type(schema.Int) {
		t.Error("a field of a built-in type no longer points at it")
	}
}

// Extending must not disturb the text a schema prints as, beyond what the
// extension adds: a schema that has been through an extension has to read back
// as the same schema.
func TestExtendSchema_PrintsAndReadsBack(t *testing.T) {
	base := mustBuild(t, `
		"A schema."
		schema { query: Query }
		interface Node { id: ID! }
		type User implements Node {
			"The identifier."
			id: ID!
			old: String @deprecated(reason: "Use name.")
		}
		enum Colour { RED "Faded." PUCE @deprecated }
		input Paging { limit: Int = 10 after: String = null }
		directive @auth(role: String! = "user") repeatable on FIELD_DEFINITION
		type Query { node: Node users(paging: Paging): [User!]! }
	`)

	got := mustExtend(t, base, `
		extend type User { name: String }
		extend enum Colour { GREEN }
	`)
	printed := utilities.PrintSchema(got)

	again, err := utilities.BuildSchema(printed)
	if err != nil {
		t.Fatalf("reading back what was written: %v\n%s", err, printed)
	}
	if second := utilities.PrintSchema(again); second != printed {
		t.Errorf("the text drifted:\nfirst:\n%s\n\nsecond:\n%s", printed, second)
	}

	// What the extension added is in the text, and what was there is intact.
	for _, want := range []string{
		"name: String",
		"GREEN",
		`"""A schema."""`,
		`old: String @deprecated(reason: "Use name.")`,
		"directive @auth(role: String! = \"user\") repeatable on FIELD_DEFINITION",
		"users(paging: Paging): [User!]!",
		"limit: Int = 10",
		"after: String = null",
	} {
		if !strings.Contains(printed, want) {
			t.Errorf("the printed schema does not contain %q:\n%s", want, printed)
		}
	}
}

// Extending twice has to work as well as extending once: the result of an
// extension is an ordinary schema.
func TestExtendSchema_Repeatedly(t *testing.T) {
	s := mustBuild(t, `type Query { a: String }`)
	for _, sdl := range []string{
		`extend type Query { b: String }`,
		`extend type Query { c: String }`,
		"type User { name: String }\nextend type Query { me: User }",
	} {
		s = mustExtend(t, s, sdl)
	}

	for _, name := range []string{"a", "b", "c", "me"} {
		if s.QueryType().Field(name) == nil {
			t.Errorf("Query has no field %q; fields = %v", name, fieldNamesOf(s.QueryType().Fields()))
		}
	}
	if err := schema.AssertValidSchema(s); err != nil {
		t.Errorf("the schema is not sound after three extensions:\n%v", err)
	}
}

// An empty document leaves the schema saying the same thing, which is the
// least a rebuild has to manage.
func TestExtendSchema_WithNothingToAdd(t *testing.T) {
	base := mustBuild(t, `
		interface Node { id: ID! }
		type User implements Node { id: ID! friend: User }
		union Media = User
		enum Colour { RED }
		input Filter { byId: ID }
		scalar DateTime @specifiedBy(url: "https://example.com")
		type Query { me: User media: Media colour: Colour when: DateTime f(x: Filter): String }
	`)

	got, err := utilities.ExtendSchema(base, &language.Document{})
	if err != nil {
		t.Fatalf("extending with an empty document: %v", err)
	}
	if before, after := utilities.PrintSchema(base), utilities.PrintSchema(got); before != after {
		t.Errorf("an empty extension changed the schema:\nbefore:\n%s\n\nafter:\n%s", before, after)
	}
	if err := schema.AssertValidSchema(got); err != nil {
		t.Errorf("the rebuilt schema is not sound:\n%v", err)
	}
}

// A real schema is the honest test of a rebuild: every construct at once,
// nothing chosen to suit the implementation.
func TestExtendSchema_GitHubSchema(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "language", "testdata", "github-schema.graphql"))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	base := mustBuild(t, string(body))

	got := mustExtend(t, base, `extend type Repository { claudeReviewed: Boolean }`)

	if err := schema.AssertValidSchema(got); err != nil {
		t.Fatalf("the extended schema is not sound:\n%v", err)
	}
	if n, want := len(got.Types()), len(base.Types()); n != want {
		t.Errorf("%d types after extending, want %d", n, want)
	}
	repository, _ := got.Type("Repository").(*schema.ObjectType)
	if repository == nil {
		t.Fatal("Repository was lost")
	}
	if repository.Field("claudeReviewed") == nil {
		t.Error("the extension did not take effect")
	}
	// Everything that mentioned Repository has to see the new one.
	owner, _ := got.Type("RepositoryOwner").(*schema.InterfaceType)
	if owner == nil {
		t.Fatal("RepositoryOwner was lost")
	}
	repositories := owner.Field("repositories")
	if repositories == nil {
		t.Fatal("RepositoryOwner.repositories was lost")
	}
	// The printed schema still reads back as itself.
	printed := utilities.PrintSchema(got)
	again, err := utilities.BuildSchema(printed)
	if err != nil {
		t.Fatalf("reading back the extended schema: %v", err)
	}
	if utilities.PrintSchema(again) != printed {
		t.Error("the text drifted after extending")
	}
}

func BenchmarkExtendSchema_GitHubSchema(b *testing.B) {
	body, err := os.ReadFile(filepath.Join("..", "language", "testdata", "github-schema.graphql"))
	if err != nil {
		b.Fatal(err)
	}
	base, err := utilities.BuildSchema(string(body))
	if err != nil {
		b.Fatal(err)
	}
	doc, err := language.ParseString(`extend type Repository { claudeReviewed: Boolean }`)
	if err != nil {
		b.Fatal(err)
	}
	// As for building: the document is checked before it is applied, and what
	// that costs is the difference between these two.
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
				if _, err := utilities.ExtendSchema(base, doc, c.opts...); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// An extension can make a type implement an interface it did not before.
func TestExtendSchema_Interfaces(t *testing.T) {
	base := mustBuild(t, `
		interface Node { id: ID! }
		type User { id: ID! }
		type Query { me: User node: Node }
	`)

	t.Run("an interface is taken on", func(t *testing.T) {
		got := mustExtend(t, base, `extend type User implements Node`)
		user := got.Type("User").(*schema.ObjectType)
		if len(user.Interfaces()) != 1 || user.Interfaces()[0].Name() != "Node" {
			t.Fatalf("User implements %v, want [Node]", user.Interfaces())
		}
		// The interface has to be the one in the new schema, or the schema
		// would hold two Nodes and nothing would match.
		if user.Interfaces()[0].Named() != got.Type("Node") {
			t.Error("User implements the old Node")
		}
		if err := schema.AssertValidSchema(got); err != nil {
			t.Errorf("the extended schema is not sound:\n%v", err)
		}
		// And the schema now knows User as one of Node's implementations.
		var found bool
		for _, impl := range got.PossibleTypes(got.Type("Node").(schema.AbstractType)) {
			if impl.Name() == "User" {
				found = true
			}
		}
		if !found {
			t.Error("User was not indexed as an implementation of Node")
		}
	})

	// Declaring the same interface twice is a mistake in the document rather
	// than something to quietly tidy up, and the validator is what says so.
	t.Run("declared twice", func(t *testing.T) {
		with := mustBuild(t, `
			interface Node { id: ID! }
			type User implements Node { id: ID! }
			type Query { me: User node: Node }
		`)
		got := mustExtend(t, with, `extend type User implements Node`)
		err := schema.AssertValidSchema(got)
		if err == nil {
			t.Fatal("declaring an interface twice was accepted")
		}
		if !strings.Contains(err.Error(), "once") {
			t.Errorf("error = %v, want it to say the interface is implemented twice", err)
		}
	})
}

// A document redeclaring a directive the schema already has is refused. There
// is no replacing one: the two lists are put end to end, so carrying on would
// leave the schema holding two directives of one name.
func TestExtendSchema_RedeclaringADirective(t *testing.T) {
	base := mustBuild(t, `
		directive @auth(role: String!) on FIELD_DEFINITION
		type Query { a: String }
	`)
	const redeclared = `directive @auth(role: String!, scope: String) on FIELD_DEFINITION | OBJECT`

	_, err := utilities.ExtendSchemaSource(base, redeclared)
	if err == nil {
		t.Fatal("redeclaring a directive the schema already has was accepted")
	}
	if want := `Directive "@auth" already exists in the schema. It cannot be redefined.`; err.Error() != want {
		t.Errorf("error = %q\nwant %q", err, want)
	}

	// Asking for the check to be skipped is asking for what it was refusing,
	// and what that produces is a schema holding both.
	t.Run("with the check skipped", func(t *testing.T) {
		got, err := utilities.ExtendSchemaSource(base, redeclared, utilities.AssumeValidSDL())
		if err != nil {
			t.Fatalf("extending: %v", err)
		}
		var count, withScope int
		for _, d := range got.Directives() {
			if d.Name() != "auth" {
				continue
			}
			count++
			if d.Arg("scope") != nil {
				withScope++
			}
		}
		if count != 2 {
			t.Errorf("@auth appears %d times, want 2", count)
		}
		if withScope != 1 {
			t.Errorf("%d of them have the new argument, want 1", withScope)
		}
	})
}

// A subscription root is set the same way as the others, and is easy to leave
// out of a switch.
func TestExtendSchema_SubscriptionRoot(t *testing.T) {
	base := mustBuild(t, `type Query { a: String }`)
	got := mustExtend(t, base, `
		type Subscription { events: String }
		extend schema { subscription: Subscription }
	`)
	if got.SubscriptionType() == nil {
		t.Fatal("no subscription root")
	}
	if got.SubscriptionType() != got.Type("Subscription") {
		t.Error("the subscription root is not the type of that name in the new schema")
	}
	if err := schema.AssertValidSchema(got); err != nil {
		t.Errorf("the extended schema is not sound:\n%v", err)
	}
}

// A schema definition in the applied document, rather than an extension of
// one, is refused: the schema being extended has its roots already, and a
// second definition of them is not an extension of anything.
func TestExtendSchema_SchemaDefinitionInTheDocument(t *testing.T) {
	base := mustBuild(t, `type Query { a: String }`)
	const doc = `
		"Now described."
		schema { query: Root }
		type Root { b: String }
	`
	_, err := utilities.ExtendSchemaSource(base, doc)
	if err == nil {
		t.Fatal("a schema definition in an extension document was accepted")
	}
	if want := "Cannot define a new schema within a schema extension."; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %v, want it to mention %q", err, want)
	}

	// The machinery underneath does name the roots outright, which is what
	// the check is standing in front of.
	t.Run("with the check skipped", func(t *testing.T) {
		got, err := utilities.ExtendSchemaSource(base, doc, utilities.AssumeValidSDL())
		if err != nil {
			t.Fatalf("extending: %v", err)
		}
		if got.QueryType() == nil || got.QueryType().Name() != "Root" {
			t.Fatalf("the query root is %v, want Root", got.QueryType())
		}
		if got.Description() != "Now described." {
			t.Errorf("Description = %q, want the one the document gave", got.Description())
		}
	})
}

// @oneOf can be applied to an input object by an extension as well as by its
// definition, since it is what says the type takes exactly one of its fields.
func TestExtendSchema_OneOfByExtension(t *testing.T) {
	base := mustBuild(t, `input Filter { byId: ID } type Query { f(x: Filter): String }`)
	got := mustExtend(t, base, `extend input Filter @oneOf { byName: String }`)

	input, _ := got.Type("Filter").(*schema.InputObjectType)
	if input == nil {
		t.Fatal("Filter was lost")
	}
	if !input.IsOneOf {
		t.Error("the extension did not mark the type as a choice of one field")
	}
	if !strings.Contains(utilities.PrintSchema(got), "input Filter @oneOf {") {
		t.Error("@oneOf was not written back out")
	}
}

// Rebuilding a schema means walking everything in it, so a schema assembled in
// Go with gaps in it must not bring the process down. A missing type or a nil
// entry is a mistake for the schema validator to report, not something to
// crash on part-way through.
func TestExtendSchema_ToleratesAMalformedSchema(t *testing.T) {
	iface := schema.NewInterface(schema.InterfaceConfig{
		Name:   "Node",
		Fields: []*schema.Field{nil, schema.NewField("id", schema.FieldConfig{Type: schema.ID})},
	})
	input := schema.NewInputObject(schema.InputObjectConfig{
		Name: "Filter",
		Fields: []*schema.InputField{
			nil,
			schema.NewInputField("byId", schema.InputFieldConfig{Type: schema.ID}),
			// A field with no type at all.
			schema.NewInputField("broken", schema.InputFieldConfig{}),
		},
	})
	enum := schema.NewEnum(schema.EnumConfig{
		Name:   "Colour",
		Values: []*schema.EnumValue{nil, schema.NewEnumValue("RED", schema.EnumValueConfig{})},
	})
	user := schema.NewObject(schema.ObjectConfig{
		Name:       "User",
		Interfaces: schema.Implements(nil, iface),
		Fields: []*schema.Field{
			schema.NewField("id", schema.FieldConfig{Type: schema.ID}),
			// A field with no type, and one whose argument has none.
			schema.NewField("broken", schema.FieldConfig{}),
			schema.NewField("search", schema.FieldConfig{
				Type: schema.String,
				Args: []*schema.Argument{nil, schema.NewArgument("filter", schema.ArgumentConfig{Type: input})},
			}),
		},
	})
	union := schema.NewUnion(schema.UnionConfig{Name: "Media", Types: schema.Members(nil, user)})
	base := schema.New(schema.Config{
		Query: schema.NewObject(schema.ObjectConfig{
			Name: "Query",
			Fields: []*schema.Field{
				schema.NewField("me", schema.FieldConfig{Type: user}),
				schema.NewField("media", schema.FieldConfig{Type: union}),
				schema.NewField("colour", schema.FieldConfig{Type: enum}),
			},
		}),
	})

	got, err := utilities.ExtendSchemaSource(base, `extend type User { email: String }`)
	if err != nil {
		t.Fatalf("extending a schema with gaps in it: %v", err)
	}

	// What was sound came through, and the extension took effect.
	extended, _ := got.Type("User").(*schema.ObjectType)
	if extended == nil {
		t.Fatal("User was lost")
	}
	for _, name := range []string{"id", "search", "email"} {
		if extended.Field(name) == nil {
			t.Errorf("User has no field %q; fields = %v", name, fieldNamesOf(extended.Fields()))
		}
	}
	// A field that named no type cannot be carried over, and the schema
	// validator is what reports the gap.
	if extended.Field("broken") != nil {
		t.Error("a field with no type was carried over")
	}
	if got.Type("Filter").(*schema.InputObjectType).Field("broken") != nil {
		t.Error("an input field with no type was carried over")
	}
	if n := len(extended.Interfaces()); n != 1 {
		t.Errorf("User implements %d interfaces, want the one that is really there", n)
	}
	if n := len(got.Type("Media").(*schema.UnionType).Types()); n != 1 {
		t.Errorf("Media has %d members, want the one that is really there", n)
	}
	if n := len(got.Type("Colour").(*schema.EnumType).Values()); n != 1 {
		t.Errorf("Colour has %d members, want the one that is really there", n)
	}
	if n := len(extended.Field("search").Args); n != 1 {
		t.Errorf("search takes %d arguments, want the one that is really there", n)
	}
}
