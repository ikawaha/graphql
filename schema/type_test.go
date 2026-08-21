package schema

import "testing"

// The concrete named types are not written yet, so these stand in for them.
// Each implements exactly the markers a real type of that shape would.

type stubInput struct{ name string }

func (s *stubInput) Name() string      { return s.name }
func (s *stubInput) String() string    { return s.name }
func (*stubInput) Description() string { return "" }
func (*stubInput) isType()             {}
func (*stubInput) isNamedType()        {}
func (*stubInput) isInputType()        {}

type stubOutput struct{ name string }

func (s *stubOutput) Name() string      { return s.name }
func (s *stubOutput) String() string    { return s.name }
func (*stubOutput) Description() string { return "" }
func (*stubOutput) isType()             {}
func (*stubOutput) isNamedType()        {}
func (*stubOutput) isOutputType()       {}

// A scalar is usable in both positions, which is what makes the two markers
// independent rather than a single flag.
type stubLeaf struct{ name string }

func (s *stubLeaf) Name() string      { return s.name }
func (*stubLeaf) Description() string { return "" }
func (s *stubLeaf) String() string    { return s.name }
func (*stubLeaf) isType()             {}
func (*stubLeaf) isNamedType()        {}
func (*stubLeaf) isInputType()        {}
func (*stubLeaf) isOutputType()       {}

func TestType_String(t *testing.T) {
	scalar := &stubLeaf{name: "Int"}
	tests := []struct {
		typ  Type
		want string
	}{
		{scalar, "Int"},
		{NewList(scalar), "[Int]"},
		{NewNonNull(scalar), "Int!"},
		{NewNonNull(NewList(scalar)), "[Int]!"},
		{NewList(NewNonNull(scalar)), "[Int!]"},
		{NewNonNull(NewList(NewNonNull(scalar))), "[Int!]!"},
		{NewList(NewList(scalar)), "[[Int]]"},
	}
	for _, tt := range tests {
		if got := tt.typ.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}

// A doubly non-null type cannot be written in a schema, so building one is a
// programming mistake rather than something to validate later.
func TestNewNonNull_RejectsDoubleWrapping(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("wrapping a non-null in another non-null did not panic")
		}
	}()
	NewNonNull(NewNonNull(&stubLeaf{name: "Int"}))
}

func TestType_Predicates(t *testing.T) {
	scalar := &stubLeaf{name: "Int"}
	list := NewList(scalar)
	nonNull := NewNonNull(scalar)

	tests := []struct {
		name      string
		predicate func(Type) bool
		accepts   []Type
		rejects   []Type
	}{
		{"IsWrappingType", IsWrappingType, []Type{list, nonNull}, []Type{scalar}},
		{"IsListType", IsListType, []Type{list}, []Type{scalar, nonNull}},
		{"IsNonNullType", IsNonNullType, []Type{nonNull}, []Type{scalar, list}},
		{"IsNamedType", IsNamedType, []Type{scalar}, []Type{list, nonNull}},
		{"IsNullableType", IsNullableType, []Type{scalar, list}, []Type{nonNull}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, typ := range tt.accepts {
				if !tt.predicate(typ) {
					t.Errorf("%s(%s) = false, want true", tt.name, typ)
				}
			}
			for _, typ := range tt.rejects {
				if tt.predicate(typ) {
					t.Errorf("%s(%s) = true, want false", tt.name, typ)
				}
			}
		})
	}
}

func TestNullableTypeOf(t *testing.T) {
	scalar := &stubLeaf{name: "Int"}
	list := NewList(scalar)

	tests := []struct {
		in   Type
		want Type
	}{
		{scalar, scalar},
		{list, list},
		{NewNonNull(scalar), scalar},
		{NewNonNull(list), list},
	}
	for _, tt := range tests {
		if got := NullableTypeOf(tt.in); got != tt.want {
			t.Errorf("NullableTypeOf(%s) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestNamedTypeOf(t *testing.T) {
	scalar := &stubLeaf{name: "Int"}
	tests := []struct {
		in   Type
		want NamedType
	}{
		{scalar, scalar},
		{NewList(scalar), scalar},
		{NewNonNull(scalar), scalar},
		{NewNonNull(NewList(NewNonNull(scalar))), scalar},
		{nil, nil},
	}
	for _, tt := range tests {
		if got := NamedTypeOf(tt.in); got != tt.want {
			t.Errorf("NamedTypeOf(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// Whether a wrapper may be used as input or output depends on what it wraps,
// so the check has to look through the wrappers.
func TestIsInputAndOutputType(t *testing.T) {
	input := &stubInput{name: "Filter"}
	output := &stubOutput{name: "User"}
	leaf := &stubLeaf{name: "Int"}

	tests := []struct {
		typ         Type
		wantInput   bool
		wantOutput  bool
		description string
	}{
		{leaf, true, true, "a scalar is usable in both positions"},
		{input, true, false, "an input object is input only"},
		{output, false, true, "an object is output only"},
		{NewList(input), true, false, "a list of an input object is input only"},
		{NewNonNull(output), false, true, "a non-null object is output only"},
		{NewNonNull(NewList(NewNonNull(input))), true, false, "wrappers nest"},
		{NewList(leaf), true, true, "a list of a scalar is usable in both"},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			if got := IsInputType(tt.typ); got != tt.wantInput {
				t.Errorf("IsInputType(%s) = %v, want %v", tt.typ, got, tt.wantInput)
			}
			if got := IsOutputType(tt.typ); got != tt.wantOutput {
				t.Errorf("IsOutputType(%s) = %v, want %v", tt.typ, got, tt.wantOutput)
			}
		})
	}
}

func TestType_Unwrap(t *testing.T) {
	scalar := &stubLeaf{name: "Int"}
	if got := NewList(scalar).Unwrap(); got != scalar {
		t.Errorf("List.Unwrap() = %v, want the element type", got)
	}
	if got := NewNonNull(scalar).Unwrap(); got != scalar {
		t.Errorf("NonNull.Unwrap() = %v, want the wrapped type", got)
	}
}

// A half-built type still prints something rather than panicking, so a type
// under construction can be logged.
func TestType_StringOnIncompleteWrappers(t *testing.T) {
	if got := (&List{}).String(); got != "[?]" {
		t.Errorf("List with no element type printed %q", got)
	}
	if got := (&NonNull{}).String(); got != "?!" {
		t.Errorf("NonNull with no wrapped type printed %q", got)
	}
	var absentList *List
	if got := absentList.String(); got != "[?]" {
		t.Errorf("nil List printed %q", got)
	}
	var absentNonNull *NonNull
	if got := absentNonNull.String(); got != "?!" {
		t.Errorf("nil NonNull printed %q", got)
	}
}
