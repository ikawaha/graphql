package execution_test

// The harness the ported legacy @defer and @stream cases run against.
//
// graphql-js keeps the older payload format alongside the current one, with
// its own copies of the defer and stream suites. The schema and the root value
// are the same as the current suites use; only the shape of the answer differs.

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/execution"
)

// knownLegacyDivergences are the cases this implementation does not match, and
// why. Each is asserted to *still* differ, so that closing one cannot go
// unnoticed.
var knownLegacyDivergences = map[string]string{}

func runPortedLegacy(t *testing.T, sdl string, defaultRoot func(*testing.T) any, cases []portedIncrementalCase) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			s := buildSchema(t, sdl)
			for _, coordinate := range tt.failing {
				failField(t, s, coordinate)
			}
			root := defaultRoot(t)
			if tt.root != "" {
				decoder := json.NewDecoder(strings.NewReader(tt.root))
				decoder.UseNumber()
				if err := decoder.Decode(&root); err != nil {
					t.Fatalf("reading the root value: %v", err)
				}
			}

			result := execution.ExecuteLegacyIncrementally(context.Background(), execution.Request{
				Schema:    s,
				Document:  mustParse(t, tt.query),
				RootValue: root,
			})

			var got any
			if result.Subsequent == nil {
				got = decodeJSON(t, mustMarshal(t, result.Initial))
			} else {
				responses := []any{decodeJSON(t, mustMarshal(t, result.Initial))}
				for payload := range result.Subsequent {
					responses = append(responses, decodeJSON(t, mustMarshal(t, payload)))
				}
				got = responses
			}

			want := decodeJSON(t, tt.want)
			if why, listed := knownLegacyDivergences[tt.name]; listed {
				if reflect.DeepEqual(got, want) {
					t.Errorf("this case now matches graphql-js; remove it from the known divergences (%s)", why)
				} else {
					t.Logf("known divergence: %s", why)
				}
				return
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("run =\n%s\nwant\n%s", mustMarshal(t, got), tt.want)
			}
		})
	}
}
