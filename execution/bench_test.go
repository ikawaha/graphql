package execution_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

// The introspection query against a real schema is the standard measure: it
// touches every kind of type, nests deeply, and is what every client sends
// first. This is the baseline later work is compared against.
func BenchmarkExecute_Introspection(b *testing.B) {
	body, err := os.ReadFile(filepath.Join("..", "language", "testdata", "github-schema.graphql"))
	if err != nil {
		b.Fatal(err)
	}
	s, err := utilities.BuildSchema(string(body))
	if err != nil {
		b.Fatal(err)
	}
	doc, err := language.ParseString(fullIntrospectionQuery)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		result := execution.Execute(ctx, execution.Request{Schema: s, Document: doc})
		if len(result.Errors) != 0 {
			b.Fatalf("unexpected errors: %v", result.Errors)
		}
	}
}

// A wide, shallow query is what most real requests look like, and it is the
// shape concurrency is meant to help with.
func BenchmarkExecute_WideQuery(b *testing.B) {
	s, err := utilities.BuildSchema(`
		type Query { people: [Person] }
		type Person { name: String age: Int email: String city: String }
	`)
	if err != nil {
		b.Fatal(err)
	}
	if err := schema.AssertValidSchema(s); err != nil {
		b.Fatal(err)
	}
	people := make([]*benchPerson, 200)
	for i := range people {
		people[i] = &benchPerson{Name: "n", Age: int32(i), Email: "e", City: "c"}
	}
	doc, err := language.ParseString(`{ people { name age email city } }`)
	if err != nil {
		b.Fatal(err)
	}
	root := map[string]any{"people": people}
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		result := execution.Execute(ctx, execution.Request{Schema: s, Document: doc, RootValue: root})
		if len(result.Errors) != 0 {
			b.Fatalf("unexpected errors: %v", result.Errors)
		}
	}
}

type benchPerson struct {
	Name  string
	Age   int32
	Email string
	City  string
}

// What Request.Concurrency costs and what it buys.
//
// Resolvers run one after another by default. Letting siblings run alongside
// each other cannot make a resolver that answers from memory any faster — the
// goroutines are pure overhead there — and is worth a great deal when they
// wait on something. Both halves are measured, because a number for either
// one on its own would be read as the whole story.
func BenchmarkExecute_Concurrency(b *testing.B) {
	const fields = 8
	sdl := "type Query {"
	query := "{"
	for i := range fields {
		name := string(rune('a' + i))
		sdl += " " + name + ": String"
		query += " " + name
	}
	sdl += " }"
	query += " }"

	doc, err := language.ParseString(query)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()

	run := func(b *testing.B, resolve schema.FieldResolver, concurrency int) {
		s, err := utilities.BuildSchema(sdl)
		if err != nil {
			b.Fatal(err)
		}
		for _, field := range s.QueryType().Fields() {
			field.Resolve = resolve
		}
		b.ReportAllocs()
		for b.Loop() {
			result := execution.Execute(ctx, execution.Request{
				Schema: s, Document: doc, Concurrency: concurrency,
			})
			if len(result.Errors) != 0 {
				b.Fatalf("unexpected errors: %v", result.Errors)
			}
		}
	}

	// A resolver that answers from memory. Anything above one costs.
	fromMemory := func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		return "x", nil
	}
	// A resolver that waits, which is what a database call looks like from
	// here. Eight of these one after another take eight times as long as one.
	waiting := func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		time.Sleep(100 * time.Microsecond)
		return "x", nil
	}

	for _, tt := range []struct {
		name    string
		resolve schema.FieldResolver
	}{
		{"answering from memory", fromMemory},
		{"waiting", waiting},
	} {
		b.Run(tt.name, func(b *testing.B) {
			for _, concurrency := range []int{1, 4, fields} {
				b.Run(fmt.Sprintf("concurrency=%d", concurrency), func(b *testing.B) {
					run(b, tt.resolve, concurrency)
				})
			}
		})
	}
}

// The entries of a list are worked on alongside one another too, which is the
// shape a request that fetches a list of things has.
func BenchmarkExecute_ConcurrencyOverAList(b *testing.B) {
	const entries = 8
	s, err := utilities.BuildSchema(`
		type Query { people: [Person] }
		type Person { name: String }
	`)
	if err != nil {
		b.Fatal(err)
	}
	// A resolver that waits, which is what a database call looks like from
	// here. Eight of these one after another take eight times as long as one.
	s.Type("Person").(*schema.ObjectType).Field("name").Resolve =
		func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
			time.Sleep(100 * time.Microsecond)
			return "x", nil
		}
	people := make([]any, entries)
	for i := range people {
		people[i] = map[string]any{}
	}
	doc, err := language.ParseString(`{ people { name } }`)
	if err != nil {
		b.Fatal(err)
	}
	root := map[string]any{"people": people}
	ctx := context.Background()

	for _, concurrency := range []int{1, 4, entries} {
		b.Run(fmt.Sprintf("concurrency=%d", concurrency), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				result := execution.Execute(ctx, execution.Request{
					Schema: s, Document: doc, RootValue: root, Concurrency: concurrency,
				})
				if len(result.Errors) != 0 {
					b.Fatalf("unexpected errors: %v", result.Errors)
				}
			}
		})
	}
}

// A field with arguments inside a list is the shape a paged request has, and
// the one where working the arguments out once rather than once per entry
// tells.
func BenchmarkExecute_ArgumentsInAList(b *testing.B) {
	s, err := utilities.BuildSchema(`
		type Query { users: [User] }
		type User { name: String posts(first: Int = 3, order: String): [String] }
	`)
	if err != nil {
		b.Fatal(err)
	}
	s.Type("User").(*schema.ObjectType).Field("posts").Resolve =
		func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
			return []any{"a", "b", "c"}, nil
		}
	users := make([]any, 100)
	for i := range users {
		users[i] = map[string]any{"name": "n"}
	}
	doc, err := language.ParseString(`{ users { name posts(first: 5, order: "date") } }`)
	if err != nil {
		b.Fatal(err)
	}
	root := map[string]any{"users": users}
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		result := execution.Execute(ctx, execution.Request{Schema: s, Document: doc, RootValue: root})
		if len(result.Errors) != 0 {
			b.Fatalf("unexpected errors: %v", result.Errors)
		}
	}
}
