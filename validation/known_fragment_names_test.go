package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestKnownFragmentNames(t *testing.T) {
	s := testSchema(t)
	rule := validation.KnownFragmentNamesRule

	t.Run("known fragments", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				human(id: 4) {
					...HumanFields1
					... on Human {
						...HumanFields2
					}
					... {
						name
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

	t.Run("an unknown fragment", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				human(id: 4) {
					...UnknownFragment1
					... on Human {
						...UnknownFragment2
					}
				}
			}
			fragment HumanFields on Human {
				name
				...UnknownFragment3
			}
		`,
			want{Message: `Unknown fragment "UnknownFragment1"`, At: []at{{3, 6}}},
			want{Message: `Unknown fragment "UnknownFragment2"`, At: []at{{5, 7}}},
			want{Message: `Unknown fragment "UnknownFragment3"`, At: []at{{11, 5}}},
		)
	})
}
