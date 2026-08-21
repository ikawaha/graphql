package utilities_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

// printBuilt reads SDL and writes it back out, which is the shape most of
// these tests take.
func printBuilt(t *testing.T, sdl string) string {
	t.Helper()
	return utilities.PrintSchema(mustBuild(t, sdl))
}

func TestPrintSchema_Simple(t *testing.T) {
	got := printBuilt(t, `type Query { hello: String count: Int! }`)
	// No trailing newline: what comes back is a string, as it is in graphql-js.
	want := strings.Join([]string{
		"type Query {",
		"  hello: String",
		"  count: Int!",
		"}",
	}, "\n")
	if got != want {
		t.Errorf("=\n%s\nwant\n%s", got, want)
	}
}

// A schema definition is written only when it says something the conventional
// names do not.
func TestPrintSchema_SchemaDefinition(t *testing.T) {
	t.Run("left out when the roots have their usual names", func(t *testing.T) {
		got := printBuilt(t, `type Query { a: String }`)
		if strings.Contains(got, "schema {") {
			t.Errorf("a schema definition was written for conventional roots:\n%s", got)
		}
	})

	t.Run("written when a root is named otherwise", func(t *testing.T) {
		got := printBuilt(t, "schema { query: Root }\ntype Root { a: String }")
		if !strings.HasPrefix(got, "schema {\n  query: Root\n}") {
			t.Errorf("=\n%s\nwant it to begin with a schema definition", got)
		}
	})

	t.Run("written when the schema has a description", func(t *testing.T) {
		got := printBuilt(t, "\"The schema.\"\nschema { query: Query }\ntype Query { a: String }")
		// A description is written as a block string whenever it can be, which
		// is how real schemas are written.
		if !strings.HasPrefix(got, `"""The schema."""`+"\nschema {") {
			t.Errorf("=\n%s\nwant the description above the definition", got)
		}
	})
}

// A description that reads better across several lines is written as a block
// string rather than one escaped line.
func TestPrintSchema_Descriptions(t *testing.T) {
	got := printBuilt(t, `
		"A person."
		type User {
			"""
			The name.

			Over several lines.
			"""
			name: String
			plain: String
		}
		type Query { me: User }
	`)

	want := strings.Join([]string{
		`"""A person."""`,
		"type User {",
		`  """`,
		"  The name.",
		// An indented block string carries its indent onto every line,
		// including the empty ones.
		"  ",
		"  Over several lines.",
		`  """`,
		"  name: String",
		"  plain: String",
		"}",
	}, "\n")
	if !strings.Contains(got, want) {
		t.Errorf("=\n%s\nwant it to contain\n%s", got, want)
	}
}

// A described field after the first gets a blank line above it, so that a run
// of documented fields does not read as one wall of text.
func TestPrintSchema_BlankLineBetweenDescribedFields(t *testing.T) {
	got := printBuilt(t, `
		type Query {
			"First."
			a: String
			"Second."
			b: String
		}
	`)
	want := strings.Join([]string{
		"type Query {",
		`  """First."""`,
		"  a: String",
		"",
		`  """Second."""`,
		"  b: String",
		"}",
	}, "\n")
	if got != want {
		t.Errorf("=\n%s\nwant\n%s", got, want)
	}
}

func TestPrintSchema_Deprecation(t *testing.T) {
	got := printBuilt(t, `
		type Query {
			withReason: String @deprecated(reason: "Use other.")
			bare: String @deprecated
			fine: String
		}
	`)

	if !strings.Contains(got, `withReason: String @deprecated(reason: "Use other.")`) {
		t.Errorf("a reason given was not written back:\n%s", got)
	}
	// A reason equal to the one that would be assumed anyway is left out, so
	// the text stays as short as what was written.
	if !strings.Contains(got, "bare: String @deprecated\n") {
		t.Errorf("a bare deprecation was not written back bare:\n%s", got)
	}
	if strings.Contains(got, "fine: String @deprecated") {
		t.Errorf("a field that is not deprecated was marked:\n%s", got)
	}
}

func TestPrintSchema_AllKindsOfType(t *testing.T) {
	got := printBuilt(t, `
		scalar DateTime @specifiedBy(url: "https://example.com")
		interface Node { id: ID! }
		type User implements Node { id: ID! name: String }
		type Photo { url: String }
		union Media = User | Photo
		enum Colour { RED GREEN }
		input Filter @oneOf { byId: ID byName: String }
		type Query {
			node: Node
			media: Media
			colour: Colour
			search(filter: Filter, limit: Int = 10): [User!]
			when: DateTime
		}
	`)

	for _, want := range []string{
		`scalar DateTime @specifiedBy(url: "https://example.com")`,
		"interface Node {",
		"type User implements Node {",
		"union Media = User | Photo",
		"enum Colour {\n  RED\n  GREEN\n}",
		"input Filter @oneOf {",
		"search(filter: Filter, limit: Int = 10): [User!]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}
}

// An argument list goes on one line unless an argument carries a description,
// which needs a line of its own.
func TestPrintSchema_ArgumentLayout(t *testing.T) {
	inline := printBuilt(t, `type Query { f(a: Int, b: String): String }`)
	if !strings.Contains(inline, "f(a: Int, b: String): String") {
		t.Errorf("plain arguments were not kept on one line:\n%s", inline)
	}

	described := printBuilt(t, `
		type Query {
			f(
				"How many."
				a: Int
				b: String
			): String
		}
	`)
	want := strings.Join([]string{
		"  f(",
		`    """How many."""`,
		"    a: Int",
		"    b: String",
		"  ): String",
	}, "\n")
	if !strings.Contains(described, want) {
		t.Errorf("=\n%s\nwant it to contain\n%s", described, want)
	}
}

// The types and directives every schema has are left out, since writing them
// down would only add noise.
func TestPrintSchema_OmitsBuiltIns(t *testing.T) {
	got := printBuilt(t, `type Query { a: String b: Int c: Boolean d: ID e: Float }`)

	for _, absent := range []string{
		"scalar String", "scalar Int", "scalar Boolean", "scalar ID", "scalar Float",
		"__Schema", "__Type", "directive @skip", "directive @deprecated",
	} {
		if strings.Contains(got, absent) {
			t.Errorf("output contains %q, which every schema has:\n%s", absent, got)
		}
	}

	// A directive the author declared is written.
	withDirective := printBuilt(t, `
		directive @auth(role: String!) on FIELD_DEFINITION
		type Query { a: String }
	`)
	if !strings.Contains(withDirective, "directive @auth(role: String!) on FIELD_DEFINITION") {
		t.Errorf("a declared directive was left out:\n%s", withDirective)
	}
}

func TestPrintIntrospectionSchema(t *testing.T) {
	got := utilities.PrintIntrospectionSchema(mustBuild(t, `type Query { a: String }`))
	for _, want := range []string{"type __Schema {", "type __Type {", "enum __TypeKind {"} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q", want)
		}
	}
	if strings.Contains(got, "type Query {") {
		t.Error("the schema's own types were written alongside the introspection ones")
	}
}

// Writing a schema out, reading it back and writing it again has to give the
// same text. That is what makes the output usable as a record of a schema: it
// does not drift each time it passes through.
func TestPrintSchema_IsStableAcrossARoundTrip(t *testing.T) {
	sources := []string{
		`type Query { a: String }`,
		`
			"A schema."
			schema { query: Root }
			type Root { a: String }
		`,
		`
			scalar DateTime @specifiedBy(url: "https://example.com")
			interface Node { id: ID! }
			interface Named implements Node { id: ID! name: String }
			type User implements Node & Named {
				"The identifier."
				id: ID!
				name: String
				old: String @deprecated(reason: "Use name.")
				bare: String @deprecated
			}
			type Photo { url: String }
			union Media = User | Photo
			enum Colour { RED "Faded." PUCE @deprecated }
			input Filter @oneOf { byId: ID byName: String }
			input Paging { limit: Int = 10 after: String = null nested: Filter }
			directive @auth(role: String! = "user") repeatable on FIELD_DEFINITION | OBJECT
			type Query {
				node: Node
				media: Media
				search(
					"What to look for."
					filter: Filter
					paging: Paging
				): [User!]!
				when: DateTime
			}
		`,
	}

	for i, sdl := range sources {
		t.Run(strings.Fields(sdl + " ")[0], func(t *testing.T) {
			once := printBuilt(t, sdl)
			twice := printBuilt(t, once)
			if once != twice {
				t.Errorf("case %d drifted on the second pass:\nfirst:\n%s\n\nsecond:\n%s", i, once, twice)
			}
		})
	}
}

// The real test is a schema written by someone else. Reading the GitHub schema
// and writing it back has to give text that reads back as the same schema and
// writes out identically, which exercises every construct at once.
func TestPrintSchema_GitHubSchemaRoundTrip(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "language", "testdata", "github-schema.graphql"))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}

	first := mustBuild(t, string(body))
	printed := utilities.PrintSchema(first)

	second, err := utilities.BuildSchema(printed)
	if err != nil {
		t.Fatalf("reading back what was written: %v", err)
	}
	if err := schema.AssertValidSchema(second); err != nil {
		t.Fatalf("the schema read back is not sound:\n%v", err)
	}

	// The same set of types survives.
	if got, want := len(second.Types()), len(first.Types()); got != want {
		t.Errorf("%d types after the round trip, want %d", got, want)
	}
	for _, typ := range first.Types() {
		if second.Type(typ.Name()) == nil {
			t.Errorf("%s was lost", typ.Name())
		}
	}

	// And writing it again gives the same text.
	again := utilities.PrintSchema(second)
	if printed != again {
		t.Error("the text drifted on the second pass")
		for i := range min(len(printed), len(again)) {
			if printed[i] != again[i] {
				t.Fatalf("first difference at byte %d:\nfirst:  %q\nsecond: %q",
					i, excerpt(printed, i), excerpt(again, i))
			}
		}
	}
}

// excerpt shows the text around an offset, for reporting a difference.
func excerpt(s string, at int) string {
	start := max(at-60, 0)
	end := min(at+60, len(s))
	return s[start:end]
}

func BenchmarkPrintSchema_GitHubSchema(b *testing.B) {
	body, err := os.ReadFile(filepath.Join("..", "language", "testdata", "github-schema.graphql"))
	if err != nil {
		b.Fatal(err)
	}
	s, err := utilities.BuildSchema(string(body))
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	for b.Loop() {
		utilities.PrintSchema(s)
	}
}

// A printed schema follows the order its source declared, which is what makes
// the output readable as a record of what someone wrote rather than as the
// order a walk happened to reach things in.
func TestPrintSchema_KeepsDeclarationOrder(t *testing.T) {
	got := printBuilt(t, `
		type Zebra { a: String }
		type Query { z: Zebra a: Apple }
		type Apple { a: String }
		enum Middle { A }
		scalar Last
	`)
	want := []string{"type Zebra", "type Query", "type Apple", "enum Middle", "scalar Last"}
	at := -1
	for _, header := range want {
		found := strings.Index(got, header)
		if found < 0 {
			t.Fatalf("the output has no %q:\n%s", header, got)
		}
		if found < at {
			t.Fatalf("%q is out of the order it was declared in:\n%s", header, got)
		}
		at = found
	}
}
