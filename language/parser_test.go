package language

import (
	"errors"
	"strings"
	"testing"
)

func mustParse(t *testing.T, body string, opts ...ParseOption) *Document {
	t.Helper()
	doc, err := ParseString(body, opts...)
	if err != nil {
		t.Fatalf("parsing %q: %v", body, err)
	}
	return doc
}

// parseErr parses body expecting failure and returns the syntax error.
func parseErr(t *testing.T, body string, opts ...ParseOption) *SyntaxError {
	t.Helper()
	doc, err := ParseString(body, opts...)
	if err == nil {
		t.Fatalf("parsing %q succeeded with %d definitions, want a syntax error",
			body, len(doc.Definitions))
	}
	var se *SyntaxError
	if !errors.As(err, &se) {
		t.Fatalf("parsing %q: error is %T, want *SyntaxError", body, err)
	}
	return se
}

// only returns the single definition of a document, failing if there is not
// exactly one.
func only[T Definition](t *testing.T, doc *Document) T {
	t.Helper()
	var zero T
	if len(doc.Definitions) != 1 {
		t.Fatalf("document has %d definitions, want 1", len(doc.Definitions))
	}
	def, ok := doc.Definitions[0].(T)
	if !ok {
		t.Fatalf("definition is %T, want %T", doc.Definitions[0], zero)
	}
	return def
}

func TestParse_ShorthandQuery(t *testing.T) {
	doc := mustParse(t, "{ hero }")
	op := only[*OperationDefinition](t, doc)

	if op.Operation != OperationQuery {
		t.Errorf("Operation = %q, want %q", op.Operation, OperationQuery)
	}
	if op.Name != nil {
		t.Errorf("Name = %v, want nil for a shorthand query", op.Name)
	}
	if got := len(op.SelectionSet.Selections); got != 1 {
		t.Fatalf("%d selections, want 1", got)
	}
	field, ok := op.SelectionSet.Selections[0].(*Field)
	if !ok {
		t.Fatalf("selection is %T, want *Field", op.SelectionSet.Selections[0])
	}
	if field.Name.Value != "hero" {
		t.Errorf("field name = %q, want %q", field.Name.Value, "hero")
	}
}

func TestParse_OperationTypes(t *testing.T) {
	tests := []struct {
		body string
		want OperationType
	}{
		{"query Q { a }", OperationQuery},
		{"mutation M { a }", OperationMutation},
		{"subscription S { a }", OperationSubscription},
	}
	for _, tt := range tests {
		t.Run(string(tt.want), func(t *testing.T) {
			op := only[*OperationDefinition](t, mustParse(t, tt.body))
			if op.Operation != tt.want {
				t.Errorf("Operation = %q, want %q", op.Operation, tt.want)
			}
		})
	}
}

func TestParse_FieldAliasAndArguments(t *testing.T) {
	doc := mustParse(t, `{ alias: hero(episode: JEDI, first: 10) { name } }`)
	op := only[*OperationDefinition](t, doc)
	field := op.SelectionSet.Selections[0].(*Field)

	if field.Alias == nil || field.Alias.Value != "alias" {
		t.Errorf("Alias = %v, want alias", field.Alias)
	}
	if field.Name.Value != "hero" {
		t.Errorf("Name = %q, want hero", field.Name.Value)
	}
	if got := field.ResponseKey(); got != "alias" {
		t.Errorf("ResponseKey() = %q, want alias", got)
	}
	if got := len(field.Arguments); got != 2 {
		t.Fatalf("%d arguments, want 2", got)
	}
	if field.Arguments[0].Name.Value != "episode" {
		t.Errorf("first argument = %q, want episode", field.Arguments[0].Name.Value)
	}
	if _, ok := field.Arguments[0].Value.(*EnumValue); !ok {
		t.Errorf("first argument value is %T, want *EnumValue", field.Arguments[0].Value)
	}
	if field.SelectionSet == nil {
		t.Error("SelectionSet is nil, want a nested selection")
	}
}

// A field without arguments must have a nil slice, not an empty one: nil is
// how the AST says the parentheses were absent.
func TestParse_AbsentListsAreNil(t *testing.T) {
	op := only[*OperationDefinition](t, mustParse(t, "{ hero }"))
	field := op.SelectionSet.Selections[0].(*Field)

	if field.Arguments != nil {
		t.Errorf("Arguments = %v, want nil when there are no parentheses", field.Arguments)
	}
	if field.Directives != nil {
		t.Errorf("Directives = %v, want nil when there are none", field.Directives)
	}
	if field.SelectionSet != nil {
		t.Error("SelectionSet is not nil for a leaf field")
	}
	if op.VariableDefinitions != nil {
		t.Errorf("VariableDefinitions = %v, want nil", op.VariableDefinitions)
	}
}

// The grammar spells an argument list as one or more arguments in
// parentheses, so empty parentheses are a syntax error rather than an empty
// list. The same holds for the other bracketed lists that may be left out
// entirely.
func TestParse_EmptyBracketedListsAreRejected(t *testing.T) {
	for _, body := range []string{
		"{ hero() }",
		"query Q() { f }",
		"type T { f: Int }\ntype U {}",
		"input In {}",
		"enum E {}",
	} {
		t.Run(body, func(t *testing.T) {
			_ = parseErr(t, body)
		})
	}
}

// A list value and an object value, unlike the bracketed lists above, may be
// empty.
func TestParse_EmptyListAndObjectValuesAreAllowed(t *testing.T) {
	for _, body := range []string{"[]", "{}"} {
		v, err := ParseValue(NewSource(body))
		if err != nil {
			t.Fatalf("ParseValue(%q): %v", body, err)
		}
		switch v := v.(type) {
		case *ListValue:
			if len(v.Values) != 0 {
				t.Errorf("%q parsed %d values, want 0", body, len(v.Values))
			}
		case *ObjectValue:
			if len(v.Fields) != 0 {
				t.Errorf("%q parsed %d fields, want 0", body, len(v.Fields))
			}
		default:
			t.Errorf("%q parsed as %T", body, v)
		}
	}
}

func TestParse_VariableDefinitions(t *testing.T) {
	doc := mustParse(t, `query Q($a: Int = 1, $b: [String!]!, $c: Boolean @dir) { f }`)
	op := only[*OperationDefinition](t, doc)

	if got := len(op.VariableDefinitions); got != 3 {
		t.Fatalf("%d variable definitions, want 3", got)
	}

	a := op.VariableDefinitions[0]
	if a.Variable.Name.Value != "a" {
		t.Errorf("first variable = %q, want a", a.Variable.Name.Value)
	}
	if _, ok := a.DefaultValue.(*IntValue); !ok {
		t.Errorf("default value is %T, want *IntValue", a.DefaultValue)
	}

	b := op.VariableDefinitions[1]
	nonNull, ok := b.Type.(*NonNullType)
	if !ok {
		t.Fatalf("second variable type is %T, want *NonNullType", b.Type)
	}
	list, ok := nonNull.Type.(*ListType)
	if !ok {
		t.Fatalf("inner type is %T, want *ListType", nonNull.Type)
	}
	if _, ok := list.Type.(*NonNullType); !ok {
		t.Errorf("list element type is %T, want *NonNullType", list.Type)
	}

	c := op.VariableDefinitions[2]
	if c.DefaultValue != nil {
		t.Errorf("DefaultValue = %v, want nil when there is no default", c.DefaultValue)
	}
	if len(c.Directives) != 1 {
		t.Errorf("%d directives, want 1", len(c.Directives))
	}
}

func TestParse_Values(t *testing.T) {
	tests := []struct {
		name string
		body string
		want Kind
	}{
		{"int", "1", KindIntValue},
		{"negative int", "-1", KindIntValue},
		{"float", "1.5", KindFloatValue},
		{"string", `"s"`, KindStringValue},
		{"block string", `"""s"""`, KindStringValue},
		{"true", "true", KindBooleanValue},
		{"false", "false", KindBooleanValue},
		{"null", "null", KindNullValue},
		{"enum", "JEDI", KindEnumValue},
		{"list", "[1, 2]", KindListValue},
		{"empty list", "[]", KindListValue},
		{"object", "{a: 1}", KindObjectValue},
		{"empty object", "{}", KindObjectValue},
		{"variable", "$v", KindVariable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := ParseValue(NewSource(tt.body))
			if err != nil {
				t.Fatalf("ParseValue(%q): %v", tt.body, err)
			}
			if v.Kind() != tt.want {
				t.Errorf("Kind() = %v, want %v", v.Kind(), tt.want)
			}
		})
	}
}

func TestParse_BlockStringIsMarked(t *testing.T) {
	for _, tt := range []struct {
		body      string
		wantBlock bool
	}{
		{`"s"`, false},
		{`"""s"""`, true},
	} {
		v, err := ParseValue(NewSource(tt.body))
		if err != nil {
			t.Fatalf("ParseValue(%q): %v", tt.body, err)
		}
		s, ok := v.(*StringValue)
		if !ok {
			t.Fatalf("value is %T, want *StringValue", v)
		}
		if s.Block != tt.wantBlock {
			t.Errorf("%s: Block = %v, want %v", tt.body, s.Block, tt.wantBlock)
		}
		if s.Value != "s" {
			t.Errorf("%s: Value = %q, want %q", tt.body, s.Value, "s")
		}
	}
}

func TestParseConstValue_RejectsVariables(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"bare variable", "$v"},
		{"variable in a list", "[$v]"},
		{"variable in an object", "{a: $v}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConstValue(NewSource(tt.body))
			if err == nil {
				t.Fatalf("ParseConstValue(%q) succeeded, want an error", tt.body)
			}
			if !strings.Contains(err.Error(), "in constant value") {
				t.Errorf("error = %v, want it to mention a constant value", err)
			}
		})
	}
}

// A default value is a constant position, so a variable there is a syntax
// error rather than something validation has to catch later.
func TestParse_RejectsVariableInDefaultValue(t *testing.T) {
	err := parseErr(t, `query Q($a: Int = $b) { f }`)
	if !strings.Contains(err.Description, `Unexpected variable "$b" in constant value.`) {
		t.Errorf("description = %q, want it to name the variable", err.Description)
	}
}

func TestParseType(t *testing.T) {
	tests := []struct {
		body string
		want Kind
	}{
		{"Int", KindNamedType},
		{"[Int]", KindListType},
		{"Int!", KindNonNullType},
		{"[Int!]!", KindNonNullType},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			typ, err := ParseType(NewSource(tt.body))
			if err != nil {
				t.Fatalf("ParseType(%q): %v", tt.body, err)
			}
			if typ.Kind() != tt.want {
				t.Errorf("Kind() = %v, want %v", typ.Kind(), tt.want)
			}
			named := NamedTypeOf(typ)
			if named == nil || named.Name.Value != "Int" {
				t.Errorf("NamedTypeOf() = %v, want Int", named)
			}
		})
	}
}

func TestParse_Fragments(t *testing.T) {
	doc := mustParse(t, `
		{
			...Named
			... on Human { height }
			... @skip(if: true) { inline }
		}
		fragment Named on Character { name }
	`)
	if got := len(doc.Definitions); got != 2 {
		t.Fatalf("%d definitions, want 2", got)
	}

	op := doc.Definitions[0].(*OperationDefinition)
	selections := op.SelectionSet.Selections
	if got := len(selections); got != 3 {
		t.Fatalf("%d selections, want 3", got)
	}

	spread, ok := selections[0].(*FragmentSpread)
	if !ok {
		t.Fatalf("first selection is %T, want *FragmentSpread", selections[0])
	}
	if spread.Name.Value != "Named" {
		t.Errorf("spread name = %q, want Named", spread.Name.Value)
	}

	typed, ok := selections[1].(*InlineFragment)
	if !ok {
		t.Fatalf("second selection is %T, want *InlineFragment", selections[1])
	}
	if typed.TypeCondition == nil || typed.TypeCondition.Name.Value != "Human" {
		t.Errorf("type condition = %v, want Human", typed.TypeCondition)
	}

	untyped, ok := selections[2].(*InlineFragment)
	if !ok {
		t.Fatalf("third selection is %T, want *InlineFragment", selections[2])
	}
	if untyped.TypeCondition != nil {
		t.Errorf("type condition = %v, want nil", untyped.TypeCondition)
	}
	if len(untyped.Directives) != 1 {
		t.Errorf("%d directives, want 1", len(untyped.Directives))
	}

	frag := doc.Definitions[1].(*FragmentDefinition)
	if frag.Name.Value != "Named" || frag.TypeCondition.Name.Value != "Character" {
		t.Errorf("fragment = %q on %q, want Named on Character",
			frag.Name.Value, frag.TypeCondition.Name.Value)
	}
}

func TestParse_FragmentNameCannotBeOn(t *testing.T) {
	err := parseErr(t, "fragment on on Type { f }")
	if !strings.Contains(err.Description, "Unexpected") {
		t.Errorf("description = %q, want an unexpected-token error", err.Description)
	}
}

func TestParse_ExperimentalFragmentArguments(t *testing.T) {
	const body = `
		{ t { ...A(x: true) } }
		fragment A($x: Boolean = false) on T { f }
	`
	// Without the option the argument list is not part of the grammar.
	if _, err := ParseString(body); err == nil {
		t.Error("parsing fragment arguments succeeded without the option, want an error")
	}

	doc := mustParse(t, body, ExperimentalFragmentArguments())
	op := doc.Definitions[0].(*OperationDefinition)
	inner := op.SelectionSet.Selections[0].(*Field).SelectionSet.Selections[0]
	spread, ok := inner.(*FragmentSpread)
	if !ok {
		t.Fatalf("selection is %T, want *FragmentSpread", inner)
	}
	if got := len(spread.Arguments); got != 1 {
		t.Fatalf("%d spread arguments, want 1", got)
	}
	if spread.Arguments[0].Name.Value != "x" {
		t.Errorf("argument name = %q, want x", spread.Arguments[0].Name.Value)
	}

	frag := doc.Definitions[1].(*FragmentDefinition)
	if got := len(frag.VariableDefinitions); got != 1 {
		t.Fatalf("%d fragment variable definitions, want 1", got)
	}
}

func TestParse_SchemaDefinitionAndTypes(t *testing.T) {
	doc := mustParse(t, `
		"The schema."
		schema @onSchema {
			query: Query
			mutation: Mutation
		}
		scalar DateTime @specifiedBy(url: "https://example.com")
		type Query implements Node & Named @onObject {
			"A field."
			field(arg: Int = 1 @onArg): String!
		}
		interface Node { id: ID! }
		union Result = A | B
		enum Color { RED GREEN }
		input Filter { limit: Int = 10 }
		directive @auth(role: String) repeatable on FIELD | OBJECT
	`)
	if got := len(doc.Definitions); got != 8 {
		t.Fatalf("%d definitions, want 8", got)
	}

	schema := doc.Definitions[0].(*SchemaDefinition)
	if schema.Description == nil || schema.Description.Value != "The schema." {
		t.Errorf("schema description = %v, want %q", schema.Description, "The schema.")
	}
	if got := len(schema.OperationTypes); got != 2 {
		t.Errorf("%d operation types, want 2", got)
	}

	obj := doc.Definitions[2].(*ObjectTypeDefinition)
	if got := len(obj.Interfaces); got != 2 {
		t.Errorf("%d interfaces, want 2", got)
	}
	if got := len(obj.Fields); got != 1 {
		t.Fatalf("%d fields, want 1", got)
	}
	field := obj.Fields[0]
	if field.Description == nil || field.Description.Value != "A field." {
		t.Errorf("field description = %v, want %q", field.Description, "A field.")
	}
	if got := len(field.Arguments); got != 1 {
		t.Fatalf("%d field arguments, want 1", got)
	}
	if field.Arguments[0].DefaultValue == nil {
		t.Error("argument default value is nil, want an int")
	}

	union := doc.Definitions[4].(*UnionTypeDefinition)
	if got := len(union.Types); got != 2 {
		t.Errorf("%d union members, want 2", got)
	}

	directive := doc.Definitions[7].(*DirectiveDefinition)
	if !directive.Repeatable {
		t.Error("Repeatable = false, want true")
	}
	if got := len(directive.Locations); got != 2 {
		t.Errorf("%d locations, want 2", got)
	}
}

// A union or a directive location list may begin with a leading delimiter.
func TestParse_LeadingDelimiters(t *testing.T) {
	doc := mustParse(t, `
		union Result = | A | B
		directive @d on | FIELD | OBJECT
	`)
	union := doc.Definitions[0].(*UnionTypeDefinition)
	if got := len(union.Types); got != 2 {
		t.Errorf("%d union members, want 2", got)
	}
	directive := doc.Definitions[1].(*DirectiveDefinition)
	if got := len(directive.Locations); got != 2 {
		t.Errorf("%d locations, want 2", got)
	}
}

func TestParse_Extensions(t *testing.T) {
	doc := mustParse(t, `
		extend schema { subscription: Subscription }
		extend scalar S @onScalar
		extend type T { added: Int }
		extend interface I @onInterface
		extend union U = C
		extend enum E { BLUE }
		extend input In { extra: Int }
		extend directive @d @onDirective
	`)
	if got := len(doc.Definitions); got != 8 {
		t.Fatalf("%d definitions, want 8", got)
	}
	for i, def := range doc.Definitions {
		if _, ok := def.(TypeSystemExtension); !ok {
			t.Errorf("definition %d is %T, want a TypeSystemExtension", i, def)
		}
	}
}

// An extension that adds nothing is a mistake, not a no-op.
func TestParse_EmptyExtensionsAreRejected(t *testing.T) {
	for _, body := range []string{
		"extend schema",
		"extend scalar S",
		"extend type T",
		"extend interface I",
		"extend union U",
		"extend enum E",
		"extend input In",
		"extend directive @d",
	} {
		t.Run(body, func(t *testing.T) {
			_ = parseErr(t, body)
		})
	}
}

func TestParse_EnumValueCannotBeReserved(t *testing.T) {
	for _, word := range []string{"true", "false", "null"} {
		t.Run(word, func(t *testing.T) {
			err := parseErr(t, "enum E { "+word+" }")
			if !strings.Contains(err.Description, "reserved and cannot be used for an enum value") {
				t.Errorf("description = %q, want it to say the name is reserved", err.Description)
			}
		})
	}
}

func TestParse_UnknownDirectiveLocationIsRejected(t *testing.T) {
	_ = parseErr(t, "directive @d on NOWHERE")
}

func TestParse_Errors(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"empty document", "", "Unexpected <EOF>."},
		{"unclosed selection set", "{", "Expected Name, found <EOF>."},
		{"missing selection", "{ }", "Expected Name, found \"}\"."},
		{"unknown keyword", "notanoperation Foo { field }", `Unexpected Name "notanoperation".`},
		{"missing colon in argument", "{ f(a 1) }", `Expected ":", found Int "1".`},
		{"description on shorthand query", `"desc" { field }`, "descriptions are not supported on shorthand queries"},
		{"description before extend", `"desc" extend type T { f: Int }`, "only GraphQL definitions support descriptions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseErr(t, tt.body)
			if !strings.Contains(err.Description, tt.want) {
				t.Errorf("description = %q, want it to contain %q", err.Description, tt.want)
			}
		})
	}
}

func TestParse_Locations(t *testing.T) {
	const body = "{ hero }"
	doc := mustParse(t, body)
	op := only[*OperationDefinition](t, doc)
	field := op.SelectionSet.Selections[0].(*Field)

	if loc := field.Location(); loc == nil {
		t.Fatal("field has no location")
	} else {
		if got := body[loc.Start:loc.End]; got != "hero" {
			t.Errorf("field spans %q, want %q", got, "hero")
		}
		if loc.Source == nil {
			t.Error("location has no source")
		}
		if loc.StartToken == nil || loc.EndToken == nil {
			t.Error("location is missing its tokens")
		}
	}

	// The document spans the whole input.
	if loc := doc.Location(); loc == nil {
		t.Fatal("document has no location")
	} else if loc.Start != 0 || loc.End != len(body) {
		t.Errorf("document spans [%d,%d), want [0,%d)", loc.Start, loc.End, len(body))
	}
}

// A node's span must cover exactly the text it was parsed from, so a nested
// node always sits inside its parent.
func TestParse_NestedLocationsNest(t *testing.T) {
	const body = "query Q { a { b } }"
	doc := mustParse(t, body)
	op := doc.Definitions[0].(*OperationDefinition)
	outer := op.SelectionSet.Selections[0].(*Field)
	inner := outer.SelectionSet.Selections[0].(*Field)

	if got := body[outer.Loc.Start:outer.Loc.End]; got != "a { b }" {
		t.Errorf("outer field spans %q, want %q", got, "a { b }")
	}
	if got := body[inner.Loc.Start:inner.Loc.End]; got != "b" {
		t.Errorf("inner field spans %q, want %q", got, "b")
	}
	if inner.Loc.Start < outer.Loc.Start || inner.Loc.End > outer.Loc.End {
		t.Error("inner field is not contained in the outer field")
	}
}

func TestParse_NoLocation(t *testing.T) {
	doc := mustParse(t, "{ hero }", NoLocation())
	if doc.Location() != nil {
		t.Error("document has a location, want none")
	}
	op := doc.Definitions[0].(*OperationDefinition)
	if op.Location() != nil {
		t.Error("operation has a location, want none")
	}
	if op.SelectionSet.Selections[0].Location() != nil {
		t.Error("field has a location, want none")
	}
}

func TestParse_MaxTokens(t *testing.T) {
	const body = "{ a b c }" // brace, three names, brace: five tokens
	if _, err := ParseString(body, MaxTokens(5)); err != nil {
		t.Errorf("parsing with a limit of 5 failed: %v", err)
	}
	err := parseErr(t, body, MaxTokens(4))
	if !strings.Contains(err.Description, "Document contains more than 4 tokens") {
		t.Errorf("description = %q, want it to mention the token limit", err.Description)
	}
}

func TestDocument_TokenCount(t *testing.T) {
	doc := mustParse(t, "{ a b c }")
	if doc.TokenCount != 5 {
		t.Errorf("TokenCount = %d, want 5", doc.TokenCount)
	}
}

func TestParseSchemaCoordinate(t *testing.T) {
	tests := []struct {
		body string
		want Kind
	}{
		{"Type", KindTypeCoordinate},
		{"Type.field", KindMemberCoordinate},
		{"Type.field(arg:)", KindArgumentCoordinate},
		{"@directive", KindDirectiveCoordinate},
		{"@directive(arg:)", KindDirectiveArgumentCoordinate},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			c, err := ParseSchemaCoordinate(NewSource(tt.body))
			if err != nil {
				t.Fatalf("ParseSchemaCoordinate(%q): %v", tt.body, err)
			}
			if c.Kind() != tt.want {
				t.Errorf("Kind() = %v, want %v", c.Kind(), tt.want)
			}
		})
	}

	// A directive coordinate has no member part.
	if _, err := ParseSchemaCoordinate(NewSource("@directive.field")); err == nil {
		t.Error("parsing @directive.field succeeded, want an error")
	}
}

// A syntax error found by the lexer has to travel out of the parser intact.
func TestParse_ReportsLexerErrors(t *testing.T) {
	err := parseErr(t, "{ field(arg: 'quoted') }")
	if !strings.Contains(err.Description, "Unexpected single quote character") {
		t.Errorf("description = %q, want the lexer's message", err.Description)
	}
}

func TestParse_KitchenSink(t *testing.T) {
	body := readFixture(t, "kitchen-sink.graphql")
	doc, err := Parse(NewSource(body, SourceName("kitchen-sink.graphql")))
	if err != nil {
		t.Fatalf("parsing the kitchen sink: %v", err)
	}
	if len(doc.Definitions) == 0 {
		t.Fatal("no definitions parsed")
	}
}

func TestParse_GitHubSchema(t *testing.T) {
	body := readFixture(t, "github-schema.graphql")
	doc, err := Parse(NewSource(body, SourceName("github-schema.graphql")))
	if err != nil {
		t.Fatalf("parsing the GitHub schema: %v", err)
	}
	if len(doc.Definitions) < 100 {
		t.Fatalf("%d definitions, want a large schema", len(doc.Definitions))
	}
}

func BenchmarkParse_GitHubSchema(b *testing.B) {
	body, err := readFixtureBytes("github-schema.graphql")
	if err != nil {
		b.Fatal(err)
	}
	source := NewSource(string(body))
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Parse(source); err != nil {
			b.Fatal(err)
		}
	}
}
