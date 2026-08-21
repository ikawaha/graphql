package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/NoDeprecatedCustomRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_NoDeprecatedCustom(t *testing.T) {
	runPorted(t, validation.NoDeprecatedCustomRule, []portedCase{
		{
			name: `ignores fields that are not deprecated`,
			ownSchema: `
      type Query {
        normalField: String
        deprecatedField: String @deprecated(reason: "Some field reason.")
      }
    `,
			steps: []portedStep{
				{
					query: `
        {
          normalField
        }
      `,
				},
			},
		},
		{
			name: `ignores unknown fields`,
			ownSchema: `
      type Query {
        normalField: String
        deprecatedField: String @deprecated(reason: "Some field reason.")
      }
    `,
			steps: []portedStep{
				{
					query: `
        {
          unknownField
        }

        fragment UnknownFragment on UnknownType {
          deprecatedField
        }
      `,
				},
			},
		},
		{
			name: `reports error when a deprecated field is selected`,
			ownSchema: `
      type Query {
        normalField: String
        deprecatedField: String @deprecated(reason: "Some field reason.")
      }
    `,
			steps: []portedStep{
				{
					query: `
        {
          deprecatedField
        }

        fragment QueryFragment on Query {
          deprecatedField
        }
      `,
					want: []want{
						{At: []at{{3, 11}}},
						{At: []at{{7, 11}}},
					},
				},
			},
		},
		{
			name: `ignores arguments that are not deprecated`,
			ownSchema: `
      type Query {
        someField(
          normalArg: String,
          deprecatedArg: String @deprecated(reason: "Some arg reason."),
        ): String
      }
    `,
			steps: []portedStep{
				{
					query: `
        {
          normalField(normalArg: "")
        }
      `,
				},
			},
		},
		{
			name: `ignores unknown arguments`,
			ownSchema: `
      type Query {
        someField(
          normalArg: String,
          deprecatedArg: String @deprecated(reason: "Some arg reason."),
        ): String
      }
    `,
			steps: []portedStep{
				{
					query: `
        {
          someField(unknownArg: "")
          unknownField(deprecatedArg: "")
        }
      `,
				},
			},
		},
		{
			name: `reports error when a deprecated argument is used`,
			ownSchema: `
      type Query {
        someField(
          normalArg: String,
          deprecatedArg: String @deprecated(reason: "Some arg reason."),
        ): String
      }
    `,
			steps: []portedStep{
				{
					query: `
        {
          someField(deprecatedArg: "")
        }
      `,
					want: []want{
						{At: []at{{3, 21}}},
					},
				},
			},
		},
		{
			name: `ignores arguments that are not deprecated (2)`,
			ownSchema: `
      type Query {
        someField: String
      }

      directive @someDirective(
        normalArg: String,
        deprecatedArg: String @deprecated(reason: "Some arg reason."),
      ) on FIELD
    `,
			steps: []portedStep{
				{
					query: `
        {
          someField @someDirective(normalArg: "")
        }
      `,
				},
			},
		},
		{
			name: `ignores unknown arguments (2)`,
			ownSchema: `
      type Query {
        someField: String
      }

      directive @someDirective(
        normalArg: String,
        deprecatedArg: String @deprecated(reason: "Some arg reason."),
      ) on FIELD
    `,
			steps: []portedStep{
				{
					query: `
        {
          someField @someDirective(unknownArg: "")
          someField @unknownDirective(deprecatedArg: "")
        }
      `,
				},
			},
		},
		{
			name: `reports error when a deprecated argument is used (2)`,
			ownSchema: `
      type Query {
        someField: String
      }

      directive @someDirective(
        normalArg: String,
        deprecatedArg: String @deprecated(reason: "Some arg reason."),
      ) on FIELD
    `,
			steps: []portedStep{
				{
					query: `
        {
          someField @someDirective(deprecatedArg: "")
        }
      `,
					want: []want{
						{At: []at{{3, 36}}},
					},
				},
			},
		},
		{
			name: `ignores input fields that are not deprecated`,
			ownSchema: `
      input InputType {
        normalField: String
        deprecatedField: String @deprecated(reason: "Some input field reason.")
      }

      type Query {
        someField(someArg: InputType): String
      }

      directive @someDirective(someArg: InputType) on FIELD
    `,
			steps: []portedStep{
				{
					query: `
        {
          someField(
            someArg: { normalField: "" }
          ) @someDirective(someArg: { normalField: "" })
        }
      `,
				},
			},
		},
		{
			name: `ignores unknown input fields`,
			ownSchema: `
      input InputType {
        normalField: String
        deprecatedField: String @deprecated(reason: "Some input field reason.")
      }

      type Query {
        someField(someArg: InputType): String
      }

      directive @someDirective(someArg: InputType) on FIELD
    `,
			steps: []portedStep{
				{
					query: `
        {
          someField(
            someArg: { unknownField: "" }
          )

          someField(
            unknownArg: { unknownField: "" }
          )

          unknownField(
            unknownArg: { unknownField: "" }
          )
        }
      `,
				},
			},
		},
		{
			name: `reports error when a deprecated input field is used`,
			ownSchema: `
      input InputType {
        normalField: String
        deprecatedField: String @deprecated(reason: "Some input field reason.")
      }

      type Query {
        someField(someArg: InputType): String
      }

      directive @someDirective(someArg: InputType) on FIELD
    `,
			steps: []portedStep{
				{
					query: `
        {
          someField(
            someArg: { deprecatedField: "" }
          ) @someDirective(someArg: { deprecatedField: "" })
        }
      `,
					want: []want{
						{At: []at{{4, 24}}},
						{At: []at{{5, 39}}},
					},
				},
			},
		},
		{
			name: `ignores enum values that are not deprecated`,
			ownSchema: `
      enum EnumType {
        NORMAL_VALUE
        DEPRECATED_VALUE @deprecated(reason: "Some enum reason.")
      }

      type Query {
        someField(enumArg: EnumType): String
      }
    `,
			steps: []portedStep{
				{
					query: `
        {
          normalField(enumArg: NORMAL_VALUE)
        }
      `,
				},
			},
		},
		{
			name: `ignores unknown enum values`,
			ownSchema: `
      enum EnumType {
        NORMAL_VALUE
        DEPRECATED_VALUE @deprecated(reason: "Some enum reason.")
      }

      type Query {
        someField(enumArg: EnumType): String
      }
    `,
			steps: []portedStep{
				{
					query: `
        query (
          $unknownValue: EnumType = UNKNOWN_VALUE
          $unknownType: UnknownType = UNKNOWN_VALUE
        ) {
          someField(enumArg: UNKNOWN_VALUE)
          someField(unknownArg: UNKNOWN_VALUE)
          unknownField(unknownArg: UNKNOWN_VALUE)
        }

        fragment SomeFragment on Query {
          someField(enumArg: UNKNOWN_VALUE)
        }
      `,
				},
			},
		},
		{
			name: `reports error when a deprecated enum value is used`,
			ownSchema: `
      enum EnumType {
        NORMAL_VALUE
        DEPRECATED_VALUE @deprecated(reason: "Some enum reason.")
      }

      type Query {
        someField(enumArg: EnumType): String
      }
    `,
			steps: []portedStep{
				{
					query: `
        query (
          $variable: EnumType = DEPRECATED_VALUE
        ) {
          someField(enumArg: DEPRECATED_VALUE)
        }
      `,
					want: []want{
						{At: []at{{3, 33}}},
						{At: []at{{5, 30}}},
					},
				},
			},
		},
	})
}
