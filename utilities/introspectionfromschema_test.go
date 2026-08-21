package utilities_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

// This is what the phase exists to prove: a schema described to a client, and
// rebuilt from that description, is the same schema. It is the only check that
// the introspection resolvers, the query, and the rebuilding all agree — each
// could be self-consistently wrong on its own.
func TestIntrospectionRoundTrip(t *testing.T) {
	const sdl = `
		"The whole schema."
		schema { query: Query }

		"When something happened."
		scalar DateTime @specifiedBy(url: "https://example.com/datetime")

		interface Node { id: ID! }
		interface Named implements Node {
			id: ID!
			"What it is called."
			name: String
		}

		"A person."
		type User implements Node & Named {
			id: ID!
			name: String
			"No longer used."
			old: String @deprecated(reason: "Use name.")
			bare: String @deprecated
			friends(
				"How many to return."
				first: Int = 10
				after: String = null
				filter: Filter
			): [User!]!
			joined: DateTime
		}

		type Photo implements Node { id: ID! url: String }

		"Either of the two."
		union Media = User | Photo

		"How something looks."
		enum Colour {
			RED
			"Faded."
			PUCE @deprecated(reason: "Out of fashion.")
			BARE @deprecated
		}

		"What to look for."
		input Filter {
			"Match this."
			term: String = "all"
			limit: Int! = 5
			nested: Filter
			old: String @deprecated(reason: "Use term.")
		}

		input Choice @oneOf { byId: ID byName: String }

		"Who may see it."
		directive @auth(
			"Which role."
			role: String! = "user"
		) repeatable on FIELD_DEFINITION | OBJECT

		type Query {
			node(id: ID!): Node
			media: Media
			colour: Colour
			search(filter: Filter, choose: Choice): [User!]
		}
	`

	original, err := utilities.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	if err := schema.AssertValidSchema(original); err != nil {
		t.Fatalf("the schema is not sound: %v", err)
	}

	answer, err := utilities.IntrospectionFromSchema(
		context.Background(), original, utilities.WithEverything())
	if err != nil {
		t.Fatalf("asking the schema about itself: %v", err)
	}

	rebuilt, err := utilities.BuildClientSchema(answer)
	if err != nil {
		t.Fatalf("rebuilding from the answer: %v", err)
	}
	if err := schema.AssertValidSchema(rebuilt); err != nil {
		t.Fatalf("the rebuilt schema is not sound:\n%v", err)
	}

	before := utilities.PrintSchema(original)
	after := utilities.PrintSchema(rebuilt)
	if before != after {
		t.Error("the schema did not survive the round trip")
		for i := range min(len(before), len(after)) {
			if before[i] != after[i] {
				t.Fatalf("first difference at byte %d:\nbefore: %q\nafter:  %q",
					i, excerptAround(before, i), excerptAround(after, i))
			}
		}
		t.Fatalf("one is longer than the other:\nbefore:\n%s\n\nafter:\n%s", before, after)
	}
}

// The answer is JSON on the wire, so it has to survive being written and read
// back: a client rebuilds from what arrived, not from the map the server made.
func TestIntrospectionRoundTripThroughJSON(t *testing.T) {
	original, err := utilities.BuildSchema(`
		type Query { me: User search(term: String = "all"): [User!] }
		"A person." type User { name: String old: String @deprecated }
		scalar Odd @specifiedBy(url: "https://example.com")
	`)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}

	answer, err := utilities.IntrospectionFromSchema(
		context.Background(), original, utilities.WithEverything())
	if err != nil {
		t.Fatalf("asking the schema about itself: %v", err)
	}
	onTheWire, err := json.Marshal(answer)
	if err != nil {
		t.Fatalf("writing the answer: %v", err)
	}

	var received utilities.IntrospectionQueryResult
	if err := json.Unmarshal(onTheWire, &received); err != nil {
		t.Fatalf("reading the answer: %v", err)
	}
	rebuilt, err := utilities.BuildClientSchema(&received)
	if err != nil {
		t.Fatalf("rebuilding: %v", err)
	}
	if got, want := utilities.PrintSchema(rebuilt), utilities.PrintSchema(original); got != want {
		t.Errorf("the schema did not survive JSON:\ngot:\n%s\n\nwant:\n%s", got, want)
	}
}

// A real schema is the honest test: every construct at once, nothing chosen to
// suit the implementation.
func TestIntrospectionRoundTrip_GitHubSchema(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "language", "testdata", "github-schema.graphql"))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	original, err := utilities.BuildSchema(string(body))
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}

	answer, err := utilities.IntrospectionFromSchema(
		context.Background(), original, utilities.WithEverything())
	if err != nil {
		t.Fatalf("asking the schema about itself: %v", err)
	}
	rebuilt, err := utilities.BuildClientSchema(answer)
	if err != nil {
		t.Fatalf("rebuilding from the answer: %v", err)
	}
	if err := schema.AssertValidSchema(rebuilt); err != nil {
		t.Fatalf("the rebuilt schema is not sound:\n%v", err)
	}

	if got, want := len(rebuilt.Types()), len(original.Types()); got != want {
		t.Errorf("%d types after the round trip, want %d", got, want)
	}
	before, after := utilities.PrintSchema(original), utilities.PrintSchema(rebuilt)
	if before != after {
		t.Error("the schema did not survive the round trip")
		for i := range min(len(before), len(after)) {
			if before[i] != after[i] {
				t.Fatalf("first difference at byte %d:\nbefore: %q\nafter:  %q",
					i, excerptAround(before, i), excerptAround(after, i))
			}
		}
	}
}

// A schema described without asking for everything comes back missing what was
// not asked for, and saying so is better than pretending otherwise.
func TestIntrospectionRoundTrip_PartialAnswer(t *testing.T) {
	original, err := utilities.BuildSchema(`
		"A person." type User { name: String }
		type Query { me: User }
	`)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}

	answer, err := utilities.IntrospectionFromSchema(
		context.Background(), original, utilities.WithoutDescriptions())
	if err != nil {
		t.Fatalf("asking the schema about itself: %v", err)
	}
	rebuilt, err := utilities.BuildClientSchema(answer)
	if err != nil {
		t.Fatalf("rebuilding: %v", err)
	}
	if strings.Contains(utilities.PrintSchema(rebuilt), "A person") {
		t.Error("a description came back that was never asked for")
	}
	if rebuilt.Type("User") == nil {
		t.Error("the type itself was lost")
	}
}

func TestIntrospectionFromSchema_Errors(t *testing.T) {
	if _, err := utilities.IntrospectionFromSchema(context.Background(), nil); err == nil {
		t.Error("describing no schema succeeded")
	}
	if _, err := utilities.BuildClientSchema(nil); err == nil {
		t.Error("rebuilding from nothing succeeded")
	}
	if _, err := utilities.BuildClientSchema(&utilities.IntrospectionQueryResult{}); err == nil {
		t.Error("rebuilding from an answer with no query type succeeded")
	}
}

// excerptAround shows the text either side of an offset, for reporting a
// difference.
func excerptAround(s string, at int) string {
	start := max(at-70, 0)
	end := min(at+70, len(s))
	return s[start:end]
}
