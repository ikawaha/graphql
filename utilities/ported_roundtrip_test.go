package utilities_test

// Ported from graphql-js src/utilities/__tests__/buildASTSchema-test.ts: SDL
// read into a schema and printed back out should read the same.
//
// It is one assertion checking two things at once, which is why it is worth
// having: a schema that prints back as itself has been read correctly, and a
// printer that reproduces what it was given has printed correctly.

import (
	"testing"

	"github.com/ikawaha/graphql/utilities"
)

func TestPortedSchemaRoundTrip(t *testing.T) {
	runPortedCycles(t, knownCycleDivergences, []portedCycle{
		{
			name: `Empty type`,
			sdl:  `type EmptyType`,
		},
		{
			name: `Simple type`,
			sdl: `type Query {
  str: String
  int: Int
  float: Float
  id: ID
  bool: Boolean
}`,
		},
		{
			name: `With directives`,
			sdl: `directive @foo(arg: Int) on FIELD

directive @repeatableFoo(arg: Int) repeatable on FIELD`,
		},
		{
			name: `Supports descriptions`,
			sdl: `"""Do you agree that this is the most creative schema ever?"""
schema {
  query: Query
}

"""This is a directive"""
directive @foo(
  """It has an argument"""
  arg: Int
) on FIELD

"""Who knows what inside this scalar?"""
scalar MysteryScalar

"""This is a input object type"""
input FooInput {
  """It has a field"""
  field: Int
}

"""This is a interface type"""
interface Energy {
  """It also has a field"""
  str: String
}

"""There is nothing inside!"""
union BlackHole

"""With an enum"""
enum Color {
  RED

  """Not a creative color"""
  GREEN
  BLUE
}

"""What a great type"""
type Query {
  """And a field to boot"""
  str: String
}`,
		},
		{
			name: `Type modifiers`,
			sdl: `type Query {
  nonNullStr: String!
  listOfStrings: [String]
  listOfNonNullStrings: [String!]
  nonNullListOfStrings: [String]!
  nonNullListOfNonNullStrings: [String!]!
}`,
		},
		{
			name: `Recursive type`,
			sdl: `type Query {
  str: String
  recurse: Query
}`,
		},
		{
			name: `Two types circular`,
			sdl: `type TypeOne {
  str: String
  typeTwo: TypeTwo
}

type TypeTwo {
  str: String
  typeOne: TypeOne
}`,
		},
		{
			name: `Single argument field`,
			sdl: `type Query {
  str(int: Int): String
  floatToStr(float: Float): String
  idToStr(id: ID): String
  booleanToStr(bool: Boolean): String
  strToStr(bool: String): String
}`,
		},
		{
			name: `Simple type with multiple arguments`,
			sdl: `type Query {
  str(int: Int, bool: Boolean): String
}`,
		},
		{
			name: `Empty interface`,
			sdl:  `interface EmptyInterface`,
		},
		{
			name: `Simple type with interface`,
			sdl: `type Query implements WorldInterface {
  str: String
}

interface WorldInterface {
  str: String
}`,
		},
		{
			name: `Simple interface hierarchy`,
			sdl: `schema {
  query: Child
}

interface Child implements Parent {
  str: String
}

type Hello implements Parent & Child {
  str: String
}

interface Parent {
  str: String
}`,
		},
		{
			name: `Empty enum`,
			sdl:  `enum EmptyEnum`,
		},
		{
			name: `Simple output enum`,
			sdl: `enum Hello {
  WORLD
}

type Query {
  hello: Hello
}`,
		},
		{
			name: `Simple input enum`,
			sdl: `enum Hello {
  WORLD
}

type Query {
  str(hello: Hello): String
}`,
		},
		{
			name: `Multiple value enum`,
			sdl: `enum Hello {
  WO
  RLD
}

type Query {
  hello: Hello
}`,
		},
		{
			name: `Empty union`,
			sdl:  `union EmptyUnion`,
		},
		{
			name: `Simple Union`,
			sdl: `union Hello = World

type Query {
  hello: Hello
}

type World {
  str: String
}`,
		},
		{
			name: `Multiple Union`,
			sdl: `union Hello = WorldOne | WorldTwo

type Query {
  hello: Hello
}

type WorldOne {
  str: String
}

type WorldTwo {
  str: String
}`,
		},
		{
			name: `Custom Scalar`,
			sdl: `scalar CustomScalar

type Query {
  customScalar: CustomScalar
}`,
		},
		{
			name: `Empty Input Object`,
			sdl:  `input EmptyInputObject`,
		},
		{
			name: `Simple Input Object`,
			sdl: `input Input {
  int: Int
}

type Query {
  field(in: Input): String
}`,
		},
		{
			name: `Simple argument field with default`,
			sdl: `type Query {
  str(int: Int = 2): String
}`,
		},
		{
			name: `Custom scalar argument field with default`,
			sdl: `scalar CustomScalar

type Query {
  str(int: CustomScalar = 2): String
}`,
		},
		{
			name: `Simple type with mutation`,
			sdl: `schema {
  query: HelloScalars
  mutation: Mutation
}

type HelloScalars {
  str: String
  int: Int
  bool: Boolean
}

type Mutation {
  addHelloScalars(str: String, int: Int, bool: Boolean): HelloScalars
}`,
		},
		{
			name: `Simple type with subscription`,
			sdl: `schema {
  query: HelloScalars
  subscription: Subscription
}

type HelloScalars {
  str: String
  int: Int
  bool: Boolean
}

type Subscription {
  subscribeHelloScalars(str: String, int: Int, bool: Boolean): HelloScalars
}`,
		},
		{
			name: `Unreferenced type implementing referenced interface`,
			sdl: `type Concrete implements Interface {
  key: String
}

interface Interface {
  key: String
}

type Query {
  interface: Interface
}`,
		},
		{
			name: `Unreferenced interface implementing referenced interface`,
			sdl: `interface Child implements Parent {
  key: String
}

interface Parent {
  key: String
}

type Query {
  interfaceField: Parent
}`,
		},
		{
			name: `Unreferenced type implementing referenced union`,
			sdl: `type Concrete {
  key: String
}

type Query {
  union: Union
}

union Union = Concrete`,
		},
		{
			name: `Supports @deprecated`,
			sdl: `enum MyEnum {
  VALUE
  OLD_VALUE @deprecated
  OTHER_VALUE @deprecated(reason: "Terrible reasons")
}

input MyInput {
  oldInput: String @deprecated
  otherInput: String @deprecated(reason: "Use newInput")
  newInput: String
}

type Query {
  field1: String @deprecated
  field2: Int @deprecated(reason: "Because I said so")
  enum: MyEnum
  field3(oldArg: String @deprecated, arg: String): String
  field4(oldArg: String @deprecated(reason: "Why not?"), arg: String): String
  field5(arg: MyInput): String
}`,
		},
		{
			name: `Supports @specifiedBy`,
			sdl: `scalar Foo @specifiedBy(url: "https://example.com/foo_spec")

type Query {
  foo: Foo @deprecated
}`,
		},
	})
}

// portedCycle is one of graphql-js's round trips.
type portedCycle struct {
	name string
	sdl  string
}

// knownCycleDivergences are the cases this implementation does not match, and
// why. Each is asserted to *still* diverge, so that closing one cannot go
// unnoticed.
var knownCycleDivergences = map[string]string{
	// A schema here holds an object type for each root, so one naming an
	// interface cannot be built at all. graphql-js builds it and leaves
	// validation to say what is wrong with it.
}

func runPortedCycles(t *testing.T, known map[string]string, cases []portedCycle) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			s, err := utilities.BuildSchema(tt.sdl)
			if err != nil {
				if _, listed := known[tt.name]; listed {
					t.Logf("known divergence: %s", known[tt.name])
					return
				}
				t.Fatalf("building the schema: %v\n%s", err, tt.sdl)
			}
			got := utilities.PrintSchema(s)

			if why, listed := known[tt.name]; listed {
				if got == tt.sdl {
					t.Errorf("this case now matches graphql-js; remove it from the known divergences (%s)", why)
				} else {
					t.Logf("known divergence: %s", why)
				}
				return
			}
			if got != tt.sdl {
				t.Errorf("printed back as\n%s\nwant\n%s", got, tt.sdl)
			}
		})
	}
}
