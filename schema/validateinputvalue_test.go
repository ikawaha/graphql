package schema_test

import (
	"strings"
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// messagesOf renders the complaints for comparison.
func messagesOf(errs []schema.InputValueError) []string {
	out := make([]string, len(errs))
	for i, err := range errs {
		out[i] = err.Error()
	}
	return out
}

// The two halves walk the same ground separately, so they could drift apart:
// a value the fast check rejects but the slow one cannot explain would leave a
// caller with no message, and one the fast check accepts but the slow one
// complains about would report a problem that is not there. This pins them
// together over a spread of values.
func TestValidateInputValue_AgreesWithCoercion(t *testing.T) {
	oneOf := schema.NewInputObject(schema.InputObjectConfig{
		Name:    "Ref",
		IsOneOf: true,
		Fields: []*schema.InputField{
			schema.NewInputField("id", schema.InputFieldConfig{Type: schema.ID}),
			schema.NewInputField("name", schema.InputFieldConfig{Type: schema.String}),
		},
	})
	colour := schema.NewEnum(schema.EnumConfig{
		Name:   "Colour",
		Values: []*schema.EnumValue{schema.NewEnumValue("RED", schema.EnumValueConfig{})},
	})

	cases := []struct {
		name string
		in   any
		typ  schema.Type
	}{
		{"a sound Int", 1, schema.Int},
		{"a string where an Int is wanted", "1", schema.Int},
		{"null where it is allowed", nil, schema.String},
		{"null where it is forbidden", nil, schema.NewNonNull(schema.String)},
		{"a sound list", []any{1, 2}, schema.NewList(schema.Int)},
		{"a list with a bad element", []any{1, "no"}, schema.NewList(schema.Int)},
		{"a lone value for a list", 1, schema.NewList(schema.Int)},
		{"null in a list of non-nulls", []any{nil}, schema.NewList(schema.NewNonNull(schema.Int))},
		{"a sound input object", map[string]any{"optional": "x"}, testInputObject()},
		{"an input object that is not an object", "x", testInputObject()},
		{"an unknown field", map[string]any{"nope": 1}, testInputObject()},
		{"a field of the wrong type", map[string]any{"optional": 1}, testInputObject()},
		{"a sound enum member", "RED", colour},
		{"an unknown enum member", "BLUE", colour},
		{"a number where an enum is wanted", 1, colour},
		{"one field of a oneOf", map[string]any{"id": "1"}, oneOf},
		{"no fields of a oneOf", map[string]any{}, oneOf},
		{"two fields of a oneOf", map[string]any{"id": "1", "name": "a"}, oneOf},
		{"a null field of a oneOf", map[string]any{"id": nil}, oneOf},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, coerced := schema.CoerceInputValue(tt.in, tt.typ)
			problems := schema.ValidateInputValue(tt.in, tt.typ)

			switch {
			case coerced && len(problems) > 0:
				t.Errorf("the value converted but was reported as wrong:\n\t%s",
					strings.Join(messagesOf(problems), "\n\t"))
			case !coerced && len(problems) == 0:
				t.Error("the value did not convert but nothing was reported as wrong")
			}
		})
	}
}

// A value left out and a value given as null are both refused by a non-null
// type, and saying which helps whoever has to fix it.
func TestValidateInputValue_MissingVersusNull(t *testing.T) {
	nonNull := schema.NewNonNull(schema.String)

	absent := schema.ValidateSuppliedInputValue(value.Nothing[any](), nonNull)
	if len(absent) != 1 {
		t.Fatalf("%d complaints for a value left out, want 1", len(absent))
	}
	if !strings.Contains(absent[0].Message, "to be provided") {
		t.Errorf("message = %q, want it to say the value was not provided", absent[0].Message)
	}

	null := schema.ValidateInputValue(nil, nonNull)
	if len(null) != 1 {
		t.Fatalf("%d complaints for null, want 1", len(null))
	}
	if !strings.Contains(null[0].Message, "not to be null") {
		t.Errorf("message = %q, want it to say the value must not be null", null[0].Message)
	}

	// The two must not report the same thing, or the distinction is lost.
	if absent[0].Message == null[0].Message {
		t.Error("a value left out and a null value were reported identically")
	}

	// A nullable type minds neither.
	if got := schema.ValidateSuppliedInputValue(value.Nothing[any](), schema.String); len(got) != 0 {
		t.Errorf("a nullable type complained about a value left out: %v", messagesOf(got))
	}
}

// A complaint says where inside the value the problem is, so that a large
// input can be corrected without guesswork.
func TestValidateInputValue_ReportsThePath(t *testing.T) {
	nested := schema.NewInputObject(schema.InputObjectConfig{
		Name: "Outer",
		Fields: []*schema.InputField{
			schema.NewInputField("items", schema.InputFieldConfig{
				Type: schema.NewList(schema.Int),
			}),
		},
	})

	problems := schema.ValidateInputValue(
		map[string]any{"items": []any{1, "no", 3}}, nested)
	if len(problems) != 1 {
		t.Fatalf("%d complaints, want 1: %v", len(problems), messagesOf(problems))
	}
	if got := problems[0].Error(); !strings.Contains(got, "items[1]") {
		t.Errorf("complaint = %q, want it to point at items[1]", got)
	}

	// A problem with the value as a whole has no path.
	whole := schema.ValidateInputValue("x", nested)
	if len(whole) != 1 {
		t.Fatalf("%d complaints, want 1", len(whole))
	}
	if len(whole[0].Path) != 0 {
		t.Errorf("path = %v, want none", whole[0].Path)
	}
	if strings.HasPrefix(whole[0].Error(), "at ") {
		t.Errorf("complaint = %q, want no location prefix", whole[0].Error())
	}
}

func TestValidateInputValue_InputObject(t *testing.T) {
	strict := schema.NewInputObject(schema.InputObjectConfig{
		Name: "Strict",
		Fields: []*schema.InputField{
			schema.NewInputField("needed", schema.InputFieldConfig{Type: schema.NewNonNull(schema.String)}),
			schema.NewInputField("spare", schema.InputFieldConfig{Type: schema.String}),
		},
	})

	tests := []struct {
		name string
		in   any
		want string
	}{
		{"not an object", 1, "to be an object"},
		{"a required field left out", map[string]any{}, `include required field "needed"`},
		{"a required field given as null", map[string]any{"needed": nil}, "not to be null"},
		{"an unknown field", map[string]any{"needed": "x", "extra": 1},
			`not to include unknown field "extra"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problems := schema.ValidateInputValue(tt.in, strict)
			if len(problems) == 0 {
				t.Fatal("nothing was reported")
			}
			joined := strings.Join(messagesOf(problems), "\n")
			if !strings.Contains(joined, tt.want) {
				t.Errorf("complaints =\n%s\nwant one mentioning %q", joined, tt.want)
			}
		})
	}

	// A message about an unknown field shows the value it was found in, and
	// suggests the field that was probably meant.
	problems := schema.ValidateInputValue(map[string]any{"needed": "x", "spar": 1}, strict)
	joined := strings.Join(messagesOf(problems), "\n")
	for _, want := range []string{`Did you mean "spare"?`, `{ needed: "x", spar: 1 }`} {
		if !strings.Contains(joined, want) {
			t.Errorf("complaints do not mention %s:\n%s", want, joined)
		}
	}
}

func TestValidateInputValue_OneOf(t *testing.T) {
	oneOf := schema.NewInputObject(schema.InputObjectConfig{
		Name:    "Ref",
		IsOneOf: true,
		Fields: []*schema.InputField{
			schema.NewInputField("id", schema.InputFieldConfig{Type: schema.ID}),
			schema.NewInputField("name", schema.InputFieldConfig{Type: schema.String}),
		},
	})

	tests := []struct {
		name string
		in   map[string]any
		want string
	}{
		{"nothing at all", map[string]any{}, "exactly one field must be specified"},
		{"two fields", map[string]any{"id": "1", "name": "a"}, "exactly one field must be specified"},
		{"one field given as null", map[string]any{"id": nil}, "must be non-null"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problems := schema.ValidateInputValue(tt.in, oneOf)
			joined := strings.Join(messagesOf(problems), "\n")
			if !strings.Contains(joined, tt.want) {
				t.Errorf("complaints =\n%s\nwant one mentioning %q", joined, tt.want)
			}
		})
	}

	if got := schema.ValidateInputValue(map[string]any{"id": "1"}, oneOf); len(got) != 0 {
		t.Errorf("a sound value was reported as wrong: %v", messagesOf(got))
	}
}

func TestValidateInputValue_Enum(t *testing.T) {
	colour := schema.NewEnum(schema.EnumConfig{
		Name: "Colour",
		Values: []*schema.EnumValue{
			schema.NewEnumValue("RED", schema.EnumValueConfig{}),
			schema.NewEnumValue("GREEN", schema.EnumValueConfig{}),
		},
	})

	// An unknown member is reported by name, along with the member it was
	// probably meant to be.
	problems := schema.ValidateInputValue("RE", colour)
	if len(problems) != 1 {
		t.Fatalf("%d complaints, want 1", len(problems))
	}
	message := problems[0].Message
	for _, want := range []string{`"RE"`, `"Colour"`, `Did you mean the enum value "RED"?`} {
		if !strings.Contains(message, want) {
			t.Errorf("message = %q, want it to mention %s", message, want)
		}
	}

	// Something that is not a name at all is a different mistake.
	notAName := schema.ValidateInputValue(1, colour)
	if len(notAName) != 1 || !strings.Contains(notAName[0].Message, "cannot represent non-string value") {
		t.Errorf("complaints = %v, want one about it not being a name", messagesOf(notAName))
	}
}

// A scalar's own refusal is passed through, so a custom scalar can say
// precisely what it objected to.
func TestValidateInputValue_ScalarCarriesTheReason(t *testing.T) {
	problems := schema.ValidateInputValue("no", schema.Int)
	if len(problems) != 1 {
		t.Fatalf("%d complaints, want 1", len(problems))
	}
	if !strings.Contains(problems[0].Message, "Int cannot represent") {
		t.Errorf("message = %q, want the scalar's own words", problems[0].Message)
	}
}

// Several problems are reported together rather than stopping at the first, so
// that one round of corrections can fix them all.
func TestValidateInputValue_ReportsEverythingWrong(t *testing.T) {
	in := schema.NewInputObject(schema.InputObjectConfig{
		Name: "Several",
		Fields: []*schema.InputField{
			schema.NewInputField("a", schema.InputFieldConfig{Type: schema.Int}),
			schema.NewInputField("b", schema.InputFieldConfig{Type: schema.Int}),
		},
	})
	problems := schema.ValidateInputValue(map[string]any{"a": "no", "b": "also no"}, in)
	if len(problems) != 2 {
		t.Errorf("%d complaints, want 2:\n\t%s", len(problems), strings.Join(messagesOf(problems), "\n\t"))
	}
}

func TestValidateInputValue_SoundValues(t *testing.T) {
	values := []struct {
		in  any
		typ schema.Type
	}{
		{1, schema.Int},
		{nil, schema.String},
		{"x", schema.NewNonNull(schema.String)},
		{[]any{1, nil}, schema.NewList(schema.Int)},
		{map[string]any{"optional": "x"}, testInputObject()},
		{map[string]any{}, testInputObject()},
	}
	for _, tt := range values {
		if got := schema.ValidateInputValue(tt.in, tt.typ); len(got) != 0 {
			t.Errorf("%#v against %s was reported as wrong: %v", tt.in, tt.typ, messagesOf(got))
		}
	}
}

// A type that cannot hold input at all is reported rather than passed over.
func TestValidateInputValue_NonInputType(t *testing.T) {
	object := schema.NewObject(schema.ObjectConfig{
		Name:   "User",
		Fields: []*schema.Field{schema.NewField("a", schema.FieldConfig{Type: schema.String})},
	})
	problems := schema.ValidateInputValue("x", object)
	if len(problems) != 1 || !strings.Contains(problems[0].Message, "input type") {
		t.Errorf("complaints = %v, want one about it not being an input type", messagesOf(problems))
	}
}
