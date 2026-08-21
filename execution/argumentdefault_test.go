package execution_test

// A default declared in the schema is coerced when the argument is left out.
// One that will not coerce fails the field and says why.
//
// ValidateSchema reports such a default too, so a server that checks its
// schema never reaches this. The executor checks because it may be handed a
// schema that was never checked.

import (
	"context"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// mustParseValue reads a value literal, as a schema written in SDL would hold
// one.
func mustParseValue(t *testing.T, body string) language.Value {
	t.Helper()
	v, err := language.ParseValue(language.NewSource(body))
	if err != nil {
		t.Fatalf("parsing %q: %v", body, err)
	}
	return v
}

func TestExecute_AnArgumentDefaultThatWillNotCoerce(t *testing.T) {
	for _, tt := range []struct {
		name string
		// def is the default as the schema holds it: written as a literal, or
		// supplied in Go as a value.
		def  value.Maybe[schema.DefaultInput]
		says string
	}{
		{
			name: "a value supplied in Go",
			def:  schema.DefaultValue("not a number"),
			says: `Argument "Query.f(in:)" has invalid default value: ` +
				`Int cannot represent non-integer value: "not a number"`,
		},
		{
			name: "a literal written in the schema",
			def:  schema.DefaultLiteral(mustParseValue(t, `"not a number"`)),
			says: `Argument "Query.f(in:)" has invalid default value: ` +
				`Int cannot represent non-integer value: "not a number"`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			query := schema.NewObject(schema.ObjectConfig{
				Name: "Query",
				Fields: []*schema.Field{
					schema.NewField("f", schema.FieldConfig{
						Type: schema.String,
						Args: []*schema.Argument{
							schema.NewArgument("in", schema.ArgumentConfig{
								Type: schema.Int, Default: tt.def,
							}),
						},
						Resolve: func(
							_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo,
						) (any, error) {
							v, _ := args.Get("in")
							return value.Describe(v), nil
						},
					}),
				},
			})
			// The schema is unsound, which is the premise: this is the check
			// that catches a server which never looked.
			if errs := schema.ValidateSchema(schema.New(schema.Config{Query: query})); len(errs) == 0 {
				t.Fatal("the schema was accepted, so there is nothing for the executor to catch")
			}

			// Executed against one built as sound, since an executor refuses
			// an unsound schema outright; what is being checked here is what
			// it does with the default once it is running.
			s := schema.New(schema.Config{AssumeValid: true, Query: query})
			result := execution.Execute(context.Background(), execution.Request{
				Schema: s, Document: mustParse(t, `{ f }`),
			})
			if len(result.Errors) != 1 {
				t.Fatalf("errors = %v, want one", result.Errors)
			}
			if got := result.Errors[0].Message; got != tt.says {
				t.Errorf("says %q\nwant %q", got, tt.says)
			}
			if got := jsonOf(t, result); !strings.Contains(got, `"f":null`) {
				t.Errorf("response = %s, want the field nulled", got)
			}
		})
	}
}

// A default that does coerce is used, which is the ordinary case the one above
// is the failure of.
func TestExecute_AnArgumentDefaultThatCoerces(t *testing.T) {
	s := schema.New(schema.Config{
		Query: schema.NewObject(schema.ObjectConfig{
			Name: "Query",
			Fields: []*schema.Field{
				schema.NewField("f", schema.FieldConfig{
					Type: schema.String,
					Args: []*schema.Argument{
						schema.NewArgument("in", schema.ArgumentConfig{
							Type: schema.Int, Default: schema.DefaultValue(int32(7)),
						}),
					},
					Resolve: func(
						_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo,
					) (any, error) {
						v, _ := args.Get("in")
						return value.Describe(v), nil
					},
				}),
			},
		}),
	})
	if errs := schema.ValidateSchema(s); len(errs) != 0 {
		t.Fatalf("the schema is not sound: %v", errs)
	}

	result := execution.Execute(context.Background(), execution.Request{
		Schema: s, Document: mustParse(t, `{ f }`),
	})
	if got := jsonOf(t, result); got != `{"data":{"f":"7"}}` {
		t.Errorf("response = %s", got)
	}
}
