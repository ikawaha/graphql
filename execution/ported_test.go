package execution_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
	"github.com/ikawaha/graphql/value"
)

// The ported_*_test.go files hold graphql-js's own execution tests, taken from
// src/execution/__tests__ (MIT, Copyright (c) GraphQL Contributors; see the
// NOTICE file). Each file brings the schema its cases run against.
//
// What is compared is the whole response, message for message. Elsewhere the
// ported tests compare only structure, on the grounds that matching several
// thousand lines of English catches nothing that structure does not; here the
// messages are the greater part of what the cases are about, so they are
// compared too, and every difference is listed as a known divergence.

// portedCase is one of graphql-js's cases: a document, the variables a request
// supplied, and the whole response expected back.
type portedCase struct {
	name string
	// query is the document, written as graphql-js wrote it, so that the
	// places an error reports are the ones graphql-js reported.
	query string
	// variables is the request's variables as JSON, or empty where the request
	// supplied none. A variable written as "@@undefined" was named and then
	// explicitly not given a value, which graphql-js spells `undefined`.
	variables string
	// root is the value the operation's own fields are read from, where a case
	// brings one of its own.
	root any
	// sdl is a schema of the case's own, where the file's schema is not what
	// the case is about.
	sdl string
	// built is a schema of the case's own that had to be assembled in Go
	// rather than written as SDL, because of a resolver or an isTypeOf.
	built *schema.Schema
	// operation names which operation to run, where the document holds more
	// than one.
	operation string
	// fieldResolver and typeResolver are the request's own, for the cases
	// about supplying them.
	fieldResolver schema.FieldResolver
	typeResolver  schema.TypeResolver
	// want is the whole response, as JSON.
	want string
}

// runPorted runs a file's cases against the schema it brought.
//
// known lists the cases this implementation does not match, and why. Each is
// asserted to *still* differ, so that closing one cannot go unnoticed.
func runPorted(
	t *testing.T,
	s *schema.Schema,
	root any,
	known map[string]string,
	cases []portedCase,
) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			from := root
			if tt.root != nil {
				from = tt.root
			}
			against := s
			if tt.built != nil {
				against = tt.built
			}
			if tt.sdl != "" {
				built, err := utilities.BuildSchema(tt.sdl)
				if err != nil {
					t.Fatalf("building the schema: %v", err)
				}
				against = built
			}
			result := execution.Execute(context.Background(), execution.Request{
				Schema:        against,
				Document:      mustParse(t, tt.query),
				OperationName: tt.operation,
				RootValue:     from,
				Variables:     portedVariables(t, tt.variables),
				FieldResolver: tt.fieldResolver,
				TypeResolver:  tt.typeResolver,
			})
			got := decodeJSON(t, mustMarshal(t, result))
			want := decodeJSON(t, tt.want)

			if why, listed := known[tt.name]; listed {
				if reflect.DeepEqual(got, want) {
					t.Errorf("this case now matches graphql-js; remove it from the known divergences (%s)", why)
				} else {
					t.Logf("known divergence: %s", why)
				}
				return
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("response =\n%s\nwant\n%s", mustMarshal(t, result), tt.want)
			}
		})
	}
}

// portedVariables reads a case's variables, turning the marker graphql-js
// writes as `undefined` into a variable that was named and then not given a
// value.
func portedVariables(t *testing.T, encoded string) map[string]value.Maybe[any] {
	t.Helper()
	if encoded == "" {
		return nil
	}
	var supplied map[string]any
	decoder := json.NewDecoder(strings.NewReader(encoded))
	decoder.UseNumber()
	if err := decoder.Decode(&supplied); err != nil {
		t.Fatalf("reading the variables: %v", err)
	}
	variables := make(map[string]value.Maybe[any], len(supplied))
	for name, held := range supplied {
		if held == "@@undefined" {
			variables[name] = value.Nothing[any]()
			continue
		}
		variables[name] = value.Just(withoutUnsupplied(held))
	}
	return variables
}

// withoutUnsupplied drops the keys inside a variable that were named and given
// no value. graphql-js writes one as `undefined`, which is a key that is there
// holding nothing; here it is a key that is not there.
func withoutUnsupplied(held any) any {
	switch typed := held.(type) {
	case map[string]any:
		kept := make(map[string]any, len(typed))
		for name, inner := range typed {
			if inner == "@@undefined" {
				continue
			}
			kept[name] = withoutUnsupplied(inner)
		}
		return kept
	case []any:
		kept := make([]any, len(typed))
		for i, inner := range typed {
			kept[i] = withoutUnsupplied(inner)
		}
		return kept
	default:
		return held
	}
}

func decodeJSON(t *testing.T, encoded string) any {
	t.Helper()
	var out any
	decoder := json.NewDecoder(strings.NewReader(encoded))
	decoder.UseNumber()
	if err := decoder.Decode(&out); err != nil {
		t.Fatalf("reading %s: %v", encoded, err)
	}
	return out
}

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	encoded, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("writing the response: %v", err)
	}
	return string(encoded)
}

// mustQuoteJSON renders a string as a JSON string, for building an expected
// response around a message with quotes in it.
func mustQuoteJSON(t *testing.T, text string) string {
	t.Helper()
	encoded, err := json.Marshal(text)
	if err != nil {
		t.Fatalf("quoting %q: %v", text, err)
	}
	return string(encoded)
}

// equalJSON compares two decoded responses.
func equalJSON(got, want any) bool { return reflect.DeepEqual(got, want) }
