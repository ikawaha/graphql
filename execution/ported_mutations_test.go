package execution_test

// Ported from graphql-js src/execution/__tests__/mutations-test.ts. The cases
// about @defer are left out: they turn on a mutation not waiting for a
// deferred payload, and incremental delivery is answered by
// ExecuteIncrementally, which has tests of its own.
//
// The promise-returning mutations of the original are ordinary resolvers here,
// so the two halves of each query do the same thing; what the cases are really
// about — that one mutation finishes before the next begins — is unchanged.

import (
	"context"
	"errors"
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

func TestPortedMutations(t *testing.T) {
	runPorted(t, testMutationsSchema(t), nil, nil, []portedCase{
		{
			name: `evaluates mutations serially`,
			query: `
      mutation M {
        first: immediatelyChangeTheNumber(newNumber: 1) {
          theNumber
        },
        second: promiseToChangeTheNumber(newNumber: 2) {
          theNumber
        },
        third: immediatelyChangeTheNumber(newNumber: 3) {
          theNumber
        }
        fourth: promiseToChangeTheNumber(newNumber: 4) {
          theNumber
        },
        fifth: immediatelyChangeTheNumber(newNumber: 5) {
          theNumber
        }
      }
    `,
			want: `{"data": {
				"first": {"theNumber": 1}, "second": {"theNumber": 2}, "third": {"theNumber": 3},
				"fourth": {"theNumber": 4}, "fifth": {"theNumber": 5}}}`,
		},
		{
			name:  `does not include illegal mutation fields in output`,
			query: `mutation { thisIsIllegalDoNotIncludeMe }`,
			want:  `{"data": {}}`,
		},
		{
			name: `evaluates mutations correctly in the presence of a failed mutation`,
			query: `
      mutation M {
        first: immediatelyChangeTheNumber(newNumber: 1) {
          theNumber
        },
        second: promiseToChangeTheNumber(newNumber: 2) {
          theNumber
        },
        third: failToChangeTheNumber(newNumber: 3) {
          theNumber
        }
        fourth: promiseToChangeTheNumber(newNumber: 4) {
          theNumber
        },
        fifth: immediatelyChangeTheNumber(newNumber: 5) {
          theNumber
        }
        sixth: promiseAndFailToChangeTheNumber(newNumber: 6) {
          theNumber
        }
      }
    `,
			want: `{"data": {
				"first": {"theNumber": 1}, "second": {"theNumber": 2}, "third": null,
				"fourth": {"theNumber": 4}, "fifth": {"theNumber": 5}, "sixth": null},
				"errors": [
					{"message": "Cannot change the number",
					 "locations": [{"line": 9, "column": 9}], "path": ["third"]},
					{"message": "Cannot change the number",
					 "locations": [{"line": 18, "column": 9}], "path": ["sixth"]}]}`,
		},
		{
			name: `mutation inside of a fragment`,
			query: `
      mutation M {
        second: immediatelyChangeTheNumber(newNumber: 2) {
          theNumber
        }
        ...MutationFragment
      }
      fragment MutationFragment on Mutation {
        first: promiseToChangeTheNumber(newNumber: 1) {
          theNumber
        },
      }
    `,
			want: `{"data": {"second": {"theNumber": 2}, "first": {"theNumber": 1}}}`,
		},
	})
}

// numberHolder is graphql-js's own root: a number that each mutation changes,
// so that a mutation seeing the wrong number means they did not run in order.
type numberHolder struct{ TheNumber int }

// testMutationsSchema is graphql-js's own schema from mutations-test.ts.
func testMutationsSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := utilities.BuildSchema(`
		type NumberHolder {
			theNumber: Int
			promiseToGetTheNumber: Int
		}

		type Query {
			numberHolder: NumberHolder
		}

		type Mutation {
			immediatelyChangeTheNumber(newNumber: Int): NumberHolder
			promiseToChangeTheNumber(newNumber: Int): NumberHolder
			failToChangeTheNumber(newNumber: Int): NumberHolder
			promiseAndFailToChangeTheNumber(newNumber: Int): NumberHolder
		}
	`)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}

	held := &numberHolder{}
	change := func(_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo) (any, error) {
		given, _ := args.Get("newNumber")
		held.TheNumber = int(given.(int32))
		return held, nil
	}
	fail := func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		return nil, errors.New("Cannot change the number")
	}
	mutation := s.MutationType()
	mutation.Field("immediatelyChangeTheNumber").Resolve = change
	mutation.Field("promiseToChangeTheNumber").Resolve = change
	mutation.Field("failToChangeTheNumber").Resolve = fail
	mutation.Field("promiseAndFailToChangeTheNumber").Resolve = fail
	s.QueryType().Field("numberHolder").Resolve = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		return held, nil
	}
	return s
}
