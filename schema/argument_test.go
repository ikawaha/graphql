package schema

import (
	"github.com/ikawaha/graphql/value"
	"testing"

	"github.com/ikawaha/graphql/language"
)

func testScalar(name string) *ScalarType { return NewScalar(ScalarConfig{Name: name}) }

// The three states a default value can be in are what the whole design rests
// on, and they are what decides whether a caller must supply an argument.
func TestArgument_DefaultThreeStates(t *testing.T) {
	intType := testScalar("Int")

	tests := []struct {
		name        string
		config      ArgumentConfig
		wantHas     bool
		wantValue   any
		description string
	}{
		{
			name:        "no default at all",
			config:      ArgumentConfig{Type: intType},
			wantHas:     false,
			description: "an unset field means the argument has no default",
		},
		{
			name:        "an explicit absence",
			config:      ArgumentConfig{Type: intType, Default: NoDefault()},
			wantHas:     false,
			description: "NoDefault says the same thing out loud",
		},
		{
			name:        "a default of null",
			config:      ArgumentConfig{Type: intType, Default: DefaultValue(nil)},
			wantHas:     true,
			wantValue:   nil,
			description: "a default that is null is still a default",
		},
		{
			name:        "a default value",
			config:      ArgumentConfig{Type: intType, Default: DefaultValue(7)},
			wantHas:     true,
			wantValue:   7,
			description: "an ordinary default",
		},
		{
			name:        "a default of zero",
			config:      ArgumentConfig{Type: intType, Default: DefaultValue(0)},
			wantHas:     true,
			wantValue:   0,
			description: "zero is a value, not an absence",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arg := NewArgument("a", tt.config)
			got, has := arg.Default.Get()
			if has != tt.wantHas {
				t.Fatalf("%s: default present = %v, want %v", tt.description, has, tt.wantHas)
			}
			if has && got.Value != tt.wantValue {
				t.Errorf("default value = %v, want %v", got.Value, tt.wantValue)
			}
		})
	}
}

// An argument is required only when its type is non-null and it has no default
// at all. A default of null makes it optional, which is exactly the case that
// a design without three states would get wrong.
func TestIsRequiredArgument(t *testing.T) {
	intType := testScalar("Int")

	tests := []struct {
		name   string
		config ArgumentConfig
		want   bool
	}{
		{"nullable, no default", ArgumentConfig{Type: intType}, false},
		{"nullable, default null", ArgumentConfig{Type: intType, Default: DefaultValue(nil)}, false},
		{"nullable, default value", ArgumentConfig{Type: intType, Default: DefaultValue(1)}, false},
		{"non-null, no default", ArgumentConfig{Type: NewNonNull(intType)}, true},
		{"non-null, default null", ArgumentConfig{Type: NewNonNull(intType), Default: DefaultValue(nil)}, false},
		{"non-null, default value", ArgumentConfig{Type: NewNonNull(intType), Default: DefaultValue(1)}, false},
		{"non-null list, no default", ArgumentConfig{Type: NewNonNull(NewList(intType))}, true},
		{"list of non-null, no default", ArgumentConfig{Type: NewList(NewNonNull(intType))}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRequiredArgument(NewArgument("a", tt.config)); got != tt.want {
				t.Errorf("IsRequiredArgument() = %v, want %v", got, tt.want)
			}
		})
	}

	if IsRequiredArgument(nil) {
		t.Error("IsRequiredArgument(nil) = true, want false")
	}
}

// An input object field follows the same rule.
func TestIsRequiredInputField(t *testing.T) {
	intType := testScalar("Int")

	tests := []struct {
		name   string
		config InputFieldConfig
		want   bool
	}{
		{"nullable, no default", InputFieldConfig{Type: intType}, false},
		{"non-null, no default", InputFieldConfig{Type: NewNonNull(intType)}, true},
		{"non-null, default null", InputFieldConfig{Type: NewNonNull(intType), Default: DefaultValue(nil)}, false},
		{"non-null, default value", InputFieldConfig{Type: NewNonNull(intType), Default: DefaultValue(1)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRequiredInputField(NewInputField("f", tt.config)); got != tt.want {
				t.Errorf("IsRequiredInputField() = %v, want %v", got, tt.want)
			}
		})
	}

	if IsRequiredInputField(nil) {
		t.Error("IsRequiredInputField(nil) = true, want false")
	}
}

// A schema read from SDL keeps the literal its default was written as, rather
// than converting it, so that printing the schema gives back what was written.
func TestArgument_DefaultLiteral(t *testing.T) {
	literal := &language.EnumValue{Value: "JEDI"}
	arg := NewArgument("episode", ArgumentConfig{
		Type:    testScalar("Episode"),
		Default: DefaultLiteral(literal),
	})

	got, has := arg.Default.Get()
	if !has {
		t.Fatal("default is absent, want it present")
	}
	if got.Literal != literal {
		t.Errorf("Literal = %v, want the literal that was supplied", got.Literal)
	}
	if got.Value != nil {
		t.Errorf("Value = %v, want nil when a literal was supplied", got.Value)
	}
}

func TestArgument_Accessors(t *testing.T) {
	arg := NewArgument("first", ArgumentConfig{
		Description:       value.Just("How many."),
		Type:              NewNonNull(testScalar("Int")),
		DeprecationReason: DeprecatedFor("Use last."),
	})

	if got := arg.Name(); got != "first" {
		t.Errorf("Name() = %q, want %q", got, "first")
	}
	if got := arg.Description(); got != "How many." {
		t.Errorf("Description() = %q, want %q", got, "How many.")
	}
	if !arg.IsDeprecated() {
		t.Error("IsDeprecated() = false, want true")
	}
	// An argument on its own has no owner to name, so it is named alone.
	if got := arg.String(); got != "first" {
		t.Errorf("String() = %q, want %q", got, "first")
	}

	plain := NewArgument("a", ArgumentConfig{Type: testScalar("Int")})
	if plain.IsDeprecated() {
		t.Error("IsDeprecated() = true for an argument with no reason")
	}
}

func TestInputField_Accessors(t *testing.T) {
	field := NewInputField("limit", InputFieldConfig{
		Description:       value.Just("How many."),
		Type:              testScalar("Int"),
		DeprecationReason: DeprecatedFor("Gone."),
	})

	if got := field.Name(); got != "limit" {
		t.Errorf("Name() = %q, want %q", got, "limit")
	}
	if got := field.Description(); got != "How many." {
		t.Errorf("Description() = %q, want %q", got, "How many.")
	}
	if !field.IsDeprecated() {
		t.Error("IsDeprecated() = false, want true")
	}
	if got := field.String(); got != "limit" {
		t.Errorf("String() = %q, want %q", got, "limit")
	}
}

// TestArgumentAndInputField_NameTheirOwner is graphql-js's toString on
// GraphQLArgument and GraphQLInputField: once a field, a directive or an
// input object owns one, it names itself the way a schema coordinate does.
func TestArgumentAndInputField_NameTheirOwner(t *testing.T) {
	shared := NewArgument("name", ArgumentConfig{Type: testScalar("String")})
	query := NewObject(ObjectConfig{
		Name: "Query",
		Fields: []*Field{
			NewField("greeting", FieldConfig{Type: testScalar("String"), Args: []*Argument{shared}}),
		},
	})
	if got, want := query.Field("greeting").Args[0].String(), "Query.greeting(name:)"; got != want {
		t.Errorf("field argument: got %q, want %q", got, want)
	}

	skip := NewDirective(DirectiveConfig{Name: "skip", Args: []*Argument{shared}})
	if got, want := skip.Args[0].String(), "@skip(name:)"; got != want {
		t.Errorf("directive argument: got %q, want %q", got, want)
	}

	// The argument the caller built is untouched, so the same one may be
	// written into both without either claiming it.
	if got, want := shared.String(), "name"; got != want {
		t.Errorf("the caller's own argument: got %q, want %q", got, want)
	}

	point := NewInputObject(InputObjectConfig{
		Name:   "Point",
		Fields: []*InputField{NewInputField("x", InputFieldConfig{Type: testScalar("Int")})},
	})
	if got, want := point.Field("x").String(), "Point.x"; got != want {
		t.Errorf("input field: got %q, want %q", got, want)
	}
}

// A half-built argument still prints something rather than panicking.
func TestArgument_StringWhenIncomplete(t *testing.T) {
	if got := NewArgument("a", ArgumentConfig{}).String(); got != "a" {
		t.Errorf("String() with no type = %q, want %q", got, "a")
	}
	var absent *Argument
	if got := absent.String(); got != "<nil>" {
		t.Errorf("String() on nil = %q, want %q", got, "<nil>")
	}
	if got := NewInputField("f", InputFieldConfig{}).String(); got != "f" {
		t.Errorf("String() with no type = %q, want %q", got, "f")
	}
	var absentField *InputField
	if got := absentField.String(); got != "<nil>" {
		t.Errorf("String() on nil = %q, want %q", got, "<nil>")
	}
}
