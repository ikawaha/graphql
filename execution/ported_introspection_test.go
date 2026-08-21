package execution_test

// Ported from graphql-js src/type/__tests__/introspection-test.ts: what a
// schema says about itself when asked.

import "testing"

// knownIntrospectionDivergences are the cases this implementation does not
// match, and why. Each is asserted to *still* differ, so that closing one
// cannot go unnoticed.
var knownIntrospectionDivergences = map[string]string{}

func TestPortedIntrospection(t *testing.T) {
	runPorted(t, nil, nil, knownIntrospectionDivergences, []portedCase{
		{
			name: "supports the __type root field",
			sdl:  `type Query { someField: String }`,
			query: `{
        __type(name: "Query") {
          name
        }
      }`,
			want: `{"data":{"__type":{"name":"Query"}}}`,
		},
		{
			name: "introspects any default value",
			sdl: `
      input InputObjectWithDefaultValues {
        a: String = "Emoji: \u{1F600}"
        b: Complex = { x: ["abc"], y: 123 }
      }

      input Complex {
        x: [String]
        y: Int
      }

      type Query {
        someField(someArg: InputObjectWithDefaultValues): String
      }`,
			query: `{
        __type(name: "InputObjectWithDefaultValues") {
          inputFields {
            name
            defaultValue
          }
        }
      }`,
			want: `{"data":{"__type":{"inputFields":[` +
				`{"name":"a","defaultValue":"\"Emoji: 😀\""},` +
				`{"name":"b","defaultValue":"{ x: [\"abc\"], y: 123 }"}]}}}`,
		},
		{
			name: "identifies deprecated fields",
			sdl: `
      type Query {
        nonDeprecated: String
        deprecated: String @deprecated(reason: "Removed in 1.0")
        deprecatedWithEmptyReason: String @deprecated(reason: "")
      }`,
			query: `{
        __type(name: "Query") {
          fields(includeDeprecated: true) {
            name
            isDeprecated,
            deprecationReason
          }
        }
      }`,
			want: `{"data":{"__type":{"fields":[` +
				`{"name":"nonDeprecated","isDeprecated":false,"deprecationReason":null},` +
				`{"name":"deprecated","isDeprecated":true,"deprecationReason":"Removed in 1.0"},` +
				`{"name":"deprecatedWithEmptyReason","isDeprecated":true,"deprecationReason":""}]}}}`,
		},
		{
			name: "respects the includeDeprecated parameter for fields",
			sdl: `
      type Query {
        nonDeprecated: String
        deprecated: String @deprecated(reason: "Removed in 1.0")
      }`,
			query: `{
        __type(name: "Query") {
          trueFields: fields(includeDeprecated: true) {
            name
          }
          falseFields: fields(includeDeprecated: false) {
            name
          }
          omittedFields: fields {
            name
          }
        }
      }`,
			want: `{"data":{"__type":{` +
				`"trueFields":[{"name":"nonDeprecated"},{"name":"deprecated"}],` +
				`"falseFields":[{"name":"nonDeprecated"}],` +
				`"omittedFields":[{"name":"nonDeprecated"}]}}}`,
		},
		{
			name: "identifies deprecated args",
			sdl: `
      type Query {
        someField(
          nonDeprecated: String
          deprecated: String @deprecated(reason: "Removed in 1.0")
          deprecatedWithEmptyReason: String @deprecated(reason: "")
        ): String
      }`,
			query: `{
        __type(name: "Query") {
          fields {
            args(includeDeprecated: true) {
              name
              isDeprecated,
              deprecationReason
            }
          }
        }
      }`,
			want: `{"data":{"__type":{"fields":[{"args":[` +
				`{"name":"nonDeprecated","isDeprecated":false,"deprecationReason":null},` +
				`{"name":"deprecated","isDeprecated":true,"deprecationReason":"Removed in 1.0"},` +
				`{"name":"deprecatedWithEmptyReason","isDeprecated":true,"deprecationReason":""}]}]}}}`,
		},
		{
			name: "respects the includeDeprecated parameter for args",
			sdl: `
      type Query {
        someField(
          nonDeprecated: String
          deprecated: String @deprecated(reason: "Removed in 1.0")
        ): String
      }`,
			query: `{
        __type(name: "Query") {
          fields {
            trueArgs: args(includeDeprecated: true) {
              name
            }
            falseArgs: args(includeDeprecated: false) {
              name
            }
            omittedArgs: args {
              name
            }
          }
        }
      }`,
			want: `{"data":{"__type":{"fields":[{` +
				`"trueArgs":[{"name":"nonDeprecated"},{"name":"deprecated"}],` +
				`"falseArgs":[{"name":"nonDeprecated"}],` +
				`"omittedArgs":[{"name":"nonDeprecated"}]}]}}}`,
		},
		{
			name: "identifies deprecated enum values",
			sdl: `
      enum SomeEnum {
        NON_DEPRECATED
        DEPRECATED @deprecated(reason: "Removed in 1.0")
        ALSO_NON_DEPRECATED
      }

      type Query {
        someField(someArg: SomeEnum): String
      }`,
			query: `{
        __type(name: "SomeEnum") {
          enumValues(includeDeprecated: true) {
            name
            isDeprecated,
            deprecationReason
          }
        }
      }`,
			want: `{"data":{"__type":{"enumValues":[` +
				`{"name":"NON_DEPRECATED","isDeprecated":false,"deprecationReason":null},` +
				`{"name":"DEPRECATED","isDeprecated":true,"deprecationReason":"Removed in 1.0"},` +
				`{"name":"ALSO_NON_DEPRECATED","isDeprecated":false,"deprecationReason":null}]}}}`,
		},
		{
			name: "respects the includeDeprecated parameter for enum values",
			sdl: `
      enum SomeEnum {
        NON_DEPRECATED
        DEPRECATED @deprecated(reason: "Removed in 1.0")
        DEPRECATED_WITH_EMPTY_REASON @deprecated(reason: "")
        ALSO_NON_DEPRECATED
      }

      type Query {
        someField(someArg: SomeEnum): String
      }`,
			query: `{
        __type(name: "SomeEnum") {
          trueValues: enumValues(includeDeprecated: true) {
            name
          }
          falseValues: enumValues(includeDeprecated: false) {
            name
          }
          omittedValues: enumValues {
            name
          }
        }
      }`,
			want: `{"data":{"__type":{` +
				`"trueValues":[{"name":"NON_DEPRECATED"},{"name":"DEPRECATED"},` +
				`{"name":"DEPRECATED_WITH_EMPTY_REASON"},{"name":"ALSO_NON_DEPRECATED"}],` +
				`"falseValues":[{"name":"NON_DEPRECATED"},{"name":"ALSO_NON_DEPRECATED"}],` +
				`"omittedValues":[{"name":"NON_DEPRECATED"},{"name":"ALSO_NON_DEPRECATED"}]}}}`,
		},
		{
			name: "identifies deprecated for input fields",
			sdl: `
      input SomeInputObject {
        nonDeprecated: String
        deprecated: String @deprecated(reason: "Removed in 1.0")
        deprecatedWithEmptyReason: String @deprecated(reason: "")
      }

      type Query {
        someField(someArg: SomeInputObject): String
      }`,
			query: `{
        __type(name: "SomeInputObject") {
          inputFields(includeDeprecated: true) {
            name
            isDeprecated,
            deprecationReason
          }
        }
      }`,
			want: `{"data":{"__type":{"inputFields":[` +
				`{"name":"nonDeprecated","isDeprecated":false,"deprecationReason":null},` +
				`{"name":"deprecated","isDeprecated":true,"deprecationReason":"Removed in 1.0"},` +
				`{"name":"deprecatedWithEmptyReason","isDeprecated":true,"deprecationReason":""}]}}}`,
		},
		{
			name: "respects the includeDeprecated parameter for input fields",
			sdl: `
      input SomeInputObject {
        nonDeprecated: String
        deprecated: String @deprecated(reason: "Removed in 1.0")
      }

      type Query {
        someField(someArg: SomeInputObject): String
      }`,
			query: `{
        __type(name: "SomeInputObject") {
          trueFields: inputFields(includeDeprecated: true) {
            name
          }
          falseFields: inputFields(includeDeprecated: false) {
            name
          }
          omittedFields: inputFields {
            name
          }
        }
      }`,
			want: `{"data":{"__type":{` +
				`"trueFields":[{"name":"nonDeprecated"},{"name":"deprecated"}],` +
				`"falseFields":[{"name":"nonDeprecated"}],` +
				`"omittedFields":[{"name":"nonDeprecated"}]}}}`,
		},
		{
			name: "identifies oneOf for input objects",
			sdl: `
      input SomeInputObject @oneOf {
        a: String
      }

      input AnotherInputObject {
        a: String
        b: String
      }

      type Query {
        someField(someArg: SomeInputObject): String
        anotherField(anotherArg: AnotherInputObject): String
      }`,
			query: `{
        oneOfInputObject: __type(name: "SomeInputObject") {
          isOneOf
        }
        inputObject: __type(name: "AnotherInputObject") {
          isOneOf
        }
      }`,
			want: `{"data":{"oneOfInputObject":{"isOneOf":true},"inputObject":{"isOneOf":false}}}`,
		},
		{
			name: "returns null for oneOf for other types",
			sdl: `
      type SomeObject implements SomeInterface {
        fieldA: String
      }
      enum SomeEnum {
        SomeObject
      }
      interface SomeInterface {
        fieldA: String
      }
      union SomeUnion = SomeObject
      type Query {
        someField(enum: SomeEnum): SomeUnion
        anotherField(enum: SomeEnum): SomeInterface
      }`,
			query: `{
        object: __type(name: "SomeObject") {
          isOneOf
        }
        enum: __type(name: "SomeEnum") {
          isOneOf
        }
        interface: __type(name: "SomeInterface") {
          isOneOf
        }
        scalar: __type(name: "String") {
          isOneOf
        }
        union: __type(name: "SomeUnion") {
          isOneOf
        }
      }`,
			want: `{"data":{"object":{"isOneOf":null},"enum":{"isOneOf":null},` +
				`"interface":{"isOneOf":null},"scalar":{"isOneOf":null},"union":{"isOneOf":null}}}`,
		},
		{
			name: "exposes descriptions",
			sdl: `
      """Enum description"""
      enum SomeEnum {
        """Value description"""
        VALUE
      }

      """Object description"""
      type SomeObject {
        """Field description"""
        someField(arg: SomeEnum): String
      }

      """Schema description"""
      schema {
        query: SomeObject
      }`,
			query: `{
        Schema: __schema { description }
        SomeObject: __type(name: "SomeObject") {
          description,
          fields {
            name
            description
          }
        }
        SomeEnum: __type(name: "SomeEnum") {
          description
          enumValues {
            name
            description
          }
        }
      }`,
			want: `{"data":{"Schema":{"description":"Schema description"},` +
				`"SomeObject":{"description":"Object description",` +
				`"fields":[{"name":"someField","description":"Field description"}]},` +
				`"SomeEnum":{"description":"Enum description",` +
				`"enumValues":[{"name":"VALUE","description":"Value description"}]}}}`,
		},
		{
			name: "identifies deprecated directives",
			sdl: `
          type Query {
            someField: String
          }
          directive @isNotDeprecated on FIELD_DEFINITION
          directive @isDeprecated @deprecated(reason: "No longer supported") on FIELD_DEFINITION
          directive @isDeprecatedWithEmptyReason @deprecated(reason: "") on FIELD_DEFINITION`,
			query: `{
        __schema {
          directives(includeDeprecated: true) {
            name
            isDeprecated
            deprecationReason
          }
        }
      }`,
			want: `{"data":{"__schema":{"directives":[` +
				`{"name":"isNotDeprecated","isDeprecated":false,"deprecationReason":null},` +
				`{"name":"isDeprecated","isDeprecated":true,"deprecationReason":"No longer supported"},` +
				`{"name":"isDeprecatedWithEmptyReason","isDeprecated":true,"deprecationReason":""},` +
				`{"name":"include","isDeprecated":false,"deprecationReason":null},` +
				`{"name":"skip","isDeprecated":false,"deprecationReason":null},` +
				`{"name":"deprecated","isDeprecated":false,"deprecationReason":null},` +
				`{"name":"specifiedBy","isDeprecated":false,"deprecationReason":null},` +
				`{"name":"oneOf","isDeprecated":false,"deprecationReason":null}]}}}`,
		},
		{
			name: "respects the includeDeprecated parameter for directives",
			sdl: `
          type Query {
            someField: String
          }
          directive @isNotDeprecated on FIELD_DEFINITION
          directive @isDeprecated @deprecated(reason: "No longer supported") on FIELD_DEFINITION`,
			query: `{
        __schema {
          trueDirectives: directives(includeDeprecated: true) {
            name
          }
          falseDirectives: directives(includeDeprecated: false) {
            name
          }
          omittedDirectives: directives {
            name
          }
        }
      }`,
			want: `{"data":{"__schema":{` +
				`"trueDirectives":[{"name":"isNotDeprecated"},{"name":"isDeprecated"},{"name":"include"},` +
				`{"name":"skip"},{"name":"deprecated"},{"name":"specifiedBy"},{"name":"oneOf"}],` +
				`"falseDirectives":[{"name":"isNotDeprecated"},{"name":"include"},` +
				`{"name":"skip"},{"name":"deprecated"},{"name":"specifiedBy"},{"name":"oneOf"}],` +
				`"omittedDirectives":[{"name":"isNotDeprecated"},{"name":"include"},` +
				`{"name":"skip"},{"name":"deprecated"},{"name":"specifiedBy"},{"name":"oneOf"}]}}}`,
		},
	})
}
