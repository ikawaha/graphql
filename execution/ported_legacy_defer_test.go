package execution_test

// Ported from graphql-js
// src/execution/legacyIncremental/__tests__/legacy-defer-test.ts.
//
// The cases left out need promises — a resolver that has not answered yet, an
// abort part way through — or a root value holding functions, which the
// extractor cannot take and the harness supplies another way.

import "testing"

func TestPortedLegacyDefer(t *testing.T) {
	runPortedLegacy(t, portedIncrementalSDL,
		func(*testing.T) any { return portedIncrementalRoot() }, []portedIncrementalCase{
			{
				name:  "Can defer fragments containing scalar types",
				query: "\n      query HeroNameQuery {\n        hero {\n          id\n          ...NameFragment @defer\n        }\n      }\n      fragment NameFragment on Hero {\n        name\n      }\n    ",
				want:  "[{\"data\": {\"hero\": {\"id\": \"1\"}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"name\": \"Luke\"}, \"path\": [\"hero\"]}], \"hasNext\": false}]",
			},
			{
				name:  "Returns label from defer directive",
				query: "\n      query HeroNameQuery {\n        hero {\n          id\n          ...NameFragment @defer(label: \"defer-label\")\n        }\n      }\n      fragment NameFragment on Hero {\n        name\n      }\n    ",
				want:  "[{\"data\": {\"hero\": {\"id\": \"1\"}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"name\": \"Luke\"}, \"path\": [\"hero\"], \"label\": \"defer-label\"}], \"hasNext\": false}]",
			},
			{
				name:  "Treats null defer label the same as no label",
				query: "\n      query HeroNameQuery {\n        hero {\n          id\n          ...NameFragment @defer(label: null)\n        }\n      }\n      fragment NameFragment on Hero {\n        name\n      }\n    ",
				want:  "[{\"data\": {\"hero\": {\"id\": \"1\"}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"name\": \"Luke\"}, \"path\": [\"hero\"]}], \"hasNext\": false}]",
			},
			{
				name:  "Can disable defer using if argument",
				query: "\n      query HeroNameQuery {\n        hero {\n          id\n          ...NameFragment @defer(if: false)\n        }\n      }\n      fragment NameFragment on Hero {\n        name\n      }\n    ",
				want:  "{\"data\": {\"hero\": {\"id\": \"1\", \"name\": \"Luke\"}}}",
			},
			{
				name:  "Does not disable defer with null if argument",
				query: "\n      query HeroNameQuery($shouldDefer: Boolean) {\n        hero {\n          id\n          ...NameFragment @defer(if: $shouldDefer)\n        }\n      }\n      fragment NameFragment on Hero {\n        name\n      }\n    ",
				want:  "[{\"data\": {\"hero\": {\"id\": \"1\"}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"name\": \"Luke\"}, \"path\": [\"hero\"]}], \"hasNext\": false}]",
			},
			{
				name:  "Can defer fragments on the top level Query field",
				query: "\n      query HeroNameQuery {\n        ...QueryFragment @defer(label: \"DeferQuery\")\n      }\n      fragment QueryFragment on Query {\n        hero {\n          id\n        }\n      }\n    ",
				want:  "[{\"data\": {}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"hero\": {\"id\": \"1\"}}, \"path\": [], \"label\": \"DeferQuery\"}], \"hasNext\": false}]",
			},
			{
				name:  "Can defer a fragment within an already deferred fragment",
				query: "\n      query HeroNameQuery {\n        hero {\n          ...TopFragment @defer(label: \"DeferTop\")\n        }\n      }\n      fragment TopFragment on Hero {\n        id\n        ...NestedFragment @defer(label: \"DeferNested\")\n      }\n      fragment NestedFragment on Hero {\n        friends {\n          name\n        }\n      }\n    ",
				want:  "[{\"data\": {\"hero\": {}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"id\": \"1\"}, \"path\": [\"hero\"], \"label\": \"DeferTop\"}, {\"data\": {\"friends\": [{\"name\": \"Han\"}, {\"name\": \"Leia\"}, {\"name\": \"C-3PO\"}]}, \"path\": [\"hero\"], \"label\": \"DeferNested\"}], \"hasNext\": false}]",
			},
			{
				name:  "Emits deferred fragments even when also selected without @defer, deferred fragment is first",
				query: "\n      query HeroNameQuery {\n        hero {\n          ...TopFragment @defer(label: \"DeferTop\")\n          ...TopFragment\n        }\n      }\n      fragment TopFragment on Hero {\n        name\n      }\n    ",
				want:  "[{\"data\": {\"hero\": {\"name\": \"Luke\"}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"name\": \"Luke\"}, \"path\": [\"hero\"], \"label\": \"DeferTop\"}], \"hasNext\": false}]",
			},
			{
				name:  "Skips deferred fragments when also selected without @defer, non-deferred fragment is first",
				query: "\n      query HeroNameQuery {\n        hero {\n          ...TopFragment\n          ...TopFragment @defer(label: \"DeferTop\")\n        }\n      }\n      fragment TopFragment on Hero {\n        name\n      }\n    ",
				want:  "{\"data\": {\"hero\": {\"name\": \"Luke\"}}}",
			},
			{
				name:  "Can defer an inline fragment",
				query: "\n      query HeroNameQuery {\n        hero {\n          id\n          ... on Hero @defer(label: \"InlineDeferred\") {\n            name\n          }\n        }\n      }\n    ",
				want:  "[{\"data\": {\"hero\": {\"id\": \"1\"}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"name\": \"Luke\"}, \"path\": [\"hero\"], \"label\": \"InlineDeferred\"}], \"hasNext\": false}]",
			},
			{
				name:  "Does not emit empty defer fragments",
				query: "\n      query HeroNameQuery {\n        hero {\n          ... @defer {\n            name @skip(if: true)\n          }\n        }\n      }\n      fragment TopFragment on Hero {\n        name\n      }\n    ",
				want:  "{\"data\": {\"hero\": {}}}",
			},
			{
				name:  "Emits children of empty defer fragments",
				query: "\n      query HeroNameQuery {\n        hero {\n          ... @defer {\n            ... @defer {\n              name\n            }\n          }\n        }\n      }\n    ",
				want:  "[{\"data\": {\"hero\": {}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"name\": \"Luke\"}, \"path\": [\"hero\"]}], \"hasNext\": false}]",
			},
			{
				name:  "Can separately emit defer fragments with different labels with varying fields",
				query: "\n      query HeroNameQuery {\n        hero {\n          ... @defer(label: \"DeferID\") {\n            id\n          }\n          ... @defer(label: \"DeferName\") {\n            name\n          }\n        }\n      }\n    ",
				want:  "[{\"data\": {\"hero\": {}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"id\": \"1\"}, \"path\": [\"hero\"], \"label\": \"DeferID\"}, {\"data\": {\"name\": \"Luke\"}, \"path\": [\"hero\"], \"label\": \"DeferName\"}], \"hasNext\": false}]",
			},
			{
				name:  "Separately emits defer fragments with different labels with varying subfields",
				query: "\n      query HeroNameQuery {\n        ... @defer(label: \"DeferID\") {\n          hero {\n            id\n          }\n        }\n        ... @defer(label: \"DeferName\") {\n          hero {\n            name\n          }\n        }\n      }\n    ",
				want:  "[{\"data\": {}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"hero\": {\"id\": \"1\"}}, \"path\": [], \"label\": \"DeferID\"}, {\"data\": {\"hero\": {\"name\": \"Luke\"}}, \"path\": [], \"label\": \"DeferName\"}], \"hasNext\": false}]",
			},
			{
				name:  "Separately emits defer fragments with varying subfields of same priorities but different level of defers",
				query: "\n      query HeroNameQuery {\n        hero {\n          ... @defer(label: \"DeferID\") {\n            id\n          }\n        }\n        ... @defer(label: \"DeferName\") {\n          hero {\n            name\n          }\n        }\n      }\n    ",
				want:  "[{\"data\": {\"hero\": {}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"id\": \"1\"}, \"path\": [\"hero\"], \"label\": \"DeferID\"}, {\"data\": {\"hero\": {\"name\": \"Luke\"}}, \"path\": [], \"label\": \"DeferName\"}], \"hasNext\": false}]",
			},
			{
				name:  "Separately emits nested defer fragments with varying subfields of same priorities but different level of defers",
				query: "\n      query HeroNameQuery {\n        ... @defer(label: \"DeferName\") {\n          hero {\n            name\n            ... @defer(label: \"DeferID\") {\n              id\n            }\n          }\n        }\n      }\n    ",
				want:  "[{\"data\": {}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"hero\": {\"name\": \"Luke\"}}, \"path\": [], \"label\": \"DeferName\"}, {\"data\": {\"id\": \"1\"}, \"path\": [\"hero\"], \"label\": \"DeferID\"}], \"hasNext\": false}]",
			},
			{
				name:  "Skips duplicate nested defers on the same object",
				query: "\n      query {\n        hero {\n          friends {\n            ... @defer {\n              ...FriendFrag\n              ... @defer {\n                ...FriendFrag\n                ... @defer {\n                  ...FriendFrag\n                  ... @defer {\n                    ...FriendFrag\n                  }\n                }\n              }\n            }\n          }\n        }\n      }\n\n      fragment FriendFrag on Friend {\n        id\n        name\n      }\n    ",
				want:  "[{\"data\": {\"hero\": {\"friends\": [{}, {}, {}]}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"id\": \"2\", \"name\": \"Han\"}, \"path\": [\"hero\", \"friends\", 0]}, {\"data\": {\"id\": \"3\", \"name\": \"Leia\"}, \"path\": [\"hero\", \"friends\", 1]}, {\"data\": {\"id\": \"4\", \"name\": \"C-3PO\"}, \"path\": [\"hero\", \"friends\", 2]}], \"hasNext\": false}]",
			},
			{
				name:  "Does not deduplicate fields present in the initial payload",
				query: "\n      query {\n        hero {\n          nestedObject {\n            deeperObject {\n              foo\n            }\n          }\n          anotherNestedObject {\n            deeperObject {\n              foo\n            }\n          }\n          ... @defer {\n            nestedObject {\n              deeperObject {\n                bar\n              }\n            }\n            anotherNestedObject {\n              deeperObject {\n                foo\n              }\n            }\n          }\n        }\n      }\n    ",
				root:  "{\"hero\": {\"nestedObject\": {\"deeperObject\": {\"foo\": \"foo\", \"bar\": \"bar\"}}, \"anotherNestedObject\": {\"deeperObject\": {\"foo\": \"foo\"}}}}",
				want:  "[{\"data\": {\"hero\": {\"nestedObject\": {\"deeperObject\": {\"foo\": \"foo\"}}, \"anotherNestedObject\": {\"deeperObject\": {\"foo\": \"foo\"}}}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"nestedObject\": {\"deeperObject\": {\"bar\": \"bar\"}}, \"anotherNestedObject\": {\"deeperObject\": {\"foo\": \"foo\"}}}, \"path\": [\"hero\"]}], \"hasNext\": false}]",
			},
			{
				name:  "Does not deduplicate fields present in a parent defer payload",
				query: "\n      query {\n        hero {\n          ... @defer {\n            nestedObject {\n              deeperObject {\n                foo\n                ... @defer {\n                  foo\n                  bar\n                }\n              }\n            }\n          }\n        }\n      }\n    ",
				root:  "{\"hero\": {\"nestedObject\": {\"deeperObject\": {\"foo\": \"foo\", \"bar\": \"bar\"}}}}",
				want:  "[{\"data\": {\"hero\": {}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"nestedObject\": {\"deeperObject\": {\"foo\": \"foo\"}}}, \"path\": [\"hero\"]}, {\"data\": {\"foo\": \"foo\", \"bar\": \"bar\"}, \"path\": [\"hero\", \"nestedObject\", \"deeperObject\"]}], \"hasNext\": false}]",
			},
			{
				name:  "Skips duplicate fields with deferred fragments at multiple levels",
				query: "\n      query {\n        hero {\n          nestedObject {\n            deeperObject {\n              foo\n            }\n          }\n          ... @defer {\n            nestedObject {\n              deeperObject {\n                foo\n                bar\n              }\n              ... @defer {\n                deeperObject {\n                  foo\n                  bar\n                  baz\n                  ... @defer {\n                    foo\n                    bar\n                    baz\n                    bak\n                  }\n                }\n              }\n            }\n          }\n        }\n      }\n    ",
				root:  "{\"hero\": {\"nestedObject\": {\"deeperObject\": {\"foo\": \"foo\", \"bar\": \"bar\", \"baz\": \"baz\", \"bak\": \"bak\"}}}}",
				want:  "[{\"data\": {\"hero\": {\"nestedObject\": {\"deeperObject\": {\"foo\": \"foo\"}}}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"nestedObject\": {\"deeperObject\": {\"foo\": \"foo\", \"bar\": \"bar\"}}}, \"path\": [\"hero\"]}, {\"data\": {\"deeperObject\": {\"foo\": \"foo\", \"bar\": \"bar\", \"baz\": \"baz\"}}, \"path\": [\"hero\", \"nestedObject\"]}, {\"data\": {\"foo\": \"foo\", \"bar\": \"bar\", \"baz\": \"baz\", \"bak\": \"bak\"}, \"path\": [\"hero\", \"nestedObject\", \"deeperObject\"]}], \"hasNext\": false}]",
			},
			{
				name:  "Does not deduplicate multiple fields from deferred fragments from different branches occurring at the same level",
				query: "\n      query {\n        hero {\n          nestedObject {\n            deeperObject {\n              ... @defer {\n                foo\n              }\n            }\n          }\n          ... @defer {\n            nestedObject {\n              deeperObject {\n                ... @defer {\n                  foo\n                  bar\n                }\n              }\n            }\n          }\n        }\n      }\n    ",
				root:  "{\"hero\": {\"nestedObject\": {\"deeperObject\": {\"foo\": \"foo\", \"bar\": \"bar\"}}}}",
				want:  "[{\"data\": {\"hero\": {\"nestedObject\": {\"deeperObject\": {}}}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"foo\": \"foo\"}, \"path\": [\"hero\", \"nestedObject\", \"deeperObject\"]}, {\"data\": {\"nestedObject\": {\"deeperObject\": {}}}, \"path\": [\"hero\"]}, {\"data\": {\"foo\": \"foo\", \"bar\": \"bar\"}, \"path\": [\"hero\", \"nestedObject\", \"deeperObject\"]}], \"hasNext\": false}]",
			},
			{
				name:  "Does not deduplicate fields with deferred fragments in different branches at multiple non-overlapping levels",
				query: "\n      query {\n        a {\n          b {\n            c {\n              d\n            }\n            ... @defer {\n              e {\n                f\n              }\n            }\n          }\n        }\n        ... @defer {\n          a {\n            b {\n              e {\n                f\n              }\n            }\n          }\n          g {\n            h\n          }\n        }\n      }\n    ",
				root:  "{\"a\": {\"b\": {\"c\": {\"d\": \"d\"}, \"e\": {\"f\": \"f\"}}}, \"g\": {\"h\": \"h\"}}",
				want:  "[{\"data\": {\"a\": {\"b\": {\"c\": {\"d\": \"d\"}}}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"e\": {\"f\": \"f\"}}, \"path\": [\"a\", \"b\"]}, {\"data\": {\"a\": {\"b\": {\"e\": {\"f\": \"f\"}}}, \"g\": {\"h\": \"h\"}}, \"path\": []}], \"hasNext\": false}]",
			},
			{
				name:  "Correctly bundles varying subfields into incremental data records, duplicating fields from a parent defer",
				query: "\n      query HeroNameQuery {\n        ... @defer {\n          hero {\n            id\n          }\n        }\n        ... @defer {\n          hero {\n            name\n            shouldBeWithNameDespiteAdditionalDefer: name\n            ... @defer {\n              shouldBeWithNameDespiteAdditionalDefer: name\n            }\n          }\n        }\n      }\n    ",
				want:  "[{\"data\": {}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"hero\": {\"id\": \"1\"}}, \"path\": []}, {\"data\": {\"hero\": {\"name\": \"Luke\", \"shouldBeWithNameDespiteAdditionalDefer\": \"Luke\"}}, \"path\": []}, {\"data\": {\"shouldBeWithNameDespiteAdditionalDefer\": \"Luke\"}, \"path\": [\"hero\"]}], \"hasNext\": false}]",
			},
			{
				name:  "Nulls cross defer boundaries, null first",
				query: "\n      query {\n        ... @defer {\n          a {\n            someField\n            b {\n              c {\n                nonNullErrorField\n              }\n            }\n          }\n        }\n        a {\n          ... @defer {\n            b {\n              c {\n                d\n              }\n            }\n          }\n        }\n      }\n    ",
				root:  "{\"a\": {\"b\": {\"c\": {\"d\": \"d\"}}, \"someField\": \"someField\"}}",
				want:  "[{\"data\": {\"a\": {}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"b\": {\"c\": {\"d\": \"d\"}}}, \"path\": [\"a\"]}, {\"data\": {\"a\": {\"b\": {\"c\": null}, \"someField\": \"someField\"}}, \"path\": [], \"errors\": [{\"message\": \"Cannot return null for non-nullable field c.nonNullErrorField.\", \"locations\": [{\"line\": 8, \"column\": 17}], \"path\": [\"a\", \"b\", \"c\", \"nonNullErrorField\"]}]}], \"hasNext\": false}]",
			},
			{
				name:  "Nulls cross defer boundaries, value first",
				query: "\n      query {\n        ... @defer {\n          a {\n            b {\n              c {\n                d\n              }\n            }\n          }\n        }\n        a {\n          ... @defer {\n            someField\n            b {\n              c {\n                nonNullErrorField\n              }\n            }\n          }\n        }\n      }\n    ",
				root:  "{\"a\": {\"b\": {\"c\": {\"d\": \"d\"}, \"nonNullErrorFIeld\": null}, \"someField\": \"someField\"}}",
				want:  "[{\"data\": {\"a\": {}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"b\": {\"c\": null}, \"someField\": \"someField\"}, \"path\": [\"a\"], \"errors\": [{\"message\": \"Cannot return null for non-nullable field c.nonNullErrorField.\", \"locations\": [{\"line\": 17, \"column\": 17}], \"path\": [\"a\", \"b\", \"c\", \"nonNullErrorField\"]}]}, {\"data\": {\"a\": {\"b\": {\"c\": {\"d\": \"d\"}}}}, \"path\": []}], \"hasNext\": false}]",
			},
			{
				name:  "Handles cancelling child deferred fragments if parent fragment fails",
				query: "\n      query {\n        ... @defer {\n          a {\n            someField\n            b {\n              c {\n                nonNullErrorField\n              }\n            }\n          }\n          ... @defer {\n            a {\n              someField\n            }\n          }\n        }\n        a {\n          ... @defer {\n            b {\n              c {\n                d\n              }\n            }\n          }\n        }\n      }\n    ",
				root:  "{\"a\": {\"b\": {\"c\": {\"d\": \"d\"}}, \"someField\": \"someField\"}}",
				want:  "[{\"data\": {\"a\": {}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"b\": {\"c\": {\"d\": \"d\"}}}, \"path\": [\"a\"]}, {\"data\": {\"a\": {\"b\": {\"c\": null}, \"someField\": \"someField\"}}, \"path\": [], \"errors\": [{\"message\": \"Cannot return null for non-nullable field c.nonNullErrorField.\", \"locations\": [{\"line\": 8, \"column\": 17}], \"path\": [\"a\", \"b\", \"c\", \"nonNullErrorField\"]}]}, {\"data\": {\"a\": {\"someField\": \"someField\"}}, \"path\": []}], \"hasNext\": false}]",
			},
			{
				name:  "Handles multiple erroring deferred grouped field sets",
				query: "\n      query {\n        a {\n          b {\n            c {\n              ... @defer {\n                someError: nonNullErrorField\n              }\n            }\n          }\n        }\n        a {\n          b {\n            c {\n              ... @defer {\n                anotherError: nonNullErrorField\n              }\n            }\n          }\n        }\n      }\n    ",
				root:  "{\"a\": {\"b\": {\"c\": {\"nonNullErrorField\": null}}}}",
				want:  "[{\"data\": {\"a\": {\"b\": {\"c\": {}}}}, \"hasNext\": true}, {\"incremental\": [{\"data\": null, \"path\": [\"a\", \"b\", \"c\"], \"errors\": [{\"message\": \"Cannot return null for non-nullable field c.nonNullErrorField.\", \"locations\": [{\"line\": 7, \"column\": 17}], \"path\": [\"a\", \"b\", \"c\", \"someError\"]}]}, {\"data\": null, \"path\": [\"a\", \"b\", \"c\"], \"errors\": [{\"message\": \"Cannot return null for non-nullable field c.nonNullErrorField.\", \"locations\": [{\"line\": 16, \"column\": 17}], \"path\": [\"a\", \"b\", \"c\", \"anotherError\"]}]}], \"hasNext\": false}]",
			},
			{
				name:  "Handles multiple erroring deferred grouped field sets for the same fragment",
				query: "\n      query {\n        a {\n          b {\n            c {\n              ... @defer {\n                someError: nonNullErrorField\n                anotherError: nonNullErrorField\n              }\n            }\n          }\n        }\n      }\n    ",
				root:  "{\"a\": {\"b\": {\"c\": {\"d\": \"d\", \"nonNullErrorField\": null}}}}",
				want:  "[{\"data\": {\"a\": {\"b\": {\"c\": {}}}}, \"hasNext\": true}, {\"incremental\": [{\"data\": null, \"path\": [\"a\", \"b\", \"c\"], \"errors\": [{\"message\": \"Cannot return null for non-nullable field c.nonNullErrorField.\", \"locations\": [{\"line\": 7, \"column\": 17}], \"path\": [\"a\", \"b\", \"c\", \"someError\"]}]}], \"hasNext\": false}]",
			},
			{
				name:  "Keeps deferred work outside nulled error paths",
				query: "\n      query {\n        a {\n          ... @defer {\n            someField\n          }\n          nonNullErrorField\n        }\n        g {\n          ... @defer {\n            h\n          }\n        }\n      }\n    ",
				root:  "{\"a\": {\"someField\": \"someField\", \"nonNullErrorField\": null}, \"g\": {\"h\": \"value\"}}",
				want:  "[{\"data\": {\"a\": null, \"g\": {}}, \"errors\": [{\"message\": \"Cannot return null for non-nullable field a.nonNullErrorField.\", \"locations\": [{\"line\": 7, \"column\": 11}], \"path\": [\"a\", \"nonNullErrorField\"]}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"h\": \"value\"}, \"path\": [\"g\"]}], \"hasNext\": false}]",
			},
			{
				name:  "Does not deduplicate list fields",
				query: "\n      query {\n        hero {\n          friends {\n            name\n          }\n          ... @defer {\n            friends {\n              name\n            }\n          }\n        }\n      }\n    ",
				want:  "[{\"data\": {\"hero\": {\"friends\": [{\"name\": \"Han\"}, {\"name\": \"Leia\"}, {\"name\": \"C-3PO\"}]}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"friends\": [{\"name\": \"Han\"}, {\"name\": \"Leia\"}, {\"name\": \"C-3PO\"}]}, \"path\": [\"hero\"]}], \"hasNext\": false}]",
			},
			{
				name:  "Does not deduplicate list fields with non-overlapping fields",
				query: "\n      query {\n        hero {\n          friends {\n            name\n          }\n          ... @defer {\n            friends {\n              id\n            }\n          }\n        }\n      }\n    ",
				want:  "[{\"data\": {\"hero\": {\"friends\": [{\"name\": \"Han\"}, {\"name\": \"Leia\"}, {\"name\": \"C-3PO\"}]}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"friends\": [{\"id\": \"2\"}, {\"id\": \"3\"}, {\"id\": \"4\"}]}, \"path\": [\"hero\"]}], \"hasNext\": false}]",
			},
			{
				name:  "Returns payloads from synchronous data in correct order",
				query: "\n      query HeroNameQuery {\n        hero {\n          id\n          ...NameFragment @defer\n        }\n      }\n      fragment NameFragment on Hero {\n        name\n        friends {\n          ...NestedFragment @defer\n        }\n      }\n      fragment NestedFragment on Friend {\n        name\n      }\n    ",
				want:  "[{\"data\": {\"hero\": {\"id\": \"1\"}}, \"hasNext\": true}, {\"incremental\": [{\"data\": {\"name\": \"Luke\", \"friends\": [{}, {}, {}]}, \"path\": [\"hero\"]}, {\"data\": {\"name\": \"Han\"}, \"path\": [\"hero\", \"friends\", 0]}, {\"data\": {\"name\": \"Leia\"}, \"path\": [\"hero\", \"friends\", 1]}, {\"data\": {\"name\": \"C-3PO\"}, \"path\": [\"hero\", \"friends\", 2]}], \"hasNext\": false}]",
			},
		})
}
