package utilities_test

// Ported from graphql-js src/utilities/__tests__/separateOperations-test.ts:
// one document holding several operations split into one document per
// operation, each carrying the fragments that operation reaches.

import (
	"strings"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/utilities"
)

func TestPortedSeparateOperations(t *testing.T) {
	for _, tt := range []struct {
		name     string
		document string
		want     map[string]string
	}{
		{
			name: `separates one AST into multiple, maintaining document order`,
			document: `{
  ...Y
  ...X
}

query One {
  foo
  bar
  ...A
  ...X
}

fragment A on T {
  field
  ...B
}

fragment X on T {
  fieldX
}

query Two {
  ...A
  ...Y
  baz
}

fragment Y on T {
  fieldY
}

fragment B on T {
  something
}
`,
			want: map[string]string{
				"": `{
  ...Y
  ...X
}

fragment X on T {
  fieldX
}

fragment Y on T {
  fieldY
}
`,
				"One": `query One {
  foo
  bar
  ...A
  ...X
}

fragment A on T {
  field
  ...B
}

fragment X on T {
  fieldX
}

fragment B on T {
  something
}
`,
				"Two": `fragment A on T {
  field
  ...B
}

query Two {
  ...A
  ...Y
  baz
}

fragment Y on T {
  fieldY
}

fragment B on T {
  something
}
`,
			},
		},
		{
			name: `survives circular dependencies`,
			document: `query One {
  ...A
}

fragment A on T {
  ...B
}

fragment B on T {
  ...A
}

query Two {
  ...B
}
`,
			want: map[string]string{
				"One": `query One {
  ...A
}

fragment A on T {
  ...B
}

fragment B on T {
  ...A
}
`,
				"Two": `fragment A on T {
  ...B
}

fragment B on T {
  ...A
}

query Two {
  ...B
}
`,
			},
		},
		{
			name: `distinguish query and fragment names`,
			document: `{
  ...NameClash
}

fragment NameClash on T {
  oneField
}

query NameClash {
  ...ShouldBeSkippedInFirstQuery
}

fragment ShouldBeSkippedInFirstQuery on T {
  twoField
}
`,
			want: map[string]string{
				"": `{
  ...NameClash
}

fragment NameClash on T {
  oneField
}
`,
				"NameClash": `query NameClash {
  ...ShouldBeSkippedInFirstQuery
}

fragment ShouldBeSkippedInFirstQuery on T {
  twoField
}
`,
			},
		},
		{
			name: `ignores type definitions`,
			document: `query Foo {
  ...Bar
}

fragment Bar on T {
  baz
}

scalar Foo
type Bar`,
			want: map[string]string{
				"Foo": `query Foo {
  ...Bar
}

fragment Bar on T {
  baz
}
`,
			},
		},
		{
			name: `handles unknown fragments`,
			document: `{
  ...Unknown
  ...Known
}

fragment Known on T {
  someField
}
`,
			want: map[string]string{
				"": `{
  ...Unknown
  ...Known
}

fragment Known on T {
  someField
}
`,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := language.ParseString(tt.document)
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}
			got := utilities.SeparateOperations(doc)
			if len(got) != len(tt.want) {
				t.Fatalf("%d operations, want %d", len(got), len(tt.want))
			}
			for name, want := range tt.want {
				part, found := got[name]
				if !found {
					t.Errorf("no operation named %q", name)
					continue
				}
				if written := language.Print(part); written != strings.TrimRight(want, "\n") {
					t.Errorf("%q =\n%s\nwant\n%s", name, written, want)
				}
			}
		})
	}
}
