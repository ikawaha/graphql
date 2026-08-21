package validation_test

import (
	"strings"
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestMaxIntrospectionDepth(t *testing.T) {
	s := testSchema(t)
	rule := validation.MaxIntrospectionDepthRule

	t.Run("an ordinary query", func(t *testing.T) {
		expectValid(t, s, rule, `{ dog { name } }`)
	})

	t.Run("a shallow introspection query", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				__schema {
					queryType {
						name
						fields { name }
					}
				}
			}
		`)
	})

	// Nesting the fields that lead back to a type is what makes the response
	// grow out of proportion to the request.
	t.Run("deeply nested introspection", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				__schema {
					types {
						fields { type { fields { type { fields { type { name } } } } } }
					}
				}
			}
		`,
			want{Message: "Maximum introspection depth exceeded"},
		)
	})

	t.Run("deeply nested through __type", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				__type(name: "Dog") {
					fields { type { fields { type { fields { type { name } } } } } }
				}
			}
		`,
			want{Message: "Maximum introspection depth exceeded"},
		)
	})

	// Depth reached through fragments counts the same as depth written inline.
	t.Run("depth reached through fragments", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				__schema { types { ...F1 } }
			}
			fragment F1 on __Type { fields { type { ...F2 } } }
			fragment F2 on __Type { fields { type { ...F3 } } }
			fragment F3 on __Type { fields { type { name } } }
		`,
			want{Message: "Maximum introspection depth exceeded"},
		)
	})

	// A cycle between fragments is a separate complaint, and following it here
	// must not spin.
	t.Run("a cycle between fragments terminates", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				__schema { types { ...F1 } }
			}
			fragment F1 on __Type { ...F2 }
			fragment F2 on __Type { ...F1 }
		`)
	})

	// Each way into the introspection types is measured on its own, since each
	// costs what it costs.
	t.Run("reported for each introspection root", func(t *testing.T) {
		deep := "fields { type { fields { type { fields { type { name } } } } } }"
		query := "{ a: __schema { types { " + deep + " } } b: __schema { types { " + deep + " } } }"
		expectErrors(t, s, rule, query,
			want{Message: "Maximum introspection depth exceeded"},
			want{Message: "Maximum introspection depth exceeded"},
		)
		if strings.Count(query, "__schema") != 2 {
			t.Fatal("the test document no longer has two introspection roots")
		}
	})
}
