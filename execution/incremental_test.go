package execution_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/validation"
)

// A schema has to declare @defer and @stream for a document to use them:
// neither is one of the directives every schema has.
const incrementalSDL = `
	directive @defer(if: Boolean = true, label: String) on FRAGMENT_SPREAD | INLINE_FRAGMENT
	directive @stream(if: Boolean = true, label: String, initialCount: Int = 0) on FIELD

	type Author { name: String bio: String }
	type Post {
		title: String
		body: String
		author: Author
		comments: [String]
	}
	type Query {
		post: Post
		posts: [Post]
		names: [String]
	}`

// runIncrementally checks the document and runs it, returning the whole
// exchange as JSON: the first payload, then each that followed.
func runIncrementally(
	ctx context.Context,
	t *testing.T,
	s *schema.Schema,
	query string,
	req execution.Request,
) []string {
	t.Helper()
	doc, err := language.ParseString(query)
	if err != nil {
		t.Fatalf("parsing: %v\n%s", err, query)
	}
	if errs := validation.Validate(s, doc); len(errs) != 0 {
		t.Fatalf("the test document does not validate: %v\n%s", errs, query)
	}
	req.Schema = s
	req.Document = doc
	result := execution.ExecuteIncrementally(ctx, req)

	payloads := []string{marshal(t, result.Initial)}
	if result.Subsequent == nil {
		return payloads
	}
	timeout := time.After(5 * time.Second)
	for {
		select {
		case payload, open := <-result.Subsequent:
			if !open {
				return payloads
			}
			payloads = append(payloads, marshal(t, payload))
		case <-timeout:
			t.Fatal("the response did not finish within five seconds")
		}
	}
}

func marshal(t *testing.T, v any) string {
	t.Helper()
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("rendering: %v", err)
	}
	return string(out)
}

// assertPayloads compares the whole exchange, which is what a client sees.
func assertPayloads(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%d payloads, want %d:\n%s", len(got), len(want), strings.Join(got, "\n"))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("payload %d =\n  %s\nwant\n  %s", i, got[i], want[i])
		}
	}
}

func incrementalRoot() map[string]any {
	return map[string]any{
		"post": map[string]any{
			"title":    "A title",
			"body":     "A body",
			"author":   map[string]any{"name": "Ada", "bio": "A bio"},
			"comments": []any{"one", "two", "three"},
		},
	}
}

// @defer says a fragment's fields need not hold up the response: what the rest
// of the document asked for is sent first, and the fragment's fields follow.
func TestExecuteIncrementally_Defer(t *testing.T) {
	s := buildSchema(t, incrementalSDL)

	t.Run("an inline fragment", func(t *testing.T) {
		got := runIncrementally(context.Background(), t, s, `
			{
				post {
					title
					... @defer(label: "slow") { body }
				}
			}
		`, execution.Request{RootValue: incrementalRoot()})

		assertPayloads(t, got, []string{
			`{"data":{"post":{"title":"A title"}},"pending":[{"id":"0","path":["post"],"label":"slow"}],"hasNext":true}`,
			`{"hasNext":false,"incremental":[{"id":"0","data":{"body":"A body"}}],"completed":[{"id":"0"}]}`,
		})
	})

	t.Run("a named fragment", func(t *testing.T) {
		got := runIncrementally(context.Background(), t, s, `
			{
				post { title ...Slow @defer(label: "slow") }
			}
			fragment Slow on Post { body }
		`, execution.Request{RootValue: incrementalRoot()})

		assertPayloads(t, got, []string{
			`{"data":{"post":{"title":"A title"}},"pending":[{"id":"0","path":["post"],"label":"slow"}],"hasNext":true}`,
			`{"hasNext":false,"incremental":[{"id":"0","data":{"body":"A body"}}],"completed":[{"id":"0"}]}`,
		})
	})

	// A field asked for both inside a deferred fragment and outside it is not
	// deferred: the client asked for it in the first response, and sending it
	// again later would deliver it twice.
	t.Run("a field asked for both ways", func(t *testing.T) {
		got := runIncrementally(context.Background(), t, s, `
			{
				post {
					title
					... @defer { title body }
				}
			}
		`, execution.Request{RootValue: incrementalRoot()})

		assertPayloads(t, got, []string{
			`{"data":{"post":{"title":"A title"}},"pending":[{"id":"0","path":["post"]}],"hasNext":true}`,
			`{"hasNext":false,"incremental":[{"id":"0","data":{"body":"A body"}}],"completed":[{"id":"0"}]}`,
		})
	})

	// Switching it off asks for nothing to be deferred, so the response is an
	// ordinary one.
	t.Run("switched off", func(t *testing.T) {
		got := runIncrementally(context.Background(), t, s, `
			{
				post { title ... @defer(if: false) { body } }
			}
		`, execution.Request{RootValue: incrementalRoot()})

		assertPayloads(t, got, []string{
			`{"data":{"post":{"title":"A title","body":"A body"}}}`,
		})
	})

	t.Run("driven by a variable", func(t *testing.T) {
		got := runIncrementally(context.Background(), t, s, `
			query ($defer: Boolean!) {
				post { title ... @defer(if: $defer) { body } }
			}
		`, execution.Request{RootValue: incrementalRoot(), Variables: vars(map[string]any{"defer": false})})

		assertPayloads(t, got, []string{
			`{"data":{"post":{"title":"A title","body":"A body"}}}`,
		})
	})

	// A deferred fragment inside another one is deferred again, and is only
	// announced once the fragment holding it has been delivered. Both are
	// delivered in the same payload: a resolver here answers rather than
	// promising to answer, so the inner fragment is ready as soon as the outer
	// one has said where it goes.
	t.Run("nested", func(t *testing.T) {
		got := runIncrementally(context.Background(), t, s, `
			{
				post {
					title
					... @defer(label: "outer") {
						body
						author { name ... @defer(label: "inner") { bio } }
					}
				}
			}
		`, execution.Request{RootValue: incrementalRoot()})

		assertPayloads(t, got, []string{
			`{"data":{"post":{"title":"A title"}},"pending":[{"id":"0","path":["post"],"label":"outer"}],"hasNext":true}`,
			`{"hasNext":false,"pending":[{"id":"1","path":["post","author"],"label":"inner"}],"incremental":[{"id":"0","data":{"body":"A body","author":{"name":"Ada"}}},{"id":"1","data":{"bio":"A bio"}}],"completed":[{"id":"0"},{"id":"1"}]}`,
		})
	})

	// A document that defers nothing gets the whole response at once and no
	// channel to wait on.
	t.Run("nothing deferred", func(t *testing.T) {
		got := runIncrementally(context.Background(), t, s, `{ post { title body } }`,
			execution.Request{RootValue: incrementalRoot()})
		assertPayloads(t, got, []string{
			`{"data":{"post":{"title":"A title","body":"A body"}}}`,
		})
	})
}

// A document asking for parts of the response to arrive later cannot be
// answered with one response, and saying so beats sending something the client
// is not waiting for.
func TestExecute_RefusesIncrementalDelivery(t *testing.T) {
	s := buildSchema(t, incrementalSDL)
	doc, err := language.ParseString(`{ post { title ... @defer { body } } }`)
	if err != nil {
		t.Fatal(err)
	}
	result := execution.Execute(context.Background(),
		execution.Request{Schema: s, Document: doc, RootValue: incrementalRoot()})

	if len(result.Errors) != 1 {
		t.Fatalf("%d errors, want 1", len(result.Errors))
	}
	if !strings.Contains(result.Errors[0].Message, "ExecuteIncrementally") {
		t.Errorf("message = %q, want it to say what to use instead", result.Errors[0].Message)
	}

	// One that is switched off asks for nothing, so it runs as an ordinary
	// query.
	t.Run("switched off", func(t *testing.T) {
		doc, err := language.ParseString(`{ post { title ... @defer(if: false) { body } } }`)
		if err != nil {
			t.Fatal(err)
		}
		result := execution.Execute(context.Background(),
			execution.Request{Schema: s, Document: doc, RootValue: incrementalRoot()})
		if len(result.Errors) != 0 {
			t.Fatalf("errors = %v", result.Errors)
		}
		if got := jsonOf(t, result); !strings.Contains(got, `"body":"A body"`) {
			t.Errorf("response = %s", got)
		}
	})

	// A schema that does not declare the directives costs nothing to check.
	t.Run("a schema without them", func(t *testing.T) {
		plain := buildSchema(t, `type Query { a: String }`)
		doc, err := language.ParseString(`{ a }`)
		if err != nil {
			t.Fatal(err)
		}
		result := execution.Execute(context.Background(),
			execution.Request{Schema: plain, Document: doc, RootValue: map[string]any{"a": "A"}})
		if len(result.Errors) != 0 {
			t.Fatalf("errors = %v", result.Errors)
		}
	})
}

// @stream says the entries of a list past the first few need not hold up the
// response, which is the point: a client sees what a server can produce
// quickly without waiting for the rest.
func TestExecuteIncrementally_Stream(t *testing.T) {
	s := buildSchema(t, incrementalSDL)
	root := map[string]any{"names": []any{"one", "two", "three"}}

	t.Run("the entries past the first few", func(t *testing.T) {
		got := runIncrementally(context.Background(), t, s,
			`{ names @stream(initialCount: 1, label: "rest") }`,
			execution.Request{RootValue: root})

		assertPayloads(t, got, []string{
			`{"data":{"names":["one"]},"pending":[{"id":"0","path":["names"],"label":"rest"}],"hasNext":true}`,
			`{"hasNext":false,"incremental":[{"id":"0","items":["two","three"]}],"completed":[{"id":"0"}]}`,
		})
	})

	t.Run("nothing in the first payload", func(t *testing.T) {
		got := runIncrementally(context.Background(), t, s,
			`{ names @stream(initialCount: 0) }`, execution.Request{RootValue: root})

		if !strings.Contains(got[0], `"names":[]`) {
			t.Errorf("the first payload = %s, want an empty list", got[0])
		}
		assertPayloads(t, got[1:], []string{
			`{"hasNext":false,"incremental":[{"id":"0","items":["one","two","three"]}],"completed":[{"id":"0"}]}`,
		})
	})

	// A list that fits in what was asked for is not streamed at all:
	// announcing a piece that arrives empty tells a client nothing.
	t.Run("a list short enough to fit", func(t *testing.T) {
		got := runIncrementally(context.Background(), t, s,
			`{ names @stream(initialCount: 5) }`, execution.Request{RootValue: root})
		assertPayloads(t, got, []string{
			`{"data":{"names":["one","two","three"]}}`,
		})
	})

	t.Run("switched off", func(t *testing.T) {
		got := runIncrementally(context.Background(), t, s,
			`{ names @stream(if: false, initialCount: 1) }`, execution.Request{RootValue: root})
		assertPayloads(t, got, []string{
			`{"data":{"names":["one","two","three"]}}`,
		})
	})

	// An entry whose fields fail in a way the list survives is delivered as a
	// null alongside the rest, with what went wrong beside it.
	t.Run("an entry that fails", func(t *testing.T) {
		s := buildSchema(t, incrementalSDL)
		s.Type("Post").(*schema.ObjectType).Field("title").Resolve =
			func(_ context.Context, src any, _ schema.Arguments, _ *schema.ResolveInfo) (any, error) {
				title := src.(map[string]any)["title"].(string)
				if title == "bad" {
					return nil, errors.New("cannot read the title")
				}
				return title, nil
			}
		root := map[string]any{"posts": []any{
			map[string]any{"title": "one"},
			map[string]any{"title": "bad"},
			map[string]any{"title": "three"},
		}}

		got := runIncrementally(context.Background(), t, s,
			`{ posts @stream(initialCount: 1) { title } }`, execution.Request{RootValue: root})

		if len(got) != 2 {
			t.Fatalf("%d payloads, want 2:\n%s", len(got), strings.Join(got, "\n"))
		}
		if !strings.Contains(got[1], "cannot read the title") ||
			!strings.Contains(got[1], `"path":["posts",1,"title"]`) {
			t.Errorf("the failing entry = %s", got[1])
		}
		// The stream carries on: one bad entry is not the end of the list.
		if !strings.Contains(got[1], `"items":[{"title":null},{"title":"three"}]`) {
			t.Errorf("the stream did not carry on: %s", got[1])
		}
	})

	// A @defer inside a streamed entry is found when that entry is completed,
	// so it is announced alongside the entry rather than in the first payload.
	t.Run("a deferred fragment inside a streamed entry", func(t *testing.T) {
		root := map[string]any{"posts": []any{
			map[string]any{"title": "one", "body": "first"},
			map[string]any{"title": "two", "body": "second"},
		}}
		got := runIncrementally(context.Background(), t, s, `
			{
				posts @stream(initialCount: 1) {
					title
					... @defer(label: "body") { body }
				}
			}
		`, execution.Request{RootValue: root})

		// The first entry is in the first payload, and its deferred fragment
		// is announced there.
		if !strings.Contains(got[0], `"posts":[{"title":"one"}]`) {
			t.Errorf("the first payload = %s", got[0])
		}
		if !strings.Contains(got[0], `"path":["posts",0],"label":"body"`) {
			t.Errorf("the deferred fragment of the first entry was not announced: %s", got[0])
		}
		// The second entry brings its own, announced with it.
		joined := strings.Join(got, "\n")
		if !strings.Contains(joined, `"path":["posts",1],"label":"body"`) {
			t.Errorf("the deferred fragment of the streamed entry was not announced:\n%s", joined)
		}
		for _, wanted := range []string{`{"body":"first"}`, `{"body":"second"}`} {
			if !strings.Contains(joined, wanted) {
				t.Errorf("the exchange does not contain %s:\n%s", wanted, joined)
			}
		}
		// And the last payload says there is nothing more.
		if !strings.Contains(got[len(got)-1], `"hasNext":false`) {
			t.Errorf("the last payload does not end the response: %s", got[len(got)-1])
		}
	})
}

// This is the property the whole of incremental delivery rests on: a client
// that puts the payloads back together must end up with exactly the response
// it would have got had nothing been deferred. Anything else — a field
// delivered twice, one delivered nowhere, a list entry out of order — shows up
// here and nowhere else.
func TestExecuteIncrementally_PayloadsReassemble(t *testing.T) {
	s := buildSchema(t, incrementalSDL)
	root := map[string]any{
		"post": map[string]any{
			"title":    "A title",
			"body":     "A body",
			"author":   map[string]any{"name": "Ada", "bio": "A bio"},
			"comments": []any{"one", "two", "three"},
		},
		"posts": []any{
			map[string]any{"title": "first", "body": "1", "author": map[string]any{"name": "A", "bio": "a"}},
			map[string]any{"title": "second", "body": "2", "author": map[string]any{"name": "B", "bio": "b"}},
			map[string]any{"title": "third", "body": "3", "author": map[string]any{"name": "C", "bio": "c"}},
		},
		"names": []any{"one", "two", "three", "four"},
	}

	queries := []struct {
		name       string
		deferred   string
		equivalent string
	}{
		{
			name:       "a deferred fragment",
			deferred:   `{ post { title ... @defer { body } } }`,
			equivalent: `{ post { title body } }`,
		},
		{
			name:       "several at once",
			deferred:   `{ post { title ... @defer { body } author { name ... @defer { bio } } } }`,
			equivalent: `{ post { title body author { name bio } } }`,
		},
		{
			name:       "nested deferrals",
			deferred:   `{ post { title ... @defer { body author { name ... @defer { bio } } } } }`,
			equivalent: `{ post { title body author { name bio } } }`,
		},
		{
			name:       "a streamed list",
			deferred:   `{ names @stream(initialCount: 2) }`,
			equivalent: `{ names }`,
		},
		{
			name:       "a stream starting from nothing",
			deferred:   `{ names @stream(initialCount: 0) }`,
			equivalent: `{ names }`,
		},
		{
			name:       "a deferred fragment inside a streamed entry",
			deferred:   `{ posts @stream(initialCount: 1) { title ... @defer { body } } }`,
			equivalent: `{ posts { title body } }`,
		},
		{
			name:       "a stream and a deferral side by side",
			deferred:   `{ names @stream(initialCount: 1) post { title ... @defer { body } } }`,
			equivalent: `{ names post { title body } }`,
		},
		{
			name:       "a field asked for both inside and outside",
			deferred:   `{ post { title ... @defer { title body } } }`,
			equivalent: `{ post { title body } }`,
		},
		{
			// Two deferrals at one position overlap: the fields are delivered
			// once, and both are completed.
			name:       "two deferrals at one position",
			deferred:   `{ post { title ... @defer(label: "a") { body } ... @defer(label: "b") { body } } }`,
			equivalent: `{ post { title body } }`,
		},
	}

	for _, tt := range queries {
		t.Run(tt.name, func(t *testing.T) {
			assembled := assemble(context.Background(), t, s, tt.deferred, root)

			plain := run(t, s, tt.equivalent, execution.Request{RootValue: root})
			if len(plain.Errors) != 0 {
				t.Fatalf("the equivalent query failed: %v", plain.Errors)
			}
			data, _ := plain.Data.Get()

			// What is compared is the data, not the order it is written in: a
			// client putting payloads back together holds them in whatever its
			// own map does, and the order of the first payload is pinned by the
			// tests that compare it as text.
			var want any
			if err := json.Unmarshal([]byte(marshal(t, data)), &want); err != nil {
				t.Fatalf("reading the equivalent response: %v", err)
			}
			if !reflect.DeepEqual(assembled, want) {
				t.Errorf("reassembled =\n  %s\nwant\n  %s",
					marshal(t, assembled), marshal(t, want))
			}
		})
	}
}

// assemble runs a document incrementally and puts the payloads back together
// the way a client would.
func assemble(ctx context.Context, t *testing.T, s *schema.Schema, query string, root any) any {
	t.Helper()
	doc, err := language.ParseString(query)
	if err != nil {
		t.Fatalf("parsing: %v\n%s", err, query)
	}
	if errs := validation.Validate(s, doc); len(errs) != 0 {
		t.Fatalf("the test document does not validate: %v", errs)
	}
	result := execution.ExecuteIncrementally(ctx,
		execution.Request{Schema: s, Document: doc, RootValue: root})

	data, present := result.Initial.Data.Get()
	if !present {
		t.Fatalf("the first payload has no data: %v", result.Initial.Errors)
	}
	// Working on the parsed JSON keeps the client's side of this honest: it
	// only has what went over the wire.
	var assembled any
	if err := json.Unmarshal([]byte(marshal(t, data)), &assembled); err != nil {
		t.Fatalf("reading the first payload: %v", err)
	}

	paths := map[string][]any{}
	for _, pending := range result.Initial.Pending {
		paths[pending.ID] = pending.Path
	}
	if result.Subsequent == nil {
		return assembled
	}

	seen := map[string]bool{}
	timeout := time.After(5 * time.Second)
	for done := false; !done; {
		select {
		case payload, open := <-result.Subsequent:
			if !open {
				done = true
				break
			}
			for _, pending := range payload.Pending {
				paths[pending.ID] = pending.Path
			}
			for _, incremental := range payload.Incremental {
				path, known := paths[incremental.ID]
				if !known {
					t.Fatalf("payload %q was never announced", incremental.ID)
				}
				if incremental.Data != nil {
					var fields any
					if err := json.Unmarshal([]byte(marshal(t, incremental.Data)), &fields); err != nil {
						t.Fatalf("reading a payload: %v", err)
					}
					mergeAt(t, assembled, path, fields.(map[string]any))
				}
				for _, entry := range incremental.Items {
					var parsed any
					if err := json.Unmarshal([]byte(marshal(t, entry)), &parsed); err != nil {
						t.Fatalf("reading an entry: %v", err)
					}
					appendAt(t, assembled, path, parsed)
				}
			}
			for _, completed := range payload.Completed {
				if seen[completed.ID] {
					t.Errorf("%q was completed twice", completed.ID)
				}
				seen[completed.ID] = true
			}
			if !payload.HasNext {
				// Every announced piece has to have been completed by the end,
				// or a client would still be waiting.
				for id := range paths {
					if !seen[id] {
						t.Errorf("%q was announced but never completed", id)
					}
				}
			}
		case <-timeout:
			t.Fatal("the response did not finish within five seconds")
		}
	}
	return assembled
}

// mergeAt puts a deferred fragment's fields into the object they belong to.
func mergeAt(t *testing.T, root any, path []any, fields map[string]any) {
	t.Helper()
	at := walkTo(t, root, path)
	object, isObject := at.(map[string]any)
	if !isObject {
		t.Fatalf("the payload at %v belongs to a %T, not an object", path, at)
	}
	for key, v := range fields {
		if _, already := object[key]; already {
			t.Errorf("%q at %v was delivered twice", key, path)
		}
		object[key] = v
	}
}

// appendAt adds a streamed entry to the list it belongs to.
func appendAt(t *testing.T, root any, path []any, entry any) {
	t.Helper()
	if len(path) == 0 {
		t.Fatal("a streamed entry belongs nowhere")
	}
	parent := walkTo(t, root, path[:len(path)-1])
	object, isObject := parent.(map[string]any)
	if !isObject {
		t.Fatalf("the list at %v is not held by an object", path)
	}
	key, isKey := path[len(path)-1].(string)
	if !isKey {
		t.Fatalf("the last step of %v is not a field name", path)
	}
	list, isList := object[key].([]any)
	if !isList {
		t.Fatalf("%v is a %T, not a list", path, object[key])
	}
	object[key] = append(list, entry)
}

// walkTo follows a response path to what it names.
func walkTo(t *testing.T, at any, path []any) any {
	t.Helper()
	for _, step := range path {
		switch key := step.(type) {
		case string:
			object, isObject := at.(map[string]any)
			if !isObject {
				t.Fatalf("cannot follow %q into a %T", key, at)
			}
			at = object[key]
		case float64:
			list, isList := at.([]any)
			if !isList {
				t.Fatalf("cannot follow an index into a %T", at)
			}
			at = list[int(key)]
		case int:
			list, isList := at.([]any)
			if !isList {
				t.Fatalf("cannot follow an index into a %T", at)
			}
			at = list[key]
		default:
			t.Fatalf("a path step is a %T", step)
		}
	}
	return at
}

// A field that fails inside a deferred fragment is reported in that
// fragment's payload, alongside whatever else it delivered.
func TestExecuteIncrementally_ErrorsInAPayload(t *testing.T) {
	t.Run("a nullable field", func(t *testing.T) {
		s := buildSchema(t, incrementalSDL)
		s.Type("Post").(*schema.ObjectType).Field("body").Resolve =
			func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
				return nil, errors.New("cannot read the body")
			}
		got := runIncrementally(context.Background(), t, s,
			`{ post { title ... @defer { body author { name } } } }`,
			execution.Request{RootValue: incrementalRoot()})

		if len(got) != 2 {
			t.Fatalf("%d payloads, want 2:\n%s", len(got), strings.Join(got, "\n"))
		}
		// The rest of the fragment still arrives; only the field that failed
		// is null.
		if !strings.Contains(got[1], `"body":null`) || !strings.Contains(got[1], `"name":"Ada"`) {
			t.Errorf("payload = %s", got[1])
		}
		if !strings.Contains(got[1], "cannot read the body") ||
			!strings.Contains(got[1], `"path":["post","body"]`) {
			t.Errorf("the error is not reported with where it happened: %s", got[1])
		}
		if !strings.Contains(got[1], `"completed":[{"id":"0"}]`) {
			t.Errorf("the piece did not complete: %s", got[1])
		}
	})

	// A field that may not be null has nowhere to put one, so the piece
	// delivers nothing and completes with the reason. A client told this
	// treats what it was waiting for as missing rather than merely late.
	t.Run("a field that may not be null", func(t *testing.T) {
		s := buildSchema(t, `
			directive @defer(if: Boolean = true, label: String) on FRAGMENT_SPREAD | INLINE_FRAGMENT
			type Post { title: String body: String! }
			type Query { post: Post }
		`)
		s.Type("Post").(*schema.ObjectType).Field("body").Resolve =
			func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
				return nil, errors.New("cannot read the body")
			}
		got := runIncrementally(context.Background(), t, s,
			`{ post { title ... @defer { body } } }`,
			execution.Request{RootValue: incrementalRoot()})

		if len(got) != 2 {
			t.Fatalf("%d payloads, want 2:\n%s", len(got), strings.Join(got, "\n"))
		}
		if strings.Contains(got[1], `"incremental"`) {
			t.Errorf("a piece that could not be delivered sent data anyway: %s", got[1])
		}
		if !strings.Contains(got[1], "cannot read the body") {
			t.Errorf("the completion does not say why: %s", got[1])
		}
		if !strings.Contains(got[1], `"hasNext":false`) {
			t.Errorf("the response did not end: %s", got[1])
		}
	})

	// A first payload that cannot stand at all has nothing to defer into, so
	// nothing is announced.
	t.Run("the first payload fails outright", func(t *testing.T) {
		s := buildSchema(t, `
			directive @defer(if: Boolean = true, label: String) on FRAGMENT_SPREAD | INLINE_FRAGMENT
			type Post { title: String body: String }
			type Query { post: Post! }
		`)
		s.QueryType().Field("post").Resolve =
			func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
				return nil, errors.New("no post")
			}
		doc, err := language.ParseString(`{ post { title ... @defer { body } } }`)
		if err != nil {
			t.Fatal(err)
		}
		result := execution.ExecuteIncrementally(context.Background(),
			execution.Request{Schema: s, Document: doc})

		if result.Subsequent != nil {
			t.Error("work was announced for a response that could not stand")
		}
		if result.Initial.HasNext {
			t.Error("the first payload promises more")
		}
		if len(result.Initial.Errors) != 1 {
			t.Fatalf("%d errors, want 1", len(result.Initial.Errors))
		}
	})

	t.Run("nothing to run", func(t *testing.T) {
		got := execution.ExecuteIncrementally(context.Background(), execution.Request{})
		if len(got.Initial.Errors) != 1 {
			t.Errorf("%d errors, want 1", len(got.Initial.Errors))
		}
		if got.Subsequent != nil {
			t.Error("a request with no schema produced a stream")
		}
	})
}

// A caller who has given up should not be made to wait for the rest, and the
// channel closes rather than leaving them reading for ever.
func TestExecuteIncrementally_Cancellation(t *testing.T) {
	s := buildSchema(t, incrementalSDL)
	root := map[string]any{"names": make([]any, 100)}
	for i := range root["names"].([]any) {
		root["names"].([]any)[i] = "name"
	}

	ctx, cancel := context.WithCancel(context.Background())
	doc, err := language.ParseString(`{ names @stream(initialCount: 1) }`)
	if err != nil {
		t.Fatal(err)
	}
	result := execution.ExecuteIncrementally(ctx,
		execution.Request{Schema: s, Document: doc, RootValue: root})
	if result.Subsequent == nil {
		t.Fatal("nothing was deferred")
	}

	// Take a couple, then give up.
	for range 2 {
		select {
		case <-result.Subsequent:
		case <-time.After(5 * time.Second):
			t.Fatal("no payload arrived")
		}
	}
	cancel()

	timeout := time.After(5 * time.Second)
	for {
		select {
		case _, open := <-result.Subsequent:
			if !open {
				return
			}
		case <-timeout:
			t.Fatal("the stream did not close after the context was cancelled")
		}
	}
}

// A mutation's root fields run in order, and deferring part of one must not
// change that.
func TestExecuteIncrementally_Mutation(t *testing.T) {
	s := buildSchema(t, `
		directive @defer(if: Boolean = true, label: String) on FRAGMENT_SPREAD | INLINE_FRAGMENT
		type Post { title: String body: String }
		type Query { post: Post }
		type Mutation { write(title: String): Post }
	`)
	var order []string
	s.MutationType().Field("write").Resolve =
		func(_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo) (any, error) {
			title, _ := args.Get("title")
			order = append(order, title.(string))
			return map[string]any{"title": title, "body": "body of " + title.(string)}, nil
		}

	got := runIncrementally(context.Background(), t, s, `
		mutation {
			a: write(title: "one") { title ... @defer { body } }
			b: write(title: "two") { title ... @defer { body } }
		}
	`, execution.Request{Concurrency: 8})

	if strings.Join(order, ",") != "one,two" {
		t.Errorf("the fields ran in order %v, want [one two]", order)
	}
	if !strings.Contains(got[0], `"a":{"title":"one"}`) || !strings.Contains(got[0], `"b":{"title":"two"}`) {
		t.Errorf("the first payload = %s", got[0])
	}
	joined := strings.Join(got, "\n")
	for _, wanted := range []string{`"body":"body of one"`, `"body":"body of two"`} {
		if !strings.Contains(joined, wanted) {
			t.Errorf("the exchange does not contain %s:\n%s", wanted, joined)
		}
	}
}

// The check that refuses incremental delivery has to look inside fragments,
// and must not walk for ever on a document that spreads one in a cycle.
func TestExecute_RefusesIncrementalInsideFragments(t *testing.T) {
	s := buildSchema(t, incrementalSDL)

	t.Run("inside a fragment", func(t *testing.T) {
		doc, err := language.ParseString(`
			{ post { ...Outer } }
			fragment Outer on Post { title ...Inner }
			fragment Inner on Post { ... @defer { body } }
		`)
		if err != nil {
			t.Fatal(err)
		}
		result := execution.Execute(context.Background(),
			execution.Request{Schema: s, Document: doc, RootValue: incrementalRoot()})
		if len(result.Errors) != 1 {
			t.Fatalf("%d errors, want 1: %v", len(result.Errors), result.Errors)
		}
	})

	// A cycle between fragments is a separate complaint; the check must
	// terminate either way.
	t.Run("a fragment cycle terminates", func(t *testing.T) {
		doc, err := language.ParseString(`
			{ post { ...A } }
			fragment A on Post { title ...B }
			fragment B on Post { body ...A }
		`)
		if err != nil {
			t.Fatal(err)
		}
		result := execution.Execute(context.Background(),
			execution.Request{Schema: s, Document: doc, RootValue: incrementalRoot()})
		if len(result.Errors) != 0 {
			t.Errorf("errors = %v", result.Errors)
		}
	})

	// A @stream is refused the same way a @defer is.
	t.Run("a stream", func(t *testing.T) {
		doc, err := language.ParseString(`{ names @stream(initialCount: 1) }`)
		if err != nil {
			t.Fatal(err)
		}
		result := execution.Execute(context.Background(), execution.Request{
			Schema: s, Document: doc, RootValue: map[string]any{"names": []any{"a", "b"}}})
		if len(result.Errors) != 1 {
			t.Fatalf("%d errors, want 1", len(result.Errors))
		}
		if !strings.Contains(result.Errors[0].Message, "@stream") {
			t.Errorf("message = %q", result.Errors[0].Message)
		}
	})
}

// A piece of work found inside a deferred fragment belongs to that fragment.
// Running it against the outermost payload's view would defer its fields all
// over again: the streamed entries came back empty and their fields arrived
// afterwards in pieces of their own.
func TestExecuteIncrementally_StreamInsideADeferredFragment(t *testing.T) {
	s := buildSchema(t, incrementalSDL)
	root := map[string]any{"posts": []any{
		map[string]any{"title": "one"},
		map[string]any{"title": "two"},
		map[string]any{"title": "three"},
	}}

	got := runIncrementally(context.Background(), t, s, `
		{
			... @defer(label: "later") {
				posts @stream(initialCount: 1) { title }
			}
		}
	`, execution.Request{RootValue: root})

	joined := strings.Join(got, "\n")
	if strings.Contains(joined, `"items":[{}]`) {
		t.Errorf("a streamed entry was delivered empty:\n%s", joined)
	}
	for _, wanted := range []string{
		`"data":{"posts":[{"title":"one"}]}`,
		`"items":[{"title":"two"},{"title":"three"}]`,
	} {
		if !strings.Contains(joined, wanted) {
			t.Errorf("the exchange does not contain %s:\n%s", wanted, joined)
		}
	}

	// And it still puts back together into the same response.
	assembled := assemble(context.Background(), t, s, `
		{ ... @defer { posts @stream(initialCount: 1) { title } } }
	`, root)
	plain := run(t, s, `{ posts { title } }`, execution.Request{RootValue: root})
	data, _ := plain.Data.Get()
	var want any
	if err := json.Unmarshal([]byte(marshal(t, data)), &want); err != nil {
		t.Fatalf("reading the equivalent response: %v", err)
	}
	if !reflect.DeepEqual(assembled, want) {
		t.Errorf("reassembled =\n  %s\nwant\n  %s", marshal(t, assembled), marshal(t, want))
	}
}

// A @defer written once in a document stands for one announced piece per place
// it is reached.
//
// What a type asks for beneath a group of selections is worked out once and
// kept, so every entry of a list is handed the same deferral. Telling the
// entries apart is the executor's job, not the collector's: each is announced
// at its own path, and each is delivered and completed on its own.
func TestExecuteIncrementally_DeferInsideAList(t *testing.T) {
	s := buildSchema(t, incrementalSDL)
	root := map[string]any{"posts": []any{
		map[string]any{"title": "one", "body": "first"},
		map[string]any{"title": "two", "body": "second"},
		map[string]any{"title": "three", "body": "third"},
	}}

	got := runIncrementally(context.Background(), t, s, `
		{ posts { title ... @defer(label: "rest") { body } } }
	`, execution.Request{RootValue: root})

	assertPayloads(t, got, []string{
		`{"data":{"posts":[{"title":"one"},{"title":"two"},{"title":"three"}]},"pending":[{"id":"0","path":["posts",0],"label":"rest"},{"id":"1","path":["posts",1],"label":"rest"},{"id":"2","path":["posts",2],"label":"rest"}],"hasNext":true}`,
		`{"hasNext":false,"incremental":[{"id":"0","data":{"body":"first"}},{"id":"1","data":{"body":"second"}},{"id":"2","data":{"body":"third"}}],"completed":[{"id":"0"},{"id":"1"},{"id":"2"}]}`,
	})
}

// Sibling fields resolving alongside one another may each meet a @defer, and
// each has to be announced in its own right.
//
// The run's count of deferred fragments is shared, and what a type asks for
// beneath a group of selections is worked out once and kept, so both are
// reached from more than one goroutine at a time. What arrives must not depend
// on which got there first.
func TestExecuteIncrementally_DeferWithConcurrency(t *testing.T) {
	s := buildSchema(t, incrementalSDL)
	root := map[string]any{"post": map[string]any{
		"title": "one", "body": "first",
		"author": map[string]any{"name": "Ada", "bio": "a bio"},
	}}
	// Two sibling object fields, each holding a @defer, so that both are met
	// from goroutines running alongside each other.
	const query = `
		{
			a: post { title ... @defer(label: "a-body") { body } }
			b: post {
				title
				... @defer(label: "b-body") { body }
				author { name ... @defer(label: "b-bio") { bio } }
			}
		}
	`

	for range 20 {
		got := runIncrementally(context.Background(), t, s,
			query, execution.Request{RootValue: root, Concurrency: 8})

		joined := strings.Join(got, "\n")
		for _, wanted := range []string{
			`"body":"first"`, `"bio":"a bio"`,
			`"path":["a"]`, `"path":["b"]`, `"path":["b","author"]`,
		} {
			if !strings.Contains(joined, wanted) {
				t.Fatalf("the exchange does not contain %s:\n%s", wanted, joined)
			}
		}
		// Three fragments, three announcements, each once.
		if n := strings.Count(joined, `"label"`); n != 3 {
			t.Fatalf("%d announcements, want 3:\n%s", n, joined)
		}
		if n := strings.Count(joined, `"completed"`); n != 1 {
			t.Fatalf("the completions came in %d payloads, want 1:\n%s", n, joined)
		}
	}
}

// Working on the entries of a list alongside one another must not change what
// the client is told, down to which identifier names which piece.
//
// Each entry announces its own deferred fragment, and the entries no longer
// take their turns in order, so the order they are announced in cannot be the
// order they were reached in. It is the order the list holds them in, which is
// the order what they found is merged in.
func TestExecuteIncrementally_DeferInsideAListWithConcurrency(t *testing.T) {
	s := buildSchema(t, incrementalSDL)
	root := map[string]any{"posts": []any{
		map[string]any{"title": "one", "body": "first"},
		map[string]any{"title": "two", "body": "second"},
		map[string]any{"title": "three", "body": "third"},
		map[string]any{"title": "four", "body": "fourth"},
	}}
	const query = `{ posts { title ... @defer(label: "rest") { body } } }`

	sequential := strings.Join(runIncrementally(context.Background(), t, s,
		query, execution.Request{RootValue: root}), "\n")
	for i := range 50 {
		parallel := strings.Join(runIncrementally(context.Background(), t, s,
			query, execution.Request{RootValue: root, Concurrency: 8}), "\n")
		if parallel != sequential {
			t.Fatalf("run %d differed:\n  parallel:\n%s\n  sequential:\n%s",
				i, parallel, sequential)
		}
	}
}
