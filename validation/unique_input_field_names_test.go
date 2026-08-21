package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestUniqueInputFieldNames(t *testing.T) {
	s := testSchema(t)
	rule := validation.UniqueInputFieldNamesRule

	t.Run("input object with fields", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				complicatedArgs {
					complexArgField(complexArg: { requiredField: true, intField: 1 })
				}
			}
		`)
	})

	// Each input object keeps its own names, so a field of the same name in a
	// nested one is not a repeat.
	t.Run("the same name at different levels", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				complicatedArgs {
					complexArgField(complexArg: {
						requiredField: true
						stringField: "a"
					})
					intArgField(intArg: 1)
				}
			}
		`)
	})

	t.Run("a field given twice", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				complicatedArgs {
					complexArgField(complexArg: { intField: 1, intField: 2 })
				}
			}
		`,
			want{Message: `only one input field named "intField"`, At: []at{{3, 33}, {3, 46}}},
		)
	})

	t.Run("a field given three times", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				complicatedArgs {
					complexArgField(complexArg: { intField: 1, intField: 2, intField: 3 })
				}
			}
		`,
			want{Message: `only one input field named "intField"`, At: []at{{3, 33}, {3, 46}}},
			want{Message: `only one input field named "intField"`, At: []at{{3, 33}, {3, 59}}},
		)
	})

	// The names of an inner object must not leak into the outer one when the
	// walk comes back out.
	t.Run("a repeat in a nested object", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				complicatedArgs {
					complexArgField(complexArg: { stringListField: [{ a: 1, a: 2 }] })
				}
			}
		`,
			want{Message: `only one input field named "a"`, At: []at{{3, 53}, {3, 59}}},
		)
	})
}
