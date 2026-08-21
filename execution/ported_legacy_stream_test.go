package execution_test

// Ported from graphql-js
// src/execution/legacyIncremental/__tests__/legacy-stream-test.ts.
//
// Most of that file is about an async iterable — a list whose entries arrive
// over time — which a Go resolver answers with a slice instead. What is left
// is what @stream does with a list already in hand, in the older payload
// format: a payload names the place of its first entry.

import "testing"

func TestPortedLegacyStream(t *testing.T) {
	runPortedLegacy(t, portedStreamSDL,
		func(*testing.T) any { return map[string]any{} }, []portedIncrementalCase{
			{
				name:  "Can stream a list field",
				query: "{ scalarList @stream(initialCount: 1) }",
				root:  "{\"scalarList\": [\"apple\", \"banana\", \"coconut\"]}",
				want:  "[{\"data\": {\"scalarList\": [\"apple\"]}, \"hasNext\": true}, {\"incremental\": [{\"items\": [\"banana\", \"coconut\"], \"path\": [\"scalarList\", 1]}], \"hasNext\": false}]",
			},
			{
				name:  "Can use default value of initialCount",
				query: "{ scalarList @stream }",
				root:  "{\"scalarList\": [\"apple\", \"banana\", \"coconut\"]}",
				want:  "[{\"data\": {\"scalarList\": []}, \"hasNext\": true}, {\"incremental\": [{\"items\": [\"apple\", \"banana\", \"coconut\"], \"path\": [\"scalarList\", 0]}], \"hasNext\": false}]",
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
				want:  "[{\"data\": {\"scalarList\": [\"apple\"]}, \"hasNext\": true}, {\"incremental\": [{\"items\": [\"banana\", \"coconut\"], \"path\": [\"scalarList\", 1], \"label\": \"scalar-stream\"}], \"hasNext\": false}]",
			},
			{
				name:  "Treats null stream label the same as no label",
				query: "{ scalarList @stream(initialCount: 1, label: null) }",
				root:  "{\"scalarList\": [\"apple\", \"banana\", \"coconut\"]}",
				want:  "[{\"data\": {\"scalarList\": [\"apple\"]}, \"hasNext\": true}, {\"incremental\": [{\"items\": [\"banana\", \"coconut\"], \"path\": [\"scalarList\", 1]}], \"hasNext\": false}]",
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
				want:  "[{\"data\": {\"scalarList\": [\"apple\", \"banana\"]}, \"hasNext\": true}, {\"incremental\": [{\"items\": [\"coconut\"], \"path\": [\"scalarList\", 2]}], \"hasNext\": false}]",
			},
			{
				name:  "Can stream multi-dimensional lists",
				query: "{ scalarListList @stream(initialCount: 1) }",
				root:  "{\"scalarListList\": [[\"apple\", \"apple\", \"apple\"], [\"banana\", \"banana\", \"banana\"], [\"coconut\", \"coconut\", \"coconut\"]]}",
				want:  "[{\"data\": {\"scalarListList\": [[\"apple\", \"apple\", \"apple\"]]}, \"hasNext\": true}, {\"incremental\": [{\"items\": [[\"banana\", \"banana\", \"banana\"], [\"coconut\", \"coconut\", \"coconut\"]], \"path\": [\"scalarListList\", 1]}], \"hasNext\": false}]",
			},
		})
}
