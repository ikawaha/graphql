package utilities_test

// Ported from graphql-js src/utilities/__tests__/extendSchema-test.ts.
//
// Each case takes a schema, extends it, and compares the definitions the
// extension added: what the extended schema prints that the original did not.

import (
	"strings"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

func TestPortedExtendSchema(t *testing.T) {
	known := map[string]string{}

	for _, tt := range []struct{ name, sdl, extension, want string }{
		{
			name: `extends objects by adding new fields`,
			sdl: `type Query {
  someObject: SomeObject
}

type SomeObject implements AnotherInterface & SomeInterface {
  self: SomeObject
  tree: [SomeObject]!
  """Old field description."""
  oldField: String
}

interface SomeInterface {
  self: SomeInterface
}

interface AnotherInterface {
  self: SomeObject
}`,
			extension: `extend type SomeObject {
  """New field description."""
  newField(arg: Boolean): String
}`,
			want: `type SomeObject implements AnotherInterface & SomeInterface {
  self: SomeObject
  tree: [SomeObject]!
  """Old field description."""
  oldField: String
  """New field description."""
  newField(arg: Boolean): String
}`,
		},
		{
			name: `extends enums by adding new values`,
			sdl: `type Query {
  someEnum(arg: SomeEnum): SomeEnum
}

directive @foo(arg: SomeEnum) on SCHEMA

enum SomeEnum {
  """Old value description."""
  OLD_VALUE
}`,
			extension: `extend enum SomeEnum {
  """New value description."""
  NEW_VALUE
}`,
			want: `enum SomeEnum {
  """Old value description."""
  OLD_VALUE
  """New value description."""
  NEW_VALUE
}`,
		},
		{
			name: `extends unions by adding new types`,
			sdl: `type Query {
  someUnion: SomeUnion
}

union SomeUnion = Foo | Biz

type Foo { foo: String }
type Biz { biz: String }
type Bar { bar: String }`,
			extension: `extend union SomeUnion = Bar
`,
			want: `union SomeUnion = Foo | Biz | Bar
`,
		},
		{
			name: `allows extension of union by adding itself`,
			sdl: `union SomeUnion
`,
			extension: `extend union SomeUnion = SomeUnion
`,
			want: `union SomeUnion = SomeUnion
`,
		},
		{
			name: `extends inputs by adding new fields`,
			sdl: `type Query {
  someInput(arg: SomeInput): String
}

directive @foo(arg: SomeInput) on SCHEMA

input SomeInput {
  """Old field description."""
  oldField: String
}`,
			extension: `extend input SomeInput {
  """New field description."""
  newField: String
}`,
			want: `input SomeInput {
  """Old field description."""
  oldField: String
  """New field description."""
  newField: String
}`,
		},
		{
			name: `extends objects by adding new fields with arguments`,
			sdl: `type SomeObject

type Query {
  someObject: SomeObject
}`,
			extension: `input NewInputObj {
  field1: Int
  field2: [Float]
  field3: String!
}

extend type SomeObject {
  newField(arg1: String, arg2: NewInputObj!): String
}`,
			want: `type SomeObject {
  newField(arg1: String, arg2: NewInputObj!): String
}

input NewInputObj {
  field1: Int
  field2: [Float]
  field3: String!
}`,
		},
		{
			name: `extends objects by adding new fields with existing types`,
			sdl: `type Query {
  someObject: SomeObject
}

type SomeObject
enum SomeEnum { VALUE }`,
			extension: `extend type SomeObject {
  newField(arg1: SomeEnum!): SomeEnum
}`,
			want: `type SomeObject {
  newField(arg1: SomeEnum!): SomeEnum
}`,
		},
		{
			name: `extends objects by adding implemented interfaces`,
			sdl: `type Query {
  someObject: SomeObject
}

type SomeObject {
  foo: String
}

interface SomeInterface {
  foo: String
}`,
			extension: `extend type SomeObject implements SomeInterface
`,
			want: `type SomeObject implements SomeInterface {
  foo: String
}`,
		},
		{
			name: `extends objects by including new types`,
			sdl: `type Query {
  someObject: SomeObject
}

type SomeObject {
  oldField: String
}`,
			extension: `enum NewEnum {
  VALUE
}

interface NewInterface {
  baz: String
}

type NewObject implements NewInterface {
  baz: String
}

scalar NewScalar

union NewUnion = NewObject
extend type SomeObject {
  newObject: NewObject
  newInterface: NewInterface
  newUnion: NewUnion
  newScalar: NewScalar
  newEnum: NewEnum
  newTree: [SomeObject]!
}`,
			want: `type SomeObject {
  oldField: String
  newObject: NewObject
  newInterface: NewInterface
  newUnion: NewUnion
  newScalar: NewScalar
  newEnum: NewEnum
  newTree: [SomeObject]!
}

enum NewEnum {
  VALUE
}

interface NewInterface {
  baz: String
}

type NewObject implements NewInterface {
  baz: String
}

scalar NewScalar

union NewUnion = NewObject`,
		},
		{
			name: `extends objects by adding implemented new interfaces`,
			sdl: `type Query {
  someObject: SomeObject
}

type SomeObject implements OldInterface {
  oldField: String
}

interface OldInterface {
  oldField: String
}`,
			extension: `extend type SomeObject implements NewInterface {
  newField: String
}

interface NewInterface {
  newField: String
}`,
			want: `type SomeObject implements OldInterface & NewInterface {
  oldField: String
  newField: String
}

interface NewInterface {
  newField: String
}`,
		},
		{
			name: `extends different types multiple times`,
			sdl: `type Query {
  someScalar: SomeScalar
  someObject(someInput: SomeInput): SomeObject
  someInterface: SomeInterface
  someEnum: SomeEnum
  someUnion: SomeUnion
}

scalar SomeScalar

type SomeObject implements SomeInterface {
  oldField: String
}

interface SomeInterface {
  oldField: String
}

enum SomeEnum {
  OLD_VALUE
}

union SomeUnion = SomeObject

input SomeInput {
  oldField: String
}`,
			extension: `scalar NewScalar

scalar AnotherNewScalar

type NewObject {
  foo: String
}

type AnotherNewObject {
  foo: String
}

interface NewInterface {
  newField: String
}

interface AnotherNewInterface {
  anotherNewField: String
}

extend scalar SomeScalar @specifiedBy(url: "http://example.com/foo_spec")

extend type SomeObject implements NewInterface {
  newField: String
}

extend type SomeObject implements AnotherNewInterface {
  anotherNewField: String
}

extend enum SomeEnum {
  NEW_VALUE
}

extend enum SomeEnum {
  ANOTHER_NEW_VALUE
}

extend union SomeUnion = NewObject

extend union SomeUnion = AnotherNewObject

extend input SomeInput {
  newField: String
}

extend input SomeInput {
  anotherNewField: String
}`,
			want: `scalar SomeScalar @specifiedBy(url: "http://example.com/foo_spec")

type SomeObject implements SomeInterface & NewInterface & AnotherNewInterface {
  oldField: String
  newField: String
  anotherNewField: String
}

enum SomeEnum {
  OLD_VALUE
  NEW_VALUE
  ANOTHER_NEW_VALUE
}

union SomeUnion = SomeObject | NewObject | AnotherNewObject

input SomeInput {
  oldField: String
  newField: String
  anotherNewField: String
}

scalar NewScalar

scalar AnotherNewScalar

type NewObject {
  foo: String
}

type AnotherNewObject {
  foo: String
}

interface NewInterface {
  newField: String
}

interface AnotherNewInterface {
  anotherNewField: String
}`,
		},
		{
			name: `extends interfaces by adding new fields`,
			sdl: `interface SomeInterface {
  oldField: String
}

interface AnotherInterface implements SomeInterface {
  oldField: String
}

type SomeObject implements SomeInterface & AnotherInterface {
  oldField: String
}

type Query {
  someInterface: SomeInterface
}`,
			extension: `extend interface SomeInterface {
  newField: String
}

extend interface AnotherInterface {
  newField: String
}

extend type SomeObject {
  newField: String
}`,
			want: `interface SomeInterface {
  oldField: String
  newField: String
}

interface AnotherInterface implements SomeInterface {
  oldField: String
  newField: String
}

type SomeObject implements SomeInterface & AnotherInterface {
  oldField: String
  newField: String
}`,
		},
		{
			name: `extends interfaces by adding new implemented interfaces`,
			sdl: `interface SomeInterface {
  oldField: String
}

interface AnotherInterface implements SomeInterface {
  oldField: String
}

type SomeObject implements SomeInterface & AnotherInterface {
  oldField: String
}

type Query {
  someInterface: SomeInterface
}`,
			extension: `interface NewInterface {
  newField: String
}

extend interface AnotherInterface implements NewInterface {
  newField: String
}

extend type SomeObject implements NewInterface {
  newField: String
}`,
			want: `interface AnotherInterface implements SomeInterface & NewInterface {
  oldField: String
  newField: String
}

type SomeObject implements SomeInterface & AnotherInterface & NewInterface {
  oldField: String
  newField: String
}

interface NewInterface {
  newField: String
}`,
		},
		{
			name: `allows extension of interface with missing Object fields`,
			sdl: `type Query {
  someInterface: SomeInterface
}

type SomeObject implements SomeInterface {
  oldField: SomeInterface
}

interface SomeInterface {
  oldField: SomeInterface
}`,
			extension: `extend interface SomeInterface {
  newField: String
}`,
			want: `interface SomeInterface {
  oldField: SomeInterface
  newField: String
}`,
		},
		{
			name: `extends interfaces multiple times`,
			sdl: `type Query {
  someInterface: SomeInterface
}

interface SomeInterface {
  some: SomeInterface
}`,
			extension: `extend interface SomeInterface {
  newFieldA: Int
}

extend interface SomeInterface {
  newFieldB(test: Boolean): String
}`,
			want: `interface SomeInterface {
  some: SomeInterface
  newFieldA: Int
  newFieldB(test: Boolean): String
}`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, err := utilities.BuildSchema(tt.sdl)
			if err != nil {
				t.Fatalf("building the schema: %v", err)
			}
			why, listed := known[tt.name]
			extended, err := utilities.ExtendSchemaSource(s, tt.extension)
			if err != nil {
				// A schema Go cannot hold is refused rather than built and
				// then reported on, which is the recorded divergence.
				if listed {
					t.Logf("known divergence: %s", why)
					return
				}
				t.Fatalf("extending the schema: %v", err)
			}
			got := added(t, s, extended)
			if listed {
				if got == strings.TrimRight(tt.want, "\n") {
					t.Errorf("this case now matches graphql-js; remove it from the known divergences (%s)", why)
				} else {
					t.Logf("known divergence: %s", why)
				}
				return
			}
			if got != strings.TrimRight(tt.want, "\n") {
				t.Errorf("added\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

// added is graphql-js's expectSchemaChanges: the definitions the extended
// schema prints that the original did not.
func added(t *testing.T, before, after *schema.Schema) string {
	t.Helper()
	was := map[string]bool{}
	for _, def := range definitionsOf(t, utilities.PrintSchema(before)) {
		was[def] = true
	}
	var fresh []string
	for _, def := range definitionsOf(t, utilities.PrintSchema(after)) {
		if !was[def] {
			fresh = append(fresh, def)
		}
	}
	return strings.Join(fresh, "\n\n")
}

// definitionsOf parses a printed schema and prints each definition back on its
// own, so that two schemas can be compared a definition at a time.
func definitionsOf(t *testing.T, sdl string) []string {
	t.Helper()
	doc, err := language.ParseString(sdl)
	if err != nil {
		t.Fatalf("reading a printed schema: %v", err)
	}
	out := make([]string, 0, len(doc.Definitions))
	for _, def := range doc.Definitions {
		out = append(out, language.Print(def))
	}
	return out
}

// TestPortedBuildASTSchema_DoNotOverrideStandardTypes is graphql-js's
// "Do not override standard types": a document that defines a type of a
// standard name does not get to replace it. The standard one stands and what
// was written is dropped, so a schema is still built.
func TestPortedBuildASTSchema_DoNotOverrideStandardTypes(t *testing.T) {
	built, err := utilities.BuildSchema("scalar ID\n\nscalar __Schema\n\ntype Query { a: ID }")
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	if got := built.Type("ID"); got != schema.Type(schema.ID) {
		t.Error("ID is not the built-in scalar")
	}
	if got := built.Type("__Schema"); got != schema.Type(schema.SchemaIntrospectionType) {
		t.Error("__Schema is not the built-in introspection type")
	}
	// Neither is printed: both belong to every schema already.
	if got, want := utilities.PrintSchema(built), "type Query {\n  a: ID\n}"; got != want {
		t.Errorf("printed\n%s\nwant\n%s", got, want)
	}
}
