package graphql_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/ikawaha/graphql"
	"github.com/ikawaha/graphql/schema"
)

// The Star Wars schema is graphql-js's own end-to-end test, and it is here for
// the same reason: it is small enough to read and awkward enough to exercise
// the whole library at once — an interface with two implementations, a union,
// an enum, arguments with defaults, a list, and a field that fails.

const starWarsSDL = `
	"One of the films."
	enum Episode { NEW_HOPE EMPIRE JEDI }

	"A character in the Star Wars trilogy."
	interface Character {
		"The id of the character."
		id: String!
		"The name of the character."
		name: String
		"The friends of the character."
		friends: [Character]
		"Which movies they appear in."
		appearsIn: [Episode]
		"All the characters they know, however far removed."
		acquaintances: [Character]
		"Where they are from, which not every character will say."
		secretBackstory: String
	}

	"A humanoid creature in the Star Wars universe."
	type Human implements Character {
		id: String!
		name: String
		friends: [Character]
		appearsIn: [Episode]
		acquaintances: [Character]
		secretBackstory: String
		"The home planet, or null if unknown."
		homePlanet: String
	}

	"A mechanical creature in the Star Wars universe."
	type Droid implements Character {
		id: String!
		name: String
		friends: [Character]
		appearsIn: [Episode]
		acquaintances: [Character]
		secretBackstory: String
		"The primary function of the droid."
		primaryFunction: String
	}

	union SearchResult = Human | Droid

	type Query {
		hero(episode: Episode = NEW_HOPE): Character
		human(id: String!): Human
		droid(id: String!): Droid
		search(text: String!): [SearchResult]
	}`

// character is what the schema's types are resolved from. One Go type stands
// for both Human and Droid, so which one a value is has to be decided by the
// schema rather than read off the value.
//
// HomePlanet is a pointer because a Go zero value cannot say "null": an
// ordinary string field would answer "" for a character whose home is unknown,
// which is a different thing from not knowing. A field that can be null in the
// schema needs a Go type that can be nil.
type character struct {
	ID              string `graphql:"id"`
	Name            string
	FriendIDs       []string `graphql:"-"`
	AppearsIn       []string
	Droid           bool `graphql:"-"`
	HomePlanet      *string
	PrimaryFunction string
}

// planet names a home world, for the characters that have one.
func planet(name string) *string { return &name }

var (
	luke   = &character{ID: "1000", Name: "Luke Skywalker", FriendIDs: []string{"1002", "1003", "2000", "2001"}, AppearsIn: []string{"NEW_HOPE", "EMPIRE", "JEDI"}, HomePlanet: planet("Tatooine")}
	vader  = &character{ID: "1001", Name: "Darth Vader", FriendIDs: []string{"1004"}, AppearsIn: []string{"NEW_HOPE", "EMPIRE", "JEDI"}, HomePlanet: planet("Tatooine")}
	han    = &character{ID: "1002", Name: "Han Solo", FriendIDs: []string{"1000", "1003", "2001"}, AppearsIn: []string{"NEW_HOPE", "EMPIRE", "JEDI"}}
	leia   = &character{ID: "1003", Name: "Leia Organa", FriendIDs: []string{"1000", "1002", "2000", "2001"}, AppearsIn: []string{"NEW_HOPE", "EMPIRE", "JEDI"}, HomePlanet: planet("Alderaan")}
	tarkin = &character{ID: "1004", Name: "Wilhuff Tarkin", FriendIDs: []string{"1001"}, AppearsIn: []string{"NEW_HOPE"}}

	threepio = &character{ID: "2000", Name: "C-3PO", Droid: true, FriendIDs: []string{"1000", "1002", "1003", "2001"}, AppearsIn: []string{"NEW_HOPE", "EMPIRE", "JEDI"}, PrimaryFunction: "Protocol"}
	artoo    = &character{ID: "2001", Name: "R2-D2", Droid: true, FriendIDs: []string{"1000", "1002", "1003"}, AppearsIn: []string{"NEW_HOPE", "EMPIRE", "JEDI"}, PrimaryFunction: "Astromech"}

	everyone = []*character{luke, vader, han, leia, tarkin, threepio, artoo}
)

func characterByID(id string) *character {
	for _, c := range everyone {
		if c.ID == id {
			return c
		}
	}
	return nil
}

var (
	starWarsOnce sync.Once
	starWarsVal  *graphql.Schema
	starWarsErr  error
)

// starWarsSchema builds the schema and wires up the resolvers. It is built
// once, since building it for each test would cost more than the tests.
func starWarsSchema(t testing.TB) *graphql.Schema {
	t.Helper()
	starWarsOnce.Do(func() { starWarsVal, starWarsErr = buildStarWars() })
	if starWarsErr != nil {
		t.Fatalf("building the Star Wars schema: %v", starWarsErr)
	}
	return starWarsVal
}

func buildStarWars() (*graphql.Schema, error) {
	s, err := graphql.BuildSchema(starWarsSDL)
	if err != nil {
		return nil, err
	}

	// Which type a character is cannot be read off the Go value, so the schema
	// is told how to decide.
	whichType := func(_ context.Context, v any, _ *graphql.ResolveInfo) (string, error) {
		c, isCharacter := v.(*character)
		if !isCharacter {
			return "", fmt.Errorf("%T is not a character", v)
		}
		if c.Droid {
			return "Droid", nil
		}
		return "Human", nil
	}
	s.Type("Character").(*schema.InterfaceType).ResolveType = whichType
	s.Type("SearchResult").(*schema.UnionType).ResolveType = whichType

	query := s.QueryType()
	query.Field("hero").Resolve = func(_ context.Context, _ any, args graphql.Arguments, _ *graphql.ResolveInfo) (any, error) {
		episode, _ := args.Get("episode")
		if episode == "EMPIRE" {
			return luke, nil
		}
		return artoo, nil
	}
	query.Field("human").Resolve = func(_ context.Context, _ any, args graphql.Arguments, _ *graphql.ResolveInfo) (any, error) {
		id, _ := args.Get("id")
		c := characterByID(id.(string))
		if c == nil || c.Droid {
			return nil, nil
		}
		return c, nil
	}
	query.Field("droid").Resolve = func(_ context.Context, _ any, args graphql.Arguments, _ *graphql.ResolveInfo) (any, error) {
		id, _ := args.Get("id")
		c := characterByID(id.(string))
		if c == nil || !c.Droid {
			return nil, nil
		}
		return c, nil
	}
	query.Field("search").Resolve = func(_ context.Context, _ any, args graphql.Arguments, _ *graphql.ResolveInfo) (any, error) {
		text, _ := args.Get("text")
		var found []any
		for _, c := range everyone {
			if strings.Contains(strings.ToLower(c.Name), strings.ToLower(text.(string))) {
				found = append(found, c)
			}
		}
		return found, nil
	}

	// friends and acquaintances are the same data reached two ways, so both
	// are resolved from the ids each character holds.
	friends := func(_ context.Context, source any, _ graphql.Arguments, _ *graphql.ResolveInfo) (any, error) {
		c, isCharacter := source.(*character)
		if !isCharacter {
			return nil, fmt.Errorf("%T is not a character", source)
		}
		out := make([]any, 0, len(c.FriendIDs))
		for _, id := range c.FriendIDs {
			out = append(out, characterByID(id))
		}
		return out, nil
	}
	// A field that always fails, for seeing what a failure looks like in a
	// response that otherwise succeeded.
	secret := func(context.Context, any, graphql.Arguments, *graphql.ResolveInfo) (any, error) {
		return nil, errors.New("secretBackstory is secret")
	}
	for _, name := range []string{"Human", "Droid"} {
		object := s.Type(name).(*schema.ObjectType)
		object.Field("friends").Resolve = friends
		object.Field("acquaintances").Resolve = friends
		object.Field("secretBackstory").Resolve = secret
	}
	return s, nil
}

// ask runs a query the way a server would, and returns the response as JSON.
func ask(t *testing.T, query string, variables map[string]any) string {
	t.Helper()
	params := graphql.Params{Schema: starWarsSchema(t), Query: query}
	if variables != nil {
		params.Variables = graphql.Variables(variables)
	}
	return jsonOf(t, graphql.Do(context.Background(), params))
}

func TestStarWars_Queries(t *testing.T) {
	tests := []struct {
		name  string
		query string
		vars  map[string]any
		want  string
	}{
		{
			name:  "the hero's name",
			query: `{ hero { name } }`,
			want:  `{"data":{"hero":{"name":"R2-D2"}}}`,
		},
		{
			// The argument was left out, so its default stands in.
			name:  "an argument's default",
			query: `{ hero { name } }`,
			want:  `{"data":{"hero":{"name":"R2-D2"}}}`,
		},
		{
			name:  "an argument given",
			query: `{ hero(episode: EMPIRE) { name } }`,
			want:  `{"data":{"hero":{"name":"Luke Skywalker"}}}`,
		},
		{
			name:  "an argument given by a variable",
			query: `query ($ep: Episode) { hero(episode: $ep) { name } }`,
			vars:  map[string]any{"ep": "EMPIRE"},
			want:  `{"data":{"hero":{"name":"Luke Skywalker"}}}`,
		},
		{
			// Writing the argument and supplying nothing asks for whatever the
			// field does without it, so the default applies.
			name:  "a variable that was not supplied",
			query: `query ($ep: Episode) { hero(episode: $ep) { name } }`,
			want:  `{"data":{"hero":{"name":"R2-D2"}}}`,
		},
		{
			name:  "nested lists",
			query: `{ hero { name friends { name } } }`,
			want: `{"data":{"hero":{"name":"R2-D2","friends":[` +
				`{"name":"Luke Skywalker"},{"name":"Han Solo"},{"name":"Leia Organa"}]}}}`,
		},
		{
			name:  "an enum in a response",
			query: `{ human(id: "1000") { name appearsIn } }`,
			want:  `{"data":{"human":{"name":"Luke Skywalker","appearsIn":["NEW_HOPE","EMPIRE","JEDI"]}}}`,
		},
		{
			name:  "aliases",
			query: `{ luke: human(id: "1000") { name } leia: human(id: "1003") { name } }`,
			want:  `{"data":{"luke":{"name":"Luke Skywalker"},"leia":{"name":"Leia Organa"}}}`,
		},
		{
			name:  "a field of the implementation, through the interface",
			query: `{ hero { name ... on Droid { primaryFunction } } }`,
			want:  `{"data":{"hero":{"name":"R2-D2","primaryFunction":"Astromech"}}}`,
		},
		{
			name:  "__typename",
			query: `{ hero { __typename name } }`,
			want:  `{"data":{"hero":{"__typename":"Droid","name":"R2-D2"}}}`,
		},
		{
			name:  "a union",
			query: `{ search(text: "o") { ... on Human { name homePlanet } ... on Droid { name primaryFunction } } }`,
			want: `{"data":{"search":[` +
				`{"name":"Han Solo","homePlanet":null},` +
				`{"name":"Leia Organa","homePlanet":"Alderaan"},` +
				`{"name":"C-3PO","primaryFunction":"Protocol"}]}}`,
		},
		{
			name: "a named fragment used twice",
			query: `
				{ luke: human(id: "1000") { ...Fields } leia: human(id: "1003") { ...Fields } }
				fragment Fields on Human { name homePlanet }
			`,
			want: `{"data":{"luke":{"name":"Luke Skywalker","homePlanet":"Tatooine"},` +
				`"leia":{"name":"Leia Organa","homePlanet":"Alderaan"}}}`,
		},
		{
			name:  "a field that is null",
			query: `{ human(id: "1002") { name homePlanet } }`,
			want:  `{"data":{"human":{"name":"Han Solo","homePlanet":null}}}`,
		},
		{
			name:  "nothing found",
			query: `{ human(id: "9999") { name } }`,
			want:  `{"data":{"human":null}}`,
		},
		{
			name:  "@skip and @include",
			query: `{ hero { name @include(if: true) friends @skip(if: true) { name } } }`,
			want:  `{"data":{"hero":{"name":"R2-D2"}}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ask(t, tt.query, tt.vars); got != tt.want {
				t.Errorf("response =\n  %s\nwant\n  %s", got, tt.want)
			}
		})
	}
}

// A field that fails is null and the failure is reported beside the data, so a
// client can see which part of the response is missing and why.
func TestStarWars_AFieldThatFails(t *testing.T) {
	got := ask(t, `{ hero { name secretBackstory } }`, nil)
	want := `{"errors":[{"message":"secretBackstory is secret",` +
		`"locations":[{"line":1,"column":15}],"path":["hero","secretBackstory"]}],` +
		`"data":{"hero":{"name":"R2-D2","secretBackstory":null}}}`
	if got != want {
		t.Errorf("response =\n  %s\nwant\n  %s", got, want)
	}

	// A failure inside a list loses only the entry it belongs to, and the path
	// says which.
	t.Run("inside a list", func(t *testing.T) {
		got := ask(t, `{ hero { friends { name secretBackstory } } }`, nil)
		for _, wanted := range []string{
			`"path":["hero","friends",0,"secretBackstory"]`,
			`"path":["hero","friends",2,"secretBackstory"]`,
			`"name":"Luke Skywalker"`,
		} {
			if !strings.Contains(got, wanted) {
				t.Errorf("the response does not contain %s:\n%s", wanted, got)
			}
		}
	})
}

// A document the schema cannot answer is refused before anything runs, and the
// response has no data at all rather than null data: the request never began.
func TestStarWars_Validation(t *testing.T) {
	tests := []struct {
		name  string
		query string
		says  string
	}{
		{"a field that is not there", `{ hero { favoriteSpaceship } }`, "Cannot query field"},
		{"a leaf given a selection", `{ hero { name { firstCharacterOfName } } }`, "must not have a selection"},
		{"an object with no selection", `{ hero }`, "must have a selection of subfields"},
		{"a misspelt field", `{ hero { nam } }`, `Did you mean \"name\"`},
		{"a required argument left out", `{ human { name } }`, "is required"},
		{"an argument of the wrong type", `{ hero(episode: "EMPIRE") { name } }`, "cannot represent non-enum value"},
		{"an unknown fragment", `{ hero { ...Missing } }`, "Unknown fragment"},
		{"a fragment on the wrong type", `{ hero { ... on String { x } } }`, "non composite type"},
		{"text that will not parse", `{ hero {`, "Syntax Error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ask(t, tt.query, nil)
			if !strings.Contains(got, tt.says) {
				t.Errorf("response =\n  %s\nwant it to say %q", got, tt.says)
			}
			if strings.Contains(got, `"data"`) {
				t.Errorf("a request that never ran came back with data:\n%s", got)
			}
		})
	}
}

// Introspection is how every client discovers a schema, so it has to answer
// for a schema built the ordinary way.
func TestStarWars_Introspection(t *testing.T) {
	tests := []struct{ query, want string }{
		{`{ __schema { queryType { name } } }`, `{"data":{"__schema":{"queryType":{"name":"Query"}}}}`},
		{`{ __type(name: "Droid") { name kind } }`, `{"data":{"__type":{"name":"Droid","kind":"OBJECT"}}}`},
		{
			`{ __type(name: "Droid") { description } }`,
			`{"data":{"__type":{"description":"A mechanical creature in the Star Wars universe."}}}`,
		},
		{
			`{ __type(name: "Character") { possibleTypes { name } } }`,
			`{"data":{"__type":{"possibleTypes":[{"name":"Human"},{"name":"Droid"}]}}}`,
		},
		{
			`{ __type(name: "Query") { fields(includeDeprecated: false) { name args { name defaultValue } } } }`,
			`{"data":{"__type":{"fields":[` +
				`{"name":"hero","args":[{"name":"episode","defaultValue":"NEW_HOPE"}]},` +
				`{"name":"human","args":[{"name":"id","defaultValue":null}]},` +
				`{"name":"droid","args":[{"name":"id","defaultValue":null}]},` +
				`{"name":"search","args":[{"name":"text","defaultValue":null}]}]}}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			if got := ask(t, tt.query, nil); got != tt.want {
				t.Errorf("response =\n  %s\nwant\n  %s", got, tt.want)
			}
		})
	}
}

// A query deep enough to be worth answering piece by piece is what @defer is
// for, and the whole library has to agree about it.
func TestStarWars_Incremental(t *testing.T) {
	s, err := graphql.BuildSchema(starWarsSDL + `
		directive @defer(if: Boolean = true, label: String) on FRAGMENT_SPREAD | INLINE_FRAGMENT
		directive @stream(if: Boolean = true, label: String, initialCount: Int = 0) on FIELD
	`)
	if err != nil {
		t.Fatalf("building: %v", err)
	}
	// The resolvers of the built schema are wired the same way.
	base := starWarsSchema(t)
	s.Type("Character").(*schema.InterfaceType).ResolveType = base.Type("Character").(*schema.InterfaceType).ResolveType
	s.Type("SearchResult").(*schema.UnionType).ResolveType = base.Type("SearchResult").(*schema.UnionType).ResolveType
	for _, name := range []string{"Query", "Human", "Droid"} {
		from := base.Type(name).(*schema.ObjectType)
		to := s.Type(name).(*schema.ObjectType)
		for _, field := range from.Fields() {
			if field.Resolve != nil {
				to.Field(field.Name()).Resolve = field.Resolve
			}
		}
	}

	result := graphql.DoIncrementally(context.Background(), graphql.Params{
		Schema: s,
		Query: `
			{
				hero {
					name
					... @defer(label: "friends") { friends @stream(initialCount: 1) { name } }
				}
			}
		`,
	})
	if len(result.Initial.Errors) != 0 {
		t.Fatalf("errors: %v", result.Initial.Errors)
	}
	if got := jsonOf(t, result.Initial); !strings.Contains(got, `"hero":{"name":"R2-D2"}`) {
		t.Errorf("the first payload = %s", got)
	}
	if result.Subsequent == nil {
		t.Fatal("nothing was deferred")
	}

	var names []string
	for payload := range result.Subsequent {
		for _, incremental := range payload.Incremental {
			for _, entry := range incremental.Items {
				if object, isObject := entry.(*graphql.OrderedMap); isObject {
					name, _ := object.Get("name")
					names = append(names, fmt.Sprint(name))
				}
			}
		}
	}
	// R2-D2's friends arrive one at a time, in order, after the first.
	if strings.Join(names, ",") != "Han Solo,Leia Organa" {
		t.Errorf("the streamed friends were %v", names)
	}
}
