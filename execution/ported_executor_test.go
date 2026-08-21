package execution_test

// Ported from graphql-js src/execution/__tests__/executor-test.ts.
//
// Where the original distinguishes a synchronous resolver from one returning a
// promise, both are the same resolver here, so the pairs collapse into one
// case; the field names are kept as graphql-js wrote them.

import (
	"context"
	"errors"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
	"github.com/ikawaha/graphql/value"
)

func TestPortedExecutor(t *testing.T) {
	const simple = `type Type { a: String } schema { query: Type }`
	fromB := map[string]any{"a": "b"}

	runPorted(t, nil, nil, knownExecutorDivergences, []portedCase{
		{
			name:      `executes arbitrary code`,
			built:     arbitraryCodeSchema(t),
			root:      arbitraryCodeData(),
			variables: `{"size": 100}`,
			query: `
      query ($size: Int) {
        a,
        b,
        x: c
        ...c
        f
        ...on DataType {
          pic(size: $size)
          promise {
            a
          }
        }
        deep {
          a
          b
          c
          deeper {
            a
            b
          }
        }
      }

      fragment c on DataType {
        d
        e
      }
    `,
			want: `{"data": {
				"a": "Apple", "b": "Banana", "x": "Cookie", "d": "Donut", "e": "Egg", "f": "Fish",
				"pic": "Pic of size: 100",
				"promise": {"a": "Apple"},
				"deep": {
					"a": "Already Been Done", "b": "Boring",
					"c": ["Contrived", null, "Confusing"],
					"deeper": [{"a": "Apple", "b": "Banana"}, null, {"a": "Apple", "b": "Banana"}]}}}`,
		},
		{
			name:  `merges parallel fragments`,
			built: mergedFragmentsSchema(t),
			query: `
      { a, ...FragOne, ...FragTwo }

      fragment FragOne on Type {
        b
        deep { b, deeper: deep { b } }
      }

      fragment FragTwo on Type {
        c
        deep { c, deeper: deep { c } }
      }
    `,
			want: `{"data": {"a": "Apple", "b": "Banana", "c": "Cherry",
				"deep": {"b": "Banana", "c": "Cherry", "deeper": {"b": "Banana", "c": "Cherry"}}}}`,
		},
		{
			name:  `correctly threads arguments`,
			built: threadedArgumentsSchema(t),
			query: `
      query Example {
        b(numArg: 123, stringArg: "foo")
      }
    `,
			want: `{"data": {"b": "{ numArg: 123, stringArg: \"foo\" }"}}`,
		},
		{
			name:  `nulls out error subtrees`,
			built: errorSubtreesSchema(t),
			root: map[string]any{
				"sync":                "sync",
				"syncReturnError":     gqlerror.New("Error getting syncReturnError"),
				"syncReturnErrorList": []any{"sync0", gqlerror.New("Error getting syncReturnErrorList1"), "sync2", gqlerror.New("Error getting syncReturnErrorList3")},
				"asyncReturnErrorWithExtensions": gqlerror.New(
					"Error getting asyncReturnErrorWithExtensions",
					gqlerror.WithExtensions(map[string]any{"foo": "bar"})),
			},
			query: `
      {
        sync
        syncError
        syncReturnError
        syncReturnErrorList
        asyncReturnErrorWithExtensions
      }
    `,
			want: `{"data": {
				"sync": "sync", "syncError": null, "syncReturnError": null,
				"syncReturnErrorList": ["sync0", null, "sync2", null],
				"asyncReturnErrorWithExtensions": null},
				"errors": [
					{"message": "Error getting syncError",
					 "locations": [{"line": 4, "column": 9}], "path": ["syncError"]},
					{"message": "Error getting syncReturnError",
					 "locations": [{"line": 5, "column": 9}], "path": ["syncReturnError"]},
					{"message": "Error getting syncReturnErrorList1",
					 "locations": [{"line": 6, "column": 9}], "path": ["syncReturnErrorList", 1]},
					{"message": "Error getting syncReturnErrorList3",
					 "locations": [{"line": 6, "column": 9}], "path": ["syncReturnErrorList", 3]},
					{"message": "Error getting asyncReturnErrorWithExtensions",
					 "locations": [{"line": 7, "column": 9}],
					 "path": ["asyncReturnErrorWithExtensions"], "extensions": {"foo": "bar"}}]}`,
		},
		{
			name:  `nulls error subtree for a failed list of objects`,
			built: failingFoodsSchema(t),
			query: `
      query {
        foods {
          name
        }
      }
    `,
			want: `{"data": {"foods": null}, "errors": [
				{"message": "Oops", "locations": [{"line": 3, "column": 9}], "path": ["foods"]}]}`,
		},
		{
			name:  `handles bubbling errors combined with non-bubbling errors`,
			built: bubblingErrorsSchema(t),
			query: `
      {
        asyncError
        asyncNonNullError
      }
    `,
			want: `{"data": null, "errors": [
				{"message": "Oops", "locations": [{"line": 3, "column": 9}], "path": ["asyncError"]},
				{"message": "Cannot return null for non-nullable field Query.asyncNonNullError.",
				 "locations": [{"line": 4, "column": 9}], "path": ["asyncNonNullError"]}]}`,
		},
		{
			name:  `full response path is included for non-nullable fields`,
			built: responsePathSchema(t),
			query: `
      query {
        nullableA {
          aliasedA: nullableA {
            nonNullA {
              anotherA: nonNullA {
                throws
              }
            }
          }
        }
      }
    `,
			want: `{"data": {"nullableA": {"aliasedA": null}}, "errors": [
				{"message": "Catch me if you can", "locations": [{"line": 7, "column": 17}],
				 "path": ["nullableA", "aliasedA", "nonNullA", "anotherA", "throws"]}]}`,
		},

		// Which operation a request runs.
		{
			name: `uses the inline operation if no operation name is provided`,
			sdl:  simple, query: `{ a }`, root: fromB,
			want: `{"data": {"a": "b"}}`,
		},
		{
			name: `uses the only operation if no operation name is provided`,
			sdl:  simple, query: `query Example { a }`, root: fromB,
			want: `{"data": {"a": "b"}}`,
		},
		{
			name: `uses the named operation if operation name is provided`,
			sdl:  simple, root: fromB, operation: "OtherExample",
			query: `
      query Example { first: a }
      query OtherExample { second: a }
    `,
			want: `{"data": {"second": "b"}}`,
		},
		{
			name: `provides error if no operation is provided`,
			sdl:  simple, query: `fragment Example on Type { a }`, root: fromB,
			want: `{"errors": [{"message": "Must provide an operation."}]}`,
		},
		{
			name: `errors if no op name is provided with multiple operations`,
			sdl:  simple,
			query: `
      query Example { a }
      query OtherExample { a }
    `,
			want: `{"errors": [{"message": "Must provide operation name if query contains multiple operations."}]}`,
		},
		{
			name: `errors if unknown operation name is provided`,
			sdl:  simple, operation: "UnknownExample",
			query: `
      query Example { a }
      query OtherExample { a }
    `,
			want: `{"errors": [{"message": "Unknown operation named \"UnknownExample\"."}]}`,
		},

		// Which root type an operation enters through.
		{
			name: `uses the query schema for queries`,
			sdl: `type Q { a: String } type M { c: String } type S { a: String }
				schema { query: Q mutation: M subscription: S }`,
			root: map[string]any{"a": "b", "c": "d"}, operation: "Q",
			query: `
      query Q { a }
      mutation M { c }
      subscription S { a }
    `,
			want: `{"data": {"a": "b"}}`,
		},
		{
			name: `uses the mutation schema for mutations`,
			sdl: `type Q { a: String } type M { c: String }
				schema { query: Q mutation: M }`,
			root: map[string]any{"a": "b", "c": "d"}, operation: "M",
			query: `
      query Q { a }
      mutation M { c }
    `,
			want: `{"data": {"c": "d"}}`,
		},
		{
			name: `uses the subscription schema for subscriptions`,
			sdl: `type Q { a: String } type S { a: String }
				schema { query: Q subscription: S }`,
			root: map[string]any{"a": "b", "c": "d"}, operation: "S",
			query: `
      query Q { a }
      subscription S { a }
    `,
			want: `{"data": {"a": "b"}}`,
		},
		{
			name:  `resolves to an error if schema does not support the query operation`,
			built: schema.New(schema.Config{AssumeValid: true}), operation: "Q",
			query: `
      query Q { __typename }
      mutation M { __typename }
      subscription S { __typename }
    `,
			want: `{"data": null, "errors": [
				{"message": "Schema is not configured to execute query operation.",
				 "locations": [{"line": 2, "column": 7}]}]}`,
		},
		{
			name:  `resolves to an error if schema does not support the mutation operation`,
			built: schema.New(schema.Config{AssumeValid: true}), operation: "M",
			query: `
      query Q { __typename }
      mutation M { __typename }
      subscription S { __typename }
    `,
			want: `{"data": null, "errors": [
				{"message": "Schema is not configured to execute mutation operation.",
				 "locations": [{"line": 3, "column": 7}]}]}`,
		},
		{
			name:  `resolves to an error if schema does not support the subscription operation`,
			built: schema.New(schema.Config{AssumeValid: true}), operation: "S",
			query: `
      query Q { __typename }
      mutation M { __typename }
      subscription S { __typename }
    `,
			want: `{"data": null, "errors": [
				{"message": "Schema is not configured to execute subscription operation.",
				 "locations": [{"line": 4, "column": 7}]}]}`,
		},

		// What comes out, and what does not.
		{
			name:  `correct field ordering despite execution order`,
			sdl:   `type Type { a: String b: String c: String d: String e: String } schema { query: Type }`,
			root:  map[string]any{"a": "a", "b": "b", "c": "c", "d": "d", "e": "e"},
			query: `{ a, b, c, d, e }`,
			want:  `{"data": {"a": "a", "b": "b", "c": "c", "d": "d", "e": "e"}}`,
		},
		{
			name: `avoids recursion`,
			sdl:  simple, root: fromB,
			query: `
      {
        a
        ...Frag
        ...Frag
      }

      fragment Frag on Type {
        a,
        ...Frag
      }
    `,
			want: `{"data": {"a": "b"}}`,
		},
		{
			name:  `ignores missing sub selections on fields`,
			sdl:   `type SomeType { b: String } type Query { a: SomeType }`,
			root:  map[string]any{"a": map[string]any{"b": "c"}},
			query: `{ a }`,
			want:  `{"data": {"a": {}}}`,
		},
		{
			name:  `does not include illegal fields in output`,
			sdl:   `type Q { a: String } schema { query: Q }`,
			query: `{ thisIsIllegalDoNotIncludeMe }`,
			want:  `{"data": {}}`,
		},
		{
			name:  `does not include arguments that were not set`,
			built: unsetArgumentsSchema(t),
			query: `{ field(a: true, c: false, e: 0) }`,
			want:  `{"data": {"field": "{ a: true, c: false, e: 0 }"}}`,
		},
		{
			name:  `fails when an isTypeOf check is not met`,
			built: isTypeOfSchema(t),
			root:  map[string]any{"specials": []any{&special{Value: "foo"}, &notSpecial{Value: "bar"}}},
			query: `{ specials { value } }`,
			want: `{"data": {"specials": [{"value": "foo"}, null]}, "errors": [
				{"message": "Expected value of type \"SpecialType\" but got: { Value: \"bar\" }.",
				 "locations": [{"line": 1, "column": 3}], "path": ["specials", 1]}]}`,
		},
		{
			name:  `fails when the output coercion of a custom scalar returns nothing`,
			built: emptyScalarSchema(t),
			query: `{ customScalar }`,
			want: `{"data": {"customScalar": null}, "errors": [
				{"message": "Expected ` + "`CustomScalar.CoerceOutputValue(\\\"CUSTOM_VALUE\\\")`" + ` to return non-nullable value, returned: undefined",
				 "locations": [{"line": 1, "column": 3}], "path": ["customScalar"]}]}`,
		},
		{
			name: `executes ignoring invalid non-executable definitions`,
			sdl:  `type Query { foo: String }`,
			query: `
      { foo }

      type Query { bar: String }
    `,
			want: `{"data": {"foo": null}}`,
		},
		{
			name: `uses a custom field resolver`,
			sdl:  `type Query { foo: String }`, query: `{ foo }`,
			fieldResolver: func(
				_ context.Context, _ any, _ schema.Arguments, info *schema.ResolveInfo,
			) (any, error) {
				return info.FieldName, nil
			},
			want: `{"data": {"foo": "foo"}}`,
		},
		{
			name: `uses a custom type resolver`,
			sdl: `interface FooInterface { bar: String }
				type FooObject implements FooInterface { bar: String }
				type Query { foo: FooInterface }`,
			root:  map[string]any{"foo": map[string]any{"bar": "bar"}},
			query: `{ foo { bar } }`,
			typeResolver: func(context.Context, any, *schema.ResolveInfo) (string, error) {
				return "FooObject", nil
			},
			want: `{"data": {"foo": {"bar": "bar"}}}`,
		},
	})
}

// knownExecutorDivergences are the cases this implementation does not match,
// and why. Each is asserted to *still* diverge, so that closing one cannot go
// unnoticed.
var knownExecutorDivergences = map[string]string{
	// graphql-js answers a schema that cannot run the operation with data
	// null, as though execution had begun and produced nothing. Nothing ran,
	// so there is no data at all here — the same answer as for a variable that
	// would not coerce, and what tells a caller that the request never started.
}

// arbitraryCodeSchema is graphql-js's first schema: two object types, one of
// which holds a list of the other.
func arbitraryCodeSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s := buildPorted(t, `
		type DataType {
			a: String b: String c: String d: String e: String f: String
			pic(size: Int): String
			deep: DeepDataType
			promise: DataType
		}
		type DeepDataType { a: String b: String c: [String] deeper: [DataType] }
		schema { query: DataType }
	`)
	// graphql-js reads pic from the value with the argument; a Go value has no
	// way to be called with one, so the field says how.
	s.QueryType().Field("pic").Resolve = func(
		_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo,
	) (any, error) {
		size, _ := args.Get("size")
		return "Pic of size: " + value.Describe(size), nil
	}
	return s
}

func arbitraryCodeData() any {
	data := map[string]any{
		"a": "Apple", "b": "Banana", "c": "Cookie", "d": "Donut", "e": "Egg", "f": "Fish",
	}
	deep := map[string]any{
		"a": "Already Been Done", "b": "Boring",
		"c":      []any{"Contrived", nil, "Confusing"},
		"deeper": []any{data, nil, data},
	}
	data["deep"] = deep
	data["promise"] = data
	return data
}

// mergedFragmentsSchema answers the same three words at every depth, so that
// what the response shows is which selections were merged.
func mergedFragmentsSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s := buildPorted(t, `type Type { a: String b: String c: String deep: Type } schema { query: Type }`)
	says := func(word string) schema.FieldResolver {
		return func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
			return word, nil
		}
	}
	s.QueryType().Field("a").Resolve = says("Apple")
	s.QueryType().Field("b").Resolve = says("Banana")
	s.QueryType().Field("c").Resolve = says("Cherry")
	s.QueryType().Field("deep").Resolve = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		return map[string]any{}, nil
	}
	return s
}

// threadedArgumentsSchema answers with the arguments a resolver was handed.
func threadedArgumentsSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s := buildPorted(t, `type Type { b(numArg: Int, stringArg: String): String } schema { query: Type }`)
	s.QueryType().Field("b").Resolve = func(
		_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo,
	) (any, error) {
		return value.Describe(args.Raw()), nil
	}
	return s
}

// errorSubtreesSchema has one field that fails outright; the rest are read
// from the root value, where a gqlerror among them is a failure.
func errorSubtreesSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s := buildPorted(t, `
		type Type {
			sync: String
			syncError: String
			syncReturnError: String
			syncReturnErrorList: [String]
			asyncReturnErrorWithExtensions: String
		}
		schema { query: Type }
	`)
	s.QueryType().Field("syncError").Resolve = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		return nil, errors.New("Error getting syncError")
	}
	return s
}

func failingFoodsSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s := buildPorted(t, `type Food { name: String } type Query { foods: [Food] }`)
	s.QueryType().Field("foods").Resolve = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		return nil, errors.New("Oops")
	}
	return s
}

// bubblingErrorsSchema has one field that fails and one that may not be null
// and is, so that both an error that stays put and one that rises are seen at
// once.
func bubblingErrorsSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s := buildPorted(t, `type Query { asyncNonNullError: String! asyncError: String }`)
	s.QueryType().Field("asyncNonNullError").Resolve = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		return nil, nil
	}
	s.QueryType().Field("asyncError").Resolve = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		return nil, errors.New("Oops")
	}
	return s
}

func responsePathSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s := buildPorted(t, `
		type A { nullableA: A nonNullA: A! throws: String! }
		type query { nullableA: A }
		schema { query: query }
	`)
	anObject := func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
		return map[string]any{}, nil
	}
	a, _ := s.Type("A").(*schema.ObjectType)
	a.Field("nullableA").Resolve = anObject
	a.Field("nonNullA").Resolve = anObject
	a.Field("throws").Resolve = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		return nil, errors.New("Catch me if you can")
	}
	s.QueryType().Field("nullableA").Resolve = anObject
	return s
}

func unsetArgumentsSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s := buildPorted(t, `
		type Type { field(a: Boolean, b: Boolean, c: Boolean, d: Int, e: Int): String }
		schema { query: Type }
	`)
	s.QueryType().Field("field").Resolve = func(
		_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo,
	) (any, error) {
		return value.Describe(args.Raw()), nil
	}
	return s
}

type special struct{ Value string }
type notSpecial struct{ Value string }

func isTypeOfSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s := buildPorted(t, `type SpecialType { value: String } type Query { specials: [SpecialType] }`)
	typed, _ := s.Type("SpecialType").(*schema.ObjectType)
	typed.IsTypeOf = func(_ context.Context, v any, _ *schema.ResolveInfo) (bool, error) {
		_, is := v.(*special)
		return is, nil
	}
	return s
}

func emptyScalarSchema(t *testing.T) *schema.Schema {
	t.Helper()
	custom := schema.NewScalar(schema.ScalarConfig{
		Name:              "CustomScalar",
		CoerceOutputValue: func(any) (value.Maybe[any], error) { return value.Nothing[any](), nil },
	})
	query := schema.NewObject(schema.ObjectConfig{
		Name: "Query",
		Fields: []*schema.Field{
			schema.NewField("customScalar", schema.FieldConfig{
				Type: custom,
				Resolve: func(context.Context, any, schema.Arguments, *schema.ResolveInfo) (any, error) {
					return "CUSTOM_VALUE", nil
				},
			}),
		},
	})
	return schema.New(schema.Config{Query: query})
}

// buildPorted builds a schema from SDL, failing the test if it will not build.
func buildPorted(t *testing.T, sdl string) *schema.Schema {
	t.Helper()
	s, err := utilities.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	return s
}

// Two of graphql-js's cases ask what a resolver was handed rather than what
// came back, so they are written out rather than run through the table.

// Ported from `populates path correctly with complex types`.
func TestPortedExecutor_Path(t *testing.T) {
	s := buildPorted(t, `
		type SomeObject { test: String }
		union SomeUnion = SomeObject
		type SomeQuery { test: [SomeUnion!]! }
		schema { query: SomeQuery }
	`)
	union, _ := s.Type("SomeUnion").(*schema.UnionType)
	union.ResolveType = func(context.Context, any, *schema.ResolveInfo) (string, error) {
		return "SomeObject", nil
	}
	var reached *value.Path
	object, _ := s.Type("SomeObject").(*schema.ObjectType)
	object.Field("test").Resolve = func(
		_ context.Context, _ any, _ schema.Arguments, info *schema.ResolveInfo,
	) (any, error) {
		reached = info.Path
		return nil, nil
	}

	result := execution.Execute(context.Background(), execution.Request{
		Schema: s,
		Document: mustParse(t, `
      query {
        l1: test {
          ... on SomeObject {
            l2: test
          }
        }
      }
    `),
		RootValue: map[string]any{"test": []any{map[string]any{}}},
	})
	if len(result.Errors) > 0 {
		t.Fatalf("errors: %v", result.Errors)
	}

	want := []struct {
		key      string
		index    int
		isIndex  bool
		typeName string
	}{
		{key: "l1", typeName: "SomeQuery"},
		{index: 0, isIndex: true},
		{key: "l2", typeName: "SomeObject"},
	}
	var got []*value.Path
	for at := reached; at != nil; at = at.Prev {
		got = append([]*value.Path{at}, got...)
	}
	if len(got) != len(want) {
		t.Fatalf("path is %d deep, want %d", len(got), len(want))
	}
	for i, w := range want {
		switch {
		case got[i].IsIndex() != w.isIndex:
			t.Errorf("segment %d: IsIndex = %v, want %v", i, got[i].IsIndex(), w.isIndex)
		case w.isIndex && got[i].Index != w.index:
			t.Errorf("segment %d: index = %d, want %d", i, got[i].Index, w.index)
		case !w.isIndex && got[i].Key != w.key:
			t.Errorf("segment %d: key = %q, want %q", i, got[i].Key, w.key)
		case got[i].TypeName != w.typeName:
			t.Errorf("segment %d: type = %q, want %q", i, got[i].TypeName, w.typeName)
		}
	}
}

// Ported from `threads root value context correctly`.
func TestPortedExecutor_RootValue(t *testing.T) {
	s := buildPorted(t, `type Type { a: String } schema { query: Type }`)
	var reached any
	s.QueryType().Field("a").Resolve = func(
		_ context.Context, source any, _ schema.Arguments, _ *schema.ResolveInfo,
	) (any, error) {
		reached = source
		return nil, nil
	}

	root := map[string]any{"contextThing": "thing"}
	execution.Execute(context.Background(), execution.Request{
		Schema:    s,
		Document:  mustParse(t, `query Example { a }`),
		RootValue: root,
	})
	held, isMap := reached.(map[string]any)
	if !isMap || held["contextThing"] != "thing" {
		t.Errorf("the resolver was handed %v, want the root value", reached)
	}
}

// Not ported, because each of these is written in a way this could not follow:
//   - provides info about current execution state: what ResolveInfo holds is
//     not the same set of things; TestExecute_ResolveInfo covers it
//   - errors if empty string is provided as operation name: Go cannot tell an
//     empty operation name from none at all
//   - handles sync errors combined with rejections: it turns on an unfinished
//     promise being abandoned
//   - nulls out error subtrees, the raw and asynchronous halves: throwing a
//     value that is not an error, and rejecting with nothing, have no Go
//     counterpart
//   - memoizes collectSubfields results: the table is not reachable from
//     outside the package; TestSubfieldsAreWorkedOutOnce covers it
//   - memoizes getStreamUsage results: what a @stream asks for is worked out
//     once per list here rather than being remembered, so there is no table
//     to look at
//
// "uses a different number of max coercion errors" is ported as
// TestPortedVariables_MaxCoercionErrors.
