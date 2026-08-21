package validation_test

import (
	"strings"
	"testing"

	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
	"github.com/ikawaha/graphql/validation"
)

// The tests in the ported_*_test.go files are graphql-js's own, taken from
// src/validation/__tests__ (MIT, Copyright (c) GraphQL Contributors; see the
// NOTICE file). They run against graphql-js's own harness schema, so that a
// failure is about the rule rather than about the schema.
//
// What is asserted is how many errors a document produces and where each
// points, which is what decision 13 settled on: matching graphql-js message
// for message would mean carrying several thousand lines of English that catch
// nothing the structure does not.

// portedStep is one document a case checks.
type portedStep struct {
	query string
	// sdl says the document is type definitions rather than an operation.
	sdl bool
	// againstOwnSchema says to check it against the schema the case built for
	// itself rather than against the shared harness.
	againstOwnSchema bool
	want             []want
}

// portedCase is one of graphql-js's test cases.
type portedCase struct {
	name string
	// ownSchema is SDL the case builds a schema from, where the shared harness
	// is not what it is about.
	ownSchema string
	// extendHarness is SDL added to the shared harness, which is how a case
	// that needs a directive or a type the harness lacks gets one.
	extendHarness string
	steps         []portedStep
}

// knownDivergences are the ported cases this implementation does not yet
// match, and why.
//
// They are listed rather than left out, and each is asserted to *still*
// diverge: if one starts matching, the test says so and the entry should go.
// That way a gap cannot be closed by accident and then quietly reopened.
// knownDivergences are the ported cases this implementation does not yet
// match, and why.
//
// They are listed rather than left out, and each is asserted to *still*
// diverge: if one starts matching, the test says so and the entry should go.
// That way a gap cannot be closed by accident and then quietly reopened.
//
// The seven below are **bugs, not decisions**: graphql-js is right and this
// implementation is wrong. They are listed so that the rest of the rule's 77
// cases can guard against regression while these are fixed.
// It is empty, and the machinery is kept for the next one.
var knownDivergences = map[string]string{}

// runPorted runs the cases against a rule.
func runPorted(t *testing.T, rule validation.Rule, cases []portedCase) {
	t.Helper()
	shared := upstreamHarness(t)
	ruleName := ruleNameOf(t)
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			// The schema is worked out first, so that a case listed as not yet
			// matching is checked against the same one it would run against.
			s := shared
			switch {
			case tt.ownSchema != "":
				built, err := utilities.BuildSchema(tt.ownSchema)
				if err != nil {
					t.Fatalf("building the schema the case supplies: %v", err)
				}
				s = built
			case tt.extendHarness != "":
				extended, err := utilities.ExtendSchemaSource(shared, tt.extendHarness)
				if err != nil {
					t.Fatalf("extending the harness as the case asks: %v", err)
				}
				s = extended
			}

			if why, known := knownDivergences[ruleName+"/"+tt.name]; known {
				assertStillDiverges(t, rule, s, tt, why)
				return
			}
			for _, step := range tt.steps {
				switch {
				case step.sdl && step.againstOwnSchema:
					expectSDLErrorsAsWritten(t, s, rule, step.query, step.want...)
				case step.sdl:
					expectSDLErrorsAsWritten(t, nil, rule, step.query, step.want...)
				default:
					expectErrorsAsWritten(t, s, rule, step.query, step.want...)
				}
			}
		})
	}
}

// ruleNameOf reads the rule from the test's own name, so that a divergence can
// be listed as "Rule/case" and read as one.
func ruleNameOf(t *testing.T) string {
	name := t.Name()
	if at := strings.Index(name, "TestPorted_"); at >= 0 {
		name = name[at+len("TestPorted_"):]
	}
	if at := strings.Index(name, "/"); at >= 0 {
		name = name[:at]
	}
	return name
}

// assertStillDiverges checks that a case listed as not yet matching still does
// not match, and says so if it has started to.
func assertStillDiverges(t *testing.T, rule validation.Rule, s *schema.Schema, tt portedCase, why string) {
	t.Helper()
	matches := true
	for _, step := range tt.steps {
		doc, err := language.ParseString(step.query, language.ExperimentalFragmentArguments())
		if err != nil {
			t.Fatalf("parsing: %v\n%s", err, step.query)
		}
		var errs []*gqlerror.Error
		if step.sdl {
			errs = validation.ValidateSDL(doc, nil, rule)
		} else {
			errs = validation.Validate(s, doc, rule)
		}
		if !sameShape(errs, step.want) {
			matches = false
		}
	}
	if matches {
		t.Errorf("this case now matches graphql-js; remove it from knownDivergences (%s)", why)
	} else {
		t.Logf("known divergence: %s", why)
	}
}

// sameShape reports whether what was reported has the shape the case expects:
// as many errors, each pointing at the same places.
func sameShape(got []*gqlerror.Error, want []want) bool {
	if len(got) != len(want) {
		return false
	}
	for i, w := range want {
		if w.At == nil {
			continue
		}
		if len(got[i].Locations) != len(w.At) {
			return false
		}
		for j, a := range w.At {
			if got[i].Locations[j].Line != a.line || got[i].Locations[j].Column != a.column {
				return false
			}
		}
	}
	return true
}
