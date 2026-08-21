package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/VariablesInAllowedPositionRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_VariablesInAllowedPosition(t *testing.T) {
	runPorted(t, validation.VariablesInAllowedPositionRule, []portedCase{
		{
			name: `Boolean => Boolean`,
			steps: []portedStep{
				{
					query: `
      query Query($booleanArg: Boolean)
      {
        complicatedArgs {
          booleanArgField(booleanArg: $booleanArg)
        }
      }
    `,
				},
			},
		},
		{
			name: `Boolean => Boolean within fragment`,
			steps: []portedStep{
				{
					query: `
      fragment booleanArgFrag on ComplicatedArgs {
        booleanArgField(booleanArg: $booleanArg)
      }
      query Query($booleanArg: Boolean)
      {
        complicatedArgs {
          ...booleanArgFrag
        }
      }
    `,
				},
				{
					query: `
      query Query($booleanArg: Boolean)
      {
        complicatedArgs {
          ...booleanArgFrag
        }
      }
      fragment booleanArgFrag on ComplicatedArgs {
        booleanArgField(booleanArg: $booleanArg)
      }
    `,
				},
			},
		},
		{
			name: `Boolean! => Boolean`,
			steps: []portedStep{
				{
					query: `
      query Query($nonNullBooleanArg: Boolean!)
      {
        complicatedArgs {
          booleanArgField(booleanArg: $nonNullBooleanArg)
        }
      }
    `,
				},
			},
		},
		{
			name: `Boolean! => Boolean within fragment`,
			steps: []portedStep{
				{
					query: `
      fragment booleanArgFrag on ComplicatedArgs {
        booleanArgField(booleanArg: $nonNullBooleanArg)
      }

      query Query($nonNullBooleanArg: Boolean!)
      {
        complicatedArgs {
          ...booleanArgFrag
        }
      }
    `,
				},
			},
		},
		{
			name: `[String] => [String]`,
			steps: []portedStep{
				{
					query: `
      query Query($stringListVar: [String])
      {
        complicatedArgs {
          stringListArgField(stringListArg: $stringListVar)
        }
      }
    `,
				},
			},
		},
		{
			name: `[String!] => [String]`,
			steps: []portedStep{
				{
					query: `
      query Query($stringListVar: [String!])
      {
        complicatedArgs {
          stringListArgField(stringListArg: $stringListVar)
        }
      }
    `,
				},
			},
		},
		{
			name: `String => [String] in item position`,
			steps: []portedStep{
				{
					query: `
      query Query($stringVar: String)
      {
        complicatedArgs {
          stringListArgField(stringListArg: [$stringVar])
        }
      }
    `,
				},
			},
		},
		{
			name: `String! => [String] in item position`,
			steps: []portedStep{
				{
					query: `
      query Query($stringVar: String!)
      {
        complicatedArgs {
          stringListArgField(stringListArg: [$stringVar])
        }
      }
    `,
				},
			},
		},
		{
			name: `ComplexInput => ComplexInput`,
			steps: []portedStep{
				{
					query: `
      query Query($complexVar: ComplexInput)
      {
        complicatedArgs {
          complexArgField(complexArg: $complexVar)
        }
      }
    `,
				},
			},
		},
		{
			name: `ComplexInput => ComplexInput in field position`,
			steps: []portedStep{
				{
					query: `
      query Query($boolVar: Boolean = false)
      {
        complicatedArgs {
          complexArgField(complexArg: {requiredArg: $boolVar})
        }
      }
    `,
				},
			},
		},
		{
			name: `Boolean! => Boolean! in directive`,
			steps: []portedStep{
				{
					query: `
      query Query($boolVar: Boolean!)
      {
        dog @include(if: $boolVar)
      }
    `,
				},
			},
		},
		{
			name: `Int => Int!`,
			steps: []portedStep{
				{
					query: `
      query Query($intArg: Int) {
        complicatedArgs {
          nonNullIntArgField(nonNullIntArg: $intArg)
        }
      }
    `,
					want: []want{
						{At: []at{{2, 19}, {4, 45}}},
					},
				},
			},
		},
		{
			name: `Int => Int! within fragment`,
			steps: []portedStep{
				{
					query: `
      fragment nonNullIntArgFieldFrag on ComplicatedArgs {
        nonNullIntArgField(nonNullIntArg: $intArg)
      }

      query Query($intArg: Int) {
        complicatedArgs {
          ...nonNullIntArgFieldFrag
        }
      }
    `,
					want: []want{
						{At: []at{{6, 19}, {3, 43}}},
					},
				},
			},
		},
		{
			name: `Int => Int! within nested fragment`,
			steps: []portedStep{
				{
					query: `
      fragment outerFrag on ComplicatedArgs {
        ...nonNullIntArgFieldFrag
      }

      fragment nonNullIntArgFieldFrag on ComplicatedArgs {
        nonNullIntArgField(nonNullIntArg: $intArg)
      }

      query Query($intArg: Int) {
        complicatedArgs {
          ...outerFrag
        }
      }
    `,
					want: []want{
						{At: []at{{10, 19}, {7, 43}}},
					},
				},
			},
		},
		{
			name: `String over Boolean`,
			steps: []portedStep{
				{
					query: `
      query Query($stringVar: String) {
        complicatedArgs {
          booleanArgField(booleanArg: $stringVar)
        }
      }
    `,
					want: []want{
						{At: []at{{2, 19}, {4, 39}}},
					},
				},
			},
		},
		{
			name: `String => [String]`,
			steps: []portedStep{
				{
					query: `
      query Query($stringVar: String) {
        complicatedArgs {
          stringListArgField(stringListArg: $stringVar)
        }
      }
    `,
					want: []want{
						{At: []at{{2, 19}, {4, 45}}},
					},
				},
			},
		},
		{
			name: `Boolean => Boolean! in directive`,
			steps: []portedStep{
				{
					query: `
      query Query($boolVar: Boolean) {
        dog @include(if: $boolVar)
      }
    `,
					want: []want{
						{At: []at{{2, 19}, {3, 26}}},
					},
				},
			},
		},
		{
			name: `String => Boolean! in directive`,
			steps: []portedStep{
				{
					query: `
      query Query($stringVar: String) {
        dog @include(if: $stringVar)
      }
    `,
					want: []want{
						{At: []at{{2, 19}, {3, 26}}},
					},
				},
			},
		},
		{
			name: `[String] => [String!]`,
			steps: []portedStep{
				{
					query: `
      query Query($stringListVar: [String])
      {
        complicatedArgs {
          stringListNonNullArgField(stringListNonNullArg: $stringListVar)
        }
      }
    `,
					want: []want{
						{At: []at{{2, 19}, {5, 59}}},
					},
				},
			},
		},
		{
			name: `Int => Int! fails when variable provides null default value`,
			steps: []portedStep{
				{
					query: `
        query Query($intVar: Int = null) {
          complicatedArgs {
            nonNullIntArgField(nonNullIntArg: $intVar)
          }
        }
      `,
					want: []want{
						{At: []at{{2, 21}, {4, 47}}},
					},
				},
			},
		},
		{
			name: `Int => Int! when variable provides non-null default value`,
			steps: []portedStep{
				{
					query: `
        query Query($intVar: Int = 1) {
          complicatedArgs {
            nonNullIntArgField(nonNullIntArg: $intVar)
          }
        }`,
				},
			},
		},
		{
			name: `Int => Int! when optional argument provides default value`,
			steps: []portedStep{
				{
					query: `
        query Query($intVar: Int) {
          complicatedArgs {
            nonNullFieldWithDefault(nonNullIntArg: $intVar)
          }
        }`,
				},
			},
		},
		{
			name: `Boolean => Boolean! in directive with default value with option`,
			steps: []portedStep{
				{
					query: `
        query Query($boolVar: Boolean = false) {
          dog @include(if: $boolVar)
        }`,
				},
			},
		},
		{
			name: `undefined in directive with default value with option`,
			steps: []portedStep{
				{
					query: `
        {
          dog @include(if: $x)
        }`,
				},
			},
		},
		{
			name: `Allows exactly one non-nullable variable`,
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
			name: `Forbids one nullable variable`,
			steps: []portedStep{
				{
					query: `
        query ($string: String) {
          complicatedArgs {
            oneOfArgField(oneOfArg: { stringField: $string })
          }
        }
      `,
					want: []want{
						{At: []at{{2, 16}, {4, 52}}},
					},
				},
			},
		},
		{
			name: `validates fragment variables defined before the operation`,
			steps: []portedStep{
				{
					query: `
        fragment A($intVar: Int) on ComplicatedArgs {
          nonNullIntArgField(nonNullIntArg: $intVar)
        }
        query Query($intVar: Int!) {
          complicatedArgs {
            ...A(i: $intVar)
          }
        }
      `,
					want: []want{
						{At: []at{{2, 20}, {3, 45}}},
					},
				},
			},
		},
		{
			name: `Boolean => Boolean (2)`,
			steps: []portedStep{
				{
					query: `
        query Query($booleanArg: Boolean)
        {
          complicatedArgs {
            ...A(b: $booleanArg)
          }
        }
        fragment A($b: Boolean) on ComplicatedArgs {
          booleanArgField(booleanArg: $b)
        }
      `,
				},
			},
		},
		{
			name: `Boolean => Boolean with default value`,
			steps: []portedStep{
				{
					query: `
        query Query($booleanArg: Boolean)
        {
          complicatedArgs {
            ...A(b: $booleanArg)
          }
        }
        fragment A($b: Boolean = true) on ComplicatedArgs {
          booleanArgField(booleanArg: $b)
        }
      `,
				},
			},
		},
		{
			name: `Boolean => Boolean!`,
			steps: []portedStep{
				{
					query: `
        query Query($ab: Boolean)
        {
          complicatedArgs {
            ...A(b: $ab)
          }
        }
        fragment A($b: Boolean!) on ComplicatedArgs {
          booleanArgField(booleanArg: $b)
        }
      `,
					want: []want{
						{At: []at{{2, 21}, {5, 21}}},
					},
				},
			},
		},
		{
			name: `Int => Int! fails when variable provides null default value (2)`,
			steps: []portedStep{
				{
					query: `
        query Query($intVar: Int = null) {
          complicatedArgs {
            ...A(i: $intVar)
          }
        }
        fragment A($i: Int!) on ComplicatedArgs {
          nonNullIntArgField(nonNullIntArg: $i)
        }
      `,
					want: []want{
						{At: []at{{2, 21}, {4, 21}}},
					},
				},
			},
		},
		{
			name: `Int fragment arg => Int! field arg fails even when shadowed by Int! variable`,
			steps: []portedStep{
				{
					query: `
        query Query($intVar: Int!) {
          complicatedArgs {
            ...A(i: $intVar)
          }
        }
        fragment A($intVar: Int) on ComplicatedArgs {
          nonNullIntArgField(nonNullIntArg: $intVar)
        }
      `,
					want: []want{
						{At: []at{{7, 20}, {8, 45}}},
					},
				},
			},
		},
		{
			name: `Allows exactly one non-nullable variable (2)`,
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
			name: `Undefined variable in oneOf input object`,
			steps: []portedStep{
				{
					query: `
      {
        complicatedArgs {
          oneOfArgField(oneOfArg: { stringField: $undefinedVariable })
        }
      }
    `,
				},
			},
		},
		{
			name: `Forbids one nullable variable (2)`,
			steps: []portedStep{
				{
					query: `
      query ($string: String) {
        complicatedArgs {
          oneOfArgField(oneOfArg: { stringField: $string })
        }
      }
    `,
					want: []want{
						{At: []at{{2, 14}, {4, 50}}},
					},
				},
			},
		},
		{
			name: `Allows using variables inside object literal in custom scalar`,
			steps: []portedStep{
				{
					query: `
      query Query($x: Float) {
        dog {
          distanceFrom(loc: {x: $x, y: 10.0})
        }
      }`,
				},
			},
		},
		{
			name: `Allows using variables inside list literal in custom scalar`,
			steps: []portedStep{
				{
					query: `
      query Query($x: Float) {
        dog {
          distanceFrom(loc: [$x, 10.0])
        }
      }`,
				},
			},
		},
	})
}
