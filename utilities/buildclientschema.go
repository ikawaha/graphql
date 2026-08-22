package utilities

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// BuildClientSchema turns what a server said about itself into a schema.
//
// The result describes the server but cannot answer for it: an introspection
// query says what fields a type has, not how their values are produced, so the
// schema has no resolvers and executing against it would return nothing. It is
// for the things a client does with a schema — validating a query before
// sending it, generating types, showing what a server offers — and for
// checking what changed between two versions of a server.
//
// What comes back is only as complete as what was asked for. A query built
// without [WithEverything] leaves parts out, and they are missing here too.
func BuildClientSchema(answer *IntrospectionQueryResult) (*schema.Schema, error) {
	if answer == nil {
		return nil, fmt.Errorf("build client schema: expected an introspection result, got nothing")
	}
	described := answer.Schema
	if described.QueryType == nil {
		return nil, fmt.Errorf("build client schema: the answer names no query type")
	}

	b := &clientSchemaBuilder{types: map[string]schema.NamedType{}}
	// Every type exists before any of them resolves what it refers to, since
	// they refer to one another freely.
	for _, described := range described.Types {
		if described == nil || described.Name == "" {
			continue
		}
		if _, twice := b.types[described.Name]; twice {
			continue
		}
		built, err := b.newType(described)
		if err != nil {
			return nil, err
		}
		b.types[described.Name] = built
		b.order = append(b.order, described.Name)
	}
	// A built-in scalar and an introspection type the answer describes is
	// shared rather than rebuilt: every schema has the same ones, and a
	// rebuilt Int would no longer be the Int that code written against this
	// library refers to. One the answer left out is not supplied, as
	// graphql-js does not supply it: an answer that uses a type it never
	// described is one this cannot faithfully rebuild.
	for _, shared := range schema.SpecifiedScalars {
		if _, described := b.types[shared.Name()]; described {
			b.types[shared.Name()] = shared
		}
	}
	for _, shared := range schema.IntrospectionTypes {
		if _, described := b.types[shared.Name()]; described {
			b.types[shared.Name()] = shared
		}
	}

	config := schema.Config{Description: documented(described.Description)}
	for _, name := range b.order {
		config.Types = append(config.Types, b.types[name])
	}

	var err error
	if config.Query, err = b.rootType(described.QueryType, "query"); err != nil {
		return nil, err
	}
	if config.Mutation, err = b.rootType(described.MutationType, "mutation"); err != nil {
		return nil, err
	}
	if config.Subscription, err = b.rootType(described.SubscriptionType, "subscription"); err != nil {
		return nil, err
	}

	for _, described := range described.Directives {
		if described == nil || described.Name == "" {
			continue
		}
		// A directive the specification defines is shared rather than rebuilt,
		// for the same reason as the built-in scalars above: code written
		// against this library refers to schema.Skip and the like by identity.
		// Only the ones the answer actually lists are taken, so a server that
		// does not have @oneOf does not gain one here.
		if described.Locations == nil {
			return nil, fmt.Errorf(
				"build client schema: introspection result missing directive locations: { name: %s }",
				strconv.Quote(described.Name))
		}
		if described.Args == nil {
			return nil, fmt.Errorf(
				"build client schema: introspection result missing directive args: { name: %s }",
				strconv.Quote(described.Name))
		}
		if shared := specifiedDirectiveNamed(described.Name); shared != nil {
			config.Directives = append(config.Directives, shared)
			continue
		}
		config.Directives = append(config.Directives, b.newDirective(described))
	}

	built := schema.New(config)
	// Assembling the schema is what reads every field list, and that is when a
	// reference to a type the answer never described turns up. Leaving the
	// field out and carrying on would give back a schema that is quietly
	// missing things, and a complaint about the wrong thing later.
	if len(b.missing) > 0 {
		return nil, b.missing[0]
	}
	return built, nil
}

// clientSchemaBuilder holds the state of one build.
type clientSchemaBuilder struct {
	types map[string]schema.NamedType
	// order keeps the names as the answer gave them, so that printing the
	// result twice gives the same text.
	order []string
	// missing records a reference to a type the answer did not describe. The
	// fields are built lazily, so this is not known until the schema has been
	// assembled and every list has been read.
	missing []error
}

// cannotResolve records a reference that could not be followed, and reports
// whether there was one.
func (b *clientSchemaBuilder) cannotResolve(err error) bool {
	if err == nil {
		return false
	}
	b.missing = append(b.missing, err)
	return true
}

// rootType resolves one of the schema's roots.
func (b *clientSchemaBuilder) rootType(ref *IntrospectionTypeRef, which string) (*schema.ObjectType, error) {
	if ref == nil {
		return nil, nil
	}
	resolved, err := b.resolveRef(ref)
	if err != nil {
		return nil, err
	}
	object, isObject := resolved.(*schema.ObjectType)
	if !isObject {
		return nil, fmt.Errorf("build client schema: the %s root %q is not an object type",
			which, ref.Name)
	}
	return object, nil
}

// resolveRef turns a described type reference into a type, rebuilding the
// list and non-null wrappers around it.
func (b *clientSchemaBuilder) resolveRef(ref *IntrospectionTypeRef) (schema.Type, error) {
	if ref == nil {
		return nil, fmt.Errorf("build client schema: a type reference is missing")
	}
	switch ref.Kind {
	case "LIST", "NON_NULL":
		// A wrapper with nothing inside it is what an introspection query that
		// stopped unfolding leaves behind: the answer says the type is a list
		// but not a list of what. Saying so names the fix, which is to ask
		// again and further; see [WithTypeDepth].
		if ref.OfType == nil {
			return nil, fmt.Errorf(
				"build client schema: a %s is described with nothing inside it, "+
					"which is what an introspection query that did not unfold far "+
					"enough leaves behind", strings.ToLower(strings.ReplaceAll(ref.Kind, "_", "-")))
		}
		inner, err := b.resolveRef(ref.OfType)
		if err != nil {
			return nil, err
		}
		if ref.Kind == "LIST" {
			return schema.NewList(inner), nil
		}
		return schema.NewNonNull(inner), nil
	}
	if ref.Name == "" {
		return nil, fmt.Errorf("build client schema: a type reference of kind %q has no name", ref.Kind)
	}
	named := b.types[ref.Name]
	if named == nil {
		return nil, fmt.Errorf("build client schema: invalid or incomplete schema, unknown type: %s. "+
			"Ensure that a full introspection query is used in order to build a client schema", ref.Name)
	}
	return named, nil
}

// newType creates the type an answer describes, with everything it refers to
// deferred until every type exists.
func (b *clientSchemaBuilder) newType(described *IntrospectionType) (schema.NamedType, error) {
	switch described.Kind {
	case "SCALAR":
		return schema.NewScalar(schema.ScalarConfig{
			Name:           described.Name,
			Description:    documented(described.Description),
			SpecifiedByURL: described.SpecifiedByURL,
		}), nil

	case "OBJECT":
		if described.Fields == nil {
			return nil, fmt.Errorf("build client schema: introspection result missing fields: %s",
				describeIntrospectionType(described))
		}
		if described.Interfaces == nil {
			return nil, fmt.Errorf("build client schema: introspection result missing interfaces: %s",
				describeIntrospectionType(described))
		}
		return schema.NewObject(schema.ObjectConfig{
			Name:            described.Name,
			Description:     documented(described.Description),
			FieldsThunk:     func() []*schema.Field { return b.fields(described.Fields) },
			InterfacesThunk: func() []schema.Declared[*schema.InterfaceType] { return b.interfaces(described.Interfaces) },
		}), nil

	case "INTERFACE":
		if described.Fields == nil {
			return nil, fmt.Errorf("build client schema: introspection result missing fields: %s",
				describeIntrospectionType(described))
		}
		// An interface with no interfaces of its own is accepted, where an
		// object is not: a server built before interfaces could implement
		// interfaces answers with nothing there, and graphql-js keeps taking
		// those answers.
		return schema.NewInterface(schema.InterfaceConfig{
			Name:            described.Name,
			Description:     documented(described.Description),
			FieldsThunk:     func() []*schema.Field { return b.fields(described.Fields) },
			InterfacesThunk: func() []schema.Declared[*schema.InterfaceType] { return b.interfaces(described.Interfaces) },
		}), nil

	case "UNION":
		if described.PossibleTypes == nil {
			return nil, fmt.Errorf("build client schema: introspection result missing possibleTypes: %s",
				describeIntrospectionType(described))
		}
		return schema.NewUnion(schema.UnionConfig{
			Name:        described.Name,
			Description: documented(described.Description),
			TypesThunk:  func() []schema.Declared[*schema.ObjectType] { return b.members(described.PossibleTypes) },
		}), nil

	case "ENUM":
		if described.EnumValues == nil {
			return nil, fmt.Errorf("build client schema: introspection result missing enumValues: %s",
				describeIntrospectionType(described))
		}
		values := make([]*schema.EnumValue, 0, len(described.EnumValues))
		for _, member := range described.EnumValues {
			if member == nil {
				continue
			}
			values = append(values, schema.NewEnumValue(member.Name, schema.EnumValueConfig{
				// An introspection answer carries no internal value — there
				// is nothing in it to carry one — so the member's value is
				// its name, which is what leaving this unset means.
				Description:       documented(member.Description),
				DeprecationReason: deprecationFrom(member.DeprecationReason),
			}))
		}
		return schema.NewEnum(schema.EnumConfig{
			Name:        described.Name,
			Description: documented(described.Description),
			Values:      values,
		}), nil

	case "INPUT_OBJECT":
		if described.InputFields == nil {
			return nil, fmt.Errorf("build client schema: introspection result missing inputFields: %s",
				describeIntrospectionType(described))
		}
		return schema.NewInputObject(schema.InputObjectConfig{
			Name:        described.Name,
			Description: documented(described.Description),
			IsOneOf:     described.IsOneOf,
			FieldsThunk: func() []*schema.InputField { return b.inputFields(described.InputFields) },
		}), nil

	default:
		return nil, fmt.Errorf("build client schema: %q is of unknown kind %q",
			described.Name, described.Kind)
	}
}

// The helpers below resolve what a type refers to, once every type exists.
//
// A reference that cannot be resolved leaves the field out rather than failing
// the build: an answer that named a type it did not list is one this cannot
// faithfully rebuild, and the schema validator reports the gap that leaves.

func (b *clientSchemaBuilder) fields(described []*IntrospectionField) []*schema.Field {
	fields := make([]*schema.Field, 0, len(described))
	for _, f := range described {
		if f == nil {
			continue
		}
		fieldType, err := b.resolveRef(f.Type)
		if b.cannotResolve(err) {
			continue
		}
		if !schema.IsOutputType(fieldType) {
			b.cannotResolve(fmt.Errorf(
				"build client schema: introspection must provide output type for fields, but received: %s",
				fieldType.String()))
			continue
		}
		if f.Args == nil {
			b.cannotResolve(fmt.Errorf(
				"build client schema: introspection result missing field args: { name: %s }",
				strconv.Quote(f.Name)))
			continue
		}
		fields = append(fields, schema.NewField(f.Name, schema.FieldConfig{
			Description:       documented(f.Description),
			Type:              fieldType,
			Args:              b.arguments(f.Args),
			DeprecationReason: deprecationFrom(f.DeprecationReason),
		}))
	}
	return fields
}

func (b *clientSchemaBuilder) arguments(described []*IntrospectionInputValue) []*schema.Argument {
	args := make([]*schema.Argument, 0, len(described))
	for _, a := range described {
		if a == nil {
			continue
		}
		argType, err := b.resolveRef(a.Type)
		if b.cannotResolve(err) {
			continue
		}
		if !schema.IsInputType(argType) {
			b.cannotResolve(fmt.Errorf(
				"build client schema: introspection must provide input type for arguments, but received: %s",
				argType.String()))
			continue
		}
		args = append(args, schema.NewArgument(a.Name, schema.ArgumentConfig{
			Description:       documented(a.Description),
			Type:              argType,
			Default:           defaultFromIntrospection(a.DefaultValue),
			DeprecationReason: deprecationFrom(a.DeprecationReason),
		}))
	}
	return args
}

func (b *clientSchemaBuilder) inputFields(described []*IntrospectionInputValue) []*schema.InputField {
	fields := make([]*schema.InputField, 0, len(described))
	for _, f := range described {
		if f == nil {
			continue
		}
		fieldType, err := b.resolveRef(f.Type)
		if b.cannotResolve(err) {
			continue
		}
		if !schema.IsInputType(fieldType) {
			b.cannotResolve(fmt.Errorf(
				"build client schema: introspection must provide input type for input fields, but received: %s",
				fieldType.String()))
			continue
		}
		fields = append(fields, schema.NewInputField(f.Name, schema.InputFieldConfig{
			Description:       documented(f.Description),
			Type:              fieldType,
			Default:           defaultFromIntrospection(f.DefaultValue),
			DeprecationReason: deprecationFrom(f.DeprecationReason),
		}))
	}
	return fields
}

func (b *clientSchemaBuilder) interfaces(described []*IntrospectionTypeRef) []schema.Declared[*schema.InterfaceType] {
	out := make([]schema.Declared[*schema.InterfaceType], 0, len(described))
	for _, ref := range described {
		resolved, err := b.resolveRef(ref)
		if b.cannotResolve(err) {
			continue
		}
		if named, isNamed := resolved.(schema.NamedType); isNamed {
			out = append(out, schema.DeclareNamed[*schema.InterfaceType](named))
		}
	}
	return out
}

func (b *clientSchemaBuilder) members(described []*IntrospectionTypeRef) []schema.Declared[*schema.ObjectType] {
	out := make([]schema.Declared[*schema.ObjectType], 0, len(described))
	for _, ref := range described {
		resolved, err := b.resolveRef(ref)
		if b.cannotResolve(err) {
			continue
		}
		if named, isNamed := resolved.(schema.NamedType); isNamed {
			out = append(out, schema.DeclareNamed[*schema.ObjectType](named))
		}
	}
	return out
}

// newDirective creates a directive the answer describes.
func (b *clientSchemaBuilder) newDirective(described *IntrospectionDirective) *schema.Directive {
	locations := make([]language.DirectiveLocation, 0, len(described.Locations))
	for _, name := range described.Locations {
		locations = append(locations, language.DirectiveLocation(name))
	}
	return schema.NewDirective(schema.DirectiveConfig{
		Name:              described.Name,
		Description:       documented(described.Description),
		Locations:         locations,
		Args:              b.arguments(described.Args),
		IsRepeatable:      described.IsRepeatable,
		DeprecationReason: deprecationFrom(described.DeprecationReason),
	})
}

// specifiedDirectiveNamed returns the built-in directive of a name, or nil.
func specifiedDirectiveNamed(name string) *schema.Directive {
	for _, specified := range schema.SpecifiedDirectives {
		if specified.Name() == name {
			return specified
		}
	}
	return nil
}

// defaultFromIntrospection reads a default value, which an answer gives as the
// text it would be written as.
//
// Keeping it as the literal rather than converting it to a Go value is what
// makes the round trip exact: the default comes back out written the way the
// server wrote it.
func defaultFromIntrospection(written *string) value.Maybe[schema.DefaultInput] {
	if written == nil {
		return value.Nothing[schema.DefaultInput]()
	}
	literal, err := language.ParseValue(language.NewSource(*written))
	if err != nil {
		// A default the server could not print is one this cannot rebuild, and
		// treating it as absent is closer than inventing one.
		return value.Nothing[schema.DefaultInput]()
	}
	return value.Just(schema.DefaultInput{Literal: literal})
}

// deprecatedBecause turns the two members an answer uses into the one this
// library keeps.
//
// A field marked deprecated with no reason given takes the reason that would
// be assumed anyway, so that it stays deprecated through the round trip.
// describeIntrospectionType names the part of an answer a complaint is about,
// the way graphql-js's inspect renders it: enough to find it in a response
// without printing the whole of it.
func describeIntrospectionType(described *IntrospectionType) string {
	if described == nil {
		return "null"
	}
	return `{ kind: ` + strconv.Quote(described.Kind) +
		`, name: ` + strconv.Quote(described.Name) + ` }`
}

// deprecationFrom reads a deprecation out of an introspection answer, where
// null and a missing field both say the element is not deprecated and an empty
// string deprecates it without saying why.
func deprecationFrom(reason *string) value.Maybe[string] {
	if reason == nil {
		return schema.NotDeprecated()
	}
	return schema.DeprecatedFor(*reason)
}

// documented turns the description an introspection answer carried into the
// three-state form a type holds. A JSON null is nothing written; the empty
// string is something written that happens to be empty, and graphql-js keeps
// the two apart.
func documented(described *string) value.Maybe[string] {
	if described == nil {
		return value.Nothing[string]()
	}
	return value.Just(*described)
}
