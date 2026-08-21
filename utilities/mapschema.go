package utilities

import (
	"reflect"

	"github.com/ikawaha/graphql/schema"
)

// SchemaMapper says how a schema should differ from the one it is rebuilt
// from.
//
// Each function is given a list as the original schema holds it and returns
// the list the new schema should have, so one hook covers reordering,
// leaving things out and adding things. Every function may be nil, meaning
// the list carries over unchanged.
//
// The values a hook receives belong to the original schema. Whatever it
// returns is rewired to point at the new types, so a hook may pass its input
// back in any order without worrying about which schema the types came from.
//
// The hooks run outside in and then inside out. A list hook settles what a
// type holds before the things in it are rebuilt, so Fields runs before
// Arguments and a field that Fields put there has its arguments mapped like
// any other. A configuration hook runs once everything under it is wired up,
// which makes it the place to look at the result rather than to shape it, and
// Schema runs last of all.
type SchemaMapper struct {
	// Types is given every named type the schema holds, less the built-in
	// scalars and the introspection types, which are always shared.
	Types func([]schema.NamedType) []schema.NamedType
	// Fields is given the fields of an object or interface type.
	Fields func(owner schema.NamedType, fields []*schema.Field) []*schema.Field
	// Arguments is given the arguments of a field or a directive. The owner is
	// written as it would be in a schema coordinate: "User.friends" for a
	// field, "@auth" for a directive.
	Arguments func(owner string, args []*schema.Argument) []*schema.Argument
	// InputFields is given the fields of an input object type.
	InputFields func(owner schema.NamedType, fields []*schema.InputField) []*schema.InputField
	// EnumValues is given the members of an enum type.
	EnumValues func(owner schema.NamedType, values []*schema.EnumValue) []*schema.EnumValue
	// UnionMembers is given the types a union stands for.
	UnionMembers func(owner schema.NamedType, members []schema.Declared[*schema.ObjectType]) []schema.Declared[*schema.ObjectType]
	// Interfaces is given the interfaces an object or interface implements.
	Interfaces func(owner schema.NamedType, interfaces []schema.Declared[*schema.InterfaceType]) []schema.Declared[*schema.InterfaceType]
	// Directives is given the directives the schema allows. The built-in ones
	// are shared rather than rebuilt, so reordering them is all this can do to
	// them; Arguments is not called for their arguments.
	Directives func([]*schema.Directive) []*schema.Directive

	// The hooks below are given a type's configuration as it stands just
	// before the type is made, which is everything the list hooks above do not
	// reach: the description, the AST nodes it came from, the extensions map,
	// and whatever else the kind holds. Each returns the configuration to
	// build from.
	//
	// A configuration that holds a thunk holds it already wired to the list
	// hooks, so a hook that wants to add to a list can call the thunk and add
	// to what comes back rather than starting over. Replacing a list means
	// replacing the thunk: where a configuration has both a list and a thunk
	// for the same thing, the thunk is what counts.

	// Scalar is given a scalar's configuration. This is where a coercer or a
	// specification URL is attached: a schema rebuilt from what a server said
	// about itself has no coercers, and this is how they are put back. The
	// built-in scalars are shared rather than rebuilt, so this is not called
	// for them.
	Scalar func(schema.ScalarConfig) schema.ScalarConfig
	// Object is given an object type's configuration.
	Object func(schema.ObjectConfig) schema.ObjectConfig
	// Interface is given an interface type's configuration.
	Interface func(schema.InterfaceConfig) schema.InterfaceConfig
	// Union is given a union type's configuration.
	Union func(schema.UnionConfig) schema.UnionConfig
	// Enum is given an enum type's configuration, members and all: a member
	// names no types, so there is nothing to defer and the list is already
	// there.
	Enum func(schema.EnumConfig) schema.EnumConfig
	// InputObject is given an input object type's configuration.
	InputObject func(schema.InputObjectConfig) schema.InputObjectConfig
	// Directive is given a directive's configuration. The built-in directives
	// are shared rather than rebuilt, so this is not called for them.
	Directive func(schema.DirectiveConfig) schema.DirectiveConfig

	// Schema is given the whole schema's configuration once every type and
	// directive in it has been rebuilt, and is the last thing to run. It is
	// where a root type is swapped for another, a type is added that nothing
	// refers to, or the schema's own description is changed; the types it is
	// given are the new ones, so a root can be picked out of Types by name.
	Schema func(schema.Config) schema.Config
}

// MapSchema returns a schema rebuilt from another, with the mapper's say over
// what each part becomes.
//
// This is how a schema is transformed wholesale: sorted, filtered down to what
// a public API should show, or added to. A schema's types point at one
// another, so changing one means rebuilding all of them; that is done here
// once rather than by every caller that wants a variation on a schema.
//
// The original is left alone, and as with [ExtendSchema] the types of the
// result are new objects: a caller holding a type from the original must look
// it up again in what comes back.
func MapSchema(s *schema.Schema, mapper SchemaMapper) *schema.Schema {
	if s == nil {
		return nil
	}
	r := newRebuilder()

	// The mapper has its say over which types there are, and in what order,
	// before any of them is rebuilt.
	types := r.rebuildableTypes(s)
	if mapper.Types != nil {
		types = mapper.Types(types)
	}
	for _, t := range types {
		if t == nil || !isRebuildable(t) {
			continue
		}
		r.order = append(r.order, t.Name())
		r.types[t.Name()] = r.rebuild(t, mapper)
	}

	config := schema.Config{
		Description:       s.DescribedAs(),
		Types:             r.builtTypes(),
		ASTNode:           s.ASTNode,
		ExtensionASTNodes: s.ExtensionASTNodes,
		Extensions:        s.Extensions,
	}
	config.Query, _ = r.types[nameOfType(s.QueryType())].(*schema.ObjectType)
	config.Mutation, _ = r.types[nameOfType(s.MutationType())].(*schema.ObjectType)
	config.Subscription, _ = r.types[nameOfType(s.SubscriptionType())].(*schema.ObjectType)

	directives := s.Directives()
	if mapper.Directives != nil {
		directives = mapper.Directives(directives)
	}
	for _, d := range directives {
		if d != nil {
			config.Directives = append(config.Directives, r.redirectDirective(d, mapper))
		}
	}

	if mapper.Schema != nil {
		config = mapper.Schema(config)
	}
	return schema.New(config)
}

// rebuilder holds the new set of types while they are being made.
//
// Rebuilding a schema is one idea used in more than one place — extending a
// schema and mapping one are both rebuilds — so the machinery for it lives
// here rather than once per caller. A type reference from the original points
// at the old objects and has to be followed by name into the new ones, with
// the list and non-null wrappers around it rebuilt to match.
type rebuilder struct {
	types map[string]schema.NamedType
	// order keeps the names in a settled order, since Go iterates a map in
	// none and a schema that came out differently each time could not be
	// printed reproducibly.
	order []string
}

// newRebuilder returns a rebuilder seeded with the types every schema shares.
func newRebuilder() *rebuilder {
	r := &rebuilder{types: map[string]schema.NamedType{}}
	// The built-in scalars and the introspection types are shared rather than
	// rebuilt: nothing can change them, and a rebuilt Int would no longer be
	// the Int that code written against this library refers to.
	for _, named := range schema.SpecifiedScalars {
		r.types[named.Name()] = named
	}
	for _, named := range schema.IntrospectionTypes {
		r.types[named.Name()] = named
	}
	return r
}

// rebuildableTypes returns the types of a schema that are rebuilt rather than
// shared.
func (r *rebuilder) rebuildableTypes(s *schema.Schema) []schema.NamedType {
	var out []schema.NamedType
	for _, t := range s.Types() {
		if t != nil && isRebuildable(t) {
			out = append(out, t)
		}
	}
	return out
}

// builtTypes returns the new types in the order they were made.
func (r *rebuilder) builtTypes() []schema.NamedType {
	out := make([]schema.NamedType, 0, len(r.order))
	for _, name := range r.order {
		if t := r.types[name]; t != nil {
			out = append(out, t)
		}
	}
	return out
}

// isRebuildable reports whether a type is one that gets rebuilt. Every schema
// holds the same built-in scalars and introspection types, so those are shared.
func isRebuildable(t schema.Type) bool {
	return !schema.IsSpecifiedScalarType(t) && !schema.IsIntrospectionType(t)
}

// redirect returns the same shape of type pointing at the new set.
func (r *rebuilder) redirect(t schema.Type) schema.Type {
	switch typ := t.(type) {
	case nil:
		return nil
	case *schema.List:
		inner := r.redirect(typ.OfType)
		if inner == nil {
			return nil
		}
		return schema.NewList(inner)
	case *schema.NonNull:
		inner := r.redirect(typ.OfType)
		if inner == nil {
			return nil
		}
		return schema.NewNonNull(inner)
	case schema.NamedType:
		return r.types[typ.Name()]
	default:
		return nil
	}
}

// rebuild recreates one type, letting the mapper say what its parts become.
//
// Everything a type refers to is deferred, because the types are made one at a
// time and a reference has to wait until all of them exist.
func (r *rebuilder) rebuild(t schema.NamedType, mapper SchemaMapper) schema.NamedType {
	// Each configuration comes from the type itself rather than being listed
	// out again here, so that a part of a type nobody thought of is carried
	// over rather than quietly dropped. What has to differ is the lists, which
	// are replaced with thunks: the types are made one at a time and a
	// reference has to wait until all of them exist.
	switch typ := t.(type) {
	case *schema.ScalarType:
		config := typ.ToConfig()
		if mapper.Scalar != nil {
			config = mapper.Scalar(config)
		}
		return schema.NewScalar(config)

	case *schema.ObjectType:
		config := typ.ToConfig()
		config.FieldsThunk = func() []*schema.Field {
			return r.redirectFields(typ, typ.Fields(), mapper)
		}
		config.InterfacesThunk = func() []schema.Declared[*schema.InterfaceType] {
			return r.redirectInterfaces(typ, typ.Interfaces(), mapper)
		}
		if mapper.Object != nil {
			config = mapper.Object(config)
		}
		return schema.NewObject(config)

	case *schema.InterfaceType:
		config := typ.ToConfig()
		config.FieldsThunk = func() []*schema.Field {
			return r.redirectFields(typ, typ.Fields(), mapper)
		}
		config.InterfacesThunk = func() []schema.Declared[*schema.InterfaceType] {
			return r.redirectInterfaces(typ, typ.Interfaces(), mapper)
		}
		if mapper.Interface != nil {
			config = mapper.Interface(config)
		}
		return schema.NewInterface(config)

	case *schema.UnionType:
		config := typ.ToConfig()
		config.TypesThunk = func() []schema.Declared[*schema.ObjectType] {
			return r.redirectMembers(typ, typ.Types(), mapper)
		}
		if mapper.Union != nil {
			config = mapper.Union(config)
		}
		return schema.NewUnion(config)

	case *schema.EnumType:
		config := typ.ToConfig()
		// A member names no types, so there is nothing to defer and the list
		// is settled here.
		values := typ.Values()
		if mapper.EnumValues != nil {
			values = mapper.EnumValues(typ, values)
		}
		config.Values = copyEnumValues(values)
		if mapper.Enum != nil {
			config = mapper.Enum(config)
		}
		return schema.NewEnum(config)

	case *schema.InputObjectType:
		config := typ.ToConfig()
		config.FieldsThunk = func() []*schema.InputField {
			return r.redirectInputFields(typ, typ.Fields(), mapper)
		}
		if mapper.InputObject != nil {
			config = mapper.InputObject(config)
		}
		return schema.NewInputObject(config)

	default:
		return nil
	}
}

// The helpers below recreate what a type held, pointing at the new set. A
// member whose type has gone is left out, and the schema validator reports the
// gap that leaves.

func (r *rebuilder) redirectFields(
	owner schema.NamedType,
	fields []*schema.Field,
	mapper SchemaMapper,
) []*schema.Field {
	if mapper.Fields != nil {
		fields = mapper.Fields(owner, fields)
	}
	out := make([]*schema.Field, 0, len(fields))
	for _, f := range fields {
		if f == nil {
			continue
		}
		fieldType := r.redirect(f.Type)
		if fieldType == nil {
			continue
		}
		out = append(out, schema.NewField(f.Name(), schema.FieldConfig{
			Description:       f.DescribedAs(),
			Type:              fieldType,
			Args:              r.redirectArguments(owner.Name()+"."+f.Name(), f.Args, mapper),
			Resolve:           f.Resolve,
			Subscribe:         f.Subscribe,
			DeprecationReason: f.DeprecationReason,
			ASTNode:           f.ASTNode,
			Extensions:        f.Extensions,
		}))
	}
	return out
}

func (r *rebuilder) redirectArguments(
	owner string,
	args []*schema.Argument,
	mapper SchemaMapper,
) []*schema.Argument {
	if mapper.Arguments != nil {
		args = mapper.Arguments(owner, args)
	}
	out := make([]*schema.Argument, 0, len(args))
	for _, a := range args {
		if a == nil {
			continue
		}
		argType := r.redirect(a.Type)
		if argType == nil {
			continue
		}
		out = append(out, schema.NewArgument(a.Name(), schema.ArgumentConfig{
			Description:       a.DescribedAs(),
			Type:              argType,
			Default:           a.Default,
			DeprecationReason: a.DeprecationReason,
			ASTNode:           a.ASTNode,
			Extensions:        a.Extensions,
		}))
	}
	return out
}

func (r *rebuilder) redirectInputFields(
	owner schema.NamedType,
	fields []*schema.InputField,
	mapper SchemaMapper,
) []*schema.InputField {
	if mapper.InputFields != nil {
		fields = mapper.InputFields(owner, fields)
	}
	out := make([]*schema.InputField, 0, len(fields))
	for _, f := range fields {
		if f == nil {
			continue
		}
		fieldType := r.redirect(f.Type)
		if fieldType == nil {
			continue
		}
		out = append(out, schema.NewInputField(f.Name(), schema.InputFieldConfig{
			Description:       f.DescribedAs(),
			Type:              fieldType,
			Default:           f.Default,
			DeprecationReason: f.DeprecationReason,
			ASTNode:           f.ASTNode,
			Extensions:        f.Extensions,
		}))
	}
	return out
}

func (r *rebuilder) redirectInterfaces(
	owner schema.NamedType,
	interfaces []schema.Declared[*schema.InterfaceType],
	mapper SchemaMapper,
) []schema.Declared[*schema.InterfaceType] {
	if mapper.Interfaces != nil {
		interfaces = mapper.Interfaces(owner, interfaces)
	}
	out := make([]schema.Declared[*schema.InterfaceType], 0, len(interfaces))
	for _, iface := range interfaces {
		if !iface.IsSet() {
			continue
		}
		// Carried over of the right kind or not, for the reason
		// redirectMembers gives.
		if replaced, ok := r.types[iface.Name()]; ok {
			out = append(out, schema.DeclareNamed[*schema.InterfaceType](replaced))
		}
	}
	return out
}

func (r *rebuilder) redirectMembers(
	owner schema.NamedType,
	members []schema.Declared[*schema.ObjectType],
	mapper SchemaMapper,
) []schema.Declared[*schema.ObjectType] {
	if mapper.UnionMembers != nil {
		members = mapper.UnionMembers(owner, members)
	}
	out := make([]schema.Declared[*schema.ObjectType], 0, len(members))
	for _, m := range members {
		if !m.IsSet() {
			continue
		}
		// Whatever the union named is carried over, of the right kind or not:
		// the new schema says what the old one said, and ValidateSchema has
		// the same to report about it.
		if replaced, ok := r.types[m.Name()]; ok {
			out = append(out, schema.DeclareNamed[*schema.ObjectType](replaced))
		}
	}
	return out
}

// redirectDirective recreates a directive pointing at the new set of types.
func (r *rebuilder) redirectDirective(d *schema.Directive, mapper SchemaMapper) *schema.Directive {
	// A built-in directive is shared rather than rebuilt, like the built-in
	// scalars: it names only shared types, and a rebuilt @skip would no longer
	// be the one code written against this library refers to. That also keeps
	// a printed schema from listing the directives every schema has.
	if schema.IsSpecifiedDirective(d) {
		return d
	}
	config := d.ToConfig()
	config.Args = r.redirectArguments("@"+d.Name(), d.Args, mapper)
	if mapper.Directive != nil {
		config = mapper.Directive(config)
	}
	return schema.NewDirective(config)
}

// copyEnumValues copies an enum's members, which name no types and so need no
// redirecting; they are copied rather than shared because a member records
// which enum it belongs to.
func copyEnumValues(values []*schema.EnumValue) []*schema.EnumValue {
	out := make([]*schema.EnumValue, 0, len(values))
	for _, v := range values {
		if v == nil {
			continue
		}
		out = append(out, schema.NewEnumValue(v.Name(), schema.EnumValueConfig{
			Description:       v.DescribedAs(),
			Value:             schema.InternalValue(v.Value),
			DeprecationReason: v.DeprecationReason,
			ASTNode:           v.ASTNode,
			Extensions:        v.Extensions,
		}))
	}
	return out
}

// nameOfType reads a root's name, coping with there being no such root.
func nameOfType(t *schema.ObjectType) string {
	if t == nil {
		return ""
	}
	return t.Name()
}

// isMissing reports whether a value in a schema's list is absent.
//
// A missing entry is often a nil pointer of a concrete type rather than an
// untyped nil, and putting one in an interface gives a value that is not equal
// to nil, so comparing will not find it.
func isMissing(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface:
		return rv.IsNil()
	default:
		return false
	}
}
