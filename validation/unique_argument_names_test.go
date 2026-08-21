package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestUniqueArgumentNames(t *testing.T) {
	s := testSchema(t)
	rule := validation.UniqueArgumentNamesRule

	t.Run("no arguments", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				dog {
					name
				}
			}
		`)
	})

	t.Run("arguments of different names", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				dog {
					isAtLocation(x: 0, y: 0)
				}
			}
		`)
	})

	// Two fields may each take an argument of the same name; the rule is about
	// one field being given the same argument twice.
	t.Run("the same argument name on different fields", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				dog {
					name(surname: true)
				}
				cat {
					name(surname: true)
				}
			}
		`)
	})

	t.Run("a directive argument", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				dog @repeatableDirective(arg: 1) {
					name
				}
			}
		`)
	})

	t.Run("one field given an argument twice", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				dog {
					isAtLocation(x: 0, x: 1)
				}
			}
		`,
			want{Message: `only one argument named "x"`, At: []at{{3, 16}, {3, 22}}},
		)
	})

	t.Run("one field given an argument three times", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				dog {
					isAtLocation(x: 0, x: 1, x: 2)
				}
			}
		`,
			want{Message: `only one argument named "x"`, At: []at{{3, 16}, {3, 22}, {3, 28}}},
		)
	})

	t.Run("a directive given an argument twice", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				dog @repeatableDirective(arg: 1, arg: 2) {
					name
				}
			}
		`,
			want{Message: `only one argument named "arg"`, At: []at{{2, 27}, {2, 35}}},
		)
	})
}
