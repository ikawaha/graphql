package schema

import (
	"github.com/ikawaha/graphql/value"
	"testing"
)

// The three states an argument can be in reach a resolver through this type,
// which is why it exposes presence separately from the value. Indexing the map
// directly would collapse "omitted" and "given as null" into the same nil.
func TestArguments_ThreeStates(t *testing.T) {
	args := NewArguments(map[string]any{
		"withValue": 7,
		"withNull":  nil,
	})

	tests := []struct {
		name      string
		arg       string
		wantValue any
		wantOK    bool
	}{
		{"a value", "withValue", 7, true},
		{"an explicit null", "withNull", nil, true},
		{"omitted", "missing", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := args.Get(tt.arg)
			if ok != tt.wantOK {
				t.Errorf("Get(%q) present = %v, want %v", tt.arg, ok, tt.wantOK)
			}
			if got != tt.wantValue {
				t.Errorf("Get(%q) = %v, want %v", tt.arg, got, tt.wantValue)
			}
			if has := args.Has(tt.arg); has != tt.wantOK {
				t.Errorf("Has(%q) = %v, want %v", tt.arg, has, tt.wantOK)
			}
		})
	}
}

func TestArguments_LenAndRaw(t *testing.T) {
	values := map[string]any{"a": 1, "b": nil}
	args := NewArguments(values)

	if got := args.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}
	if got := args.Raw(); len(got) != 2 {
		t.Errorf("Raw() has %d entries, want 2", len(got))
	}

	// The zero value is usable and simply holds nothing.
	var empty Arguments
	if got := empty.Len(); got != 0 {
		t.Errorf("Len() on the zero value = %d, want 0", got)
	}
	if _, ok := empty.Get("a"); ok {
		t.Error("Get on the zero value found something")
	}
	if empty.Has("a") {
		t.Error("Has on the zero value returned true")
	}
}

func TestInterface_Accessors(t *testing.T) {
	node := NewInterface(InterfaceConfig{Name: "Node"})
	named := NewInterface(InterfaceConfig{
		Name:        "Named",
		Description: value.Just("Something with a name."),
		Interfaces:  Implements(node),
	})

	if got := named.Name(); got != "Named" {
		t.Errorf("Name() = %q, want %q", got, "Named")
	}
	if got := named.Description(); got != "Something with a name." {
		t.Errorf("Description() = %q", got)
	}
	if got := named.String(); got != "Named" {
		t.Errorf("String() = %q, want %q", got, "Named")
	}
	if got := named.Interfaces(); len(got) != 1 || got[0].Named() != NamedType(node) {
		t.Errorf("Interfaces() = %v, want [Node]", got)
	}
}

func TestObject_Accessors(t *testing.T) {
	obj := NewObject(ObjectConfig{Name: "User", Description: value.Just("A person.")})
	if got := obj.Name(); got != "User" {
		t.Errorf("Name() = %q, want %q", got, "User")
	}
	if got := obj.Description(); got != "A person." {
		t.Errorf("Description() = %q", got)
	}
	// A type prints as its name, which is how it appears inside a wrapper.
	if got := obj.String(); got != "User" {
		t.Errorf("String() = %q, want %q", got, "User")
	}
	if got := NewNonNull(NewList(obj)).String(); got != "[User]!" {
		t.Errorf("wrapped String() = %q, want %q", got, "[User]!")
	}
}
