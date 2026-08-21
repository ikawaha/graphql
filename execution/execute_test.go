package execution_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/schema"
)

func TestExecute_Simple(t *testing.T) {
	s := buildSchema(t, `
		type Query {
			hello: String
			count: Int
			ratio: Float
			flag: Boolean
			id: ID
		}
	`)
	root := map[string]any{
		"hello": "world",
		"count": 3,
		"ratio": 1.5,
		"flag":  true,
		"id":    "abc",
	}

	// The keys come back in the order the document asked for them, not the
	// order the source happens to hold them in.
	expectJSON(t, s, `{ count hello flag ratio id }`,
		execution.Request{RootValue: root},
		`{"data":{"count":3,"hello":"world","flag":true,"ratio":1.5,"id":"abc"}}`)
}

func TestExecute_NestedObjects(t *testing.T) {
	s := buildSchema(t, `
		type Query { me: User }
		type User { name: String friend: User pets: [Pet] }
		type Pet { name: String }
	`)
	root := map[string]any{
		"me": map[string]any{
			"name": "Ada",
			"friend": map[string]any{
				"name":   "Grace",
				"friend": nil,
			},
			"pets": []any{
				map[string]any{"name": "Cat"},
				map[string]any{"name": "Dog"},
			},
		},
	}

	expectJSON(t, s, `{ me { name friend { name friend { name } } pets { name } } }`,
		execution.Request{RootValue: root},
		`{"data":{"me":{"name":"Ada","friend":{"name":"Grace","friend":null},`+
			`"pets":[{"name":"Cat"},{"name":"Dog"}]}}}`)
}

// The same field asked for twice is one field with one value, and what each
// asks for underneath is merged.
func TestExecute_MergesSelections(t *testing.T) {
	s := buildSchema(t, `
		type Query { me: User }
		type User { name: String nickname: String }
	`)
	root := map[string]any{"me": map[string]any{"name": "Ada", "nickname": "A"}}

	expectJSON(t, s, `{ me { name } me { nickname } }`,
		execution.Request{RootValue: root},
		`{"data":{"me":{"name":"Ada","nickname":"A"}}}`)

	// An alias makes them two places in the response rather than one.
	expectJSON(t, s, `{ a: me { name } b: me { nickname } }`,
		execution.Request{RootValue: root},
		`{"data":{"a":{"name":"Ada"},"b":{"nickname":"A"}}}`)
}

// A field that fails is null and the failure is reported with where it
// happened, so a client can tell which part of the response is missing.
func TestExecute_FieldError(t *testing.T) {
	s := buildSchema(t, `
		type Query { ok: String bad: String }
	`)
	query := s.QueryType()
	query.Field("bad").Resolve = func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		return nil, errors.New("no good")
	}

	result := run(t, s, `{ ok bad }`, execution.Request{RootValue: map[string]any{"ok": "fine"}})

	if got := jsonOf(t, result); got != `{"errors":[{"message":"no good","locations":[{"line":1,"column":6}],"path":["bad"]}],"data":{"ok":"fine","bad":null}}` {
		t.Errorf("response =\n  %s", got)
	}
}
