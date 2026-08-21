package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestNoUnusedFragments(t *testing.T) {
	s := testSchema(t)
	rule := validation.NoUnusedFragmentsRule

	t.Run("all fragments used", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				human(id: 4) {
					...HumanFields1
					... on Human {
						...HumanFields2
					}
				}
			}
			fragment HumanFields1 on Human {
				name
				...HumanFields3
			}
			fragment HumanFields2 on Human {
				name
			}
			fragment HumanFields3 on Human {
				name
			}
		`)
	})

	// A fragment spread before the definition it names is still a use: the
	// check runs once the whole document has been read.
	t.Run("used by an operation written above it", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				dog {
					...Later
				}
			}
			fragment Later on Dog { name }
		`)
	})

	t.Run("a fragment used by another fragment only", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				dog { name }
			}
			fragment Unused1 on Dog {
				...Unused2
			}
			fragment Unused2 on Dog {
				name
			}
		`,
			want{Message: `Fragment "Unused1" is never used`, At: []at{{4, 1}}},
			want{Message: `Fragment "Unused2" is never used`, At: []at{{7, 1}}},
		)
	})

	t.Run("some fragments unused", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				human(id: 4) {
					...HumanFields1
				}
			}
			fragment HumanFields1 on Human {
				name
			}
			fragment Unused1 on Human {
				name
			}
			fragment Unused2 on Human {
				name
			}
		`,
			want{Message: `Fragment "Unused1" is never used`, At: []at{{9, 1}}},
			want{Message: `Fragment "Unused2" is never used`, At: []at{{12, 1}}},
		)
	})
}
