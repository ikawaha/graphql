package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestUniqueFragmentNames(t *testing.T) {
	s := testSchema(t)
	rule := validation.UniqueFragmentNamesRule

	t.Run("no fragments", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				dog { name }
			}
		`)
	})

	t.Run("differently named fragments", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				dog {
					...fragA
					...fragB
				}
			}
			fragment fragA on Dog { name }
			fragment fragB on Dog { nickname }
		`)
	})

	t.Run("an operation and a fragment of the same name", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo {
				dog {
					...Foo
				}
			}
			fragment Foo on Dog { name }
		`)
	})

	t.Run("two fragments of the same name", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				dog {
					...fragA
				}
			}
			fragment fragA on Dog { name }
			fragment fragA on Dog { nickname }
		`,
			want{Message: `only one fragment named "fragA"`, At: []at{{6, 10}, {7, 10}}},
		)
	})
}
