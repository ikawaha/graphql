package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestUniqueOperationNames(t *testing.T) {
	s := testSchema(t)
	rule := validation.UniqueOperationNamesRule

	t.Run("no operations", func(t *testing.T) {
		expectValid(t, s, rule, `
			fragment fragA on Dog {
				name
			}
		`)
	})

	t.Run("one anonymous operation", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				dog {
					name
				}
			}
		`)
	})

	t.Run("differently named operations", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo {
				dog {
					name
				}
			}
			query Bar {
				dog {
					name
				}
			}
		`)
	})

	// A fragment and an operation of the same name are not in conflict: a
	// spread and a request name them in different places.
	t.Run("a fragment and an operation of the same name", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo {
				dog {
					...Foo
				}
			}
			fragment Foo on Dog {
				name
			}
		`)
	})

	t.Run("two operations of the same name", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query Foo {
				dog {
					name
				}
			}
			query Foo {
				dog {
					name
				}
			}
		`,
			want{Message: `only one operation named "Foo"`, At: []at{{1, 7}, {6, 7}}},
		)
	})

	// Two operations of a name conflict however they are written, because it
	// is the name a request gives that has to pick one out.
	t.Run("of different kinds", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query Foo {
				dog {
					name
				}
			}
			mutation Foo {
				testMutation(arg: "x")
			}
		`,
			want{Message: `only one operation named "Foo"`, At: []at{{1, 7}, {6, 10}}},
		)
	})

	t.Run("three of the same name", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query Foo { dog { name } }
			query Foo { cat { name } }
			query Foo { pet { name } }
		`,
			want{Message: `only one operation named "Foo"`, At: []at{{1, 7}, {2, 7}}},
			want{Message: `only one operation named "Foo"`, At: []at{{1, 7}, {3, 7}}},
		)
	})
}
