package execution_test

// The harness the ported @defer and @stream cases run against: graphql-js's
// own schema for them, and a comparison over the whole run.

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/schema"
)

// portedIncrementalSDL is graphql-js's schema from defer-test.ts and
// stream-test.ts, written out. The two directives are declared because a
// schema only gains them when it asks for them.
const portedIncrementalSDL = `
	directive @defer(if: Boolean! = true, label: String) on FRAGMENT_SPREAD | INLINE_FRAGMENT
	directive @stream(if: Boolean! = true, label: String, initialCount: Int = 0) on FIELD

	type Friend { id: ID name: String nonNullName: String! }
	type DeeperObject { foo: String bar: String baz: String bak: String }
	type NestedObject { deeperObject: DeeperObject name: String }
	type AnotherNestedObject { deeperObject: DeeperObject }

	type Hero {
		id: ID
		name: String
		nonNullName: String!
		friends: [Friend]
		nestedObject: NestedObject
		anotherNestedObject: AnotherNestedObject
	}

	type c { d: String nonNullErrorField: String! }
	type e { f: String }
	type b { c: c e: e }
	type a { b: b someField: String nonNullErrorField: String! }
	type g { h: String }

	type Query { hero: Hero a: a g: g }`

// portedIncrementalRoot is the value graphql-js runs these against when a case
// brings none of its own: `{ hero }`, where hero holds the three friends.
func portedIncrementalRoot() map[string]any {
	return map[string]any{
		"hero": map[string]any{
			"name": "Luke",
			"id":   1,
			"friends": []any{
				map[string]any{"name": "Han", "id": 2},
				map[string]any{"name": "Leia", "id": 3},
				map[string]any{"name": "C-3PO", "id": 4},
			},
			// graphql-js puts the GraphQLObjectType objects here, so reading
			// either of these gives the type's own name. No case that uses the
			// default root reads them; they are here so that one would answer
			// the same way.
			"nestedObject":        map[string]any{"name": "NestedObject"},
			"anotherNestedObject": map[string]any{},
		},
	}
}

// portedIncrementalCase is one run: a document, the value to run it against,
// and every response it should produce.
type portedIncrementalCase struct {
	name  string
	query string
	// root is the case's own root value as JSON, or empty for the shared one.
	root string
	// want is the whole run as JSON: either a list of responses, or a single
	// response where nothing turned out to be deferred.
	want string
	// failing names the fields, as Type.field, whose resolver should fail.
	// graphql-js writes those as a root value holding a function that throws;
	// a Go resolver is where the same thing goes.
	failing []string
}

// knownIncrementalDivergences are the cases this implementation does not
// match, and why. Each is asserted to *still* differ, so that closing one
// cannot go unnoticed.
// failField makes one field answer with an error, which is what graphql-js's
// `() => { throw new Error('bad') }` comes to here.
func failField(t *testing.T, s *schema.Schema, coordinate string) {
	t.Helper()
	typeName, fieldName, found := strings.Cut(coordinate, ".")
	if !found {
		t.Fatalf("%q is not a Type.field coordinate", coordinate)
	}
	object, isObject := s.Type(typeName).(*schema.ObjectType)
	if !isObject {
		t.Fatalf("%q is not an object type", typeName)
	}
	field := object.Field(fieldName)
	if field == nil {
		t.Fatalf("%s has no field %s", typeName, fieldName)
	}
	field.Resolve = func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		return nil, errors.New("bad")
	}
}

var knownIncrementalDivergences = map[string]string{}

// portedStreamSDL is graphql-js's schema from stream-test.ts.
const portedStreamSDL = `
	directive @stream(if: Boolean! = true, label: String, initialCount: Int = 0) on FIELD
	directive @defer(if: Boolean! = true, label: String) on FRAGMENT_SPREAD | INLINE_FRAGMENT

	type Friend { id: ID name: String nonNullName: String! }
	type DeeperNestedObject {
		nonNullScalarField: String!
		deeperNestedFriendList: [Friend]
	}
	type NestedObject {
		scalarField: String
		nonNullScalarField: String!
		nestedFriendList: [Friend]
		deeperNestedObject: DeeperNestedObject
	}
	type Query {
		scalarList: [String]
		scalarListList: [[String]]
		friendList: [Friend]
		nonNullFriendList: [Friend!]
		nestedObject: NestedObject
	}`

// runPortedStream is [runPortedIncremental] against the streaming schema,
// which graphql-js keeps separate from the one its defer cases use.
func runPortedStream(t *testing.T, cases []portedIncrementalCase) {
	t.Helper()
	runIncremental(t, portedStreamSDL, func(*testing.T) any { return map[string]any{} }, cases)
}

func runPortedIncremental(t *testing.T, cases []portedIncrementalCase) {
	t.Helper()
	runIncremental(t, portedIncrementalSDL,
		func(*testing.T) any { return portedIncrementalRoot() }, cases)
}

func runIncremental(
	t *testing.T,
	sdl string,
	defaultRoot func(*testing.T) any,
	cases []portedIncrementalCase,
) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			// A fresh schema per case, since a failing resolver is set on it.
			s := buildSchema(t, sdl)
			for _, coordinate := range tt.failing {
				failField(t, s, coordinate)
			}
			root := defaultRoot(t)
			if tt.root != "" {
				decoder := json.NewDecoder(strings.NewReader(tt.root))
				decoder.UseNumber()
				if err := decoder.Decode(&root); err != nil {
					t.Fatalf("reading the root value: %v", err)
				}
			}

			result := execution.ExecuteIncrementally(context.Background(), execution.Request{
				Schema:    s,
				Document:  mustParse(t, tt.query),
				RootValue: root,
			})

			var got any
			if result.Subsequent == nil {
				got = decodeJSON(t, mustMarshal(t, result.Initial))
			} else {
				responses := []any{decodeJSON(t, mustMarshal(t, result.Initial))}
				for payload := range result.Subsequent {
					responses = append(responses, decodeJSON(t, mustMarshal(t, payload)))
				}
				got = responses
			}

			want := decodeJSON(t, tt.want)
			if why, listed := knownIncrementalDivergences[tt.name]; listed {
				if reflect.DeepEqual(got, want) {
					t.Errorf("this case now matches graphql-js; remove it from the known divergences (%s)", why)
				} else {
					t.Logf("known divergence: %s", why)
				}
				return
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("run =\n%s\nwant\n%s", mustMarshal(t, got), tt.want)
			}
		})
	}
}
