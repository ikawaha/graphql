package value_test

import (
	"reflect"
	"testing"

	"github.com/ikawaha/graphql/value"
)

func TestPath_Root(t *testing.T) {
	var p *value.Path
	if got := p.Len(); got != 0 {
		t.Errorf("Len() = %d, want 0", got)
	}
	if got := p.AsSlice(); got != nil {
		t.Errorf("AsSlice() = %v, want nil", got)
	}
	if got := p.String(); got != "" {
		t.Errorf("String() = %q, want empty", got)
	}
	if p.IsIndex() {
		t.Error("IsIndex() on the root returned true")
	}
}

func TestPath_Build(t *testing.T) {
	tests := []struct {
		name      string
		build     func() *value.Path
		wantSlice []any
		wantStr   string
	}{
		{
			name:      "single field",
			build:     func() *value.Path { return (*value.Path)(nil).WithField("hero", "Query") },
			wantSlice: []any{"hero"},
			wantStr:   ".hero",
		},
		{
			name: "nested fields",
			build: func() *value.Path {
				return (*value.Path)(nil).WithField("hero", "Query").WithField("name", "Droid")
			},
			wantSlice: []any{"hero", "name"},
			wantStr:   ".hero.name",
		},
		{
			name: "list index",
			build: func() *value.Path {
				return (*value.Path)(nil).WithField("friends", "Query").WithIndex(0)
			},
			wantSlice: []any{"friends", 0},
			wantStr:   ".friends[0]",
		},
		{
			name: "index then field",
			build: func() *value.Path {
				return (*value.Path)(nil).
					WithField("friends", "Query").
					WithIndex(2).
					WithField("name", "Human")
			},
			wantSlice: []any{"friends", 2, "name"},
			wantStr:   ".friends[2].name",
		},
		{
			name: "nested lists",
			build: func() *value.Path {
				return (*value.Path)(nil).WithField("matrix", "Query").WithIndex(1).WithIndex(3)
			},
			wantSlice: []any{"matrix", 1, 3},
			wantStr:   ".matrix[1][3]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.build()
			if got := p.AsSlice(); !reflect.DeepEqual(got, tt.wantSlice) {
				t.Errorf("AsSlice() = %v, want %v", got, tt.wantSlice)
			}
			if got := p.String(); got != tt.wantStr {
				t.Errorf("String() = %q, want %q", got, tt.wantStr)
			}
			if got := p.Len(); got != len(tt.wantSlice) {
				t.Errorf("Len() = %d, want %d", got, len(tt.wantSlice))
			}
		})
	}
}

// Appending must not disturb the path it was derived from, since sibling
// fields share a parent path during execution.
func TestPath_AppendDoesNotMutateParent(t *testing.T) {
	parent := (*value.Path)(nil).WithField("hero", "Query")
	a := parent.WithField("name", "Droid")
	b := parent.WithField("id", "Droid")

	if got, want := parent.String(), ".hero"; got != want {
		t.Errorf("parent changed: %q, want %q", got, want)
	}
	if got, want := a.String(), ".hero.name"; got != want {
		t.Errorf("a = %q, want %q", got, want)
	}
	if got, want := b.String(), ".hero.id"; got != want {
		t.Errorf("b = %q, want %q", got, want)
	}
}

func TestPath_IsIndexAndTypeName(t *testing.T) {
	field := (*value.Path)(nil).WithField("hero", "Query")
	if field.IsIndex() {
		t.Error("IsIndex() on a field segment returned true")
	}
	if got := field.TypeName; got != "Query" {
		t.Errorf("TypeName = %q, want %q", got, "Query")
	}

	idx := field.WithIndex(0)
	if !idx.IsIndex() {
		t.Error("IsIndex() on a list segment returned false")
	}
	if got := idx.TypeName; got != "" {
		t.Errorf("TypeName on a list segment = %q, want empty", got)
	}
}
