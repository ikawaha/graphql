package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/UniqueFieldDefinitionNamesRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_UniqueFieldDefinitionNames(t *testing.T) {
	runPorted(t, validation.UniqueFieldDefinitionNamesRule, []portedCase{
		{
			name: `no fields`,
			steps: []portedStep{
				{
					query: `
      type SomeObject
      interface SomeInterface
      input SomeInputObject
    `,
					sdl: true,
				},
			},
		},
		{
			name: `one field`,
			steps: []portedStep{
				{
					query: `
      type SomeObject {
        foo: String
      }

      interface SomeInterface {
        foo: String
      }

      input SomeInputObject {
        foo: String
      }
    `,
					sdl: true,
				},
			},
		},
		{
			name: `multiple fields`,
			steps: []portedStep{
				{
					query: `
      type SomeObject {
        foo: String
        bar: String
      }

      interface SomeInterface {
        foo: String
        bar: String
      }

      input SomeInputObject {
        foo: String
        bar: String
      }
    `,
					sdl: true,
				},
			},
		},
		{
			name: `duplicate fields inside the same type definition`,
			steps: []portedStep{
				{
					query: `
      type SomeObject {
        foo: String
        bar: String
        foo: String
      }

      interface SomeInterface {
        foo: String
        bar: String
        foo: String
      }

      input SomeInputObject {
        foo: String
        bar: String
        foo: String
      }
    `,
					sdl: true,
					want: []want{
						{At: []at{{3, 9}, {5, 9}}},
						{At: []at{{9, 9}, {11, 9}}},
						{At: []at{{15, 9}, {17, 9}}},
					},
				},
			},
		},
		{
			name: `extend type with new field`,
			steps: []portedStep{
				{
					query: `
      type SomeObject {
        foo: String
      }
      extend type SomeObject {
        bar: String
      }
      extend type SomeObject {
        baz: String
      }

      interface SomeInterface {
        foo: String
      }
      extend interface SomeInterface {
        bar: String
      }
      extend interface SomeInterface {
        baz: String
      }

      input SomeInputObject {
        foo: String
      }
      extend input SomeInputObject {
        bar: String
      }
      extend input SomeInputObject {
        baz: String
      }
    `,
					sdl: true,
				},
			},
		},
		{
			name: `extend type with duplicate field`,
			steps: []portedStep{
				{
					query: `
      extend type SomeObject {
        foo: String
      }
      type SomeObject {
        foo: String
      }

      extend interface SomeInterface {
        foo: String
      }
      interface SomeInterface {
        foo: String
      }

      extend input SomeInputObject {
        foo: String
      }
      input SomeInputObject {
        foo: String
      }
    `,
					sdl: true,
					want: []want{
						{At: []at{{3, 9}, {6, 9}}},
						{At: []at{{10, 9}, {13, 9}}},
						{At: []at{{17, 9}, {20, 9}}},
					},
				},
			},
		},
		{
			name: `duplicate field inside extension`,
			steps: []portedStep{
				{
					query: `
      type SomeObject
      extend type SomeObject {
        foo: String
        bar: String
        foo: String
      }

      interface SomeInterface
      extend interface SomeInterface {
        foo: String
        bar: String
        foo: String
      }

      input SomeInputObject
      extend input SomeInputObject {
        foo: String
        bar: String
        foo: String
      }
    `,
					sdl: true,
					want: []want{
						{At: []at{{4, 9}, {6, 9}}},
						{At: []at{{11, 9}, {13, 9}}},
						{At: []at{{18, 9}, {20, 9}}},
					},
				},
			},
		},
		{
			name: `duplicate field inside different extensions`,
			steps: []portedStep{
				{
					query: `
      type SomeObject
      extend type SomeObject {
        foo: String
      }
      extend type SomeObject {
        foo: String
      }

      interface SomeInterface
      extend interface SomeInterface {
        foo: String
      }
      extend interface SomeInterface {
        foo: String
      }

      input SomeInputObject
      extend input SomeInputObject {
        foo: String
      }
      extend input SomeInputObject {
        foo: String
      }
    `,
					sdl: true,
					want: []want{
						{At: []at{{4, 9}, {7, 9}}},
						{At: []at{{12, 9}, {15, 9}}},
						{At: []at{{20, 9}, {23, 9}}},
					},
				},
			},
		},
	})
}

// Not ported, because each of these is written in a way this could not
// follow:
//   - adding new field to the type inside existing schema: a document that is not written out
//   - adding conflicting fields to existing schema twice: a document that is not written out
//   - adding fields to existing schema twice: a document that is not written out
