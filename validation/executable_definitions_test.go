package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestExecutableDefinitions(t *testing.T) {
	s := testSchema(t)
	rule := validation.ExecutableDefinitionsRule

	t.Run("with only operation", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo {
				dog {
					name
				}
			}
		`)
	})

	t.Run("with operation and fragment", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo {
				dog {
					name
					...Frag
				}
			}

			fragment Frag on Dog {
				name
			}
		`)
	})

	t.Run("with type definition", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query Foo {
				dog {
					name
				}
			}

			type Cow {
				name: String
			}

			extend type Dog {
				color: String
			}
		`,
			want{Message: `"Cow" definition is not executable`, At: []at{{7, 1}}},
			want{Message: `"Dog" definition is not executable`, At: []at{{11, 1}}},
		)
	})

	// A schema definition has no name of its own, so the message says so
	// rather than quoting nothing.
	t.Run("with schema definition", func(t *testing.T) {
		expectErrors(t, s, rule, `
			schema {
				query: Query
			}

			type Query {
				test: String
			}

			extend schema @directive
		`,
			want{Message: "schema definition is not executable", At: []at{{1, 1}}},
			want{Message: `"Query" definition is not executable`, At: []at{{5, 1}}},
			want{Message: "schema definition is not executable", At: []at{{9, 1}}},
		)
	})

	t.Run("with a directive definition", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query Foo {
				dog {
					name
				}
			}

			directive @auth on FIELD
		`,
			want{Message: `"auth" definition is not executable`, At: []at{{7, 1}}},
		)
	})
}
