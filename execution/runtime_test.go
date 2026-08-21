package execution_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// A panic in a resolver is a fault in code the server author wrote. Left
// alone it would take down every request the process is serving, not just the
// field it belongs to, so it is reported as that field failing.
func TestExecute_ResolverPanic(t *testing.T) {
	s := buildSchema(t, `type Query { ok: String bad: String }`)
	s.QueryType().Field("bad").Resolve =
		func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
			panic("something went badly wrong")
		}

	result := run(t, s, `{ ok bad }`, execution.Request{RootValue: map[string]any{"ok": "fine"}})

	// The field that panicked is null and the others still answered.
	if got := jsonOf(t, result); !strings.Contains(got, `"ok":"fine"`) || !strings.Contains(got, `"bad":null`) {
		t.Errorf("response = %s", got)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("%d errors, want 1", len(result.Errors))
	}
	if !strings.Contains(result.Errors[0].Message, "something went badly wrong") {
		t.Errorf("message = %q, want it to carry what was panicked with", result.Errors[0].Message)
	}

	// The cause is still reachable, so a server can log the stack rather than
	// only the message.
	var panicked *execution.PanicError
	if !errors.As(result.Errors[0], &panicked) {
		t.Fatal("the panic is not reachable through the error")
	}
	if len(panicked.Stack) == 0 {
		t.Error("the stack was not kept")
	}

	// A panic with an error value keeps that error reachable too.
	t.Run("panicking with an error", func(t *testing.T) {
		sentinel := errors.New("sentinel")
		s := buildSchema(t, `type Query { bad: String }`)
		s.QueryType().Field("bad").Resolve =
			func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
				panic(sentinel)
			}
		result := run(t, s, `{ bad }`, execution.Request{})
		if len(result.Errors) != 1 {
			t.Fatalf("%d errors, want 1", len(result.Errors))
		}
		if !errors.Is(result.Errors[0], sentinel) {
			t.Error("the panicked error is not reachable with errors.Is")
		}
	})
}

// A mutation's root fields are expected to happen in order, each seeing what
// the one before it did. This is the one place the specification requires
// sequence rather than allowing it.
func TestExecute_MutationsRunInOrder(t *testing.T) {
	s := buildSchema(t, `
		type Query { value: Int }
		type Mutation { push(n: Int!): Int }
	`)
	var mu sync.Mutex
	var order []int32
	s.MutationType().Field("push").Resolve =
		func(_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo) (any, error) {
			// GraphQL's Int is a signed 32-bit integer, so that is what a
			// resolver is handed.
			n, _ := args.Get("n")
			count := n.(int32)
			// A resolver that takes a moment would overtake the next one if
			// they ran together.
			time.Sleep(time.Duration(5-count) * time.Millisecond)
			mu.Lock()
			defer mu.Unlock()
			order = append(order, count)
			return count, nil
		}

	// Concurrency is asked for, and the mutation ignores it.
	expectJSON(t, s, `mutation { a: push(n: 1) b: push(n: 2) c: push(n: 3) }`,
		execution.Request{Concurrency: 8},
		`{"data":{"a":1,"b":2,"c":3}}`)

	if fmt.Sprint(order) != "[1 2 3]" {
		t.Errorf("the fields ran in order %v, want [1 2 3]", order)
	}
}

// Running fields alongside one another must not change the response: the same
// keys in the same order, and the same errors in the same order.
func TestExecute_ConcurrencyDoesNotChangeTheAnswer(t *testing.T) {
	s := buildSchema(t, `
		type Query { a: String b: String c: String d: String bad: String }
	`)
	s.QueryType().Field("bad").Resolve =
		func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
			return nil, errors.New("boom")
		}
	root := map[string]any{"a": "A", "b": "B", "c": "C", "d": "D"}
	const query = `{ d bad c b a }`

	sequential := jsonOf(t, run(t, s, query, execution.Request{RootValue: root}))
	for i := range 20 {
		parallel := jsonOf(t, run(t, s, query, execution.Request{RootValue: root, Concurrency: 8}))
		if parallel != sequential {
			t.Fatalf("run %d differed:\n  parallel:   %s\n  sequential: %s", i, parallel, sequential)
		}
	}
}

// Asking for concurrency has to actually run resolvers alongside one another,
// or the setting is a lie.
func TestExecute_ConcurrencyIsReal(t *testing.T) {
	s := buildSchema(t, `type Query { a: String b: String c: String d: String }`)
	var running, peak atomic.Int64
	hold := func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		now := running.Add(1)
		for {
			was := peak.Load()
			if now <= was || peak.CompareAndSwap(was, now) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		running.Add(-1)
		return "x", nil
	}
	for _, name := range []string{"a", "b", "c", "d"} {
		s.QueryType().Field(name).Resolve = hold
	}

	run(t, s, `{ a b c d }`, execution.Request{Concurrency: 4})
	if peak.Load() < 2 {
		t.Errorf("at most %d resolvers ran at once; concurrency had no effect", peak.Load())
	}

	// And the bound is respected, so a wide selection set cannot spawn a
	// goroutine per field.
	peak.Store(0)
	run(t, s, `{ a b c d }`, execution.Request{Concurrency: 2})
	if got := peak.Load(); got > 2 {
		t.Errorf("%d resolvers ran at once, want at most 2", got)
	}
}

// Whatever a resolver needs to know about the caller travels in the context,
// which is where a Go program already looks for it.
func TestExecute_ContextReachesResolvers(t *testing.T) {
	type key struct{}
	s := buildSchema(t, `type Query { who: String }`)
	s.QueryType().Field("who").Resolve =
		func(ctx context.Context, _ any, _ schema.Arguments, _ *schema.ResolveInfo) (any, error) {
			name, _ := ctx.Value(key{}).(string)
			return name, nil
		}

	doc := mustParse(t, `{ who }`)
	ctx := context.WithValue(context.Background(), key{}, "Ada")
	result := execution.Execute(ctx, execution.Request{Schema: s, Document: doc})
	if got := jsonOf(t, result); got != `{"data":{"who":"Ada"}}` {
		t.Errorf("response = %s", got)
	}
}

// A resolver is told where in the request it is, which is what lets one make
// decisions about the field it is answering.
func TestExecute_ResolveInfo(t *testing.T) {
	s := buildSchema(t, `type Query { me: User } type User { name: String }`)
	var seen *schema.ResolveInfo
	s.Type("User").(*schema.ObjectType).Field("name").Resolve =
		func(_ context.Context, _ any, _ schema.Arguments, info *schema.ResolveInfo) (any, error) {
			seen = info
			return "Ada", nil
		}

	run(t, s, `{ me { name } }`, execution.Request{RootValue: map[string]any{"me": map[string]any{}}})

	if seen == nil {
		t.Fatal("the resolver was not called")
	}
	if seen.FieldName != "name" {
		t.Errorf("FieldName = %q", seen.FieldName)
	}
	if seen.ParentType == nil || seen.ParentType.Name() != "User" {
		t.Errorf("ParentType = %v, want User", seen.ParentType)
	}
	if got, want := seen.Path.String(), ".me.name"; !strings.HasSuffix(got, "me.name") {
		t.Errorf("Path = %q, want it to end with %q", got, want)
	}
	if seen.Schema != s {
		t.Error("the resolver was not told which schema it is answering against")
	}
}

func TestExecute_OperationSelection(t *testing.T) {
	s := buildSchema(t, `type Query { a: String }`)
	root := map[string]any{"a": "A"}

	t.Run("one operation needs no name", func(t *testing.T) {
		expectJSON(t, s, `{ a }`, execution.Request{RootValue: root}, `{"data":{"a":"A"}}`)
	})

	t.Run("choosing by name", func(t *testing.T) {
		expectJSON(t, s, `query One { a } query Two { b: a }`,
			execution.Request{RootValue: root, OperationName: "Two"},
			`{"data":{"b":"A"}}`)
	})

	t.Run("several operations and no name", func(t *testing.T) {
		result := run(t, s, `query One { a } query Two { a }`, execution.Request{RootValue: root})
		if len(result.Errors) != 1 {
			t.Fatalf("%d errors, want 1", len(result.Errors))
		}
		if got := jsonOf(t, result); strings.Contains(got, `"data"`) {
			t.Errorf("response = %s, want no data: the request never ran", got)
		}
	})

	t.Run("a name that is not there", func(t *testing.T) {
		result := run(t, s, `query One { a }`, execution.Request{RootValue: root, OperationName: "Other"})
		if len(result.Errors) != 1 {
			t.Fatalf("%d errors, want 1", len(result.Errors))
		}
	})

	t.Run("nothing to run", func(t *testing.T) {
		result := runUnvalidated(t, s, `fragment F on Query { a }`, execution.Request{})
		if len(result.Errors) != 1 {
			t.Fatalf("%d errors, want 1", len(result.Errors))
		}
	})
}

// A caller who has given up — a client that hung up, a deadline that passed —
// should not be made to wait for the rest of the request. What was already
// resolved is still returned, alongside the reason the rest is missing.
func TestExecute_ContextCancellation(t *testing.T) {
	s := buildSchema(t, `type Query { a: String b: String c: String }`)
	ctx, cancel := context.WithCancel(context.Background())
	var resolved atomic.Int64
	s.QueryType().Field("a").Resolve =
		func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
			resolved.Add(1)
			// The caller gives up while the first field is being answered.
			cancel()
			return "A", nil
		}
	count := func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		resolved.Add(1)
		return "x", nil
	}
	s.QueryType().Field("b").Resolve = count
	s.QueryType().Field("c").Resolve = count

	doc := mustParse(t, `{ a b c }`)
	result := execution.Execute(ctx, execution.Request{Schema: s, Document: doc})

	// The field that had already run is in the response.
	if got := jsonOf(t, result); !strings.Contains(got, `"a":"A"`) {
		t.Errorf("response = %s, want the work already done to be kept", got)
	}
	if got := resolved.Load(); got != 1 {
		t.Errorf("%d resolvers ran, want 1: the rest should not have been started", got)
	}
	if len(result.Errors) != 2 {
		t.Fatalf("%d errors, want one per field not answered", len(result.Errors))
	}
	for _, err := range result.Errors {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("error %q does not carry why the request stopped", err.Message)
		}
	}
}

// A deadline that has already passed stops the request before anything runs.
func TestExecute_ExpiredDeadline(t *testing.T) {
	s := buildSchema(t, `type Query { a: String! }`)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	doc := mustParse(t, `{ a }`)
	result := execution.Execute(ctx, execution.Request{Schema: s, Document: doc,
		RootValue: map[string]any{"a": "A"}})

	// The field may not be null, so there is nowhere to put one and the whole
	// response is null.
	if got := jsonOf(t, result); !strings.Contains(got, `"data":null`) {
		t.Errorf("response = %s", got)
	}
	if len(result.Errors) != 1 || !errors.Is(result.Errors[0], context.DeadlineExceeded) {
		t.Fatalf("errors = %v, want one deadline error", result.Errors)
	}
}

// The answer to "what does this type ask for beneath these selections?" is
// worked out once and kept, and sibling fields may be asking at the same time.
// The same response has to come back however the work was scheduled.
func TestExecute_ConcurrencyWithNestedObjects(t *testing.T) {
	s := buildSchema(t, `
		type Query { left: [Node] right: [Node] }
		type Node { name: String child: Node children: [Node] }
	`)
	node := func(name string) map[string]any {
		return map[string]any{
			"name":  name,
			"child": map[string]any{"name": name + "-child"},
			"children": []any{
				map[string]any{"name": name + "-0"},
				map[string]any{"name": name + "-1"},
			},
		}
	}
	list := make([]any, 12)
	for i := range list {
		list[i] = node(string(rune('a' + i)))
	}
	root := map[string]any{"left": list, "right": list}
	const query = `
		{
			left { name child { name } children { name } }
			right { name children { name child { name } } }
		}
	`

	sequential := jsonOf(t, run(t, s, query, execution.Request{RootValue: root}))
	for i := range 20 {
		parallel := jsonOf(t, run(t, s, query, execution.Request{RootValue: root, Concurrency: 8}))
		if parallel != sequential {
			t.Fatalf("run %d differed:\n  parallel:   %s\n  sequential: %s", i, parallel, sequential)
		}
	}
}

// The same fragment reached under two different sets of arguments asks two
// different things, even though the field it names is the same one. What a
// type asks for beneath a group of selections is remembered, and the two must
// not be taken for one.
func TestExecute_FragmentArgumentsAreNotConflated(t *testing.T) {
	s := buildSchema(t, `
		type Query { items: [Item] }
		type Item { inner: Inner }
		type Inner { echo(input: String): String }
	`)
	s.Type("Inner").(*schema.ObjectType).Field("echo").Resolve =
		func(_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo) (any, error) {
			in, _ := args.Get("input")
			return in, nil
		}
	// Two entries, so that the second one asks a question the first has
	// already had answered.
	item := map[string]any{"inner": map[string]any{}}
	root := map[string]any{"items": []any{item, item}}

	doc, err := language.ParseString(`
		{
			one: items { ...say(word: "one") }
			two: items { ...say(word: "two") }
		}
		fragment say($word: String) on Item {
			inner { echo(input: $word) }
		}
	`, language.ExperimentalFragmentArguments())
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	got := jsonOf(t, execution.Execute(context.Background(),
		execution.Request{Schema: s, Document: doc, RootValue: root}))
	const want = `{"data":{"one":[{"inner":{"echo":"one"}},{"inner":{"echo":"one"}}],` +
		`"two":[{"inner":{"echo":"two"}},{"inner":{"echo":"two"}}]}}`
	if got != want {
		t.Errorf("run =\n%s\nwant\n%s", got, want)
	}
}

// The entries of a list are resolved alongside one another too, not just the
// fields of an object. A list of things each fetched from somewhere is the
// shape that gains most, and it is what a request usually looks like.
func TestExecute_ConcurrencyReachesListEntries(t *testing.T) {
	s := buildSchema(t, `
		type Query { people: [Person] }
		type Person { name: String }
	`)
	var running, peak atomic.Int64
	s.Type("Person").(*schema.ObjectType).Field("name").Resolve =
		func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
			now := running.Add(1)
			for {
				was := peak.Load()
				if now <= was || peak.CompareAndSwap(was, now) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			running.Add(-1)
			return "x", nil
		}
	root := map[string]any{"people": []any{
		map[string]any{}, map[string]any{}, map[string]any{}, map[string]any{},
	}}

	run(t, s, `{ people { name } }`, execution.Request{RootValue: root, Concurrency: 4})
	if peak.Load() < 2 {
		t.Errorf("at most %d entries were completed at once; concurrency did not reach them", peak.Load())
	}

	// And the bound holds for a list as it does for a selection set.
	peak.Store(0)
	run(t, s, `{ people { name } }`, execution.Request{RootValue: root, Concurrency: 2})
	if got := peak.Load(); got > 2 {
		t.Errorf("%d entries were completed at once, want at most 2", got)
	}
}

// Completing the entries of a list alongside one another must not change the
// response, down to which errors are reported and in what order. An entry that
// may not be null brings the list down, and nothing after it is reported —
// which is exactly what happens when the entries are completed one at a time.
func TestExecute_ConcurrencyDoesNotChangeAListAnswer(t *testing.T) {
	s := buildSchema(t, `
		type Query { nullable: [Item] nonNull: [Item!] }
		type Item { must: String! may: String }
	`)
	fail := func(field string) schema.FieldResolver {
		return func(_ context.Context, src any, _ schema.Arguments, _ *schema.ResolveInfo) (any, error) {
			if src.(map[string]any)[field] == "bad" {
				return nil, errors.New("cannot read " + field)
			}
			return "ok", nil
		}
	}
	item := s.Type("Item").(*schema.ObjectType)
	item.Field("must").Resolve = fail("must")
	item.Field("may").Resolve = fail("may")
	// The second entry brings a non-null list down, and the third would report
	// an error of its own if it were reported at all.
	items := []any{
		map[string]any{},
		map[string]any{"must": "bad"},
		map[string]any{"may": "bad"},
		map[string]any{},
	}
	root := map[string]any{"nullable": items, "nonNull": items}

	for _, query := range []string{
		`{ nullable { must may } }`,
		`{ nonNull { must may } }`,
		`{ nullable { must may } nonNull { must may } }`,
	} {
		t.Run(query, func(t *testing.T) {
			sequential := jsonOf(t, run(t, s, query, execution.Request{RootValue: root}))
			for i := range 20 {
				parallel := jsonOf(t, run(t, s, query,
					execution.Request{RootValue: root, Concurrency: 8}))
				if parallel != sequential {
					t.Fatalf("run %d differed:\n  parallel:   %s\n  sequential: %s",
						i, parallel, sequential)
				}
			}
		})
	}
}

// A field's arguments are worked out once for the object type and handed to
// every call, so what is wrong with them is worked out once too. It still has
// to be reported once per object, at that object's own path.
func TestExecute_ArgumentErrorsAreReportedPerEntry(t *testing.T) {
	s := buildSchema(t, `
		type Query { items: [Item] }
		type Item { needs(arg: String!): String }
	`)
	// The document does not supply the argument, which validation would
	// refuse; execution is asked to run it anyway, as it would be for a
	// persisted query that was checked against an older schema.
	doc, err := language.ParseString(`{ items { needs } }`)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	root := map[string]any{"items": []any{
		map[string]any{}, map[string]any{}, map[string]any{},
	}}

	result := execution.Execute(context.Background(),
		execution.Request{Schema: s, Document: doc, RootValue: root})

	if len(result.Errors) != 3 {
		t.Fatalf("%d errors, want one per entry:\n%s", len(result.Errors), jsonOf(t, result))
	}
	for i, err := range result.Errors {
		want := fmt.Sprintf(`[items %d needs]`, i)
		if got := fmt.Sprint(err.Path); got != want {
			t.Errorf("error %d is at %s, want %s", i, got, want)
		}
		if !strings.Contains(err.Message, `Argument "Item.needs(arg:)" of required type "String!" was not provided.`) {
			t.Errorf("error %d = %s", i, err.Message)
		}
	}
}
