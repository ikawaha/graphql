package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestScalarLeafs(t *testing.T) {
	s := testSchema(t)
	rule := validation.ScalarLeafsRule

	t.Run("a scalar selected as a leaf", func(t *testing.T) {
		expectValid(t, s, rule, `fragment scalarSelection on Dog { barks }`)
	})

	t.Run("an object with a selection", func(t *testing.T) {
		expectValid(t, s, rule, `
			fragment directSelection on Human {
				pets { name }
			}
		`)
	})

	t.Run("a scalar given a selection", func(t *testing.T) {
		expectErrors(t, s, rule, `fragment scalarSelectionsNotAllowedOnBoolean on Dog { barks { sinceWhen } }`,
			want{Message: `Field "barks" must not have a selection since type "Boolean" has no subfields.`, At: []at{{1, 61}}},
		)
	})

	t.Run("an enum given a selection", func(t *testing.T) {
		expectErrors(t, s, rule, `fragment f on Cat { furColor { inHexAdecimal } }`,
			want{Message: `must not have a selection since type "FurColor" has no subfields.`, At: []at{{1, 30}}},
		)
	})

	t.Run("an object without a selection", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query directQueryOnObjectWithoutSubFields {
				human
			}
		`,
			want{Message: `Field "human" of type "Human" must have a selection of subfields. Did you mean "human { ... }"?`, At: []at{{2, 2}}},
		)
	})

	t.Run("an interface without a selection", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query {
				pet
			}
		`,
			want{Message: `Field "pet" of type "Pet" must have a selection of subfields.`, At: []at{{2, 2}}},
		)
	})

	// A wrapped type is reported as written, so the message names the list
	// rather than what is in it.
	t.Run("a list of objects without a selection", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query {
				human(id: 4) {
					relatives
				}
			}
		`,
			want{Message: `of type "[Human]!" must have a selection of subfields.`, At: []at{{3, 3}}},
		)
	})
}
