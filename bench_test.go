package graphql_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ikawaha/graphql"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/utilities"
)

// The end-to-end cost of answering a request: parsing it, checking it, and
// running it. This is the number a server actually pays, and the baseline
// later work is measured against.
func BenchmarkDo(b *testing.B) {
	s, err := graphql.BuildSchema(starWarsSDL)
	if err != nil {
		b.Fatal(err)
	}
	const query = `
		query Hero($episode: Episode) {
			hero(episode: $episode) {
				name
				friends { name appearsIn }
				... on Droid { primaryFunction }
			}
		}
	`
	variables := graphql.Variables(map[string]any{"episode": "EMPIRE"})
	ctx := context.Background()

	b.Run("parse, check and run", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			graphql.Do(ctx, graphql.Params{Schema: s, Query: query, Variables: variables})
		}
	})

	// A server with persisted queries parses once. This is what the rest of a
	// request costs once that is out of the way.
	b.Run("check and run", func(b *testing.B) {
		doc, err := language.ParseString(query)
		if err != nil {
			b.Fatal(err)
		}
		b.ReportAllocs()
		for b.Loop() {
			graphql.Do(ctx, graphql.Params{Schema: s, Document: doc, Variables: variables})
		}
	})

	// And what running alone costs, for a document already known to be sound.
	b.Run("run", func(b *testing.B) {
		doc, err := language.ParseString(query)
		if err != nil {
			b.Fatal(err)
		}
		b.ReportAllocs()
		for b.Loop() {
			graphql.Do(ctx, graphql.Params{
				Schema: s, Document: doc, Variables: variables, SkipValidation: true,
			})
		}
	})
}

// Building a schema from a real one, which a server does once at startup.
func BenchmarkBuildSchema_GitHub(b *testing.B) {
	body, err := os.ReadFile(filepath.Join("language", "testdata", "github-schema.graphql"))
	if err != nil {
		b.Fatal(err)
	}
	sdl := string(body)
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := utilities.BuildSchema(sdl); err != nil {
			b.Fatal(err)
		}
	}
}
