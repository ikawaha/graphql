package language_test

// Ported from graphql-js src/language/__tests__/printer-test.ts: a document
// parsed and written back out.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ikawaha/graphql/language"
)

func TestPortedPrinter(t *testing.T) {
	for _, tt := range []struct{ name, query, want string }{
		{
			name:  `correctly prints non-query operations without name`,
			query: `query { id, name }`,
			want: `{
  id
  name
}`,
		},
		{
			name:  `correctly prints non-query operations without name (2)`,
			query: `mutation { id, name }`,
			want: `mutation {
  id
  name
}`,
		},
		{
			name:  `correctly prints non-query operations without name (3)`,
			query: `query ($foo: TestType) @testDirective { id, name }`,
			want: `query ($foo: TestType) @testDirective {
  id
  name
}`,
		},
		{
			name:  `correctly prints non-query operations without name (4)`,
			query: `mutation ($foo: TestType) @testDirective { id, name }`,
			want: `mutation ($foo: TestType) @testDirective {
  id
  name
}`,
		},
		{
			name:  `prints query with variable directives`,
			query: `query ($foo: TestType = { a: 123 } @testDirective(if: true) @test) { id }`,
			want: `query ($foo: TestType = { a: 123 } @testDirective(if: true) @test) {
  id
}`,
		},
		{
			name:  `prints fragment with argument definition directives`,
			query: `fragment Foo($foo: TestType @test) on TestType @testDirective { id }`,
			want: `fragment Foo($foo: TestType @test) on TestType @testDirective {
  id
}`,
		},
		{
			name: `correctly prints fragment defined arguments`,
			query: `fragment Foo($a: ComplexType, $b: Boolean = false) on TestType {
  id
}
`,
			want: `fragment Foo($a: ComplexType, $b: Boolean = false) on TestType {
  id
}`,
		},
		{
			name:  `prints fragment spread with arguments`,
			query: `fragment Foo on TestType { ...Bar(a: {x: $x}, b: true) }`,
			want: `fragment Foo on TestType {
  ...Bar(a: { x: $x }, b: true)
}`,
		},
		{
			name:  `prints fragment spread with multi-line arguments`,
			query: `fragment Foo on TestType { ...Bar(a: {x: $x, y: $y, z: $z, xy: $xy}, b: true, c: "a long string extending arguments over max length") }`,
			want: `fragment Foo on TestType {
  ...Bar(
    a: { x: $x, y: $y, z: $z, xy: $xy }
    b: true
    c: "a long string extending arguments over max length"
  )
}`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := language.ParseString(tt.query,
				language.ExperimentalFragmentArguments())
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}
			if got := language.Print(doc); got != tt.want {
				t.Errorf("wrote\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

// The whole kitchen sink, written back out. graphql-js's expected text is
// reproduced exactly, which is worth stating because value literals are one
// place where the two printers otherwise differ.
func TestPortedPrinter_KitchenSinkQuery(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "kitchen-sink-query.graphql"))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	doc, err := language.ParseString(string(body), language.NoLocation())
	if err != nil {
		t.Fatalf("parsing the fixture: %v", err)
	}

	const want = `"Query description"
query queryName(
"Very complex variable"
$foo: ComplexType
$site: Site = MOBILE
) @onQuery {
  whoever123is: node(id: [123, 456]) {
    id
    ... on User @onInlineFragment {
      field2 {
        id
        alias: field1(first: 10, after: $foo) @include(if: $foo) {
          id
          ...frag @onFragmentSpread
        }
      }
    }
    ... @skip(unless: $foo) {
      id
    }
    ... {
      id
    }
  }
}

mutation likeStory @onMutation {
  like(story: 123) @onField {
    story {
      id @onField
    }
  }
}

subscription StoryLikeSubscription($input: StoryLikeSubscribeInput @onVariableDefinition) @onSubscription {
  storyLikeSubscribe(input: $input) {
    story {
      likers {
        count
      }
      likeSentence {
        text
      }
    }
  }
}

"""Fragment description"""
fragment frag on Friend @onFragmentDefinition {
  foo(
    size: $size
    bar: $b
    obj: { key: "value", block: """
    block string uses \"""
    """ }
  )
}

{
  unnamed(truthy: true, falsy: false, nullish: null)
  query
}

{
  __typename
}`

	got := language.Print(doc)
	if got != want {
		t.Errorf("wrote\n%s\nwant\n%s", got, want)
	}

	// What it wrote reads back as the same document, and writes the same way
	// again.
	again, err := language.ParseString(got, language.NoLocation())
	if err != nil {
		t.Fatalf("parsing what was written: %v", err)
	}
	if second := language.Print(again); second != got {
		t.Errorf("writing it a second time gave\n%s", second)
	}
}

// The SDL kitchen sink, written back out. The expected text is graphql-js's
// own, so this says the two printers agree on a document using every
// type-system construct: where a description collapses onto one line, where a
// union breaks across lines, and how a value literal is spaced.
func TestPortedPrinter_KitchenSinkSDL(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "kitchen-sink-sdl.graphql"))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "kitchen-sink-sdl-printed.graphql"))
	if err != nil {
		t.Fatalf("reading what it should print as: %v", err)
	}
	doc, err := language.ParseString(string(body), language.NoLocation())
	if err != nil {
		t.Fatalf("parsing the fixture: %v", err)
	}

	got := language.Print(doc)
	if got != string(want) {
		t.Errorf("wrote\n%s\nwant\n%s", got, want)
	}

	// What it wrote reads back as the same document, and writes the same way
	// again.
	again, err := language.ParseString(got, language.NoLocation())
	if err != nil {
		t.Fatalf("parsing what was written: %v", err)
	}
	if second := language.Print(again); second != got {
		t.Errorf("writing it a second time gave\n%s", second)
	}
}

// A node built in Go rather than parsed still prints.
func TestPortedPrinter_MinimalNode(t *testing.T) {
	node := &language.ScalarTypeDefinition{Name: &language.Name{Value: "foo"}}
	if got := language.Print(node); got != "scalar foo" {
		t.Errorf("wrote %q, want %q", got, "scalar foo")
	}
}
