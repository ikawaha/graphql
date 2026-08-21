package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/ProvidedRequiredArgumentsRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_ProvidedRequiredArguments(t *testing.T) {
	runPorted(t, validation.ProvidedRequiredArgumentsRule, []portedCase{
		{
			name: `ignores unknown arguments`,
			steps: []portedStep{
				{
					query: `
      {
        dog {
          isHouseTrained(unknownArgument: true)
        }
      }
    `,
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
			name: `No arg on non-null field with default`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            nonNullFieldWithDefault
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
			name: `Missing one non-nullable argument`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            multipleReqs(req2: 2)
          }
        }
      `,
					want: []want{
						{At: []at{{4, 13}}},
					},
				},
			},
		},
		{
			name: `Missing multiple non-nullable arguments`,
			steps: []portedStep{
				{
					query: `
        {
          complicatedArgs {
            multipleReqs
          }
        }
      `,
					want: []want{
						{At: []at{{4, 13}}},
						{At: []at{{4, 13}}},
					},
				},
			},
		},
		{
			name: `Incorrect value and missing argument`,
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
						{At: []at{{4, 13}}},
					},
				},
			},
		},
		{
			name: `ignores unknown directives`,
			steps: []portedStep{
				{
					query: `
        {
          dog @unknown
        }
      `,
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
			name: `with directive with missing types`,
			steps: []portedStep{
				{
					query: `
        {
          dog @include {
            name @skip
          }
        }
      `,
					want: []want{
						{At: []at{{3, 15}}},
						{At: []at{{4, 18}}},
					},
				},
			},
		},
		{
			name: `Missing optional args on directive defined inside SDL`,
			steps: []portedStep{
				{
					query: `
        type Query {
          foo: String @test
        }

        directive @test(arg1: String, arg2: String! = "") on FIELD_DEFINITION
      `,
					sdl: true,
				},
			},
		},
		{
			name: `Missing arg on directive defined inside SDL`,
			steps: []portedStep{
				{
					query: `
        type Query {
          foo: String @test
        }

        directive @test(arg: String!) on FIELD_DEFINITION
      `,
					sdl: true,
					want: []want{
						{At: []at{{3, 23}}},
					},
				},
			},
		},
		{
			name: `Missing arg on standard directive`,
			steps: []portedStep{
				{
					query: `
        type Query {
          foo: String @include
        }
      `,
					sdl: true,
					want: []want{
						{At: []at{{3, 23}}},
					},
				},
			},
		},
		{
			name: `Missing arg on overridden standard directive`,
			steps: []portedStep{
				{
					query: `
        type Query {
          foo: String @deprecated
        }
        directive @deprecated(reason: String!) on FIELD
      `,
					sdl: true,
					want: []want{
						{At: []at{{3, 23}}},
					},
				},
			},
		},
		{
			name: `Missing arg on directive defined in schema extension`,
			ownSchema: `
        type Query {
          foo: String
        }
      `,
			steps: []portedStep{
				{
					query: `
          directive @test(arg: String!) on OBJECT

          extend type Query  @test
        `,
					sdl:              true,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 30}}},
					},
				},
			},
		},
		{
			name: `Missing arg on directive used in schema extension`,
			ownSchema: `
        directive @test(arg: String!) on OBJECT

        type Query {
          foo: String
        }
      `,
			steps: []portedStep{
				{
					query: `
          extend type Query @test
        `,
					sdl:              true,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{2, 29}}},
					},
				},
			},
		},
		{
			name: `ignores unknown arguments (2)`,
			steps: []portedStep{
				{
					query: `
        {
          ...Foo(unknownArgument: true)
        }
        fragment Foo on Query {
          dog
        }
      `,
				},
			},
		},
		{
			name: `Missing nullable argument with default is allowed`,
			steps: []portedStep{
				{
					query: `
          {
            ...F
          }
          fragment F($x: Int = 3) on Query {
            foo
          }
        `,
				},
			},
		},
		{
			name: `Missing nullable argument is allowed`,
			steps: []portedStep{
				{
					query: `
          {
            ...F
          }
          fragment F($x: Int) on Query {
            foo
          }
        `,
				},
			},
		},
		{
			name: `Missing non-nullable argument with default is allowed`,
			steps: []portedStep{
				{
					query: `
          {
            ...F
          }
          fragment F($x: Int! = 3) on Query {
            foo
          }
        `,
				},
			},
		},
		{
			name: `Missing non-nullable argument is not allowed`,
			steps: []portedStep{
				{
					query: `
          {
            ...F
          }
          fragment F($x: Int!) on Query {
            foo
          }
        `,
					want: []want{
						{At: []at{{3, 13}}},
					},
				},
			},
		},
		{
			name: `Supplies required variables`,
			steps: []portedStep{
				{
					query: `
          {
            ...F(x: 3)
          }
          fragment F($x: Int!) on Query {
            foo
          }
        `,
				},
			},
		},
		{
			name: `Skips missing fragments`,
			steps: []portedStep{
				{
					query: `
          {
            ...Missing(x: 3)
          }
        `,
				},
			},
		},
	})
}
