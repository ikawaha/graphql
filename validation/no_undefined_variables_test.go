package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestNoUndefinedVariables(t *testing.T) {
	s := testSchema(t)
	rule := validation.NoUndefinedVariablesRule

	t.Run("all variables defined", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo($a: String, $b: String, $c: String) {
				complicatedArgs {
					multipleReqs(req1: 1, req2: 2)
					stringArgField(stringArg: $a)
					booleanArgField(booleanArg: $b)
					idArgField(idArg: $c)
				}
			}
		`)
	})

	// A fragment's variables are supplied by whichever operation spreads it,
	// so the check has to reach through spreads.
	t.Run("defined by the operation that spreads the fragment", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo($a: String) {
				complicatedArgs { ...FragA }
			}
			fragment FragA on ComplicatedArgs {
				stringArgField(stringArg: $a)
			}
		`)
	})

	t.Run("a variable not defined", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query Foo($a: String, $b: String) {
				complicatedArgs {
					stringArgField(stringArg: $a)
					booleanArgField(booleanArg: $b)
					idArgField(idArg: $c)
				}
			}
		`,
			want{Message: `Variable "$c" is not defined by operation "Foo".`, At: []at{{5, 21}, {1, 1}}},
		)
	})

	t.Run("a variable not defined by an anonymous operation", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				complicatedArgs {
					stringArgField(stringArg: $a)
				}
			}
		`,
			want{Message: `Variable "$a" is not defined.`, At: []at{{3, 29}, {1, 1}}},
		)
	})

	t.Run("used by a fragment but defined by no operation", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query Foo {
				complicatedArgs { ...FragA }
			}
			fragment FragA on ComplicatedArgs {
				stringArgField(stringArg: $a)
			}
		`,
			want{Message: `Variable "$a" is not defined by operation "Foo".`, At: []at{{5, 28}, {1, 1}}},
		)
	})

	// Two operations spreading the same fragment are each checked on their
	// own, so one may supply the variable and the other not.
	t.Run("defined by one operation and not another", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query Foo($a: String) {
				complicatedArgs { ...FragA }
			}
			query Bar {
				complicatedArgs { ...FragA }
			}
			fragment FragA on ComplicatedArgs {
				stringArgField(stringArg: $a)
			}
		`,
			want{Message: `Variable "$a" is not defined by operation "Bar".`, At: []at{{8, 28}, {4, 1}}},
		)
	})
}

// A fragment may declare variables of its own, which the spread supplies
// rather than the operation. Treating one of those as undeclared would report
// a fault in a document that is sound.
func TestNoUndefinedVariables_FragmentArguments(t *testing.T) {
	s := testSchema(t)

	t.Run("a variable the fragment declares", func(t *testing.T) {
		expectValid(t, s, validation.NoUndefinedVariablesRule, `
			query Foo {
				complicatedArgs { ...FragA(a: "x") }
			}
			fragment FragA($a: String) on ComplicatedArgs {
				stringArgField(stringArg: $a)
			}
		`)
	})

	// Nor is such a variable one the operation declared and failed to use.
	t.Run("it is not counted as unused by the operation", func(t *testing.T) {
		expectErrors(t, s, validation.NoUnusedVariablesRule, `
			query Foo($a: String) {
				complicatedArgs { ...FragA(a: "x") }
			}
			fragment FragA($a: String) on ComplicatedArgs {
				stringArgField(stringArg: $a)
			}
		`,
			want{Message: `Variable "$a" is never used in operation "Foo".`, At: []at{{1, 11}}},
		)
	})
}
