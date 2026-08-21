package execution_test

// Ported from graphql-js src/execution/__tests__/abstract-test.ts.
//
// The original runs each case twice, once with an isTypeOf that answers at
// once and once with one that answers later; here there is only the one
// answer, so each case runs once.

import (
	"context"
	"errors"
	"testing"

	"github.com/ikawaha/graphql/schema"
)

func TestPortedAbstract(t *testing.T) {
	const petsQuery = `
      {
        pets {
          name
          ... on Dog {
            woofs
          }
          ... on Cat {
            meows
          }
        }
      }
    `
	const twoPets = `{"data": {"pets": [
		{"name": "Odie", "woofs": true}, {"name": "Garfield", "meows": false}]}}`

	runPorted(t, nil, nil, nil, []portedCase{
		{
			name:  `isTypeOf used to resolve runtime type for Interface`,
			built: petInterfaceSchema(t, isDog, isCat),
			query: petsQuery,
			want:  twoPets,
		},
		{
			name:  `isTypeOf can throw`,
			built: petInterfaceSchema(t, throwsFromIsTypeOf, nil),
			query: petsQuery,
			want: `{"data": {"pets": [null, null]}, "errors": [
				{"message": "We are testing this error",
				 "locations": [{"line": 3, "column": 9}], "path": ["pets", 0]},
				{"message": "We are testing this error",
				 "locations": [{"line": 3, "column": 9}], "path": ["pets", 1]}]}`,
		},
		{
			name:  `isTypeOf can return false`,
			built: onePetSchema(t),
			query: `
      {
        pet {
          name
        }
      }
    `,
			want: `{"data": {"pet": null}, "errors": [
				{"message": "Abstract type \"Pet\" must resolve to an Object type at runtime for field \"Query.pet\". Either the \"Pet\" type should provide a \"resolveType\" function or each possible type should provide an \"isTypeOf\" function.",
				 "locations": [{"line": 3, "column": 9}], "path": ["pet"]}]}`,
		},
		{
			name:  `isTypeOf used to resolve runtime type for Union`,
			built: petUnionSchema(t, isDog, isCat, nil),
			query: petsQuery,
			want:  twoPets,
		},
		{
			name:  `resolveType can throw`,
			built: petUnionSchema(t, nil, nil, throwsFromResolveType),
			query: petsQuery,
			want: `{"data": {"pets": [null, null]}, "errors": [
				{"message": "We are testing this error",
				 "locations": [{"line": 3, "column": 9}], "path": ["pets", 0]},
				{"message": "We are testing this error",
				 "locations": [{"line": 3, "column": 9}], "path": ["pets", 1]}]}`,
		},
		{
			name: `resolve Union type using __typename on source object`,
			sdl: `
      type Query {
        pets: [Pet]
      }

      union Pet = Cat | Dog

      type Cat {
        name: String
        meows: Boolean
      }

      type Dog {
        name: String
        woofs: Boolean
      }
    `,
			root: map[string]any{"pets": []any{
				map[string]any{"__typename": "Dog", "name": "Odie", "woofs": true},
				map[string]any{"__typename": "Cat", "name": "Garfield", "meows": false},
			}},
			query: petsQuery,
			want:  twoPets,
		},
		{
			name: `resolve Interface type using __typename on source object`,
			sdl: `
      type Query {
        pets: [Pet]
      }

      interface Pet {
        name: String
      }

      type Cat implements Pet {
        name: String
        meows: Boolean
      }

      type Dog implements Pet {
        name: String
        woofs: Boolean
      }
    `,
			root: map[string]any{"pets": []any{
				map[string]any{"__typename": "Dog", "name": "Odie", "woofs": true},
				map[string]any{"__typename": "Cat", "name": "Garfield", "meows": false},
			}},
			query: petsQuery,
			want:  twoPets,
		},
	})
}

// Ported from `resolveType on Interface yields useful error`: each way of
// naming the wrong type is a different mistake, and each is named as such.
func TestPortedAbstract_UsefulErrors(t *testing.T) {
	const sdl = `
      type Query {
        pet: Pet
      }

      interface Pet {
        name: String
      }

      type Cat implements Pet {
        name: String
      }

      type Dog implements Pet {
        name: String
      }
    `
	const query = `
      {
        pet {
          name
        }
      }
    `
	cases := []struct {
		name     string
		typeName any
		want     string
	}{
		{
			name:     "nothing at all",
			typeName: nil,
			want: `Abstract type "Pet" must resolve to an Object type at runtime for field "Query.pet". ` +
				`Either the "Pet" type should provide a "resolveType" function ` +
				`or each possible type should provide an "isTypeOf" function.`,
		},
		{
			name:     "a type the schema does not have",
			typeName: "Human",
			want:     `Abstract type "Pet" was resolved to a type "Human" that does not exist inside the schema.`,
		},
		{
			name:     "a type that is not an object",
			typeName: "String",
			want:     `Abstract type "Pet" was resolved to a non-object type "String".`,
		},
		{
			name:     "an object the abstract type does not cover",
			typeName: "__Schema",
			want:     `Runtime Object type "__Schema" is not a possible type for "Pet".`,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			pet := map[string]any{}
			if tt.typeName != nil {
				pet["__typename"] = tt.typeName
			}
			runPorted(t, nil, nil, nil, []portedCase{{
				name: tt.name, sdl: sdl, query: query,
				root: map[string]any{"pet": pet},
				want: `{"data": {"pet": null}, "errors": [{"message": ` +
					mustQuoteJSON(t, tt.want) +
					`, "locations": [{"line": 3, "column": 9}], "path": ["pet"]}]}`,
			}})
		})
	}
}

// The values the cases are about, and the ways of telling them apart.
type portedPetDog struct {
	Name  string
	Woofs bool
}

type portedPetCat struct {
	Name  string
	Meows bool
}

func isDog(_ context.Context, v any, _ *schema.ResolveInfo) (bool, error) {
	_, is := v.(*portedPetDog)
	return is, nil
}

func isCat(_ context.Context, v any, _ *schema.ResolveInfo) (bool, error) {
	_, is := v.(*portedPetCat)
	return is, nil
}

func throwsFromIsTypeOf(context.Context, any, *schema.ResolveInfo) (bool, error) {
	return false, errors.New("We are testing this error")
}

func throwsFromResolveType(context.Context, any, *schema.ResolveInfo) (string, error) {
	return "", errors.New("We are testing this error")
}

// twoPetsValue is the pair every case above asks about.
func twoPetsValue() any {
	return []any{
		&portedPetDog{Name: "Odie", Woofs: true},
		&portedPetCat{Name: "Garfield", Meows: false},
	}
}

func petInterfaceSchema(t *testing.T, dog, cat schema.IsTypeOfFn) *schema.Schema {
	t.Helper()
	s := buildPorted(t, `
		interface Pet { name: String }
		type Dog implements Pet { name: String woofs: Boolean }
		type Cat implements Pet { name: String meows: Boolean }
		type Query { pets: [Pet] }
	`)
	objectOf(t, s, "Dog").IsTypeOf = dog
	objectOf(t, s, "Cat").IsTypeOf = cat
	s.QueryType().Field("pets").Resolve = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		return twoPetsValue(), nil
	}
	return s
}

// onePetSchema has one candidate, and it says the value is not one of it.
func onePetSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s := buildPorted(t, `
		interface Pet { name: String }
		type Dog implements Pet { name: String woofs: Boolean }
		type Query { pet: Pet }
	`)
	objectOf(t, s, "Dog").IsTypeOf = func(context.Context, any, *schema.ResolveInfo) (bool, error) {
		return false, nil
	}
	s.QueryType().Field("pet").Resolve = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		// An empty map so that nothing else can say what it is.
		return map[string]any{}, nil
	}
	return s
}

func petUnionSchema(t *testing.T, dog, cat schema.IsTypeOfFn, resolve schema.TypeResolver) *schema.Schema {
	t.Helper()
	s := buildPorted(t, `
		type Dog { name: String woofs: Boolean }
		type Cat { name: String meows: Boolean }
		union Pet = Dog | Cat
		type Query { pets: [Pet] }
	`)
	objectOf(t, s, "Dog").IsTypeOf = dog
	objectOf(t, s, "Cat").IsTypeOf = cat
	if union, isUnion := s.Type("Pet").(*schema.UnionType); isUnion {
		union.ResolveType = resolve
	}
	s.QueryType().Field("pets").Resolve = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		return twoPetsValue(), nil
	}
	return s
}

func objectOf(t *testing.T, s *schema.Schema, name string) *schema.ObjectType {
	t.Helper()
	typed, isObject := s.Type(name).(*schema.ObjectType)
	if !isObject {
		t.Fatalf("the schema has no object type named %s", name)
	}
	return typed
}
