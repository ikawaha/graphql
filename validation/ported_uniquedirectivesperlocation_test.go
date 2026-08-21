package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/UniqueDirectivesPerLocationRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_UniqueDirectivesPerLocation(t *testing.T) {
	runPorted(t, validation.UniqueDirectivesPerLocationRule, []portedCase{
		{
			name: `no directives`,
			extendHarness: `
  directive @directive on FIELD | FRAGMENT_DEFINITION
  directive @directiveA on FIELD | FRAGMENT_DEFINITION
  directive @directiveB on FIELD | FRAGMENT_DEFINITION
  directive @repeatable repeatable on FIELD | FRAGMENT_DEFINITION`,
			steps: []portedStep{
				{
					query: `
      fragment Test on Type {
        field
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `unique directives in different locations`,
			extendHarness: `
  directive @directive on FIELD | FRAGMENT_DEFINITION
  directive @directiveA on FIELD | FRAGMENT_DEFINITION
  directive @directiveB on FIELD | FRAGMENT_DEFINITION
  directive @repeatable repeatable on FIELD | FRAGMENT_DEFINITION`,
			steps: []portedStep{
				{
					query: `
      fragment Test on Type @directiveA {
        field @directiveB
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `unique directives in same locations`,
			extendHarness: `
  directive @directive on FIELD | FRAGMENT_DEFINITION
  directive @directiveA on FIELD | FRAGMENT_DEFINITION
  directive @directiveB on FIELD | FRAGMENT_DEFINITION
  directive @repeatable repeatable on FIELD | FRAGMENT_DEFINITION`,
			steps: []portedStep{
				{
					query: `
      fragment Test on Type @directiveA @directiveB {
        field @directiveA @directiveB
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `same directives in different locations`,
			extendHarness: `
  directive @directive on FIELD | FRAGMENT_DEFINITION
  directive @directiveA on FIELD | FRAGMENT_DEFINITION
  directive @directiveB on FIELD | FRAGMENT_DEFINITION
  directive @repeatable repeatable on FIELD | FRAGMENT_DEFINITION`,
			steps: []portedStep{
				{
					query: `
      fragment Test on Type @directiveA {
        field @directiveA
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `same directives in similar locations`,
			extendHarness: `
  directive @directive on FIELD | FRAGMENT_DEFINITION
  directive @directiveA on FIELD | FRAGMENT_DEFINITION
  directive @directiveB on FIELD | FRAGMENT_DEFINITION
  directive @repeatable repeatable on FIELD | FRAGMENT_DEFINITION`,
			steps: []portedStep{
				{
					query: `
      fragment Test on Type {
        field @directive
        field @directive
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `repeatable directives in same location`,
			extendHarness: `
  directive @directive on FIELD | FRAGMENT_DEFINITION
  directive @directiveA on FIELD | FRAGMENT_DEFINITION
  directive @directiveB on FIELD | FRAGMENT_DEFINITION
  directive @repeatable repeatable on FIELD | FRAGMENT_DEFINITION`,
			steps: []portedStep{
				{
					query: `
      fragment Test on Type @repeatable @repeatable {
        field @repeatable @repeatable
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `unknown directives must be ignored`,
			extendHarness: `
  directive @directive on FIELD | FRAGMENT_DEFINITION
  directive @directiveA on FIELD | FRAGMENT_DEFINITION
  directive @directiveB on FIELD | FRAGMENT_DEFINITION
  directive @repeatable repeatable on FIELD | FRAGMENT_DEFINITION`,
			steps: []portedStep{
				{
					query: `
      type Test @unknown @unknown {
        field: String! @unknown @unknown
      }

      extend type Test @unknown {
        anotherField: String!
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `duplicate directives in one location`,
			extendHarness: `
  directive @directive on FIELD | FRAGMENT_DEFINITION
  directive @directiveA on FIELD | FRAGMENT_DEFINITION
  directive @directiveB on FIELD | FRAGMENT_DEFINITION
  directive @repeatable repeatable on FIELD | FRAGMENT_DEFINITION`,
			steps: []portedStep{
				{
					query: `
      fragment Test on Type {
        field @directive @directive
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 15}, {3, 26}}},
					},
				},
			},
		},
		{
			name: `many duplicate directives in one location`,
			extendHarness: `
  directive @directive on FIELD | FRAGMENT_DEFINITION
  directive @directiveA on FIELD | FRAGMENT_DEFINITION
  directive @directiveB on FIELD | FRAGMENT_DEFINITION
  directive @repeatable repeatable on FIELD | FRAGMENT_DEFINITION`,
			steps: []portedStep{
				{
					query: `
      fragment Test on Type {
        field @directive @directive @directive
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 15}, {3, 26}}},
						{At: []at{{3, 15}, {3, 37}}},
					},
				},
			},
		},
		{
			name: `different duplicate directives in one location`,
			extendHarness: `
  directive @directive on FIELD | FRAGMENT_DEFINITION
  directive @directiveA on FIELD | FRAGMENT_DEFINITION
  directive @directiveB on FIELD | FRAGMENT_DEFINITION
  directive @repeatable repeatable on FIELD | FRAGMENT_DEFINITION`,
			steps: []portedStep{
				{
					query: `
      fragment Test on Type {
        field @directiveA @directiveB @directiveA @directiveB
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 15}, {3, 39}}},
						{At: []at{{3, 27}, {3, 51}}},
					},
				},
			},
		},
		{
			name: `duplicate directives in many locations`,
			extendHarness: `
  directive @directive on FIELD | FRAGMENT_DEFINITION
  directive @directiveA on FIELD | FRAGMENT_DEFINITION
  directive @directiveB on FIELD | FRAGMENT_DEFINITION
  directive @repeatable repeatable on FIELD | FRAGMENT_DEFINITION`,
			steps: []portedStep{
				{
					query: `
      fragment Test on Type @directive @directive {
        field @directive @directive
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{2, 29}, {2, 40}}},
						{At: []at{{3, 15}, {3, 26}}},
					},
				},
			},
		},
		{
			name: `duplicate directives on SDL definitions`,
			extendHarness: `
  directive @directive on FIELD | FRAGMENT_DEFINITION
  directive @directiveA on FIELD | FRAGMENT_DEFINITION
  directive @directiveB on FIELD | FRAGMENT_DEFINITION
  directive @repeatable repeatable on FIELD | FRAGMENT_DEFINITION`,
			steps: []portedStep{
				{
					query: `
      directive @nonRepeatable on
        SCHEMA | SCALAR | OBJECT | INTERFACE | UNION | INPUT_OBJECT

      schema @nonRepeatable @nonRepeatable { query: Dummy }

      scalar TestScalar @nonRepeatable @nonRepeatable
      type TestObject @nonRepeatable @nonRepeatable
      interface TestInterface @nonRepeatable @nonRepeatable
      union TestUnion @nonRepeatable @nonRepeatable
      input TestInput @nonRepeatable @nonRepeatable
    `,
					sdl:              true,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{5, 14}, {5, 29}}},
						{At: []at{{7, 25}, {7, 40}}},
						{At: []at{{8, 23}, {8, 38}}},
						{At: []at{{9, 31}, {9, 46}}},
						{At: []at{{10, 23}, {10, 38}}},
						{At: []at{{11, 23}, {11, 38}}},
					},
				},
			},
		},
		{
			name: `duplicate directives on SDL extensions`,
			extendHarness: `
  directive @directive on FIELD | FRAGMENT_DEFINITION
  directive @directiveA on FIELD | FRAGMENT_DEFINITION
  directive @directiveB on FIELD | FRAGMENT_DEFINITION
  directive @repeatable repeatable on FIELD | FRAGMENT_DEFINITION`,
			steps: []portedStep{
				{
					query: `
      directive @nonRepeatable on
        SCHEMA | SCALAR | OBJECT | INTERFACE | UNION | INPUT_OBJECT

      extend schema @nonRepeatable @nonRepeatable

      extend scalar TestScalar @nonRepeatable @nonRepeatable
      extend type TestObject @nonRepeatable @nonRepeatable
      extend interface TestInterface @nonRepeatable @nonRepeatable
      extend union TestUnion @nonRepeatable @nonRepeatable
      extend input TestInput @nonRepeatable @nonRepeatable
    `,
					sdl:              true,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{5, 21}, {5, 36}}},
						{At: []at{{7, 32}, {7, 47}}},
						{At: []at{{8, 30}, {8, 45}}},
						{At: []at{{9, 38}, {9, 53}}},
						{At: []at{{10, 30}, {10, 45}}},
						{At: []at{{11, 30}, {11, 45}}},
					},
				},
			},
		},
		{
			name: `duplicate directives between SDL definitions and extensions`,
			extendHarness: `
  directive @directive on FIELD | FRAGMENT_DEFINITION
  directive @directiveA on FIELD | FRAGMENT_DEFINITION
  directive @directiveB on FIELD | FRAGMENT_DEFINITION
  directive @repeatable repeatable on FIELD | FRAGMENT_DEFINITION`,
			steps: []portedStep{
				{
					query: `
      directive @nonRepeatable on SCHEMA

      schema @nonRepeatable { query: Dummy }
      extend schema @nonRepeatable
    `,
					sdl:              true,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 14}, {5, 21}}},
					},
				},
				{
					query: `
      directive @nonRepeatable on SCALAR

      scalar TestScalar @nonRepeatable
      extend scalar TestScalar @nonRepeatable
      scalar TestScalar @nonRepeatable
    `,
					sdl:              true,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 25}, {5, 32}}},
						{At: []at{{4, 25}, {6, 25}}},
					},
				},
				{
					query: `
      directive @nonRepeatable on OBJECT

      extend type TestObject @nonRepeatable
      type TestObject @nonRepeatable
      extend type TestObject @nonRepeatable
    `,
					sdl:              true,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 30}, {5, 23}}},
						{At: []at{{4, 30}, {6, 30}}},
					},
				},
			},
		},
		{
			name: `duplicate directives on directive definitions`,
			extendHarness: `
  directive @directive on FIELD | FRAGMENT_DEFINITION
  directive @directiveA on FIELD | FRAGMENT_DEFINITION
  directive @directiveB on FIELD | FRAGMENT_DEFINITION
  directive @repeatable repeatable on FIELD | FRAGMENT_DEFINITION`,
			steps: []portedStep{
				{
					query: `
      directive @nonRepeatable on DIRECTIVE_DEFINITION

      directive @testDirective @nonRepeatable @nonRepeatable on FIELD_DEFINITION
    `,
					sdl:              true,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 32}, {4, 47}}},
					},
				},
			},
		},
		{
			name: `duplicate directives on directive extensions`,
			extendHarness: `
  directive @directive on FIELD | FRAGMENT_DEFINITION
  directive @directiveA on FIELD | FRAGMENT_DEFINITION
  directive @directiveB on FIELD | FRAGMENT_DEFINITION
  directive @repeatable repeatable on FIELD | FRAGMENT_DEFINITION`,
			steps: []portedStep{
				{
					query: `
      directive @nonRepeatable on DIRECTIVE_DEFINITION

      extend directive @testDirective @nonRepeatable @nonRepeatable
    `,
					sdl:              true,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 39}, {4, 54}}},
					},
				},
			},
		},
		{
			name: `duplicate directives between directive definitions and extensions`,
			extendHarness: `
  directive @directive on FIELD | FRAGMENT_DEFINITION
  directive @directiveA on FIELD | FRAGMENT_DEFINITION
  directive @directiveB on FIELD | FRAGMENT_DEFINITION
  directive @repeatable repeatable on FIELD | FRAGMENT_DEFINITION`,
			steps: []portedStep{
				{
					query: `
      directive @nonRepeatable on DIRECTIVE_DEFINITION

      directive @testDirective @nonRepeatable on FIELD_DEFINITION
      extend directive @testDirective @nonRepeatable
    `,
					sdl:              true,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 32}, {5, 39}}},
					},
				},
			},
		},
		{
			name: `duplicate directives between directive extensions`,
			extendHarness: `
  directive @directive on FIELD | FRAGMENT_DEFINITION
  directive @directiveA on FIELD | FRAGMENT_DEFINITION
  directive @directiveB on FIELD | FRAGMENT_DEFINITION
  directive @repeatable repeatable on FIELD | FRAGMENT_DEFINITION`,
			steps: []portedStep{
				{
					query: `
      directive @nonRepeatable on DIRECTIVE_DEFINITION

      extend directive @testDirective @nonRepeatable
      extend directive @testDirective @nonRepeatable
    `,
					sdl:              true,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 39}, {5, 39}}},
					},
				},
			},
		},
	})
}
