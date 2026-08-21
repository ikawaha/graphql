package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestNoFragmentCycles(t *testing.T) {
	s := testSchema(t)
	rule := validation.NoFragmentCyclesRule

	t.Run("single reference is fine", func(t *testing.T) {
		expectValid(t, s, rule, `
			fragment fragA on Dog { ...fragB }
			fragment fragB on Dog { name }
		`)
	})

	t.Run("spreading twice is not a cycle", func(t *testing.T) {
		expectValid(t, s, rule, `
			fragment fragA on Dog { ...fragB, ...fragB }
			fragment fragB on Dog { name }
		`)
	})

	// Reaching the same fragment by two routes is not a cycle: neither route
	// comes back to where it started.
	t.Run("spreading twice indirectly is not a cycle", func(t *testing.T) {
		expectValid(t, s, rule, `
			fragment fragA on Dog { ...fragB, ...fragC }
			fragment fragB on Dog { ...fragC }
			fragment fragC on Dog { name }
		`)
	})

	t.Run("a fragment spreading itself", func(t *testing.T) {
		expectErrors(t, s, rule, `
			fragment fragA on Dog { ...fragA }
		`,
			want{Message: `Cannot spread fragment "fragA" within itself.`, At: []at{{1, 25}}},
		)
	})

	t.Run("a cycle through another fragment", func(t *testing.T) {
		expectErrors(t, s, rule, `
			fragment fragA on Dog { ...fragB }
			fragment fragB on Dog { ...fragA }
		`,
			want{Message: `within itself via "fragB"`, At: []at{{1, 25}, {2, 25}}},
		)
	})

	t.Run("a longer cycle", func(t *testing.T) {
		expectErrors(t, s, rule, `
			fragment fragA on Dog { ...fragB }
			fragment fragB on Dog { ...fragC }
			fragment fragC on Dog { ...fragA }
		`,
			want{Message: `within itself via "fragB", "fragC"`, At: []at{{1, 25}, {2, 25}, {3, 25}}},
		)
	})

	// A fragment that reaches a cycle without being part of it is reported
	// once, at the cycle rather than at the way in.
	t.Run("a cycle reached from outside", func(t *testing.T) {
		expectErrors(t, s, rule, `
			fragment fragA on Dog { ...fragB }
			fragment fragB on Dog { ...fragC }
			fragment fragC on Dog { ...fragB }
		`,
			want{Message: `Cannot spread fragment "fragB" within itself via "fragC".`, At: []at{{2, 25}, {3, 25}}},
		)
	})

	// An operation cannot be spread, so the walk stops there.
	t.Run("an operation is not part of a cycle", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				dog { ...fragA }
			}
			fragment fragA on Dog { name }
		`)
	})
}
