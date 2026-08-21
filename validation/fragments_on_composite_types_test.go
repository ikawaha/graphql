package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestFragmentsOnCompositeTypes(t *testing.T) {
	s := testSchema(t)
	rule := validation.FragmentsOnCompositeTypesRule

	t.Run("on an object", func(t *testing.T) {
		expectValid(t, s, rule, `fragment validFragment on Dog { barks }`)
	})

	t.Run("on an interface", func(t *testing.T) {
		expectValid(t, s, rule, `fragment validFragment on Pet { name }`)
	})

	t.Run("on a union", func(t *testing.T) {
		expectValid(t, s, rule, `fragment validFragment on CatOrDog { __typename }`)
	})

	// An inline fragment with no condition takes the enclosing type, which is
	// composite by construction.
	t.Run("inline fragment without a condition", func(t *testing.T) {
		expectValid(t, s, rule, `
			fragment validFragment on Pet {
				... { name }
			}
		`)
	})

	t.Run("on a scalar", func(t *testing.T) {
		expectErrors(t, s, rule, `fragment scalarFragment on Boolean { bad }`,
			want{Message: `Fragment "scalarFragment" cannot condition on non composite type "Boolean"`, At: []at{{1, 28}}},
		)
	})

	t.Run("on an enum", func(t *testing.T) {
		expectErrors(t, s, rule, `fragment scalarFragment on FurColor { bad }`,
			want{Message: `cannot condition on non composite type "FurColor"`, At: []at{{1, 28}}},
		)
	})

	t.Run("on an input object", func(t *testing.T) {
		expectErrors(t, s, rule, `fragment inputFragment on ComplexInput { stringField }`,
			want{Message: `cannot condition on non composite type "ComplexInput"`, At: []at{{1, 27}}},
		)
	})

	t.Run("inline fragment on a scalar", func(t *testing.T) {
		expectErrors(t, s, rule, `
			fragment invalidFragment on Pet {
				... on String { barks }
			}
		`,
			want{Message: `Fragment cannot condition on non composite type "String"`, At: []at{{2, 9}}},
		)
	})
}
