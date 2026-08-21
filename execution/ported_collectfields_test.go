package execution_test

// Ported from graphql-js src/execution/__tests__/collectFields-test.ts, which
// is about what happens when the same fragment is spread twice and only one of
// the spreads is deferred.

import (
	"github.com/ikawaha/graphql/schema"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/language"
)

func TestPortedCollectFields(t *testing.T) {
	s := buildPorted(t, `
  type Query {
    field: String
  }`)

	tests := []struct {
		name string
		// query is the document, written as graphql-js wrote it.
		query string
		// deferrals is how many deferred fragments the walk should have met.
		deferrals int
		// selections is how many times the response key `field` should have
		// been asked for.
		selections int
	}{
		{
			name: `should not collect a deferred spread after a non-deferred spread has been collected`,
			query: `
        query {
          ...FragmentName
          ...FragmentName @defer
        }
        fragment FragmentName on Query {
          field
        }
      `,
			deferrals: 0, selections: 1,
		},
		{
			name: `should not collect a deferred spread after a deferred spread has been collected`,
			query: `
        query {
          ...FragmentName @defer
          ...FragmentName @defer
        }
        fragment FragmentName on Query {
          field
        }
      `,
			deferrals: 1, selections: 1,
		},
		{
			name: `should collect a non-deferred spread after a deferred spread has been collected`,
			query: `
        query {
          ...FragmentName @defer
          ...FragmentName
        }
        fragment FragmentName on Query {
          field
        }
      `,
			deferrals: 1, selections: 2,
		},
		{
			name: `should not collect a non-deferred spread after a non-deferred spread has been collected`,
			query: `
        query {
          ...FragmentName
          ...FragmentName
        }
        fragment FragmentName on Query {
          field
        }
      `,
			deferrals: 0, selections: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operation, fragments := operationAndFragments(t, tt.query)
			fields, deferrals := execution.CollectFieldsIncrementally(
				s, fragments, schema.VariableValues{}, s.QueryType(), operation.SelectionSet, true)

			if len(deferrals) != tt.deferrals {
				t.Errorf("%d deferred fragments, want %d", len(deferrals), tt.deferrals)
			}
			if got := len(fields.Fields("field")); got != tt.selections {
				t.Errorf("field was asked for %d times, want %d", got, tt.selections)
			}
		})
	}
}

// operationAndFragments reads the one operation and the fragments out of a
// document, which is what collecting fields needs.
func operationAndFragments(
	t *testing.T,
	query string,
) (*language.OperationDefinition, map[string]*language.FragmentDefinition) {
	t.Helper()
	var operation *language.OperationDefinition
	fragments := map[string]*language.FragmentDefinition{}
	for _, def := range mustParse(t, query).Definitions {
		switch typed := def.(type) {
		case *language.OperationDefinition:
			operation = typed
		case *language.FragmentDefinition:
			fragments[typed.Name.Value] = typed
		}
	}
	if operation == nil {
		t.Fatal("the document has no operation")
	}
	return operation, fragments
}
