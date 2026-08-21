package validation_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
	"github.com/ikawaha/graphql/validation"
)

// mustParseQuery parses a test document, after removing the indentation it
// carries from being written inside Go source.
func mustParseQuery(t testing.TB, query string) *language.Document {
	t.Helper()
	doc, err := language.ParseString(dedent(query))
	if err != nil {
		t.Fatalf("parsing the test document: %v", err)
	}
	return doc
}

func TestValidate_ASoundDocument(t *testing.T) {
	s := testSchema(t)
	doc := mustParseQuery(t, `
		query Everything($surname: Boolean, $command: DogCommand!) {
			dog {
				...DogFields
				doesKnowCommand(dogCommand: $command)
			}
			catOrDog {
				... on Cat { meows }
				... on Dog { barks }
			}
			complicatedArgs {
				multipleReqs(req1: 1, req2: 2)
				complexArgField(complexArg: { requiredField: true, stringField: "a" })
			}
			__typename
		}
		fragment DogFields on Dog {
			name(surname: $surname)
			nickname @include(if: true)
		}
	`)

	if errs := validation.Validate(s, doc); len(errs) != 0 {
		t.Errorf("a sound document was rejected:\n%s", describeErrors(errs))
	}
}

// One document can be wrong in several unrelated ways at once, and every rule
// has to keep working while the others are finding their own problems.
func TestValidate_ManyProblemsAtOnce(t *testing.T) {
	s := testSchema(t)
	doc := mustParseQuery(t, `
		query Bad($unused: String, $wrong: String) {
			dog {
				meowVolume
				name @unknownDirective
			}
			complicatedArgs {
				intArgField(intArg: "not a number")
				multipleReqs(req1: 1)
				stringArgField(stringArg: $undeclared)
				nonNullIntArgField(nonNullIntArg: $wrong)
			}
			...MissingFragment
		}
		fragment Unused on Dog { name }
	`)

	errs := validation.Validate(s, doc)
	// Each of these comes from a different rule, so finding them all together
	// is what says the rules do not tread on one another.
	for _, wanted := range []string{
		`Cannot query field "meowVolume" on type "Dog"`,
		`Unknown directive "@unknownDirective"`,
		`Int cannot represent non-integer value: "not a number"`,
		`Argument "ComplicatedArgs.multipleReqs(req2:)" of type "Int!" is required`,
		`Variable "$undeclared" is not defined by operation "Bad"`,
		`Variable "$wrong" of type "String" used in position expecting type "Int!"`,
		`Unknown fragment "MissingFragment"`,
		`Variable "$unused" is never used`,
		`Fragment "Unused" is never used`,
	} {
		found := false
		for _, err := range errs {
			if strings.Contains(err.Message, wanted) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("nothing reported %q; got:\n%s", wanted, describeErrors(errs))
		}
	}
}

// A document can be wrong in unboundedly many ways, and a client can act on a
// hundred errors no better than on a thousand. Reporting stops rather than
// letting a broken document cost more to refuse than to run.
func TestValidate_StopsAfterEnoughErrors(t *testing.T) {
	s := testSchema(t)
	var b strings.Builder
	b.WriteString("{ dog {\n")
	for i := range 500 {
		fmt.Fprintf(&b, "  notAField%d\n", i)
	}
	b.WriteString("} }")

	errs := validation.Validate(s, mustParseQuery(t, b.String()))
	if len(errs) > 200 {
		t.Errorf("%d errors reported; the cap did not take effect", len(errs))
	}
	if len(errs) == 0 {
		t.Fatal("nothing was reported")
	}
	last := errs[len(errs)-1]
	if !strings.Contains(last.Message, "Too many validation errors") {
		t.Errorf("the last error is %q, want it to say reporting stopped", last.Message)
	}
}

func TestValidate_NoDocument(t *testing.T) {
	if errs := validation.Validate(testSchema(t), nil); len(errs) != 1 {
		t.Errorf("validating nothing gave %d errors, want 1", len(errs))
	}
	if errs := validation.ValidateSDL(nil, nil); len(errs) != 1 {
		t.Errorf("validating no SDL gave %d errors, want 1", len(errs))
	}
}

// A server may add a rule of its own, or leave the specified ones out.
func TestValidate_WithChosenRules(t *testing.T) {
	s := testSchema(t)
	doc := mustParseQuery(t, `{ dog { meowVolume } }`)

	if errs := validation.Validate(s, doc, validation.KnownFragmentNamesRule); len(errs) != 0 {
		t.Errorf("a rule that has nothing to say reported %d errors", len(errs))
	}
	if errs := validation.Validate(s, doc, validation.FieldsOnCorrectTypeRule); len(errs) != 1 {
		t.Errorf("the chosen rule reported %d errors, want 1", len(errs))
	}
	// Passing no rules at all is different from passing none of them.
	if errs := validation.Validate(s, doc, []validation.Rule{}...); len(errs) != 0 {
		t.Errorf("an empty rule list reported %d errors", len(errs))
	}
}

// SDL is checked with its own set, which says what a document of definitions
// alone can settle.
func TestValidateSDL_AllRules(t *testing.T) {
	t.Run("a sound document", func(t *testing.T) {
		doc := mustParseQuery(t, `
			schema { query: Query }
			directive @tag(name: String!) on OBJECT
			type Query @tag(name: "root") {
				a: String
				b(arg: Int = 0): Int
			}
			interface Node { id: ID! }
			type Thing implements Node @tag(name: "thing") { id: ID! }
			extend type Thing { extra: String }
			enum Colour { RED GREEN }
			input Filter { byId: ID }
		`)
		if errs := validation.ValidateSDL(doc, nil); len(errs) != 0 {
			t.Errorf("a sound document was rejected:\n%s", describeErrors(errs))
		}
	})

	t.Run("many problems at once", func(t *testing.T) {
		doc := mustParseQuery(t, `
			schema { query: Query }
			schema { query: Query }
			type Query { a: String a: Int }
			type Query { b: String }
			extend type Missing { c: String }
			directive @tag(name: String!) on OBJECT
			directive @tag on FIELD
			type Thing @tag { d: Unknown }
			directive @label(text: String!) on OBJECT
			type Other @label { e: String }
		`)
		errs := validation.ValidateSDL(doc, nil)
		for _, wanted := range []string{
			"Must provide only one schema definition",
			`Field "Query.a" can only be defined once`,
			`There can be only one type named "Query"`,
			`Cannot extend type "Missing" because it is not defined`,
			`There can be only one directive named "@tag"`,
			// A directive declared twice takes its last declaration, so @tag
			// is read as the one that belongs on a field.
			`Directive "@tag" may not be used on OBJECT`,
			`Argument "@label(text:)" of type "String!" is required`,
			`Unknown type "Unknown"`,
		} {
			found := false
			for _, err := range errs {
				if strings.Contains(err.Message, wanted) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("nothing reported %q; got:\n%s", wanted, describeErrors(errs))
			}
		}
	})
}

// A real schema and a query of the shape people actually write is the honest
// test that the rules agree with one another.
func TestValidate_GitHubSchema(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "language", "testdata", "github-schema.graphql"))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	s, err := utilities.BuildSchema(string(body))
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	if err := schema.AssertValidSchema(s); err != nil {
		t.Fatalf("the schema is not sound: %v", err)
	}

	t.Run("a sound query", func(t *testing.T) {
		doc := mustParseQuery(t, `
			query Repo($owner: String!, $name: String!, $count: Int = 10) {
				repository(owner: $owner, name: $name) {
					...RepoFields
					issues(first: $count, states: [OPEN]) {
						totalCount
						nodes { ...IssueFields }
					}
				}
			}
			fragment RepoFields on Repository {
				name
				description
				owner { login ... on Organization { membersWithRole { totalCount } } }
			}
			fragment IssueFields on Issue {
				title
				author { login }
				comments(first: 1) { nodes { body } }
			}
		`)
		if errs := validation.Validate(s, doc); len(errs) != 0 {
			t.Errorf("a sound query was rejected:\n%s", describeErrors(errs))
		}
	})

	t.Run("the introspection query", func(t *testing.T) {
		// Whatever a client uses to discover a schema has to pass, including
		// the depth limit, or no tooling would work.
		doc := mustParseQuery(t, introspectionQuery)
		if errs := validation.Validate(s, doc); len(errs) != 0 {
			t.Errorf("the introspection query was rejected:\n%s", describeErrors(errs))
		}
	})
}

// introspectionQuery is what a client sends to discover a schema.
const introspectionQuery = `
	query IntrospectionQuery {
		__schema {
			queryType { name }
			mutationType { name }
			subscriptionType { name }
			types { ...FullType }
			directives {
				name
				description
				locations
				args { ...InputValue }
			}
		}
	}

	fragment FullType on __Type {
		kind
		name
		description
		fields(includeDeprecated: true) {
			name
			description
			args { ...InputValue }
			type { ...TypeRef }
			isDeprecated
			deprecationReason
		}
		inputFields { ...InputValue }
		interfaces { ...TypeRef }
		enumValues(includeDeprecated: true) {
			name
			description
			isDeprecated
			deprecationReason
		}
		possibleTypes { ...TypeRef }
	}

	fragment InputValue on __InputValue {
		name
		description
		type { ...TypeRef }
		defaultValue
	}

	fragment TypeRef on __Type {
		kind
		name
		ofType {
			kind
			name
			ofType { kind name }
		}
	}`

func BenchmarkValidate_GitHubSchema(b *testing.B) {
	body, err := os.ReadFile(filepath.Join("..", "language", "testdata", "github-schema.graphql"))
	if err != nil {
		b.Fatal(err)
	}
	s, err := utilities.BuildSchema(string(body))
	if err != nil {
		b.Fatal(err)
	}
	doc := mustParseQuery(b, introspectionQuery)
	b.ReportAllocs()
	for b.Loop() {
		if errs := validation.Validate(s, doc); len(errs) != 0 {
			b.Fatalf("unexpected errors: %d", len(errs))
		}
	}
}
