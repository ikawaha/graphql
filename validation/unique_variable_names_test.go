package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestUniqueVariableNames(t *testing.T) {
	s := testSchema(t)
	rule := validation.UniqueVariableNamesRule

	t.Run("unique variable names", func(t *testing.T) {
		expectValid(t, s, rule, `
			query A($x: Int, $y: String) { human(id: 4) { name } }
			query B($x: String, $y: Int) { human(id: 4) { name } }
		`)
	})

	t.Run("duplicate variable names", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query A($x: Int, $x: Int, $x: String) { human(id: 4) { name } }
			query B($x: String, $x: Int) { human(id: 4) { name } }
			query C($x: Int, $x: Int) { human(id: 4) { name } }
		`,
			want{Message: `only one variable named "$x"`, At: []at{{1, 10}, {1, 19}, {1, 28}}},
			want{Message: `only one variable named "$x"`, At: []at{{2, 10}, {2, 22}}},
			want{Message: `only one variable named "$x"`, At: []at{{3, 10}, {3, 19}}},
		)
	})
}
