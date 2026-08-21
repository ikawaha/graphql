package validation_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
	"github.com/ikawaha/graphql/validation"
)

// The schema below is the one nearly every rule is tested against. It is
// deliberately awkward: interfaces implementing interfaces, a union, arguments
// of every kind and defaults in every combination, because a rule is only
// interesting where the schema gives it something to disagree with.
const testSDL = `
	interface Mammal {
		mother: Mammal
		father: Mammal
	}

	interface Pet {
		name(surname: Boolean): String
	}

	interface Canine implements Mammal {
		name(surname: Boolean): String
		mother: Canine
		father: Canine
	}

	enum DogCommand { SIT HEEL DOWN }

	type Dog implements Pet & Mammal & Canine {
		name(surname: Boolean): String
		nickname: String
		barkVolume: Int
		barks: Boolean
		doesKnowCommand(dogCommand: DogCommand): Boolean
		isHouseTrained(atOtherHomes: Boolean = true): Boolean
		isAtLocation(x: Int, y: Int): Boolean
		mother: Dog
		father: Dog
	}

	type Cat implements Pet {
		name(surname: Boolean): String
		nickname: String
		meows: Boolean
		meowsVolume: Int
		furColor: FurColor
	}

	union CatOrDog = Cat | Dog

	type Human {
		name(surname: Boolean): String
		pets: [Pet]
		relatives: [Human]!
	}

	enum FurColor { BROWN BLACK TAN SPOTTED NO_FUR UNKNOWN }

	input ComplexInput {
		requiredField: Boolean!
		nonNullField: Boolean! = false
		intField: Int
		stringField: String
		booleanField: Boolean
		stringListField: [String]
	}

	input OneOfInput @oneOf {
		stringField: String
		intField: Int
	}

	type ComplicatedArgs {
		intArgField(intArg: Int): String
		nonNullIntArgField(nonNullIntArg: Int!): String
		stringArgField(stringArg: String): String
		booleanArgField(booleanArg: Boolean): String
		enumArgField(enumArg: FurColor): String
		floatArgField(floatArg: Float): String
		idArgField(idArg: ID): String
		stringListArgField(stringListArg: [String]): String
		stringListNonNullArgField(stringListNonNullArg: [String!]): String
		complexArgField(complexArg: ComplexInput): String
		oneOfArgField(oneOfArg: OneOfInput): String
		multipleReqs(req1: Int!, req2: Int!): String
		nonNullFieldWithDefault(arg: Int! = 0): String
		multipleOpts(opt1: Int = 0, opt2: Int = 0): String
		multipleOptAndReq(req1: Int!, req2: Int!, opt1: Int = 0, opt2: Int = 0): String
	}

	type Query {
		human(id: ID): Human
		alien: Alien
		dog: Dog
		cat: Cat
		pet: Pet
		catOrDog: CatOrDog
		complicatedArgs: ComplicatedArgs
	}

	type Alien {
		name(surname: Boolean): String
		homePlanet: String
	}

	type Mutation {
		testMutation(arg: String): String
	}

	type Subscription {
		newMessage: Message
		disallowedSecondRootField: Boolean
	}

	type Message {
		body: String
		sender: String
	}

	directive @onQuery on QUERY
	directive @onMutation on MUTATION
	directive @onSubscription on SUBSCRIPTION
	directive @onField on FIELD
	directive @onFragmentDefinition on FRAGMENT_DEFINITION
	directive @onFragmentSpread on FRAGMENT_SPREAD
	directive @onInlineFragment on INLINE_FRAGMENT
	directive @onVariableDefinition on VARIABLE_DEFINITION
	directive @onSchema on SCHEMA
	directive @onScalar on SCALAR
	directive @onObject on OBJECT
	directive @onFieldDefinition on FIELD_DEFINITION
	directive @onArgumentDefinition on ARGUMENT_DEFINITION
	directive @onInterface on INTERFACE
	directive @onUnion on UNION
	directive @onEnum on ENUM
	directive @onEnumValue on ENUM_VALUE
	directive @onInputObject on INPUT_OBJECT
	directive @onInputFieldDefinition on INPUT_FIELD_DEFINITION
	directive @repeatableDirective(arg: Int) repeatable on FIELD | OBJECT`

var (
	testSchemaOnce sync.Once
	testSchemaVal  *schema.Schema
	testSchemaErr  error
)

// testSchema is the schema rules are checked against. It is built once,
// because building it for each of several hundred test cases would cost more
// than the tests themselves.
func testSchema(t testing.TB) *schema.Schema {
	t.Helper()
	testSchemaOnce.Do(func() {
		testSchemaVal, testSchemaErr = utilities.BuildSchema(testSDL)
		if testSchemaErr == nil {
			testSchemaErr = schema.AssertValidSchema(testSchemaVal)
		}
	})
	if testSchemaErr != nil {
		t.Fatalf("building the test schema: %v", testSchemaErr)
	}
	return testSchemaVal
}

// TestHarness_SchemaIsSound checks the schema the rest of the tests lean on.
// A rule test that fails because the schema is wrong would be read as a
// failure of the rule, so the schema is checked on its own first.
func TestHarness_SchemaIsSound(t *testing.T) {
	s := testSchema(t)
	for _, name := range []string{"Dog", "Cat", "Human", "ComplicatedArgs", "Query", "Subscription"} {
		if s.Type(name) == nil {
			t.Errorf("the test schema has no %s", name)
		}
	}
	if s.Directive("onField") == nil {
		t.Error("the test schema has no @onField")
	}
}

// at is a place in the document an error is expected to point at.
type at struct {
	line, column int
}

// want describes an error a rule is expected to report.
//
// Following decision 13, what is compared is where the error points, not how
// it is worded: matching graphql-js message for message would mean porting
// several thousand lines of English with no gain in what is actually caught.
// Message is an optional substring, for the cases where two different problems
// point at the same place and only the wording tells them apart.
type want struct {
	Message string
	At      []at
}

// dedent removes the indentation a test document carries because it is
// written inside Go source, so that the line and column an error points at are
// the ones the document would have on its own.
func dedent(s string) string {
	lines := strings.Split(s, "\n")
	// A raw string in Go source begins and ends with a line that is only the
	// indentation of the surrounding code, and neither is part of the document.
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	prefix := ""
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lead := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if prefix == "" && i == 0 {
			prefix = lead
			continue
		}
		// The shallowest line decides, so that nothing loses meaningful
		// indentation of its own.
		if len(lead) < len(prefix) {
			prefix = lead
		}
	}
	for i, line := range lines {
		lines[i] = strings.TrimPrefix(line, prefix)
	}
	return strings.Join(lines, "\n")
}

// expectErrors runs a rule over a document and compares what it reports.
//
// Fragment arguments are parsed because some rules exist to cope with them;
// a document that does not use them is unaffected by the option.
func expectErrors(t *testing.T, s *schema.Schema, rule validation.Rule, query string, wants ...want) {
	t.Helper()
	query = dedent(query)
	doc, err := language.ParseString(query, language.ExperimentalFragmentArguments())
	if err != nil {
		t.Fatalf("parsing the test document: %v\n%s", err, query)
	}
	compare(t, validation.Validate(s, doc, rule), wants, query)
}

// expectErrorsAsWritten is [expectErrors] without removing the document's
// indentation.
//
// The cases ported from graphql-js record the line and column their own parse
// produced, which counts the blank first line and the indentation of the
// TypeScript source. Taking those away here would move every location, so a
// ported document is parsed exactly as it was written.
func expectErrorsAsWritten(t *testing.T, s *schema.Schema, rule validation.Rule, query string, wants ...want) {
	t.Helper()
	doc, err := language.ParseString(query, language.ExperimentalFragmentArguments())
	if err != nil {
		t.Fatalf("parsing the test document: %v\n%s", err, query)
	}
	compare(t, validation.Validate(s, doc, rule), wants, query)
}

// expectSDLErrorsAsWritten is [expectSDLErrors] without removing the
// document's indentation.
func expectSDLErrorsAsWritten(t *testing.T, toExtend *schema.Schema, rule validation.Rule, sdl string, wants ...want) {
	t.Helper()
	doc, err := language.ParseString(sdl, language.ExperimentalFragmentArguments())
	if err != nil {
		t.Fatalf("parsing the test document: %v\n%s", err, sdl)
	}
	compare(t, validation.ValidateSDL(doc, toExtend, rule), wants, sdl)
}

// expectValid runs a rule over a document that should pass it.
func expectValid(t *testing.T, s *schema.Schema, rule validation.Rule, query string) {
	t.Helper()
	expectErrors(t, s, rule, query)
}

// expectSDLErrors runs a rule over a document of type definitions.
func expectSDLErrors(t *testing.T, toExtend *schema.Schema, rule validation.Rule, sdl string, wants ...want) {
	t.Helper()
	sdl = dedent(sdl)
	doc, err := language.ParseString(sdl)
	if err != nil {
		t.Fatalf("parsing the test document: %v\n%s", err, sdl)
	}
	compare(t, validation.ValidateSDL(doc, toExtend, rule), wants, sdl)
}

// expectValidSDL runs a rule over a document of type definitions that should
// pass it.
func expectValidSDL(t *testing.T, toExtend *schema.Schema, rule validation.Rule, sdl string) {
	t.Helper()
	expectSDLErrors(t, toExtend, rule, sdl)
}

// compare checks reported errors against what was expected, reporting the
// whole picture at once rather than stopping at the first difference.
func compare(t *testing.T, got []*gqlerror.Error, wants []want, source string) {
	t.Helper()
	if len(got) != len(wants) {
		t.Errorf("%d errors, want %d:\n%s\nreported:\n%s",
			len(got), len(wants), indent(source), describeErrors(got))
		return
	}
	for i, w := range wants {
		g := got[i]
		if w.Message != "" && !strings.Contains(g.Message, w.Message) {
			t.Errorf("error %d: message = %q, want it to contain %q", i, g.Message, w.Message)
		}
		if w.At == nil {
			continue
		}
		if len(g.Locations) != len(w.At) {
			t.Errorf("error %d (%s): points at %v, want %d place(s)",
				i, g.Message, formatLocations(g), len(w.At))
			continue
		}
		for j, a := range w.At {
			loc := g.Locations[j]
			if loc.Line != a.line || loc.Column != a.column {
				t.Errorf("error %d (%s): location %d = %d:%d, want %d:%d",
					i, g.Message, j, loc.Line, loc.Column, a.line, a.column)
			}
		}
	}
}

// describeErrors renders what was reported, for a failure message.
func describeErrors(errs []*gqlerror.Error) string {
	if len(errs) == 0 {
		return "  (nothing)"
	}
	var b strings.Builder
	for _, e := range errs {
		fmt.Fprintf(&b, "  %s %s\n", formatLocations(e), e.Message)
	}
	return strings.TrimRight(b.String(), "\n")
}

// formatLocations renders where an error points.
func formatLocations(e *gqlerror.Error) string {
	if len(e.Locations) == 0 {
		return "(nowhere)"
	}
	parts := make([]string, len(e.Locations))
	for i, loc := range e.Locations {
		parts[i] = fmt.Sprintf("%d:%d", loc.Line, loc.Column)
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// indent shifts a document right, so it reads as a quotation in a failure.
func indent(s string) string {
	return "  " + strings.ReplaceAll(strings.TrimSpace(s), "\n", "\n  ")
}

// expectErrorsWithoutSuggestions is [expectErrors] with the suggestions turned
// off, which is what a server hiding its schema asks for.
func expectErrorsWithoutSuggestions(
	t *testing.T, s *schema.Schema, rule validation.Rule, query string, wants ...want,
) {
	t.Helper()
	query = dedent(query)
	doc, err := language.ParseString(query, language.ExperimentalFragmentArguments())
	if err != nil {
		t.Fatalf("parsing the test document: %v\n%s", err, query)
	}
	compare(t, validation.ValidateWithOptions(s, doc,
		validation.WithRules(rule), validation.WithoutSuggestions()), wants, query)
}
