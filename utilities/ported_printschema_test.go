package utilities_test

// Ported from graphql-js src/utilities/__tests__/printSchema-test.ts.
//
// Each of those cases builds a schema in JavaScript and compares what it
// prints to a piece of SDL. The schemas cannot be built the same way here, but
// the SDL can: what is checked is that each of those pieces reads back as
// itself, which is what the original also asserts of every one of them.

import "testing"

func TestPortedPrintSchema(t *testing.T) {
	runPortedCycles(t, nil, []portedCycle{
		{
			name: `Prints String Field`,
			sdl: `type Query {
  singleField: String
}`,
		},
		{
			name: `Prints [String] Field`,
			sdl: `type Query {
  singleField: [String]
}`,
		},
		{
			name: `Prints String! Field`,
			sdl: `type Query {
  singleField: String!
}`,
		},
		{
			name: `Prints [String]! Field`,
			sdl: `type Query {
  singleField: [String]!
}`,
		},
		{
			name: `Prints [String!] Field`,
			sdl: `type Query {
  singleField: [String!]
}`,
		},
		{
			name: `Prints [String!]! Field`,
			sdl: `type Query {
  singleField: [String!]!
}`,
		},
		{
			name: `Print Object Field`,
			sdl: `type Foo {
  str: String
}`,
		},
		{
			name: `Prints String Field With Int Arg`,
			sdl: `type Query {
  singleField(argOne: Int): String
}`,
		},
		{
			name: `Prints String Field With Int Arg With Default`,
			sdl: `type Query {
  singleField(argOne: Int = 2): String
}`,
		},
		{
			name: `Prints String Field With Int Arg With Default Null`,
			sdl: `type Query {
  singleField(argOne: Int = null): String
}`,
		},
		{
			name: `Prints String Field With Int! Arg`,
			sdl: `type Query {
  singleField(argOne: Int!): String
}`,
		},
		{
			name: `Prints String Field With Multiple Args`,
			sdl: `type Query {
  singleField(argOne: Int, argTwo: String): String
}`,
		},
		{
			name: `Prints String Field With Multiple Args, First is Default`,
			sdl: `type Query {
  singleField(argOne: Int = 1, argTwo: String, argThree: Boolean): String
}`,
		},
		{
			name: `Prints String Field With Multiple Args, Second is Default`,
			sdl: `type Query {
  singleField(argOne: Int, argTwo: String = "foo", argThree: Boolean): String
}`,
		},
		{
			name: `Prints String Field With Multiple Args, Last is Default`,
			sdl: `type Query {
  singleField(argOne: Int, argTwo: String, argThree: Boolean = false): String
}`,
		},
		{
			name: `Prints schema with description`,
			sdl: `"""Schema description."""
schema {
  query: Query
}

type Query`,
		},
		{
			name: `Omits schema of common names`,
			sdl: `type Query

type Mutation

type Subscription`,
		},
		{
			name: `Prints custom query root types`,
			sdl: `schema {
  query: CustomType
}

type CustomType`,
		},
		{
			name: `Prints custom mutation root types`,
			sdl: `schema {
  mutation: CustomType
}

type CustomType`,
		},
		{
			name: `Prints custom subscription root types`,
			sdl: `schema {
  subscription: CustomType
}

type CustomType`,
		},
		{
			name: `Print Interface`,
			sdl: `type Bar implements Foo {
  str: String
}

interface Foo {
  str: String
}`,
		},
		{
			name: `Print Multiple Interface`,
			sdl: `type Bar implements Foo & Baz {
  str: String
  int: Int
}

interface Foo {
  str: String
}

interface Baz {
  int: Int
}`,
		},
		{
			name: `Print Hierarchical Interface`,
			sdl: `type Bar implements Foo & Baz {
  str: String
  int: Int
}

interface Foo {
  str: String
}

interface Baz implements Foo {
  int: Int
  str: String
}

type Query {
  bar: Bar
}`,
		},
		{
			name: `Print Unions`,
			sdl: `union SingleUnion = Foo

type Foo {
  bool: Boolean
}

union MultipleUnion = Foo | Bar

type Bar {
  str: String
}`,
		},
		{
			name: `Print Input Type`,
			sdl: `input InputType {
  int: Int
}`,
		},
		{
			name: `Print Input Type with @oneOf directive`,
			sdl: `input InputType @oneOf {
  int: Int
}`,
		},
		{
			name: `Custom Scalar`,
			sdl:  `scalar Odd`,
		},
		{
			name: `Custom Scalar with specifiedByURL`,
			sdl:  `scalar Foo @specifiedBy(url: "https://example.com/foo_spec")`,
		},
		{
			name: `Enum`,
			sdl: `enum RGB {
  RED
  GREEN
  BLUE
}`,
		},
		{
			name: `Prints empty types`,
			sdl: `enum SomeEnum

input SomeInputObject

interface SomeInterface

type SomeObject

union SomeUnion`,
		},
		{
			name: `Prints custom directives`,
			sdl: `directive @simpleDirective on FIELD

"""Complex Directive"""
directive @complexDirective(stringArg: String, intArg: Int = -1) repeatable on FIELD | QUERY`,
		},
		{
			name: `Prints an empty descriptions`,
			sdl: `""""""
schema {
  query: Query
}

""""""
directive @someDirective(
  """"""
  someArg: String

  """"""
  anotherArg: String
) on QUERY

""""""
scalar SomeScalar

""""""
interface SomeInterface {
  """"""
  someField(
    """"""
    someArg: String

    """"""
    anotherArg: String
  ): String

  """"""
  anotherField(
    """"""
    someArg: String

    """"""
    anotherArg: String
  ): String
}

""""""
union SomeUnion = Query

""""""
type Query {
  """"""
  someField(
    """"""
    someArg: String

    """"""
    anotherArg: String
  ): String

  """"""
  anotherField(
    """"""
    someArg: String

    """"""
    anotherArg: String
  ): String
}

""""""
enum SomeEnum {
  """"""
  SOME_VALUE

  """"""
  ANOTHER_VALUE
}`,
		},
		{
			name: `Prints a description with only whitespace`,
			sdl: `type Query {
  " "
  singleField: String
}`,
		},
		{
			name: `One-line prints a short description`,
			sdl: `type Query {
  """This field is awesome"""
  singleField: String
}`,
		},
	})
}
