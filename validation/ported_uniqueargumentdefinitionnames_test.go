package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/UniqueArgumentDefinitionNamesRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_UniqueArgumentDefinitionNames(t *testing.T) {
	runPorted(t, validation.UniqueArgumentDefinitionNamesRule, []portedCase{
		{
			name: `no args`,
			steps: []portedStep{
				{
					query: `
      type SomeObject {
        someField: String
      }

      interface SomeInterface {
        someField: String
      }

      directive @someDirective on QUERY
    `,
					sdl: true,
				},
			},
		},
		{
			name: `one argument`,
			steps: []portedStep{
				{
					query: `
      type SomeObject {
        someField(foo: String): String
      }

      interface SomeInterface {
        someField(foo: String): String
      }

      extend type SomeObject {
        anotherField(foo: String): String
      }

      extend interface SomeInterface {
        anotherField(foo: String): String
      }

      directive @someDirective(foo: String) on QUERY
    `,
					sdl: true,
				},
			},
		},
		{
			name: `multiple arguments`,
			steps: []portedStep{
				{
					query: `
      type SomeObject {
        someField(
          foo: String
          bar: String
        ): String
      }

      interface SomeInterface {
        someField(
          foo: String
          bar: String
        ): String
      }

      extend type SomeObject {
        anotherField(
          foo: String
          bar: String
        ): String
      }

      extend interface SomeInterface {
        anotherField(
          foo: String
          bar: String
        ): String
      }

      directive @someDirective(
        foo: String
        bar: String
      ) on QUERY
    `,
					sdl: true,
				},
			},
		},
		{
			name: `duplicating arguments`,
			steps: []portedStep{
				{
					query: `
      type SomeObject {
        someField(
          foo: String
          bar: String
          foo: String
        ): String
      }

      interface SomeInterface {
        someField(
          foo: String
          bar: String
          foo: String
        ): String
      }

      extend type SomeObject {
        anotherField(
          foo: String
          bar: String
          bar: String
        ): String
      }

      extend interface SomeInterface {
        anotherField(
          bar: String
          foo: String
          foo: String
        ): String
      }

      directive @someDirective(
        foo: String
        bar: String
        foo: String
      ) on QUERY
    `,
					sdl: true,
					want: []want{
						{At: []at{{4, 11}, {6, 11}}},
						{At: []at{{12, 11}, {14, 11}}},
						{At: []at{{21, 11}, {22, 11}}},
						{At: []at{{29, 11}, {30, 11}}},
						{At: []at{{35, 9}, {37, 9}}},
					},
				},
			},
		},
	})
}
