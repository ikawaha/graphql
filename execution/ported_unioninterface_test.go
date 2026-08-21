package execution_test

// Ported from graphql-js src/execution/__tests__/union-interface-test.ts. The
// cases left out are about when an isTypeOf promise settles, which is not a
// question Go asks, and one that reaches into a resolver to see what it was
// handed, which the ResolveInfo tests cover.

import (
	"context"
	"errors"
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

func TestPortedUnionInterface(t *testing.T) {
	s, john := testUnionInterfaceSchema(t)
	runPorted(t, s, john, nil, []portedCase{
		{
			name: `can introspect on union and intersection types`,
			query: `
      {
        Named: __type(name: "Named") {
          kind
          name
          fields { name }
          interfaces { name }
          possibleTypes { name }
          enumValues { name }
          inputFields { name }
        }
        Mammal: __type(name: "Mammal") {
          kind
          name
          fields { name }
          interfaces { name }
          possibleTypes { name }
          enumValues { name }
          inputFields { name }
        }
        Pet: __type(name: "Pet") {
          kind
          name
          fields { name }
          interfaces { name }
          possibleTypes { name }
          enumValues { name }
          inputFields { name }
        }
      }
    `,
			want: `{"data": {
				"Named": {"kind": "INTERFACE", "name": "Named", "fields": [{"name": "name"}],
					"interfaces": [], "enumValues": null, "inputFields": null,
					"possibleTypes": [{"name": "Dog"}, {"name": "Cat"}, {"name": "Person"}, {"name": "Plant"}]},
				"Mammal": {"kind": "INTERFACE", "name": "Mammal",
					"fields": [{"name": "progeny"}, {"name": "mother"}, {"name": "father"}],
					"interfaces": [{"name": "Life"}], "enumValues": null, "inputFields": null,
					"possibleTypes": [{"name": "Dog"}, {"name": "Cat"}, {"name": "Person"}]},
				"Pet": {"kind": "UNION", "name": "Pet", "fields": null, "interfaces": null,
					"enumValues": null, "inputFields": null,
					"possibleTypes": [{"name": "Dog"}, {"name": "Cat"}]}}}`,
		},
		{
			// An invalid document, but one the executor should still answer.
			name: `executes using union types`,
			query: `
      {
        __typename
        name
        pets {
          __typename
          name
          barks
          meows
        }
      }
    `,
			want: `{"data": {"__typename": "Person", "name": "John", "pets": [
				{"__typename": "Cat", "name": "Garfield", "meows": false},
				{"__typename": "Dog", "name": "Odie", "barks": true}]}}`,
		},
		{
			name: `executes union types with inline fragments`,
			query: `
      {
        __typename
        name
        pets {
          __typename
          ... on Dog {
            name
            barks
          }
          ... on Cat {
            name
            meows
          }
        }
      }
    `,
			want: `{"data": {"__typename": "Person", "name": "John", "pets": [
				{"__typename": "Cat", "name": "Garfield", "meows": false},
				{"__typename": "Dog", "name": "Odie", "barks": true}]}}`,
		},
		{
			name: `executes using interface types`,
			query: `
      {
        __typename
        name
        friends {
          __typename
          name
          barks
          meows
        }
      }
    `,
			want: `{"data": {"__typename": "Person", "name": "John", "friends": [
				{"__typename": "Person", "name": "Liz"},
				{"__typename": "Dog", "name": "Odie", "barks": true}]}}`,
		},
		{
			name: `executes interface types with inline fragments`,
			query: `
      {
        __typename
        name
        friends {
          __typename
          name
          ... on Dog {
            barks
          }
          ... on Cat {
            meows
          }

          ... on Mammal {
            mother {
              __typename
              ... on Dog {
                name
                barks
              }
              ... on Cat {
                name
                meows
              }
            }
          }
        }
      }
    `,
			want: `{"data": {"__typename": "Person", "name": "John", "friends": [
				{"__typename": "Person", "name": "Liz", "mother": null},
				{"__typename": "Dog", "name": "Odie", "barks": true,
				 "mother": {"__typename": "Dog", "name": "Odie's Mom", "barks": true}}]}}`,
		},
		{
			name: `executes interface types with named fragments`,
			query: `
      {
        __typename
        name
        friends {
          __typename
          name
          ...DogBarks
          ...CatMeows
        }
      }

      fragment  DogBarks on Dog {
        barks
      }

      fragment  CatMeows on Cat {
        meows
      }
    `,
			want: `{"data": {"__typename": "Person", "name": "John", "friends": [
				{"__typename": "Person", "name": "Liz"},
				{"__typename": "Dog", "name": "Odie", "barks": true}]}}`,
		},
		{
			name: `allows fragment conditions to be abstract types`,
			query: `
      {
        __typename
        name
        pets {
          ...PetFields,
          ...on Mammal {
            mother {
              ...ProgenyFields
            }
          }
        }
        friends { ...FriendFields }
      }

      fragment PetFields on Pet {
        __typename
        ... on Dog {
          name
          barks
        }
        ... on Cat {
          name
          meows
        }
      }

      fragment FriendFields on Named {
        __typename
        name
        ... on Dog {
          barks
        }
        ... on Cat {
          meows
        }
      }

      fragment ProgenyFields on Life {
        progeny {
          __typename
        }
      }
    `,
			want: `{"data": {"__typename": "Person", "name": "John",
				"pets": [
					{"__typename": "Cat", "name": "Garfield", "meows": false,
					 "mother": {"progeny": [{"__typename": "Cat"}]}},
					{"__typename": "Dog", "name": "Odie", "barks": true,
					 "mother": {"progeny": [{"__typename": "Dog"}]}}],
				"friends": [
					{"__typename": "Person", "name": "Liz"},
					{"__typename": "Dog", "name": "Odie", "barks": true}]}}`,
		},
	})
}

// The values graphql-js writes as classes. What decides a value's type is its
// Go type, which is what the isTypeOf functions below ask.
type (
	portedDog struct {
		Name    string
		Barks   bool
		Mother  *portedDog
		Father  *portedDog
		Progeny []*portedDog
	}
	portedCat struct {
		Name    string
		Meows   bool
		Mother  *portedCat
		Father  *portedCat
		Progeny []*portedCat
	}
	portedPlant  struct{ Name string }
	portedPerson struct {
		Name             string
		Pets             []any
		Friends          []any
		Responsibilities []any
		Progeny          []*portedPerson
		Mother           *portedPerson
		Father           *portedPerson
	}
)

// testUnionInterfaceSchema is graphql-js's own schema, and the John it is
// asked about.
func testUnionInterfaceSchema(t *testing.T) (*schema.Schema, any) {
	t.Helper()
	// The types are declared in the order graphql-js happens to collect them
	// in, since that is the order an interface lists what implements it.
	s, err := utilities.BuildSchema(`
		interface Named { name: String }
		interface Life { progeny: [Life] }
		interface Mammal implements Life { progeny: [Mammal] mother: Mammal father: Mammal }

		type Dog implements Mammal & Life & Named {
			name: String barks: Boolean progeny: [Dog] mother: Dog father: Dog
		}
		type Cat implements Mammal & Life & Named {
			name: String meows: Boolean progeny: [Cat] mother: Cat father: Cat
		}
		type Person implements Named & Mammal & Life {
			name: String
			pets: [Pet]
			friends: [Named]
			responsibilities: [PetOrPlantType]
			progeny: [Person]
			mother: Person
			father: Person
		}
		type Plant implements Named { name: String }

		union Pet = Dog | Cat
		union PetOrPlantType = Plant | Dog | Cat

		schema { query: Person }
	`)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}

	isType := func(matches func(any) bool) schema.IsTypeOfFn {
		return func(_ context.Context, v any, _ *schema.ResolveInfo) (bool, error) {
			return matches(v), nil
		}
	}
	object := func(name string) *schema.ObjectType {
		typed, _ := s.Type(name).(*schema.ObjectType)
		if typed == nil {
			t.Fatalf("the schema has no object type named %s", name)
		}
		return typed
	}
	object("Dog").IsTypeOf = isType(func(v any) bool { _, is := v.(*portedDog); return is })
	object("Cat").IsTypeOf = isType(func(v any) bool { _, is := v.(*portedCat); return is })
	object("Person").IsTypeOf = isType(func(v any) bool { _, is := v.(*portedPerson); return is })
	object("Plant").IsTypeOf = func(context.Context, any, *schema.ResolveInfo) (bool, error) {
		return false, errors.New("Not sure if this is a plant")
	}
	if pet, isUnion := s.Type("Pet").(*schema.UnionType); isUnion {
		pet.ResolveType = func(_ context.Context, v any, _ *schema.ResolveInfo) (string, error) {
			switch v.(type) {
			case *portedDog:
				return "Dog", nil
			case *portedCat:
				return "Cat", nil
			}
			return "", errors.New("not a pet")
		}
	}

	garfield := &portedCat{Name: "Garfield"}
	garfield.Mother = &portedCat{Name: "Garfield's Mom"}
	garfield.Mother.Progeny = []*portedCat{garfield}

	odie := &portedDog{Name: "Odie", Barks: true}
	odie.Mother = &portedDog{Name: "Odie's Mom", Barks: true}
	odie.Mother.Progeny = []*portedDog{odie}

	liz := &portedPerson{Name: "Liz"}
	john := &portedPerson{
		Name:             "John",
		Pets:             []any{garfield, odie},
		Friends:          []any{liz, odie},
		Responsibilities: []any{garfield, &portedPlant{Name: "Fern"}},
	}
	return s, john
}
