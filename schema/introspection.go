package schema

import (
	"context"
	"fmt"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/value"
)

// The introspection schema is what lets a client ask a server what it can do.
// Every schema carries these types, and the three meta-fields at the bottom of
// this file are how a document reaches them.
//
// The types refer to one another in a loop, most obviously __Type.ofType,
// which is a __Type. They are therefore built with deferred field lists, the
// same mechanism a user's own recursive types use.

// The introspection types refer to one another in a loop, and Go counts a
// reference inside a function literal when it works out the order to
// initialise package variables in. Deferring the field lists is therefore not
// enough on its own: the variables are declared here and built in init, which
// is what breaks the cycle.
var (
	SchemaIntrospectionType     *ObjectType
	DirectiveIntrospectionType  *ObjectType
	TypeIntrospectionType       *ObjectType
	FieldIntrospectionType      *ObjectType
	InputValueIntrospectionType *ObjectType
	EnumValueIntrospectionType  *ObjectType

	// IntrospectionTypes are the types every schema carries so that it can
	// describe itself.
	IntrospectionTypes []NamedType

	// SchemaMetaField is the __schema field of the query root.
	SchemaMetaField *Field
	// TypeMetaField is the __type field of the query root.
	TypeMetaField *Field
	// TypeNameMetaField is the __typename field, which every composite type has.
	TypeNameMetaField *Field
)

func init() {
	SchemaIntrospectionType = buildSchemaIntrospectionType()
	DirectiveIntrospectionType = buildDirectiveIntrospectionType()
	TypeIntrospectionType = buildTypeIntrospectionType()
	FieldIntrospectionType = buildFieldIntrospectionType()
	InputValueIntrospectionType = buildInputValueIntrospectionType()
	EnumValueIntrospectionType = buildEnumValueIntrospectionType()

	IntrospectionTypes = buildIntrospectionTypes()

	SchemaMetaField = buildSchemaMetaField()
	TypeMetaField = buildTypeMetaField()
	TypeNameMetaField = buildTypeNameMetaField()
}

// TypeKindName is the name of a kind of type, as __TypeKind spells it.
type TypeKindName string

// The kinds a type can be.
const (
	TypeKindScalar      TypeKindName = "SCALAR"
	TypeKindObject      TypeKindName = "OBJECT"
	TypeKindInterface   TypeKindName = "INTERFACE"
	TypeKindUnion       TypeKindName = "UNION"
	TypeKindEnum        TypeKindName = "ENUM"
	TypeKindInputObject TypeKindName = "INPUT_OBJECT"
	TypeKindList        TypeKindName = "LIST"
	TypeKindNonNull     TypeKindName = "NON_NULL"
)

// KindOf reports which kind a type is.
func KindOf(t Type) TypeKindName {
	switch t.(type) {
	case *ScalarType:
		return TypeKindScalar
	case *ObjectType:
		return TypeKindObject
	case *InterfaceType:
		return TypeKindInterface
	case *UnionType:
		return TypeKindUnion
	case *EnumType:
		return TypeKindEnum
	case *InputObjectType:
		return TypeKindInputObject
	case *List:
		return TypeKindList
	case *NonNull:
		return TypeKindNonNull
	default:
		return ""
	}
}

// TypeKindType is __TypeKind, the enum naming what kind a type is.
var TypeKindType = NewEnum(EnumConfig{
	Name:        "__TypeKind",
	Description: value.Just("An enum describing what kind of type a given `__Type` is."),
	Values: []*EnumValue{
		newIntrospectionEnumValue(TypeKindScalar, "Indicates this type is a scalar."),
		newIntrospectionEnumValue(TypeKindObject, "Indicates this type is an object. `fields` and `interfaces` are valid fields."),
		newIntrospectionEnumValue(TypeKindInterface, "Indicates this type is an interface. `fields`, `interfaces`, "+
			"and `possibleTypes` are valid fields."),
		newIntrospectionEnumValue(TypeKindUnion, "Indicates this type is a union. `possibleTypes` is a valid field."),
		newIntrospectionEnumValue(TypeKindEnum, "Indicates this type is an enum. `enumValues` is a valid field."),
		newIntrospectionEnumValue(TypeKindInputObject, "Indicates this type is an input object. `inputFields` is a valid field."),
		newIntrospectionEnumValue(TypeKindList, "Indicates this type is a list. `ofType` is a valid field."),
		newIntrospectionEnumValue(TypeKindNonNull, "Indicates this type is a non-null. `ofType` is a valid field."),
	},
})

func newIntrospectionEnumValue(kind TypeKindName, description string) *EnumValue {
	return NewEnumValue(string(kind), EnumValueConfig{
		Value:       InternalValue(kind),
		Description: value.Just(description),
	})
}

// DirectiveLocationType is __DirectiveLocation, the enum naming where a
// directive may be applied.
var DirectiveLocationType = NewEnum(EnumConfig{
	Name: "__DirectiveLocation",
	Description: value.Just("A Directive can be adjacent to many parts of the GraphQL " +
		"language, a __DirectiveLocation describes one such possible " +
		"adjacencies."),
	Values: directiveLocationEnumValues(),
})

func directiveLocationEnumValues() []*EnumValue {
	locations := []language.DirectiveLocation{
		language.DirectiveLocationQuery,
		language.DirectiveLocationMutation,
		language.DirectiveLocationSubscription,
		language.DirectiveLocationField,
		language.DirectiveLocationFragmentDefinition,
		language.DirectiveLocationFragmentSpread,
		language.DirectiveLocationInlineFragment,
		language.DirectiveLocationVariableDefinition,
		language.DirectiveLocationFragmentVariableDefinition,
		language.DirectiveLocationSchema,
		language.DirectiveLocationScalar,
		language.DirectiveLocationObject,
		language.DirectiveLocationFieldDefinition,
		language.DirectiveLocationArgumentDefinition,
		language.DirectiveLocationInterface,
		language.DirectiveLocationUnion,
		language.DirectiveLocationEnum,
		language.DirectiveLocationEnumValue,
		language.DirectiveLocationInputObject,
		language.DirectiveLocationInputFieldDefinition,
		language.DirectiveLocationDirectiveDefinition,
	}
	values := make([]*EnumValue, len(locations))
	for i, loc := range locations {
		values[i] = NewEnumValue(string(loc), EnumValueConfig{
			Value:       InternalValue(loc),
			Description: value.Just(directiveLocationDescriptions[loc]),
		})
	}
	return values
}

// directiveLocationDescriptions says what each place a directive may be
// written is, in graphql-js's own words: a client reads these out of
// introspection, so they are part of what the two implementations must agree
// about.
var directiveLocationDescriptions = map[language.DirectiveLocation]string{
	language.DirectiveLocationArgumentDefinition:         "Location adjacent to an argument definition.",
	language.DirectiveLocationDirectiveDefinition:        "Location adjacent to a directive definition.",
	language.DirectiveLocationEnum:                       "Location adjacent to an enum definition.",
	language.DirectiveLocationEnumValue:                  "Location adjacent to an enum value definition.",
	language.DirectiveLocationField:                      "Location adjacent to a field.",
	language.DirectiveLocationFieldDefinition:            "Location adjacent to a field definition.",
	language.DirectiveLocationFragmentDefinition:         "Location adjacent to a fragment definition.",
	language.DirectiveLocationFragmentSpread:             "Location adjacent to a fragment spread.",
	language.DirectiveLocationFragmentVariableDefinition: "Location adjacent to a fragment variable definition.",
	language.DirectiveLocationInlineFragment:             "Location adjacent to an inline fragment.",
	language.DirectiveLocationInputFieldDefinition:       "Location adjacent to an input object field definition.",
	language.DirectiveLocationInputObject:                "Location adjacent to an input object type definition.",
	language.DirectiveLocationInterface:                  "Location adjacent to an interface definition.",
	language.DirectiveLocationMutation:                   "Location adjacent to a mutation operation.",
	language.DirectiveLocationObject:                     "Location adjacent to an object type definition.",
	language.DirectiveLocationQuery:                      "Location adjacent to a query operation.",
	language.DirectiveLocationScalar:                     "Location adjacent to a scalar definition.",
	language.DirectiveLocationSchema:                     "Location adjacent to a schema definition.",
	language.DirectiveLocationSubscription:               "Location adjacent to a subscription operation.",
	language.DirectiveLocationUnion:                      "Location adjacent to a union definition.",
	language.DirectiveLocationVariableDefinition:         "Location adjacent to an operation variable definition.",
}

// includeDeprecatedArg is the argument that decides whether deprecated members
// are listed. It defaults to false, so a client that says nothing sees only
// what is current.
func includeDeprecatedArg() *Argument {
	return NewArgument("includeDeprecated", ArgumentConfig{
		Type:    NewNonNull(Boolean),
		Default: DefaultValue(false),
	})
}

// wantsDeprecated reads that argument.
func wantsDeprecated(args Arguments) bool {
	v, ok := args.Get("includeDeprecated")
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// SchemaIntrospectionType is __Schema.
func buildSchemaIntrospectionType() *ObjectType {
	return NewObject(ObjectConfig{
		Name: "__Schema",
		Description: value.Just("A GraphQL Schema defines the capabilities of a GraphQL server. " +
			"It exposes all available types and directives on the server, as well as " +
			"the entry points for query, mutation, and subscription operations."),
		FieldsThunk: func() []*Field {
			return []*Field{
				NewField("description", FieldConfig{
					Type: String,
					Resolve: resolveSchema(func(_ context.Context, s *Schema, _ Arguments) (any, error) {
						return describedOrNil(s.DescribedAs()), nil
					}),
				}),
				NewField("types", FieldConfig{
					Type:        NewNonNull(NewList(NewNonNull(TypeIntrospectionType))),
					Description: value.Just("A list of all types supported by this server."),
					Resolve: resolveSchema(func(_ context.Context, s *Schema, _ Arguments) (any, error) {
						return anySlice(s.Types()), nil
					}),
				}),
				NewField("queryType", FieldConfig{
					Type:        NewNonNull(TypeIntrospectionType),
					Description: value.Just("The type that query operations will be rooted at."),
					Resolve: resolveSchema(func(_ context.Context, s *Schema, _ Arguments) (any, error) {
						return nilIfAbsentType(s.DeclaredRootType(language.OperationQuery)), nil
					}),
				}),
				NewField("mutationType", FieldConfig{
					Type:        TypeIntrospectionType,
					Description: value.Just("If this server supports mutation, the type that mutation operations will be rooted at."),
					Resolve: resolveSchema(func(_ context.Context, s *Schema, _ Arguments) (any, error) {
						return nilIfAbsentType(s.DeclaredRootType(language.OperationMutation)), nil
					}),
				}),
				NewField("subscriptionType", FieldConfig{
					Type:        TypeIntrospectionType,
					Description: value.Just("If this server support subscription, the type that subscription operations will be rooted at."),
					Resolve: resolveSchema(func(_ context.Context, s *Schema, _ Arguments) (any, error) {
						return nilIfAbsentType(s.DeclaredRootType(language.OperationSubscription)), nil
					}),
				}),
				NewField("directives", FieldConfig{
					Type:        NewNonNull(NewList(NewNonNull(DirectiveIntrospectionType))),
					Description: value.Just("A list of all directives supported by this server."),
					Args:        []*Argument{includeDeprecatedArg()},
					Resolve: resolveSchema(func(_ context.Context, s *Schema, args Arguments) (any, error) {
						return anySlice(filterDeprecatedDirectives(s.Directives(), wantsDeprecated(args))), nil
					}),
				}),
			}
		},
	})
}

// DirectiveIntrospectionType is __Directive.
func buildDirectiveIntrospectionType() *ObjectType {
	return NewObject(ObjectConfig{
		Name: "__Directive",
		Description: value.Just("A Directive provides a way to describe alternate runtime " +
			"execution and type validation behavior in a GraphQL " +
			"document.\n\nIn some cases, you need to provide options to " +
			"alter GraphQL's execution behavior in ways field arguments " +
			"will not suffice, such as conditionally including or " +
			"skipping a field. Directives provide this by describing " +
			"additional information to the executor."),
		FieldsThunk: func() []*Field {
			return []*Field{
				NewField("name", FieldConfig{
					Type:    NewNonNull(String),
					Resolve: resolveDirective(func(d *Directive, _ Arguments) (any, error) { return d.Name(), nil }),
				}),
				NewField("description", FieldConfig{
					Type:    String,
					Resolve: resolveDirective(func(d *Directive, _ Arguments) (any, error) { return describedOrNil(d.DescribedAs()), nil }),
				}),
				NewField("isRepeatable", FieldConfig{
					Type:    NewNonNull(Boolean),
					Resolve: resolveDirective(func(d *Directive, _ Arguments) (any, error) { return d.IsRepeatable, nil }),
				}),
				NewField("locations", FieldConfig{
					Type: NewNonNull(NewList(NewNonNull(DirectiveLocationType))),
					Resolve: resolveDirective(func(d *Directive, _ Arguments) (any, error) {
						return anySlice(d.Locations), nil
					}),
				}),
				NewField("args", FieldConfig{
					Type: NewNonNull(NewList(NewNonNull(InputValueIntrospectionType))),
					Args: []*Argument{includeDeprecatedArg()},
					Resolve: resolveDirective(func(d *Directive, args Arguments) (any, error) {
						return anySlice(filterDeprecatedArgs(d.Args, wantsDeprecated(args))), nil
					}),
				}),
				NewField("isDeprecated", FieldConfig{
					Type: NewNonNull(Boolean),
					Resolve: resolveDirective(func(d *Directive, _ Arguments) (any, error) {
						return d.IsDeprecated(), nil
					}),
				}),
				NewField("deprecationReason", FieldConfig{
					Type: String,
					Resolve: resolveDirective(func(d *Directive, _ Arguments) (any, error) {
						return reasonOrNil(d.DeprecationReason), nil
					}),
				}),
			}
		},
	})
}

// TypeIntrospectionType is __Type.
//
// Most of its fields only make sense for some kinds of type; the rest report
// null. Which fields apply to which kind is what __TypeKind says.
func buildTypeIntrospectionType() *ObjectType {
	return NewObject(ObjectConfig{
		Name: "__Type",
		Description: value.Just("The fundamental unit of any GraphQL Schema is the type. " +
			"There are many kinds of types in GraphQL as represented by " +
			"the `__TypeKind` enum.\n\nDepending on the kind of a type, " +
			"certain fields describe information about that type. Scalar " +
			"types provide no information beyond a name, description and " +
			"optional `specifiedByURL`, while Enum types provide their " +
			"values. Object and Interface types provide the fields they " +
			"describe. Abstract types, Union and Interface, provide the " +
			"Object types possible at runtime. List and NonNull types " +
			"compose other types."),
		FieldsThunk: func() []*Field {
			return []*Field{
				NewField("kind", FieldConfig{
					Type: NewNonNull(TypeKindType),
					Resolve: resolveType(func(t Type, _ Arguments) (any, error) {
						kind := KindOf(t)
						if kind == "" {
							return nil, errUnexpectedType(t)
						}
						return kind, nil
					}),
				}),
				NewField("name", FieldConfig{
					Type: String,
					Resolve: resolveType(func(t Type, _ Arguments) (any, error) {
						if named, ok := t.(NamedType); ok {
							return named.Name(), nil
						}
						return nil, nil
					}),
				}),
				NewField("description", FieldConfig{
					Type: String,
					Resolve: resolveType(func(t Type, _ Arguments) (any, error) {
						return describedOrNil(describeType(t)), nil
					}),
				}),
				NewField("specifiedByURL", FieldConfig{
					Type: String,
					Resolve: resolveType(func(t Type, _ Arguments) (any, error) {
						if s, ok := t.(*ScalarType); ok {
							return emptyToNil(s.SpecifiedByURL), nil
						}
						return nil, nil
					}),
				}),
				NewField("fields", FieldConfig{
					Type: NewList(NewNonNull(FieldIntrospectionType)),
					Args: []*Argument{includeDeprecatedArg()},
					Resolve: resolveType(func(t Type, args Arguments) (any, error) {
						var fields []*Field
						switch n := t.(type) {
						case *ObjectType:
							fields = n.Fields()
						case *InterfaceType:
							fields = n.Fields()
						default:
							return nil, nil
						}
						return anySlice(filterDeprecatedFields(fields, wantsDeprecated(args))), nil
					}),
				}),
				NewField("interfaces", FieldConfig{
					Type: NewList(NewNonNull(TypeIntrospectionType)),
					Resolve: resolveType(func(t Type, _ Arguments) (any, error) {
						// What the clause named, of the right kind or not: a
						// client asking a broken schema what it is must be
						// told what it says, as graphql-js tells them.
						switch n := t.(type) {
						case *ObjectType:
							return anySlice(namedTypes(n.Interfaces())), nil
						case *InterfaceType:
							return anySlice(namedTypes(n.Interfaces())), nil
						}
						return nil, nil
					}),
				}),
				NewField("possibleTypes", FieldConfig{
					Type: NewList(NewNonNull(TypeIntrospectionType)),
					Resolve: func(_ context.Context, source any, _ Arguments, info *ResolveInfo) (any, error) {
						abstract, ok := source.(AbstractType)
						if !ok {
							return nil, nil
						}
						if info == nil || info.Schema == nil {
							return nil, nil
						}
						// A union answers with what it named, whatever kind
						// each turned out to be, as graphql-js's
						// getPossibleTypes does; an interface answers with
						// what implements it.
						if union, isUnion := abstract.(*UnionType); isUnion {
							return anySlice(namedTypes(union.Types())), nil
						}
						return anySlice(info.Schema.PossibleTypes(abstract)), nil
					},
				}),
				NewField("enumValues", FieldConfig{
					Type: NewList(NewNonNull(EnumValueIntrospectionType)),
					Args: []*Argument{includeDeprecatedArg()},
					Resolve: resolveType(func(t Type, args Arguments) (any, error) {
						e, ok := t.(*EnumType)
						if !ok {
							return nil, nil
						}
						return anySlice(filterDeprecatedEnumValues(e.Values(), wantsDeprecated(args))), nil
					}),
				}),
				NewField("inputFields", FieldConfig{
					Type: NewList(NewNonNull(InputValueIntrospectionType)),
					Args: []*Argument{includeDeprecatedArg()},
					Resolve: resolveType(func(t Type, args Arguments) (any, error) {
						in, ok := t.(*InputObjectType)
						if !ok {
							return nil, nil
						}
						return anySlice(filterDeprecatedInputFields(in.Fields(), wantsDeprecated(args))), nil
					}),
				}),
				NewField("ofType", FieldConfig{
					Type: TypeIntrospectionType,
					Resolve: resolveType(func(t Type, _ Arguments) (any, error) {
						if w, ok := t.(WrappingType); ok {
							return w.Unwrap(), nil
						}
						return nil, nil
					}),
				}),
				NewField("isOneOf", FieldConfig{
					Type: Boolean,
					Resolve: resolveType(func(t Type, _ Arguments) (any, error) {
						if in, ok := t.(*InputObjectType); ok {
							return in.IsOneOf, nil
						}
						return nil, nil
					}),
				}),
			}
		},
	})
}

// FieldIntrospectionType is __Field.
func buildFieldIntrospectionType() *ObjectType {
	return NewObject(ObjectConfig{
		Name: "__Field",
		Description: value.Just("Object and Interface types are described by a list of Fields, each of " +
			"which has a name, potentially a list of arguments, and a return type."),
		FieldsThunk: func() []*Field {
			return []*Field{
				NewField("name", FieldConfig{
					Type:    NewNonNull(String),
					Resolve: resolveField(func(f *Field, _ Arguments) (any, error) { return f.Name(), nil }),
				}),
				NewField("description", FieldConfig{
					Type:    String,
					Resolve: resolveField(func(f *Field, _ Arguments) (any, error) { return describedOrNil(f.DescribedAs()), nil }),
				}),
				NewField("args", FieldConfig{
					Type: NewNonNull(NewList(NewNonNull(InputValueIntrospectionType))),
					Args: []*Argument{includeDeprecatedArg()},
					Resolve: resolveField(func(f *Field, args Arguments) (any, error) {
						return anySlice(filterDeprecatedArgs(f.Args, wantsDeprecated(args))), nil
					}),
				}),
				NewField("type", FieldConfig{
					Type:    NewNonNull(TypeIntrospectionType),
					Resolve: resolveField(func(f *Field, _ Arguments) (any, error) { return f.Type, nil }),
				}),
				NewField("isDeprecated", FieldConfig{
					Type:    NewNonNull(Boolean),
					Resolve: resolveField(func(f *Field, _ Arguments) (any, error) { return f.IsDeprecated(), nil }),
				}),
				NewField("deprecationReason", FieldConfig{
					Type:    String,
					Resolve: resolveField(func(f *Field, _ Arguments) (any, error) { return reasonOrNil(f.DeprecationReason), nil }),
				}),
			}
		},
	})
}

// InputValueIntrospectionType is __InputValue, which describes both an
// argument and a field of an input object, since the two have the same shape.
func buildInputValueIntrospectionType() *ObjectType {
	return NewObject(ObjectConfig{
		Name: "__InputValue",
		Description: value.Just("Arguments provided to Fields or Directives and the input fields of an " +
			"InputObject are represented as Input Values which describe their type and " +
			"optionally a default value."),
		FieldsThunk: func() []*Field {
			return []*Field{
				NewField("name", FieldConfig{
					Type:    NewNonNull(String),
					Resolve: resolveInputValue(func(v inputValueSource) (any, error) { return v.name, nil }),
				}),
				NewField("description", FieldConfig{
					Type:    String,
					Resolve: resolveInputValue(func(v inputValueSource) (any, error) { return describedOrNil(v.description), nil }),
				}),
				NewField("type", FieldConfig{
					Type:    NewNonNull(TypeIntrospectionType),
					Resolve: resolveInputValue(func(v inputValueSource) (any, error) { return v.typ, nil }),
				}),
				NewField("defaultValue", FieldConfig{
					Type:        String,
					Description: value.Just("A GraphQL-formatted string representing the default value for this input value."),
					Resolve: resolveInputValue(func(v inputValueSource) (any, error) {
						return v.defaultLiteral(), nil
					}),
				}),
				NewField("isDeprecated", FieldConfig{
					Type:    NewNonNull(Boolean),
					Resolve: resolveInputValue(func(v inputValueSource) (any, error) { return v.deprecationReason.IsSet(), nil }),
				}),
				NewField("deprecationReason", FieldConfig{
					Type:    String,
					Resolve: resolveInputValue(func(v inputValueSource) (any, error) { return reasonOrNil(v.deprecationReason), nil }),
				}),
			}
		},
	})
}

// EnumValueIntrospectionType is __EnumValue.
func buildEnumValueIntrospectionType() *ObjectType {
	return NewObject(ObjectConfig{
		Name: "__EnumValue",
		Description: value.Just("One possible value for a given Enum. Enum values are unique " +
			"values, not a placeholder for a string or numeric value. " +
			"However an Enum value is returned in a JSON response as a " +
			"string."),
		FieldsThunk: func() []*Field {
			return []*Field{
				NewField("name", FieldConfig{
					Type:    NewNonNull(String),
					Resolve: resolveEnumValue(func(v *EnumValue) (any, error) { return v.Name(), nil }),
				}),
				NewField("description", FieldConfig{
					Type:    String,
					Resolve: resolveEnumValue(func(v *EnumValue) (any, error) { return describedOrNil(v.DescribedAs()), nil }),
				}),
				NewField("isDeprecated", FieldConfig{
					Type:    NewNonNull(Boolean),
					Resolve: resolveEnumValue(func(v *EnumValue) (any, error) { return v.IsDeprecated(), nil }),
				}),
				NewField("deprecationReason", FieldConfig{
					Type:    String,
					Resolve: resolveEnumValue(func(v *EnumValue) (any, error) { return reasonOrNil(v.DeprecationReason), nil }),
				}),
			}
		},
	})
}

// IntrospectionTypes are the types every schema carries so that it can
// describe itself.
func buildIntrospectionTypes() []NamedType {
	return []NamedType{
		SchemaIntrospectionType,
		DirectiveIntrospectionType,
		DirectiveLocationType,
		TypeIntrospectionType,
		FieldIntrospectionType,
		InputValueIntrospectionType,
		EnumValueIntrospectionType,
		TypeKindType,
	}
}

// IsIntrospectionType reports whether a type is one of those.
func IsIntrospectionType(t Type) bool {
	named, ok := t.(NamedType)
	if !ok {
		return false
	}
	for _, i := range IntrospectionTypes {
		if i == named {
			return true
		}
	}
	return false
}

// The three meta-fields are how a document reaches the introspection types.
// They belong to no type in particular: __schema and __type may be asked of
// the query root, and __typename of any composite type.

// SchemaMetaField is the __schema field of the query root.
func buildSchemaMetaField() *Field {
	return NewField("__schema", FieldConfig{
		Type:        NewNonNull(SchemaIntrospectionType),
		Description: value.Just("Access the current type schema of this server."),
		Resolve: func(_ context.Context, _ any, _ Arguments, info *ResolveInfo) (any, error) {
			if info == nil {
				return nil, nil
			}
			return info.Schema, nil
		},
	})
}

// TypeMetaField is the __type field of the query root.
func buildTypeMetaField() *Field {
	return NewField("__type", FieldConfig{
		Type:        TypeIntrospectionType,
		Description: value.Just("Request the type information of a single type."),
		Args:        []*Argument{NewArgument("name", ArgumentConfig{Type: NewNonNull(String)})},
		Resolve: func(_ context.Context, _ any, args Arguments, info *ResolveInfo) (any, error) {
			if info == nil || info.Schema == nil {
				return nil, nil
			}
			name, _ := args.Get("name")
			text, _ := name.(string)
			found := info.Schema.Type(text)
			if found == nil {
				return nil, nil
			}
			return found, nil
		},
	})
}

// TypeNameMetaField is the __typename field, which every composite type has.
func buildTypeNameMetaField() *Field {
	return NewField("__typename", FieldConfig{
		Type:        NewNonNull(String),
		Description: value.Just("The name of the current Object type at runtime."),
		Resolve: func(_ context.Context, _ any, _ Arguments, info *ResolveInfo) (any, error) {
			if info == nil || info.ParentType == nil {
				return nil, nil
			}
			return info.ParentType.Name(), nil
		},
	})
}

// inputValueSource is the common shape of an argument and an input object
// field, so that __InputValue can describe either without caring which it has.
type inputValueSource struct {
	name              string
	description       value.Maybe[string]
	typ               Type
	deprecationReason value.Maybe[string]
	def               value.Maybe[DefaultInput]
}

// defaultLiteral renders the default value as it would be written in a schema,
// or nil when there is none.
//
// A schema read from SDL keeps the literal it was written as, so that is what
// gets printed and the text comes back exactly as it went in. A default
// supplied in code is a Go value, which [LiteralFromValue] turns into the
// literal that stands for it.
func (v inputValueSource) defaultLiteral() any {
	def, has := v.def.Get()
	if !has {
		return nil
	}
	if def.Literal != nil {
		return language.Print(def.Literal)
	}
	literal, ok := LiteralFromValue(def.Value, v.typ)
	if !ok {
		return nil
	}
	return language.Print(literal)
}

// reasonOrNil answers with the reason a deprecation gave, which may be the
// empty string, or with nothing at all where there was no deprecation. The
// two are different answers: graphql-js reports isDeprecated as whether the
// reason is there, not whether it says anything.
func reasonOrNil(reason value.Maybe[string]) any {
	held, said := reason.Get()
	if !said {
		return nil
	}
	return held
}

// asInputValueSource reads whichever of the two shapes it was given.
func asInputValueSource(source any) (inputValueSource, bool) {
	switch s := source.(type) {
	case *Argument:
		return inputValueSource{
			name: s.Name(), description: s.DescribedAs(),
			typ: s.Type, deprecationReason: s.DeprecationReason, def: s.Default,
		}, true
	case *InputField:
		return inputValueSource{
			name: s.Name(), description: s.DescribedAs(),
			typ: s.Type, deprecationReason: s.DeprecationReason, def: s.Default,
		}, true
	default:
		return inputValueSource{}, false
	}
}

// The helpers below adapt a resolver written against a concrete source type to
// the general resolver signature, so that each field above says only what it
// does.

func resolveSchema(fn func(context.Context, *Schema, Arguments) (any, error)) FieldResolver {
	return func(ctx context.Context, source any, args Arguments, _ *ResolveInfo) (any, error) {
		s, ok := source.(*Schema)
		if !ok {
			return nil, errUnexpectedSource("__Schema", source)
		}
		return fn(ctx, s, args)
	}
}

func resolveType(fn func(Type, Arguments) (any, error)) FieldResolver {
	return func(_ context.Context, source any, args Arguments, _ *ResolveInfo) (any, error) {
		t, ok := source.(Type)
		if !ok {
			return nil, errUnexpectedSource("__Type", source)
		}
		return fn(t, args)
	}
}

func resolveField(fn func(*Field, Arguments) (any, error)) FieldResolver {
	return func(_ context.Context, source any, args Arguments, _ *ResolveInfo) (any, error) {
		f, ok := source.(*Field)
		if !ok {
			return nil, errUnexpectedSource("__Field", source)
		}
		return fn(f, args)
	}
}

func resolveDirective(fn func(*Directive, Arguments) (any, error)) FieldResolver {
	return func(_ context.Context, source any, args Arguments, _ *ResolveInfo) (any, error) {
		d, ok := source.(*Directive)
		if !ok {
			return nil, errUnexpectedSource("__Directive", source)
		}
		return fn(d, args)
	}
}

func resolveEnumValue(fn func(*EnumValue) (any, error)) FieldResolver {
	return func(_ context.Context, source any, _ Arguments, _ *ResolveInfo) (any, error) {
		v, ok := source.(*EnumValue)
		if !ok {
			return nil, errUnexpectedSource("__EnumValue", source)
		}
		return fn(v)
	}
}

func resolveInputValue(fn func(inputValueSource) (any, error)) FieldResolver {
	return func(_ context.Context, source any, _ Arguments, _ *ResolveInfo) (any, error) {
		v, ok := asInputValueSource(source)
		if !ok {
			return nil, errUnexpectedSource("__InputValue", source)
		}
		return fn(v)
	}
}

// describeType returns a named type's description, or nothing for a wrapper.
func describeType(t Type) value.Maybe[string] {
	switch n := t.(type) {
	case *ScalarType:
		return n.DescribedAs()
	case *ObjectType:
		return n.DescribedAs()
	case *InterfaceType:
		return n.DescribedAs()
	case *UnionType:
		return n.DescribedAs()
	case *EnumType:
		return n.DescribedAs()
	case *InputObjectType:
		return n.DescribedAs()
	default:
		return value.Nothing[string]()
	}
}

// emptyToNil turns an unset description or reason into a null, since the
// introspection schema declares those fields nullable and an empty string
// would be a different answer from "there isn't one".
// describedOrNil answers with documentation as a response carries it: the text
// where something was written, even where what was written is empty, and null
// where nothing was.
func describedOrNil(described value.Maybe[string]) any {
	if text, written := described.Get(); written {
		return text
	}
	return nil
}

func emptyToNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nilIfAbsentObject keeps a nil object type from reaching a resolver as a
// non-nil interface holding nothing.
func nilIfAbsentType(t NamedType) any {
	if t == nil {
		return nil
	}
	return t
}

// anySlice widens a slice so that it can be returned from a resolver.
func anySlice[T any](items []T) []any {
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = item
	}
	return out
}

func filterDeprecatedFields(fields []*Field, include bool) []*Field {
	if include {
		return fields
	}
	out := make([]*Field, 0, len(fields))
	for _, f := range fields {
		if f != nil && !f.IsDeprecated() {
			out = append(out, f)
		}
	}
	return out
}

func filterDeprecatedArgs(args []*Argument, include bool) []*Argument {
	if include {
		return args
	}
	out := make([]*Argument, 0, len(args))
	for _, a := range args {
		if a != nil && !a.IsDeprecated() {
			out = append(out, a)
		}
	}
	return out
}

// filterDeprecatedDirectives leaves out the deprecated directives unless they
// were asked for.
func filterDeprecatedDirectives(directives []*Directive, include bool) []*Directive {
	if include {
		return directives
	}
	out := make([]*Directive, 0, len(directives))
	for _, d := range directives {
		if d != nil && !d.IsDeprecated() {
			out = append(out, d)
		}
	}
	return out
}

func filterDeprecatedInputFields(fields []*InputField, include bool) []*InputField {
	if include {
		return fields
	}
	out := make([]*InputField, 0, len(fields))
	for _, f := range fields {
		if f != nil && !f.IsDeprecated() {
			out = append(out, f)
		}
	}
	return out
}

func filterDeprecatedEnumValues(values []*EnumValue, include bool) []*EnumValue {
	if include {
		return values
	}
	out := make([]*EnumValue, 0, len(values))
	for _, v := range values {
		if v != nil && !v.IsDeprecated() {
			out = append(out, v)
		}
	}
	return out
}

// errUnexpectedSource reports a resolver being handed something that is not
// the kind of value its introspection type describes.
func errUnexpectedSource(typeName string, source any) error {
	return fmt.Errorf("%s resolver received a %T, which is not a schema element it can describe", typeName, source)
}

// errUnexpectedType reports a type that is none of the kinds the introspection
// schema knows about, which can only happen if a caller implemented the Type
// interface itself.
func errUnexpectedType(t Type) error {
	return fmt.Errorf("unexpected type %T, which is not one of the kinds __TypeKind names", t)
}
