package utilities_test

// Ported from graphql-js src/utilities/__tests__/lexicographicSortSchema-test.ts.
// Each case is a schema and the same schema with everything in name order.

import (
	"testing"

	"github.com/ikawaha/graphql/utilities"
)

func TestPortedLexicographicSort(t *testing.T) {
	for _, tt := range []struct{ name, sdl, want string }{
		{
			name: `sort fields`,
			sdl: `input Bar {
  barB: String!
  barA: String
  barC: [String]
}

interface FooInterface {
  fooB: String!
  fooA: String
  fooC: [String]
}

type FooType implements FooInterface {
  fooC: [String]
  fooA: String
  fooB: String!
}

type Query {
  dummy(arg: Bar): FooType
}`,
			want: `input Bar {
  barA: String
  barB: String!
  barC: [String]
}

interface FooInterface {
  fooA: String
  fooB: String!
  fooC: [String]
}

type FooType implements FooInterface {
  fooA: String
  fooB: String!
  fooC: [String]
}

type Query {
  dummy(arg: Bar): FooType
}`,
		},
		{
			name: `sort implemented interfaces`,
			sdl: `interface FooA {
  dummy: String
}

interface FooB {
  dummy: String
}

interface FooC implements FooB & FooA {
  dummy: String
}

type Query implements FooB & FooA & FooC {
  dummy: String
}`,
			want: `interface FooA {
  dummy: String
}

interface FooB {
  dummy: String
}

interface FooC implements FooA & FooB {
  dummy: String
}

type Query implements FooA & FooB & FooC {
  dummy: String
}`,
		},
		{
			name: `sort types in union`,
			sdl: `type FooA {
  dummy: String
}

type FooB {
  dummy: String
}

type FooC {
  dummy: String
}

union FooUnion = FooB | FooA | FooC

type Query {
  dummy: FooUnion
}`,
			want: `type FooA {
  dummy: String
}

type FooB {
  dummy: String
}

type FooC {
  dummy: String
}

union FooUnion = FooA | FooB | FooC

type Query {
  dummy: FooUnion
}`,
		},
		{
			name: `sort enum values`,
			sdl: `enum Foo {
  B
  C
  A
}

type Query {
  dummy: Foo
}`,
			want: `enum Foo {
  A
  B
  C
}

type Query {
  dummy: Foo
}`,
		},
		{
			name: `sort field arguments`,
			sdl: `type Query {
  dummy(argB: Int!, argA: String, argC: [Float]): ID
}`,
			want: `type Query {
  dummy(argA: String, argB: Int!, argC: [Float]): ID
}`,
		},
		{
			name: `sort types`,
			sdl: `type Query {
  dummy(arg1: FooF, arg2: FooA, arg3: FooG): FooD
}

type FooC implements FooE {
  dummy: String
}

enum FooG {
  enumValue
}

scalar FooA

input FooF {
  dummy: String
}

union FooD = FooC | FooB

interface FooE {
  dummy: String
}

type FooB {
  dummy: String
}`,
			want: `scalar FooA

type FooB {
  dummy: String
}

type FooC implements FooE {
  dummy: String
}

union FooD = FooB | FooC

interface FooE {
  dummy: String
}

input FooF {
  dummy: String
}

enum FooG {
  enumValue
}

type Query {
  dummy(arg1: FooF, arg2: FooA, arg3: FooG): FooD
}`,
		},
		{
			name: `sort directive arguments`,
			sdl: `directive @test(argC: Float, argA: String, argB: Int) on FIELD

type Query {
  dummy: String
}`,
			want: `directive @test(argA: String, argB: Int, argC: Float) on FIELD

type Query {
  dummy: String
}`,
		},
		{
			name: `sort directive locations`,
			sdl: `directive @test(argC: Float, argA: String, argB: Int) on UNION | FIELD | ENUM

type Query {
  dummy: String
}`,
			want: `directive @test(argA: String, argB: Int, argC: Float) on ENUM | FIELD | UNION

type Query {
  dummy: String
}`,
		},
		{
			name: `sort directives`,
			sdl: `directive @fooC on FIELD

directive @fooB on UNION

directive @fooA on ENUM

type Query {
  dummy: String
}`,
			want: `directive @fooA on ENUM

directive @fooB on UNION

directive @fooC on FIELD

type Query {
  dummy: String
}`,
		},
		{
			name: `sort recursive types`,
			sdl: `interface FooC {
  fooB: FooB
  fooA: FooA
  fooC: FooC
}

type FooB implements FooC {
  fooB: FooB
  fooA: FooA
}

type FooA implements FooC {
  fooB: FooB
  fooA: FooA
}

type Query {
  fooC: FooC
  fooB: FooB
  fooA: FooA
}`,
			want: `type FooA implements FooC {
  fooA: FooA
  fooB: FooB
}

type FooB implements FooC {
  fooA: FooA
  fooB: FooB
}

interface FooC {
  fooA: FooA
  fooB: FooB
  fooC: FooC
}

type Query {
  fooA: FooA
  fooB: FooB
  fooC: FooC
}`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, err := utilities.BuildSchema(tt.sdl)
			if err != nil {
				t.Fatalf("building the schema: %v", err)
			}
			if got := utilities.PrintSchema(utilities.LexicographicSortSchema(s)); got != tt.want {
				t.Errorf("sorted to\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}
