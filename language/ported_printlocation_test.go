package language_test

// Ported from graphql-js src/language/__tests__/printLocation-test.ts,
// source-test.ts and schemaCoordinateLexer-test.ts.

import (
	"strings"
	"testing"

	"github.com/ikawaha/graphql/language"
)

func TestPortedPrintSourceLocation_MinifiedDocument(t *testing.T) {
	const body = "query SomeMinifiedQueryWithErrorInside($foo:String!=FIRST_ERROR_HERE$bar:String)" +
		"{someField(foo:$foo bar:$bar baz:SECOND_ERROR_HERE){fieldA fieldB{fieldC fieldD" +
		"...on THIRD_ERROR_HERE}}}"
	source := language.NewSource(body)

	tests := []struct {
		marker string
		want   string
	}{
		{
			marker: "FIRST_ERROR_HERE",
			want: `GraphQL request:1:53
1 | query SomeMinifiedQueryWithErrorInside($foo:String!=FIRST_ERROR_HERE$bar:String)
  |                                                     ^
  | {someField(foo:$foo bar:$bar baz:SECOND_ERROR_HERE){fieldA fieldB{fieldC fieldD.`,
		},
		{
			marker: "SECOND_ERROR_HERE",
			want: `GraphQL request:1:114
1 | query SomeMinifiedQueryWithErrorInside($foo:String!=FIRST_ERROR_HERE$bar:String)
  | {someField(foo:$foo bar:$bar baz:SECOND_ERROR_HERE){fieldA fieldB{fieldC fieldD.
  |                                  ^
  | ..on THIRD_ERROR_HERE}}}`,
		},
		{
			marker: "THIRD_ERROR_HERE",
			want: `GraphQL request:1:166
1 | query SomeMinifiedQueryWithErrorInside($foo:String!=FIRST_ERROR_HERE$bar:String)
  | {someField(foo:$foo bar:$bar baz:SECOND_ERROR_HERE){fieldA fieldB{fieldC fieldD.
  | ..on THIRD_ERROR_HERE}}}
  |      ^`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.marker, func(t *testing.T) {
			at := language.SourceLocation{Line: 1, Column: strings.Index(body, tt.marker) + 1}
			if got := language.PrintSourceLocation(source, at); got != tt.want {
				t.Errorf("printed\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestPortedPrintSourceLocation_LineNumberPadding(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "a single digit needs no padding",
			body: "*",
			want: "Test:9:1\n9 | *\n  | ^",
		},
		{
			name: "a following line two digits wide pads the ones before it",
			body: "*\n",
			want: "Test:9:1\n 9 | *\n   | ^\n10 |",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := language.NewSource(tt.body,
				language.SourceName("Test"), language.SourceLocationOffset(9, 1))
			got := language.PrintSourceLocation(source, language.SourceLocation{Line: 1, Column: 1})
			if got != tt.want {
				t.Errorf("printed\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

// graphql-js throws; a location offset is a programming mistake rather than
// something a request can cause, so here it panics.
func TestPortedSource_RefusesAnInvalidLocationOffset(t *testing.T) {
	tests := []struct {
		name   string
		line   int
		column int
		want   string
	}{
		{"line zero", 0, 1, "line in LocationOffset is 1-indexed and must be positive"},
		{"line negative", -1, 1, "line in LocationOffset is 1-indexed and must be positive"},
		{"column zero", 1, 0, "column in LocationOffset is 1-indexed and must be positive"},
		{"column negative", 1, -1, "column in LocationOffset is 1-indexed and must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				got, isString := recover().(string)
				if !isString || !strings.Contains(got, tt.want) {
					t.Errorf("panicked with %v, want a message containing %q", got, tt.want)
				}
			}()
			language.NewSource("", language.SourceLocationOffset(tt.line, tt.column))
			t.Error("building the source did not panic")
		})
	}
}

// A schema coordinate has no ignored tokens: whitespace between its parts is
// an error rather than something skipped.
func TestPortedSchemaCoordinate_ForbidsIgnoredTokens(t *testing.T) {
	tests := []struct {
		body string
		want string
	}{
		{"\nName.field", `Syntax Error: Invalid character: U+000A.`},
		{"Foo .bar", `Syntax Error: Invalid character: " ".`},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			_, err := language.ParseSchemaCoordinate(language.NewSource(tt.body))
			if err == nil {
				t.Fatal("parsed without an error")
			}
			if err.Error() != tt.want {
				t.Errorf("error is %q, want %q", err, tt.want)
			}
		})
	}
}
