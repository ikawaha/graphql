package graphql_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ikawaha/graphql"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

// user is the sort of Go type a server already has. A field that can be null
// needs a Go type that can be nil, since a zero value cannot say "null".
type user struct {
	ID       string `graphql:"id"`
	Name     string
	Nickname *string
	Email    string `graphql:"-"` // not exposed
}

// A method stands in for a field the struct has no member for.
func (u *user) Greeting() string { return "hello, " + u.Name }

func Example() {
	s, err := graphql.BuildSchema(`
		type Query {
			user(id: ID!): User
		}
		type User {
			id: ID!
			name: String
			nickname: String
			greeting: String
		}
	`)
	if err != nil {
		log.Fatal(err)
	}

	people := map[string]*user{
		"1": {ID: "1", Name: "Ada", Email: "ada@example.com"},
	}
	s.QueryType().Field("user").Resolve = func(
		_ context.Context, _ any, args graphql.Arguments, _ *graphql.ResolveInfo,
	) (any, error) {
		id, _ := args.Get("id")
		return people[id.(string)], nil
	}

	result := graphql.Do(context.Background(), graphql.Params{
		Schema:    s,
		Query:     `query ($id: ID!) { user(id: $id) { name nickname greeting } }`,
		Variables: graphql.Variables(map[string]any{"id": "1"}),
	})
	_ = json.NewEncoder(os.Stdout).Encode(result)

	// Output:
	// {"data":{"user":{"name":"Ada","nickname":null,"greeting":"hello, Ada"}}}
}

// An argument the caller left out, one given as null, and one given a value
// are three different things, and a resolver can tell them apart.
func Example_omittedNullAndAValue() {
	s, err := graphql.BuildSchema(`type Query { search(filter: String): String }`)
	if err != nil {
		log.Fatal(err)
	}
	s.QueryType().Field("search").Resolve = func(
		_ context.Context, _ any, args graphql.Arguments, _ *graphql.ResolveInfo,
	) (any, error) {
		v, supplied := args.Get("filter")
		switch {
		case !supplied:
			return "the caller left it out", nil
		case v == nil:
			return "the caller sent null", nil
		default:
			return fmt.Sprintf("the caller sent %q", v), nil
		}
	}

	const query = `query ($f: String) { search(filter: $f) }`
	for _, variables := range []map[string]graphql.Maybe[any]{
		nil,                               // nothing supplied
		{"f": graphql.Just[any](nil)},     // supplied as null
		{"f": graphql.Just[any]("today")}, // supplied a value
	} {
		result := graphql.Do(context.Background(), graphql.Params{
			Schema: s, Query: query, Variables: variables,
		})
		data, _ := result.Data.Get()
		answer, _ := data.Get("search")
		fmt.Println(answer)
	}

	// Output:
	// the caller left it out
	// the caller sent null
	// the caller sent "today"
}

// A field that fails is null and the failure is reported beside the data, with
// the path to where it happened.
func Example_errors() {
	s, err := graphql.BuildSchema(`type Query { ok: String bad: String }`)
	if err != nil {
		log.Fatal(err)
	}
	s.QueryType().Field("bad").Resolve = func(
		context.Context, any, graphql.Arguments, *graphql.ResolveInfo,
	) (any, error) {
		return nil, fmt.Errorf("the database is down")
	}

	result := graphql.Do(context.Background(), graphql.Params{
		Schema:    s,
		Query:     `{ ok bad }`,
		RootValue: map[string]any{"ok": "fine"},
	})
	_ = json.NewEncoder(os.Stdout).Encode(result)

	// Output:
	// {"errors":[{"message":"the database is down","locations":[{"line":1,"column":6}],"path":["bad"]}],"data":{"ok":"fine","bad":null}}
}

// A subscription's root field returns a channel of whatever the server's own
// machinery produces, and each event is answered as though the operation were
// a query against it.
func Example_subscription() {
	s, err := graphql.BuildSchema(`
		type Message { body: String }
		type Query { placeholder: String }
		type Subscription { messageAdded: Message }
	`)
	if err != nil {
		log.Fatal(err)
	}

	type message struct{ Body string }
	events := make(chan message, 2)
	events <- message{Body: "hello"}
	events <- message{Body: "goodbye"}
	close(events)

	s.SubscriptionType().Field("messageAdded").Subscribe = func(
		context.Context, any, graphql.Arguments, *graphql.ResolveInfo,
	) (any, error) {
		return events, nil
	}

	sub := graphql.Subscribe(context.Background(), graphql.Params{
		Schema: s, Query: `subscription { messageAdded { body } }`,
	})
	if sub.Events == nil {
		log.Fatal(sub.Errors)
	}
	for result := range sub.Events {
		data, _ := result.Data.Get()
		added, _ := data.Get("messageAdded")
		body, _ := added.(*graphql.OrderedMap).Get("body")
		fmt.Println(body)
	}

	// Output:
	// hello
	// goodbye
}

// Which changes between two versions of a schema would break a client, and
// which would merely surprise one.
func Example_findingBreakingChanges() {
	before, err := graphql.BuildSchema(`
		enum Colour { RED GREEN }
		type Query { colour: Colour name: String age: Int }
	`)
	if err != nil {
		log.Fatal(err)
	}
	after, err := graphql.BuildSchema(`
		enum Colour { RED GREEN BLUE }
		type Query { colour: Colour name: String count: Int }
	`)
	if err != nil {
		log.Fatal(err)
	}

	// The changes come in the order the new schema declares things, so a
	// reader can follow them down the file.
	for _, change := range utilities.FindSchemaChanges(before, after) {
		fmt.Printf("%-9s %-12s %s\n", change.Severity, change.Coordinate, change.Message)
	}

	// Output:
	// dangerous Colour.BLUE  Enum value Colour.BLUE was added.
	// breaking  Query.age    Field Query.age was removed.
	// safe      Query.count  Field Query.count was added.
}

// Two spellings of one request strip to the same text, which is what makes a
// cache key.
func Example_requestsAsCacheKeys() {
	a, err := utilities.StripIgnoredCharacters("{\n  user(id: 4) {\n    name\n  }\n}")
	if err != nil {
		log.Fatal(err)
	}
	b, err := utilities.StripIgnoredCharacters("  { user(id:4){name} }  # a comment")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(a)
	fmt.Println(a == b)

	// Output:
	// {user(id:4){name}}
	// true
}

// A schema described to a client, and rebuilt from that description, is the
// same schema.
func Example_introspectionRoundTrip() {
	original, err := graphql.BuildSchema(`
		"A person."
		type User { name: String }
		type Query { me: User }
	`)
	if err != nil {
		log.Fatal(err)
	}

	answer, err := graphql.Introspect(context.Background(), original)
	if err != nil {
		log.Fatal(err)
	}
	rebuilt, err := utilities.BuildClientSchema(answer)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(utilities.PrintSchema(rebuilt) == utilities.PrintSchema(original))
	fmt.Println(strings.SplitN(utilities.PrintSchema(rebuilt), "\n", 2)[0])

	// Output:
	// true
	// """A person."""
}

// A schema written in Go rather than in SDL, which is what a server does when
// its types come from somewhere else.
func Example_schemaInGo() {
	user := schema.NewObject(schema.ObjectConfig{
		Name: "User",
		Fields: []*schema.Field{
			schema.NewField("name", schema.FieldConfig{Type: schema.String}),
		},
	})
	s := schema.New(schema.Config{
		Query: schema.NewObject(schema.ObjectConfig{
			Name: "Query",
			Fields: []*schema.Field{
				schema.NewField("me", schema.FieldConfig{
					Type: user,
					Resolve: func(context.Context, any, graphql.Arguments, *graphql.ResolveInfo) (any, error) {
						return map[string]any{"name": "Ada"}, nil
					},
				}),
			},
		}),
	})
	if err := schema.AssertValidSchema(s); err != nil {
		log.Fatal(err)
	}

	result := graphql.Do(context.Background(), graphql.Params{Schema: s, Query: `{ me { name } }`})
	_ = json.NewEncoder(os.Stdout).Encode(result)

	// Output:
	// {"data":{"me":{"name":"Ada"}}}
}
