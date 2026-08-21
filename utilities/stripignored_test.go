package utilities_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/utilities"
)

func TestStripIgnoredCharacters(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"nothing", "", ""},
		{"whitespace", "  { field }  ", "{field}"},
		{"line breaks", "{\n  field\n}", "{field}"},
		{"commas", "{ a, b, c }", "{a b c}"},
		{"comments", "{ # what this is\n  field }", "{field}"},
		// Two names run together would lex as one, so a space stays between.
		{"names stay apart", "query Foo { bar }", "query Foo{bar}"},
		{"a name and a number", "f(a: 1 b: 2)", "f(a:1 b:2)"},
		{"punctuation needs nothing", "{a{b{c}}}", "{a{b{c}}}"},
		// A spread is punctuation, so nothing is needed after it.
		{"a spread before a name", "{ ... on Foo { a } }", "{...on Foo{a}}"},
		// Two strings run together would begin a block string.
		{"two strings", `{ f(a: "" b: "") }`, `{f(a:"" b:"")}`},
		{"strings are left alone", `{ f(a: "  spaces  ") }`, `{f(a:"  spaces  ")}`},
		{"a string after a name", `f(a:"x")`, `f(a:"x")`},
		// An argument list is written without the commas the author used.
		{"an argument list", "field(a: 1, b: 2, c: 3)", "field(a:1 b:2 c:3)"},
		{"SDL", "type Query {\n  # a field\n  a: String\n}", "type Query{a:String}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := utilities.StripIgnoredCharacters(tt.in)
			if err != nil {
				t.Fatalf("stripping %q: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("= %q, want %q", got, tt.want)
			}
		})
	}
}

// A block string is rewritten to the shortest form with the same value, which
// is the one place the text inside a token changes.
func TestStripIgnoredCharacters_BlockStrings(t *testing.T) {
	tests := []struct{ in, want string }{
		{`"""   x   """`, `"""   x   """`},
		{"\"\"\"\n  hello\n  world\n\"\"\"", "\"\"\"hello\nworld\"\"\""},
		{"\"\"\"\n\n  x\n\n\"\"\"", `"""x"""`},
		// The value has to survive, however the layout changes.
		{"{ f(a: \"\"\"\n  kept\n\"\"\") }", "{f(a:\"\"\"kept\"\"\")}"},
	}
	for _, tt := range tests {
		got, err := utilities.StripIgnoredCharacters(tt.in)
		if err != nil {
			t.Fatalf("stripping %q: %v", tt.in, err)
		}
		if got != tt.want {
			t.Errorf("strip(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	// The shape of the text may change; what it says may not.
	t.Run("the value survives", func(t *testing.T) {
		for _, written := range []string{
			"\"\"\"\n  hello\n  world\n\"\"\"",
			"\"\"\"\n\n  indented\n    further\n\n\"\"\"",
			`"""   leading and trailing spaces   """`,
			"\"\"\"has a \\\"\"\" inside\"\"\"",
			`"""x"""`,
		} {
			stripped, err := utilities.StripIgnoredCharacters(written)
			if err != nil {
				t.Fatalf("stripping %q: %v", written, err)
			}
			before, err := language.ParseValue(language.NewSource(written))
			if err != nil {
				t.Fatalf("parsing %q: %v", written, err)
			}
			after, err := language.ParseValue(language.NewSource(stripped))
			if err != nil {
				t.Fatalf("parsing the stripped %q: %v", stripped, err)
			}
			was := before.(*language.StringValue).Value
			is := after.(*language.StringValue).Value
			if was != is {
				t.Errorf("stripping %q changed the value from %q to %q", written, was, is)
			}
		}
	})
}

// The whole point is that what comes back means the same thing, so it has to
// parse to the same document.
func TestStripIgnoredCharacters_MeansTheSame(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "language", "testdata", "github-schema.graphql"))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	original := string(body)

	stripped, err := utilities.StripIgnoredCharacters(original)
	if err != nil {
		t.Fatalf("stripping: %v", err)
	}
	if len(stripped) >= len(original) {
		t.Errorf("stripping made it no shorter: %d bytes from %d", len(stripped), len(original))
	}

	// The two build the same schema, which is the strongest statement of "the
	// same thing" available.
	before, err := utilities.BuildSchema(original)
	if err != nil {
		t.Fatalf("building from the original: %v", err)
	}
	after, err := utilities.BuildSchema(stripped)
	if err != nil {
		t.Fatalf("building from the stripped text: %v", err)
	}
	if got, want := utilities.PrintSchema(after), utilities.PrintSchema(before); got != want {
		t.Error("the stripped text describes a different schema")
	}

	// And stripping it again changes nothing, so the result is settled.
	twice, err := utilities.StripIgnoredCharacters(stripped)
	if err != nil {
		t.Fatalf("stripping again: %v", err)
	}
	if twice != stripped {
		t.Error("stripping twice gave something different from stripping once")
	}
}

// Text that is not made of valid tokens is reported rather than half-stripped.
func TestStripIgnoredCharacters_Invalid(t *testing.T) {
	for _, body := range []string{`{ "unterminated }`, "{ ~ }", `"""unterminated`} {
		if _, err := utilities.StripIgnoredCharacters(body); err == nil {
			t.Errorf("stripping %q succeeded", body)
		}
	}
}

// A document that only differs in layout strips to the same text, which is
// what makes this usable as a cache key.
func TestStripIgnoredCharacters_SameRequestSameText(t *testing.T) {
	a, err := utilities.StripIgnoredCharacters("{\n  user(id: 4) {\n    name\n  }\n}")
	if err != nil {
		t.Fatal(err)
	}
	b, err := utilities.StripIgnoredCharacters("  { user(id:4){name} }  # comment")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("the same request stripped to %q and %q", a, b)
	}
	if _, err := language.ParseString(a); err != nil {
		t.Errorf("the stripped text does not parse: %v", err)
	}
}
