package language_test

// Ported from graphql-js src/language/__tests__/schema-parser-test.ts. Its
// error cases are in ported_parser_test.go; what is left is the shape each
// definition parses to, asserted here by writing the tree back out. The
// printer is canonical — a leading `&` or `|` is not written back — so a
// document that reads correctly is a document that prints as its tidy form.

import (
	"testing"

	"github.com/ikawaha/graphql/language"
)

func TestPortedSchemaParser_Shapes(t *testing.T) {
	for _, tt := range []struct{ name, in, want string }{
		{
			name: "simple type",
			in:   "type Hello {\n  world: String\n}",
			want: "type Hello {\n  world: String\n}",
		},
		{
			name: "type with a description",
			in:   "\"Description\"\ntype Hello {\n  world: String\n}",
			want: "\"Description\"\ntype Hello {\n  world: String\n}",
		},
		{
			name: "type with a description written across lines",
			in:   "\"\"\"\nDescription\n\"\"\"\n# Even with comments between them\ntype Hello {\n  world: String\n}",
			want: "\"\"\"Description\"\"\"\ntype Hello {\n  world: String\n}",
		},
		{
			name: "schema with a description",
			in:   "\"Description\"\nschema {\n  query: Foo\n}",
			want: "\"Description\"\nschema {\n  query: Foo\n}",
		},
		{
			name: "a simple extension",
			in:   "extend type Hello {\n  world: String\n}",
			want: "extend type Hello {\n  world: String\n}",
		},
		{
			name: "an object extension with no fields",
			in:   "extend type Hello implements Greeting",
			want: "extend type Hello implements Greeting",
		},
		{
			name: "an interface extension with no fields",
			in:   "extend interface Hello implements Greeting",
			want: "extend interface Hello implements Greeting",
		},
		{
			name: "an object extension with no fields, followed by another",
			in:   "extend type Hello implements Greeting\n\nextend type Hello implements SecondGreeting",
			want: "extend type Hello implements Greeting\n\nextend type Hello implements SecondGreeting",
		},
		{
			name: "a schema extension",
			in:   "extend schema {\n  mutation: Mutation\n}",
			want: "extend schema {\n  mutation: Mutation\n}",
		},
		{
			name: "a schema extension with only directives",
			in:   "extend schema @directive",
			want: "extend schema @directive",
		},
		{
			name: "a non-null type",
			in:   "type Hello {\n  world: String!\n}",
			want: "type Hello {\n  world: String!\n}",
		},
		{
			name: "an interface inheriting an interface",
			in:   "interface Hello implements World {\n  field: String\n}",
			want: "interface Hello implements World {\n  field: String\n}",
		},
		{
			name: "a type inheriting an interface",
			in:   "type Hello implements World {\n  field: String\n}",
			want: "type Hello implements World {\n  field: String\n}",
		},
		{
			name: "a type inheriting several interfaces",
			in:   "type Hello implements Wo & rld {\n  field: String\n}",
			want: "type Hello implements Wo & rld {\n  field: String\n}",
		},
		{
			name: "an interface inheriting several interfaces",
			in:   "interface Hello implements Wo & rld {\n  field: String\n}",
			want: "interface Hello implements Wo & rld {\n  field: String\n}",
		},
		{
			name: "a type whose implements clause has a leading ampersand",
			in:   "type Hello implements & Wo & rld {\n  field: String\n}",
			want: "type Hello implements Wo & rld {\n  field: String\n}",
		},
		{
			name: "an interface whose implements clause has a leading ampersand",
			in:   "interface Hello implements & Wo & rld {\n  field: String\n}",
			want: "interface Hello implements Wo & rld {\n  field: String\n}",
		},
		{
			name: "an enum with one member",
			in:   "enum Hello {\n  WORLD\n}",
			want: "enum Hello {\n  WORLD\n}",
		},
		{
			name: "an enum with two members",
			in:   "enum Hello {\n  WO\n  RLD\n}",
			want: "enum Hello {\n  WO\n  RLD\n}",
		},
		{
			name: "a simple interface",
			in:   "interface Hello {\n  world: String\n}",
			want: "interface Hello {\n  world: String\n}",
		},
		{
			name: "a field with an argument",
			in:   "type Hello {\n  world(flag: Boolean): String\n}",
			want: "type Hello {\n  world(flag: Boolean): String\n}",
		},
		{
			name: "a field with an argument that has a default",
			in:   "type Hello {\n  world(flag: Boolean = true): String\n}",
			want: "type Hello {\n  world(flag: Boolean = true): String\n}",
		},
		{
			name: "a field with a list argument",
			in:   "type Hello {\n  world(things: [String]): String\n}",
			want: "type Hello {\n  world(things: [String]): String\n}",
		},
		{
			name: "a field with two arguments",
			in:   "type Hello {\n  world(argOne: Boolean, argTwo: Int): String\n}",
			want: "type Hello {\n  world(argOne: Boolean, argTwo: Int): String\n}",
		},
		{
			name: "a union of one type",
			in:   "union Hello = World",
			want: "union Hello = World",
		},
		{
			name: "a union of two types",
			in:   "union Hello = Wo | Rld",
			want: "union Hello = Wo | Rld",
		},
		{
			name: "a union with a leading pipe",
			in:   "union Hello = | Wo | Rld",
			want: "union Hello = Wo | Rld",
		},
		{
			name: "a scalar",
			in:   "scalar Hello",
			want: "scalar Hello",
		},
		{
			name: "an input object",
			in:   "input Hello {\n  world: String\n}",
			want: "input Hello {\n  world: String\n}",
		},
		{
			name: "a directive definition",
			in:   "directive @foo(arg: Int) on FIELD",
			want: "directive @foo(arg: Int) on FIELD",
		},
		{
			name: "a repeatable directive definition",
			in:   "directive @foo(arg: Int) repeatable on FIELD",
			want: "directive @foo(arg: Int) repeatable on FIELD",
		},
		{
			name: "a directive extension",
			in:   "extend directive @foo @bar",
			want: "extend directive @foo @bar",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := language.ParseString(tt.in)
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}
			if got := language.Print(doc); got != tt.want {
				t.Errorf("wrote\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}
