package utilities_test

// Ported from graphql-js src/utilities/__tests__/findSchemaChanges-test.ts:
// two versions of a schema, and what changed between them.

import (
	"testing"

	"github.com/ikawaha/graphql/utilities"
)

// portedChange is one difference graphql-js expects to be reported.
type portedChange struct {
	kind string
	says string
}

// knownChangeDivergences are the cases this implementation does not match, and
// why. Each is asserted to *still* diverge, so that closing one cannot go
// unnoticed.
var knownChangeDivergences = map[string]string{
	// graphql-js writes the directive's own name where the argument's belongs,
	// so its message reads "Description of @Foo(Foo)". That is a slip rather
	// than a decision; the argument is named here.
	"should detect if a directive arg changes description": "graphql-js names the directive twice instead of the argument",
}

func TestPortedFindSchemaChanges(t *testing.T) {
	for _, tt := range []struct {
		name          string
		before, after string
		want          []portedChange
	}{
		{
			name: `should detect if a type was added`,
			before: `type Type1
`,
			after: `type Type1
type Type2`,
			want: []portedChange{
				{kind: "TYPE_ADDED", says: "Type2 was added."},
			},
		},
		{
			name: `should detect a type changing description`,
			before: `type Type1
`,
			after: `"New Description"
type Type1`,
			want: []portedChange{
				{kind: "DESCRIPTION_CHANGED", says: "Description of Type1 has changed to \"New Description\"."},
			},
		},
		{
			name: `should detect if a field was added`,
			before: `type Query {
  foo: String
}`,
			after: `type Query {
  foo: String
  bar: String
}`,
			want: []portedChange{
				{kind: "FIELD_ADDED", says: "Field Query.bar was added."},
			},
		},
		{
			name: `should detect a field changing description`,
			before: `type Query {
  foo: String
  bar: String
}`,
			after: `type Query {
  foo: String
  "New Description"
  bar: String
}`,
			want: []portedChange{
				{kind: "DESCRIPTION_CHANGED", says: "Description of field Query.bar has changed to \"New Description\"."},
			},
		},
		{
			name: `should detect if a default value was added`,
			before: `type Query {
  foo(x: String): String
}`,
			after: `type Query {
  foo(x: String = "bar"): String
}`,
			want: []portedChange{
				{kind: "ARG_DEFAULT_VALUE_ADDED", says: "Query.foo(x:) added a defaultValue \"bar\"."},
			},
		},
		{
			name: `should detect if an arg value changes safely`,
			before: `type Query {
  foo(x: String!): String
}`,
			after: `type Query {
  foo(x: String): String
}`,
			want: []portedChange{
				{kind: "ARG_CHANGED_KIND_SAFE", says: "Argument Query.foo(x:) has changed type from String! to String."},
			},
		},
		{
			name: `should detect if an arg value changes description`,
			before: `type Query {
  foo(x: String!): String
}`,
			after: `type Query {
  foo(
    "New Description"
    x: String!
  ): String
}`,
			want: []portedChange{
				{kind: "DESCRIPTION_CHANGED", says: "Description of argument Query.foo(x:) has changed to \"New Description\"."},
			},
		},
		{
			name: `should detect if a directive was added`,
			before: `type Query {
  foo: String
}`,
			after: `directive @Foo on FIELD_DEFINITION

type Query {
  foo: String
}`,
			want: []portedChange{
				{kind: "DIRECTIVE_ADDED", says: "Directive @Foo was added."},
			},
		},
		{
			name: `should detect if a changes argument safely`,
			before: `directive @Foo(foo: String!) on FIELD_DEFINITION

type Query {
  foo: String
}`,
			after: `directive @Foo(foo: String) on FIELD_DEFINITION

type Query {
  foo: String
}`,
			want: []portedChange{
				{kind: "ARG_CHANGED_KIND_SAFE", says: "Argument @Foo(foo:) has changed type from String! to String."},
			},
		},
		{
			name: `should detect if a default value is added to an argument`,
			before: `directive @Foo(foo: String) on FIELD_DEFINITION

type Query {
  foo: String
}`,
			after: `directive @Foo(foo: String = "Foo") on FIELD_DEFINITION

type Query {
  foo: String
}`,
			want: []portedChange{
				{kind: "ARG_DEFAULT_VALUE_ADDED", says: "@Foo(foo:) added a defaultValue \"Foo\"."},
			},
		},
		{
			name: `should detect if a default value is removed from an argument`,
			before: `directive @Foo(foo: String = "Foo") on FIELD_DEFINITION

type Query {
  foo: String
}`,
			after: `directive @Foo(foo: String) on FIELD_DEFINITION

type Query {
  foo: String
}`,
			want: []portedChange{
				{kind: "ARG_DEFAULT_VALUE_CHANGE", says: "@Foo(foo:) defaultValue was removed."},
			},
		},
		{
			name: `should detect if a default value is changed in an argument`,
			before: `directive @Foo(foo: String = "Bar") on FIELD_DEFINITION

type Query {
  foo: String
}`,
			after: `directive @Foo(foo: String = "Foo") on FIELD_DEFINITION

type Query {
  foo: String
}`,
			want: []portedChange{
				{kind: "ARG_DEFAULT_VALUE_CHANGE", says: "@Foo(foo:) has changed defaultValue from \"Bar\" to \"Foo\"."},
			},
		},
		{
			name: `should detect if a directive argument does a breaking change`,
			before: `directive @Foo(foo: String) on FIELD_DEFINITION

type Query {
  foo: String
}`,
			after: `directive @Foo(foo: String!) on FIELD_DEFINITION

type Query {
  foo: String
}`,
			want: []portedChange{
				{kind: "ARG_CHANGED_KIND", says: "Argument @Foo(foo:) has changed type from String to String!."},
			},
		},
		{
			name: `should not detect if a directive argument default value does not change`,
			before: `directive @Foo(foo: String = "FOO") on FIELD_DEFINITION

type Query {
  foo: String
}`,
			after: `directive @Foo(foo: String = "FOO") on FIELD_DEFINITION

type Query {
  foo: String
}`,
			want: []portedChange{},
		},
		{
			name: `should detect if a directive changes description`,
			before: `directive @Foo on FIELD_DEFINITION

type Query {
  foo: String
}`,
			after: `"New Description"
directive @Foo on FIELD_DEFINITION

type Query {
  foo: String
}`,
			want: []portedChange{
				{kind: "DESCRIPTION_CHANGED", says: "Description of @Foo has changed to \"New Description\"."},
			},
		},
		{
			name: `should detect if a directive becomes repeatable`,
			before: `directive @Foo on FIELD_DEFINITION

type Query {
  foo: String
}`,
			after: `directive @Foo repeatable on FIELD_DEFINITION

type Query {
  foo: String
}`,
			want: []portedChange{
				{kind: "DIRECTIVE_REPEATABLE_ADDED", says: "Repeatable flag was added to @Foo."},
			},
		},
		{
			name: `should detect if a directive adds locations`,
			before: `directive @Foo on FIELD_DEFINITION

type Query {
  foo: String
}`,
			after: `directive @Foo on FIELD_DEFINITION | QUERY

type Query {
  foo: String
}`,
			want: []portedChange{
				{kind: "DIRECTIVE_LOCATION_ADDED", says: "QUERY was added to @Foo."},
			},
		},
		{
			name: `should detect if a directive arg gets added`,
			before: `directive @Foo on FIELD_DEFINITION

type Query {
  foo: String
}`,
			after: `directive @Foo(arg1: String) on FIELD_DEFINITION

type Query {
  foo: String
}`,
			want: []portedChange{
				{kind: "OPTIONAL_DIRECTIVE_ARG_ADDED", says: "An optional argument @Foo(arg1:) was added."},
			},
		},
		{
			name: `should detect if a directive arg changes description`,
			before: `directive @Foo(
  arg1: String
) on FIELD_DEFINITION

type Query {
  foo: String
}`,
			after: `directive @Foo(
  "New Description"
  arg1: String
) on FIELD_DEFINITION

type Query {
  foo: String
}`,
			want: []portedChange{
				{kind: "DESCRIPTION_CHANGED", says: "Description of @Foo(Foo) has changed to \"New Description\"."},
			},
		},
		{
			name: `should detect if an enum member changes description`,
			before: `enum Foo {
  TEST
}

type Query {
  foo: String
}`,
			after: `enum Foo {
  "New Description"
  TEST
}

type Query {
  foo: String
}`,
			want: []portedChange{
				{kind: "DESCRIPTION_CHANGED", says: "Description of enum value Foo.TEST has changed to \"New Description\"."},
			},
		},
		{
			name: `should detect if an input field changes description`,
			before: `input Foo {
  x: String
}

type Query {
  foo: String
}`,
			after: `input Foo {
  "New Description"
  x: String
}

type Query {
  foo: String
}`,
			want: []portedChange{
				{kind: "DESCRIPTION_CHANGED", says: "Description of input-field Foo.x has changed to \"New Description\"."},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before, after := mustBuild(t, tt.before), mustBuild(t, tt.after)
			found := utilities.FindSchemaChanges(before, after)

			got := make([]portedChange, len(found))
			for i, change := range found {
				got[i] = portedChange{kind: change.Kind, says: change.Message}
			}
			same := len(got) == len(tt.want)
			if same {
				for i := range got {
					if got[i] != tt.want[i] {
						same = false
						break
					}
				}
			}
			if why, listed := knownChangeDivergences[tt.name]; listed {
				if same {
					t.Errorf("this case now matches graphql-js; remove it from the known divergences (%s)", why)
				} else {
					t.Logf("known divergence: %s", why)
				}
				return
			}
			if !same {
				t.Errorf("found %v, want %v", got, tt.want)
			}
		})
	}
}
