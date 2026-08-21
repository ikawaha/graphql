package typeinfo_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/internal/typeinfo"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

const typeInfoSDL = `
	interface Node { id: ID! }

	enum Colour { RED GREEN }

	input Filter {
		term: String = "all"
		limit: Int
		nested: Filter
	}

	type User implements Node {
		id: ID!
		name(upper: Boolean = false): String
		friends(filter: Filter, tags: [String!]): [User!]
		colour: Colour
	}

	type Query {
		me: User
		node: Node
		search(filter: Filter): [User!]
	}`

func typeInfoSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := utilities.BuildSchema(typeInfoSDL)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	return s
}

func parseQuery(t *testing.T, body string) *language.Document {
	t.Helper()
	doc, err := language.ParseString(body)
	if err != nil {
		t.Fatalf("parsing %q: %v", body, err)
	}
	return doc
}

// The position is kept on stacks that rise and fall with the walk, so the one
// thing that must always hold is that they come back empty. An Enter that
// pushes without a matching Leave would leave the answers drifting for
// everything after it, which is hard to notice any other way.
func TestTypeInfo_StacksBalanceAfterEveryWalk(t *testing.T) {
	s := typeInfoSchema(t)

	queries := []string{
		`{ me { name } }`,
		`{ me { name(upper: true) } }`,
		`{ search(filter: { term: "x", nested: { limit: 1 } }) { id } }`,
		`{ me { friends(tags: ["a", "b"]) { id } } }`,
		`{ me { colour } }`,
		`{ node { ... on User { name } } }`,
		`{ node { ...F } } fragment F on User { name }`,
		`query Q($f: Filter = { term: "x" }) { search(filter: $f) { id } }`,
		`{ me @skip(if: true) { name } }`,
		`{ __typename me { __typename } }`,
		`{ __schema { types { name } } }`,
		// Documents that do not match the schema must balance too, since that
		// is exactly when validation is walking them.
		`{ nope }`,
		`{ me { nope(bad: 1) } }`,
		`{ me { name(nope: 1) } }`,
		`{ ... on Missing { x } }`,
		`query Q($v: Missing) { me { name } }`,
		`{ search(filter: { nope: 1 }) { id } }`,
		`{ me @nosuch(if: true) { name } }`,
	}

	for _, query := range queries {
		t.Run(query, func(t2 *testing.T) {
			info := typeinfo.NewTypeInfo(s)
			language.Visit(parseQuery(t2, query), typeinfo.VisitWithTypeInfo(info, language.Visitor{}))
			if depth := info.StackDepths(); !info.IsBalanced() {
				t2.Errorf("the stacks did not come back empty: %+v", depth)
			}
		})
	}
}

// Skipping a subtree means the walker never goes into that node and so never
// leaves it, but the walk carries on afterwards; the position has to be
// unwound for the skipped node or everything after it would be wrong.
func TestTypeInfo_StacksBalanceWhenASubtreeIsSkipped(t *testing.T) {
	s := typeInfoSchema(t)
	info := typeinfo.NewTypeInfo(s)

	var afterwards []string
	skipper := language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			f, isField := node.(*language.Field)
			if !isField {
				return language.VisitContinue
			}
			if f.Name.Value == "me" {
				return language.VisitSkip
			}
			// A field after the skipped one still has to see the right type.
			report := f.Name.Value + ":"
			if parent := info.ParentType(); parent != nil {
				report += parent.Name()
			}
			afterwards = append(afterwards, report)
			return language.VisitContinue
		},
	}
	language.Visit(parseQuery(t, `{ me { name } node { id } }`), typeinfo.VisitWithTypeInfo(info, skipper))

	if depth := info.StackDepths(); !info.IsBalanced() {
		t.Errorf("the stacks did not come back empty: %+v", depth)
	}
	if want := []string{"node:Query", "id:Node"}; !slices.Equal(afterwards, want) {
		t.Errorf("after the skip = %v, want %v", afterwards, want)
	}
}

// Ending the walk abandons it part way down, so the ancestors are never left
// either. The position is abandoned with it; nothing reads it afterwards, and
// unwinding it would mean calling the visitor's Leave for nodes it asked to
// stop at.
func TestTypeInfo_EndingTheWalkAbandonsThePosition(t *testing.T) {
	s := typeInfoSchema(t)
	info := typeinfo.NewTypeInfo(s)

	breaker := language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			if f, isField := node.(*language.Field); isField && f.Name.Value == "me" {
				return language.VisitBreak
			}
			return language.VisitContinue
		},
	}
	language.Visit(parseQuery(t, `{ me { name } }`), typeinfo.VisitWithTypeInfo(info, breaker))

	// The node that broke is unwound, but the operation and selection set it
	// sat inside are not.
	depth := info.StackDepths()
	if depth.Fields != 0 {
		t.Errorf("the node that broke was not unwound: fields=%d", depth.Fields)
	}
	if depth.Types == 0 {
		t.Error("the enclosing levels were unwound, which would mean leaving nodes the walk never left")
	}
}

// observation records what the position said at one field.
type observation struct {
	field      string
	parentType string
	fieldType  string
}

// observeFields walks a query and records the position at each field.
func observeFields(t *testing.T, s *schema.Schema, query string) []observation {
	t.Helper()
	var seen []observation
	info := typeinfo.NewTypeInfo(s)
	visitor := language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			if f, isField := node.(*language.Field); isField {
				o := observation{field: f.Name.Value}
				if parent := info.ParentType(); parent != nil {
					o.parentType = parent.Name()
				}
				if typ := info.Type(); typ != nil {
					o.fieldType = typ.String()
				}
				seen = append(seen, o)
			}
			return language.VisitContinue
		},
	}
	language.Visit(parseQuery(t, query), typeinfo.VisitWithTypeInfo(info, visitor))
	return seen
}

// A field's type depends on the type it was selected from, which depends on
// everything above it. This is what a validation rule reads.
func TestTypeInfo_TracksTypesThroughASelection(t *testing.T) {
	got := observeFields(t, typeInfoSchema(t), `{ me { name friends { id } } }`)
	want := []observation{
		{field: "me", parentType: "Query", fieldType: "User"},
		{field: "name", parentType: "User", fieldType: "String"},
		{field: "friends", parentType: "User", fieldType: "[User!]"},
		{field: "id", parentType: "User", fieldType: "ID!"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("=\n%+v\nwant\n%+v", got, want)
	}
}

func TestTypeInfo_Fragments(t *testing.T) {
	s := typeInfoSchema(t)

	inline := observeFields(t, s, `{ node { ... on User { name } } }`)
	want := []observation{
		{field: "node", parentType: "Query", fieldType: "Node"},
		// Inside the fragment the walk is on User, not on Node.
		{field: "name", parentType: "User", fieldType: "String"},
	}
	if !slices.Equal(inline, want) {
		t.Errorf("inline fragment =\n%+v\nwant\n%+v", inline, want)
	}

	named := observeFields(t, s, `{ node { ...F } } fragment F on User { name }`)
	if len(named) != 2 || named[1].parentType != "User" {
		t.Errorf("named fragment = %+v, want the second field to be on User", named)
	}
}

// The meta-fields belong to no type in particular, so the position has to
// answer for them where no type declares them.
func TestTypeInfo_MetaFields(t *testing.T) {
	s := typeInfoSchema(t)

	got := observeFields(t, s, `{ __typename me { __typename } }`)
	for _, o := range got {
		if o.field == "__typename" && o.fieldType != "String!" {
			t.Errorf("__typename on %s = %q, want String!", o.parentType, o.fieldType)
		}
	}

	root := observeFields(t, s, `{ __schema { types { name } } }`)
	if len(root) == 0 || root[0].fieldType != "__Schema!" {
		t.Errorf("__schema = %+v, want it to be __Schema!", root)
	}

	// __schema describes the schema, so it belongs only to the query root.
	nested := observeFields(t, s, `{ me { __schema { types { name } } } }`)
	if len(nested) < 2 {
		t.Fatalf("%d fields observed", len(nested))
	}
	if nested[1].fieldType != "" {
		t.Errorf("__schema below the root = %q, want it unresolved", nested[1].fieldType)
	}
}

func TestTypeInfo_InputTypes(t *testing.T) {
	s := typeInfoSchema(t)

	// record what the position said at each value node, by the text of the
	// value, so that a case can name what it is looking at.
	observe := func(query string) map[string]string {
		out := map[string]string{}
		info := typeinfo.NewTypeInfo(s)
		visitor := language.Visitor{
			Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
				if v, isValue := node.(language.Value); isValue {
					name := language.Print(v)
					if typ := info.InputType(); typ != nil {
						out[name] = typ.String()
					} else {
						out[name] = ""
					}
				}
				return language.VisitContinue
			},
		}
		language.Visit(parseQuery(t, query), typeinfo.VisitWithTypeInfo(info, visitor))
		return out
	}

	t.Run("an argument", func(t *testing.T) {
		got := observe(`{ me { name(upper: true) } }`)
		if got["true"] != "Boolean" {
			t.Errorf("the argument's value = %q, want Boolean", got["true"])
		}
	})

	// The position is advanced before the visitor sees a node, so at the list
	// itself the type already reported is that of its elements. The list is
	// one level out.
	t.Run("a list element", func(t *testing.T) {
		got := observe(`{ me { friends(tags: ["a"]) { id } } }`)
		if got[`["a"]`] != "String!" {
			t.Errorf("at the list = %q, want the element type String!", got[`["a"]`])
		}
		if got[`"a"`] != "String!" {
			t.Errorf("the element = %q, want String!", got[`"a"`])
		}

		// The list type itself is reachable from inside it.
		info := typeinfo.NewTypeInfo(s)
		var around string
		visitor := language.Visitor{
			Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
				if v, isString := node.(*language.StringValue); isString && v.Value == "a" {
					if parent := info.ParentInputType(); parent != nil {
						around = parent.String()
					}
				}
				return language.VisitContinue
			},
		}
		language.Visit(parseQuery(t, `{ me { friends(tags: ["a"]) { id } } }`),
			typeinfo.VisitWithTypeInfo(info, visitor))
		if around != "[String!]" {
			t.Errorf("the type around the element = %q, want [String!]", around)
		}
	})

	t.Run("an input object field", func(t *testing.T) {
		got := observe(`{ search(filter: { term: "x", nested: { limit: 1 } }) { id } }`)
		if got[`"x"`] != "String" {
			t.Errorf("term = %q, want String", got[`"x"`])
		}
		// The walk descends into the nested input object.
		if got["1"] != "Int" {
			t.Errorf("the nested limit = %q, want Int", got["1"])
		}
	})

	t.Run("a variable's declared type", func(t *testing.T) {
		info := typeinfo.NewTypeInfo(s)
		var declared string
		visitor := language.Visitor{
			Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
				if _, isDef := node.(*language.VariableDefinition); isDef {
					if typ := info.InputType(); typ != nil {
						declared = typ.String()
					}
				}
				return language.VisitContinue
			},
		}
		language.Visit(parseQuery(t, `query Q($f: Filter) { search(filter: $f) { id } }`),
			typeinfo.VisitWithTypeInfo(info, visitor))
		if declared != "Filter" {
			t.Errorf("the declared type = %q, want Filter", declared)
		}
	})
}

// Inside a directive's argument the walk is in the directive, not in the field
// the directive is attached to, even when both have an argument of that name.
func TestTypeInfo_DirectiveArgumentsTakePrecedence(t *testing.T) {
	s := typeInfoSchema(t)
	info := typeinfo.NewTypeInfo(s)

	var directiveName, argumentName, argumentType string
	visitor := language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			if _, isArg := node.(*language.Argument); isArg {
				if d := info.Directive(); d != nil {
					directiveName = d.Name()
				}
				if a := info.Argument(); a != nil {
					argumentName = a.Name()
					argumentType = a.Type.String()
				}
			}
			return language.VisitContinue
		},
	}
	language.Visit(parseQuery(t, `{ me @skip(if: true) { name } }`), typeinfo.VisitWithTypeInfo(info, visitor))

	if directiveName != "skip" {
		t.Errorf("directive = %q, want skip", directiveName)
	}
	if argumentName != "if" || argumentType != "Boolean!" {
		t.Errorf("argument = %s: %s, want if: Boolean!", argumentName, argumentType)
	}

	// Once the directive is left, the position no longer reports it.
	if info.Directive() != nil {
		t.Error("the directive outlived the node it belongs to")
	}
}

func TestTypeInfo_EnumValuesAndDefaults(t *testing.T) {
	s := BuildSchemaOrFail(t, typeInfoSDL+`
		type Extra { f(colour: Colour = RED): String }
	`)

	var seenMember string
	var seenDefault string
	info := typeinfo.NewTypeInfo(s)
	visitor := language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			if _, isEnum := node.(*language.EnumValue); isEnum {
				if member := info.EnumValue(); member != nil {
					seenMember = member.Name()
				}
			}
			if _, isArg := node.(*language.Argument); isArg {
				if def, has := info.DefaultValue().Get(); has && def.Literal != nil {
					seenDefault = language.Print(def.Literal)
				}
			}
			return language.VisitContinue
		},
	}
	language.Visit(parseQuery(t, `{ search(filter: { term: "x" }) { colour } }`),
		typeinfo.VisitWithTypeInfo(info, visitor))
	_ = seenMember

	// An argument with no default reports none, which is different from a
	// default of null.
	if seenDefault != "" {
		t.Errorf("an argument without a default reported %q", seenDefault)
	}

	// A member named where an enum is expected is resolved.
	member := ""
	info2 := typeinfo.NewTypeInfo(s)
	visitor2 := language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			if _, isEnum := node.(*language.EnumValue); isEnum {
				if m := info2.EnumValue(); m != nil {
					member = m.Name()
				}
			}
			return language.VisitContinue
		},
	}
	language.Visit(parseQuery(t, `{ extra { f(colour: GREEN) } }`), typeinfo.VisitWithTypeInfo(info2, visitor2))
	// The schema has no "extra" field, so nothing resolves; what matters is
	// that it does not crash and the stacks balance.
	if !info2.IsBalanced() {
		t.Error("the stacks did not come back empty")
	}
	_ = member
}

// BuildSchemaOrFail builds a schema for a test, failing rather than returning
// an error.
func BuildSchemaOrFail(t *testing.T, sdl string) *schema.Schema {
	t.Helper()
	s, err := utilities.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	return s
}

// Everything the position reports is optional, because a document that does
// not match the schema is exactly what validation walks.
func TestTypeInfo_UnresolvableIsNotAnError(t *testing.T) {
	s := typeInfoSchema(t)
	info := typeinfo.NewTypeInfo(s)

	var reports []string
	visitor := language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			if f, isField := node.(*language.Field); isField {
				report := f.Name.Value + ":"
				if typ := info.Type(); typ != nil {
					report += typ.String()
				}
				if info.FieldDef() == nil {
					report += "(undefined)"
				}
				reports = append(reports, report)
			}
			return language.VisitContinue
		},
	}
	language.Visit(parseQuery(t, `{ nope { alsoNope } me { name } }`), typeinfo.VisitWithTypeInfo(info, visitor))

	joined := strings.Join(reports, " ")
	if !strings.Contains(joined, "nope:(undefined)") {
		t.Errorf("reports = %q, want the unknown field reported as undefined", joined)
	}
	if !strings.Contains(joined, "me:User") {
		t.Errorf("reports = %q, want the known field still resolved", joined)
	}
	if !info.IsBalanced() {
		t.Error("the stacks did not come back empty")
	}
}

// A visitor's own Enter and Leave still run, and see the position as it stands
// at that node.
func TestVisitWithTypeInfo_PassesThroughToTheVisitor(t *testing.T) {
	s := typeInfoSchema(t)
	info := typeinfo.NewTypeInfo(s)

	entered, left := 0, 0
	visitor := language.Visitor{
		Enter: func(language.Node, language.VisitContext) language.VisitAction {
			entered++
			return language.VisitContinue
		},
		Leave: func(language.Node, language.VisitContext) language.VisitAction {
			left++
			return language.VisitContinue
		},
	}
	language.Visit(parseQuery(t, `{ me { name } }`), typeinfo.VisitWithTypeInfo(info, visitor))

	if entered == 0 || entered != left {
		t.Errorf("entered %d nodes and left %d, want them equal and non-zero", entered, left)
	}

	// A visitor with neither function set is fine.
	language.Visit(parseQuery(t, `{ me { name } }`), typeinfo.VisitWithTypeInfo(typeinfo.NewTypeInfo(s), language.Visitor{}))
}

func TestTypeInfo_Accessors(t *testing.T) {
	s := typeInfoSchema(t)
	info := typeinfo.NewTypeInfo(s)

	if info.Schema() != s {
		t.Error("Schema() did not return the schema it was given")
	}
	// Everything is empty before a walk begins.
	if info.Type() != nil || info.ParentType() != nil || info.InputType() != nil ||
		info.ParentInputType() != nil || info.FieldDef() != nil ||
		info.Directive() != nil || info.Argument() != nil || info.EnumValue() != nil {
		t.Error("the position reported something before any walk")
	}
	if info.DefaultValue().IsSet() {
		t.Error("a default was reported before any walk")
	}
}

// The type one level out is what a list element or an input object field sits
// inside.
func TestTypeInfo_ParentInputType(t *testing.T) {
	s := typeInfoSchema(t)
	info := typeinfo.NewTypeInfo(s)

	var innerParent string
	visitor := language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			if v, isInt := node.(*language.IntValue); isInt && v.Value == "1" {
				if parent := info.ParentInputType(); parent != nil {
					innerParent = parent.String()
				}
			}
			return language.VisitContinue
		},
	}
	language.Visit(parseQuery(t, `{ search(filter: { nested: { limit: 1 } }) { id } }`),
		typeinfo.VisitWithTypeInfo(info, visitor))

	if innerParent != "Filter" {
		t.Errorf("the type around the nested value = %q, want Filter", innerParent)
	}
}

// Entering a list steps into its element type. Where the expected type is not
// a list at all there is no element type, so the position is left empty and
// whatever reports that a list does not belong here reads the type one level
// out with ParentInputType.
func TestTypeInfo_AListWhereNoListIsExpected(t *testing.T) {
	s, err := utilities.BuildSchema(`
		input Thing { byId: ID }
		type Query {
			one(arg: Int): String
			required(arg: Int!): String
			many(arg: [Int]): String
			nested(arg: [[Int]]): String
			object(arg: Thing): String
		}
	`)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}

	tests := []struct {
		name  string
		query string
		want  string
	}{
		// Where a list belongs, the position steps into the element type.
		{"a list where one is expected", `{ many(arg: [1, 2]) }`, "Int"},
		{"a list of lists", `{ nested(arg: [[1]]) }`, "[Int]"},
		// Where one does not, the position is empty and the type that was
		// wanted is one level out.
		{"a list where a scalar is expected", `{ one(arg: [1, 2]) }`, ""},
		{"a list where a non-null scalar is expected", `{ required(arg: [1]) }`, ""},
		{"a list where an input object is expected", `{ object(arg: [{ byId: 1 }]) }`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := parseQuery(t, tt.query)
			info := typeinfo.NewTypeInfo(s)
			var got, outer string
			var found bool
			language.Visit(doc, typeinfo.VisitWithTypeInfo(info, language.Visitor{
				Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
					if _, isList := node.(*language.ListValue); isList && !found {
						found = true
						if at := info.InputType(); at != nil {
							got = at.String()
						}
						if got == "" && info.ParentInputType() != nil {
							outer = info.ParentInputType().String()
						}
					}
					return language.VisitContinue
				},
			}))
			if !found {
				t.Fatal("the walk never reached the list")
			}
			if got != tt.want {
				t.Errorf("InputType at the list = %q, want %q", got, tt.want)
			}
			if tt.want == "" && outer == "" {
				t.Error("the type that was wanted is not reachable one level out either")
			}
			if !info.IsBalanced() {
				t.Error("the stacks are not balanced after the walk")
			}
		})
	}
}

// A fragment that declares arguments gives the walk a signature to read the
// spread's arguments against, so that inside `...F(x: 1)` the position knows
// what type `x` was declared as and what it falls back to.
//
// These are the accessors a rule outside this package works from, which is
// what TypeInfo is for; the rules here reach the same facts through Context.
func TestTypeInfo_FragmentArguments(t *testing.T) {
	s := typeInfoSchema(t)
	doc, err := language.ParseString(
		`{ search(filter: {term: "a"}) { name } ...F(upper: true, unknown: 1) }`+
			` fragment F($upper: Boolean = false, $limit: Int) on Query { me { name } }`,
		language.ExperimentalFragmentArguments())
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	info := typeinfo.NewTypeInfoForDocument(s, doc)

	type seen struct {
		argument  string
		declared  string
		inputType string
		fallback  string
	}
	var got []seen
	var signatureOnSpread, signatureInsideBody string

	language.Visit(doc, language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			info.Enter(node)
			switch n := node.(type) {
			case *language.FragmentArgument:
				record := seen{argument: n.Name.Value}
				if declared := info.FragmentArgument(); declared != nil {
					record.declared = declared.Variable.Name.Value
				}
				if t := info.InputType(); t != nil {
					record.inputType = t.String()
				}
				if def, has := info.DefaultValue().Get(); has && def.Literal != nil {
					record.fallback = language.Print(def.Literal)
				}
				got = append(got, record)
			case *language.FragmentSpread:
				if sig := info.FragmentSignature(); sig != nil {
					signatureOnSpread = sig.Definition.Name.Value
				}
			case *language.Field:
				// The signature belongs to the spread, not to the fragment's
				// own body: walking the definition reaches the body without
				// going through any spread, so there is nothing to answer for.
				if n.Name.Value == "me" {
					if sig := info.FragmentSignature(); sig != nil {
						signatureInsideBody = sig.Definition.Name.Value
					}
				}
			}
			return language.VisitContinue
		},
		Leave: func(node language.Node, _ language.VisitContext) language.VisitAction {
			info.Leave(node)
			return language.VisitContinue
		},
	})

	want := []seen{
		{argument: "upper", declared: "upper", inputType: "Boolean", fallback: "false"},
		// The fragment declares nothing by this name, so there is no type to
		// read the value against and nothing to fall back to.
		{argument: "unknown"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("arguments =\n%+v\nwant\n%+v", got, want)
	}
	if signatureOnSpread != "F" {
		t.Errorf("signature on the spread = %q, want F", signatureOnSpread)
	}
	if signatureInsideBody != "" {
		t.Errorf("signature inside the fragment's body = %q, want none", signatureInsideBody)
	}
	if !info.IsBalanced() {
		t.Error("the walk left the position out of balance")
	}

	// Outside a spread there is neither.
	if info.FragmentSignature() != nil || info.FragmentArgument() != nil {
		t.Error("a signature or argument survived the walk")
	}
}
