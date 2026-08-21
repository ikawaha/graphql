package validation_test

import (
	"sync"
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

// upstreamHarnessSDL is graphql-js's own validation test schema, copied
// unmodified from src/validation/__tests__/harness.ts (MIT, Copyright (c)
// GraphQL Contributors; see the NOTICE file).
//
// It is kept separate from testSDL rather than merged into it. The tests
// ported from graphql-js were written against these exact shapes — this root
// is named QueryRoot, and there is no mutation or subscription root — and a
// schema that differed would make a ported case pass or fail for a reason that
// has nothing to do with the rule under test.
const upstreamHarnessSDL = `

  interface Mammal {
    mother: Mammal
    father: Mammal
  }

  interface Pet {
    name(surname: Boolean): String
  }

  interface Canine implements Mammal {
    name(surname: Boolean): String
    mother: Canine
    father: Canine
  }

  enum DogCommand {
    SIT
    HEEL
    DOWN
  }

  scalar GeoPoint

  type Dog implements Pet & Mammal & Canine {
    name(surname: Boolean): String
    nickname: String
    barkVolume: Int
    barks: Boolean
    doesKnowCommand(dogCommand: DogCommand): Boolean
    isHouseTrained(atOtherHomes: Boolean = true): Boolean
    isAtLocation(x: Int, y: Int): Boolean
    distanceFrom(loc: GeoPoint): Float
    mother: Dog
    father: Dog
  }

  type Cat implements Pet {
    name(surname: Boolean): String
    nickname: String
    meows: Boolean
    meowsVolume: Int
    furColor: FurColor
  }

  union CatOrDog = Cat | Dog

  type Human {
    name(surname: Boolean): String
    pets: [Pet]
    relatives: [Human]!
  }

  enum FurColor {
    BROWN
    BLACK
    TAN
    SPOTTED
    NO_FUR
    UNKNOWN
  }

  input ComplexInput {
    requiredField: Boolean!
    nonNullField: Boolean! = false
    intField: Int
    stringField: String
    booleanField: Boolean
    stringListField: [String]
  }

  input OneOfInput @oneOf {
    stringField: String
    intField: Int
  }

  type ComplicatedArgs {
    # TODO List
    # TODO Coercion
    # TODO NotNulls
    intArgField(intArg: Int): String
    nonNullIntArgField(nonNullIntArg: Int!): String
    stringArgField(stringArg: String): String
    booleanArgField(booleanArg: Boolean): String
    enumArgField(enumArg: FurColor): String
    floatArgField(floatArg: Float): String
    idArgField(idArg: ID): String
    stringListArgField(stringListArg: [String]): String
    stringListNonNullArgField(stringListNonNullArg: [String!]): String
    complexArgField(complexArg: ComplexInput): String
    oneOfArgField(oneOfArg: OneOfInput): String
    multipleReqs(req1: Int!, req2: Int!): String
    nonNullFieldWithDefault(arg: Int! = 0): String
    multipleOpts(opt1: Int = 0, opt2: Int = 0): String
    multipleOptAndReq(req1: Int!, req2: Int!, opt1: Int = 0, opt2: Int = 0): String
  }

  type QueryRoot {
    human(id: ID): Human
    dog: Dog
    cat: Cat
    pet: Pet
    catOrDog: CatOrDog
    complicatedArgs: ComplicatedArgs
  }

  schema {
    query: QueryRoot
  }

  directive @onField on FIELD`

var (
	upstreamOnce sync.Once
	upstreamVal  *schema.Schema
	upstreamErr  error
)

// upstreamHarness is the schema the ported cases run against.
func upstreamHarness(t testing.TB) *schema.Schema {
	t.Helper()
	upstreamOnce.Do(func() {
		upstreamVal, upstreamErr = utilities.BuildSchema(upstreamHarnessSDL)
		if upstreamErr == nil {
			upstreamErr = schema.AssertValidSchema(upstreamVal)
		}
	})
	if upstreamErr != nil {
		t.Fatalf("building the upstream harness schema: %v", upstreamErr)
	}
	return upstreamVal
}

// TestUpstreamHarness_SchemaIsSound checks the schema the ported cases lean on,
// so that one of them failing is never the schema's fault.
func TestUpstreamHarness_SchemaIsSound(t *testing.T) {
	s := upstreamHarness(t)
	for _, name := range []string{
		"Mammal", "Pet", "Canine", "DogCommand", "GeoPoint", "Dog", "Cat",
		"CatOrDog", "Human", "FurColor", "ComplexInput", "OneOfInput",
		"ComplicatedArgs", "QueryRoot",
	} {
		if s.Type(name) == nil {
			t.Errorf("the upstream harness has no %s", name)
		}
	}
	if s.QueryType() == nil || s.QueryType().Name() != "QueryRoot" {
		t.Errorf("the query root is %v, want QueryRoot", s.QueryType())
	}
	if s.MutationType() != nil || s.SubscriptionType() != nil {
		t.Error("the upstream harness has no mutation or subscription root")
	}
	if s.Directive("onField") == nil {
		t.Error("the upstream harness has no @onField")
	}
}
