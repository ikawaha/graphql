package graphql_test

// The three states a GraphQL input can be in — not supplied, supplied as null,
// supplied with a value — are the distinction this library is built around,
// and the one Go does not have a spelling for. These check that each survives
// the whole way from a request body to what a resolver is handed, at every
// place the distinction can be made: a variable, an argument, a field of an
// input object, and a field of one nested inside another.
//
// What makes the difference observable is the default: a value that was not
// supplied falls back to a default, and one supplied as null does not.

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ikawaha/graphql"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// threeStateSchema answers with a description of what each argument was: the
// word for the state, or the value itself.
func threeStateSchema(t *testing.T) *schema.Schema {
	t.Helper()
	state := func(held any, present bool) string {
		switch {
		case !present:
			return "absent"
		case held == nil:
			return "null"
		default:
			return fmt.Sprintf("%v", held)
		}
	}
	inner := schema.NewInputObject(schema.InputObjectConfig{
		Name: "Inner",
		Fields: []*schema.InputField{
			schema.NewInputField("plain", schema.InputFieldConfig{Type: schema.String}),
			schema.NewInputField("defaulted", schema.InputFieldConfig{
				Type: schema.String, Default: schema.DefaultValue("D"),
			}),
		},
	})
	outer := schema.NewInputObject(schema.InputObjectConfig{
		Name:   "Outer",
		Fields: []*schema.InputField{schema.NewInputField("inner", schema.InputFieldConfig{Type: inner})},
	})
	describeInner := func(held any) string {
		if held == nil {
			return "null"
		}
		fields := held.(map[string]any)
		out := ""
		for _, name := range []string{"plain", "defaulted"} {
			v, ok := fields[name]
			out += name + "=" + state(v, ok) + " "
		}
		return out
	}
	return schema.New(schema.Config{Query: schema.NewObject(schema.ObjectConfig{
		Name: "Query",
		Fields: []*schema.Field{
			schema.NewField("argument", schema.FieldConfig{
				Type: schema.String,
				Args: []*schema.Argument{
					schema.NewArgument("plain", schema.ArgumentConfig{Type: schema.String}),
					schema.NewArgument("defaulted", schema.ArgumentConfig{
						Type: schema.String, Default: schema.DefaultValue("D"),
					}),
				},
				Resolve: func(_ context.Context, _ any, a schema.Arguments, _ *schema.ResolveInfo) (any, error) {
					out := ""
					for _, name := range []string{"plain", "defaulted"} {
						held, present := a.Get(name)
						out += name + "=" + state(held, present) + " "
					}
					return out, nil
				},
			}),
			schema.NewField("object", schema.FieldConfig{
				Type: schema.String,
				Args: []*schema.Argument{schema.NewArgument("in", schema.ArgumentConfig{Type: inner})},
				Resolve: func(_ context.Context, _ any, a schema.Arguments, _ *schema.ResolveInfo) (any, error) {
					held, present := a.Get("in")
					if !present {
						return "absent", nil
					}
					return describeInner(held), nil
				},
			}),
			schema.NewField("nested", schema.FieldConfig{
				Type: schema.String,
				Args: []*schema.Argument{schema.NewArgument("in", schema.ArgumentConfig{Type: outer})},
				Resolve: func(_ context.Context, _ any, a schema.Arguments, _ *schema.ResolveInfo) (any, error) {
					held, present := a.Get("in")
					if !present {
						return "absent", nil
					}
					fields := held.(map[string]any)
					v, ok := fields["inner"]
					if !ok {
						return "inner=absent", nil
					}
					return "inner=" + describeInner(v), nil
				},
			}),
		},
	})})
}

func TestThreeStates(t *testing.T) {
	s := threeStateSchema(t)
	tests := []struct {
		name  string
		query string
		body  string
		want  string
	}{
		// An argument written in the document.
		{"an argument left out", `{ argument }`, "", `{"data":{"argument":"plain=absent defaulted=D "}}`},
		{"an argument written as null", `{ argument(plain: null, defaulted: null) }`, "",
			`{"data":{"argument":"plain=null defaulted=null "}}`},
		{"an argument given a value", `{ argument(plain: "V", defaulted: "V") }`, "",
			`{"data":{"argument":"plain=V defaulted=V "}}`},

		// An argument whose value is a variable. A variable the request did
		// not supply leaves the argument unsupplied, rather than null.
		{"a variable not supplied", `query($v: String) { argument(plain: $v, defaulted: $v) }`, `{}`,
			`{"data":{"argument":"plain=absent defaulted=D "}}`},
		{"a variable supplied as null", `query($v: String) { argument(plain: $v, defaulted: $v) }`,
			`{"v":null}`, `{"data":{"argument":"plain=null defaulted=null "}}`},
		{"a variable supplied with a value", `query($v: String) { argument(plain: $v, defaulted: $v) }`,
			`{"v":"V"}`, `{"data":{"argument":"plain=V defaulted=V "}}`},

		// A field of an input object, written out.
		{"an input field left out", `{ object(in: {}) }`, "",
			`{"data":{"object":"plain=absent defaulted=D "}}`},
		{"an input field written as null", `{ object(in: {plain: null, defaulted: null}) }`, "",
			`{"data":{"object":"plain=null defaulted=null "}}`},

		// A field of an input object whose value is a variable.
		{"an input field naming a variable not supplied",
			`query($v: String) { object(in: {plain: $v, defaulted: $v}) }`, `{}`,
			`{"data":{"object":"plain=absent defaulted=D "}}`},
		{"an input field naming a variable supplied as null",
			`query($v: String) { object(in: {plain: $v, defaulted: $v}) }`, `{"v":null}`,
			`{"data":{"object":"plain=null defaulted=null "}}`},

		// The whole input object supplied as a variable, which is how a
		// request body usually carries one.
		{"a key left out of the value supplied", `query($in: Inner) { object(in: $in) }`, `{"in":{}}`,
			`{"data":{"object":"plain=absent defaulted=D "}}`},
		{"a key supplied as null", `query($in: Inner) { object(in: $in) }`, `{"in":{"plain":null}}`,
			`{"data":{"object":"plain=null defaulted=D "}}`},
		{"the variable itself not supplied", `query($in: Inner) { object(in: $in) }`, `{}`,
			`{"data":{"object":"absent"}}`},
		{"the variable itself supplied as null", `query($in: Inner) { object(in: $in) }`, `{"in":null}`,
			`{"data":{"object":"null"}}`},

		// One input object inside another, both ways round.
		{"a key left out one level down", `query($in: Outer) { nested(in: $in) }`, `{"in":{"inner":{}}}`,
			`{"data":{"nested":"inner=plain=absent defaulted=D "}}`},
		{"a key supplied as null one level down", `query($in: Outer) { nested(in: $in) }`,
			`{"in":{"inner":{"plain":null,"defaulted":null}}}`,
			`{"data":{"nested":"inner=plain=null defaulted=null "}}`},
		{"an object supplied as null one level down", `query($in: Outer) { nested(in: $in) }`,
			`{"in":{"inner":null}}`, `{"data":{"nested":"inner=null"}}`},
		{"an object left out one level down", `query($in: Outer) { nested(in: $in) }`, `{"in":{}}`,
			`{"data":{"nested":"inner=absent"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The variables are read the way a server reads a request body, so
			// that what is checked is what a caller would really hand over.
			var variables map[string]value.Maybe[any]
			if tt.body != "" {
				if err := json.Unmarshal([]byte(tt.body), &variables); err != nil {
					t.Fatalf("reading the variables: %v", err)
				}
			}
			result := graphql.Do(context.Background(), graphql.Params{
				Schema: s, Query: tt.query, Variables: variables,
			})
			got, err := json.Marshal(result)
			if err != nil {
				t.Fatalf("writing the response: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("\n got %s\nwant %s", got, tt.want)
			}
		})
	}
}
