package execution_test

// Ported from graphql-js src/execution/__tests__/nonnull-test.ts. Only the
// cases that read a root value are here: the rest turn on when a promise
// settles, which is not a question Go asks.

import (
	"errors"
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

func TestPortedNonNull(t *testing.T) {
	runPorted(t, testNonNullSchema(t), nil, knownNonNullDivergences, []portedCase{
		{
			name: `that returns null`,
			query: `
      {
        sync
      }
    `,
			root: nullingData,
			want: `{"data": {"sync": null}}`,
		},
		{
			name: `that throws`,
			query: `
      {
        sync
      }
    `,
			root: throwingData,
			want: `{"data": {"sync": null}, "errors": [{"message": "sync", "path": ["sync"], "locations": [{"line": 3, "column": 9}]}]}`,
		},
		{
			name: `that returns null (2)`,
			query: `
      {
        syncNest {
          syncNonNull,
        }
      }
    `,
			root: nullingData,
			want: `{"data": {"syncNest": null}, "errors": [{"message": "Cannot return null for non-nullable field DataType.syncNonNull.", "path": ["syncNest", "syncNonNull"], "locations": [{"line": 4, "column": 11}]}]}`,
		},
		{
			name: `that throws (2)`,
			query: `
      {
        syncNest {
          syncNonNull,
        }
      }
    `,
			root: throwingData,
			want: `{"data": {"syncNest": null}, "errors": [{"message": "syncNonNull", "path": ["syncNest", "syncNonNull"], "locations": [{"line": 4, "column": 11}]}]}`,
		},
		{
			name: `that returns null (3)`,
			query: `
      {
        syncNonNull
      }
    `,
			root: nullingData,
			want: `{"data": null, "errors": [{"message": "Cannot return null for non-nullable field DataType.syncNonNull.", "path": ["syncNonNull"], "locations": [{"line": 3, "column": 9}]}]}`,
		},
		{
			name: `that throws (3)`,
			query: `
      {
        syncNonNull
      }
    `,
			root: throwingData,
			want: `{"data": null, "errors": [{"message": "syncNonNull", "path": ["syncNonNull"], "locations": [{"line": 3, "column": 9}]}]}`,
		},
		{
			name: `nullable and non-nullable root fields throw nested errors`,
			query: `
        {
          promiseNonNullNest {
            syncNonNull
          }
          promiseNest {
            syncNonNull
          }
        }
      `,
			root: throwingData,
			want: `{"data": null, "errors": [{"message": "syncNonNull", "path": ["promiseNest", "syncNonNull"], "locations": [{"line": 7, "column": 13}]}, {"message": "syncNonNull", "path": ["promiseNonNullNest", "syncNonNull"], "locations": [{"line": 4, "column": 13}]}]}`,
		},
		{
			name: `a nullable root field throws a slower nested error after a non-nullable root field throws a nested error`,
			query: `
        {
          promiseNonNullNest {
            syncNonNull
          }
          promiseNest {
            promiseNonNull
          }
        }
      `,
			root: throwingData,
			want: `{"data": null, "errors": [{"message": "syncNonNull", "path": ["promiseNonNullNest", "syncNonNull"], "locations": [{"line": 4, "column": 13}]}]}`,
		},
		{
			name: `nullable and non-nullable nested fields throw nested errors`,
			query: `
        {
          syncNest {
            promiseNonNullNest {
              syncNonNull
            }
            promiseNest {
              syncNonNull
            }
          }
        }
      `,
			root: throwingData,
			want: `{"data": {"syncNest": null}, "errors": [{"message": "syncNonNull", "path": ["syncNest", "promiseNest", "syncNonNull"], "locations": [{"line": 8, "column": 15}]}, {"message": "syncNonNull", "path": ["syncNest", "promiseNonNullNest", "syncNonNull"], "locations": [{"line": 5, "column": 15}]}]}`,
		},
		{
			name: `a nullable nested field throws a slower nested error after a non-nullable nested field throws a nested error`,
			query: `
        {
          syncNest {
            promiseNonNullNest {
              syncNonNull
            }
            promiseNest {
              promiseNest {
                promiseNest {
                  promiseNonNull
                }
              }
            }
          }
        }
      `,
			root: throwingData,
			want: `{"data": {"syncNest": null}, "errors": [{"message": "syncNonNull", "path": ["syncNest", "promiseNonNullNest", "syncNonNull"], "locations": [{"line": 5, "column": 15}]}]}`,
		},
	})
}

// Not ported, because each of these is written in a way this could not
// follow:
//   - suppresses a later error after a parent has been nulled: its root value is written as JavaScript

// knownNonNullDivergences are the cases this implementation does not match,
// and why. Each is asserted to *still* diverge, so that closing one cannot go
// unnoticed.
var knownNonNullDivergences = map[string]string{
	// graphql-js loses the slower of the two errors: by the time it arrives
	// the object it belongs to has already been nulled, and there is nowhere
	// left to put it. Nothing here is slower than anything else, so both
	// errors are reported.
	"a nullable root field throws a slower nested error after a non-nullable root field throws a nested error":     "there is no slower error in Go",
	"a nullable nested field throws a slower nested error after a non-nullable nested field throws a nested error": "there is no slower error in Go",
}

// throwingData and nullingData are graphql-js's own root values: every field
// of the first fails, and every field of the second is null. A nest answers
// with itself, so a query can go as deep as it likes.
type throwingRoot struct{}

func (throwingRoot) Sync() (any, error)        { return nil, errors.New("sync") }
func (throwingRoot) SyncNonNull() (any, error) { return nil, errors.New("syncNonNull") }
func (throwingRoot) Promise() (any, error)     { return nil, errors.New("promise") }

func (throwingRoot) PromiseNonNull() (any, error) { return nil, errors.New("promiseNonNull") }
func (d throwingRoot) SyncNest() any              { return d }
func (d throwingRoot) SyncNonNullNest() any       { return d }
func (d throwingRoot) PromiseNest() any           { return d }
func (d throwingRoot) PromiseNonNullNest() any    { return d }

type nullingRoot struct{}

func (nullingRoot) Sync() any           { return nil }
func (nullingRoot) SyncNonNull() any    { return nil }
func (nullingRoot) Promise() any        { return nil }
func (nullingRoot) PromiseNonNull() any { return nil }
func (d nullingRoot) SyncNest() any     { return d }
func (d nullingRoot) SyncNonNullNest() any {
	return d
}
func (d nullingRoot) PromiseNest() any        { return d }
func (d nullingRoot) PromiseNonNullNest() any { return d }

var (
	throwingData any = throwingRoot{}
	nullingData  any = nullingRoot{}
)

// testNonNullSchema is graphql-js's own schema from nonnull-test.ts.
func testNonNullSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := utilities.BuildSchema(`
		type DataType {
			sync: String
			syncNonNull: String!
			promise: String
			promiseNonNull: String!
			syncNest: DataType
			syncNonNullNest: DataType!
			promiseNest: DataType
			promiseNonNullNest: DataType!
		}

		schema {
			query: DataType
		}
	`)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	return s
}
