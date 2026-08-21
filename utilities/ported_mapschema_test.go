package utilities_test

import (
	"github.com/ikawaha/graphql/value"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

// Ported from graphql-js src/utilities/__tests__/mapSchemaConfig-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
//
// graphql-js has one mapper per kind of element, each given that element's
// configuration. [utilities.SchemaMapper] splits the same ground differently:
// a list hook says what a type holds and a configuration hook says what the
// type itself is. The cases below are the upstream ones written in that shape,
// which is how the two are shown to reach the same places.

// expectSchemaMapping builds a schema, maps it, and says what the result
// should print as.
func expectSchemaMapping(t *testing.T, sdl string, mapper utilities.SchemaMapper, want string) {
	t.Helper()
	got := utilities.PrintSchema(utilities.MapSchema(mustBuild(t, sdl), mapper))
	if got != dedent(want) {
		t.Errorf("the mapped schema printed as:\n%s\nwant:\n%s", got, dedent(want))
	}
}

// dedent strips the tabs a test uses to keep its expectations readable. The
// printer indents with spaces, so no indentation of its own is lost.
func dedent(s string) string {
	lines := strings.Split(strings.Trim(s, "\n"), "\n")
	depth := -1
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, "\t")
		if trimmed == "" {
			continue
		}
		if tabs := len(line) - len(trimmed); depth < 0 || tabs < depth {
			depth = tabs
		}
	}
	for i, line := range lines {
		if len(line) >= depth {
			lines[i] = line[depth:]
		} else {
			lines[i] = strings.TrimLeft(line, "\t")
		}
	}
	// No trailing newline, as graphql-js's own dedent leaves none: what a
	// schema prints as is a string, not a file.
	return strings.TrimRight(strings.Join(lines, "\n"), " \t\n")
}

// describedField returns a copy of a field with a description, which is how a
// list hook says what one field of the list becomes.
func describedField(f *schema.Field, description string) *schema.Field {
	return schema.NewField(f.Name(), schema.FieldConfig{
		Description:       value.Just(description),
		Type:              f.Type,
		Args:              f.Args,
		Resolve:           f.Resolve,
		Subscribe:         f.Subscribe,
		DeprecationReason: f.DeprecationReason,
		ASTNode:           f.ASTNode,
		Extensions:        f.Extensions,
	})
}

func describedArgument(a *schema.Argument, description string) *schema.Argument {
	return schema.NewArgument(a.Name(), schema.ArgumentConfig{
		Description:       value.Just(description),
		Type:              a.Type,
		Default:           a.Default,
		DeprecationReason: a.DeprecationReason,
		ASTNode:           a.ASTNode,
		Extensions:        a.Extensions,
	})
}

func describedInputField(f *schema.InputField, description string) *schema.InputField {
	return schema.NewInputField(f.Name(), schema.InputFieldConfig{
		Description:       value.Just(description),
		Type:              f.Type,
		Default:           f.Default,
		DeprecationReason: f.DeprecationReason,
		ASTNode:           f.ASTNode,
		Extensions:        f.Extensions,
	})
}

func describedEnumValue(v *schema.EnumValue, description string) *schema.EnumValue {
	return schema.NewEnumValue(v.Name(), schema.EnumValueConfig{
		Description:       value.Just(description),
		Value:             schema.InternalValue(v.Value),
		DeprecationReason: v.DeprecationReason,
		ASTNode:           v.ASTNode,
		Extensions:        v.Extensions,
	})
}

func TestPortedMapSchema_NoMappers(t *testing.T) {
	expectSchemaMapping(t, "type Query", utilities.SchemaMapper{}, "type Query")
}

func TestPortedMapSchema_Scalar(t *testing.T) {
	expectSchemaMapping(t, "scalar SomeScalar", utilities.SchemaMapper{
		Scalar: func(c schema.ScalarConfig) schema.ScalarConfig {
			c.Description = value.Just("Some description")
			return c
		},
	}, `
		"""Some description"""
		scalar SomeScalar
	`)
}

func TestPortedMapSchema_Arguments(t *testing.T) {
	sdl := `
		type SomeType {
			field(arg: String): String
		}
	`

	t.Run("can map arguments", func(t *testing.T) {
		expectSchemaMapping(t, sdl, utilities.SchemaMapper{
			Arguments: func(_ string, args []*schema.Argument) []*schema.Argument {
				out := make([]*schema.Argument, 0, len(args))
				for _, a := range args {
					out = append(out, describedArgument(a, "Some description"))
				}
				return out
			},
		}, `
			type SomeType {
			  field(
			    """Some description"""
			    arg: String
			  ): String
			}
		`)
	})

	t.Run("names the field and the type an argument belongs to", func(t *testing.T) {
		var visited []string
		utilities.MapSchema(mustBuild(t, sdl), utilities.SchemaMapper{
			Arguments: func(owner string, args []*schema.Argument) []*schema.Argument {
				visited = append(visited, owner)
				return args
			},
		})
		if len(visited) != 1 || visited[0] != "SomeType.field" {
			t.Errorf("the argument hook was told %v, want [SomeType.field]", visited)
		}
	})
}

func TestPortedMapSchema_Fields(t *testing.T) {
	describeFields := utilities.SchemaMapper{
		Fields: func(_ schema.NamedType, fields []*schema.Field) []*schema.Field {
			out := make([]*schema.Field, 0, len(fields))
			for _, f := range fields {
				out = append(out, describedField(f, "Some description"))
			}
			return out
		},
	}

	for _, tt := range []struct {
		name string
		sdl  string
		want string
	}{
		{
			name: "can map fields",
			sdl:  "type SomeType { field: String }",
			want: `
				type SomeType {
				  """Some description"""
				  field: String
				}
			`,
		},
		{
			name: "can map fields with a non-null return type",
			sdl:  "type SomeType { field: String! }",
			want: `
				type SomeType {
				  """Some description"""
				  field: String!
				}
			`,
		},
		{
			name: "can map fields with a list return type",
			sdl:  "type SomeType { field: [String] }",
			want: `
				type SomeType {
				  """Some description"""
				  field: [String]
				}
			`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			expectSchemaMapping(t, tt.sdl, describeFields, tt.want)
		})
	}

	// graphql-js maps a field after its arguments, so its field mapper sees
	// the mapped ones. Here the order is the other way about, because the
	// field hook settles which fields there are and a field it added has its
	// arguments mapped like any other. The place to look at a field once its
	// arguments are mapped is the object hook, which runs after both.
	t.Run("an object sees fields whose arguments were mapped", func(t *testing.T) {
		var seen string
		expectSchemaMapping(t, "type SomeType { field(arg: String): String }", utilities.SchemaMapper{
			Arguments: func(_ string, args []*schema.Argument) []*schema.Argument {
				out := make([]*schema.Argument, 0, len(args))
				for _, a := range args {
					out = append(out, describedArgument(a, "Some argument description"))
				}
				return out
			},
			Fields: func(_ schema.NamedType, fields []*schema.Field) []*schema.Field {
				out := make([]*schema.Field, 0, len(fields))
				for _, f := range fields {
					out = append(out, describedField(f, "Some field description"))
				}
				return out
			},
			Object: func(c schema.ObjectConfig) schema.ObjectConfig {
				fields := c.FieldsThunk()
				seen = fields[0].Args[0].Description()
				return c
			},
		}, `
			type SomeType {
			  """Some field description"""
			  field(
			    """Some argument description"""
			    arg: String
			  ): String
			}
		`)
		if seen != "Some argument description" {
			t.Errorf("the object hook saw the argument described as %q", seen)
		}
	})
}

func TestPortedMapSchema_ObjectType(t *testing.T) {
	t.Run("can map object types", func(t *testing.T) {
		expectSchemaMapping(t, "type SomeType", utilities.SchemaMapper{
			Object: func(c schema.ObjectConfig) schema.ObjectConfig {
				c.Description = value.Just("Some description")
				return c
			},
		}, `
			"""Some description"""
			type SomeType
		`)
	})

	t.Run("maps object types after mapping fields", func(t *testing.T) {
		var seen string
		expectSchemaMapping(t, "type SomeType { field: String }", utilities.SchemaMapper{
			Fields: func(_ schema.NamedType, fields []*schema.Field) []*schema.Field {
				return []*schema.Field{describedField(fields[0], "Some field description")}
			},
			Object: func(c schema.ObjectConfig) schema.ObjectConfig {
				seen = c.FieldsThunk()[0].Description()
				c.Description = value.Just("Some object description")
				return c
			},
		}, `
			"""Some object description"""
			type SomeType {
			  """Some field description"""
			  field: String
			}
		`)
		if seen != "Some field description" {
			t.Errorf("the object hook saw the field described as %q", seen)
		}
	})

	t.Run("maps object types after mapping interfaces", func(t *testing.T) {
		var seen string
		expectSchemaMapping(t, `
			interface SomeInterface
			type SomeType implements SomeInterface
		`, utilities.SchemaMapper{
			Interface: func(c schema.InterfaceConfig) schema.InterfaceConfig {
				c.Description = value.Just("Some interface description")
				return c
			},
			Object: func(c schema.ObjectConfig) schema.ObjectConfig {
				seen = c.InterfacesThunk()[0].Named().Description()
				c.Description = value.Just("Some object description")
				return c
			},
		}, `
			"""Some interface description"""
			interface SomeInterface

			"""Some object description"""
			type SomeType implements SomeInterface
		`)
		if seen != "Some interface description" {
			t.Errorf("the object hook saw the interface described as %q", seen)
		}
	})
}

func TestPortedMapSchema_InterfaceType(t *testing.T) {
	t.Run("can map interface types", func(t *testing.T) {
		expectSchemaMapping(t, "interface SomeInterface", utilities.SchemaMapper{
			Interface: func(c schema.InterfaceConfig) schema.InterfaceConfig {
				c.Description = value.Just("Some description")
				return c
			},
		}, `
			"""Some description"""
			interface SomeInterface
		`)
	})

	t.Run("maps interface types after mapping fields", func(t *testing.T) {
		expectSchemaMapping(t, "interface SomeInterface { field: String }", utilities.SchemaMapper{
			Fields: func(_ schema.NamedType, fields []*schema.Field) []*schema.Field {
				return []*schema.Field{describedField(fields[0], "Some field description")}
			},
			Interface: func(c schema.InterfaceConfig) schema.InterfaceConfig {
				if got := c.FieldsThunk()[0].Description(); got != "Some field description" {
					t.Errorf("the interface hook saw the field described as %q", got)
				}
				c.Description = value.Just("Some interface description")
				return c
			},
		}, `
			"""Some interface description"""
			interface SomeInterface {
			  """Some field description"""
			  field: String
			}
		`)
	})

	t.Run("maps interface types after mapping the interfaces they implement", func(t *testing.T) {
		expectSchemaMapping(t, `
			interface SomeInterface
			interface AnotherInterface implements SomeInterface
		`, utilities.SchemaMapper{
			Interface: func(c schema.InterfaceConfig) schema.InterfaceConfig {
				if c.Name == "AnotherInterface" {
					if got := c.InterfacesThunk()[0].Named().Description(); got != "Some description" {
						t.Errorf("the interface hook saw the parent described as %q", got)
					}
					c.Description = value.Just("Another description")
					return c
				}
				c.Description = value.Just("Some description")
				return c
			},
		}, `
			"""Some description"""
			interface SomeInterface

			"""Another description"""
			interface AnotherInterface implements SomeInterface
		`)
	})
}

func TestPortedMapSchema_UnionType(t *testing.T) {
	t.Run("can map union types", func(t *testing.T) {
		expectSchemaMapping(t, `
			type SomeType
			union SomeUnion = SomeType
		`, utilities.SchemaMapper{
			Union: func(c schema.UnionConfig) schema.UnionConfig {
				c.Description = value.Just("Some description")
				return c
			},
		}, `
			type SomeType

			"""Some description"""
			union SomeUnion = SomeType
		`)
	})

	t.Run("maps union types after mapping their members", func(t *testing.T) {
		expectSchemaMapping(t, `
			type SomeType
			union SomeUnion = SomeType
		`, utilities.SchemaMapper{
			Object: func(c schema.ObjectConfig) schema.ObjectConfig {
				c.Description = value.Just("Some object description")
				return c
			},
			Union: func(c schema.UnionConfig) schema.UnionConfig {
				if got := c.TypesThunk()[0].Named().Description(); got != "Some object description" {
					t.Errorf("the union hook saw its member described as %q", got)
				}
				c.Description = value.Just("Some union description")
				return c
			},
		}, `
			"""Some object description"""
			type SomeType

			"""Some union description"""
			union SomeUnion = SomeType
		`)
	})
}

func TestPortedMapSchema_EnumType(t *testing.T) {
	t.Run("can map enum values", func(t *testing.T) {
		expectSchemaMapping(t, "enum SomeEnum { SOME_VALUE }", utilities.SchemaMapper{
			EnumValues: func(_ schema.NamedType, values []*schema.EnumValue) []*schema.EnumValue {
				return []*schema.EnumValue{describedEnumValue(values[0], "Some value description")}
			},
		}, `
			enum SomeEnum {
			  """Some value description"""
			  SOME_VALUE
			}
		`)
	})

	t.Run("can map enum types", func(t *testing.T) {
		expectSchemaMapping(t, "enum SomeEnum", utilities.SchemaMapper{
			Enum: func(c schema.EnumConfig) schema.EnumConfig {
				c.Description = value.Just("Some description")
				return c
			},
		}, `
			"""Some description"""
			enum SomeEnum
		`)
	})

	t.Run("maps enum types after mapping their values", func(t *testing.T) {
		expectSchemaMapping(t, "enum SomeEnum { SOME_VALUE }", utilities.SchemaMapper{
			EnumValues: func(_ schema.NamedType, values []*schema.EnumValue) []*schema.EnumValue {
				return []*schema.EnumValue{describedEnumValue(values[0], "Some value description")}
			},
			Enum: func(c schema.EnumConfig) schema.EnumConfig {
				if got := c.Values[0].Description(); got != "Some value description" {
					t.Errorf("the enum hook saw its member described as %q", got)
				}
				c.Description = value.Just("Some enum description")
				return c
			},
		}, `
			"""Some enum description"""
			enum SomeEnum {
			  """Some value description"""
			  SOME_VALUE
			}
		`)
	})
}

func TestPortedMapSchema_InputObjectType(t *testing.T) {
	t.Run("can map input fields", func(t *testing.T) {
		expectSchemaMapping(t, "input SomeInput { field: String }", utilities.SchemaMapper{
			InputFields: func(_ schema.NamedType, fields []*schema.InputField) []*schema.InputField {
				return []*schema.InputField{describedInputField(fields[0], "Some field description")}
			},
		}, `
			input SomeInput {
			  """Some field description"""
			  field: String
			}
		`)
	})

	t.Run("can map input object types", func(t *testing.T) {
		expectSchemaMapping(t, "input SomeInput", utilities.SchemaMapper{
			InputObject: func(c schema.InputObjectConfig) schema.InputObjectConfig {
				c.Description = value.Just("Some description")
				return c
			},
		}, `
			"""Some description"""
			input SomeInput
		`)
	})

	t.Run("maps input object types after mapping their fields", func(t *testing.T) {
		expectSchemaMapping(t, "input SomeInput { field: String }", utilities.SchemaMapper{
			InputFields: func(_ schema.NamedType, fields []*schema.InputField) []*schema.InputField {
				return []*schema.InputField{describedInputField(fields[0], "Some field description")}
			},
			InputObject: func(c schema.InputObjectConfig) schema.InputObjectConfig {
				if got := c.FieldsThunk()[0].Description(); got != "Some field description" {
					t.Errorf("the input object hook saw the field described as %q", got)
				}
				c.Description = value.Just("Some input object description")
				return c
			},
		}, `
			"""Some input object description"""
			input SomeInput {
			  """Some field description"""
			  field: String
			}
		`)
	})
}

func TestPortedMapSchema_Directive(t *testing.T) {
	t.Run("can map directives", func(t *testing.T) {
		expectSchemaMapping(t, "directive @someDirective on FIELD", utilities.SchemaMapper{
			Directive: func(c schema.DirectiveConfig) schema.DirectiveConfig {
				c.Description = value.Just("Some description")
				return c
			},
		}, `
			"""Some description"""
			directive @someDirective on FIELD
		`)
	})

	t.Run("maps directives after mapping their arguments", func(t *testing.T) {
		expectSchemaMapping(t, "directive @someDirective(arg: String) on FIELD", utilities.SchemaMapper{
			Arguments: func(owner string, args []*schema.Argument) []*schema.Argument {
				if owner != "@someDirective" {
					t.Errorf("the argument hook was told %q, want @someDirective", owner)
				}
				return []*schema.Argument{describedArgument(args[0], "Some argument description")}
			},
			Directive: func(c schema.DirectiveConfig) schema.DirectiveConfig {
				if got := c.Args[0].Description(); got != "Some argument description" {
					t.Errorf("the directive hook saw the argument described as %q", got)
				}
				c.Description = value.Just("Some directive description")
				return c
			},
		}, `
			"""Some directive description"""
			directive @someDirective(
			  """Some argument description"""
			  arg: String
			) on FIELD
		`)
	})
}

func TestPortedMapSchema_Schema(t *testing.T) {
	t.Run("can map the schema", func(t *testing.T) {
		expectSchemaMapping(t, `
			type Query { field: String }
			type MutationRoot { field: String }
		`, utilities.SchemaMapper{
			Schema: func(c schema.Config) schema.Config {
				c.Description = value.Just("Some description")
				for _, t := range c.Types {
					if named, isObject := t.(*schema.ObjectType); isObject && named.Name() == "MutationRoot" {
						c.Mutation = named
					}
				}
				return c
			},
		}, `
			"""Some description"""
			schema {
			  query: Query
			  mutation: MutationRoot
			}

			type Query {
			  field: String
			}

			type MutationRoot {
			  field: String
			}
		`)
	})

	// The types and the directives a schema hook is given are the new ones,
	// which is what graphql-js hands its schema mapper through getNamedTypes.
	t.Run("maps the schema after mapping types and directives", func(t *testing.T) {
		expectSchemaMapping(t, `
			type Query { field: String }
			directive @someDirective on FIELD
		`, utilities.SchemaMapper{
			Object: func(c schema.ObjectConfig) schema.ObjectConfig {
				c.Description = value.Just("Some object description")
				return c
			},
			Directive: func(c schema.DirectiveConfig) schema.DirectiveConfig {
				c.Description = value.Just("Some directive description")
				return c
			},
			Schema: func(c schema.Config) schema.Config {
				if got := c.Query.Description(); got != "Some object description" {
					t.Errorf("the schema hook saw the query type described as %q", got)
				}
				for _, d := range c.Directives {
					if d.Name() == "someDirective" && d.Description() != "Some directive description" {
						t.Errorf("the schema hook saw the directive described as %q", d.Description())
					}
				}
				c.Description = value.Just("Some schema description")
				return c
			},
		}, `
			"""Some schema description"""
			schema {
			  query: Query
			}

			"""Some directive description"""
			directive @someDirective on FIELD

			"""Some object description"""
			type Query {
			  field: String
			}
		`)
	})

	// graphql-js gives a mapper setNamedType and getNamedTypes so that it can
	// add a type nothing refers to. Here the schema hook holds the list, so
	// adding to it is all there is to do.
	t.Run("a type can be added that nothing refers to", func(t *testing.T) {
		expectSchemaMapping(t, "type Query { field: String }", utilities.SchemaMapper{
			Schema: func(c schema.Config) schema.Config {
				c.Types = append(c.Types, schema.NewObject(schema.ObjectConfig{
					Name:   "SomeType",
					Fields: []*schema.Field{schema.NewField("field", schema.FieldConfig{Type: schema.String})},
				}))
				return c
			},
		}, `
			type Query {
			  field: String
			}

			type SomeType {
			  field: String
			}
		`)
	})
}
