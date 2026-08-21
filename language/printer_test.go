package language

import (
	"strings"
	"testing"
)

// printParsed parses a document and prints it back.
func printParsed(t *testing.T, body string, opts ...ParseOption) string {
	t.Helper()
	return Print(mustParse(t, body, opts...))
}

func TestPrint_Nil(t *testing.T) {
	if got := Print(nil); got != "" {
		t.Errorf("Print(nil) = %q, want empty", got)
	}
}

func TestPrint_ShorthandQuery(t *testing.T) {
	got := printParsed(t, "{hero}")
	want := "{\n  hero\n}"
	if got != want {
		t.Errorf("Print() =\n%s\nwant\n%s", got, want)
	}
}

// A named query, or one with variables or directives, cannot use the
// shorthand form and has to print its prefix.
func TestPrint_OperationPrefix(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"anonymous query", "{a}", "{\n  a\n}"},
		{"named query", "query Q {a}", "query Q {\n  a\n}"},
		{"mutation", "mutation {a}", "mutation {\n  a\n}"},
		{"anonymous with variables", "query ($v: Int) {a}", "query ($v: Int) {\n  a\n}"},
		{"anonymous with directives", "query @d {a}", "query @d {\n  a\n}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := printParsed(t, tt.body); got != tt.want {
				t.Errorf("Print() =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestPrint_Values(t *testing.T) {
	tests := []struct {
		body string
		want string
	}{
		{"1", "1"},
		{"-1", "-1"},
		{"1.5", "1.5"},
		{"true", "true"},
		{"false", "false"},
		{"null", "null"},
		{"JEDI", "JEDI"},
		{"$v", "$v"},
		{`"s"`, `"s"`},
		{"[1, 2]", "[1, 2]"},
		{"[]", "[]"},
		{"{a: 1}", "{ a: 1 }"},
		{"{}", "{  }"},
		{"[[1], {a: [2]}]", "[[1], { a: [2] }]"},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			v, err := ParseValue(NewSource(tt.body))
			if err != nil {
				t.Fatalf("ParseValue(%q): %v", tt.body, err)
			}
			if got := Print(v); got != tt.want {
				t.Errorf("Print() = %q, want %q", got, tt.want)
			}
		})
	}
}

// A string prints back in the form it was written in, because the AST records
// which form that was.
func TestPrint_StringForms(t *testing.T) {
	tests := []struct {
		body string
		want string
	}{
		{`"simple"`, `"simple"`},
		{`"has \" quote"`, `"has \" quote"`},
		{`"""block"""`, `"""block"""`},
		{"\"\"\"multi\nline\"\"\"", "\"\"\"\nmulti\nline\n\"\"\""},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			v, err := ParseValue(NewSource(tt.body))
			if err != nil {
				t.Fatalf("ParseValue(%q): %v", tt.body, err)
			}
			if got := Print(v); got != tt.want {
				t.Errorf("Print() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrint_Types(t *testing.T) {
	for _, body := range []string{"Int", "[Int]", "Int!", "[Int!]!", "[[Int]]"} {
		t.Run(body, func(t *testing.T) {
			typ, err := ParseType(NewSource(body))
			if err != nil {
				t.Fatalf("ParseType(%q): %v", body, err)
			}
			if got := Print(typ); got != body {
				t.Errorf("Print() = %q, want %q", got, body)
			}
		})
	}
}

func TestPrint_SchemaCoordinates(t *testing.T) {
	for _, body := range []string{
		"Type", "Type.field", "Type.field(arg:)", "@directive", "@directive(arg:)",
		"__Type", "Type.__metafield", "Type.__metafield(arg:)",
	} {
		t.Run(body, func(t *testing.T) {
			c, err := ParseSchemaCoordinate(NewSource(body))
			if err != nil {
				t.Fatalf("ParseSchemaCoordinate(%q): %v", body, err)
			}
			if got := Print(c); got != body {
				t.Errorf("Print() = %q, want %q", got, body)
			}
		})
	}
}

func TestPrint_Definitions(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"scalar", "scalar S", "scalar S"},
		{"scalar with directive", "scalar S @d", "scalar S @d"},
		{"empty object type", "type T", "type T"},
		{"object type", "type T {f: Int}", "type T {\n  f: Int\n}"},
		{
			name: "object type with interfaces",
			body: "type T implements A&B {f: Int}",
			want: "type T implements A & B {\n  f: Int\n}",
		},
		{"union", "union U = A|B", "union U = A | B"},
		{"union with leading delimiter", "union U = |A|B", "union U = A | B"},
		{"enum", "enum E {A B}", "enum E {\n  A\n  B\n}"},
		{"input", "input In {a: Int = 1}", "input In {\n  a: Int = 1\n}"},
		{
			name: "directive definition",
			body: "directive @d(a: Int) repeatable on FIELD|OBJECT",
			want: "directive @d(a: Int) repeatable on FIELD | OBJECT",
		},
		{
			name: "field with arguments",
			body: "type T {f(a: Int = 1, b: String): Int!}",
			want: "type T {\n  f(a: Int = 1, b: String): Int!\n}",
		},
		{"schema", "schema {query: Q}", "schema {\n  query: Q\n}"},
		{"extension", "extend type T {f: Int}", "extend type T {\n  f: Int\n}"},
		{"directive extension", "extend directive @d @other", "extend directive @d @other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := printParsed(t, tt.body); got != tt.want {
				t.Errorf("Print() =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestPrint_Descriptions(t *testing.T) {
	got := printParsed(t, `"A type." type T { "A field." f: Int }`)
	want := "\"A type.\"\ntype T {\n  \"A field.\"\n  f: Int\n}"
	if got != want {
		t.Errorf("Print() =\n%s\nwant\n%s", got, want)
	}
}

func TestPrint_Fragments(t *testing.T) {
	got := printParsed(t, "{...A ...on T{a} ...@d{b}} fragment A on T{c}")
	want := strings.Join([]string{
		"{",
		"  ...A",
		"  ... on T {",
		"    a",
		"  }",
		"  ... @d {",
		"    b",
		"  }",
		"}",
		"",
		"fragment A on T {",
		"  c",
		"}",
	}, "\n")
	if got != want {
		t.Errorf("Print() =\n%s\nwant\n%s", got, want)
	}
}

func TestPrint_DefinitionsSeparatedByBlankLine(t *testing.T) {
	got := printParsed(t, "scalar A scalar B")
	want := "scalar A\n\nscalar B"
	if got != want {
		t.Errorf("Print() = %q, want %q", got, want)
	}
}

// A long argument list is broken across lines so that it stays readable.
func TestPrint_LongArgumentListWraps(t *testing.T) {
	long := strings.Repeat("x", 60)
	got := printParsed(t, "{ field(argument: \""+long+"\", other: 1) }")
	if !strings.Contains(got, "(\n") {
		t.Errorf("a long argument list was not wrapped:\n%s", got)
	}

	short := printParsed(t, "{ field(a: 1, b: 2) }")
	if strings.Contains(short, "(\n") {
		t.Errorf("a short argument list was wrapped:\n%s", short)
	}
}

// An argument that carries a description is itself multi-line, which puts
// every argument of that definition on its own line.
func TestPrint_ArgumentDescriptionsForceOnePerLine(t *testing.T) {
	got := printParsed(t, `type T { f("desc" a: Int, b: Int): Int }`)
	want := strings.Join([]string{
		"type T {",
		"  f(",
		`    "desc"`,
		"    a: Int",
		"    b: Int",
		"  ): Int",
		"}",
	}, "\n")
	if got != want {
		t.Errorf("Print() =\n%s\nwant\n%s", got, want)
	}
}

// Printing is idempotent: formatting an already formatted document changes
// nothing. This is the property that makes the printer safe to run repeatedly,
// and it is what a round trip through the parser has to preserve.
func TestPrint_IsIdempotent(t *testing.T) {
	bodies := []string{
		"{hero}",
		"query Q($v: Int = 1) @d {a: b(c: [1, {d: null}]) {e}}",
		"{...A ...on T{a}} fragment A on T{b}",
		`"desc" type T implements A & B @d {f(a: Int = 1): [Int!]!}`,
		"schema {query: Q mutation: M}",
		"union U = A | B enum E {A B} input In {a: Int}",
		"extend type T @d extend union U = C",
		"directive @d(a: Int) repeatable on FIELD | OBJECT",
		"{f(a: \"" + strings.Repeat("x", 100) + "\")}",
		"{f(a: [" + strings.Repeat("1, ", 40) + "1])}",
	}
	for _, body := range bodies {
		t.Run(body[:min(len(body), 40)], func(t *testing.T) {
			once := printParsed(t, body)
			twice := printParsed(t, once)
			if once != twice {
				t.Errorf("printing twice differs:\nfirst:\n%s\nsecond:\n%s", once, twice)
			}
		})
	}
}

// The whole point of the printer is that its output parses back to the same
// document. Running it over the fixtures checks that against real input,
// including every construct the kitchen sink covers.
func TestPrint_RoundTripsFixtures(t *testing.T) {
	for _, name := range []string{"kitchen-sink.graphql", "github-schema.graphql"} {
		t.Run(name, func(t *testing.T) {
			source := NewSource(readFixture(t, name), SourceName(name))
			doc, err := Parse(source)
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}

			once := Print(doc)
			reparsed, err := Parse(NewSource(once, SourceName(name+" (printed)")))
			if err != nil {
				t.Fatalf("reparsing the printed document: %v", err)
			}
			twice := Print(reparsed)
			if once != twice {
				t.Error("printing the reparsed document produced different output")
			}
			if got, want := len(reparsed.Definitions), len(doc.Definitions); got != want {
				t.Errorf("reparsed document has %d definitions, want %d", got, want)
			}
		})
	}
}

// The kitchen sink uses the experimental fragment syntax too, which has to
// survive a round trip when the option is on.
func TestPrint_RoundTripsExperimentalFragments(t *testing.T) {
	const body = `
		{ t { ...A(x: true) } }
		fragment A($x: Boolean = false) on T { f }
	`
	once := printParsed(t, body, ExperimentalFragmentArguments())
	twice := printParsed(t, once, ExperimentalFragmentArguments())
	if once != twice {
		t.Errorf("printing twice differs:\nfirst:\n%s\nsecond:\n%s", once, twice)
	}
	for _, want := range []string{"...A(x: true)", "fragment A($x: Boolean = false) on T"} {
		if !strings.Contains(once, want) {
			t.Errorf("output does not contain %q:\n%s", want, once)
		}
	}
}

func BenchmarkPrint_GitHubSchema(b *testing.B) {
	body, err := readFixtureBytes("github-schema.graphql")
	if err != nil {
		b.Fatal(err)
	}
	doc, err := Parse(NewSource(string(body)))
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	for b.Loop() {
		Print(doc)
	}
}
