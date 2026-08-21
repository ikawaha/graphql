package execution_test

// Ported from graphql-js src/execution/incremental/__tests__/defer-test.ts.
//
// The cases here are the ones that turn on where a deferred fragment's fields
// sit: a fragment is announced where it was written, and fields of it that are
// found deeper arrive under that same announcement with a subPath saying how
// much further down they go. They were left out of the first pass because this
// implementation gave a deeper piece an announcement of its own instead.

import "testing"

func TestPortedDefer_SubPath(t *testing.T) {
	runPortedIncremental(t, []portedIncrementalCase{
		{
			name:  "Separately emits defer fragments with different labels with varying subfields",
			query: "\n      query HeroNameQuery {\n        ... @defer(label: \"DeferID\") {\n          hero {\n            id\n          }\n        }\n        ... @defer(label: \"DeferName\") {\n          hero {\n            name\n          }\n        }\n      }\n    ",
			want:  "[{\"data\": {}, \"pending\": [{\"id\": \"0\", \"path\": [], \"label\": \"DeferID\"}, {\"id\": \"1\", \"path\": [], \"label\": \"DeferName\"}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"hero\": {}}, \"id\": \"0\"}, {\"data\": {\"id\": \"1\"}, \"id\": \"0\", \"subPath\": [\"hero\"]}, {\"data\": {\"name\": \"Luke\"}, \"id\": \"1\", \"subPath\": [\"hero\"]}], \"completed\": [{\"id\": \"0\"}, {\"id\": \"1\"}], \"hasNext\": false}]",
		},
		{
			name:  "Separately emits defer fragments with varying subfields of same priorities but different level of defers",
			query: "\n      query HeroNameQuery {\n        hero {\n          ... @defer(label: \"DeferID\") {\n            id\n          }\n        }\n        ... @defer(label: \"DeferName\") {\n          hero {\n            name\n          }\n        }\n      }\n    ",
			want:  "[{\"data\": {\"hero\": {}}, \"pending\": [{\"id\": \"0\", \"path\": [\"hero\"], \"label\": \"DeferID\"}, {\"id\": \"1\", \"path\": [], \"label\": \"DeferName\"}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"id\": \"1\"}, \"id\": \"0\"}, {\"data\": {\"name\": \"Luke\"}, \"id\": \"1\", \"subPath\": [\"hero\"]}], \"completed\": [{\"id\": \"0\"}, {\"id\": \"1\"}], \"hasNext\": false}]",
		},
		{
			name:  "Deduplicates fields present in the initial payload",
			query: "\n      query {\n        hero {\n          nestedObject {\n            deeperObject {\n              foo\n            }\n          }\n          anotherNestedObject {\n            deeperObject {\n              foo\n            }\n          }\n          ... @defer {\n            nestedObject {\n              deeperObject {\n                bar\n              }\n            }\n            anotherNestedObject {\n              deeperObject {\n                foo\n              }\n            }\n          }\n        }\n      }\n    ",
			root:  "{\"hero\": {\"nestedObject\": {\"deeperObject\": {\"foo\": \"foo\", \"bar\": \"bar\"}}, \"anotherNestedObject\": {\"deeperObject\": {\"foo\": \"foo\"}}}}",
			want:  "[{\"data\": {\"hero\": {\"nestedObject\": {\"deeperObject\": {\"foo\": \"foo\"}}, \"anotherNestedObject\": {\"deeperObject\": {\"foo\": \"foo\"}}}}, \"pending\": [{\"id\": \"0\", \"path\": [\"hero\"]}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"bar\": \"bar\"}, \"id\": \"0\", \"subPath\": [\"nestedObject\", \"deeperObject\"]}], \"completed\": [{\"id\": \"0\"}], \"hasNext\": false}]",
		},
		{
			name:  "Deduplicates fields with deferred fragments at multiple levels",
			query: "\n      query {\n        hero {\n          nestedObject {\n            deeperObject {\n              foo\n            }\n          }\n          ... @defer {\n            nestedObject {\n              deeperObject {\n                foo\n                bar\n              }\n              ... @defer {\n                deeperObject {\n                  foo\n                  bar\n                  baz\n                  ... @defer {\n                    foo\n                    bar\n                    baz\n                    bak\n                  }\n                }\n              }\n            }\n          }\n        }\n      }\n    ",
			root:  "{\"hero\": {\"nestedObject\": {\"deeperObject\": {\"foo\": \"foo\", \"bar\": \"bar\", \"baz\": \"baz\", \"bak\": \"bak\"}}}}",
			want:  "[{\"data\": {\"hero\": {\"nestedObject\": {\"deeperObject\": {\"foo\": \"foo\"}}}}, \"pending\": [{\"id\": \"0\", \"path\": [\"hero\"]}], \"hasNext\": true}, {\"pending\": [{\"id\": \"1\", \"path\": [\"hero\", \"nestedObject\"]}, {\"id\": \"2\", \"path\": [\"hero\", \"nestedObject\", \"deeperObject\"]}], \"incremental\": [{\"data\": {\"bar\": \"bar\"}, \"id\": \"0\", \"subPath\": [\"nestedObject\", \"deeperObject\"]}, {\"data\": {\"baz\": \"baz\"}, \"id\": \"1\", \"subPath\": [\"deeperObject\"]}, {\"data\": {\"bak\": \"bak\"}, \"id\": \"2\"}], \"completed\": [{\"id\": \"0\"}, {\"id\": \"1\"}, {\"id\": \"2\"}], \"hasNext\": false}]",
		},
		{
			name:  "Correctly bundles varying subfields into incremental data records unique by defer combination, ignoring fields in a fragment masked by a parent defer",
			query: "\n      query HeroNameQuery {\n        ... @defer {\n          hero {\n            id\n          }\n        }\n        ... @defer {\n          hero {\n            name\n            shouldBeWithNameDespiteAdditionalDefer: name\n            ... @defer {\n              shouldBeWithNameDespiteAdditionalDefer: name\n            }\n          }\n        }\n      }\n    ",
			want:  "[{\"data\": {}, \"pending\": [{\"id\": \"0\", \"path\": []}, {\"id\": \"1\", \"path\": []}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"hero\": {}}, \"id\": \"0\"}, {\"data\": {\"id\": \"1\"}, \"id\": \"0\", \"subPath\": [\"hero\"]}, {\"data\": {\"name\": \"Luke\", \"shouldBeWithNameDespiteAdditionalDefer\": \"Luke\"}, \"id\": \"1\", \"subPath\": [\"hero\"]}], \"completed\": [{\"id\": \"0\"}, {\"id\": \"1\"}], \"hasNext\": false}]",
		},
		{
			name:  "Nulls cross defer boundaries, null first",
			query: "\n      query {\n        ... @defer {\n          a {\n            someField\n            b {\n              c {\n                nonNullErrorField\n              }\n            }\n          }\n        }\n        a {\n          ... @defer {\n            b {\n              c {\n                d\n              }\n            }\n          }\n        }\n      }\n    ",
			root:  "{\"a\": {\"b\": {\"c\": {\"d\": \"d\"}}, \"someField\": \"someField\"}}",
			want:  "[{\"data\": {\"a\": {}}, \"pending\": [{\"id\": \"0\", \"path\": [\"a\"]}, {\"id\": \"1\", \"path\": []}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"b\": {\"c\": {}}}, \"id\": \"0\"}, {\"data\": {\"d\": \"d\"}, \"id\": \"0\", \"subPath\": [\"b\", \"c\"]}], \"completed\": [{\"id\": \"1\", \"errors\": [{\"message\": \"Cannot return null for non-nullable field c.nonNullErrorField.\", \"locations\": [{\"line\": 8, \"column\": 17}], \"path\": [\"a\", \"b\", \"c\", \"nonNullErrorField\"]}]}, {\"id\": \"0\"}], \"hasNext\": false}]",
		},
		{
			name:  "Nulls cross defer boundaries, value first",
			query: "\n      query {\n        ... @defer {\n          a {\n            b {\n              c {\n                d\n              }\n            }\n          }\n        }\n        a {\n          ... @defer {\n            someField\n            b {\n              c {\n                nonNullErrorField\n              }\n            }\n          }\n        }\n      }\n    ",
			root:  "{\"a\": {\"b\": {\"c\": {\"d\": \"d\"}, \"nonNullErrorFIeld\": null}, \"someField\": \"someField\"}}",
			want:  "[{\"data\": {\"a\": {}}, \"pending\": [{\"id\": \"0\", \"path\": [\"a\"]}, {\"id\": \"1\", \"path\": []}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"b\": {\"c\": {}}}, \"id\": \"0\"}, {\"data\": {\"d\": \"d\"}, \"id\": \"1\", \"subPath\": [\"a\", \"b\", \"c\"]}], \"completed\": [{\"id\": \"1\"}, {\"id\": \"0\", \"errors\": [{\"message\": \"Cannot return null for non-nullable field c.nonNullErrorField.\", \"locations\": [{\"line\": 17, \"column\": 17}], \"path\": [\"a\", \"b\", \"c\", \"nonNullErrorField\"]}]}], \"hasNext\": false}]",
		},
		{
			name:  "Handles cancelling child deferred fragments if parent fragment fails",
			query: "\n      query {\n        ... @defer {\n          a {\n            someField\n            b {\n              c {\n                nonNullErrorField\n              }\n            }\n          }\n          ... @defer {\n            a {\n              someField\n            }\n          }\n        }\n        a {\n          ... @defer {\n            b {\n              c {\n                d\n              }\n            }\n          }\n        }\n      }\n    ",
			root:  "{\"a\": {\"b\": {\"c\": {\"d\": \"d\"}}, \"someField\": \"someField\"}}",
			want:  "[{\"data\": {\"a\": {}}, \"pending\": [{\"id\": \"0\", \"path\": [\"a\"]}, {\"id\": \"1\", \"path\": []}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"b\": {\"c\": {}}}, \"id\": \"0\"}, {\"data\": {\"d\": \"d\"}, \"id\": \"0\", \"subPath\": [\"b\", \"c\"]}], \"completed\": [{\"id\": \"1\", \"errors\": [{\"message\": \"Cannot return null for non-nullable field c.nonNullErrorField.\", \"locations\": [{\"line\": 8, \"column\": 17}], \"path\": [\"a\", \"b\", \"c\", \"nonNullErrorField\"]}]}, {\"id\": \"0\"}], \"hasNext\": false}]",
		},
		{
			name:  "Handles multiple erroring deferred grouped field sets for the same fragment",
			query: "\n      query {\n        ... @defer {\n          a {\n            b {\n              someC: c {\n                d: d\n              }\n              anotherC: c {\n                d: d\n              }\n            }\n          }\n        }\n        ... @defer {\n          a {\n            b {\n              someC: c {\n                someError: nonNullErrorField\n              }\n              anotherC: c {\n                anotherError: nonNullErrorField\n              }\n            }\n          }\n        }\n      }\n    ",
			root:  "{\"a\": {\"b\": {\"c\": {\"d\": \"d\", \"nonNullErrorField\": null}}}}",
			want:  "[{\"data\": {}, \"pending\": [{\"id\": \"0\", \"path\": []}, {\"id\": \"1\", \"path\": []}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"a\": {\"b\": {\"someC\": {}, \"anotherC\": {}}}}, \"id\": \"0\"}, {\"data\": {\"d\": \"d\"}, \"id\": \"0\", \"subPath\": [\"a\", \"b\", \"someC\"]}, {\"data\": {\"d\": \"d\"}, \"id\": \"0\", \"subPath\": [\"a\", \"b\", \"anotherC\"]}], \"completed\": [{\"id\": \"1\", \"errors\": [{\"message\": \"Cannot return null for non-nullable field c.nonNullErrorField.\", \"locations\": [{\"line\": 19, \"column\": 17}], \"path\": [\"a\", \"b\", \"someC\", \"someError\"]}]}, {\"id\": \"0\"}], \"hasNext\": false}]",
		},
		{
			name:  "Does not deduplicate list fields with non-overlapping fields",
			query: "\n      query {\n        hero {\n          friends {\n            name\n          }\n          ... @defer {\n            friends {\n              id\n            }\n          }\n        }\n      }\n    ",
			want:  "[{\"data\": {\"hero\": {\"friends\": [{\"name\": \"Han\"}, {\"name\": \"Leia\"}, {\"name\": \"C-3PO\"}]}}, \"pending\": [{\"id\": \"0\", \"path\": [\"hero\"]}], \"hasNext\": true}, {\"incremental\": [{\"data\": {\"id\": \"2\"}, \"id\": \"0\", \"subPath\": [\"friends\", 0]}, {\"data\": {\"id\": \"3\"}, \"id\": \"0\", \"subPath\": [\"friends\", 1]}, {\"data\": {\"id\": \"4\"}, \"id\": \"0\", \"subPath\": [\"friends\", 2]}], \"completed\": [{\"id\": \"0\"}], \"hasNext\": false}]",
		},
	})
}
