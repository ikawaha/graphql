package schema_test

// Ported from graphql-js src/type/__tests__/extensions-test.ts. There it
// checks that a type's extensions survive toConfig(); here every element of a
// schema is given one and the schema is rebuilt, which is the same question
// asked the way this library asks it.

import (
	"strings"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

// ext names an extension by the element that carries it, so a value found in
// the wrong place is obvious.
func ext(on string) map[string]any { return map[string]any{"on": on} }

// extensionsSchema builds a schema in which every element carries an
// extension.
func extensionsSchema() *schema.Schema {
	dummy := schema.NewScalar(schema.ScalarConfig{
		Name: "DummyScalar", Extensions: ext("scalar"),
	})
	someEnum := schema.NewEnum(schema.EnumConfig{
		Name: "SomeEnum",
		Values: []*schema.EnumValue{
			schema.NewEnumValue("SOME_VALUE", schema.EnumValueConfig{Extensions: ext("enum value")}),
		},
		Extensions: ext("enum"),
	})
	someInput := schema.NewInputObject(schema.InputObjectConfig{
		Name: "SomeInputObject",
		Fields: []*schema.InputField{
			schema.NewInputField("someInputField", schema.InputFieldConfig{
				Type: dummy, Extensions: ext("input field"),
			}),
		},
		Extensions: ext("input object"),
	})
	someInterface := schema.NewInterface(schema.InterfaceConfig{
		Name: "SomeInterface",
		Fields: []*schema.Field{
			schema.NewField("someField", schema.FieldConfig{
				Type: dummy,
				Args: []*schema.Argument{
					schema.NewArgument("someArg", schema.ArgumentConfig{
						Type: dummy, Extensions: ext("interface field argument"),
					}),
				},
				Extensions: ext("interface field"),
			}),
		},
		Extensions: ext("interface"),
	})
	someObject := schema.NewObject(schema.ObjectConfig{
		Name:       "SomeObject",
		Interfaces: schema.Implements(someInterface),
		Fields: []*schema.Field{
			schema.NewField("someField", schema.FieldConfig{
				Type: dummy,
				Args: []*schema.Argument{
					schema.NewArgument("someArg", schema.ArgumentConfig{
						Type: dummy, Extensions: ext("field argument"),
					}),
				},
				Extensions: ext("field"),
			}),
		},
		Extensions: ext("object"),
	})
	someUnion := schema.NewUnion(schema.UnionConfig{
		Name: "SomeUnion", Types: schema.Members(someObject),
		Extensions: ext("union"),
	})
	query := schema.NewObject(schema.ObjectConfig{
		Name: "Query",
		Fields: []*schema.Field{
			schema.NewField("object", schema.FieldConfig{Type: someObject}),
			schema.NewField("iface", schema.FieldConfig{Type: someInterface}),
			schema.NewField("union", schema.FieldConfig{Type: someUnion}),
			schema.NewField("enum", schema.FieldConfig{Type: someEnum}),
			schema.NewField("input", schema.FieldConfig{
				Type: dummy,
				Args: []*schema.Argument{
					schema.NewArgument("in", schema.ArgumentConfig{Type: someInput}),
				},
			}),
		},
	})
	return schema.New(schema.Config{
		Query: query,
		Directives: append(append([]*schema.Directive{}, schema.SpecifiedDirectives...), schema.NewDirective(schema.DirectiveConfig{
			Name:      "someDirective",
			Locations: []language.DirectiveLocation{language.DirectiveLocationField},
			Args: []*schema.Argument{
				schema.NewArgument("someArg", schema.ArgumentConfig{
					Type: dummy, Extensions: ext("directive argument"),
				}),
			},
			Extensions: ext("directive"),
		})),
		Extensions: ext("schema"),
	})
}

// found collects every extension a schema carries, keyed by where it was
// found, so the whole set can be compared at once.
func found(t *testing.T, s *schema.Schema) map[string]string {
	t.Helper()
	out := map[string]string{}
	note := func(where string, extensions map[string]any) {
		if on, carried := extensions["on"]; carried {
			out[where] = on.(string)
		}
	}
	note("schema", s.Extensions)
	for _, d := range s.Directives() {
		if d.Name() != "someDirective" {
			continue
		}
		note("@someDirective", d.Extensions)
		for _, a := range d.Args {
			note("@someDirective("+a.Name()+":)", a.Extensions)
		}
	}
	for _, named := range s.Types() {
		switch t2 := named.(type) {
		case *schema.ScalarType:
			note(t2.Name(), t2.Extensions)
		case *schema.EnumType:
			note(t2.Name(), t2.Extensions)
			for _, v := range t2.Values() {
				note(t2.Name()+"."+v.Name(), v.Extensions)
			}
		case *schema.UnionType:
			note(t2.Name(), t2.Extensions)
		case *schema.InputObjectType:
			note(t2.Name(), t2.Extensions)
			for _, f := range t2.Fields() {
				note(t2.Name()+"."+f.Name(), f.Extensions)
			}
		case *schema.ObjectType:
			note(t2.Name(), t2.Extensions)
			for _, f := range t2.Fields() {
				note(t2.Name()+"."+f.Name(), f.Extensions)
				for _, a := range f.Args {
					note(t2.Name()+"."+f.Name()+"("+a.Name()+":)", a.Extensions)
				}
			}
		case *schema.InterfaceType:
			note(t2.Name(), t2.Extensions)
			for _, f := range t2.Fields() {
				note(t2.Name()+"."+f.Name(), f.Extensions)
				for _, a := range f.Args {
					note(t2.Name()+"."+f.Name()+"("+a.Name()+":)", a.Extensions)
				}
			}
		}
	}
	return out
}

var wantExtensions = map[string]string{
	"schema":                            "schema",
	"@someDirective":                    "directive",
	"@someDirective(someArg:)":          "directive argument",
	"DummyScalar":                       "scalar",
	"SomeEnum":                          "enum",
	"SomeEnum.SOME_VALUE":               "enum value",
	"SomeInputObject":                   "input object",
	"SomeInputObject.someInputField":    "input field",
	"SomeInterface":                     "interface",
	"SomeInterface.someField":           "interface field",
	"SomeInterface.someField(someArg:)": "interface field argument",
	"SomeObject":                        "object",
	"SomeObject.someField":              "field",
	"SomeObject.someField(someArg:)":    "field argument",
	"SomeUnion":                         "union",
}

func compareExtensions(t *testing.T, got map[string]string) {
	t.Helper()
	for where, want := range wantExtensions {
		if got[where] != want {
			t.Errorf("%s carries %q, want %q", where, got[where], want)
		}
	}
	for where, on := range got {
		if _, expected := wantExtensions[where]; !expected {
			t.Errorf("%s carries %q, which nothing put there", where, on)
		}
	}
}

// An element with no extensions has none, rather than an empty map standing in
// for one.
func TestPortedExtensions_AbsentWhenNotGiven(t *testing.T) {
	s := schema.New(schema.Config{
		Query: schema.NewObject(schema.ObjectConfig{
			Name: "Query",
			Fields: []*schema.Field{
				schema.NewField("f", schema.FieldConfig{Type: schema.String}),
			},
		}),
	})
	if s.Extensions != nil {
		t.Errorf("the schema carries %v, want nothing", s.Extensions)
	}
	if got := found(t, s); len(got) != 0 {
		t.Errorf("extensions were found where none were given: %v", got)
	}
}

func TestPortedExtensions_EveryElementCarriesThem(t *testing.T) {
	compareExtensions(t, found(t, extensionsSchema()))
}

// Extending a schema rebuilds every type in it, so this is the question
// graphql-js asks of toConfig: does what was put on an element come back out.
func TestPortedExtensions_SurviveExtending(t *testing.T) {
	extended, err := utilities.ExtendSchemaSource(extensionsSchema(),
		"extend type SomeObject { anotherField: DummyScalar }")
	if err != nil {
		t.Fatalf("extending: %v", err)
	}
	compareExtensions(t, found(t, extended))
}

func TestPortedExtensions_SurviveSorting(t *testing.T) {
	compareExtensions(t, found(t, utilities.LexicographicSortSchema(extensionsSchema())))
}

// TestPortedDeprecation_ThreeStates is what graphql-js keeps in
// `deprecationReason?: Maybe<string>` and reports as `deprecationReason != null`.
// Three answers are possible and they differ: nothing said, deprecated with a
// reason, and deprecated with none.
//
// The reason a schema writes is a String!, so it cannot be null; that case is
// TestDeprecation_ReasonCannotBeNull below. Only a schema built in Go can hold
// a deprecation with nothing in it, which is what NotDeprecated says.
func TestPortedDeprecation_ThreeStates(t *testing.T) {
	s, err := utilities.BuildSchema(`
		type Query {
			plain: String
			bare: String @deprecated
			empty: String @deprecated(reason: "")
			given: String @deprecated(reason: "Gone")
		}
	`)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	tests := []struct {
		field      string
		deprecated bool
		reason     string
	}{
		{"plain", false, ""},
		{"bare", true, schema.DefaultDeprecationReason},
		{"empty", true, ""},
		{"given", true, "Gone"},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			f := s.QueryType().Field(tt.field)
			if f.IsDeprecated() != tt.deprecated {
				t.Errorf("IsDeprecated() = %v, want %v", f.IsDeprecated(), tt.deprecated)
			}
			said, deprecated := f.DeprecationReason.Get()
			if deprecated != tt.deprecated {
				t.Errorf("the reason is %v there, want %v", deprecated, tt.deprecated)
			}
			if deprecated && said != tt.reason {
				t.Errorf("reason = %q, want %q", said, tt.reason)
			}
		})
	}

	// What is printed back out says the same three things.
	want := "type Query {\n  plain: String\n  bare: String @deprecated\n" +
		"  empty: String @deprecated(reason: \"\")\n" +
		"  given: String @deprecated(reason: \"Gone\")\n}"
	if got := strings.TrimSpace(utilities.PrintSchema(s)); got != want {
		t.Errorf("printed\n%s\nwant\n%s", got, want)
	}
}

// TestDeprecation_ReasonCannotBeNull is the other half: @deprecated declares
// its reason as String!, so writing null there is not a smaller answer but an
// invalid one. graphql-js refuses to build the schema, from the same coercion,
// and neither implementation reports it during SDL validation — the rule that
// would, ValuesOfCorrectType, is not one of the SDL rules.
func TestDeprecation_ReasonCannotBeNull(t *testing.T) {
	// The wording is graphql-js's, taken from running it: what is wrong with
	// the value is whatever the argument's own type says, so a null and a
	// number are refused differently.
	tests := []struct{ written, says string }{
		{"null", `Expected value of non-null type "String!" not to be null.`},
		{"123", "String cannot represent a non string value: 123"},
		{"true", "String cannot represent a non string value: true"},
	}
	for _, tt := range tests {
		t.Run(tt.written, func(t *testing.T) {
			_, err := utilities.BuildSchema(
				"type Query { a: String @deprecated(reason: " + tt.written + ") }")
			if err == nil {
				t.Fatalf("a reason written as %s was accepted", tt.written)
			}
			want := `argument "@deprecated(reason:)" has invalid value: ` + tt.says
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refused with\n %v\nwant it to say\n %s", err, want)
			}
		})
	}
}
