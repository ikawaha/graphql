package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestValuesOfCorrectType(t *testing.T) {
	s := testSchema(t)
	rule := validation.ValuesOfCorrectTypeRule

	t.Run("values of the right type", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				complicatedArgs {
					intArgField(intArg: 2)
					floatArgField(floatArg: 1.5)
					stringArgField(stringArg: "foo")
					booleanArgField(booleanArg: true)
					idArgField(idArg: "someId")
					enumArgField(enumArg: BROWN)
					stringListArgField(stringListArg: ["one", "two"])
					complexArgField(complexArg: { requiredField: true, intField: 1 })
				}
			}
		`)
	})

	// An Int literal is acceptable where a Float is wanted, and an ID takes
	// either a string or an integer.
	t.Run("values the schema converts", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				complicatedArgs {
					floatArgField(floatArg: 1)
					idArgField(idArg: 1)
				}
			}
		`)
	})

	t.Run("null where the type is nullable", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				complicatedArgs {
					intArgField(intArg: null)
					stringListArgField(stringListArg: null)
				}
			}
		`)
	})

	// A single value is accepted where a list is wanted, which the
	// specification calls list input coercion.
	t.Run("a single value where a list is wanted", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				complicatedArgs {
					stringListArgField(stringListArg: "one")
				}
			}
		`)
	})

	// The type itself says why, which is more use than "it does not fit".
	t.Run("a string where an Int is wanted", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				complicatedArgs {
					intArgField(intArg: "three")
				}
			}
		`,
			want{Message: `Int cannot represent non-integer value: "three"`, At: []at{{3, 23}}},
		)
	})

	t.Run("a float where an Int is wanted", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				complicatedArgs {
					intArgField(intArg: 1.5)
				}
			}
		`,
			want{Message: `Int cannot represent non-integer value: 1.5`, At: []at{{3, 23}}},
		)
	})

	t.Run("null where the type is not nullable", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				complicatedArgs {
					nonNullIntArgField(nonNullIntArg: null)
				}
			}
		`,
			want{Message: `Expected value of non-null type "Int!" not to be null.`, At: []at{{3, 37}}},
		)
	})

	t.Run("a value not in the enum", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				complicatedArgs {
					enumArgField(enumArg: PURPLE)
				}
			}
		`,
			want{Message: `Value "PURPLE" does not exist in "FurColor" enum.`, At: []at{{3, 25}}},
		)
	})

	// An enum member is written as a bare name, so a string is not one.
	t.Run("a string where an enum is wanted", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				complicatedArgs {
					enumArgField(enumArg: "BROWN")
				}
			}
		`,
			want{Message: `Enum "FurColor" cannot represent non-enum value: "BROWN".`, At: []at{{3, 25}}},
		)
	})

	t.Run("a required input field left out", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				complicatedArgs {
					complexArgField(complexArg: { intField: 1 })
				}
			}
		`,
			want{Message: `Expected value of type "ComplexInput" to include required field "requiredField"`, At: []at{{3, 31}}},
		)
	})

	// The element type is what each entry is judged against, so one bad entry
	// is reported where it is written.
	t.Run("a bad entry in a list", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				complicatedArgs {
					stringListArgField(stringListArg: ["one", 2])
				}
			}
		`,
			want{Message: `String cannot represent a non string value: 2`, At: []at{{3, 45}}},
		)
	})

	// Where the list itself does not belong, saying so once is more use than
	// complaining about every entry.
	t.Run("a list where a scalar is wanted", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				complicatedArgs {
					intArgField(intArg: [1, 2, 3])
				}
			}
		`,
			want{Message: `Int cannot represent non-integer value: [1, 2, 3]`, At: []at{{3, 23}}},
		)
	})

	t.Run("an object where a scalar is wanted", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				complicatedArgs {
					intArgField(intArg: { a: 1 })
				}
			}
		`,
			want{Message: `Int cannot represent non-integer value: { a: 1 }`, At: []at{{3, 23}}},
		)
	})

	t.Run("a variable default of the wrong type", func(t *testing.T) {
		expectErrors(t, s, rule, `
			query Foo($a: Int = "not a number") {
				complicatedArgs { intArgField(intArg: $a) }
			}
		`,
			want{Message: `Int cannot represent non-integer value: "not a number"`, At: []at{{1, 21}}},
		)
	})

	t.Run("exactly one key for a oneOf input", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				complicatedArgs {
					oneOfArgField(oneOfArg: { stringField: "a" })
				}
			}
		`)
	})

	t.Run("more than one key for a oneOf input", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				complicatedArgs {
					oneOfArgField(oneOfArg: { stringField: "a", intField: 1 })
				}
			}
		`,
			want{Message: `Within OneOf Input Object type "OneOfInput", exactly one field must be specified`, At: []at{{3, 27}}},
		)
	})

	t.Run("no keys for a oneOf input", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				complicatedArgs {
					oneOfArgField(oneOfArg: {})
				}
			}
		`,
			want{Message: `exactly one field must be specified`, At: []at{{3, 27}}},
		)
	})

	t.Run("null as the one key of a oneOf input", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				complicatedArgs {
					oneOfArgField(oneOfArg: { stringField: null })
				}
			}
		`,
			want{Message: `Within OneOf Input Object type "OneOfInput", exactly one field must be specified`, At: []at{{3, 27}}},
		)
	})

	// What a variable holds is not known while a document is being checked, so
	// nothing is said about one here; whether it may stand where it is written
	// is settled by VariablesInAllowedPosition.
	t.Run("a variable as the one key of a oneOf input", func(t *testing.T) {
		expectValid(t, s, rule, `
			query Foo($s: String) {
				complicatedArgs {
					oneOfArgField(oneOfArg: { stringField: $s })
				}
			}
		`)
	})
}
