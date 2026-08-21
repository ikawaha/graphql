package schema

import (
	"github.com/ikawaha/graphql/value"
	"slices"
	"testing"

	"github.com/ikawaha/graphql/language"
)

func argNames(args []*Argument) []string {
	names := make([]string, len(args))
	for i, a := range args {
		names[i] = a.Name()
	}
	return names
}

// Each built-in directive has to be applicable exactly where the
// specification says, since validation reports anything else as an error.
func TestSpecifiedDirectives_Shape(t *testing.T) {
	tests := []struct {
		directive *Directive
		name      string
		locations []language.DirectiveLocation
		args      []string
	}{
		{
			directive: Include,
			name:      "include",
			locations: []language.DirectiveLocation{
				language.DirectiveLocationField,
				language.DirectiveLocationFragmentSpread,
				language.DirectiveLocationInlineFragment,
			},
			args: []string{"if"},
		},
		{
			directive: Skip,
			name:      "skip",
			locations: []language.DirectiveLocation{
				language.DirectiveLocationField,
				language.DirectiveLocationFragmentSpread,
				language.DirectiveLocationInlineFragment,
			},
			args: []string{"if"},
		},
		{
			directive: Deprecated,
			name:      "deprecated",
			locations: []language.DirectiveLocation{
				language.DirectiveLocationFieldDefinition,
				language.DirectiveLocationArgumentDefinition,
				language.DirectiveLocationInputFieldDefinition,
				language.DirectiveLocationEnumValue,
				language.DirectiveLocationDirectiveDefinition,
			},
			args: []string{"reason"},
		},
		{
			directive: SpecifiedBy,
			name:      "specifiedBy",
			locations: []language.DirectiveLocation{language.DirectiveLocationScalar},
			args:      []string{"url"},
		},
		{
			directive: OneOf,
			name:      "oneOf",
			locations: []language.DirectiveLocation{language.DirectiveLocationInputObject},
			args:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := tt.directive
			if got := d.Name(); got != tt.name {
				t.Errorf("Name() = %q, want %q", got, tt.name)
			}
			if got := d.String(); got != "@"+tt.name {
				t.Errorf("String() = %q, want %q", got, "@"+tt.name)
			}
			if !slices.Equal(d.Locations, tt.locations) {
				t.Errorf("Locations = %v, want %v", d.Locations, tt.locations)
			}
			if got := argNames(d.Args); !slices.Equal(got, tt.args) {
				t.Errorf("args = %v, want %v", got, tt.args)
			}
			if d.IsRepeatable {
				t.Error("IsRepeatable = true, want false")
			}
			if d.Description() == "" {
				t.Error("Description() is empty")
			}
		})
	}
}

// The if argument of skip and include has to be supplied: it is non-null and
// has no default.
func TestSkipAndInclude_RequireTheirArgument(t *testing.T) {
	for _, d := range []*Directive{Skip, Include} {
		arg := d.Arg("if")
		if arg == nil {
			t.Fatalf("%s has no if argument", d)
		}
		if !IsNonNullType(arg.Type) {
			t.Errorf("%s.if type = %v, want it non-null", d, arg.Type)
		}
		if !IsRequiredArgument(arg) {
			t.Errorf("%s.if is not required, want it required", d)
		}
	}
}

// The reason of @deprecated is non-null and yet optional, because it has a
// default. A design that could not tell "no default" from "a default of null"
// would get this wrong.
func TestDeprecated_ReasonIsOptionalDespiteBeingNonNull(t *testing.T) {
	reason := Deprecated.Arg("reason")
	if reason == nil {
		t.Fatal("@deprecated has no reason argument")
	}
	if !IsNonNullType(reason.Type) {
		t.Errorf("reason type = %v, want it non-null", reason.Type)
	}
	if IsRequiredArgument(reason) {
		t.Error("reason is required, want it optional because it has a default")
	}

	got, has := reason.Default.Get()
	if !has {
		t.Fatal("reason has no default")
	}
	if got.Value != DefaultDeprecationReason {
		t.Errorf("default = %v, want %q", got.Value, DefaultDeprecationReason)
	}
}

func TestDirective_HasLocation(t *testing.T) {
	if !Skip.HasLocation(language.DirectiveLocationField) {
		t.Error("@skip is not allowed on a field")
	}
	if Skip.HasLocation(language.DirectiveLocationObject) {
		t.Error("@skip is allowed on an object type")
	}

	var absent *Directive
	if absent.HasLocation(language.DirectiveLocationField) {
		t.Error("HasLocation on nil = true, want false")
	}
}

func TestDirective_Arg(t *testing.T) {
	if got := SpecifiedBy.Arg("url"); got == nil {
		t.Error("Arg(url) = nil")
	}
	if got := SpecifiedBy.Arg("missing"); got != nil {
		t.Errorf("Arg(missing) = %v, want nil", got)
	}
	if got := OneOf.Arg("anything"); got != nil {
		t.Errorf("Arg on a directive with no arguments = %v, want nil", got)
	}

	var absent *Directive
	if absent.Arg("url") != nil {
		t.Error("Arg on nil returned something")
	}
	if got := absent.String(); got != "<nil>" {
		t.Errorf("String() on nil = %q", got)
	}
}

func TestSpecifiedDirectives(t *testing.T) {
	want := []string{"include", "skip", "deprecated", "specifiedBy", "oneOf"}
	got := make([]string, len(SpecifiedDirectives))
	for i, d := range SpecifiedDirectives {
		got[i] = d.Name()
	}
	if !slices.Equal(got, want) {
		t.Errorf("SpecifiedDirectives = %v, want %v", got, want)
	}
	for _, d := range SpecifiedDirectives {
		if !IsSpecifiedDirective(d) {
			t.Errorf("%s was not recognised as a specified directive", d)
		}
	}

	custom := NewDirective(DirectiveConfig{Name: "auth"})
	if IsSpecifiedDirective(custom) {
		t.Error("a custom directive was recognised as a specified one")
	}
	// A directive of a built-in's name counts as that directive, whether or
	// not it is the same object: a schema that declares @skip for itself has
	// declared the one the specification names. graphql-js asks by name too.
	own := NewDirective(DirectiveConfig{Name: "skip"})
	if !IsSpecifiedDirective(own) {
		t.Error("a schema's own @skip was not recognised as the specified one")
	}
	if IsSpecifiedDirective(nil) {
		t.Error("IsSpecifiedDirective(nil) = true, want false")
	}
}

// Incremental delivery is not implemented, so a schema does not gain these
// unless it asks. They are defined because validation has rules about them.
func TestDeferAndStream_AreNotSpecifiedDirectives(t *testing.T) {
	for _, d := range []*Directive{Defer, Stream} {
		if IsSpecifiedDirective(d) {
			t.Errorf("%s is among the specified directives, want it left out", d)
		}
	}

	if !slices.Equal(Defer.Locations, []language.DirectiveLocation{
		language.DirectiveLocationFragmentSpread,
		language.DirectiveLocationInlineFragment,
	}) {
		t.Errorf("@defer locations = %v", Defer.Locations)
	}
	if !slices.Equal(Stream.Locations, []language.DirectiveLocation{
		language.DirectiveLocationField,
	}) {
		t.Errorf("@stream locations = %v", Stream.Locations)
	}

	// Both default their condition to true, so writing them bare means "yes".
	for _, d := range []*Directive{Defer, Stream} {
		arg := d.Arg("if")
		if arg == nil {
			t.Fatalf("%s has no if argument", d)
		}
		if IsRequiredArgument(arg) {
			t.Errorf("%s.if is required, want it optional", d)
		}
		got, has := arg.Default.Get()
		if !has || got.Value != true {
			t.Errorf("%s.if default = %v (present=%v), want true", d, got.Value, has)
		}
		if d.Arg("label") == nil {
			t.Errorf("%s has no label argument", d)
		}
	}
}

func TestNewDirective_Repeatable(t *testing.T) {
	d := NewDirective(DirectiveConfig{
		Name:         "tag",
		Description:  value.Just("Adds a tag."),
		Locations:    []language.DirectiveLocation{language.DirectiveLocationFieldDefinition},
		IsRepeatable: true,
		Args: []*Argument{
			NewArgument("name", ArgumentConfig{Type: NewNonNull(String)}),
		},
	})

	if !d.IsRepeatable {
		t.Error("IsRepeatable = false, want true")
	}
	if got := d.String(); got != "@tag" {
		t.Errorf("String() = %q, want %q", got, "@tag")
	}
	if !d.HasLocation(language.DirectiveLocationFieldDefinition) {
		t.Error("the declared location was not recorded")
	}
}
