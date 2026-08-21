package schema_test

import (
	"strings"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

// literalSchema holds the types the literals below are checked against.
func literalSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := utilities.BuildSchema(`
		enum Colour { RED GREEN }
		input Filter { required: Boolean! optional: String nested: Filter }
		input Choice @oneOf { byId: ID byName: String }
		type Query {
			f(
				int: Int, string: String, colour: Colour,
				list: [Int], listOfLists: [[Int]], required: Int!,
				filter: Filter, choose: Choice
			): String
		}
	`)
	if err != nil {
		t.Fatalf("building: %v", err)
	}
	return s
}

// typeOf reads the type of one of the test field's arguments.
func typeOf(t *testing.T, s *schema.Schema, arg string) schema.Type {
	t.Helper()
	def := s.QueryType().Field("f").Arg(arg)
	if def == nil {
		t.Fatalf("there is no argument %q", arg)
	}
	return def.Type
}

// literalOf parses a value the way a document holds one.
func literalOf(t *testing.T, text string) language.Value {
	t.Helper()
	v, err := language.ParseValue(language.NewSource(text))
	if err != nil {
		t.Fatalf("parsing %q: %v", text, err)
	}
	return v
}

// A literal either fits the type where it is written or it does not, and where
// it does not the type itself says why.
func TestValidateInputLiteral(t *testing.T) {
	s := literalSchema(t)

	t.Run("literals that fit", func(t *testing.T) {
		for _, tt := range []struct{ arg, literal string }{
			{"int", "1"}, {"int", "null"},
			{"string", `"a"`}, {"colour", "RED"},
			{"list", "[1, 2]"}, {"list", "null"},
			// A single value stands for a list of one.
			{"list", "1"},
			{"listOfLists", "[[1], [2, 3]]"},
			{"required", "1"},
			{"filter", "{ required: true }"},
			{"filter", "{ required: true, optional: \"a\", nested: { required: false } }"},
			{"choose", `{ byId: "1" }`},
		} {
			t.Run(tt.arg+" = "+tt.literal, func(t *testing.T) {
				got := schema.ValidateInputLiteral(literalOf(t, tt.literal), typeOf(t, s, tt.arg), schema.VariableValues{})
				if len(got) != 0 {
					t.Errorf("rejected: %v", got)
				}
			})
		}
	})

	t.Run("literals that do not", func(t *testing.T) {
		for _, tt := range []struct{ arg, literal, says string }{
			// The scalar knows why, and what it says is what comes back.
			{"int", `"1"`, "Int cannot represent non-integer value"},
			{"int", "1.5", "Int cannot represent non-integer value"},
			{"string", "1", "String cannot represent a non string value"},
			{"colour", "PUCE", `Value "PUCE" does not exist in "Colour" enum.`},
			{"colour", `"RED"`, `Enum "Colour" cannot represent non-enum value`},
			{"required", "null", `non-null type "Int!" not to be null`},
			{"list", `[1, "two"]`, "Int cannot represent non-integer value"},
			{"filter", "1", `to be an object`},
			{"filter", "{ optional: \"a\" }", `to include required field "required"`},
			{"filter", `{ required: true, unknown: 1 }`, `not to include unknown field "unknown"`},
			// A misspelling is answered with the name it is nearest.
			{"filter", `{ required: true, optionl: "a" }`, `Did you mean "optional"?`},
			{"filter", `{ required: true, nested: { optional: "a" } }`, `to include required field "required"`},
			{"choose", "{}", "exactly one field must be specified"},
			{"choose", `{ byId: "1", byName: "a" }`, "exactly one field must be specified"},
			{"choose", "{ byId: null }", "exactly one field must be specified"},
		} {
			t.Run(tt.arg+" = "+tt.literal, func(t *testing.T) {
				got := schema.ValidateInputLiteral(literalOf(t, tt.literal), typeOf(t, s, tt.arg), schema.VariableValues{})
				if len(got) == 0 {
					t.Fatal("accepted")
				}
				if !strings.Contains(got[0].Error(), tt.says) {
					t.Errorf("said %q, want it to mention %q", got[0].Error(), tt.says)
				}
				if got[0].Node == nil {
					t.Error("nothing in the document was blamed")
				}
			})
		}
	})

	// The path says which part of a value is at fault, which is the whole
	// point once a value has more than one part.
	t.Run("where the fault is", func(t *testing.T) {
		for _, tt := range []struct{ arg, literal, at string }{
			{"list", `[1, "two"]`, "[1]"},
			{"listOfLists", `[[1], [2, "three"]]`, "[1][1]"},
			{"filter", `{ required: true, nested: { required: 1 } }`, "nested.required"},
		} {
			got := schema.ValidateInputLiteral(literalOf(t, tt.literal), typeOf(t, s, tt.arg), schema.VariableValues{})
			if len(got) != 1 {
				t.Fatalf("%s = %s: %d complaints, want 1: %v", tt.arg, tt.literal, len(got), got)
			}
			if !strings.HasPrefix(got[0].Error(), "at "+tt.at+":\n") {
				t.Errorf("%s = %s: %q, want it to point at %s", tt.arg, tt.literal, got[0].Error(), tt.at)
			}
		}
	})
}

// What a variable holds is not known while a document is only being checked,
// so nothing is said about one; once a request has arrived and the values are
// known, the same walk can check them.
func TestValidateInputLiteral_Variables(t *testing.T) {
	s := literalSchema(t)
	literal := literalOf(t, "$v")

	t.Run("while the document is being checked", func(t *testing.T) {
		if got := schema.ValidateInputLiteral(literal, typeOf(t, s, "required"), schema.VariableValues{}); len(got) != 0 {
			t.Errorf("a variable was judged with nothing to judge it by: %v", got)
		}
	})

	t.Run("once the values are known", func(t *testing.T) {
		for _, tt := range []struct {
			name      string
			variables map[string]any
			says      string
		}{
			{"supplied a value", map[string]any{"v": int32(1)}, ""},
			{"not supplied", map[string]any{}, "to provide a runtime value"},
			{"supplied as null", map[string]any{"v": nil}, "not to be null"},
		} {
			t.Run(tt.name, func(t *testing.T) {
				got := schema.ValidateInputLiteral(literal, typeOf(t, s, "required"), schema.NewVariableValues(tt.variables, nil))
				if tt.says == "" {
					if len(got) != 0 {
						t.Errorf("rejected: %v", got)
					}
					return
				}
				if len(got) != 1 {
					t.Fatalf("%d complaints, want 1: %v", len(got), got)
				}
				if !strings.Contains(got[0].Error(), tt.says) {
					t.Errorf("said %q, want it to mention %q", got[0].Error(), tt.says)
				}
			})
		}
	})

	// A nullable position takes whatever the variable holds, so there is
	// nothing to say either way.
	t.Run("a nullable position", func(t *testing.T) {
		if got := schema.ValidateInputLiteral(literal, typeOf(t, s, "int"), schema.NewVariableValues(map[string]any{}, nil)); len(got) != 0 {
			t.Errorf("rejected: %v", got)
		}
	})
}

func TestValidateInputLiteral_Nothing(t *testing.T) {
	s := literalSchema(t)
	if got := schema.ValidateInputLiteral(nil, typeOf(t, s, "int"), schema.VariableValues{}); len(got) != 0 {
		t.Errorf("checking nothing gave %v", got)
	}
	if got := schema.ValidateInputLiteral(literalOf(t, "1"), nil, schema.VariableValues{}); len(got) != 0 {
		t.Errorf("checking against nothing gave %v", got)
	}
}
