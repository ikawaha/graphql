package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/NoSchemaIntrospectionCustomRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_NoSchemaIntrospectionCustom(t *testing.T) {
	runPorted(t, validation.NoSchemaIntrospectionCustomRule, []portedCase{
		{
			name: `ignores valid fields including __typename`,
			ownSchema: `
  type Query {
    someQuery: SomeType
  }

  type SomeType {
    someField: String
    introspectionField: __EnumValue
  }`,
			steps: []portedStep{
				{
					query: `
      {
        someQuery {
          __typename
          someField
        }
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `ignores fields not in the schema`,
			ownSchema: `
  type Query {
    someQuery: SomeType
  }

  type SomeType {
    someField: String
    introspectionField: __EnumValue
  }`,
			steps: []portedStep{
				{
					query: `
      {
        __introspect
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `reports error when a field with an introspection type is requested`,
			ownSchema: `
  type Query {
    someQuery: SomeType
  }

  type SomeType {
    someField: String
    introspectionField: __EnumValue
  }`,
			steps: []portedStep{
				{
					query: `
      {
        __schema {
          queryType {
            name
          }
        }
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 9}}},
						{At: []at{{4, 11}}},
					},
				},
			},
		},
		{
			name: `reports error when a field with an introspection type is requested and aliased`,
			ownSchema: `
  type Query {
    someQuery: SomeType
  }

  type SomeType {
    someField: String
    introspectionField: __EnumValue
  }`,
			steps: []portedStep{
				{
					query: `
      {
        s: __schema {
          queryType {
            name
          }
        }
      }
      `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 9}}},
						{At: []at{{4, 11}}},
					},
				},
			},
		},
		{
			name: `reports error when using a fragment with a field with an introspection type`,
			ownSchema: `
  type Query {
    someQuery: SomeType
  }

  type SomeType {
    someField: String
    introspectionField: __EnumValue
  }`,
			steps: []portedStep{
				{
					query: `
      {
        ...QueryFragment
      }

      fragment QueryFragment on Query {
        __schema {
          queryType {
            name
          }
        }
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{7, 9}}},
						{At: []at{{8, 11}}},
					},
				},
			},
		},
		{
			name: `reports error for non-standard introspection fields`,
			ownSchema: `
  type Query {
    someQuery: SomeType
  }

  type SomeType {
    someField: String
    introspectionField: __EnumValue
  }`,
			steps: []portedStep{
				{
					query: `
      {
        someQuery {
          introspectionField
        }
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 11}}},
					},
				},
			},
		},
	})
}
