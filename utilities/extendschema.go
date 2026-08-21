package utilities

import (
	"fmt"
	"github.com/ikawaha/graphql/gqlerror"
	"slices"

	"github.com/ikawaha/graphql/internal/typeinfo"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// ExtendSchema returns a schema with the definitions and extensions of a
// document applied to an existing one.
//
// The original is left alone. A schema's types point at one another, so
// extending one type means every type that mentions it must point at the new
// version: the whole schema is therefore rebuilt, with each type recreated and
// its references resolved by name through the new set. Types nothing extends
// come out the same in every way that matters, but they are not the same
// objects, so a caller holding a type from the old schema must look it up
// again in the new one.
//
// The document is checked against the rules a schema definition must follow
// before anything is built from it, which is what reports a type the schema
// already has or a directive nothing declares. [AssumeValidSDL] skips that
// check.
func ExtendSchema(s *schema.Schema, doc *language.Document, opts ...BuildOption) (*schema.Schema, error) {
	if doc == nil {
		return nil, fmt.Errorf("%s: expected a document, got nothing", operationName(s))
	}
	config := newBuildConfig(opts)
	if !config.skipSDLCheck() {
		if err := assertValidSDL(doc, s); err != nil {
			return nil, err
		}
	}
	e := &schemaExtender{
		rebuilder:   newRebuilder(),
		base:        s,
		what:        operationName(s),
		assumeValid: config.assumeValid,
		definitions: map[string]language.TypeDefinition{},
		extensions:  map[string][]language.TypeExtension{},
	}
	if err := e.collect(doc); err != nil {
		return nil, err
	}
	return e.build()
}

// ExtendSchemaSource parses SDL and applies it to an existing schema.
func ExtendSchemaSource(s *schema.Schema, source string, opts ...BuildOption) (*schema.Schema, error) {
	doc, err := language.ParseString(source, newBuildConfig(opts).parse...)
	if err != nil {
		return nil, err
	}
	return ExtendSchema(s, doc, opts...)
}

// operationName names what the caller asked for, for error messages.
func operationName(base *schema.Schema) string {
	if base == nil {
		return "build schema"
	}
	return "extend schema"
}

// schemaExtender holds the state of one extension.
type schemaExtender struct {
	// rebuilder owns the new set of types and the rewiring of references into
	// it, which extending a schema and mapping one both need.
	*rebuilder

	base *schema.Schema
	// what names the operation in error messages, since the same code both
	// builds a schema from nothing and extends one that already exists.
	what string
	// assumeValid marks the schema this produces as one nothing need check,
	// which is what a caller who already knows it is sound asks for.
	assumeValid bool

	definitions map[string]language.TypeDefinition
	extensions  map[string][]language.TypeExtension

	directiveNodes   []*language.DirectiveDefinition
	schemaNode       *language.SchemaDefinition
	schemaExtensions []*language.SchemaExtension

	// missing holds the types the document named that nothing defines. They
	// are found while the fields are being built, which happens inside a thunk
	// that has no caller to return an error to, so they are kept here and
	// answered once the schema is assembled.
	missing []error
}

// collect reads the document and works out what each name will be built from.
func (e *schemaExtender) collect(doc *language.Document) error {
	// A name the base schema already has keeps its place, so that extending a
	// schema does not shuffle it.
	if e.base != nil {
		for _, t := range e.base.Types() {
			if t == nil || !isRebuildable(t) {
				continue
			}
			e.order = append(e.order, t.Name())
		}
	}

	for _, def := range doc.Definitions {
		switch node := def.(type) {
		case *language.SchemaDefinition:
			// A document holding more than one is refused by the check it
			// went through first. graphql-js keeps the last of them, since
			// each is assigned over the one before, and so does this.
			e.schemaNode = node

		case *language.SchemaExtension:
			e.schemaExtensions = append(e.schemaExtensions, node)

		case *language.DirectiveDefinition:
			if node.Name == nil {
				return fmt.Errorf("%s: a directive definition has no name", e.what)
			}
			e.directiveNodes = append(e.directiveNodes, node)

		case language.TypeExtension:
			name := typeExtensionName(node)
			if name == "" {
				return fmt.Errorf("%s: a type extension has no name", e.what)
			}
			e.extensions[name] = append(e.extensions[name], node)

		case language.TypeDefinition:
			name := typeDefinitionName(node)
			if name == "" {
				return fmt.Errorf("%s: a type definition has no name", e.what)
			}
			if _, builtIn := e.types[name]; builtIn {
				// A document redefining a built-in scalar or an introspection
				// type does not get to: the standard one stands, and what was
				// written is dropped. graphql-js does the same, reaching for
				// its own type before building the one described.
				continue
			}
			// A name defined twice, or defined here and held by the schema
			// already, is refused by the check the document went through
			// first. graphql-js sets each definition into one map in turn and
			// so keeps the last; this keeps the last for the same reason, and
			// leaves the name where it first appeared.
			if _, seen := e.definitions[name]; !seen && e.baseType(name) == nil {
				e.order = append(e.order, name)
			}
			e.definitions[name] = node
		}
	}

	// Extending something that is not there is refused by the check the
	// document went through first. With that waived, graphql-js builds a
	// schema in which the extension applied to nothing, and so does this:
	// a name nothing defines is never built, so nothing reads its extensions.

	// Every type is created before any of them resolves what it refers to.
	for _, name := range e.order {
		built, err := e.newType(name)
		if err != nil {
			return err
		}
		e.types[name] = built
	}
	return nil
}

// baseType returns the type the base schema had under a name, if any.
func (e *schemaExtender) baseType(name string) schema.NamedType {
	if e.base == nil {
		return nil
	}
	t := e.base.Type(name)
	if t == nil || !isRebuildable(t) {
		return nil
	}
	return t
}

// lookup finds a type by name in the new set.
func (e *schemaExtender) lookup(name string) schema.NamedType { return e.types[name] }

// resolveRef resolves a type reference written in the document.
func (e *schemaExtender) resolveRef(node language.Type) (schema.Type, error) {
	resolved, ok := typeinfo.TypeFromASTWith(node, e.lookup)
	if !ok {
		err := e.unknownType(node)
		e.missing = append(e.missing, err)
		return nil, err
	}
	return resolved, nil
}

// unknownType is what a reference to a type the schema does not have comes to.
//
// The name at the centre of the reference is what is named, not the reference
// as it was written: `[Missing!]!` is unusable because nothing defines
// Missing, and that is what graphql-js says too.
func (e *schemaExtender) unknownType(node language.Type) error {
	if named := language.NamedTypeOf(node); named != nil && named.Name != nil {
		return unknownTypeNamed(named.Name.Value)
	}
	// No name anywhere in it. The grammar cannot produce such a reference, so
	// this is a document assembled in Go rather than parsed.
	return fmt.Errorf("%s: a type reference names nothing", e.what)
}

// unknownTypeNamed words it as graphql-js does, which is without saying what
// was being done: the same sentence is raised while building a schema and
// while extending one.
func unknownTypeNamed(name string) error {
	return gqlerror.Newf("Unknown type: %q.", name)
}

// cannotName records a name used where a type of a particular sort was wanted
// and is either not defined at all or is not of that sort.
// deprecationReason reads an applied @deprecated, recording a reason the
// directive's own String! will not take rather than guessing what was meant.
func (e *schemaExtender) deprecationReason(directives []*language.Directive) value.Maybe[string] {
	reason, refused := deprecationReason(directives)
	if refused == nil {
		return reason
	}
	// What is wrong with it is whatever the argument's own type says, which is
	// how graphql-js words it: a null and a number are refused differently.
	declared := schema.Deprecated.Arg("reason").Type
	for _, why := range schema.ValidateInputLiteral(refused, declared, schema.VariableValues{}) {
		e.missing = append(e.missing, fmt.Errorf(
			"%s: argument %q has invalid value: %s", e.what, "@deprecated(reason:)", why.Message))
	}
	return reason
}

func (e *schemaExtender) cannotName(name, wanted string) {
	if e.types[name] == nil {
		e.missing = append(e.missing, unknownTypeNamed(name))
		return
	}
	e.missing = append(e.missing, fmt.Errorf("%s: %s is not %s", e.what, name, wanted))
}

// newType creates the type a name will have, from whichever of the base
// schema and the document describe it.
func (e *schemaExtender) newType(name string) (schema.NamedType, error) {
	extensions := e.extensions[name]

	// A definition in the document stands in place of whatever the schema
	// held under that name. Redefining what a schema already has is refused
	// before this runs; graphql-js, asked to skip that check, sets the
	// document's definition over the schema's, and this does the same.
	if def := e.definitions[name]; def != nil {
		return e.createFromAST(def, extensions)
	}
	if existing := e.baseType(name); existing != nil {
		return e.rebuild(existing, e.mapperFor(extensions)), nil
	}
	return e.createFromAST(nil, extensions)
}

// mapperFor says what the extensions of a type add to it, in the form the
// shared rebuilder takes.
//
// Each hook is given what the type already held and returns that plus what the
// extensions bring. The list it is given belongs to the original schema, so it
// is copied before anything is added to it.
func (e *schemaExtender) mapperFor(extensions []language.TypeExtension) SchemaMapper {
	if len(extensions) == 0 {
		return SchemaMapper{}
	}
	return SchemaMapper{
		Fields: func(_ schema.NamedType, fields []*schema.Field) []*schema.Field {
			return append(slices.Clone(fields), e.fieldsFromExtensions(extensions)...)
		},
		InputFields: func(_ schema.NamedType, fields []*schema.InputField) []*schema.InputField {
			return append(slices.Clone(fields), e.inputFieldsFromExtensions(extensions)...)
		},
		Interfaces: func(_ schema.NamedType, ifaces []schema.Declared[*schema.InterfaceType]) []schema.Declared[*schema.InterfaceType] {
			return append(slices.Clone(ifaces), e.interfacesFromExtensions(extensions)...)
		},
		UnionMembers: func(_ schema.NamedType, members []schema.Declared[*schema.ObjectType]) []schema.Declared[*schema.ObjectType] {
			return append(slices.Clone(members), e.membersFromExtensions(extensions)...)
		},
		EnumValues: func(_ schema.NamedType, values []*schema.EnumValue) []*schema.EnumValue {
			return append(slices.Clone(values), e.valuesFromExtensions(extensions)...)
		},
		// An extension is how a scalar already in the schema is told where it
		// is specified, so it fills the gap when the type has no URL yet.
		Scalar: func(c schema.ScalarConfig) schema.ScalarConfig {
			if c.SpecifiedByURL == "" {
				c.SpecifiedByURL = specifiedByFromExtensions(extensions)
			}
			c.ExtensionASTNodes = appendExtensions(c.ExtensionASTNodes, extensions)
			return c
		},
		InputObject: func(c schema.InputObjectConfig) schema.InputObjectConfig {
			c.IsOneOf = c.IsOneOf || oneOfInExtensions(extensions)
			c.ExtensionASTNodes = appendExtensions(c.ExtensionASTNodes, extensions)
			return c
		},
		// The remaining hooks are here for one reason: a type has to remember
		// what extended it. Nothing else about these kinds changes.
		Object: func(c schema.ObjectConfig) schema.ObjectConfig {
			c.ExtensionASTNodes = appendExtensions(c.ExtensionASTNodes, extensions)
			return c
		},
		Interface: func(c schema.InterfaceConfig) schema.InterfaceConfig {
			c.ExtensionASTNodes = appendExtensions(c.ExtensionASTNodes, extensions)
			return c
		},
		Union: func(c schema.UnionConfig) schema.UnionConfig {
			c.ExtensionASTNodes = appendExtensions(c.ExtensionASTNodes, extensions)
			return c
		},
		Enum: func(c schema.EnumConfig) schema.EnumConfig {
			c.ExtensionASTNodes = appendExtensions(c.ExtensionASTNodes, extensions)
			return c
		},
	}
}

// appendExtensions adds the extensions of one kind to what a type already
// held, which is how a type extended more than once remembers every one.
//
// The kind is settled by what the type's own list is a list of, so each hook
// above needs only to hand over both lists.
func appendExtensions[T language.TypeExtension](held []T, extensions []language.TypeExtension) []T {
	out := slices.Clone(held)
	for _, ext := range extensions {
		if node, isKind := ext.(T); isKind {
			out = append(out, node)
		}
	}
	return out
}

// createFromAST creates a type the document defines, merging in anything that
// extends it in the same document.
func (e *schemaExtender) createFromAST(node language.TypeDefinition, extensions []language.TypeExtension) (schema.NamedType, error) {
	switch def := node.(type) {
	case *language.ScalarTypeDefinition:
		url := directiveArgumentString(def.Directives, "specifiedBy", "url")
		if url == "" {
			url = specifiedByFromExtensions(extensions)
		}
		return schema.NewScalar(schema.ScalarConfig{
			Name:              def.Name.Value,
			Description:       descriptionOf(def.Description),
			SpecifiedByURL:    url,
			ASTNode:           def,
			ExtensionASTNodes: appendExtensions[*language.ScalarTypeExtension](nil, extensions),
		}), nil

	case *language.ObjectTypeDefinition:
		return schema.NewObject(schema.ObjectConfig{
			Name:              def.Name.Value,
			Description:       descriptionOf(def.Description),
			ASTNode:           def,
			ExtensionASTNodes: appendExtensions[*language.ObjectTypeExtension](nil, extensions),
			FieldsThunk: func() []*schema.Field {
				return append(e.fieldsFromAST(def.Fields), e.fieldsFromExtensions(extensions)...)
			},
			InterfacesThunk: func() []schema.Declared[*schema.InterfaceType] {
				return append(e.interfacesFromAST(def.Interfaces), e.interfacesFromExtensions(extensions)...)
			},
		}), nil

	case *language.InterfaceTypeDefinition:
		return schema.NewInterface(schema.InterfaceConfig{
			Name:              def.Name.Value,
			Description:       descriptionOf(def.Description),
			ASTNode:           def,
			ExtensionASTNodes: appendExtensions[*language.InterfaceTypeExtension](nil, extensions),
			FieldsThunk: func() []*schema.Field {
				return append(e.fieldsFromAST(def.Fields), e.fieldsFromExtensions(extensions)...)
			},
			InterfacesThunk: func() []schema.Declared[*schema.InterfaceType] {
				return append(e.interfacesFromAST(def.Interfaces), e.interfacesFromExtensions(extensions)...)
			},
		}), nil

	case *language.UnionTypeDefinition:
		return schema.NewUnion(schema.UnionConfig{
			Name:              def.Name.Value,
			Description:       descriptionOf(def.Description),
			ASTNode:           def,
			ExtensionASTNodes: appendExtensions[*language.UnionTypeExtension](nil, extensions),
			TypesThunk: func() []schema.Declared[*schema.ObjectType] {
				return append(e.membersFromAST(def.Types), e.membersFromExtensions(extensions)...)
			},
		}), nil

	case *language.EnumTypeDefinition:
		values := append(e.valuesFromAST(def.Values), e.valuesFromExtensions(extensions)...)
		return schema.NewEnum(schema.EnumConfig{
			Name:              def.Name.Value,
			Description:       descriptionOf(def.Description),
			Values:            values,
			ASTNode:           def,
			ExtensionASTNodes: appendExtensions[*language.EnumTypeExtension](nil, extensions),
		}), nil

	case *language.InputObjectTypeDefinition:
		return schema.NewInputObject(schema.InputObjectConfig{
			Name:              def.Name.Value,
			Description:       descriptionOf(def.Description),
			IsOneOf:           hasDirective(def.Directives, "oneOf") || oneOfInExtensions(extensions),
			ASTNode:           def,
			ExtensionASTNodes: appendExtensions[*language.InputObjectTypeExtension](nil, extensions),
			FieldsThunk: func() []*schema.InputField {
				return append(e.inputFieldsFromAST(def.Fields), e.inputFieldsFromExtensions(extensions)...)
			},
		}), nil

	default:
		return nil, fmt.Errorf("%s: unexpected definition %T", e.what, node)
	}
}

// The AST helpers below build from the document, and are shared by the
// definition and the extension paths.

func (e *schemaExtender) fieldsFromAST(nodes []*language.FieldDefinition) []*schema.Field {
	fields := make([]*schema.Field, 0, len(nodes))
	for _, node := range nodes {
		if node == nil || node.Name == nil {
			continue
		}
		fieldType, err := e.resolveRef(node.Type)
		if err != nil {
			continue
		}
		fields = append(fields, schema.NewField(node.Name.Value, schema.FieldConfig{
			Description:       descriptionOf(node.Description),
			Type:              fieldType,
			Args:              e.argumentsFromAST(node.Arguments),
			DeprecationReason: e.deprecationReason(node.Directives),
			ASTNode:           node,
		}))
	}
	return fields
}

func (e *schemaExtender) argumentsFromAST(nodes []*language.InputValueDefinition) []*schema.Argument {
	args := make([]*schema.Argument, 0, len(nodes))
	for _, node := range nodes {
		if node == nil || node.Name == nil {
			continue
		}
		argType, err := e.resolveRef(node.Type)
		if err != nil {
			continue
		}
		args = append(args, schema.NewArgument(node.Name.Value, schema.ArgumentConfig{
			Description:       descriptionOf(node.Description),
			Type:              argType,
			Default:           defaultOf(node.DefaultValue),
			DeprecationReason: e.deprecationReason(node.Directives),
			ASTNode:           node,
		}))
	}
	return args
}

func (e *schemaExtender) inputFieldsFromAST(nodes []*language.InputValueDefinition) []*schema.InputField {
	fields := make([]*schema.InputField, 0, len(nodes))
	for _, node := range nodes {
		if node == nil || node.Name == nil {
			continue
		}
		fieldType, err := e.resolveRef(node.Type)
		if err != nil {
			continue
		}
		fields = append(fields, schema.NewInputField(node.Name.Value, schema.InputFieldConfig{
			Description:       descriptionOf(node.Description),
			Type:              fieldType,
			Default:           defaultOf(node.DefaultValue),
			DeprecationReason: e.deprecationReason(node.Directives),
			ASTNode:           node,
		}))
	}
	return fields
}

// interfacesFromAST reads an implements clause, holding whatever each name
// turned out to be. One that is not an interface is what ValidateSchema
// reports, for the reason membersFromAST gives.
func (e *schemaExtender) interfacesFromAST(nodes []*language.NamedType) []schema.Declared[*schema.InterfaceType] {
	out := make([]schema.Declared[*schema.InterfaceType], 0, len(nodes))
	for _, node := range nodes {
		if node == nil || node.Name == nil {
			continue
		}
		named, defined := e.types[node.Name.Value]
		if !defined || named == nil {
			e.cannotName(node.Name.Value, "an interface")
			continue
		}
		out = append(out, schema.DeclareNamed[*schema.InterfaceType](named))
	}
	return out
}

// membersFromAST reads a union's members, holding whatever each name turned
// out to be. A member that is not an object type is what ValidateSchema
// reports; refusing to build would leave nothing for it to report against,
// and nothing to print or describe to a client either.
func (e *schemaExtender) membersFromAST(nodes []*language.NamedType) []schema.Declared[*schema.ObjectType] {
	out := make([]schema.Declared[*schema.ObjectType], 0, len(nodes))
	for _, node := range nodes {
		if node == nil || node.Name == nil {
			continue
		}
		named, defined := e.types[node.Name.Value]
		if !defined || named == nil {
			// Nothing of that name at all, which is reported where every
			// unknown name is.
			e.cannotName(node.Name.Value, "an object type")
			continue
		}
		out = append(out, schema.DeclareNamed[*schema.ObjectType](named))
	}
	return out
}

func (e *schemaExtender) valuesFromAST(nodes []*language.EnumValueDefinition) []*schema.EnumValue {
	out := make([]*schema.EnumValue, 0, len(nodes))
	for _, node := range nodes {
		if node == nil || node.Name == nil {
			continue
		}
		out = append(out, schema.NewEnumValue(node.Name.Value, schema.EnumValueConfig{
			Description:       descriptionOf(node.Description),
			DeprecationReason: e.deprecationReason(node.Directives),
			ASTNode:           node,
		}))
	}
	return out
}

// The extension helpers below gather what each kind of extension adds. An
// extension of the wrong kind for the type it names contributes nothing here;
// the schema validator is what reports the mismatch.

func (e *schemaExtender) fieldsFromExtensions(extensions []language.TypeExtension) []*schema.Field {
	var out []*schema.Field
	for _, ext := range extensions {
		switch node := ext.(type) {
		case *language.ObjectTypeExtension:
			out = append(out, e.fieldsFromAST(node.Fields)...)
		case *language.InterfaceTypeExtension:
			out = append(out, e.fieldsFromAST(node.Fields)...)
		}
	}
	return out
}

func (e *schemaExtender) inputFieldsFromExtensions(extensions []language.TypeExtension) []*schema.InputField {
	var out []*schema.InputField
	for _, ext := range extensions {
		if node, ok := ext.(*language.InputObjectTypeExtension); ok {
			out = append(out, e.inputFieldsFromAST(node.Fields)...)
		}
	}
	return out
}

func (e *schemaExtender) interfacesFromExtensions(extensions []language.TypeExtension) []schema.Declared[*schema.InterfaceType] {
	var out []schema.Declared[*schema.InterfaceType]
	for _, ext := range extensions {
		switch node := ext.(type) {
		case *language.ObjectTypeExtension:
			out = append(out, e.interfacesFromAST(node.Interfaces)...)
		case *language.InterfaceTypeExtension:
			out = append(out, e.interfacesFromAST(node.Interfaces)...)
		}
	}
	return out
}

func (e *schemaExtender) membersFromExtensions(extensions []language.TypeExtension) []schema.Declared[*schema.ObjectType] {
	var out []schema.Declared[*schema.ObjectType]
	for _, ext := range extensions {
		if node, ok := ext.(*language.UnionTypeExtension); ok {
			out = append(out, e.membersFromAST(node.Types)...)
		}
	}
	return out
}

func (e *schemaExtender) valuesFromExtensions(extensions []language.TypeExtension) []*schema.EnumValue {
	var out []*schema.EnumValue
	for _, ext := range extensions {
		if node, ok := ext.(*language.EnumTypeExtension); ok {
			out = append(out, e.valuesFromAST(node.Values)...)
		}
	}
	return out
}

// build assembles the extended schema once every type exists.
func (e *schemaExtender) build() (*schema.Schema, error) {
	config := schema.Config{AssumeValid: e.assumeValid, Directives: e.allDirectives()}
	for _, name := range e.order {
		if t := e.types[name]; t != nil {
			config.Types = append(config.Types, t)
		}
	}

	// The base schema's roots carry over, unless the document names others, and
	// so does everything else it was built with.
	if e.base != nil {
		config.Description = e.base.DescribedAs()
		config.Extensions = e.base.Extensions
		config.ASTNode = e.base.ASTNode
		config.ExtensionASTNodes = append(config.ExtensionASTNodes, e.base.ExtensionASTNodes...)
		config.Query, _ = e.types[nameOfType(e.base.QueryType())].(*schema.ObjectType)
		config.Mutation, _ = e.types[nameOfType(e.base.MutationType())].(*schema.ObjectType)
		config.Subscription, _ = e.types[nameOfType(e.base.SubscriptionType())].(*schema.ObjectType)
	}

	nodes := make([]*language.SchemaDefinition, 0, 1)
	if e.schemaNode != nil {
		nodes = append(nodes, e.schemaNode)
		config.Description = descriptionOf(e.schemaNode.Description)
		config.ASTNode = e.schemaNode
	}
	for _, node := range nodes {
		if err := e.applyRoots(&config, node.OperationTypes); err != nil {
			return nil, err
		}
	}
	for _, ext := range e.schemaExtensions {
		if err := e.applyRoots(&config, ext.OperationTypes); err != nil {
			return nil, err
		}
	}
	config.ExtensionASTNodes = append(config.ExtensionASTNodes, e.schemaExtensions...)

	// With no base schema and nothing said about the roots, they are found by
	// their conventional names.
	if e.base == nil && e.schemaNode == nil {
		roots := []struct {
			name string
			into *schema.NamedType
		}{
			{"Query", &config.Query},
			{"Mutation", &config.Mutation},
			{"Subscription", &config.Subscription},
		}
		for _, root := range roots {
			if *root.into != nil {
				continue
			}
			named, defined := e.types[root.name]
			if !defined || named == nil {
				continue
			}
			// A type of the conventional name is the root whatever kind it
			// is. One a request cannot enter through is what ValidateSchema
			// reports, as it is in graphql-js; leaving it out here would
			// instead build a schema the document did not describe.
			*root.into = named
		}
	}

	built := schema.New(config)
	// Building the fields is what turns up a name nothing defines, and that
	// happens lazily, so the answer is not known until the schema has been
	// assembled and every thunk has run.
	if len(e.missing) > 0 {
		return nil, e.missing[0]
	}
	return built, nil
}

// applyRoots points the roots at whichever types a schema definition or
// extension names.
func (e *schemaExtender) applyRoots(config *schema.Config, operations []*language.OperationTypeDefinition) error {
	for _, op := range operations {
		if op == nil || op.Type == nil || op.Type.Name == nil {
			continue
		}
		root, defined := e.types[op.Type.Name.Value]
		if !defined || root == nil {
			// Nothing of that name: reported where every unknown name is.
			e.cannotName(op.Type.Name.Value, "a type")
			continue
		}
		switch op.Operation {
		case language.OperationQuery:
			config.Query = root
		case language.OperationMutation:
			config.Mutation = root
		case language.OperationSubscription:
			config.Subscription = root
		}
	}
	return nil
}

// allDirectives returns the directives the extended schema allows.
//
// Extending a schema keeps what it had at the front and adds what the document
// declares after; building one from a document puts the document's own
// directives first, since the built-in ones it did not declare are only being
// filled in.
func (e *schemaExtender) allDirectives() []*schema.Directive {
	var out []*schema.Directive
	defined := make(map[string]bool, len(e.directiveNodes))
	for _, node := range e.directiveNodes {
		defined[node.Name.Value] = true
	}

	if e.base != nil {
		// What the schema already declares, then what the document declares,
		// which is how graphql-js puts the two lists together. A document
		// redeclaring a directive the schema has is refused before this runs,
		// so the only way to reach a schema holding two of one name is to
		// have asked for the check to be skipped.
		for _, d := range e.base.Directives() {
			if d == nil {
				continue
			}
			out = append(out, e.redirectDirective(d, SchemaMapper{}))
		}
		for _, node := range e.directiveNodes {
			out = append(out, e.directiveFromAST(node))
		}
		return out
	}

	for _, node := range e.directiveNodes {
		out = append(out, e.directiveFromAST(node))
	}
	for _, d := range schema.SpecifiedDirectives {
		if !defined[d.Name()] {
			out = append(out, d)
		}
	}
	return out
}

// directiveFromAST creates a directive the document declares.
func (e *schemaExtender) directiveFromAST(node *language.DirectiveDefinition) *schema.Directive {
	locations := make([]language.DirectiveLocation, 0, len(node.Locations))
	for _, loc := range node.Locations {
		if loc != nil {
			locations = append(locations, language.DirectiveLocation(loc.Value))
		}
	}
	return schema.NewDirective(schema.DirectiveConfig{
		Name:              node.Name.Value,
		Description:       descriptionOf(node.Description),
		Locations:         locations,
		Args:              e.argumentsFromAST(node.Arguments),
		IsRepeatable:      node.Repeatable,
		DeprecationReason: e.deprecationReason(node.Directives),
		ASTNode:           node,
	})
}

// typeExtensionName reads the name an extension applies to.
func typeExtensionName(node language.TypeExtension) string {
	switch ext := node.(type) {
	case *language.ScalarTypeExtension:
		return nameOf(ext.Name)
	case *language.ObjectTypeExtension:
		return nameOf(ext.Name)
	case *language.InterfaceTypeExtension:
		return nameOf(ext.Name)
	case *language.UnionTypeExtension:
		return nameOf(ext.Name)
	case *language.EnumTypeExtension:
		return nameOf(ext.Name)
	case *language.InputObjectTypeExtension:
		return nameOf(ext.Name)
	default:
		return ""
	}
}

// Only two directives on an extension change the type it applies to: a scalar
// can be told where it is specified, and an input object can be made a choice
// of one field. The rest apply to the extension itself, which the schema does
// not keep.

// specifiedByFromExtensions reads a URL given by any scalar extension.
func specifiedByFromExtensions(extensions []language.TypeExtension) string {
	for _, ext := range extensions {
		if scalar, ok := ext.(*language.ScalarTypeExtension); ok {
			if url := directiveArgumentString(scalar.Directives, "specifiedBy", "url"); url != "" {
				return url
			}
		}
	}
	return ""
}

// oneOfInExtensions reports whether any input object extension marks the type
// as a choice of exactly one field.
func oneOfInExtensions(extensions []language.TypeExtension) bool {
	for _, ext := range extensions {
		if input, ok := ext.(*language.InputObjectTypeExtension); ok {
			if hasDirective(input.Directives, "oneOf") {
				return true
			}
		}
	}
	return false
}
