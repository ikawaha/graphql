package graphql_test

// Concurrency invariance.
//
// The recording under testdata/differential says what graphql-js answered, and
// the tests beside this one check that this implementation answers the same —
// running its fields one after another, since that is the default. Nothing
// there ever sets Params.Concurrency, so the executor's parallel path is
// compared with nothing at all.
//
// These run the same corpora again with the fields of an object, and the
// entries of a list, worked on alongside one another. The answer has to be the
// one already known to match graphql-js, byte for byte: a response keys its
// fields in the order the document wrote them however they were resolved, a
// list keeps its order, and which of several failures nulls an object is
// decided by the document's order too, not by which goroutine finished first.
//
// Run these under -race for what they are worth.

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ikawaha/graphql"
	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

// concurrencies are the widths every case is run at. One is the answer the
// recording was compared against; the rest have to agree with it.
var concurrencies = []int{1, 2, 4, 8}

// sameAcrossWidths runs one case at each width and reports where the answers
// part company.
func sameAcrossWidths(t *testing.T, answer func(concurrency int) string) {
	t.Helper()
	var first string
	for i, width := range concurrencies {
		got := answer(width)
		if i == 0 {
			first = got
			continue
		}
		if got != first {
			t.Errorf("at concurrency %d the answer differs%s", width, sideBySide(got, first))
			return
		}
	}
}

func TestConcurrency_Executions(t *testing.T) {
	runExecutionsConcurrently(t, "executions")
}

func TestConcurrency_Coercions(t *testing.T) {
	runExecutionsConcurrently(t, "coercions")
}

func runExecutionsConcurrently(t *testing.T, corpus string) {
	t.Helper()
	cases, _ := read[executionCase, executionAnswer](t, corpus)
	for _, tt := range cases {
		t.Run(tt.Name, func(t *testing.T) {
			built, err := utilities.BuildSchema(tt.SDL)
			if err != nil {
				t.Skipf("the schema does not build: %v", err)
			}
			root := decodeRoot(t, tt.Root)
			sameAcrossWidths(t, func(concurrency int) string {
				params := graphql.Params{
					Schema:        built,
					Query:         tt.Query,
					OperationName: tt.OperationName,
					Variables:     tt.Variables,
					RootValue:     root,
					Concurrency:   concurrency,
				}
				if tt.EchoArgs {
					params.FieldResolver = echoArgs
				}
				return response(t, graphql.Do(context.Background(), params))
			})
		})
	}
}

func TestConcurrency_Incremental(t *testing.T) {
	cases, built := incrementalCorpus(t, "incremental")
	for _, tt := range cases {
		t.Run(tt.Name, func(t *testing.T) {
			root := decodeRoot(t, tt.Root)
			sameAcrossWidths(t, func(concurrency int) string {
				got := graphql.DoIncrementally(context.Background(), graphql.Params{
					Schema: built, Query: tt.Query, RootValue: root,
					FieldResolver: failing, SkipValidation: true,
					Concurrency: concurrency,
				})
				payloads := []any{got.Initial}
				if got.Subsequent != nil {
					for part := range got.Subsequent {
						payloads = append(payloads, part)
					}
				}
				return sequence(t, payloads)
			})
		})
	}
}

func TestConcurrency_LegacyIncremental(t *testing.T) {
	// Both formats are driven from the one corpus; only the recording differs.
	cases, built := incrementalCorpus(t, "incremental")
	for _, tt := range cases {
		t.Run(tt.Name, func(t *testing.T) {
			root := decodeRoot(t, tt.Root)
			sameAcrossWidths(t, func(concurrency int) string {
				got := graphql.DoLegacyIncrementally(context.Background(), graphql.Params{
					Schema: built, Query: tt.Query, RootValue: root,
					FieldResolver: failing, SkipValidation: true,
					Concurrency: concurrency,
				})
				payloads := []any{got.Initial}
				if got.Subsequent != nil {
					for part := range got.Subsequent {
						payloads = append(payloads, part)
					}
				}
				return sequence(t, payloads)
			})
		})
	}
}

func TestConcurrency_Subscriptions(t *testing.T) {
	cases, _ := read[subscriptionCase, incrementalAnswer](t, "subscriptions")
	sdl, err := os.ReadFile(filepath.Join(fixtures, "corpus", "subscription.graphql"))
	if err != nil {
		t.Fatalf("reading the schema: %v", err)
	}
	built, err := utilities.BuildSchema(string(sdl))
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}

	for _, tt := range cases {
		t.Run(tt.Name, func(t *testing.T) {
			doc, err := language.ParseString(tt.Query)
			if err != nil {
				t.Skipf("the document does not parse: %v", err)
			}
			sameAcrossWidths(t, func(concurrency int) string {
				// A fresh channel each time: the events are read once.
				events := make(chan any, len(tt.Events))
				for _, raw := range tt.Events {
					var event any
					decoder := json.NewDecoder(bytes.NewReader(raw))
					decoder.UseNumber()
					if err := decoder.Decode(&event); err != nil {
						t.Fatalf("reading an event: %v", err)
					}
					events <- event
				}
				close(events)

				got := execution.Subscribe(context.Background(), execution.Request{
					Schema:      built,
					Document:    doc,
					RootValue:   map[string]any{tt.Field: (<-chan any)(events)},
					Concurrency: concurrency,
				})
				if len(got.Errors) > 0 {
					return "could not start: " + got.Errors[0].Message
				}
				payloads := []any{}
				for payload := range got.Events {
					payloads = append(payloads, payload)
				}
				return sequence(t, payloads)
			})
		})
	}
}

// TestConcurrency_Introspection covers the biggest single response there is,
// which is the one most likely to show a merge going wrong.
func TestConcurrency_Introspection(t *testing.T) {
	sdl, err := os.ReadFile(filepath.Join(fixtures, "corpus", "subscription.graphql"))
	if err != nil {
		t.Fatalf("reading the schema: %v", err)
	}
	built, err := utilities.BuildSchema(string(sdl))
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	query := utilities.IntrospectionQuery(utilities.WithEverything())
	sameAcrossWidths(t, func(concurrency int) string {
		return response(t, graphql.Do(context.Background(), graphql.Params{
			Schema: built, Query: query, Concurrency: concurrency,
		}))
	})
}

// decodeRoot reads a corpus root value, keeping numbers as they were written.
func decodeRoot(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	if len(raw) == 0 {
		return nil
	}
	var root any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		t.Fatalf("reading the root value: %v", err)
	}
	return root
}

// incrementalCorpus reads one of the two incremental corpora and the schema
// they are both written against.
func incrementalCorpus(t *testing.T, corpus string) ([]incrementalCase, *schema.Schema) {
	t.Helper()
	cases, _ := read[incrementalCase, incrementalAnswer](t, corpus)
	sdl, err := os.ReadFile(filepath.Join(fixtures, "corpus", "incremental.graphql"))
	if err != nil {
		t.Fatalf("reading the schema: %v", err)
	}
	built, err := utilities.BuildSchema(string(sdl))
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	return cases, built
}
