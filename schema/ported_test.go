package schema_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

// The tests in ported_*_test.go are graphql-js's own, taken from
// src/type/__tests__ (MIT, Copyright (c) GraphQL Contributors; see the NOTICE
// file).
//
// What is asserted is how many things a schema is wrong about and where each
// points, which is what decision 13 settled on.

// at is a place in the SDL an error is expected to point at.
type at struct {
	line, column int
}

// schemaWant describes one thing a schema is expected to be wrong about.
type schemaWant struct {
	At []at
}

// portedSchemaCase is one of graphql-js's schema validation cases.
type portedSchemaCase struct {
	name string
	sdl  string
	want []schemaWant
	// mangle spoils the built schema before it is checked, which is how
	// graphql-js's own cases reach a state its parser cannot produce: they
	// build a schema and then take part of its AST away.
	mangle func(*schema.Schema)
}

// knownSchemaGaps are the ported cases this implementation does not yet match,
// and why. Each is asserted to *still* differ, so a gap cannot be closed by
// accident and then quietly reopened.
var knownSchemaGaps = map[string]string{}

// runPortedSchemaCases builds each schema and compares what the validator says
// about it.
func runPortedSchemaCases(t *testing.T, cases []portedSchemaCase) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			// The SDL is parsed as written: the cases record the line and
			// column their own parse produced.
			why, known := knownSchemaGaps[tt.name]
			s, err := utilities.BuildSchema(tt.sdl)
			if err != nil {
				// A schema Go cannot hold is refused rather than built and
				// then reported on, which is one of the recorded gaps.
				if known {
					t.Logf("known gap: %s", why)
					return
				}
				t.Fatalf("building the schema: %v\n%s", err, tt.sdl)
			}
			if tt.mangle != nil {
				tt.mangle(s)
			}
			got := schema.ValidateSchema(s)

			if known {
				if len(got) == len(tt.want) {
					t.Errorf("this case now matches graphql-js; remove it from knownSchemaGaps (%s)", why)
				} else {
					t.Logf("known gap: %s", why)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("%d errors, want %d:\n%s\nreported:\n%s",
					len(got), len(tt.want), tt.sdl, describeSchemaErrors(got))
			}
			for i, w := range tt.want {
				// A case that records no expected place says nothing about
				// where the error should point, so there is nothing to check.
				if w.At == nil {
					continue
				}
				if len(got[i].Locations) != len(w.At) {
					t.Errorf("error %d (%s): points at %v, want %d place(s)",
						i, got[i].Message, formatLocations(got[i].Locations), len(w.At))
					continue
				}
				for j, a := range w.At {
					if got[i].Locations[j].Line != a.line || got[i].Locations[j].Column != a.column {
						t.Errorf("error %d (%s): location %d = %d:%d, want %d:%d",
							i, got[i].Message, j,
							got[i].Locations[j].Line, got[i].Locations[j].Column, a.line, a.column)
					}
				}
			}
		})
	}
}

func describeSchemaErrors(errs []*gqlerror.Error) string {
	if len(errs) == 0 {
		return "  (nothing)"
	}
	var b strings.Builder
	for _, e := range errs {
		fmt.Fprintf(&b, "  %s %s\n", formatLocations(e.Locations), e.Message)
	}
	return strings.TrimRight(b.String(), "\n")
}

func formatLocations(locs []language.SourceLocation) string {
	if len(locs) == 0 {
		return "(nowhere)"
	}
	parts := make([]string, len(locs))
	for i, loc := range locs {
		parts[i] = fmt.Sprintf("%d:%d", loc.Line, loc.Column)
	}
	return "[" + strings.Join(parts, " ") + "]"
}
