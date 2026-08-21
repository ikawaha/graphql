package schema

import (
	"github.com/ikawaha/graphql/value"
	"slices"
	"testing"
)

func enumNames(values []*EnumValue) []string {
	names := make([]string, len(values))
	for i, v := range values {
		names[i] = v.Name()
	}
	return names
}

func TestEnum_ValuesKeepDeclarationOrder(t *testing.T) {
	e := NewEnum(EnumConfig{
		Name: "Episode",
		Values: []*EnumValue{
			NewEnumValue("NEWHOPE", EnumValueConfig{}),
			NewEnumValue("EMPIRE", EnumValueConfig{}),
			NewEnumValue("JEDI", EnumValueConfig{}),
		},
	})

	want := []string{"NEWHOPE", "EMPIRE", "JEDI"}
	if got := enumNames(e.Values()); !slices.Equal(got, want) {
		t.Errorf("Values() = %v, want %v", got, want)
	}
}

// A member with no internal value stands for its own name, which is what a
// schema read from SDL gets: there is nowhere in SDL to write anything else.
func TestEnum_ValueDefaultsToTheName(t *testing.T) {
	e := NewEnum(EnumConfig{
		Name:   "Episode",
		Values: []*EnumValue{NewEnumValue("JEDI", EnumValueConfig{})},
	})

	member := e.Value("JEDI")
	if member == nil {
		t.Fatal("Value(JEDI) = nil")
	}
	if member.Value != "JEDI" {
		t.Errorf("internal value = %v, want the name", member.Value)
	}
}

// The two lookups run in opposite directions: a document names a member, and a
// resolver returns the internal value.
func TestEnum_LookupsBothWays(t *testing.T) {
	e := NewEnum(EnumConfig{
		Name: "Colour",
		Values: []*EnumValue{
			NewEnumValue("RED", EnumValueConfig{Value: InternalValue(0)}),
			NewEnumValue("GREEN", EnumValueConfig{Value: InternalValue(1)}),
		},
	})

	green := e.Value("GREEN")
	if green == nil {
		t.Fatal("Value(GREEN) = nil")
	}
	if green.Value != 1 {
		t.Errorf("GREEN internal value = %v, want 1", green.Value)
	}
	if got := e.ValueFor(1); got != green {
		t.Errorf("ValueFor(1) = %v, want GREEN", got)
	}
	if e.Value("BLUE") != nil {
		t.Error("Value(BLUE) found something")
	}
	if e.ValueFor(99) != nil {
		t.Error("ValueFor(99) found something")
	}
}

// An internal value that cannot be a map key must still be findable, so it is
// looked up by scanning rather than being indexed. Indexing it would panic.
func TestEnum_UncomparableInternalValues(t *testing.T) {
	slice := []int{1, 2}
	e := NewEnum(EnumConfig{
		Name: "Weird",
		Values: []*EnumValue{
			NewEnumValue("SLICE", EnumValueConfig{Value: InternalValue(slice)}),
			NewEnumValue("PLAIN", EnumValueConfig{Value: InternalValue("plain")}),
		},
	})

	if got := e.ValueFor([]int{1, 2}); got == nil || got.Name() != "SLICE" {
		t.Errorf("ValueFor(a slice) = %v, want SLICE", got)
	}
	if got := e.ValueFor("plain"); got == nil || got.Name() != "PLAIN" {
		t.Errorf("ValueFor(plain) = %v, want PLAIN", got)
	}
	if e.ValueFor([]int{9}) != nil {
		t.Error("ValueFor found a slice that is not a member")
	}
}

// A resolver that returns nothing must not be taken to mean some member. nil
// is never a member's internal value, since a member with none stands for its
// own name.
func TestEnum_ValueForNil(t *testing.T) {
	e := NewEnum(EnumConfig{
		Name:   "Colour",
		Values: []*EnumValue{NewEnumValue("RED", EnumValueConfig{Value: InternalValue(0)})},
	})
	if got := e.ValueFor(nil); got != nil {
		t.Errorf("ValueFor(nil) = %v, want no member", got)
	}
}

func TestEnum_MemberAccessors(t *testing.T) {
	member := NewEnumValue("JEDI", EnumValueConfig{
		Description:       value.Just("Return of the Jedi."),
		DeprecationReason: DeprecatedFor("Renamed."),
	})
	if got := member.String(); got != "JEDI" {
		t.Errorf("String() before attachment = %q, want %q", got, "JEDI")
	}

	e := NewEnum(EnumConfig{Name: "Episode", Values: []*EnumValue{member}})

	// The type holds a member of its own, so the one handed in is left as it
	// was and the type's is the one that knows which enum it belongs to.
	if got := member.Parent(); got != nil {
		t.Errorf("the member handed in was attached to %v", got)
	}
	held := e.Value("JEDI")
	if got := held.Description(); got != "Return of the Jedi." {
		t.Errorf("Description() = %q", got)
	}
	if !held.IsDeprecated() {
		t.Error("IsDeprecated() = false, want true")
	}
	if got := held.Parent(); got != e {
		t.Errorf("Parent() = %v, want the enum", got)
	}
	if got := held.String(); got != "Episode.JEDI" {
		t.Errorf("String() = %q, want %q", got, "Episode.JEDI")
	}

	var absent *EnumValue
	if got := absent.String(); got != "<nil>" {
		t.Errorf("String() on nil = %q", got)
	}
}

// A name written twice is one member: the last of them, where the name first
// appeared. That is what graphql-js's object literal of members comes to, and
// a Go list standing in for one answers the same.
func TestEnum_DuplicatesAndNils(t *testing.T) {
	first := NewEnumValue("A", EnumValueConfig{Value: InternalValue(1)})
	second := NewEnumValue("A", EnumValueConfig{Value: InternalValue(2)})
	last := NewEnumValue("B", EnumValueConfig{Value: InternalValue(3)})
	e := NewEnum(EnumConfig{
		Name:   "Dup",
		Values: []*EnumValue{nil, first, last, second},
	})

	if got := e.Value("A"); got == nil || got.Value != 2 {
		t.Errorf("Value(\"A\") = %v, want the last member of that name", got)
	}
	// The hole stays, A keeps the place it first had, and B follows it.
	var named []string
	for _, v := range e.Values() {
		if v == nil {
			named = append(named, "(nil)")
			continue
		}
		named = append(named, v.Name())
	}
	if want := []string{"(nil)", "A", "B"}; !slices.Equal(named, want) {
		t.Errorf("Values() = %v, want %v", named, want)
	}
}

func TestEnum_Predicates(t *testing.T) {
	e := NewEnum(EnumConfig{Name: "Episode"})

	if !IsEnumType(e) {
		t.Error("IsEnumType = false, want true")
	}
	// An enum can appear on either side of a request and a selection ends at
	// one, the same as a scalar.
	if !IsInputType(e) || !IsOutputType(e) || !IsLeafType(e) {
		t.Errorf("enum: input=%v output=%v leaf=%v, want all true",
			IsInputType(e), IsOutputType(e), IsLeafType(e))
	}
	if IsCompositeType(e) || IsAbstractType(e) {
		t.Error("an enum was reported as composite or abstract")
	}
	if got := e.String(); got != "Episode" {
		t.Errorf("String() = %q", got)
	}
	if got := e.Description(); got != "" {
		t.Errorf("Description() = %q, want empty", got)
	}
	var absent *EnumType
	if got := absent.String(); got != "<nil>" {
		t.Errorf("String() on nil = %q", got)
	}
}
