package schema_test

import (
	"testing"

	"github.com/ikawaha/graphql/language"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

// Ported from graphql-js src/type/__tests__/validation-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPortedSchemaValidation(t *testing.T) {
	runPortedSchemaCases(t, []portedSchemaCase{
		{
			name: `accepts a Schema with different root types`,
			sdl: `
      type SomeObject1 {
        field: String
      }

      type SomeObject2 {
        field: String
      }

      type SomeObject3 {
        field: String
      }

      schema {
        query: SomeObject1
        mutation: SomeObject2
        subscription: SomeObject3
      }
    `,
		},
		{
			name: `rejects a Schema where the same type is used for multiple root types`,
			sdl: `
      type SomeObject {
        field: String
      }

      type UniqueObject {
        field: String
      }

      schema {
        query: SomeObject
        mutation: UniqueObject
        subscription: SomeObject
      }
    `,
			want: []schemaWant{
				{At: []at{{11, 16}, {13, 23}}},
			},
		},
		{
			name: `rejects a Schema where the same type is used for all root types`,
			sdl: `
      type SomeObject {
        field: String
      }

      schema {
        query: SomeObject
        mutation: SomeObject
        subscription: SomeObject
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 16}, {8, 19}, {9, 23}}},
			},
		},
		{
			name: `accepts an Object type with fields object`,
			sdl: `
      type Query {
        field: SomeObject
      }

      type SomeObject {
        field: String
      }
    `,
		},
		{
			name: `accepts a Union type with member types`,
			sdl: `
      type Query {
        test: GoodUnion
      }

      type TypeA {
        field: String
      }

      type TypeB {
        field: String
      }

      union GoodUnion =
        | TypeA
        | TypeB
    `,
		},
		{
			name: `rejects a Union type with non-Object members types with malformed AST`,
			sdl: `
      type Query {
        test: BadUnion
      }

      type TypeA {
        field: String
      }

      type TypeB {
        field: String
      }

      union BadUnion =
        | TypeA
        | String
        | TypeB
    `,
			want: []schemaWant{
				{At: []at{}},
			},
			// graphql-js takes the nodes away before checking, so its
			// complaint points nowhere. Reproduced here the same way.
			mangle: func(s *schema.Schema) {
				s.Type("BadUnion").(*schema.UnionType).ASTNode.Types = nil
			},
		},
		{
			name: `accepts an Input Object type with fields`,
			sdl: `
      type Query {
        field(arg: SomeInputObject): String
      }

      input SomeInputObject {
        field: String
      }
    `,
		},
		{
			name: `accepts an Input Object with breakable circular reference`,
			sdl: `
      type Query {
        field(arg: SomeInputObject): String
      }

      input SomeInputObject {
        self: SomeInputObject
        arrayOfSelf: [SomeInputObject]
        nonNullArrayOfSelf: [SomeInputObject]!
        nonNullArrayOfNonNullSelf: [SomeInputObject!]!
        intermediateSelf: AnotherInputObject
      }

      input AnotherInputObject {
        parent: SomeInputObject
      }
    `,
		},
		{
			name: `rejects an Input Object with non-breakable circular reference`,
			sdl: `
      type Query {
        field(arg: SomeInputObject): String
      }

      input SomeInputObject {
        nonNullSelf: SomeInputObject!
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 9}}},
			},
		},
		{
			name: `rejects Input Objects with non-breakable circular reference spread across them`,
			sdl: `
      type Query {
        field(arg: SomeInputObject): String
      }

      input SomeInputObject {
        startLoop: AnotherInputObject!
      }

      input AnotherInputObject {
        nextInLoop: YetAnotherInputObject!
      }

      input YetAnotherInputObject {
        closeLoop: SomeInputObject!
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 9}, {11, 9}, {15, 9}}},
			},
		},
		{
			name: `rejects Input Objects with multiple non-breakable circular reference`,
			sdl: `
      type Query {
        field(arg: SomeInputObject): String
      }

      input SomeInputObject {
        startLoop: AnotherInputObject!
      }

      input AnotherInputObject {
        closeLoop: SomeInputObject!
        startSecondLoop: YetAnotherInputObject!
      }

      input YetAnotherInputObject {
        closeSecondLoop: AnotherInputObject!
        nonNullSelf: YetAnotherInputObject!
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 9}, {11, 9}}},
				{At: []at{{12, 9}, {16, 9}}},
				{At: []at{{17, 9}}},
			},
		},
		{
			name: `rejects an Input Object with multiple non-breakable circular references`,
			sdl: `
      type Query {
        field(arg: A): String
      }

      input A {
        b: B!
        c: C!
      }

      input B {
        a: A!
      }

      input C {
        a: A!
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 9}, {12, 9}}},
				{At: []at{{8, 9}, {16, 9}}},
			},
		},
		{
			name: `rejects Input Objects with default value circular reference (SDL)`,
			sdl: `
      type Query {
        field(arg1: A, arg2: B, arg3: C, arg4: D, arg5: E): String
      }

      input A {
        x: A = {}
      }

      input B {
        x: B2 = {}
      }

      input B2 {
        x: B3 = {}
      }

      input B3 {
        x: B = {}
      }

      input C {
        x: [C] = [{}]
      }

      input D {
        x: D = { x: { x: {} } }
      }

      input E {
        x: E = { x: null }
        y: E = { y: null }
      }

      input F {
        x: F2! = {}
      }

      input F2 {
        x: F = { x: {} }
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 16}}},
				{At: []at{{11, 17}, {15, 17}, {19, 16}}},
				{At: []at{{23, 18}}},
				{At: []at{{27, 16}}},
				{At: []at{{31, 16}, {32, 16}}},
				{At: []at{{40, 16}}},
			},
		},
		{
			name: `rejects an Input Object type with incorrectly typed fields`,
			sdl: `
      type Query {
        field(arg: SomeInputObject): String
      }

      type SomeObject {
        field: String
      }

      union SomeUnion = SomeObject

      input SomeInputObject {
        badObject: SomeObject
        badUnion: SomeUnion
        goodInputObject: SomeInputObject
      }
    `,
			want: []schemaWant{
				{At: []at{{13, 20}}},
				{At: []at{{14, 19}}},
			},
		},
		{
			name: `rejects an Input Object type with required field that is deprecated`,
			sdl: `
      type Query {
        field(arg: SomeInputObject): String
      }

      input SomeInputObject {
        badField: String! @deprecated
        optionalField: String @deprecated
        anotherOptionalField: String! = "" @deprecated
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 27}, {7, 19}}},
			},
		},
		{
			name: `rejects with relevant locations for a non-output type as an Object field type`,
			sdl: `
      type Query {
        field: [SomeInputObject]
      }

      input SomeInputObject {
        field: String
      }
    `,
			want: []schemaWant{
				{At: []at{{3, 16}}},
			},
		},
		{
			name: `rejects an Object implementing a non-Interface type`,
			sdl: `
      type Query {
        test: BadObject
      }

      input SomeInputObject {
        field: String
      }

      type BadObject implements SomeInputObject {
        field: String
      }
    `,
			want: []schemaWant{
				{At: []at{{10, 33}}},
			},
		},
		{
			name: `rejects an Object implementing a non-Interface type with malformed AST`,
			sdl: `
      type Query {
        test: BadObject
      }

      input SomeInputObject {
        field: String
      }

      type BadObject implements SomeInputObject {
        field: String
      }
    `,
			want: []schemaWant{
				{At: []at{}},
			},
			// graphql-js takes the nodes away before checking, so its
			// complaint points nowhere. Reproduced here the same way.
			mangle: func(s *schema.Schema) {
				s.Type("BadObject").(*schema.ObjectType).ASTNode.Interfaces = nil
			},
		},
		{
			name: `rejects an Object implementing the same interface twice`,
			sdl: `
      type Query {
        test: AnotherObject
      }

      interface AnotherInterface {
        field: String
      }

      type AnotherObject implements AnotherInterface & AnotherInterface {
        field: String
      }
    `,
			want: []schemaWant{
				{At: []at{{10, 37}, {10, 56}}},
			},
		},
		{
			name: `rejects a non-output type as an Interface field type with locations`,
			sdl: `
      type Query {
        test: SomeInterface
      }

      interface SomeInterface {
        field: SomeInputObject
      }

      input SomeInputObject {
        foo: String
      }

      type SomeObject implements SomeInterface {
        field: SomeInputObject
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 16}}},
				{At: []at{{15, 16}}},
			},
		},
		{
			name: `accepts an interface not implemented by at least one object`,
			sdl: `
      type Query {
        test: SomeInterface
      }

      interface SomeInterface {
        foo: String
      }
    `,
		},
		{
			name: `rejects a required argument that is deprecated`,
			sdl: `
      directive @BadDirective(
        badArg: String! @deprecated
        optionalArg: String @deprecated
        anotherOptionalArg: String! = "" @deprecated
      ) on FIELD

      type Query {
        test(
          badArg: String! @deprecated
          optionalArg: String @deprecated
          anotherOptionalArg: String! = "" @deprecated
        ): String
      }
    `,
			want: []schemaWant{
				{At: []at{{3, 25}, {3, 17}}},
				{At: []at{{10, 27}, {10, 19}}},
			},
		},
		{
			name: `rejects a non-input type as a field arg with locations`,
			sdl: `
      type Query {
        test(arg: SomeObject): String
      }

      type SomeObject {
        foo: String
      }
    `,
			want: []schemaWant{
				{At: []at{{3, 19}}},
			},
		},
		{
			name: `rejects an argument with invalid default values (SDL)`,
			sdl: `
      type Query {
        field(arg: Int = 3.14): Int
      }

      directive @bad(arg: Int = 2.718) on FIELD
    `,
			want: []schemaWant{
				{At: []at{{6, 33}}},
				{At: []at{{3, 26}}},
			},
		},
		{
			name: `rejects a non-input type as an input object field with locations`,
			sdl: `
      type Query {
        test(arg: SomeInputObject): String
      }

      input SomeInputObject {
        foo: SomeObject
      }

      type SomeObject {
        bar: String
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 14}}},
			},
		},
		{
			name: `rejects an Input Object field with invalid default values (SDL)`,
			sdl: `
    type Query {
      field(arg: SomeInputObject): Int
    }

    input SomeInputObject {
      field: Int = 3.14
    }
  `,
			want: []schemaWant{
				{At: []at{{7, 20}}},
			},
		},
		{
			name: `accepts a OneOf Input Object with a scalar field`,
			sdl: `
      type Query {
        test(arg: A): Int
      }

      input A @oneOf {
        a: Int
      }
    `,
		},
		{
			name: `accepts a OneOf Input Object with a recursive list field`,
			sdl: `
      type Query {
        test(arg: A): Int
      }

      input A @oneOf {
        a: [A!]
      }
    `,
		},
		{
			name: `accepts a OneOf Input Object referencing a non-OneOf input object`,
			sdl: `
      type Query {
        test(arg: A): Int
      }

      input A @oneOf {
        b: B
      }

      input B {
        x: Int
      }
    `,
		},
		{
			name: `accepts a OneOf Input Object referencing an already checked input object`,
			sdl: `
      type Query {
        a(arg: A): Int
      }

      input B {
        value: Int
      }

      input A @oneOf {
        b: B
      }
    `,
		},
		{
			name: `accepts a OneOf Input Object with multiple acyclic input object fields`,
			sdl: `
      type Query {
        test(arg: A): Int
      }

      input A @oneOf {
        b: B
        c: C
      }

      input B {
        value: Int
      }

      input C {
        value: Int
      }
    `,
		},
		{
			name: `accepts a OneOf/OneOf cycle with a scalar escape`,
			sdl: `
      type Query {
        test(arg: A): Int
      }

      input A @oneOf {
        b: B
        escape: Int
      }

      input B @oneOf {
        a: A
      }
    `,
		},
		{
			name: `accepts a OneOf/non-OneOf cycle with a nullable escape`,
			sdl: `
      type Query {
        test(arg: A): Int
      }

      input A @oneOf {
        b: B
      }

      input B {
        a: A
      }
    `,
		},
		{
			name: `accepts a OneOf/non-OneOf with scalar escape`,
			sdl: `
      type Query {
        test(arg: A): Int
      }

      input A @oneOf {
        b: B
        escape: Int
      }

      input B {
        a: A!
      }
    `,
		},
		{
			name: `accepts a non-OneOf/non-OneOf cycle with a nullable escape`,
			sdl: `
      type Query {
        test(arg: A): Int
      }

      input A {
        b: B!
      }

      input B {
        a: A
      }
    `,
		},
		{
			name: `accepts a non-OneOf/non-OneOf cycle with a non-null list of non-null items escape`,
			sdl: `
      type Query {
        test(arg: A): Int
      }

      input A {
        b: [B!]!
      }

      input B {
        a: A!
      }
    `,
		},
		{
			name: `rejects non-nullable fields`,
			sdl: `
      type Query {
        test(arg: SomeInputObject): String
      }

      input SomeInputObject @oneOf {
        a: String
        b: String!
      }
    `,
			want: []schemaWant{
				{At: []at{{8, 12}}},
			},
		},
		{
			name: `rejects fields with default values`,
			sdl: `
      type Query {
        test(arg: SomeInputObject): String
      }

      input SomeInputObject @oneOf {
        a: String
        b: String = "foo"
      }
    `,
			want: []schemaWant{
				{At: []at{{8, 9}}},
			},
		},
		{
			name: `rejects a self-referencing OneOf type with no escapes`,
			sdl: `
      type Query {
        test(arg: A): Int
      }

      input A @oneOf {
        self: A
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 9}}},
			},
		},
		{
			name: `rejects a non-OneOf Input Object requiring an unbreakable OneOf cycle`,
			sdl: `
      type Query {
        a(arg: A): Int
      }

      input T @oneOf {
        self: T
      }

      input A {
        t: T!
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 9}}},
			},
		},
		{
			name: `rejects a mixed OneOf/non-OneOf cycle with no escapes`,
			sdl: `
      type Query {
        test(arg: A): Int
      }

      input A @oneOf {
        b: B
      }

      input B {
        a: A!
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 9}, {11, 9}}},
			},
		},
		{
			name: `rejects multiple OneOf branches without duplicate cycle reports`,
			sdl: `
      type Query {
        test(arg: A): Int
      }

      input A @oneOf {
        b: B
        c: C
      }

      input B {
        a: A!
      }

      input C {
        a: A!
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 9}, {12, 9}}},
				{At: []at{{8, 9}, {16, 9}}},
			},
		},
		{
			name: `rejects a non-OneOf/non-OneOf cycle with required scalar, list, and finite input fields`,
			sdl: `
      type Query {
        test(arg: A): Int
      }

      input A {
        list: [B]!
        finite: Finite!
        b: B!
      }

      input B {
        value: Int!
        a: A!
      }

      input Finite {
        value: Int!
      }
    `,
			want: []schemaWant{
				{At: []at{{9, 9}, {14, 9}}},
			},
		},
		{
			name: `rejects a larger mixed OneOf/non-OneOf cycle with no escapes`,
			sdl: `
      type Query {
        test(arg: A): Int
      }

      input A @oneOf {
        b: B
      }

      input B {
        c: C!
      }

      input C @oneOf {
        a: A
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 9}, {11, 9}, {15, 9}}},
			},
		},
		{
			name: `accepts an Object which implements an Interface`,
			sdl: `
      type Query {
        test: AnotherObject
      }

      interface AnotherInterface {
        field(input: String): String
      }

      type AnotherObject implements AnotherInterface {
        field(input: String): String
      }
    `,
		},
		{
			name: `accepts an Object which implements an Interface along with more fields`,
			sdl: `
      type Query {
        test: AnotherObject
      }

      interface AnotherInterface {
        field(input: String): String
      }

      type AnotherObject implements AnotherInterface {
        field(input: String): String
        anotherField: String
      }
    `,
		},
		{
			name: `accepts an Object which implements an Interface field along with additional optional arguments`,
			sdl: `
      type Query {
        test: AnotherObject
      }

      interface AnotherInterface {
        field(input: String): String
      }

      type AnotherObject implements AnotherInterface {
        field(input: String, anotherInput: String): String
      }
    `,
		},
		{
			name: `rejects an Object missing an Interface field`,
			sdl: `
      type Query {
        test: AnotherObject
      }

      interface AnotherInterface {
        field(input: String): String
      }

      type AnotherObject implements AnotherInterface {
        anotherField: String
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 9}, {10, 7}}},
			},
		},
		{
			name: `rejects an Object with an incorrectly typed Interface field`,
			sdl: `
      type Query {
        test: AnotherObject
      }

      interface AnotherInterface {
        field(input: String): String
      }

      type AnotherObject implements AnotherInterface {
        field(input: String): Int
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 31}, {11, 31}}},
			},
		},
		{
			name: `rejects an Object with a differently typed Interface field`,
			sdl: `
      type Query {
        test: AnotherObject
      }

      type A { foo: String }
      type B { foo: String }

      interface AnotherInterface {
        field: A
      }

      type AnotherObject implements AnotherInterface {
        field: B
      }
    `,
			want: []schemaWant{
				{At: []at{{10, 16}, {14, 16}}},
			},
		},
		{
			name: `accepts an Object with a subtyped Interface field (interface)`,
			sdl: `
      type Query {
        test: AnotherObject
      }

      interface AnotherInterface {
        field: AnotherInterface
      }

      type AnotherObject implements AnotherInterface {
        field: AnotherObject
      }
    `,
		},
		{
			name: `accepts an Object with a subtyped Interface field (union)`,
			sdl: `
      type Query {
        test: AnotherObject
      }

      type SomeObject {
        field: String
      }

      union SomeUnionType = SomeObject

      interface AnotherInterface {
        field: SomeUnionType
      }

      type AnotherObject implements AnotherInterface {
        field: SomeObject
      }
    `,
		},
		{
			name: `rejects an Object missing an Interface argument`,
			sdl: `
      type Query {
        test: AnotherObject
      }

      interface AnotherInterface {
        field(input: String): String
      }

      type AnotherObject implements AnotherInterface {
        field: String
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 15}, {11, 9}}},
			},
		},
		{
			name: `rejects an Object with an incorrectly typed Interface argument`,
			sdl: `
      type Query {
        test: AnotherObject
      }

      interface AnotherInterface {
        field(input: String): String
      }

      type AnotherObject implements AnotherInterface {
        field(input: Int): String
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 22}, {11, 22}}},
			},
		},
		{
			name: `rejects an Object with both an incorrectly typed field and argument`,
			sdl: `
      type Query {
        test: AnotherObject
      }

      interface AnotherInterface {
        field(input: String): String
      }

      type AnotherObject implements AnotherInterface {
        field(input: Int): Int
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 31}, {11, 28}}},
				{At: []at{{7, 22}, {11, 22}}},
			},
		},
		{
			name: `rejects an Object which implements an Interface field along with additional required arguments`,
			sdl: `
      type Query {
        test: AnotherObject
      }

      interface AnotherInterface {
        field(baseArg: String): String
      }

      type AnotherObject implements AnotherInterface {
        field(
          baseArg: String,
          requiredArg: String!
          optionalArg1: String,
          optionalArg2: String = "",
        ): String
      }
    `,
			want: []schemaWant{
				{At: []at{{13, 11}, {7, 9}}},
			},
		},
		{
			name: `accepts an Object with an equivalently wrapped Interface field type`,
			sdl: `
      type Query {
        test: AnotherObject
      }

      interface AnotherInterface {
        field: [String]!
      }

      type AnotherObject implements AnotherInterface {
        field: [String]!
      }
    `,
		},
		{
			name: `rejects an Object with a non-list Interface field list type`,
			sdl: `
      type Query {
        test: AnotherObject
      }

      interface AnotherInterface {
        field: [String]
      }

      type AnotherObject implements AnotherInterface {
        field: String
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 16}, {11, 16}}},
			},
		},
		{
			name: `rejects an Object with a list Interface field non-list type`,
			sdl: `
      type Query {
        test: AnotherObject
      }

      interface AnotherInterface {
        field: String
      }

      type AnotherObject implements AnotherInterface {
        field: [String]
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 16}, {11, 16}}},
			},
		},
		{
			name: `accepts an Object with a subset non-null Interface field type`,
			sdl: `
      type Query {
        test: AnotherObject
      }

      interface AnotherInterface {
        field: String
      }

      type AnotherObject implements AnotherInterface {
        field: String!
      }
    `,
		},
		{
			name: `rejects an Object with a superset nullable Interface field type`,
			sdl: `
      type Query {
        test: AnotherObject
      }

      interface AnotherInterface {
        field: String!
      }

      type AnotherObject implements AnotherInterface {
        field: String
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 16}, {11, 16}}},
			},
		},
		{
			name: `rejects an Object missing a transitive interface`,
			sdl: `
      type Query {
        test: AnotherObject
      }

      interface SuperInterface {
        field: String!
      }

      interface AnotherInterface implements SuperInterface {
        field: String!
      }

      type AnotherObject implements AnotherInterface {
        field: String!
      }
    `,
			want: []schemaWant{
				{At: []at{{10, 45}, {14, 37}}},
			},
		},
		{
			name: `accepts an Interface which implements an Interface`,
			sdl: `
      type Query {
        test: ChildInterface
      }

      interface ParentInterface {
        field(input: String): String
      }

      interface ChildInterface implements ParentInterface {
        field(input: String): String
      }
    `,
		},
		{
			name: `accepts an Interface which implements an Interface along with more fields`,
			sdl: `
      type Query {
        test: ChildInterface
      }

      interface ParentInterface {
        field(input: String): String
      }

      interface ChildInterface implements ParentInterface {
        field(input: String): String
        anotherField: String
      }
    `,
		},
		{
			name: `accepts an Interface which implements an Interface field along with additional optional arguments`,
			sdl: `
      type Query {
        test: ChildInterface
      }

      interface ParentInterface {
        field(input: String): String
      }

      interface ChildInterface implements ParentInterface {
        field(input: String, anotherInput: String): String
      }
    `,
		},
		{
			name: `rejects an Interface missing an Interface field`,
			sdl: `
      type Query {
        test: ChildInterface
      }

      interface ParentInterface {
        field(input: String): String
      }

      interface ChildInterface implements ParentInterface {
        anotherField: String
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 9}, {10, 7}}},
			},
		},
		{
			name: `rejects an Interface with an incorrectly typed Interface field`,
			sdl: `
      type Query {
        test: ChildInterface
      }

      interface ParentInterface {
        field(input: String): String
      }

      interface ChildInterface implements ParentInterface {
        field(input: String): Int
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 31}, {11, 31}}},
			},
		},
		{
			name: `rejects an Interface with a differently typed Interface field`,
			sdl: `
      type Query {
        test: ChildInterface
      }

      type A { foo: String }
      type B { foo: String }

      interface ParentInterface {
        field: A
      }

      interface ChildInterface implements ParentInterface {
        field: B
      }
    `,
			want: []schemaWant{
				{At: []at{{10, 16}, {14, 16}}},
			},
		},
		{
			name: `accepts an Interface with a subtyped Interface field (interface)`,
			sdl: `
      type Query {
        test: ChildInterface
      }

      interface ParentInterface {
        field: ParentInterface
      }

      interface ChildInterface implements ParentInterface {
        field: ChildInterface
      }
    `,
		},
		{
			name: `accepts an Interface with a subtyped Interface field (union)`,
			sdl: `
      type Query {
        test: ChildInterface
      }

      type SomeObject {
        field: String
      }

      union SomeUnionType = SomeObject

      interface ParentInterface {
        field: SomeUnionType
      }

      interface ChildInterface implements ParentInterface {
        field: SomeObject
      }
    `,
		},
		{
			name: `rejects an Interface implementing a non-Interface type`,
			sdl: `
      type Query {
        field: String
      }

      input SomeInputObject {
        field: String
      }

      interface BadInterface implements SomeInputObject {
        field: String
      }
    `,
			want: []schemaWant{
				{At: []at{{10, 41}}},
			},
		},
		{
			name: `rejects an Interface missing an Interface argument`,
			sdl: `
      type Query {
        test: ChildInterface
      }

      interface ParentInterface {
        field(input: String): String
      }

      interface ChildInterface implements ParentInterface {
        field: String
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 15}, {11, 9}}},
			},
		},
		{
			name: `rejects an Interface with an incorrectly typed Interface argument`,
			sdl: `
      type Query {
        test: ChildInterface
      }

      interface ParentInterface {
        field(input: String): String
      }

      interface ChildInterface implements ParentInterface {
        field(input: Int): String
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 22}, {11, 22}}},
			},
		},
		{
			name: `rejects an Interface with both an incorrectly typed field and argument`,
			sdl: `
      type Query {
        test: ChildInterface
      }

      interface ParentInterface {
        field(input: String): String
      }

      interface ChildInterface implements ParentInterface {
        field(input: Int): Int
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 31}, {11, 28}}},
				{At: []at{{7, 22}, {11, 22}}},
			},
		},
		{
			name: `rejects an Interface which implements an Interface field along with additional required arguments`,
			sdl: `
      type Query {
        test: ChildInterface
      }

      interface ParentInterface {
        field(baseArg: String): String
      }

      interface ChildInterface implements ParentInterface {
        field(
          baseArg: String,
          requiredArg: String!
          optionalArg1: String,
          optionalArg2: String = "",
        ): String
      }
    `,
			want: []schemaWant{
				{At: []at{{13, 11}, {7, 9}}},
			},
		},
		{
			name: `accepts an Interface with an equivalently wrapped Interface field type`,
			sdl: `
      type Query {
        test: ChildInterface
      }

      interface ParentInterface {
        field: [String]!
      }

      interface ChildInterface implements ParentInterface {
        field: [String]!
      }
    `,
		},
		{
			name: `rejects an Interface with a non-list Interface field list type`,
			sdl: `
      type Query {
        test: ChildInterface
      }

      interface ParentInterface {
        field: [String]
      }

      interface ChildInterface implements ParentInterface {
        field: String
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 16}, {11, 16}}},
			},
		},
		{
			name: `rejects an Interface with a list Interface field non-list type`,
			sdl: `
      type Query {
        test: ChildInterface
      }

      interface ParentInterface {
        field: String
      }

      interface ChildInterface implements ParentInterface {
        field: [String]
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 16}, {11, 16}}},
			},
		},
		{
			name: `accepts an Interface with a subset non-null Interface field type`,
			sdl: `
      type Query {
        test: ChildInterface
      }

      interface ParentInterface {
        field: String
      }

      interface ChildInterface implements ParentInterface {
        field: String!
      }
    `,
		},
		{
			name: `rejects an Interface with a superset nullable Interface field type`,
			sdl: `
      type Query {
        test: ChildInterface
      }

      interface ParentInterface {
        field: String!
      }

      interface ChildInterface implements ParentInterface {
        field: String
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 16}, {11, 16}}},
			},
		},
		{
			name: `rejects an Object missing a transitive interface (2)`,
			sdl: `
      type Query {
        test: ChildInterface
      }

      interface SuperInterface {
        field: String!
      }

      interface ParentInterface implements SuperInterface {
        field: String!
      }

      interface ChildInterface implements ParentInterface {
        field: String!
      }
    `,
			want: []schemaWant{
				{At: []at{{10, 44}, {14, 43}}},
			},
		},
		{
			name: `rejects a self reference interface`,
			sdl: `
      type Query {
        test: FooInterface
      }

      interface FooInterface implements FooInterface {
        field: String
      }
    `,
			want: []schemaWant{
				{At: []at{{6, 41}}},
			},
		},
		{
			name: `rejects a circular Interface implementation`,
			sdl: `
      type Query {
        test: FooInterface
      }

      interface FooInterface implements BarInterface {
        field: String
      }

      interface BarInterface implements FooInterface {
        field: String
      }
    `,
			want: []schemaWant{
				{At: []at{{10, 41}, {6, 41}}},
				{At: []at{{6, 41}, {10, 41}}},
			},
		},
		{
			name: `rejects deprecated implementation field when interface field is not deprecated`,
			sdl: `
      interface Node {
        id: ID!
      }

      type Foo implements Node {
        id: ID! @deprecated
      }

      type Query {
        foo: Foo
      }
    `,
			want: []schemaWant{
				{At: []at{{7, 17}, {7, 13}}},
			},
		},
	})
}

// Not ported, because each of these is written in a way this could not
// follow:
//   - accepts a Schema whose query type is an object type: the schema is not built from one piece of SDL
//   - accepts a Schema whose query and mutation types are object types: the schema is not built from one piece of SDL
//   - accepts a Schema whose query and subscription types are object types: the schema is not built from one piece of SDL
//   - rejects a Schema without a query type: the schema is not built from one piece of SDL
//   - rejects a Schema whose query root type is not an Object type: the schema is not built from one piece of SDL
//   - rejects a Schema whose mutation type is an input type: the schema is not built from one piece of SDL
//   - rejects a Schema whose subscription type is an input type: the schema is not built from one piece of SDL
//   - rejects a Schema extended with invalid root types: the schema is not built from one piece of SDL
//   - rejects a Schema whose types are incorrectly typed: the schema is built with the object API
//   - rejects a Schema whose directives are incorrectly typed: the schema is built with the object API
//   - rejects a Schema whose directives have empty locations: the schema is built with the object API
//   - rejects an Object type with missing fields: the schema is built with the object API
//   - rejects an Object type with incorrectly named fields: the schema is built with the object API
//   - accepts field args with valid names: the schema is built with the object API
//   - rejects field arg with invalid names: the schema is built with the object API
//   - rejects a Union type with empty types: the schema is not built from one piece of SDL
//   - rejects a Union type with duplicated member type: it does not check validateSchema exactly once
//   - rejects a Union type with non-Object members types: the schema is built with the object API
//   - rejects an Input Object type with missing fields: the schema is not built from one piece of SDL
//   - accepts Input Objects with default values without circular references (SDL): it does not check validateSchema exactly once
//   - accepts Input Objects with default values without circular references (programmatic): the schema is built with the object API
//   - rejects Input Objects with default value circular reference (programmatic): the schema is built with the object API
//   - rejects an Enum type without values: the schema is not built from one piece of SDL
//   - rejects an Enum type with incorrectly named values: the schema is built with the object API
//   - rejects an empty Object field type: the schema is not built from one piece of SDL
//   - rejects a non-type value as an Object field type: the schema is not built from one piece of SDL
//   - rejects an Object implementing a non-type value: the schema is built with the object API
//   - rejects an Object implementing the extended interface due to missing field: the schema is not built from one piece of SDL
//   - rejects an Object implementing the extended interface due to missing field args: the schema is not built from one piece of SDL
//   - rejects Objects implementing the extended interface due to mismatching interface type: the schema is not built from one piece of SDL
//   - rejects an empty Interface field type: the schema is not built from one piece of SDL
//   - rejects a non-type value as an Interface field type: the schema is not built from one piece of SDL
//   - rejects an empty field arg type: the schema is not built from one piece of SDL
//   - rejects a non-type value as a field arg type: the schema is not built from one piece of SDL
//   - rejects an argument with invalid default values (programmatic): the schema is built with the object API
//   - Attempts to offer a suggested fix if possible (programmatic): the schema is built with the object API
//   - Attempts to offer a suggested fix if possible (SDL): the schema is built with the object API
//   - rejects an empty input field type: the schema is not built from one piece of SDL
//   - rejects a non-type value as an input field type: the schema is not built from one piece of SDL
//   - rejects an Input Object field with invalid default values (programmatic): the schema is built with the object API
//   - checks each shared unbreakable OneOf subgraph once: the schema is built with the object API
//   - does not throw on valid schemas: it does not check validateSchema exactly once
//   - combines multiple errors: the schema is not built from one piece of SDL

// TestPortedSchemaValidation_ImplementsTwiceViaExtension is graphql-js's
// "rejects an Object implementing the same interface twice due to extension".
// It is on its own because the case needs a second document extending the
// first, which the table above has no way to say.
func TestPortedSchemaValidation_ImplementsTwiceViaExtension(t *testing.T) {
	s, err := utilities.BuildSchema(`
      type Query {
        test: AnotherObject
      }

      interface AnotherInterface {
        field: String
      }

      type AnotherObject implements AnotherInterface {
        field: String
      }
    `)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	doc, err := language.ParseString("extend type AnotherObject implements AnotherInterface")
	if err != nil {
		t.Fatalf("parsing the extension: %v", err)
	}
	extended, err := utilities.ExtendSchema(s, doc)
	if err != nil {
		t.Fatalf("extending the schema: %v", err)
	}

	got := schema.ValidateSchema(extended)
	if len(got) != 1 {
		t.Fatalf("%d errors, want 1:\n%s", len(got), describeSchemaErrors(got))
	}
	if want := "Type AnotherObject can only implement AnotherInterface once."; got[0].Message != want {
		t.Errorf("\n got %s\nwant %s", got[0].Message, want)
	}
	// Both documents named it: the original at line 10, the extension at its
	// own line 1.
	want := []struct{ line, column int }{{10, 37}, {1, 38}}
	if len(got[0].Locations) != len(want) {
		t.Fatalf("points at %d places, want %d", len(got[0].Locations), len(want))
	}
	for i, at := range want {
		if got[0].Locations[i].Line != at.line || got[0].Locations[i].Column != at.column {
			t.Errorf("location %d = %d:%d, want %d:%d", i,
				got[0].Locations[i].Line, got[0].Locations[i].Column, at.line, at.column)
		}
	}
}

// TestPortedSchemaValidation_RootTypeWording pins the wording of the rule that
// names several operations at once. The ported cases above compare how many
// errors there are and where they point (decision 13); here the text matters,
// because which operations share the type is the whole of what it says.
func TestPortedSchemaValidation_RootTypeWording(t *testing.T) {
	tests := []struct {
		name   string
		sdl    string
		want   string
		places int
	}{
		{
			name:   "all three",
			places: 3,
			sdl: `
      type SomeObject { field: String }
      schema { query: SomeObject  mutation: SomeObject  subscription: SomeObject }
    `,
			want: `All root types must be different, "SomeObject" type is used as query, mutation, and subscription root types.`,
		},
		{
			name:   "two of the three",
			places: 2,
			sdl: `
      type SomeObject { field: String }
      type UniqueObject { field: String }
      schema { query: SomeObject  mutation: SomeObject  subscription: UniqueObject }
    `,
			want: `All root types must be different, "SomeObject" type is used as query and mutation root types.`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := utilities.BuildSchema(tt.sdl)
			if err != nil {
				t.Fatalf("building the schema: %v", err)
			}
			got := schema.ValidateSchema(s)
			if len(got) != 1 {
				t.Fatalf("%d errors, want 1:\n%s", len(got), describeSchemaErrors(got))
			}
			if got[0].Message != tt.want {
				t.Errorf("\n got %s\nwant %s", got[0].Message, tt.want)
			}
			// Every place the type is named is pointed at.
			if len(got[0].Locations) != tt.places {
				t.Errorf("points at %d places, want %d", len(got[0].Locations), tt.places)
			}
		})
	}
}

// TestPortedSchemaValidation_FiniteValueWording pins the wording of the one
// rule whose message names a chain of fields rather than a single place. The
// ported cases above compare how many errors there are and where they point
// (decision 13); here the text matters, because the chain it lists is the
// whole of what the reader is being told.
func TestPortedSchemaValidation_FiniteValueWording(t *testing.T) {
	tests := []struct {
		name string
		sdl  string
		want []string
	}{
		{
			name: "a field that leads back to itself",
			sdl: `
      type Query { field(arg: SomeInputObject): String }
      input SomeInputObject { nonNullSelf: SomeInputObject! }
    `,
			want: []string{
				"Input Object SomeInputObject cannot be provided a finite value because it references itself through fields: SomeInputObject.nonNullSelf.",
			},
		},
		{
			name: "a chain across three input objects",
			sdl: `
      type Query { field(arg: SomeInputObject): String }
      input SomeInputObject { startLoop: AnotherInputObject! }
      input AnotherInputObject { nextInLoop: YetAnotherInputObject! }
      input YetAnotherInputObject { closeLoop: SomeInputObject! }
    `,
			want: []string{
				"Input Object SomeInputObject cannot be provided a finite value because it references itself through fields: SomeInputObject.startLoop, AnotherInputObject.nextInLoop, YetAnotherInputObject.closeLoop.",
			},
		},
		{
			name: "several chains at once",
			sdl: `
      type Query { field(arg: SomeInputObject): String }
      input SomeInputObject { startLoop: AnotherInputObject! }
      input AnotherInputObject {
        closeLoop: SomeInputObject!
        startSecondLoop: YetAnotherInputObject!
      }
      input YetAnotherInputObject {
        closeSecondLoop: AnotherInputObject!
        nonNullSelf: YetAnotherInputObject!
      }
    `,
			want: []string{
				"Input Object SomeInputObject cannot be provided a finite value because it references itself through fields: SomeInputObject.startLoop, AnotherInputObject.closeLoop.",
				"Input Object AnotherInputObject cannot be provided a finite value because it references itself through fields: AnotherInputObject.startSecondLoop, YetAnotherInputObject.closeSecondLoop.",
				"Input Object YetAnotherInputObject cannot be provided a finite value because it references itself through fields: YetAnotherInputObject.nonNullSelf.",
			},
		},
		{
			name: "one input object caught in two chains",
			sdl: `
      type Query { field(arg: A): String }
      input A { b: B!  c: C! }
      input B { a: A! }
      input C { a: A! }
    `,
			want: []string{
				"Input Object A cannot be provided a finite value because it references itself through fields: A.b, B.a.",
				"Input Object A cannot be provided a finite value because it references itself through fields: A.c, C.a.",
			},
		},
		{
			name: "a chain that can be broken is not a chain",
			sdl: `
      type Query { field(arg: SomeInputObject): String }
      input SomeInputObject {
        self: SomeInputObject
        arrayOfSelf: [SomeInputObject]
        nonNullArrayOfSelf: [SomeInputObject]!
        nonNullArrayOfNonNullSelf: [SomeInputObject!]!
        intermediateSelf: AnotherInputObject
      }
      input AnotherInputObject { parent: SomeInputObject }
    `,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := utilities.BuildSchema(tt.sdl)
			if err != nil {
				t.Fatalf("building the schema: %v", err)
			}
			got := schema.ValidateSchema(s)
			if len(got) != len(tt.want) {
				t.Fatalf("%d errors, want %d:\n%s", len(got), len(tt.want), describeSchemaErrors(got))
			}
			for i, want := range tt.want {
				if got[i].Message != want {
					t.Errorf("error %d:\n got %s\nwant %s", i, got[i].Message, want)
				}
			}
		})
	}
}
