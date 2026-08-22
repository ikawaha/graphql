package utilities_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

// This is what a client actually has: JSON that came back from a server. The
// round trip is checked elsewhere against a schema this library built; here
// the answer is written out by hand, so nothing this library produced is
// standing in for what a server said.
const introspectionJSON = `{
  "__schema": {
    "description": "The whole schema.",
    "queryType": { "name": "Query" },
    "mutationType": null,
    "subscriptionType": null,
    "types": [
      { "kind": "SCALAR", "name": "String", "description": null },
      { "kind": "SCALAR", "name": "Int", "description": null },
      { "kind": "SCALAR", "name": "ID", "description": null },
      { "kind": "SCALAR", "name": "Boolean", "description": null },
      {
        "kind": "OBJECT", "name": "Query", "description": null,
        "fields": [
          {
            "name": "me", "description": "The viewer.",
            "args": [], "type": { "kind": "OBJECT", "name": "User", "ofType": null },
            "isDeprecated": false, "deprecationReason": null
          },
          {
            "name": "search", "description": null,
            "args": [
              {
                "name": "term", "description": "What to look for.",
                "type": { "kind": "SCALAR", "name": "String", "ofType": null },
                "defaultValue": "\"all\"", "isDeprecated": false, "deprecationReason": null
              },
              {
                "name": "limit", "description": null,
                "type": { "kind": "SCALAR", "name": "Int", "ofType": null },
                "defaultValue": null, "isDeprecated": false, "deprecationReason": null
              }
            ],
            "type": {
              "kind": "NON_NULL", "name": null,
              "ofType": {
                "kind": "LIST", "name": null,
                "ofType": {
                  "kind": "NON_NULL", "name": null,
                  "ofType": { "kind": "OBJECT", "name": "User", "ofType": null }
                }
              }
            },
            "isDeprecated": false, "deprecationReason": null
          }
        ],
        "inputFields": null, "interfaces": [], "enumValues": null, "possibleTypes": null
      },
      {
        "kind": "OBJECT", "name": "User", "description": "A person.",
        "fields": [
          {
            "name": "id", "description": null, "args": [],
            "type": {
              "kind": "NON_NULL", "name": null,
              "ofType": { "kind": "SCALAR", "name": "ID", "ofType": null }
            },
            "isDeprecated": false, "deprecationReason": null
          },
          {
            "name": "old", "description": null, "args": [],
            "type": { "kind": "SCALAR", "name": "String", "ofType": null },
            "isDeprecated": true, "deprecationReason": "Use name."
          },
          {
            "name": "bare", "description": null, "args": [],
            "type": { "kind": "SCALAR", "name": "String", "ofType": null },
            "isDeprecated": true, "deprecationReason": "No longer supported"
          },
          {
            "name": "colour", "description": null, "args": [],
            "type": { "kind": "ENUM", "name": "Colour", "ofType": null },
            "isDeprecated": false, "deprecationReason": null
          }
        ],
        "inputFields": null,
        "interfaces": [{ "kind": "INTERFACE", "name": "Node", "ofType": null }],
        "enumValues": null, "possibleTypes": null
      },
      {
        "kind": "INTERFACE", "name": "Node", "description": null,
        "fields": [
          {
            "name": "id", "description": null, "args": [],
            "type": {
              "kind": "NON_NULL", "name": null,
              "ofType": { "kind": "SCALAR", "name": "ID", "ofType": null }
            },
            "isDeprecated": false, "deprecationReason": null
          }
        ],
        "inputFields": null, "interfaces": [], "enumValues": null,
        "possibleTypes": [{ "kind": "OBJECT", "name": "User", "ofType": null }]
      },
      {
        "kind": "ENUM", "name": "Colour", "description": null,
        "fields": null, "inputFields": null, "interfaces": null,
        "enumValues": [
          { "name": "RED", "description": "Warm.", "isDeprecated": false, "deprecationReason": null },
          { "name": "PUCE", "description": null, "isDeprecated": true, "deprecationReason": "Out of fashion." }
        ],
        "possibleTypes": null
      },
      {
        "kind": "SCALAR", "name": "DateTime", "description": null,
        "specifiedByURL": "https://example.com/datetime",
        "fields": null, "inputFields": null, "interfaces": null,
        "enumValues": null, "possibleTypes": null
      },
      {
        "kind": "INPUT_OBJECT", "name": "Choice", "description": null, "isOneOf": true,
        "fields": null,
        "inputFields": [
          {
            "name": "byId", "description": null,
            "type": { "kind": "SCALAR", "name": "ID", "ofType": null },
            "defaultValue": null, "isDeprecated": false, "deprecationReason": null
          },
          {
            "name": "byName", "description": null,
            "type": { "kind": "SCALAR", "name": "String", "ofType": null },
            "defaultValue": null, "isDeprecated": false, "deprecationReason": null
          }
        ],
        "interfaces": null, "enumValues": null, "possibleTypes": null
      }
    ],
    "directives": [
      {
        "name": "skip", "description": null, "isRepeatable": false,
        "locations": ["FIELD", "FRAGMENT_SPREAD", "INLINE_FRAGMENT"],
        "args": [
          {
            "name": "if", "description": null,
            "type": {
              "kind": "NON_NULL", "name": null,
              "ofType": { "kind": "SCALAR", "name": "Boolean", "ofType": null }
            },
            "defaultValue": null, "isDeprecated": false, "deprecationReason": null
          }
        ]
      },
      {
        "name": "auth", "description": "Who may see it.", "isRepeatable": true,
        "locations": ["FIELD_DEFINITION"],
        "args": [
          {
            "name": "role", "description": null,
            "type": {
              "kind": "NON_NULL", "name": null,
              "ofType": { "kind": "SCALAR", "name": "String", "ofType": null }
            },
            "defaultValue": "\"user\"", "isDeprecated": false, "deprecationReason": null
          }
        ]
      }
    ]
  }
}`

func TestBuildClientSchema_FromJSON(t *testing.T) {
	var answer utilities.IntrospectionQueryResult
	if err := json.Unmarshal([]byte(introspectionJSON), &answer); err != nil {
		t.Fatalf("reading the answer: %v", err)
	}
	s, err := utilities.BuildClientSchema(&answer)
	if err != nil {
		t.Fatalf("building: %v", err)
	}
	if err := schema.AssertValidSchema(s); err != nil {
		t.Fatalf("the schema is not sound:\n%v", err)
	}

	text := utilities.PrintSchema(s)
	for _, wanted := range []string{
		`"""The whole schema."""`,
		`"""A person."""`,
		"type User implements Node {",
		// An argument carrying a description needs a line of its own, so the
		// list goes across several.
		"  search(\n    \"\"\"What to look for.\"\"\"\n    term: String = \"all\"\n    limit: Int\n  ): [User!]!",
		`old: String @deprecated(reason: "Use name.")`,
		// A deprecation with no reason keeps the one that would be assumed.
		"bare: String @deprecated\n",
		`"""Warm."""`,
		`PUCE @deprecated(reason: "Out of fashion.")`,
		`scalar DateTime @specifiedBy(url: "https://example.com/datetime")`,
		"input Choice @oneOf {",
		`directive @auth(role: String! = "user") repeatable on FIELD_DEFINITION`,
	} {
		if !strings.Contains(text, wanted) {
			t.Errorf("the schema does not contain %q:\n%s", wanted, text)
		}
	}
	// A built-in directive the answer listed is shared, so it is not written
	// out as though the server had declared it.
	if strings.Contains(text, "directive @skip") {
		t.Errorf("a directive every schema has was written out:\n%s", text)
	}
	if s.Directive("skip") != schema.Skip {
		t.Error("@skip was rebuilt rather than shared")
	}

	t.Run("the schema knows what implements what", func(t *testing.T) {
		node, _ := s.Type("Node").(*schema.InterfaceType)
		if node == nil {
			t.Fatal("Node was lost")
		}
		var found bool
		for _, impl := range s.PossibleTypes(node) {
			if impl.Name() == "User" {
				found = true
			}
		}
		if !found {
			t.Error("User was not indexed as an implementation of Node")
		}
	})

	// A schema built from an answer cannot answer for the server, which is
	// worth being sure of rather than discovering at runtime.
	t.Run("it has no resolvers", func(t *testing.T) {
		if s.QueryType().Field("me").Resolve != nil {
			t.Error("a field came back with a resolver, which an answer cannot describe")
		}
	})

	// And it reads back as itself, so it is a schema like any other.
	t.Run("it survives being written and read", func(t *testing.T) {
		again, err := utilities.BuildSchema(text)
		if err != nil {
			t.Fatalf("reading it back: %v", err)
		}
		if got := utilities.PrintSchema(again); got != text {
			t.Error("the schema does not survive being written and read")
		}
	})
}

// A default of null and no default at all are different things, and the answer
// tells them apart by null against the string "null".
func TestBuildClientSchema_DefaultValues(t *testing.T) {
	answer := `{"__schema":{"queryType":{"name":"Query"},"types":[{
		"kind":"OBJECT","name":"Query","fields":[{
			"name":"f","args":[
				{"name":"none","type":{"kind":"SCALAR","name":"String"},"defaultValue":null},
				{"name":"null","type":{"kind":"SCALAR","name":"String"},"defaultValue":"null"},
				{"name":"value","type":{"kind":"SCALAR","name":"String"},"defaultValue":"\"x\""},
				{"name":"object","type":{"kind":"INPUT_OBJECT","name":"F"},"defaultValue":"{a: 1}"}
			],
			"type":{"kind":"SCALAR","name":"String"},"isDeprecated":false
		}],"interfaces":[]
	},{
		"kind":"INPUT_OBJECT","name":"F","inputFields":[
			{"name":"a","type":{"kind":"SCALAR","name":"Int"},"defaultValue":null}
		]
	},{
		"kind":"SCALAR","name":"String"
	},{
		"kind":"SCALAR","name":"Int"
	}],"directives":[]}}`

	var parsed utilities.IntrospectionQueryResult
	if err := json.Unmarshal([]byte(answer), &parsed); err != nil {
		t.Fatalf("reading the answer: %v", err)
	}
	s, err := utilities.BuildClientSchema(&parsed)
	if err != nil {
		t.Fatalf("building: %v", err)
	}

	field := s.QueryType().Field("f")
	if _, has := field.Arg("none").Default.Get(); has {
		t.Error("an argument with no default came back with one")
	}
	for _, tt := range []struct{ arg, want string }{
		{"null", "null"}, {"value", `"x"`}, {"object", "{ a: 1 }"},
	} {
		text := utilities.PrintSchema(s)
		if !strings.Contains(text, tt.arg+": "+argTypeIn(t, s, tt.arg)+" = "+tt.want) {
			t.Errorf("the default of %s is not %s:\n%s", tt.arg, tt.want, text)
		}
	}

	// A default the server printed as something unparseable is treated as
	// absent rather than bringing the build down.
	t.Run("a default that will not parse", func(t *testing.T) {
		broken := strings.Replace(answer, `"defaultValue":"\"x\""`, `"defaultValue":"not a literal"`, 1)
		var parsed utilities.IntrospectionQueryResult
		if err := json.Unmarshal([]byte(broken), &parsed); err != nil {
			t.Fatalf("reading: %v", err)
		}
		s, err := utilities.BuildClientSchema(&parsed)
		if err != nil {
			t.Fatalf("building: %v", err)
		}
		if _, has := s.QueryType().Field("f").Arg("value").Default.Get(); has {
			t.Error("an unparseable default was kept")
		}
	})
}

// argTypeIn reads how an argument's type is written, for building an
// expectation.
func argTypeIn(t *testing.T, s *schema.Schema, name string) string {
	t.Helper()
	arg := s.QueryType().Field("f").Arg(name)
	if arg == nil {
		t.Fatalf("there is no argument %q", name)
	}
	return arg.Type.String()
}

// An answer that does not describe a schema is reported rather than producing
// something half-built.
func TestBuildClientSchema_BadAnswers(t *testing.T) {
	tests := []struct{ name, json, want string }{
		{"no query type", `{"__schema":{"types":[],"directives":[]}}`, "query type"},
		{
			name: "a root that is not an object",
			json: `{"__schema":{"queryType":{"name":"S"},"types":[
				{"kind":"SCALAR","name":"S"}],"directives":[]}}`,
			want: "not an object type",
		},
		{
			name: "a type of an unknown kind",
			json: `{"__schema":{"queryType":{"name":"Query"},"types":[
				{"kind":"WHAT","name":"Query"}],"directives":[]}}`,
			want: "unknown kind",
		},
		{
			name: "a reference to a type that was not listed",
			json: `{"__schema":{"queryType":{"name":"Missing"},"types":[],"directives":[]}}`,
			want: "unknown type",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var parsed utilities.IntrospectionQueryResult
			if err := json.Unmarshal([]byte(tt.json), &parsed); err != nil {
				t.Fatalf("reading: %v", err)
			}
			_, err := utilities.BuildClientSchema(&parsed)
			if err == nil {
				t.Fatal("built without complaint")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to mention %q", err, tt.want)
			}
		})
	}
	if _, err := utilities.BuildClientSchema(nil); err == nil {
		t.Error("building from nothing succeeded")
	}
}

// The query a client sends is only useful if it is one a schema can answer, so
// what the generator produces has to parse and to name only real fields.
func TestIntrospectionQuery(t *testing.T) {
	t.Run("what it asks for", func(t *testing.T) {
		plain := utilities.IntrospectionQuery()
		full := utilities.IntrospectionQuery(utilities.WithEverything())

		// The defaults ask for what every server can answer; the rest is
		// opt-in, because an older server rejects a query naming a field its
		// introspection schema does not have.
		for _, optional := range []string{"specifiedByURL", "isOneOf", "isRepeatable"} {
			if strings.Contains(plain, optional) {
				t.Errorf("the plain query asks for %s", optional)
			}
			if !strings.Contains(full, optional) {
				t.Errorf("the full query does not ask for %s", optional)
			}
		}
		if !strings.Contains(plain, "description") {
			t.Error("the plain query does not ask for descriptions")
		}
		if strings.Contains(utilities.IntrospectionQuery(utilities.WithoutDescriptions()), "description") {
			t.Error("descriptions were asked for after being turned off")
		}
	})

	t.Run("each option on its own", func(t *testing.T) {
		for _, tt := range []struct {
			option utilities.IntrospectionOption
			asks   string
		}{
			{utilities.WithSpecifiedByURL(), "specifiedByURL"},
			{utilities.WithDirectiveIsRepeatable(), "isRepeatable"},
			{utilities.WithOneOf(), "isOneOf"},
			{utilities.WithInputValueDeprecation(), "includeDeprecated"},
		} {
			if !strings.Contains(utilities.IntrospectionQuery(tt.option), tt.asks) {
				t.Errorf("the option did not ask for %s", tt.asks)
			}
		}
		if !strings.Contains(utilities.IntrospectionQuery(utilities.WithSchemaDescription()),
			"__schema {\n    description") {
			t.Error("the schema's own description was not asked for")
		}
	})
}

// TestBuildClientSchema_TruncatedTypeReference covers an answer whose type
// references stop part way, which is what an introspection query asked to
// unfold fewer levels than the schema needs comes back with.
//
// graphql-js throws "Decorated type deeper than introspection query."; the
// refusal here says the same thing in its own words rather than only that
// something is missing, since the fix is to ask again and further.
func TestBuildClientSchema_TruncatedTypeReference(t *testing.T) {
	s, err := utilities.BuildSchema(`type Query { deep: [[[String!]!]!]! }`)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("deep enough", func(t *testing.T) {
		answer, err := utilities.IntrospectionFromSchema(context.Background(), s)
		if err != nil {
			t.Fatal(err)
		}
		rebuilt, err := utilities.BuildClientSchema(answer)
		if err != nil {
			t.Fatal(err)
		}
		if got := utilities.PrintSchema(rebuilt); !strings.Contains(got, "deep: [[[String!]!]!]!") {
			t.Errorf("the type came back as\n%s", got)
		}
	})

	t.Run("not deep enough", func(t *testing.T) {
		answer, err := utilities.IntrospectionFromSchema(context.Background(), s,
			utilities.WithTypeDepth(2))
		if err != nil {
			t.Fatal(err)
		}
		_, err = utilities.BuildClientSchema(answer)
		if err == nil {
			t.Fatal("a truncated answer built a schema")
		}
		if !strings.Contains(err.Error(), "did not unfold far enough") {
			t.Errorf("the refusal does not name the cause: %v", err)
		}
	})
}
