package graphql_test

// Ported from graphql-js src/type/__tests__/enumType-test.ts. These go through
// the whole of Do — parse, check, run — because what they are about is how an
// enum behaves as an argument and as a result, and the checking is half of it.

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/ikawaha/graphql"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

func TestPortedEnumType(t *testing.T) {
	runPortedEnumCases(t, []portedEnumCase{
		{
			name:  `accepts enum literals as input`,
			query: `{ colorInt(fromEnum: GREEN) }`,
			want:  `{"data": {"colorInt": 1}}`,
		},
		{
			name:  `enum may be output type`,
			query: `{ colorEnum(fromInt: 1) }`,
			want:  `{"data": {"colorEnum": "GREEN"}}`,
		},
		{
			name:  `enum may be both input and output type`,
			query: `{ colorEnum(fromEnum: GREEN) }`,
			want:  `{"data": {"colorEnum": "GREEN"}}`,
		},
		{
			name:  `does not accept string literals`,
			query: `{ colorEnum(fromEnum: "GREEN") }`,
			want:  `{"errors": [{"message": "Enum \"Color\" cannot represent non-enum value: \"GREEN\". Did you mean the enum value \"GREEN\"?", "locations": [{"line": 1, "column": 23}]}]}`,
		},
		{
			name:  `does not accept values not in the enum`,
			query: `{ colorEnum(fromEnum: GREENISH) }`,
			want:  `{"errors": [{"message": "Value \"GREENISH\" does not exist in \"Color\" enum. Did you mean the enum value \"GREEN\"?", "locations": [{"line": 1, "column": 23}]}]}`,
		},
		{
			name:  `does not accept values with incorrect casing`,
			query: `{ colorEnum(fromEnum: green) }`,
			want:  `{"errors": [{"message": "Value \"green\" does not exist in \"Color\" enum. Did you mean the enum value \"GREEN\" or \"RED\"?", "locations": [{"line": 1, "column": 23}]}]}`,
		},
		{
			name:  `does not accept incorrect internal value`,
			query: `{ colorEnum(fromString: "GREEN") }`,
			want:  `{"data": {"colorEnum": null}, "errors": [{"message": "Enum \"Color\" cannot represent value: \"GREEN\"", "locations": [{"line": 1, "column": 3}], "path": ["colorEnum"]}]}`,
		},
		{
			name:  `does not accept internal value in place of enum literal`,
			query: `{ colorEnum(fromEnum: 1) }`,
			want:  `{"errors": [{"message": "Enum \"Color\" cannot represent non-enum value: 1.", "locations": [{"line": 1, "column": 23}]}]}`,
		},
		{
			name:  `does not accept enum literal in place of int`,
			query: `{ colorEnum(fromInt: GREEN) }`,
			want:  `{"errors": [{"message": "Int cannot represent non-integer value: GREEN", "locations": [{"line": 1, "column": 22}]}]}`,
		},
		{
			name: `enum value may have an internal value of 0`,
			query: `
      {
        colorEnum(fromEnum: RED)
        colorInt(fromEnum: RED)
      }
    `,
			want: `{"data": {"colorEnum": "RED", "colorInt": 0}}`,
		},
		{
			name: `enum inputs may be nullable`,
			query: `
      {
        colorEnum
        colorInt
      }
    `,
			want: `{"data": {"colorEnum": null, "colorInt": null}}`,
		},
		{
			name: `may be internally represented with complex values`,
			query: `
      {
        first: complexEnum
        second: complexEnum(fromEnum: TWO)
        good: complexEnum(provideGoodValue: true)
        bad: complexEnum(provideBadValue: true)
      }
    `,
			want: `{"data": {"first": "ONE", "second": "TWO", "good": "TWO", "bad": null}, "errors": [{"message": "Enum \"Complex\" cannot represent value: { someRandomValue: 123 }", "locations": [{"line": 6, "column": 9}], "path": ["bad"]}]}`,
		},
		{
			name:  `may have values specified via a callback`,
			query: `{ thunkValuesString(fromEnum: B) }`,
			want:  `{"data": {"thunkValuesString": "b"}}`,
		},
	})
}

// Not ported, because each of these is written in a way this could not
// follow:
//   - accepts JSON string as enum variable: the query is not a literal
//   - accepts enum literals as input arguments to mutations: the query is not a literal
//   - accepts enum literals as input arguments to subscriptions: the query is not a literal
//   - can be introspected without error: it does not call executeQuery
//   - does not accept internal value as enum variable: the query is not a literal
//   - does not accept internal value variable as enum input: the query is not a literal
//   - does not accept string variables as enum input: the query is not a literal
//   - does not accept values not in the enum (no suggestions): the variables are not a literal
//   - presents a getValue() API for complex enums: it does not call executeQuery
//   - presents a getValues() API for complex enums: it does not call executeQuery

// portedEnumCase is one of graphql-js's cases: a document, the variables a
// request supplied, and the whole response expected back.
type portedEnumCase struct {
	name      string
	query     string
	variables string
	want      string
}

// knownEnumDivergences are the cases this implementation does not match, and
// why. Each is asserted to *still* diverge, so that closing one cannot go
// unnoticed.
var knownEnumDivergences = map[string]string{
	// The value the message shows is a Go struct, whose fields have to be
	// exported to be seen, so it reads "{ Number: 123 }" where graphql-js
	// reads "{ someRandomValue: 123 }". Everything else about the case
	// matches.
	"may be internally represented with complex values": "a Go struct's fields are named as Go names them",
}

func runPortedEnumCases(t *testing.T, cases []portedEnumCase) {
	t.Helper()
	s := portedEnumSchema(t)
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result := graphql.Do(context.Background(), graphql.Params{
				Schema:    s,
				Query:     tt.query,
				Variables: portedEnumVariables(t, tt.variables),
			})
			got := decodeJSONValue(t, mustJSON(t, result))
			want := decodeJSONValue(t, tt.want)

			if why, listed := knownEnumDivergences[tt.name]; listed {
				if reflect.DeepEqual(got, want) {
					t.Errorf("this case now matches graphql-js; remove it from the known divergences (%s)", why)
				} else {
					t.Logf("known divergence: %s", why)
				}
				return
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("response =\n%s\nwant\n%s", mustJSON(t, result), tt.want)
			}
		})
	}
}

// portedEnumSchema is graphql-js's own schema from enumType-test.ts. Its enum
// members hold numbers and objects rather than their own names, which is what
// the cases are about.
func portedEnumSchema(t *testing.T) *graphql.Schema {
	t.Helper()
	colour := schema.NewEnum(schema.EnumConfig{
		Name: "Color",
		Values: []*schema.EnumValue{
			schema.NewEnumValue("RED", schema.EnumValueConfig{Value: schema.InternalValue(0)}),
			schema.NewEnumValue("GREEN", schema.EnumValueConfig{Value: schema.InternalValue(1)}),
			schema.NewEnumValue("BLUE", schema.EnumValueConfig{Value: schema.InternalValue(2)}),
		},
	})
	// The two values a Complex member holds are told apart by identity, which
	// is what makes returning a value of the same shape but not the same one
	// a mistake.
	one := &complexEnumValue{Name: "one"}
	two := &complexEnumValue{Number: 123}
	complexEnum := schema.NewEnum(schema.EnumConfig{
		Name: "Complex",
		Values: []*schema.EnumValue{
			schema.NewEnumValue("ONE", schema.EnumValueConfig{Value: schema.InternalValue(one)}),
			schema.NewEnumValue("TWO", schema.EnumValueConfig{Value: schema.InternalValue(two)}),
		},
	})
	// graphql-js writes this enum's members lazily, to check that they are
	// read when they are wanted. An enum's members name no types, so nothing
	// here needs delaying and they are written out.
	thunked := schema.NewEnum(schema.EnumConfig{
		Name: "ThunkValues",
		Values: []*schema.EnumValue{
			schema.NewEnumValue("A", schema.EnumValueConfig{Value: schema.InternalValue("a")}),
			schema.NewEnumValue("B", schema.EnumValueConfig{Value: schema.InternalValue("b")}),
		},
	})

	first := func(args schema.Arguments, names ...string) any {
		for _, name := range names {
			if held, supplied := args.Get(name); supplied {
				return held
			}
		}
		return nil
	}
	answer := func(names ...string) schema.FieldResolver {
		return func(_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo) (any, error) {
			return first(args, names...), nil
		}
	}

	query := schema.NewObject(schema.ObjectConfig{
		Name: "Query",
		Fields: []*schema.Field{
			schema.NewField("colorEnum", schema.FieldConfig{
				Type: colour,
				Args: []*schema.Argument{
					schema.NewArgument("fromEnum", schema.ArgumentConfig{Type: colour}),
					schema.NewArgument("fromInt", schema.ArgumentConfig{Type: schema.Int}),
					schema.NewArgument("fromString", schema.ArgumentConfig{Type: schema.String}),
				},
				Resolve: answer("fromInt", "fromString", "fromEnum"),
			}),
			schema.NewField("colorInt", schema.FieldConfig{
				Type: schema.Int,
				Args: []*schema.Argument{
					schema.NewArgument("fromEnum", schema.ArgumentConfig{Type: colour}),
				},
				Resolve: answer("fromEnum"),
			}),
			schema.NewField("complexEnum", schema.FieldConfig{
				Type: complexEnum,
				Args: []*schema.Argument{
					schema.NewArgument("fromEnum", schema.ArgumentConfig{
						Type:    complexEnum,
						Default: value.Just(schema.DefaultInput{Value: "ONE"}),
					}),
					schema.NewArgument("provideGoodValue", schema.ArgumentConfig{Type: schema.Boolean}),
					schema.NewArgument("provideBadValue", schema.ArgumentConfig{Type: schema.Boolean}),
				},
				Resolve: func(
					_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo,
				) (any, error) {
					if good, _ := args.Get("provideGoodValue"); good == true {
						return two, nil
					}
					if bad, _ := args.Get("provideBadValue"); bad == true {
						// The same shape, but not the same value: a member is
						// recognised by which value it is, not what it looks
						// like.
						return &complexEnumValue{Number: 123}, nil
					}
					held, _ := args.Get("fromEnum")
					return held, nil
				},
			}),
			schema.NewField("thunkValuesString", schema.FieldConfig{
				Type: schema.String,
				Args: []*schema.Argument{
					schema.NewArgument("fromEnum", schema.ArgumentConfig{Type: thunked}),
				},
				Resolve: answer("fromEnum"),
			}),
		},
	})
	mutation := schema.NewObject(schema.ObjectConfig{
		Name: "Mutation",
		Fields: []*schema.Field{
			schema.NewField("favoriteEnum", schema.FieldConfig{
				Type:    colour,
				Args:    []*schema.Argument{schema.NewArgument("color", schema.ArgumentConfig{Type: colour})},
				Resolve: answer("color"),
			}),
		},
	})
	subscription := schema.NewObject(schema.ObjectConfig{
		Name: "Subscription",
		Fields: []*schema.Field{
			schema.NewField("subscribeToEnum", schema.FieldConfig{
				Type:    colour,
				Args:    []*schema.Argument{schema.NewArgument("color", schema.ArgumentConfig{Type: colour})},
				Resolve: answer("color"),
			}),
		},
	})

	s := schema.New(schema.Config{Query: query, Mutation: mutation, Subscription: subscription})
	if err := schema.AssertValidSchema(s); err != nil {
		t.Fatalf("the ported schema is not sound: %v", err)
	}
	return s
}

func portedEnumVariables(t *testing.T, encoded string) map[string]graphql.Maybe[any] {
	t.Helper()
	if encoded == "" {
		return nil
	}
	var supplied map[string]any
	decoder := json.NewDecoder(strings.NewReader(encoded))
	decoder.UseNumber()
	if err := decoder.Decode(&supplied); err != nil {
		t.Fatalf("reading the variables: %v", err)
	}
	variables := make(map[string]graphql.Maybe[any], len(supplied))
	for name, held := range supplied {
		if held == "@@undefined" {
			variables[name] = graphql.Nothing[any]()
			continue
		}
		variables[name] = graphql.Just(held)
	}
	return variables
}

func decodeJSONValue(t *testing.T, encoded string) any {
	t.Helper()
	var out any
	decoder := json.NewDecoder(strings.NewReader(encoded))
	decoder.UseNumber()
	if err := decoder.Decode(&out); err != nil {
		t.Fatalf("reading %s: %v", encoded, err)
	}
	return out
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	encoded, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("writing the response: %v", err)
	}
	return string(encoded)
}

// complexEnumValue is what graphql-js writes as a bare object: a value a
// member holds that is neither a name nor a number, so that which member a
// value is has to be settled by identity.
type complexEnumValue struct {
	Name   string
	Number int
}

// Not ported: `may be internally represented with complex values using legacy
// internal defaults`. graphql-js keeps a second, deprecated way of writing a
// default — one that is used as it stands rather than coerced — and that case
// is about it. There is one way to write a default here.
