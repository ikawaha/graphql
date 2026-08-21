package execution_test

// The defer cases graphql-js writes with a root value holding a function.
//
// Two shapes: a function that throws, which is a resolver that fails here, and
// one that answers with a constant, which is just that constant in the root
// value. Both are written out by hand, since the shape does not survive being
// read off the TypeScript.

import "testing"

func TestPortedDefer_Errors(t *testing.T) {
	// The root value graphql-js spreads `...hero` into, with one field the
	// case overrides. Only the fields each case reads are given.
	const heroWithNoFriends = `{"hero": {"id": 1, "name": "Luke", "friends": []}}`
	const heroWithNoNestedObject = `{"hero": {"id": 1, "name": "Luke", "nestedObject": null}}`
	const heroWithNoNonNullName = `{"hero": {"id": 1, "name": "Luke", "nonNullName": null}}`

	runPortedIncremental(t, []portedIncrementalCase{
		{
			name:    "Can defer fragments with errors on the top level Query field",
			query:   "\n      query HeroNameQuery {\n        ...QueryFragment @defer(label: \"DeferQuery\")\n      }\n      fragment QueryFragment on Query {\n        hero {\n          name\n        }\n      }\n    ",
			failing: []string{"Hero.name"},
			want:    "[{\"data\": {}, \"pending\": [{\"id\": \"0\", \"path\": [], \"label\": \"DeferQuery\"}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"hero\": {\"name\": null}}, \"errors\": [{\"message\": \"bad\", \"locations\": [{\"line\": 7, \"column\": 11}], \"path\": [\"hero\", \"name\"]}], \"id\": \"0\"}], \"completed\": [{\"id\": \"0\"}], \"hasNext\": false}]",
		},
		{
			name:    "Handles errors thrown in deferred fragments",
			query:   "\n      query HeroNameQuery {\n        hero {\n          id\n          ...NameFragment @defer\n        }\n      }\n      fragment NameFragment on Hero {\n        name\n      }\n    ",
			failing: []string{"Hero.name"},
			want:    "[{\"data\": {\"hero\": {\"id\": \"1\"}}, \"pending\": [{\"id\": \"0\", \"path\": [\"hero\"]}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"name\": null}, \"id\": \"0\", \"errors\": [{\"message\": \"bad\", \"locations\": [{\"line\": 9, \"column\": 9}], \"path\": [\"hero\", \"name\"]}]}], \"completed\": [{\"id\": \"0\"}], \"hasNext\": false}]",
		},
		{
			name:  "Handles non-nullable errors thrown in deferred fragments",
			query: "\n      query HeroNameQuery {\n        hero {\n          id\n          ...NameFragment @defer\n        }\n      }\n      fragment NameFragment on Hero {\n        nonNullName\n      }\n    ",
			root:  heroWithNoNonNullName,
			want:  "[{\"data\": {\"hero\": {\"id\": \"1\"}}, \"pending\": [{\"id\": \"0\", \"path\": [\"hero\"]}], \"hasNext\": true}, {\"completed\": [{\"id\": \"0\", \"errors\": [{\"message\": \"Cannot return null for non-nullable field Hero.nonNullName.\", \"locations\": [{\"line\": 9, \"column\": 9}], \"path\": [\"hero\", \"nonNullName\"]}]}], \"hasNext\": false}]",
		},
		{
			name:  "Handles non-nullable errors thrown outside deferred fragments",
			query: "\n      query HeroNameQuery {\n        hero {\n          nonNullName\n          ...NameFragment @defer\n        }\n      }\n      fragment NameFragment on Hero {\n        id\n      }\n    ",
			root:  heroWithNoNonNullName,
			want:  "{\"errors\": [{\"message\": \"Cannot return null for non-nullable field Hero.nonNullName.\", \"locations\": [{\"line\": 4, \"column\": 11}], \"path\": [\"hero\", \"nonNullName\"]}], \"data\": {\"hero\": null}}",
		},
		{
			name:  "Deduplicates list fields that return empty lists",
			query: "\n      query {\n        hero {\n          friends {\n            name\n          }\n          ... @defer {\n            friends {\n              name\n            }\n          }\n        }\n      }\n    ",
			root:  heroWithNoFriends,
			want:  "{\"data\": {\"hero\": {\"friends\": []}}}",
		},
		{
			name:  "Deduplicates null object fields",
			query: "\n      query {\n        hero {\n          nestedObject {\n            name\n          }\n          ... @defer {\n            nestedObject {\n              name\n            }\n          }\n        }\n      }\n    ",
			root:  heroWithNoNestedObject,
			want:  "{\"data\": {\"hero\": {\"nestedObject\": null}}}",
		},
		{
			name:  "Cancels deferred fields when deferred result exhibits null bubbling",
			query: "\n      query {\n        ... @defer {\n          hero {\n            nonNullName\n            name\n          }\n        }\n      }\n    ",
			root:  heroWithNoNonNullName,
			want:  "[{\"data\": {}, \"pending\": [{\"id\": \"0\", \"path\": []}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"hero\": null}, \"errors\": [{\"message\": \"Cannot return null for non-nullable field Hero.nonNullName.\", \"locations\": [{\"line\": 5, \"column\": 13}], \"path\": [\"hero\", \"nonNullName\"]}], \"id\": \"0\"}], \"completed\": [{\"id\": \"0\"}], \"hasNext\": false}]",
		},
	})
}
