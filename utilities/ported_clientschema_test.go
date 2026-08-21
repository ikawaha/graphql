package utilities_test

// Ported from graphql-js src/utilities/__tests__/buildClientSchema-test.ts:
// a schema described to a client and rebuilt from that description should be
// the same schema, and describing it again should give the same answer.

import (
	"context"
	"reflect"
	"testing"

	"github.com/ikawaha/graphql/utilities"
)

// knownClientSchemaDivergences are the cases this implementation does not
// match, and why. Each is asserted to *still* diverge, so that closing one
// cannot go unnoticed.
var knownClientSchemaDivergences = map[string]string{}

func TestPortedBuildClientSchema(t *testing.T) {
	for _, tt := range []struct{ name, sdl string }{
		{
			name: `builds a simple schema`,
			sdl: `"""Simple schema"""
schema {
  query: Simple
}

"""This is a simple type"""
type Simple {
  """This is a string field"""
  string: String
}`,
		},
		{
			name: `builds a simple schema with all operation types`,
			sdl: `schema {
  query: QueryType
  mutation: MutationType
  subscription: SubscriptionType
}

"""This is a simple mutation type"""
type MutationType {
  """Set the string field"""
  string: String
}

"""This is a simple query type"""
type QueryType {
  """This is a string field"""
  string: String
}

"""This is a simple subscription type"""
type SubscriptionType {
  """This is a string field"""
  string: String
}`,
		},
		{
			name: `uses built-in scalars when possible`,
			sdl: `scalar CustomScalar

type Query {
  int: Int
  float: Float
  string: String
  boolean: Boolean
  id: ID
  custom: CustomScalar
}`,
		},
		{
			name: `builds a schema with a recursive type reference`,
			sdl: `schema {
  query: Recur
}

type Recur {
  recur: Recur
}`,
		},
		{
			name: `builds a schema with a circular type reference`,
			sdl: `type Dog {
  bestFriend: Human
}

type Human {
  bestFriend: Dog
}

type Query {
  dog: Dog
  human: Human
}`,
		},
		{
			name: `builds a schema with an interface`,
			sdl: `type Dog implements Friendly {
  bestFriend: Friendly
}

interface Friendly {
  """The best friend of this friendly thing"""
  bestFriend: Friendly
}

type Human implements Friendly {
  bestFriend: Friendly
}

type Query {
  friendly: Friendly
}`,
		},
		{
			name: `builds a schema with an interface hierarchy`,
			sdl: `type Dog implements Friendly & Named {
  bestFriend: Friendly
  name: String
}

interface Friendly implements Named {
  """The best friend of this friendly thing"""
  bestFriend: Friendly
  name: String
}

type Human implements Friendly & Named {
  bestFriend: Friendly
  name: String
}

interface Named {
  name: String
}

type Query {
  friendly: Friendly
}`,
		},
		{
			name: `builds a schema with an implicit interface`,
			sdl: `type Dog implements Friendly {
  bestFriend: Friendly
}

interface Friendly {
  """The best friend of this friendly thing"""
  bestFriend: Friendly
}

type Query {
  dog: Dog
}`,
		},
		{
			name: `builds a schema with a union`,
			sdl: `type Dog {
  bestFriend: Friendly
}

union Friendly = Dog | Human

type Human {
  bestFriend: Friendly
}

type Query {
  friendly: Friendly
}`,
		},
		{
			name: `builds a schema with complex field values`,
			sdl: `type Query {
  string: String
  listOfString: [String]
  nonNullString: String!
  nonNullListOfString: [String]!
  nonNullListOfNonNullString: [String!]!
}`,
		},
		{
			name: `builds a schema with field arguments`,
			sdl: `type Query {
  """A field with a single arg"""
  one(
    """This is an int arg"""
    intArg: Int
  ): String

  """A field with a two args"""
  two(
    """This is a list of int arg"""
    listArg: [Int]

    """This is a required arg"""
    requiredArg: Boolean!
  ): String
}`,
		},
		{
			name: `builds a schema with default value on custom scalar field`,
			sdl: `scalar CustomScalar

type Query {
  testField(testArg: CustomScalar = "default"): String
}`,
		},
		{
			name: `builds a schema with an input object`,
			sdl: `"""An input address"""
input Address {
  """What street is this address?"""
  street: String!

  """The city the address is within?"""
  city: String!

  """The country (blank will assume USA)."""
  country: String = "USA"
}

type Query {
  """Get a geocode from an address"""
  geocode(
    """The address to lookup"""
    address: Address
  ): String
}`,
		},
		{
			name: `builds a schema with field arguments with default values`,
			sdl: `input Geo {
  lat: Float
  lon: Float
}

type Query {
  defaultID(intArg: ID = "123"): String
  defaultInt(intArg: Int = 30): String
  defaultList(listArg: [Int] = [1, 2, 3]): String
  defaultObject(objArg: Geo = { lat: 37.485, lon: -122.148 }): String
  defaultNull(intArg: Int = null): String
  noDefault(intArg: Int): String
}`,
		},
		{
			name: `builds a schema with custom directives`,
			sdl: `"""This is a custom directive"""
directive @customDirective repeatable on FIELD

type Query {
  string: String
}`,
		},
		{
			name: `builds a schema aware of deprecation`,
			sdl: `directive @someDirective(
  """This is a shiny new argument"""
  shinyArg: SomeInputObject

  """This was our design mistake :("""
  oldArg: String @deprecated(reason: "Use shinyArg")
) on QUERY

enum Color {
  """So rosy"""
  RED

  """So grassy"""
  GREEN

  """So calming"""
  BLUE

  """So sickening"""
  MAUVE @deprecated(reason: "No longer in fashion")
}

input SomeInputObject {
  """Nothing special about it, just deprecated for some unknown reason"""
  oldField: String @deprecated(reason: "Don't use it, use newField instead!")

  """Same field but with a new name"""
  newField: String
}

type Query {
  """This is a shiny string field"""
  shinyString: String

  """This is a deprecated string field"""
  deprecatedString: String @deprecated(reason: "Use shinyString")

  """Color of a week"""
  color: Color

  """Some random field"""
  someField(
    """This is a shiny new argument"""
    shinyArg: SomeInputObject

    """This was our design mistake :("""
    oldArg: String @deprecated(reason: "Use shinyArg")
  ): String
}`,
		},
		{
			name: `builds a schema with empty deprecation reasons`,
			sdl: `directive @someDirective(someArg: SomeInputObject @deprecated(reason: "")) on QUERY

type Query {
  someField(someArg: SomeInputObject @deprecated(reason: "")): SomeEnum @deprecated(reason: "")
}

input SomeInputObject {
  someInputField: String @deprecated(reason: "")
}

enum SomeEnum {
  SOME_VALUE @deprecated(reason: "")
}`,
		},
		{
			name: `builds a schema with specifiedBy url`,
			sdl: `scalar Foo @specifiedBy(url: "https://example.com/foo_spec")

type Query {
  foo: Foo
}`,
		},
		{
			name: `builds a schema with @oneOf directive`,
			sdl: `type Query {
  someField(someArg: SomeInputObject): String
}

input SomeInputObject @oneOf {
  someInputField1: String
  someInputField2: String
}`,
		},
		{
			name: `succeeds on deep (<= 8 levels) types`,
			sdl: `type Query {
  foo: [[[[String!]!]!]!]!
}`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			why, listed := knownClientSchemaDivergences[tt.name]
			server := mustBuild(t, tt.sdl)
			first, err := utilities.IntrospectionFromSchema(context.Background(), server)
			if err != nil {
				t.Fatalf("describing the schema: %v", err)
			}
			client, err := utilities.BuildClientSchema(first)
			if err != nil {
				if listed {
					t.Logf("known divergence: %s", why)
					return
				}
				t.Fatalf("rebuilding from the description: %v", err)
			}
			again, err := utilities.IntrospectionFromSchema(context.Background(), client)
			if err != nil {
				t.Fatalf("describing the rebuilt schema: %v", err)
			}

			printed := utilities.PrintSchema(client)
			same := reflect.DeepEqual(again, first) && printed == tt.sdl
			if listed {
				if same {
					t.Errorf("this case now matches graphql-js; remove it from the known divergences (%s)", why)
				} else {
					t.Logf("known divergence: %s", why)
				}
				return
			}
			if !reflect.DeepEqual(again, first) {
				t.Error("describing the rebuilt schema gave a different answer")
			}
			if printed != tt.sdl {
				t.Errorf("came back as\n%s\nwant\n%s", printed, tt.sdl)
			}
		})
	}
}
