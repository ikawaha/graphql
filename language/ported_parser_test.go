package language_test

// Ported from graphql-js src/language/__tests__/parser-test.ts and
// schema-parser-test.ts: what is said about a document that will not parse,
// and where it points.

import (
	"testing"

	"github.com/ikawaha/graphql/language"
)

func TestPortedParser_Errors(t *testing.T) {
	for _, tt := range []struct {
		name, in_, says string
		line, column    int
	}{
		{name: "{ ...MissingOn } fragment MissingOn Type", in_: "\n      { ...MissingOn }\n      fragment MissingOn Type\n    ", says: "Syntax Error: Expected \"on\", found Name \"Type\".", line: 3, column: 26},
		{name: "{ field: {} }", in_: "{ field: {} }", says: "Syntax Error: Expected Name, found \"{\".", line: 1, column: 10},
		{name: "notAnOperation Foo { field }", in_: "notAnOperation Foo { field }", says: "Syntax Error: Unexpected Name \"notAnOperation\".", line: 1, column: 1},
		{name: "...", in_: "...", says: "Syntax Error: Unexpected \"...\".", line: 1, column: 1},
		{name: "{ \"\"", in_: "{ \"\"", says: "Syntax Error: Expected Name, found String \"\".", line: 1, column: 3},
		{name: "fragment on on on { on }", in_: "fragment on on on { on }", says: "Syntax Error: Unexpected Name \"on\".", line: 1, column: 10},
		{name: "{ ...on }", in_: "{ ...on }", says: "Syntax Error: Expected Name, found \"}\".", line: 1, column: 9},
		{name: "enum Test { VALID, true }", in_: "enum Test { VALID, true }", says: "Syntax Error: Name \"true\" is reserved and cannot be used for an enum value.", line: 1, column: 20},
		{name: "enum Test { VALID, false }", in_: "enum Test { VALID, false }", says: "Syntax Error: Name \"false\" is reserved and cannot be used for an enum value.", line: 1, column: 20},
		{name: "enum Test { VALID, null }", in_: "enum Test { VALID, null }", says: "Syntax Error: Name \"null\" is reserved and cannot be used for an enum value.", line: 1, column: 20},
		{name: "\"Description\" 1", in_: "\"Description\" 1", says: "Syntax Error: Unexpected Int \"1\".", line: 1, column: 15},
		{name: "extend scalar Hello", in_: "extend scalar Hello", says: "Syntax Error: Unexpected <EOF>.", line: 1, column: 20},
		{name: "extend type Hello", in_: "extend type Hello", says: "Syntax Error: Unexpected <EOF>.", line: 1, column: 18},
		{name: "extend interface Hello", in_: "extend interface Hello", says: "Syntax Error: Unexpected <EOF>.", line: 1, column: 23},
		{name: "extend union Hello", in_: "extend union Hello", says: "Syntax Error: Unexpected <EOF>.", line: 1, column: 19},
		{name: "extend enum Hello", in_: "extend enum Hello", says: "Syntax Error: Unexpected <EOF>.", line: 1, column: 18},
		{name: "extend input Hello", in_: "extend input Hello", says: "Syntax Error: Unexpected <EOF>.", line: 1, column: 19},
		{name: "\"Description\" extend type Hello { world: String }", in_: "\n      \"Description\"\n      extend type Hello {\n        world: String\n      }\n    ", says: "Syntax Error: Unexpected description, only GraphQL definitions support descriptions.", line: 2, column: 7},
		{name: "extend \"Description\" type Hello { world: String }", in_: "\n      extend \"Description\" type Hello {\n        world: String\n      }\n    ", says: "Syntax Error: Unexpected String \"Description\".", line: 2, column: 14},
		{name: "\"Description\" extend interface Hello { world: String }", in_: "\n      \"Description\"\n      extend interface Hello {\n        world: String\n      }\n    ", says: "Syntax Error: Unexpected description, only GraphQL definitions support descriptions.", line: 2, column: 7},
		{name: "extend \"Description\" interface Hello { world: String }", in_: "\n      extend \"Description\" interface Hello {\n        world: String\n      }\n    ", says: "Syntax Error: Unexpected String \"Description\".", line: 2, column: 14},
		{name: "extend schema", in_: "extend schema", says: "Syntax Error: Unexpected <EOF>.", line: 1, column: 14},
		{name: "extend schema { unknown: SomeType }", in_: "extend schema { unknown: SomeType }", says: "Syntax Error: Unexpected Name \"unknown\".", line: 1, column: 17},
		{name: "union Hello = |", in_: "union Hello = |", says: "Syntax Error: Expected Name, found <EOF>.", line: 1, column: 16},
		{name: "union Hello = || Wo | Rld", in_: "union Hello = || Wo | Rld", says: "Syntax Error: Expected Name, found \"|\".", line: 1, column: 16},
		{name: "union Hello = Wo || Rld", in_: "union Hello = Wo || Rld", says: "Syntax Error: Expected Name, found \"|\".", line: 1, column: 19},
		{name: "union Hello = | Wo | Rld |", in_: "union Hello = | Wo | Rld |", says: "Syntax Error: Expected Name, found <EOF>.", line: 1, column: 27},
		{name: "input Hello { world(foo: Int): String }", in_: "\n      input Hello {\n        world(foo: Int): String\n      }\n    ", says: "Syntax Error: Expected \":\", found \"(\".", line: 3, column: 14},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := language.ParseString(tt.in_)
			if err == nil {
				t.Fatalf("%q parsed without complaint", tt.in_)
			}
			var syntax *language.SyntaxError
			said, line, column := err.Error(), 0, 0
			if errorsAs(err, &syntax) {
				said = syntax.Error()
				line, column = syntax.Location.Line, syntax.Location.Column
			}
			if said != tt.says {
				t.Errorf("said %q, want %q", said, tt.says)
			}
			if line != tt.line || column != tt.column {
				t.Errorf("at %d:%d, want %d:%d", line, column, tt.line, tt.column)
			}
		})
	}
}
