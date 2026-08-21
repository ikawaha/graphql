package execution_test

// Ported from graphql-js src/execution/__tests__/variables-test.ts, the
// "using fragment arguments" section: what a fragment's own variables are
// bound to when the fragment is spread.
//
// The syntax is experimental, so these documents are parsed with the option
// that reads it; that is the only thing that separates them from any other
// request.

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/language"
)

// knownFragmentArgumentDivergences are the cases this implementation does not
// match, and why. Each is asserted to *still* differ, so that closing one
// cannot go unnoticed.
var knownFragmentArgumentDivergences = map[string]string{
	// A request whose fragment arguments will not coerce never began, so it
	// has no data. graphql-js answers `data: null` here and no data at all for
	// the invalid-type case just below, depending on which of its two checks
	// caught the problem; here the answer is the same either way.
	// COMPATIBILITY.md records this.
}

func TestPortedFragmentArguments(t *testing.T) {
	s := testVariablesSchema(t)
	for _, tt := range []struct{ name, query, variables, want string }{
		{
			name:  "when there are no fragment arguments",
			query: "\n        query {\n          ...a\n        }\n        fragment a on TestType {\n          fieldWithNonNullableStringInput(input: \"A\")\n        }\n      ",
			want:  "{\"data\": {\"fieldWithNonNullableStringInput\": \"\\\"A\\\"\"}}",
		},
		{
			name:  "when a value is required and provided",
			query: "\n        query {\n          ...a(value: \"A\")\n        }\n        fragment a($value: String!) on TestType {\n          fieldWithNonNullableStringInput(input: $value)\n        }\n      ",
			want:  "{\"data\": {\"fieldWithNonNullableStringInput\": \"\\\"A\\\"\"}}",
		},
		{
			name:  "when a value is required and not provided",
			query: "\n        query {\n          ...a\n        }\n        fragment a($value: String!) on TestType {\n          fieldWithNullableStringInput(input: $value)\n        }\n      ",
			want:  "{\"data\": null, \"errors\": [{\"message\": \"Variable \\\"$value\\\" defined by fragment \\\"a\\\" of required type \\\"String!\\\" was not provided.\", \"locations\": [{\"line\": 3, \"column\": 11}]}]}",
		},
		{
			name:  "when the definition has a default and is provided",
			query: "\n        query {\n          ...a(value: \"A\")\n        }\n        fragment a($value: String! = \"B\") on TestType {\n          fieldWithNonNullableStringInput(input: $value)\n        }\n      ",
			want:  "{\"data\": {\"fieldWithNonNullableStringInput\": \"\\\"A\\\"\"}}",
		},
		{
			name:  "when the definition has a default and is not provided",
			query: "\n        query {\n          ...a\n        }\n        fragment a($value: String! = \"B\") on TestType {\n          fieldWithNonNullableStringInput(input: $value)\n        }\n      ",
			want:  "{\"data\": {\"fieldWithNonNullableStringInput\": \"\\\"B\\\"\"}}",
		},
		{
			name:  "when the definition has an invalid default and is not provided",
			query: "query { ...a } fragment a($value: String = 123) on TestType { fieldWithNullableStringInput(input: $value) }",
			want:  "{\"data\": null, \"errors\": [{\"message\": \"Variable \\\"$value\\\" defined by fragment \\\"a\\\" has invalid default value: String cannot represent a non string value: 123\", \"locations\": [{\"line\": 1, \"column\": 9}]}]}",
		},
		{
			name:  "does not allow invalid types to be used as fragment variables",
			query: "\n        query {\n          ...a\n        }\n        fragment a($value: TestType!) on TestType {\n          fieldWithNullableStringInput(input: $value)\n        }\n      ",
			want:  "{\"errors\": [{\"message\": \"Variable \\\"$value\\\" expected value of type \\\"TestType!\\\" which cannot be used as an input type.\", \"locations\": [{\"line\": 5, \"column\": 28}]}]}",
		},
		{
			name:  "when a definition has a default, is not provided, and spreads another fragment",
			query: "\n        query {\n          ...a\n        }\n        fragment a($a: String! = \"B\") on TestType {\n          ...b(b: $a)\n        }\n        fragment b($b: String!) on TestType {\n          fieldWithNonNullableStringInput(input: $b)\n        }\n      ",
			want:  "{\"data\": {\"fieldWithNonNullableStringInput\": \"\\\"B\\\"\"}}",
		},
		{
			name:  "when the definition has a non-nullable default and is provided null",
			query: "\n        query {\n          ...a(value: null)\n        }\n        fragment a($value: String! = \"B\") on TestType {\n          fieldWithNullableStringInput(input: $value)\n        }\n      ",
			want:  "{\"data\": null, \"errors\": [{\"message\": \"Variable \\\"$value\\\" defined by fragment \\\"a\\\" has invalid value: Expected value of non-null type \\\"String!\\\" not to be null.\", \"locations\": [{\"line\": 3, \"column\": 23}]}]}",
		},
		{
			name:  "when the definition has no default and is not provided",
			query: "\n        query {\n          ...a\n        }\n        fragment a($value: String) on TestType {\n          fieldWithNonNullableStringInputAndDefaultArgumentValue(input: $value)\n        }\n      ",
			want:  "{\"data\": {\"fieldWithNonNullableStringInputAndDefaultArgumentValue\": \"\\\"Hello World\\\"\"}}",
		},
		{
			name:  "when an argument is shadowed by an operation variable",
			query: "\n        query($x: String! = \"A\") {\n          ...a(x: \"B\")\n        }\n        fragment a($x: String) on TestType {\n          fieldWithNullableStringInput(input: $x)\n        }\n      ",
			want:  "{\"data\": {\"fieldWithNullableStringInput\": \"\\\"B\\\"\"}}",
		},
		{
			name:  "when a nullable argument without a field default is not provided and shadowed by an operation variable",
			query: "\n        query($x: String = \"A\") {\n          ...a\n        }\n        fragment a($x: String) on TestType {\n          fieldWithNullableStringInput(input: $x)\n        }\n      ",
			want:  "{\"data\": {\"fieldWithNullableStringInput\": null}}",
		},
		{
			name:  "when a nullable argument with a field default is not provided and shadowed by an operation variable",
			query: "\n        query($x: String = \"A\") {\n          ...a\n        }\n        fragment a($x: String) on TestType {\n          fieldWithNonNullableStringInputAndDefaultArgumentValue(input: $x)\n        }\n      ",
			want:  "{\"data\": {\"fieldWithNonNullableStringInputAndDefaultArgumentValue\": \"\\\"Hello World\\\"\"}}",
		},
		{
			name:  "when a fragment-variable is shadowed by an intermediate fragment-spread but defined in the operation-variables",
			query: "\n        query($x: String = \"A\") {\n          ...a\n        }\n        fragment a($x: String) on TestType {\n          ...b\n        }\n\n        fragment b on TestType {\n          fieldWithNullableStringInput(input: $x)\n        }\n      ",
			want:  "{\"data\": {\"fieldWithNullableStringInput\": \"\\\"A\\\"\"}}",
		},
		{
			name:  "when a fragment is used with different args",
			query: "\n        query($x: String = \"Hello\") {\n          a: nested {\n            ...a(x: \"a\")\n          }\n          b: nested {\n            ...a(x: \"b\", b: true)\n          }\n          hello: nested {\n            ...a(x: $x)\n          }\n        }\n        fragment a($x: String, $b: Boolean = false) on NestedType {\n          a: echo(input: $x) @skip(if: $b)\n          b: echo(input: $x) @include(if: $b)\n        }\n      ",
			want:  "{\"data\": {\"a\": {\"a\": \"\\\"a\\\"\"}, \"b\": {\"b\": \"\\\"b\\\"\"}, \"hello\": {\"a\": \"\\\"Hello\\\"\"}}}",
		},
		{
			name:  "when the argument variable is nested in a complex type",
			query: "\n        query {\n          ...a(value: \"C\")\n        }\n        fragment a($value: String) on TestType {\n          list(input: [\"A\", \"B\", $value, \"D\"])\n        }\n      ",
			want:  "{\"data\": {\"list\": \"[\\\"A\\\", \\\"B\\\", \\\"C\\\", \\\"D\\\"]\"}}",
		},
		{
			name:  "when argument variables are used recursively",
			query: "\n        query {\n          ...a(aValue: \"C\")\n        }\n        fragment a($aValue: String) on TestType {\n          ...b(bValue: $aValue)\n        }\n        fragment b($bValue: String) on TestType {\n          list(input: [\"A\", \"B\", $bValue, \"D\"])\n        }\n      ",
			want:  "{\"data\": {\"list\": \"[\\\"A\\\", \\\"B\\\", \\\"C\\\", \\\"D\\\"]\"}}",
		},
		{
			name:  "when argument variables with the same name are used directly and recursively",
			query: "\n        query {\n          ...a(value: \"A\")\n        }\n        fragment a($value: String!) on TestType {\n          ...b(value: \"B\")\n          fieldInFragmentA: fieldWithNonNullableStringInput(input: $value)\n        }\n        fragment b($value: String!) on TestType {\n          fieldInFragmentB: fieldWithNonNullableStringInput(input: $value)\n        }\n      ",
			want:  "{\"data\": {\"fieldInFragmentA\": \"\\\"A\\\"\", \"fieldInFragmentB\": \"\\\"B\\\"\"}}",
		},
		{
			name:  "when argument passed in as list",
			query: "\n        query Q($opValue: String = \"op\") {\n          ...a(aValue: \"A\")\n        }\n        fragment a($aValue: String, $bValue: String) on TestType {\n          ...b(aValue: [$aValue, \"B\"], bValue: [$bValue, $opValue])\n        }\n        fragment b($aValue: [String], $bValue: [String], $cValue: String) on TestType {\n          aList: list(input: $aValue)\n          bList: list(input: $bValue)\n          cList: list(input: [$cValue])\n        }\n      ",
			want:  "{\"data\": {\"aList\": \"[\\\"A\\\", \\\"B\\\"]\", \"bList\": \"[null, \\\"op\\\"]\", \"cList\": \"[null]\"}}",
		},
		{
			name:  "when argument passed to a directive",
			query: "\n        query {\n          ...a(value: true)\n        }\n        fragment a($value: Boolean!) on TestType {\n          fieldWithNonNullableStringInput @skip(if: $value)\n        }\n      ",
			want:  "{\"data\": {}}",
		},
		{
			name:  "when argument passed to a directive on a nested field",
			query: "\n        query {\n          ...a(value: true)\n        }\n        fragment a($value: Boolean!) on TestType {\n          nested { echo(input: \"echo\") @skip(if: $value) }\n        }\n      ",
			want:  "{\"data\": {\"nested\": {}}}",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := language.ParseString(tt.query, language.ExperimentalFragmentArguments())
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}
			result := execution.Execute(context.Background(), execution.Request{
				Schema:    s,
				Document:  doc,
				Variables: portedVariables(t, tt.variables),
			})
			got := decodeJSON(t, mustMarshal(t, result))
			want := decodeJSON(t, tt.want)

			if why, listed := knownFragmentArgumentDivergences[tt.name]; listed {
				if reflect.DeepEqual(got, want) {
					t.Errorf("this case now matches graphql-js; remove it from the known divergences (%s)", why)
				} else {
					t.Logf("known divergence: %s", why)
				}
				return
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("response =\n%s\nwant\n%s", mustMarshal(t, result), tt.want)
			}
		})
	}
}

// A fragment's own variable decides a @defer written inside it. The operation
// declares a variable of the same name and defaults it to false, and that does
// not answer the fragment's: the fragment's is unset, so the directive's own
// default stands and the fragment is deferred.
//
// graphql-js checks only that the request produced an incremental response;
// here the whole of it is compared, since that costs nothing extra.
func TestPortedFragmentArguments_DecideDefer(t *testing.T) {
	s := buildSchema(t, `
		directive @defer(if: Boolean! = true, label: String) on FRAGMENT_SPREAD | INLINE_FRAGMENT
		type Query { a: String b: String }
	`)
	const query = `
		query($shouldDefer: Boolean = false) {
			a
			...f%s
		}
		fragment f($shouldDefer: Boolean) on Query {
			... @defer(if: $shouldDefer) {
				b
			}
		}
	`

	for _, tt := range []struct {
		name     string
		supplied string
		initial  string
		deferred bool
	}{
		{
			name:     "the operation's variable does not answer the fragment's",
			supplied: "",
			initial:  `{"data":{"a":"A"},"pending":[{"id":"0","path":[]}],"hasNext":true}`,
			deferred: true,
		},
		{
			name:     "the spread says not to defer",
			supplied: "(shouldDefer: false)",
			initial:  `{"data":{"a":"A","b":"B"}}`,
			deferred: false,
		},
		{
			name:     "the spread says to defer",
			supplied: "(shouldDefer: true)",
			initial:  `{"data":{"a":"A"},"pending":[{"id":"0","path":[]}],"hasNext":true}`,
			deferred: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := language.ParseString(fmt.Sprintf(query, tt.supplied),
				language.ExperimentalFragmentArguments())
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}
			result := execution.ExecuteIncrementally(context.Background(), execution.Request{
				Schema: s, Document: doc, RootValue: map[string]any{"a": "A", "b": "B"},
			})
			payloads := 0
			if result.Subsequent != nil {
				for range result.Subsequent {
					payloads++
				}
			}
			if got := string(mustMarshal(t, result.Initial)); got != tt.initial {
				t.Errorf("first response = %s\nwant %s", got, tt.initial)
			}
			if deferred := payloads > 0; deferred != tt.deferred {
				t.Errorf("%d payloads followed, want deferred = %v", payloads, tt.deferred)
			}
		})
	}
}
