package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestVariablesInAllowedPosition(t *testing.T) {
	s := testSchema(t)
	rule := validation.VariablesInAllowedPositionRule

	t.Run("a nullable variable in a nullable position", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo($a: Boolean) {
				complicatedArgs { booleanArgField(booleanArg: $a) }
			}
		`)
	})

	t.Run("a non-null variable in a nullable position", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo($a: Boolean!) {
				complicatedArgs { booleanArgField(booleanArg: $a) }
			}
		`)
	})

	t.Run("a non-null variable in a non-null position", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo($a: Int!) {
				complicatedArgs { nonNullIntArgField(nonNullIntArg: $a) }
			}
		`)
	})

	t.Run("a list variable in a list position", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo($a: [String]) {
				complicatedArgs { stringListArgField(stringListArg: $a) }
			}
		`)
	})

	// The variable's own default means null can never reach the position, so a
	// nullable variable is allowed where a non-null value is wanted.
	t.Run("a nullable variable with a default in a non-null position", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo($a: Int = 1) {
				complicatedArgs { nonNullIntArgField(nonNullIntArg: $a) }
			}
		`)
	})

	// The position's own default does the same job.
	t.Run("a nullable variable where the location has a default", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo($a: Int) {
				complicatedArgs { nonNullFieldWithDefault(arg: $a) }
			}
		`)
	})

	t.Run("a nullable variable in a non-null position", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query Foo($a: Int) {
				complicatedArgs { nonNullIntArgField(nonNullIntArg: $a) }
			}
		`,
			want{Message: `Variable "$a" of type "Int" used in position expecting type "Int!".`, At: []at{{1, 11}, {2, 54}}},
		)
	})

	// A default of null supplies null, which is what the position cannot take.
	t.Run("a variable defaulting to null in a non-null position", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query Foo($a: Int = null) {
				complicatedArgs { nonNullIntArgField(nonNullIntArg: $a) }
			}
		`,
			want{Message: `used in position expecting type "Int!"`, At: []at{{1, 11}, {2, 54}}},
		)
	})

	t.Run("a variable of an unrelated type", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query Foo($a: String) {
				complicatedArgs { intArgField(intArg: $a) }
			}
		`,
			want{Message: `Variable "$a" of type "String" used in position expecting type "Int".`, At: []at{{1, 11}, {2, 40}}},
		)
	})

	// A list of nullable elements cannot stand where the elements must not be
	// null, since the elements are what would be missing.
	t.Run("a list of nullable elements where they must not be null", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query Foo($a: [String]) {
				complicatedArgs { stringListNonNullArgField(stringListNonNullArg: $a) }
			}
		`,
			want{Message: `of type "[String]" used in position expecting type "[String!]"`, At: []at{{1, 11}, {2, 68}}},
		)
	})

	// The check reaches through fragments, since the operation supplies what
	// they use.
	t.Run("through a fragment", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query Foo($a: Int) {
				complicatedArgs { ...FragA }
			}
			fragment FragA on ComplicatedArgs {
				nonNullIntArgField(nonNullIntArg: $a)
			}
		`,
			want{Message: `used in position expecting type "Int!"`, At: []at{{1, 11}, {5, 36}}},
		)
	})
}
