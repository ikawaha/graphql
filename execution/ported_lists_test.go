package execution_test

// Ported from graphql-js src/execution/__tests__/lists-test.ts, the part about
// what a list may hold and what happens when it holds the wrong thing. The
// rest of that file is about JavaScript iterators, generators and promises,
// and has no counterpart here.
//
// graphql-js writes a failed entry as a JavaScript Error in the list; here it
// is a *gqlerror.Error, which is the one value a server would not be returning
// as data.

import (
	"testing"

	"github.com/ikawaha/graphql/gqlerror"
)

func TestPortedLists(t *testing.T) {
	const query = `{ listField }`
	nulled := `{"data": {"listField": null}}`
	cannotBeNull := func(path string) string {
		return `{"message": "Cannot return null for non-nullable field Query.listField.",` +
			`"locations": [{"line": 1, "column": 3}], "path": ` + path + `}`
	}
	bad := func(path string) string {
		return `{"message": "bad", "locations": [{"line": 1, "column": 3}], "path": ` + path + `}`
	}

	runPorted(t, nil, nil, nil, []portedCase{
		// Contains values.
		{
			name: `contains values, as [Int]`, sdl: `type Query { listField: [Int] }`, query: query,
			root: map[string]any{"listField": []any{1, 2}},
			want: `{"data": {"listField": [1, 2]}}`,
		},
		{
			name: `contains values, as [Int]!`, sdl: `type Query { listField: [Int]! }`, query: query,
			root: map[string]any{"listField": []any{1, 2}},
			want: `{"data": {"listField": [1, 2]}}`,
		},
		{
			name: `contains values, as [Int!]`, sdl: `type Query { listField: [Int!] }`, query: query,
			root: map[string]any{"listField": []any{1, 2}},
			want: `{"data": {"listField": [1, 2]}}`,
		},
		{
			name: `contains values, as [Int!]!`, sdl: `type Query { listField: [Int!]! }`, query: query,
			root: map[string]any{"listField": []any{1, 2}},
			want: `{"data": {"listField": [1, 2]}}`,
		},

		// Contains null.
		{
			name: `contains null, as [Int]`, sdl: `type Query { listField: [Int] }`, query: query,
			root: map[string]any{"listField": []any{1, nil, 2}},
			want: `{"data": {"listField": [1, null, 2]}}`,
		},
		{
			name: `contains null, as [Int]!`, sdl: `type Query { listField: [Int]! }`, query: query,
			root: map[string]any{"listField": []any{1, nil, 2}},
			want: `{"data": {"listField": [1, null, 2]}}`,
		},
		{
			name: `contains null, as [Int!]`, sdl: `type Query { listField: [Int!] }`, query: query,
			root: map[string]any{"listField": []any{1, nil, 2}},
			want: `{"data": {"listField": null}, "errors": [` + cannotBeNull(`["listField", 1]`) + `]}`,
		},
		{
			name: `contains null, as [Int!]!`, sdl: `type Query { listField: [Int!]! }`, query: query,
			root: map[string]any{"listField": []any{1, nil, 2}},
			want: `{"data": null, "errors": [` + cannotBeNull(`["listField", 1]`) + `]}`,
		},

		// Returns null.
		{
			name: `returns null, as [Int]`, sdl: `type Query { listField: [Int] }`, query: query,
			root: map[string]any{"listField": nil}, want: nulled,
		},
		{
			name: `returns null, as [Int]!`, sdl: `type Query { listField: [Int]! }`, query: query,
			root: map[string]any{"listField": nil},
			want: `{"data": null, "errors": [` + cannotBeNull(`["listField"]`) + `]}`,
		},
		{
			name: `returns null, as [Int!]`, sdl: `type Query { listField: [Int!] }`, query: query,
			root: map[string]any{"listField": nil}, want: nulled,
		},
		{
			name: `returns null, as [Int!]!`, sdl: `type Query { listField: [Int!]! }`, query: query,
			root: map[string]any{"listField": nil},
			want: `{"data": null, "errors": [` + cannotBeNull(`["listField"]`) + `]}`,
		},

		// Contains an entry that failed.
		{
			name: `contains error, as [Int]`, sdl: `type Query { listField: [Int] }`, query: query,
			root: map[string]any{"listField": []any{1, gqlerror.New("bad"), 2}},
			want: `{"data": {"listField": [1, null, 2]}, "errors": [` + bad(`["listField", 1]`) + `]}`,
		},
		{
			name: `contains error, as [Int]!`, sdl: `type Query { listField: [Int]! }`, query: query,
			root: map[string]any{"listField": []any{1, gqlerror.New("bad"), 2}},
			want: `{"data": {"listField": [1, null, 2]}, "errors": [` + bad(`["listField", 1]`) + `]}`,
		},
		{
			name: `contains error, as [Int!]`, sdl: `type Query { listField: [Int!] }`, query: query,
			root: map[string]any{"listField": []any{1, gqlerror.New("bad"), 2}},
			want: `{"data": {"listField": null}, "errors": [` + bad(`["listField", 1]`) + `]}`,
		},
		{
			name: `contains error, as [Int!]!`, sdl: `type Query { listField: [Int!]! }`, query: query,
			root: map[string]any{"listField": []any{1, gqlerror.New("bad"), 2}},
			want: `{"data": null, "errors": [` + bad(`["listField", 1]`) + `]}`,
		},

		// Something that is not a list at all.
		{
			name: `a lone string is not a list`, sdl: `type Query { listField: [String] }`, query: query,
			root: map[string]any{"listField": "Singular"},
			want: `{"data": {"listField": null}, "errors": [{` +
				`"message": "Expected Iterable, but did not find one for field \"Query.listField\".",` +
				`"locations": [{"line": 1, "column": 3}], "path": ["listField"]}]}`,
		},
	})
}
