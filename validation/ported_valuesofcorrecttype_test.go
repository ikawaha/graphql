package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/ValuesOfCorrectTypeRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_ValuesOfCorrectType(t *testing.T) {
	runPorted(t, validation.ValuesOfCorrectTypeRule, []portedCase{
		{
			name: `Good int value`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            intArgField(intArg: 2)
          }
        }
      `,
				},
			},
		},
		{
			name: `Good negative int value`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            intArgField(intArg: -2)
          }
        }
      `,
				},
			},
		},
		{
			name: `Good boolean value`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            booleanArgField(booleanArg: true)
          }
        }
      `,
				},
			},
		},
		{
			name: `Good string value`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            stringArgField(stringArg: "foo")
          }
        }
      `,
				},
			},
		},
		{
			name: `Good float value`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            floatArgField(floatArg: 1.1)
          }
        }
      `,
				},
			},
		},
		{
			name: `Good negative float value`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            floatArgField(floatArg: -1.1)
          }
        }
      `,
				},
			},
		},
		{
			name: `Int into Float`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            floatArgField(floatArg: 1)
          }
        }
      `,
				},
			},
		},
		{
			name: `Int into ID`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            idArgField(idArg: 1)
          }
        }
      `,
				},
			},
		},
		{
			name: `String into ID`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            idArgField(idArg: "someIdString")
          }
        }
      `,
				},
			},
		},
		{
			name: `Good enum value`,
			steps: []portedStep{
				{
					query: `
        {
          dog {
            doesKnowCommand(dogCommand: SIT)
          }
        }
      `,
				},
			},
		},
		{
			name: `Enum with undefined value`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            enumArgField(enumArg: UNKNOWN)
          }
        }
      `,
				},
			},
		},
		{
			name: `Enum with null value`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            enumArgField(enumArg: NO_FUR)
          }
        }
      `,
				},
			},
		},
		{
			name: `null into nullable type`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            intArgField(intArg: null)
          }
        }
      `,
				},
				{
					query: `
        {
          dog(a: null, b: null, c:{ requiredField: true, intField: null }) {
            name
          }
        }
      `,
				},
			},
		},
		{
			name: `Int into String`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            stringArgField(stringArg: 1)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 39}}},
					},
				},
			},
		},
		{
			name: `Float into String`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            stringArgField(stringArg: 1.0)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 39}}},
					},
				},
			},
		},
		{
			name: `Boolean into String`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            stringArgField(stringArg: true)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 39}}},
					},
				},
			},
		},
		{
			name: `Unquoted String into String`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            stringArgField(stringArg: BAR)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 39}}},
					},
				},
			},
		},
		{
			name: `String into Int`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            intArgField(intArg: "3")
          }
        }
      `,
					want: []want{
						{At: []at{{4, 33}}},
					},
				},
			},
		},
		{
			name: `Big Int into Int`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            intArgField(intArg: 829384293849283498239482938)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 33}}},
					},
				},
			},
		},
		{
			name: `Unquoted String into Int`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            intArgField(intArg: FOO)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 33}}},
					},
				},
			},
		},
		{
			name: `Simple Float into Int`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            intArgField(intArg: 3.0)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 33}}},
					},
				},
			},
		},
		{
			name: `Float into Int`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            intArgField(intArg: 3.333)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 33}}},
					},
				},
			},
		},
		{
			name: `String into Float`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            floatArgField(floatArg: "3.333")
          }
        }
      `,
					want: []want{
						{At: []at{{4, 37}}},
					},
				},
			},
		},
		{
			name: `Boolean into Float`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            floatArgField(floatArg: true)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 37}}},
					},
				},
			},
		},
		{
			name: `Unquoted into Float`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            floatArgField(floatArg: FOO)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 37}}},
					},
				},
			},
		},
		{
			name: `Int into Boolean`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            booleanArgField(booleanArg: 2)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 41}}},
					},
				},
			},
		},
		{
			name: `Float into Boolean`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            booleanArgField(booleanArg: 1.0)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 41}}},
					},
				},
			},
		},
		{
			name: `String into Boolean`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            booleanArgField(booleanArg: "true")
          }
        }
      `,
					want: []want{
						{At: []at{{4, 41}}},
					},
				},
			},
		},
		{
			name: `Unquoted into Boolean`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            booleanArgField(booleanArg: TRUE)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 41}}},
					},
				},
			},
		},
		{
			name: `Float into ID`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            idArgField(idArg: 1.0)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 31}}},
					},
				},
			},
		},
		{
			name: `Boolean into ID`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            idArgField(idArg: true)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 31}}},
					},
				},
			},
		},
		{
			name: `Unquoted into ID`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            idArgField(idArg: SOMETHING)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 31}}},
					},
				},
			},
		},
		{
			name: `Int into Enum`,
			steps: []portedStep{
				{
					query: `
        {
          dog {
            doesKnowCommand(dogCommand: 2)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 41}}},
					},
				},
			},
		},
		{
			name: `Float into Enum`,
			steps: []portedStep{
				{
					query: `
        {
          dog {
            doesKnowCommand(dogCommand: 1.0)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 41}}},
					},
				},
			},
		},
		{
			name: `String into Enum`,
			steps: []portedStep{
				{
					query: `
        {
          dog {
            doesKnowCommand(dogCommand: "SIT")
          }
        }
      `,
					want: []want{
						{At: []at{{4, 41}}},
					},
				},
			},
		},
		{
			name: `String into Enum (no suggestion)`,
			steps: []portedStep{
				{
					query: `
        {
          dog {
            doesKnowCommand(dogCommand: "SIT")
          }
        }
      `,
					want: []want{
						{At: []at{{4, 41}}},
					},
				},
			},
		},
		{
			name: `Boolean into Enum`,
			steps: []portedStep{
				{
					query: `
        {
          dog {
            doesKnowCommand(dogCommand: true)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 41}}},
					},
				},
			},
		},
		{
			name: `Unknown Enum Value into Enum`,
			steps: []portedStep{
				{
					query: `
        {
          dog {
            doesKnowCommand(dogCommand: JUGGLE)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 41}}},
					},
				},
			},
		},
		{
			name: `Different case Enum Value into Enum`,
			steps: []portedStep{
				{
					query: `
        {
          dog {
            doesKnowCommand(dogCommand: sit)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 41}}},
					},
				},
			},
		},
		{
			name: `Different case Enum Value into Enum (no suggestion)`,
			steps: []portedStep{
				{
					query: `
        {
          dog {
            doesKnowCommand(dogCommand: sit)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 41}}},
					},
				},
			},
		},
		{
			name: `Good list value`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            stringListArgField(stringListArg: ["one", null, "two"])
          }
        }
      `,
				},
			},
		},
		{
			name: `Empty list value`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            stringListArgField(stringListArg: [])
          }
        }
      `,
				},
			},
		},
		{
			name: `Null value`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            stringListArgField(stringListArg: null)
          }
        }
      `,
				},
			},
		},
		{
			name: `Single value into List`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            stringListArgField(stringListArg: "one")
          }
        }
      `,
				},
			},
		},
		{
			name: `Incorrect item type`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            stringListArgField(stringListArg: ["one", 2])
          }
        }
      `,
					want: []want{
						{At: []at{{4, 55}}},
					},
				},
			},
		},
		{
			name: `Single value of incorrect type`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            stringListArgField(stringListArg: 1)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 47}}},
					},
				},
			},
		},
		{
			name: `Arg on optional arg`,
			steps: []portedStep{
				{
					query: `
        {
          dog {
            isHouseTrained(atOtherHomes: true)
          }
        }
      `,
				},
			},
		},
		{
			name: `No Arg on optional arg`,
			steps: []portedStep{
				{
					query: `
        {
          dog {
            isHouseTrained
          }
        }
      `,
				},
			},
		},
		{
			name: `Multiple args`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            multipleReqs(req1: 1, req2: 2)
          }
        }
      `,
				},
			},
		},
		{
			name: `Multiple args reverse order`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            multipleReqs(req2: 2, req1: 1)
          }
        }
      `,
				},
			},
		},
		{
			name: `No args on multiple optional`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            multipleOpts
          }
        }
      `,
				},
			},
		},
		{
			name: `One arg on multiple optional`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            multipleOpts(opt1: 1)
          }
        }
      `,
				},
			},
		},
		{
			name: `Second arg on multiple optional`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            multipleOpts(opt2: 1)
          }
        }
      `,
				},
			},
		},
		{
			name: `Multiple required args on mixedList`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            multipleOptAndReq(req1: 3, req2: 4)
          }
        }
      `,
				},
			},
		},
		{
			name: `Multiple required and one optional arg on mixedList`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            multipleOptAndReq(req1: 3, req2: 4, opt1: 5)
          }
        }
      `,
				},
			},
		},
		{
			name: `All required and optional args on mixedList`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            multipleOptAndReq(req1: 3, req2: 4, opt1: 5, opt2: 6)
          }
        }
      `,
				},
			},
		},
		{
			name: `Incorrect value type`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            multipleReqs(req2: "two", req1: "one")
          }
        }
      `,
					want: []want{
						{At: []at{{4, 32}}},
						{At: []at{{4, 45}}},
					},
				},
			},
		},
		{
			name: `Incorrect value and missing argument (ProvidedRequiredArgumentsRule)`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            multipleReqs(req1: "one")
          }
        }
      `,
					want: []want{
						{At: []at{{4, 32}}},
					},
				},
			},
		},
		{
			name: `Null value (2)`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            multipleReqs(req1: null)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 32}}},
					},
				},
			},
		},
		{
			name: `Optional arg, despite required field in type`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            complexArgField
          }
        }
      `,
				},
			},
		},
		{
			name: `Partial object, only required`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            complexArgField(complexArg: { requiredField: true })
          }
        }
      `,
				},
			},
		},
		{
			name: `Partial object, required field can be falsy`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            complexArgField(complexArg: { requiredField: false })
          }
        }
      `,
				},
			},
		},
		{
			name: `Partial object, including required`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            complexArgField(complexArg: { requiredField: true, intField: 4 })
          }
        }
      `,
				},
			},
		},
		{
			name: `Full object`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            complexArgField(complexArg: {
              requiredField: true,
              intField: 4,
              stringField: "foo",
              booleanField: false,
              stringListField: ["one", "two"]
            })
          }
        }
      `,
				},
			},
		},
		{
			name: `Full object with fields in different order`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            complexArgField(complexArg: {
              stringListField: ["one", "two"],
              booleanField: false,
              requiredField: true,
              stringField: "foo",
              intField: 4,
            })
          }
        }
      `,
				},
			},
		},
		{
			name: `Exactly one field`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            oneOfArgField(oneOfArg: { stringField: "abc" })
          }
        }
      `,
				},
			},
		},
		{
			name: `Exactly one non-nullable variable`,
			steps: []portedStep{
				{
					query: `
        query ($string: String!) {
          complicatedArgs {
            oneOfArgField(oneOfArg: { stringField: $string })
          }
        }
      `,
				},
			},
		},
		{
			name: `Partial object, missing required`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            complexArgField(complexArg: { intField: 4 })
          }
        }
      `,
					want: []want{
						{At: []at{{4, 41}}},
					},
				},
			},
		},
		{
			name: `Partial object, invalid field type`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            complexArgField(complexArg: {
              stringListField: ["one", 2],
              requiredField: true,
            })
          }
        }
      `,
					want: []want{
						{At: []at{{5, 40}}},
					},
				},
			},
		},
		{
			name: `Partial object, null to non-null field`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            complexArgField(complexArg: {
              requiredField: true,
              nonNullField: null,
            })
          }
        }
      `,
					want: []want{
						{At: []at{{6, 29}}},
					},
				},
			},
		},
		{
			name: `Partial object, unknown field arg`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            complexArgField(complexArg: {
              requiredField: true,
              invalidField: "value"
            })
          }
        }
      `,
					want: []want{
						{At: []at{{6, 15}}},
					},
				},
			},
		},
		{
			name: `Partial object, unknown field arg (no suggestions)`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            complexArgField(complexArg: {
              requiredField: true,
              invalidField: "value"
            })
          }
        }
      `,
					want: []want{
						{At: []at{{6, 15}}},
					},
				},
			},
		},
		{
			name: `allows custom scalar to accept complex literals`,
			steps: []portedStep{
				{
					query: `
          {
            test1: anyArg(arg: 123)
            test2: anyArg(arg: "abc")
            test3: anyArg(arg: [123, "abc"])
            test4: anyArg(arg: {deep: [123, "abc"]})
          }
        `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Invalid field type`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            oneOfArgField(oneOfArg: { stringField: 2 })
          }
        }
      `,
					want: []want{
						{At: []at{{4, 52}}},
					},
				},
			},
		},
		{
			name: `Exactly one null field`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            oneOfArgField(oneOfArg: { stringField: null })
          }
        }
      `,
					want: []want{
						{At: []at{{4, 37}}},
					},
				},
			},
		},
		{
			name: `More than one field`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            oneOfArgField(oneOfArg: { stringField: "abc", intField: 123 })
          }
        }
      `,
					want: []want{
						{At: []at{{4, 37}}},
					},
				},
			},
		},
		{
			name: `Unknown field does not add a oneOf error`,
			steps: []portedStep{
				{
					query: `
          {
            complicatedArgs {
              oneOfArgField(oneOfArg: {
                stringField: "abc",
                invalidField: 123
              })
            }
          }
        `,
					want: []want{
						{At: []at{{6, 17}}},
					},
				},
			},
		},
		{
			name: `with directives of valid types`,
			steps: []portedStep{
				{
					query: `
        {
          dog @include(if: true) {
            name
          }
          human @skip(if: false) {
            name
          }
        }
      `,
				},
			},
		},
		{
			name: `with directive with incorrect types`,
			steps: []portedStep{
				{
					query: `
        {
          dog @include(if: "yes") {
            name @skip(if: ENUM)
          }
        }
      `,
					want: []want{
						{At: []at{{3, 28}}},
						{At: []at{{4, 28}}},
					},
				},
			},
		},
		{
			name: `variables with valid default values`,
			steps: []portedStep{
				{
					query: `
        query WithDefaultValues(
          $a: Int = 1,
          $b: String = "ok",
          $c: ComplexInput = { requiredField: true, intField: 3 }
          $d: Int! = 123
        ) {
          dog { name }
        }
      `,
				},
			},
		},
		{
			name: `variables with valid default null values`,
			steps: []portedStep{
				{
					query: `
        query WithDefaultValues(
          $a: Int = null,
          $b: String = null,
          $c: ComplexInput = { requiredField: true, intField: null }
        ) {
          dog { name }
        }
      `,
				},
			},
		},
		{
			name: `variables with invalid default null values`,
			steps: []portedStep{
				{
					query: `
        query WithDefaultValues(
          $a: Int! = null,
          $b: String! = null,
          $c: ComplexInput = { requiredField: null, intField: null }
        ) {
          dog { name }
        }
      `,
					want: []want{
						{At: []at{{3, 22}}},
						{At: []at{{4, 25}}},
						{At: []at{{5, 47}}},
					},
				},
			},
		},
		{
			name: `variables with invalid default values`,
			steps: []portedStep{
				{
					query: `
        query InvalidDefaultValues(
          $a: Int = "one",
          $b: String = 4,
          $c: ComplexInput = "NotVeryComplex"
        ) {
          dog { name }
        }
      `,
					want: []want{
						{At: []at{{3, 21}}},
						{At: []at{{4, 24}}},
						{At: []at{{5, 30}}},
					},
				},
			},
		},
		{
			name: `variables with complex invalid default values`,
			steps: []portedStep{
				{
					query: `
        query WithDefaultValues(
          $a: ComplexInput = { requiredField: 123, intField: "abc" }
        ) {
          dog { name }
        }
      `,
					want: []want{
						{At: []at{{3, 47}}},
						{At: []at{{3, 62}}},
					},
				},
			},
		},
		{
			name: `complex variables missing required field`,
			steps: []portedStep{
				{
					query: `
        query MissingRequiredField($a: ComplexInput = {intField: 3}) {
          dog { name }
        }
      `,
					want: []want{
						{At: []at{{2, 55}}},
					},
				},
			},
		},
		{
			name: `list variables with invalid item`,
			steps: []portedStep{
				{
					query: `
        query InvalidItem($a: [String] = ["one", 2]) {
          dog { name }
        }
      `,
					want: []want{
						{At: []at{{2, 50}}},
					},
				},
			},
		},
		{
			name: `list variables with invalid item (2)`,
			steps: []portedStep{
				{
					query: `
        fragment InvalidItem($a: [String] = ["one", 2]) on Query {
          dog { name }
        }
      `,
					want: []want{
						{At: []at{{2, 53}}},
					},
				},
			},
		},
		{
			name: `fragment spread with invalid argument value`,
			steps: []portedStep{
				{
					query: `
        fragment GivesString on Query {
          ...ExpectsInt(a: "three")
        }
        fragment ExpectsInt($a: Int) on Query {
          dog { name }
        }
      `,
					want: []want{
						{At: []at{{3, 28}}},
					},
				},
			},
		},
	})
}

// Not ported, because each of these is written in a way this could not
// follow:
//   - reports original error for custom scalar which throws: nothing to run
//   - reports error for custom scalar that returns undefined: a document that is not written out
