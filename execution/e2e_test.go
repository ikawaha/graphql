package execution_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
	"github.com/ikawaha/graphql/validation"
	"github.com/ikawaha/graphql/value"
)

// user is the sort of Go type a server would already have, resolved by the
// default resolver rather than by anything written for GraphQL.
type user struct {
	Name    string
	Status  string
	Friends []*user
}

// The parts of this library are only worth having if they fit together, so
// this runs one request through all of them: SDL to a schema, text to a
// document, the document past the rules, and the whole thing to a response.
// It uses the pieces a real server would — variables with a default, a
// fragment on an interface narrowed by an inline one, a directive driven by a
// variable, an enum, a list, and concurrency turned on.
func TestEndToEnd(t *testing.T) {
	s, err := utilities.BuildSchema(`
		enum Status { ACTIVE RETIRED }
		interface Named { name: String! }
		type User implements Named { name: String! status: Status friends: [User!]! }
		type Query { user(name: String!): User }
	`)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	if err := schema.AssertValidSchema(s); err != nil {
		t.Fatalf("the schema is not sound: %v", err)
	}

	s.QueryType().Field("user").Resolve =
		func(_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo) (any, error) {
			name, _ := args.Get("name")
			return &user{
				Name:    name.(string),
				Status:  "ACTIVE",
				Friends: []*user{{Name: "Grace", Status: "RETIRED", Friends: []*user{}}},
			}, nil
		}

	doc, err := language.ParseString(`
		query Profile($name: String!, $withFriends: Boolean! = true) {
			user(name: $name) {
				...Card
				friends @include(if: $withFriends) { ...Card }
			}
		}
		fragment Card on Named {
			name
			... on User { status }
		}
	`)
	if err != nil {
		t.Fatalf("parsing the query: %v", err)
	}
	if errs := validation.Validate(s, doc); len(errs) != 0 {
		t.Fatalf("the query does not validate: %v", errs)
	}

	result := execution.Execute(context.Background(), execution.Request{
		Schema:        s,
		Document:      doc,
		OperationName: "Profile",
		Variables:     map[string]value.Maybe[any]{"name": value.Just[any]("Ada")},
		Concurrency:   4,
	})

	out, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("rendering the response: %v", err)
	}
	want := `{"data":{"user":{"name":"Ada","status":"ACTIVE",` +
		`"friends":[{"name":"Grace","status":"RETIRED"}]}}}`
	if string(out) != want {
		t.Errorf("response =\n  %s\nwant\n  %s", out, want)
	}
}
