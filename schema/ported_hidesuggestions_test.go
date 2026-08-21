package schema_test

// Ported from the graphql-js cases that pass hideSuggestions to
// validateInputValue and validateInputLiteral: the same values as elsewhere,
// with the "Did you mean …?" left off.

import (
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

func hideSuggestionsSchema(t *testing.T) (*schema.EnumType, *schema.InputObjectType) {
	t.Helper()
	enum := schema.NewEnum(schema.EnumConfig{
		Name: "TestEnum",
		Values: []*schema.EnumValue{
			schema.NewEnumValue("FOO", schema.EnumValueConfig{}),
			schema.NewEnumValue("BAR", schema.EnumValueConfig{}),
		},
	})
	input := schema.NewInputObject(schema.InputObjectConfig{
		Name: "TestInputObject",
		Fields: []*schema.InputField{
			schema.NewInputField("foo", schema.InputFieldConfig{
				Type: schema.NewNonNull(schema.Int),
			}),
			schema.NewInputField("bar", schema.InputFieldConfig{Type: schema.Int}),
		},
	})
	return enum, input
}

func TestPortedWithoutSuggestions_InputValue(t *testing.T) {
	enum, input := hideSuggestionsSchema(t)

	for _, tt := range []struct {
		name    string
		in      any
		as      schema.Type
		with    string
		without string
	}{
		{
			name:    "a misspelled enum member",
			in:      "foo",
			as:      enum,
			with:    `Value "foo" does not exist in "TestEnum" enum. Did you mean the enum value "FOO"?`,
			without: `Value "foo" does not exist in "TestEnum" enum.`,
		},
		{
			name: "a misspelled field of an input object",
			in:   map[string]any{"foo": 123, "bart": 123},
			as:   input,
			with: `Expected value of type "TestInputObject" not to include unknown field "bart". ` +
				`Did you mean "bar"? Found: { bart: 123, foo: 123 }.`,
			without: `Expected value of type "TestInputObject" not to include unknown field "bart", ` +
				`found: { bart: 123, foo: 123 }.`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			expectFirstMessage(t, tt.with, valueMessagesOf(schema.ValidateInputValue(tt.in, tt.as)))
			expectFirstMessage(t, tt.without,
				valueMessagesOf(schema.ValidateInputValue(tt.in, tt.as, schema.WithoutSuggestions())))
		})
	}
}

func TestPortedWithoutSuggestions_InputLiteral(t *testing.T) {
	enum, input := hideSuggestionsSchema(t)

	for _, tt := range []struct {
		name          string
		written       string
		as            schema.Type
		with, without string
	}{
		{
			name:    "a misspelled enum member",
			written: "foo",
			as:      enum,
			with:    `Value "foo" does not exist in "TestEnum" enum. Did you mean the enum value "FOO"?`,
			without: `Value "foo" does not exist in "TestEnum" enum.`,
		},
		{
			name:    "a misspelled field of an input object",
			written: `{ foo: 123, bart: 123 }`,
			as:      input,
			with: `Expected value of type "TestInputObject" not to include unknown field "bart". ` +
				`Did you mean "bar"? Found: { foo: 123, bart: 123 }.`,
			without: `Expected value of type "TestInputObject" not to include unknown field "bart", ` +
				`found: { foo: 123, bart: 123 }.`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			literal, err := language.ParseValue(language.NewSource(tt.written))
			if err != nil {
				t.Fatalf("parsing %q: %v", tt.written, err)
			}
			expectFirstMessage(t, tt.with,
				literalMessagesOf(schema.ValidateInputLiteral(literal, tt.as, schema.VariableValues{})))
			expectFirstMessage(t, tt.without,
				literalMessagesOf(schema.ValidateInputLiteral(literal, tt.as, schema.VariableValues{},
					schema.WithoutSuggestions())))
		})
	}
}

func valueMessagesOf(found []schema.InputValueError) []string {
	out := make([]string, len(found))
	for i, why := range found {
		out[i] = why.Message
	}
	return out
}

func literalMessagesOf(found []schema.LiteralError) []string {
	out := make([]string, len(found))
	for i, why := range found {
		out[i] = why.Message
	}
	return out
}

func expectFirstMessage(t *testing.T, want string, got []string) {
	t.Helper()
	if len(got) == 0 {
		t.Fatalf("nothing was reported, want %q", want)
	}
	if got[0] != want {
		t.Errorf("says %q\nwant %q", got[0], want)
	}
}
