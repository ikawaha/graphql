package execution_test

// Ported from graphql-js src/execution/incremental/__tests__/defer-test.ts.
//
// What is compared is the whole run: the first response and every payload that
// followed, in order. The cases that are left out need promises — a resolver
// that has not answered yet, an abort part way through — which a Go resolver
// cannot be in.

import "testing"

func TestPortedDefer(t *testing.T) {
	runPortedIncremental(t, portedDeferCases)
}

var portedDeferCases = []portedIncrementalCase{
	{
		name:  "Can defer fragments containing scalar types",
		query: "\n      query HeroNameQuery {\n        hero {\n          id\n          ...NameFragment @defer\n        }\n      }\n      fragment NameFragment on Hero {\n        name\n      }\n    ",
		want:  "[{\"data\": {\"hero\": {\"id\": \"1\"}}, \"pending\": [{\"id\": \"0\", \"path\": [\"hero\"]}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"name\": \"Luke\"}, \"id\": \"0\"}], \"completed\": [{\"id\": \"0\"}], \"hasNext\": false}]",
	},
	{
		name:  "Returns label from defer directive",
		query: "\n      query HeroNameQuery {\n        hero {\n          id\n          ...NameFragment @defer(label: \"defer-label\")\n        }\n      }\n      fragment NameFragment on Hero {\n        name\n      }\n    ",
		want:  "[{\"data\": {\"hero\": {\"id\": \"1\"}}, \"pending\": [{\"id\": \"0\", \"path\": [\"hero\"], \"label\": \"defer-label\"}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"name\": \"Luke\"}, \"id\": \"0\"}], \"completed\": [{\"id\": \"0\"}], \"hasNext\": false}]",
	},
	{
		name:  "Treats null defer label the same as no label",
		query: "\n      query HeroNameQuery {\n        hero {\n          id\n          ...NameFragment @defer(label: null)\n        }\n      }\n      fragment NameFragment on Hero {\n        name\n      }\n    ",
		want:  "[{\"data\": {\"hero\": {\"id\": \"1\"}}, \"pending\": [{\"id\": \"0\", \"path\": [\"hero\"]}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"name\": \"Luke\"}, \"id\": \"0\"}], \"completed\": [{\"id\": \"0\"}], \"hasNext\": false}]",
	},
	{
		name:  "Can disable defer using if argument",
		query: "\n      query HeroNameQuery {\n        hero {\n          id\n          ...NameFragment @defer(if: false)\n        }\n      }\n      fragment NameFragment on Hero {\n        name\n      }\n    ",
		want:  "{\"data\": {\"hero\": {\"id\": \"1\", \"name\": \"Luke\"}}}",
	},
	{
		name:  "Does not disable defer with null if argument",
		query: "\n      query HeroNameQuery($shouldDefer: Boolean) {\n        hero {\n          id\n          ...NameFragment @defer(if: $shouldDefer)\n        }\n      }\n      fragment NameFragment on Hero {\n        name\n      }\n    ",
		want:  "[{\"data\": {\"hero\": {\"id\": \"1\"}}, \"pending\": [{\"id\": \"0\", \"path\": [\"hero\"]}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"name\": \"Luke\"}, \"id\": \"0\"}], \"completed\": [{\"id\": \"0\"}], \"hasNext\": false}]",
	},
	{
		name:  "Can defer fragments on the top level Query field",
		query: "\n      query HeroNameQuery {\n        ...QueryFragment @defer(label: \"DeferQuery\")\n      }\n      fragment QueryFragment on Query {\n        hero {\n          id\n        }\n      }\n    ",
		want:  "[{\"data\": {}, \"pending\": [{\"id\": \"0\", \"path\": [], \"label\": \"DeferQuery\"}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"hero\": {\"id\": \"1\"}}, \"id\": \"0\"}], \"completed\": [{\"id\": \"0\"}], \"hasNext\": false}]",
	},
	{
		name:  "Can defer a fragment within an already deferred fragment",
		query: "\n      query HeroNameQuery {\n        hero {\n          ...TopFragment @defer(label: \"DeferTop\")\n        }\n      }\n      fragment TopFragment on Hero {\n        id\n        ...NestedFragment @defer(label: \"DeferNested\")\n      }\n      fragment NestedFragment on Hero {\n        friends {\n          name\n        }\n      }\n    ",
		want:  "[{\"data\": {\"hero\": {}}, \"pending\": [{\"id\": \"0\", \"path\": [\"hero\"], \"label\": \"DeferTop\"}], \"hasNext\": true}, {\"pending\": [{\"id\": \"1\", \"path\": [\"hero\"], \"label\": \"DeferNested\"}], \"incremental\": [{\"data\": {\"id\": \"1\"}, \"id\": \"0\"}, {\"data\": {\"friends\": [{\"name\": \"Han\"}, {\"name\": \"Leia\"}, {\"name\": \"C-3PO\"}]}, \"id\": \"1\"}], \"completed\": [{\"id\": \"0\"}, {\"id\": \"1\"}], \"hasNext\": false}]",
	},
	{
		name:  "Can defer a fragment that is also not deferred, deferred fragment is first",
		query: "\n      query HeroNameQuery {\n        hero {\n          ...TopFragment @defer(label: \"DeferTop\")\n          ...TopFragment\n        }\n      }\n      fragment TopFragment on Hero {\n        name\n      }\n    ",
		want:  "{\"data\": {\"hero\": {\"name\": \"Luke\"}}}",
	},
	{
		name:  "Can defer a fragment that is also not deferred, non-deferred fragment is first",
		query: "\n      query HeroNameQuery {\n        hero {\n          ...TopFragment\n          ...TopFragment @defer(label: \"DeferTop\")\n        }\n      }\n      fragment TopFragment on Hero {\n        name\n      }\n    ",
		want:  "{\"data\": {\"hero\": {\"name\": \"Luke\"}}}",
	},
	{
		name:  "Can defer an inline fragment",
		query: "\n      query HeroNameQuery {\n        hero {\n          id\n          ... on Hero @defer(label: \"InlineDeferred\") {\n            name\n          }\n        }\n      }\n    ",
		want:  "[{\"data\": {\"hero\": {\"id\": \"1\"}}, \"pending\": [{\"id\": \"0\", \"path\": [\"hero\"], \"label\": \"InlineDeferred\"}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"name\": \"Luke\"}, \"id\": \"0\"}], \"completed\": [{\"id\": \"0\"}], \"hasNext\": false}]",
	},
	{
		name:  "Does not emit empty defer fragments",
		query: "\n      query HeroNameQuery {\n        hero {\n          ... @defer {\n            name @skip(if: true)\n          }\n        }\n      }\n      fragment TopFragment on Hero {\n        name\n      }\n    ",
		want:  "{\"data\": {\"hero\": {}}}",
	},
	{
		name:  "Emits children of empty defer fragments",
		query: "\n      query HeroNameQuery {\n        hero {\n          ... @defer {\n            ... @defer {\n              name\n            }\n          }\n        }\n      }\n    ",
		want:  "[{\"data\": {\"hero\": {}}, \"pending\": [{\"id\": \"0\", \"path\": [\"hero\"]}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"name\": \"Luke\"}, \"id\": \"0\"}], \"completed\": [{\"id\": \"0\"}], \"hasNext\": false}]",
	},
	{
		name:  "Can separately emit defer fragments with different labels with varying fields",
		query: "\n      query HeroNameQuery {\n        hero {\n          ... @defer(label: \"DeferID\") {\n            id\n          }\n          ... @defer(label: \"DeferName\") {\n            name\n          }\n        }\n      }\n    ",
		want:  "[{\"data\": {\"hero\": {}}, \"pending\": [{\"id\": \"0\", \"path\": [\"hero\"], \"label\": \"DeferID\"}, {\"id\": \"1\", \"path\": [\"hero\"], \"label\": \"DeferName\"}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"id\": \"1\"}, \"id\": \"0\"}, {\"data\": {\"name\": \"Luke\"}, \"id\": \"1\"}], \"completed\": [{\"id\": \"0\"}, {\"id\": \"1\"}], \"hasNext\": false}]",
	},
	{
		name:  "Separately emits nested defer fragments with varying subfields of same priorities but different level of defers",
		query: "\n      query HeroNameQuery {\n        ... @defer(label: \"DeferName\") {\n          hero {\n            name\n            ... @defer(label: \"DeferID\") {\n              id\n            }\n          }\n        }\n      }\n    ",
		want:  "[{\"data\": {}, \"pending\": [{\"id\": \"0\", \"path\": [], \"label\": \"DeferName\"}], \"hasNext\": true}, {\"pending\": [{\"id\": \"1\", \"path\": [\"hero\"], \"label\": \"DeferID\"}], \"incremental\": [{\"data\": {\"hero\": {\"name\": \"Luke\"}}, \"id\": \"0\"}, {\"data\": {\"id\": \"1\"}, \"id\": \"1\"}], \"completed\": [{\"id\": \"0\"}, {\"id\": \"1\"}], \"hasNext\": false}]",
	},
	{
		name:  "Can deduplicate multiple defers on the same object",
		query: "\n      query {\n        hero {\n          friends {\n            ... @defer {\n              ...FriendFrag\n              ... @defer {\n                ...FriendFrag\n                ... @defer {\n                  ...FriendFrag\n                  ... @defer {\n                    ...FriendFrag\n                  }\n                }\n              }\n            }\n          }\n        }\n      }\n\n      fragment FriendFrag on Friend {\n        id\n        name\n      }\n    ",
		want:  "[{\"data\": {\"hero\": {\"friends\": [{}, {}, {}]}}, \"pending\": [{\"id\": \"0\", \"path\": [\"hero\", \"friends\", 0]}, {\"id\": \"1\", \"path\": [\"hero\", \"friends\", 1]}, {\"id\": \"2\", \"path\": [\"hero\", \"friends\", 2]}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"id\": \"2\", \"name\": \"Han\"}, \"id\": \"0\"}, {\"data\": {\"id\": \"3\", \"name\": \"Leia\"}, \"id\": \"1\"}, {\"data\": {\"id\": \"4\", \"name\": \"C-3PO\"}, \"id\": \"2\"}], \"completed\": [{\"id\": \"0\"}, {\"id\": \"1\"}, {\"id\": \"2\"}], \"hasNext\": false}]",
	},
	{
		name:  "Deduplicates fields present in a parent defer payload",
		query: "\n      query {\n        hero {\n          ... @defer {\n            nestedObject {\n              deeperObject {\n                foo\n                ... @defer {\n                  foo\n                  bar\n                }\n              }\n            }\n          }\n        }\n      }\n    ",
		root:  "{\"hero\": {\"nestedObject\": {\"deeperObject\": {\"foo\": \"foo\", \"bar\": \"bar\"}}}}",
		want:  "[{\"data\": {\"hero\": {}}, \"pending\": [{\"id\": \"0\", \"path\": [\"hero\"]}], \"hasNext\": true}, {\"pending\": [{\"id\": \"1\", \"path\": [\"hero\", \"nestedObject\", \"deeperObject\"]}], \"incremental\": [{\"data\": {\"nestedObject\": {\"deeperObject\": {\"foo\": \"foo\"}}}, \"id\": \"0\"}, {\"data\": {\"bar\": \"bar\"}, \"id\": \"1\"}], \"completed\": [{\"id\": \"0\"}, {\"id\": \"1\"}], \"hasNext\": false}]",
	},
	{
		name:  "Deduplicates multiple fields from deferred fragments from different branches occurring at the same level",
		query: "\n      query {\n        hero {\n          nestedObject {\n            deeperObject {\n              ... @defer {\n                foo\n              }\n            }\n          }\n          ... @defer {\n            nestedObject {\n              deeperObject {\n                ... @defer {\n                  foo\n                  bar\n                }\n              }\n            }\n          }\n        }\n      }\n    ",
		root:  "{\"hero\": {\"nestedObject\": {\"deeperObject\": {\"foo\": \"foo\", \"bar\": \"bar\"}}}}",
		want:  "[{\"data\": {\"hero\": {\"nestedObject\": {\"deeperObject\": {}}}}, \"pending\": [{\"id\": \"0\", \"path\": [\"hero\", \"nestedObject\", \"deeperObject\"]}, {\"id\": \"1\", \"path\": [\"hero\", \"nestedObject\", \"deeperObject\"]}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"foo\": \"foo\"}, \"id\": \"0\"}, {\"data\": {\"bar\": \"bar\"}, \"id\": \"1\"}], \"completed\": [{\"id\": \"0\"}, {\"id\": \"1\"}], \"hasNext\": false}]",
	},
	{
		name:  "Deduplicate fields with deferred fragments in different branches at multiple non-overlapping levels",
		query: "\n      query {\n        a {\n          b {\n            c {\n              d\n            }\n            ... @defer {\n              e {\n                f\n              }\n            }\n          }\n        }\n        ... @defer {\n          a {\n            b {\n              e {\n                f\n              }\n            }\n          }\n          g {\n            h\n          }\n        }\n      }\n    ",
		root:  "{\"a\": {\"b\": {\"c\": {\"d\": \"d\"}, \"e\": {\"f\": \"f\"}}}, \"g\": {\"h\": \"h\"}}",
		want:  "[{\"data\": {\"a\": {\"b\": {\"c\": {\"d\": \"d\"}}}}, \"pending\": [{\"id\": \"0\", \"path\": [\"a\", \"b\"]}, {\"id\": \"1\", \"path\": []}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"e\": {\"f\": \"f\"}}, \"id\": \"0\"}, {\"data\": {\"g\": {\"h\": \"h\"}}, \"id\": \"1\"}], \"completed\": [{\"id\": \"0\"}, {\"id\": \"1\"}], \"hasNext\": false}]",
	},
	{
		name:  "Handles multiple erroring deferred grouped field sets",
		query: "\n      query {\n        ... @defer {\n          a {\n            b {\n              c {\n                someError: nonNullErrorField\n              }\n            }\n          }\n        }\n        ... @defer {\n          a {\n            b {\n              c {\n                anotherError: nonNullErrorField\n              }\n            }\n          }\n        }\n      }\n    ",
		root:  "{\"a\": {\"b\": {\"c\": {\"nonNullErrorField\": null}}}}",
		want:  "[{\"data\": {}, \"pending\": [{\"id\": \"0\", \"path\": []}, {\"id\": \"1\", \"path\": []}], \"hasNext\": true}, {\"completed\": [{\"id\": \"0\", \"errors\": [{\"message\": \"Cannot return null for non-nullable field c.nonNullErrorField.\", \"locations\": [{\"line\": 7, \"column\": 17}], \"path\": [\"a\", \"b\", \"c\", \"someError\"]}]}, {\"id\": \"1\", \"errors\": [{\"message\": \"Cannot return null for non-nullable field c.nonNullErrorField.\", \"locations\": [{\"line\": 16, \"column\": 17}], \"path\": [\"a\", \"b\", \"c\", \"anotherError\"]}]}], \"hasNext\": false}]",
	},
	{
		name:  "Keeps deferred work outside nulled error paths",
		query: "\n      query {\n        a {\n          ... @defer {\n            someField\n          }\n          nonNullErrorField\n        }\n        g {\n          ... @defer {\n            h\n          }\n        }\n      }\n    ",
		root:  "{\"a\": {\"someField\": \"someField\", \"nonNullErrorField\": null}, \"g\": {\"h\": \"value\"}}",
		want:  "[{\"data\": {\"a\": null, \"g\": {}}, \"errors\": [{\"message\": \"Cannot return null for non-nullable field a.nonNullErrorField.\", \"locations\": [{\"line\": 7, \"column\": 11}], \"path\": [\"a\", \"nonNullErrorField\"]}], \"pending\": [{\"id\": \"0\", \"path\": [\"g\"]}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"h\": \"value\"}, \"id\": \"0\"}], \"completed\": [{\"id\": \"0\"}], \"hasNext\": false}]",
	},
	{
		name:  "Deduplicates list fields",
		query: "\n      query {\n        hero {\n          friends {\n            name\n          }\n          ... @defer {\n            friends {\n              name\n            }\n          }\n        }\n      }\n    ",
		want:  "{\"data\": {\"hero\": {\"friends\": [{\"name\": \"Han\"}, {\"name\": \"Leia\"}, {\"name\": \"C-3PO\"}]}}}",
	},
	{
		name:  "Returns payloads from synchronous data in correct order",
		query: "\n      query HeroNameQuery {\n        hero {\n          id\n          ...NameFragment @defer\n        }\n      }\n      fragment NameFragment on Hero {\n        name\n        friends {\n          ...NestedFragment @defer\n        }\n      }\n      fragment NestedFragment on Friend {\n        name\n      }\n    ",
		want:  "[{\"data\": {\"hero\": {\"id\": \"1\"}}, \"pending\": [{\"id\": \"0\", \"path\": [\"hero\"]}], \"hasNext\": true}, {\"pending\": [{\"id\": \"1\", \"path\": [\"hero\", \"friends\", 0]}, {\"id\": \"2\", \"path\": [\"hero\", \"friends\", 1]}, {\"id\": \"3\", \"path\": [\"hero\", \"friends\", 2]}], \"incremental\": [{\"data\": {\"name\": \"Luke\", \"friends\": [{}, {}, {}]}, \"id\": \"0\"}, {\"data\": {\"name\": \"Han\"}, \"id\": \"1\"}, {\"data\": {\"name\": \"Leia\"}, \"id\": \"2\"}, {\"data\": {\"name\": \"C-3PO\"}, \"id\": \"3\"}], \"completed\": [{\"id\": \"0\"}, {\"id\": \"1\"}, {\"id\": \"2\"}, {\"id\": \"3\"}], \"hasNext\": false}]",
	},
}
