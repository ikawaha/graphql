package execution_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/schema"
)

// timestamps is embedded, to check that a Go type composed of parts stands for
// a GraphQL type with all their fields.
type timestamps struct {
	CreatedAt string
}

type person struct {
	timestamps
	Name string
	// A tag says which GraphQL field this one answers to, where the names
	// differ.
	Handle string `graphql:"username"`
	// A field tagged "-" is deliberately not exposed.
	Secret string `graphql:"-"`
	// An unexported field is not exposed either.
	internal string
	Best     *person
	Friends  []*person
}

// A method stands in for a field the struct has no member for.
func (p *person) Greeting() string { return "hello, " + p.Name }

// A method may take the context.
func (p *person) Locale(ctx context.Context) string {
	if v, _ := ctx.Value(localeKey{}).(string); v != "" {
		return v
	}
	return "en"
}

// And may fail.
func (p *person) Risky() (string, error) {
	if p.Name == "" {
		return "", errors.New("no name")
	}
	return "fine", nil
}

type localeKey struct{}

const personSDL = `
	type Person {
		name: String
		username: String
		createdAt: String
		greeting: String
		locale: String
		risky: String
		secret: String
		best: Person
		friends: [Person]
	}
	type Query { me: Person }`

// Go has no property access, so what stands in for graphql-js reading one is
// spelt out. These are the rules a server author has to be able to rely on.
func TestDefaultResolver_Structs(t *testing.T) {
	s := buildSchema(t, personSDL)
	me := &person{
		timestamps: timestamps{CreatedAt: "yesterday"},
		Name:       "Ada",
		Handle:     "ada",
		Secret:     "hidden",
		internal:   "hidden",
		Best:       &person{Name: "Grace"},
		Friends:    []*person{{Name: "Alan"}, nil},
	}
	req := execution.Request{RootValue: map[string]any{"me": me}}

	t.Run("a field matching by name", func(t *testing.T) {
		expectJSON(t, s, `{ me { name } }`, req, `{"data":{"me":{"name":"Ada"}}}`)
	})

	t.Run("a field named by a tag", func(t *testing.T) {
		expectJSON(t, s, `{ me { username } }`, req, `{"data":{"me":{"username":"ada"}}}`)
	})

	t.Run("a field of an embedded struct", func(t *testing.T) {
		expectJSON(t, s, `{ me { createdAt } }`, req, `{"data":{"me":{"createdAt":"yesterday"}}}`)
	})

	// Nothing the author did not mean to expose is exposed, however the schema
	// is written.
	t.Run("what is held back", func(t *testing.T) {
		expectJSON(t, s, `{ me { secret } }`, req, `{"data":{"me":{"secret":null}}}`)
	})

	t.Run("a method standing in for a field", func(t *testing.T) {
		expectJSON(t, s, `{ me { greeting } }`, req, `{"data":{"me":{"greeting":"hello, Ada"}}}`)
	})

	t.Run("a method taking the context", func(t *testing.T) {
		doc := mustParse(t, `{ me { locale } }`)
		ctx := context.WithValue(context.Background(), localeKey{}, "ja")
		result := execution.Execute(ctx, execution.Request{Schema: s, Document: doc, RootValue: req.RootValue})
		if got := jsonOf(t, result); got != `{"data":{"me":{"locale":"ja"}}}` {
			t.Errorf("response = %s", got)
		}
	})

	t.Run("a method that succeeds", func(t *testing.T) {
		expectJSON(t, s, `{ me { risky } }`, req, `{"data":{"me":{"risky":"fine"}}}`)
	})

	t.Run("a method that fails", func(t *testing.T) {
		result := run(t, s, `{ me { risky } }`,
			execution.Request{RootValue: map[string]any{"me": &person{}}})
		if len(result.Errors) != 1 {
			t.Fatalf("%d errors, want 1", len(result.Errors))
		}
		if !strings.Contains(result.Errors[0].Message, "no name") {
			t.Errorf("message = %q", result.Errors[0].Message)
		}
	})

	t.Run("nested objects", func(t *testing.T) {
		expectJSON(t, s, `{ me { best { name } friends { name } } }`, req,
			`{"data":{"me":{"best":{"name":"Grace"},"friends":[{"name":"Alan"},null]}}}`)
	})

	// An absent object has absent fields rather than failing.
	t.Run("a nil pointer", func(t *testing.T) {
		expectJSON(t, s, `{ me { best { name best { name } } } }`,
			execution.Request{RootValue: map[string]any{"me": &person{Name: "Ada"}}},
			`{"data":{"me":{"best":null}}}`)
	})

	// A value rather than a pointer works for the fields, though not for
	// methods declared on the pointer type.
	t.Run("a struct held by value", func(t *testing.T) {
		expectJSON(t, s, `{ me { name } }`,
			execution.Request{RootValue: map[string]any{"me": person{Name: "Ada"}}},
			`{"data":{"me":{"name":"Ada"}}}`)
	})
}

// A field the source has nothing for is null, which is what a missing property
// gives in JavaScript, rather than an error about the server's types.
func TestDefaultResolver_MissingField(t *testing.T) {
	s := buildSchema(t, `type Query { present: String absent: String }`)
	expectJSON(t, s, `{ present absent }`,
		execution.Request{RootValue: map[string]any{"present": "here"}},
		`{"data":{"present":"here","absent":null}}`)

	t.Run("a source that is not an object at all", func(t *testing.T) {
		expectJSON(t, s, `{ present }`, execution.Request{RootValue: 42},
			`{"data":{"present":null}}`)
	})
}

// ResolverFor spares the common resolver from asserting the type of its
// source, and says clearly what happened when the source is not what the
// schema was wired up for.
func TestResolverFor(t *testing.T) {
	s := buildSchema(t, `type Query { me: Person } type Person { name: String shout: String }`)
	s.Type("Person").(*schema.ObjectType).Field("shout").Resolve =
		execution.ResolverFor(func(_ context.Context, p *person) (string, error) {
			return strings.ToUpper(p.Name), nil
		})

	expectJSON(t, s, `{ me { shout } }`,
		execution.Request{RootValue: map[string]any{"me": &person{Name: "Ada"}}},
		`{"data":{"me":{"shout":"ADA"}}}`)

	t.Run("the wrong kind of source", func(t *testing.T) {
		result := run(t, s, `{ me { shout } }`,
			execution.Request{RootValue: map[string]any{"me": map[string]any{"name": "Ada"}}})
		if len(result.Errors) != 1 {
			t.Fatalf("%d errors, want 1", len(result.Errors))
		}
		if !strings.Contains(result.Errors[0].Message, "Person.shout") {
			t.Errorf("message = %q, want it to name the field", result.Errors[0].Message)
		}
	})
}

// A pointer is how a Go type says a value may be absent, so a field that can
// be null is naturally held as one. The value it points at is what the scalar
// or enum is given; a nil one is null.
func TestDefaultResolver_Pointers(t *testing.T) {
	type holder struct {
		Name   *string
		Count  *int32
		Flag   *bool
		Colour *string
		Nested **string
	}
	s := buildSchema(t, `
		enum Colour { RED GREEN }
		type Holder { name: String count: Int flag: Boolean colour: Colour nested: String }
		type Query { held: Holder }
	`)

	name, colour, deep := "Ada", "RED", "deep"
	nested := &deep
	count := int32(3)
	flag := true
	expectJSON(t, s, `{ held { name count flag colour nested } }`,
		execution.Request{RootValue: map[string]any{"held": &holder{
			Name: &name, Count: &count, Flag: &flag, Colour: &colour, Nested: &nested,
		}}},
		`{"data":{"held":{"name":"Ada","count":3,"flag":true,"colour":"RED","nested":"deep"}}}`)

	// A nil pointer is null rather than a fault, which is the whole reason for
	// holding a nullable field as one.
	t.Run("nil is null", func(t *testing.T) {
		expectJSON(t, s, `{ held { name count flag colour nested } }`,
			execution.Request{RootValue: map[string]any{"held": &holder{}}},
			`{"data":{"held":{"name":null,"count":null,"flag":null,"colour":null,"nested":null}}}`)
	})

	// A field that may not be null and is held as a nil pointer is a fault in
	// the server, reported rather than passed on.
	t.Run("nil where null is not allowed", func(t *testing.T) {
		s := buildSchema(t, `type Holder { name: String! } type Query { held: Holder }`)
		result := run(t, s, `{ held { name } }`,
			execution.Request{RootValue: map[string]any{"held": &holder{}}})
		if len(result.Errors) != 1 {
			t.Fatalf("%d errors, want 1", len(result.Errors))
		}
		if got := jsonOf(t, result); !strings.Contains(got, `"held":null`) {
			t.Errorf("response = %s", got)
		}
	})
}
