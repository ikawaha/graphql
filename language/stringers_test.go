package language

import "testing"

// The String methods are what puts these values into an error message, so what
// they render is part of what a reader sees.
func TestStringers(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"a token kind that is punctuation", TokenBraceL.String(), "{"},
		{"a token kind that is a name", TokenName.String(), "Name"},
		{"the start of the document", TokenSOF.String(), "<SOF>"},
		{"the end of the document", TokenEOF.String(), "<EOF>"},

		{"a node kind", KindField.String(), "Field"},
		{"a schema coordinate kind", KindMemberCoordinate.String(), "MemberCoordinate"},

		{"an operation type", OperationQuery.String(), "query"},
		{"a directive location", DirectiveLocationFieldDefinition.String(), "FIELD_DEFINITION"},

		// A token carrying a value shows it, quoted; one that is only
		// punctuation is named by its kind alone.
		{"a token with a value", (&Token{Kind: TokenName, Value: "hero"}).String(), `Name "hero"`},
		{"a token with none", (&Token{Kind: TokenBraceL}).String(), "{"},
		{"a token that is not there", (*Token)(nil).String(), "<nil>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("String() = %q, want %q", tt.got, tt.want)
			}
		})
	}
}
