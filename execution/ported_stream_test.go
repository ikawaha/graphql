package execution_test

// Ported from graphql-js src/execution/incremental/__tests__/stream-test.ts.
//
// Most of that file is about an async iterable — a list whose entries arrive
// over time — which a Go resolver answers with a slice instead, so the cases
// about waiting for the next entry have nothing to be. What is left is what
// @stream does with a list already in hand.

import "testing"

func TestPortedStream(t *testing.T) {
	runPortedStream(t, []portedIncrementalCase{
		{
			name:  "Can stream a list field",
			query: "{ scalarList @stream(initialCount: 1) }",
			root:  "{\"scalarList\": [\"apple\", \"banana\", \"coconut\"]}",
			want:  "[{\"data\": {\"scalarList\": [\"apple\"]}, \"pending\": [{\"id\": \"0\", \"path\": [\"scalarList\"]}], \"hasNext\": true}, {\"incremental\": [{\"items\": [\"banana\", \"coconut\"], \"id\": \"0\"}], \"completed\": [{\"id\": \"0\"}], \"hasNext\": false}]",
		},
		{
			name:  "Can use default value of initialCount",
			query: "{ scalarList @stream }",
			root:  "{\"scalarList\": [\"apple\", \"banana\", \"coconut\"]}",
			want:  "[{\"data\": {\"scalarList\": []}, \"pending\": [{\"id\": \"0\", \"path\": [\"scalarList\"]}], \"hasNext\": true}, {\"incremental\": [{\"items\": [\"apple\", \"banana\", \"coconut\"], \"id\": \"0\"}], \"completed\": [{\"id\": \"0\"}], \"hasNext\": false}]",
		},
		{
			name:  "Negative values of initialCount throw field errors",
			query: "{ scalarList @stream(initialCount: -2) }",
			root:  "{\"scalarList\": [\"apple\", \"banana\", \"coconut\"]}",
			want:  "{\"errors\": [{\"message\": \"initialCount must be a positive integer\", \"locations\": [{\"line\": 1, \"column\": 3}], \"path\": [\"scalarList\"]}], \"data\": {\"scalarList\": null}}",
		},
		{
			name:  "Returns label from stream directive",
			query: "{ scalarList @stream(initialCount: 1, label: \"scalar-stream\") }",
			root:  "{\"scalarList\": [\"apple\", \"banana\", \"coconut\"]}",
			want:  "[{\"data\": {\"scalarList\": [\"apple\"]}, \"pending\": [{\"id\": \"0\", \"path\": [\"scalarList\"], \"label\": \"scalar-stream\"}], \"hasNext\": true}, {\"incremental\": [{\"items\": [\"banana\", \"coconut\"], \"id\": \"0\"}], \"completed\": [{\"id\": \"0\"}], \"hasNext\": false}]",
		},
		{
			name:  "Treats null stream label the same as no label",
			query: "{ scalarList @stream(initialCount: 1, label: null) }",
			root:  "{\"scalarList\": [\"apple\", \"banana\", \"coconut\"]}",
			want:  "[{\"data\": {\"scalarList\": [\"apple\"]}, \"pending\": [{\"id\": \"0\", \"path\": [\"scalarList\"]}], \"hasNext\": true}, {\"incremental\": [{\"items\": [\"banana\", \"coconut\"], \"id\": \"0\"}], \"completed\": [{\"id\": \"0\"}], \"hasNext\": false}]",
		},
		{
			name:  "Can disable @stream using if argument",
			query: "{ scalarList @stream(initialCount: 0, if: false) }",
			root:  "{\"scalarList\": [\"apple\", \"banana\", \"coconut\"]}",
			want:  "{\"data\": {\"scalarList\": [\"apple\", \"banana\", \"coconut\"]}}",
		},
		{
			name:  "Does not disable stream with null if argument",
			query: "query ($shouldStream: Boolean) { scalarList @stream(initialCount: 2, if: $shouldStream) }",
			root:  "{\"scalarList\": [\"apple\", \"banana\", \"coconut\"]}",
			want:  "[{\"data\": {\"scalarList\": [\"apple\", \"banana\"]}, \"pending\": [{\"id\": \"0\", \"path\": [\"scalarList\"]}], \"hasNext\": true}, {\"incremental\": [{\"items\": [\"coconut\"], \"id\": \"0\"}], \"completed\": [{\"id\": \"0\"}], \"hasNext\": false}]",
		},
		{
			name:  "Can stream multi-dimensional lists",
			query: "{ scalarListList @stream(initialCount: 1) }",
			root:  "{\"scalarListList\": [[\"apple\", \"apple\", \"apple\"], [\"banana\", \"banana\", \"banana\"], [\"coconut\", \"coconut\", \"coconut\"]]}",
			want:  "[{\"data\": {\"scalarListList\": [[\"apple\", \"apple\", \"apple\"]]}, \"pending\": [{\"id\": \"0\", \"path\": [\"scalarListList\"]}], \"hasNext\": true}, {\"incremental\": [{\"items\": [[\"banana\", \"banana\", \"banana\"], [\"coconut\", \"coconut\", \"coconut\"]], \"id\": \"0\"}], \"completed\": [{\"id\": \"0\"}], \"hasNext\": false}]",
		},
		{
			name:  "Handles null returned in non-null list items after initialCount is reached",
			query: "query { nonNullFriendList @stream(initialCount: 1) { name } }",
			root:  "{\"nonNullFriendList\": [{\"name\": \"Luke\", \"id\": 1}, null, {\"name\": \"Han\", \"id\": 2}]}",
			want:  "[{\"data\": {\"nonNullFriendList\": [{\"name\": \"Luke\"}]}, \"pending\": [{\"id\": \"0\", \"path\": [\"nonNullFriendList\"]}], \"hasNext\": true}, {\"completed\": [{\"id\": \"0\", \"errors\": [{\"message\": \"Cannot return null for non-nullable field Query.nonNullFriendList.\", \"locations\": [{\"line\": 1, \"column\": 9}], \"path\": [\"nonNullFriendList\", 1]}]}], \"hasNext\": false}]",
		},
		{
			name:  "Handles errors thrown by completeValue after initialCount is reached",
			query: "query { scalarList @stream(initialCount: 1) }",
			root:  "{\"scalarList\": [\"Luke\", {}]}",
			want:  "[{\"data\": {\"scalarList\": [\"Luke\"]}, \"pending\": [{\"id\": \"0\", \"path\": [\"scalarList\"]}], \"hasNext\": true}, {\"incremental\": [{\"items\": [null], \"id\": \"0\", \"errors\": [{\"message\": \"String cannot represent value: {}\", \"locations\": [{\"line\": 1, \"column\": 9}], \"path\": [\"scalarList\", 1]}]}], \"completed\": [{\"id\": \"0\"}], \"hasNext\": false}]",
		},
	})
}
