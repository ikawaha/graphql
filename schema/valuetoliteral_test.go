package schema_test

import (
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// TestLiteralFromValue_SpecifiedScalars pins what each specified scalar will
// and will not write as a literal, which is what decides whether a default
// value given in code survives into the printed schema.
//
// Every expectation here was taken from graphql-js: the ones that render match
// what printSchema writes, and the ones that do not are the ones its
// getDefaultValueAST refuses.
func TestLiteralFromValue_SpecifiedScalars(t *testing.T) {
	tests := []struct {
		name  string
		typ   schema.Type
		value any
		want  string // empty means the value cannot be written as this type
	}{
		{"an integer for Int", schema.Int, 5, "5"},
		{"a whole float for Int", schema.Int, 5.0, "5"},
		{"a fractional float for Int", schema.Int, 5.5, ""},
		{"digits in a string for Int", schema.Int, "5", ""},
		{"a number too large for Int", schema.Int, 1e10, ""},
		{"an integer for Float", schema.Float, 5, "5"},
		{"a fraction for Float", schema.Float, 1.5, "1.5"},
		{"a number in a string for Float", schema.Float, "1.5", ""},
		{"a string for String", schema.String, "x", `"x"`},
		{"a number for String", schema.String, 5, ""},
		{"a boolean for String", schema.String, true, ""},
		{"a boolean for Boolean", schema.Boolean, true, "true"},
		{"a string for Boolean", schema.Boolean, "t", ""},
		{"digits for ID", schema.ID, "7", "7"},
		{"a word for ID", schema.ID, "a", `"a"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			literal, ok := schema.LiteralFromValue(test.value, test.typ)
			if test.want == "" {
				if ok {
					t.Fatalf("wrote %s, wanted nothing", language.Print(literal))
				}
				return
			}
			if !ok {
				t.Fatal("wrote nothing")
			}
			if got := language.Print(literal); got != test.want {
				t.Errorf("wrote %s, wanted %s", got, test.want)
			}
		})
	}
}

// TestLiteralFromValue_CustomScalar covers the scalar whose external form is
// not its internal one, which is most of them: a DateTime takes a string and
// gives out a time.
//
// The value being written is the external one, so a custom scalar that says
// nothing about literals still has its default written out. Coercing first
// would turn the string into the internal form, which is the form that cannot
// be written, and the default would go missing.
func TestLiteralFromValue_CustomScalar(t *testing.T) {
	type instant struct{ text string }
	dateTime := schema.NewScalar(schema.ScalarConfig{
		Name: "DateTime",
		CoerceInputValue: func(external any) (value.Maybe[any], error) {
			text, isText := external.(string)
			if !isText {
				return value.Nothing[any](), nil
			}
			return value.Just[any](instant{text}), nil
		},
	})

	literal, ok := schema.LiteralFromValue("2024-01-01T00:00:00Z", dateTime)
	if !ok {
		t.Fatal("wrote nothing")
	}
	if got, want := language.Print(literal), `"2024-01-01T00:00:00Z"`; got != want {
		t.Errorf("wrote %s, wanted %s", got, want)
	}

	// A scalar with a rendering of its own is handed the external value too.
	seen := make(chan any, 1)
	watched := schema.NewScalar(schema.ScalarConfig{
		Name: "Watched",
		ValueToLiteral: func(external any, _ schema.Type) (language.Value, error) {
			seen <- external
			return &language.StringValue{Value: "written"}, nil
		},
	})
	if _, ok := schema.LiteralFromValue("outside", watched); !ok {
		t.Fatal("wrote nothing")
	}
	if got := <-seen; got != "outside" {
		t.Errorf("handed %#v, wanted the external value", got)
	}
}

// TestLiteralFromGoValue_StopsDescending covers a value that refers to itself.
// Go cannot recover from running out of stack, so the walk gives up rather
// than find out how deep the value goes.
func TestLiteralFromGoValue_StopsDescending(t *testing.T) {
	selfMap := map[string]any{}
	selfMap["self"] = selfMap
	selfSlice := make([]any, 1)
	selfSlice[0] = selfSlice

	for name, held := range map[string]any{
		"a map holding itself":   selfMap,
		"a slice holding itself": selfSlice,
	} {
		t.Run(name, func(t *testing.T) {
			if literal, ok := schema.LiteralFromGoValue(held); ok {
				t.Errorf("wrote %s, wanted nothing", language.Print(literal))
			}
		})
	}

	// A value that is merely deep, rather than endless, is still written.
	deep := any("bottom")
	for range 8 {
		deep = []any{deep}
	}
	if _, ok := schema.LiteralFromGoValue(deep); !ok {
		t.Error("wrote nothing for a value only eight deep")
	}
}
