package schema

import (
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr string
	}{
		{"Query", ""},
		{"_", ""},
		{"_private", ""},
		{"a1", ""},
		{"snake_case", ""},
		{"CamelCase", ""},
		{"__typename", ""},
		{"", "non-empty"},
		{"1Query", "must start with"},
		{"9", "must start with"},
		{"has space", "must only contain"},
		{"has-dash", "must only contain"},
		{"has.dot", "must only contain"},
		{"名前", "must only contain"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.name)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("ValidateName(%q) = %v, want no error", tt.name, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateName(%q) = nil, want an error", tt.name)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}

// A name that both starts badly and contains a bad character is reported for
// the character, since that is the more specific complaint.
func TestValidateName_ReportsTheContentFirst(t *testing.T) {
	err := ValidateName("1 2")
	if err == nil {
		t.Fatal("ValidateName = nil, want an error")
	}
	if !strings.Contains(err.Error(), "must only contain") {
		t.Errorf("error = %q, want it to complain about the characters", err)
	}
}

func TestValidateEnumValueName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr string
	}{
		{"ACTIVE", ""},
		{"_PRIVATE", ""},
		{"True", ""},
		{"NULL", ""},
		{"true", "cannot be named"},
		{"false", "cannot be named"},
		{"null", "cannot be named"},
		{"", "non-empty"},
		{"1ACTIVE", "must start with"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEnumValueName(tt.name)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("ValidateEnumValueName(%q) = %v, want no error", tt.name, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateEnumValueName(%q) = nil, want an error", tt.name)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}
