package validation_test

// Ported from the graphql-js cases that pass hideSuggestions: the same
// documents as elsewhere, with the "Did you mean …?" left off.
//
// A suggestion is worked out from the schema, so it names types, fields and
// enum members the document got close to. A server that does not answer
// introspection is hiding those names on purpose.

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestPortedWithoutSuggestions_FieldsOnCorrectType(t *testing.T) {
	s := testSchema(t)
	rule := validation.FieldsOnCorrectTypeRule

	t.Run("a field not defined on a fragment", func(t *testing.T) {
		const query = `
			fragment fieldNotDefined on Dog {
				meowVolume
			}
		`
		expectErrors(t, s, rule, query,
			want{Message: `Cannot query field "meowVolume" on type "Dog". Did you mean "barkVolume"?`})
		expectErrorsWithoutSuggestions(t, s, rule, query,
			want{Message: `Cannot query field "meowVolume" on type "Dog".`})
	})

	t.Run("a field the interface's implementations have", func(t *testing.T) {
		const query = `
			fragment deepFieldNotDefined on Pet {
				nickname
			}
		`
		expectErrors(t, s, rule, query,
			want{Message: `Cannot query field "nickname" on type "Pet". ` +
				`Did you mean to use an inline fragment on "Cat" or "Dog"?`})
		expectErrorsWithoutSuggestions(t, s, rule, query,
			want{Message: `Cannot query field "nickname" on type "Pet".`})
	})
}

func TestPortedWithoutSuggestions_KnownArgumentNames(t *testing.T) {
	s := testSchema(t)
	rule := validation.KnownArgumentNamesRule

	t.Run("a misspelled argument of a field", func(t *testing.T) {
		const query = `
			fragment invalidArgName on Dog {
				doesKnowCommand(DogCommand: true)
			}
		`
		expectErrors(t, s, rule, query,
			want{Message: `Unknown argument "DogCommand" on field "Dog.doesKnowCommand". ` +
				`Did you mean "dogCommand"?`})
		expectErrorsWithoutSuggestions(t, s, rule, query,
			want{Message: `Unknown argument "DogCommand" on field "Dog.doesKnowCommand".`})
	})

	t.Run("a misspelled argument of a directive", func(t *testing.T) {
		const query = `
			{
				dog @skip(iff: true)
			}
		`
		expectErrors(t, s, rule, query,
			want{Message: `Unknown argument "iff" on directive "@skip". Did you mean "if"?`})
		expectErrorsWithoutSuggestions(t, s, rule, query,
			want{Message: `Unknown argument "iff" on directive "@skip".`})
	})

	t.Run("a misspelled argument of a fragment", func(t *testing.T) {
		const query = `
			{
				dog {
					...withArg(command: SIT)
				}
			}
			fragment withArg($dogCommand: DogCommand) on Dog {
				doesKnowCommand(dogCommand: $dogCommand)
			}
		`
		expectErrors(t, s, rule, query,
			want{Message: `Unknown argument "command" on fragment "withArg". Did you mean "dogCommand"?`})
		expectErrorsWithoutSuggestions(t, s, rule, query,
			want{Message: `Unknown argument "command" on fragment "withArg".`})
	})
}

func TestPortedWithoutSuggestions_KnownTypeNames(t *testing.T) {
	s := testSchema(t)
	rule := validation.KnownTypeNamesRule
	const query = `
		query Foo($var: [JumbledUpLetters!]!) {
			user(id: 4) {
				name
				pets { ... on Badger { name }, ...PetFields }
			}
		}
		fragment PetFields on Peat {
			name
		}
	`
	expectErrorsWithoutSuggestions(t, s, rule, query,
		want{Message: `Unknown type "JumbledUpLetters".`},
		want{Message: `Unknown type "Badger".`},
		want{Message: `Unknown type "Peat".`},
	)
}

// The suggestions the schema package works out for an input value are reached
// through the rule, so turning them off there has to reach all the way down.
func TestPortedWithoutSuggestions_ValuesOfCorrectType(t *testing.T) {
	s := testSchema(t)
	rule := validation.ValuesOfCorrectTypeRule

	t.Run("a misspelled enum member", func(t *testing.T) {
		const query = `
			{
				dog {
					doesKnowCommand(dogCommand: sit)
				}
			}
		`
		expectErrors(t, s, rule, query,
			want{Message: `Value "sit" does not exist in "DogCommand" enum. ` +
				`Did you mean the enum value "SIT"?`})
		expectErrorsWithoutSuggestions(t, s, rule, query,
			want{Message: `Value "sit" does not exist in "DogCommand" enum.`})
	})

	t.Run("an unknown field of an input object", func(t *testing.T) {
		const query = `
			{
				complicatedArgs {
					complexArgField(complexArg: {
						requiredField: true,
						invalidField: "value"
					})
				}
			}
		`
		expectErrorsWithoutSuggestions(t, s, rule, query,
			want{Message: `Expected value of type "ComplexInput" not to include unknown field ` +
				`"invalidField", found: { requiredField: true, invalidField: "value" }.`})
	})
}
