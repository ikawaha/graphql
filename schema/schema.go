package schema

import (
	"fmt"
	"github.com/ikawaha/graphql/value"
	"reflect"
	"slices"
	"sync"

	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/language"
)

// Config describes a schema.
type Config struct {
	Description value.Maybe[string]

	// Query, Mutation and Subscription are the types a request enters through.
	// A schema without a query type can hold definitions but cannot answer a
	// request.
	//
	// Each is a [NamedType] rather than an [ObjectType] because a schema may
	// name something else — nothing stops an author writing
	// `schema { query: SomeInput }` — and what is wrong with that is
	// [ValidateSchema]'s to report, as it is graphql-js's. An object type is
	// assigned here as it always was; [Schema.QueryType] and its siblings
	// still answer with one, and answer with nothing where the schema named
	// something a request cannot enter through.
	Query        NamedType
	Mutation     NamedType
	Subscription NamedType

	// Types names types that should be part of the schema even though nothing
	// reachable from the roots refers to them. An object type that only ever
	// appears as a member of an interface, resolved at run time, has to be
	// listed here or the schema will not know about it.
	Types []NamedType

	// Directives are the directives the schema allows. Leaving this empty
	// gives the schema the built-in ones; supplying any replaces them, so a
	// schema that wants both has to list them.
	Directives []*Directive

	// AssumeValid says the schema is sound without checking. A schema built
	// from one that was already checked — by a mapper, or by extending — has
	// nothing new to find, and a server that checked its schema at startup
	// need not have every request check it again.
	AssumeValid bool

	ASTNode           *language.SchemaDefinition
	ExtensionASTNodes []*language.SchemaExtension
	Extensions        map[string]any
}

// Schema is a complete GraphQL type system: the types a request may reach, the
// directives it may use, and the types it enters through.
//
// A schema is finished when it is built. Every deferred field list is resolved
// during construction, so nothing is computed lazily afterwards and a single
// schema can serve any number of requests at once without further
// synchronisation.
type Schema struct {
	description value.Maybe[string]

	query        NamedType
	mutation     NamedType
	subscription NamedType

	types       []NamedType
	typesByName map[string]NamedType

	directives      []*Directive
	directiveByName map[string]*Directive

	// implementations maps an interface's name to the types that implement it.
	implementations map[string]*Implementations

	// collectErrors records problems found while gathering the types, such as
	// two types sharing a name. Building a schema does not fail; the problems
	// are reported by [ValidateSchema] along with everything else.
	collectErrors []error

	// What is wrong with the schema, worked out once. A schema does not change
	// after it is built, so the answer cannot either, and every request would
	// otherwise pay for the same walk. graphql-js remembers it the same way.
	checked  sync.Once
	problems []*gqlerror.Error

	ASTNode           *language.SchemaDefinition
	ExtensionASTNodes []*language.SchemaExtension
	Extensions        map[string]any
}

// Implementations are the types that implement an interface.
type Implementations struct {
	// Objects are the object types that implement it.
	Objects []*ObjectType
	// Interfaces are the other interfaces that implement it.
	Interfaces []*InterfaceType
}

// New returns a schema holding everything reachable from the given roots.
//
// Building the schema walks every type it can reach, which resolves the
// deferred field lists that let types refer to one another. Once that walk is
// done the schema no longer changes.
func New(config Config) *Schema {
	s := &Schema{
		description:       config.Description,
		query:             presentType(config.Query),
		mutation:          presentType(config.Mutation),
		subscription:      presentType(config.Subscription),
		directives:        slices.Clone(config.Directives),
		ASTNode:           config.ASTNode,
		ExtensionASTNodes: config.ExtensionASTNodes,
		Extensions:        config.Extensions,
	}
	if s.directives == nil {
		s.directives = SpecifiedDirectives
	}

	s.directiveByName = make(map[string]*Directive, len(s.directives))
	for _, d := range s.directives {
		if d == nil {
			continue
		}
		if _, exists := s.directiveByName[d.name]; !exists {
			s.directiveByName[d.name] = d
		} else {
			s.collectErrors = append(s.collectErrors,
				fmt.Errorf("schema contains more than one directive named %q", d.name))
		}
	}

	s.collectTypes(config.Types)
	s.indexImplementations()
	if config.AssumeValid {
		s.checked.Do(func() {})
	}
	return s
}

// collectTypes gathers every named type the schema can reach.
func (s *Schema) collectTypes(extra []NamedType) {
	s.typesByName = make(map[string]NamedType)

	roots := make([]Type, 0, 8)
	for _, root := range []NamedType{s.query, s.mutation, s.subscription} {
		if root != nil {
			roots = append(roots, root)
		}
	}
	for _, d := range s.directives {
		if d == nil {
			continue
		}
		for _, arg := range d.Args {
			if arg != nil && arg.Type != nil {
				roots = append(roots, arg.Type)
			}
		}
	}

	// Every schema can describe itself, so the introspection types belong to
	// it whether or not anything in the schema refers to them.
	for _, t := range IntrospectionTypes {
		roots = append(roots, t)
	}

	seen := make(map[NamedType]bool)

	// The types the caller listed come first, each followed by whatever it
	// refers to. That order is the one an author wrote or a tool chose, and it
	// is what the schema prints in, so honouring it is what makes a printed
	// schema follow its source and what lets a schema be sorted.
	//
	// Holding all of them back before walking any is what keeps a listed type
	// in its place: without it, the first listed type to mention a later one
	// would drag it forward.
	for _, t := range extra {
		if !isAbsentType(t) {
			seen[t] = true
		}
	}
	for _, t := range extra {
		if isAbsentType(t) {
			continue
		}
		delete(seen, t)
		s.collectFrom(t, seen)
	}

	for _, root := range roots {
		s.collectFrom(root, seen)
	}
}

// collectFrom adds a type and everything it refers to.
//
// Reading a type's fields here is what forces its deferred field list to be
// built, so this walk is also what makes the schema finished.
func (s *Schema) collectFrom(t Type, seen map[NamedType]bool) {
	named := NamedTypeOf(t)
	if isAbsentType(named) || seen[named] {
		return
	}
	seen[named] = true

	if existing, clash := s.typesByName[named.Name()]; clash && existing != named {
		s.collectErrors = append(s.collectErrors,
			fmt.Errorf("schema contains more than one type named %q", named.Name()))
		return
	}
	s.typesByName[named.Name()] = named
	s.types = append(s.types, named)

	switch n := named.(type) {
	case *UnionType:
		// Whatever the union named is part of the schema, even where it is
		// not a type a value could turn out to be: a client asking about a
		// broken schema is told what it says.
		for _, member := range n.Types() {
			if named := member.Named(); named != nil {
				s.collectFrom(named, seen)
			}
		}
	case *ObjectType:
		s.collectFromFields(n.Interfaces(), n.Fields(), seen)
	case *InterfaceType:
		s.collectFromFields(n.Interfaces(), n.Fields(), seen)
	case *InputObjectType:
		for _, f := range n.Fields() {
			if f != nil {
				s.collectFrom(f.Type, seen)
			}
		}
	}
}

// collectFromFields walks the interfaces and fields an object or interface
// declares.
func (s *Schema) collectFromFields(interfaces []Declared[*InterfaceType], fields []*Field, seen map[NamedType]bool) {
	// Whatever an implements clause named is part of the schema, of the right
	// kind or not, for the reason a union's members are.
	for _, iface := range interfaces {
		if named := iface.Named(); named != nil {
			s.collectFrom(named, seen)
		}
	}
	for _, f := range fields {
		if f == nil {
			continue
		}
		s.collectFrom(f.Type, seen)
		for _, arg := range f.Args {
			if arg != nil {
				s.collectFrom(arg.Type, seen)
			}
		}
	}
}

// indexImplementations records, for each interface, the types that implement
// it, so that answering "what could this abstract type be" is a lookup rather
// than a search.
func (s *Schema) indexImplementations() {
	s.implementations = make(map[string]*Implementations)

	for _, t := range s.types {
		switch n := t.(type) {
		case *ObjectType:
			for _, declared := range n.Interfaces() {
				iface, isInterface := declared.Get()
				if !isInterface {
					continue
				}
				impl := s.implementationsFor(iface.Name())
				impl.Objects = append(impl.Objects, n)
			}
		case *InterfaceType:
			// An interface is listed even when nothing implements it, so that
			// asking about it gives an empty answer rather than nothing at all.
			s.implementationsFor(n.Name())
			for _, declared := range n.Interfaces() {
				iface, isInterface := declared.Get()
				if !isInterface {
					continue
				}
				impl := s.implementationsFor(iface.Name())
				impl.Interfaces = append(impl.Interfaces, n)
			}
		}
	}
}

// implementationsFor returns the record for an interface, creating it if it is
// not there yet.
func (s *Schema) implementationsFor(name string) *Implementations {
	impl, ok := s.implementations[name]
	if !ok {
		impl = &Implementations{}
		s.implementations[name] = impl
	}
	return impl
}

// A nil *Schema is a schema that knows nothing: every lookup below answers as
// it would for a name the schema does not have. That is a real state rather
// than a mistake, because SDL can be checked before there is a schema to check
// it against, and it keeps every reader from having to guard for itself.

// Description is the documentation written for the schema, if any.
func (s *Schema) Description() string { return s.DescribedAs().Or("") }

// DescribedAs is the documentation written for the schema, telling one written
// as the empty string from none at all. graphql-js keeps the two apart and
// prints and describes them differently.
func (s *Schema) DescribedAs() value.Maybe[string] {
	if s == nil {
		return value.Nothing[string]()
	}
	return s.description
}

// QueryType is the type a query enters through, or nil if the schema has none
// a query could enter through.
//
// A schema that names something other than an object type is unsound and
// [ValidateSchema] says so; until then this answers with nothing, since there
// is no object to enter. [Schema.DeclaredRootType] answers with whatever was
// named.
func (s *Schema) QueryType() *ObjectType {
	return asObjectType(s.DeclaredRootType(language.OperationQuery))
}

// MutationType is the type a mutation enters through, or nil.
func (s *Schema) MutationType() *ObjectType {
	return asObjectType(s.DeclaredRootType(language.OperationMutation))
}

// SubscriptionType is the type a subscription enters through, or nil.
func (s *Schema) SubscriptionType() *ObjectType {
	return asObjectType(s.DeclaredRootType(language.OperationSubscription))
}

// RootType returns the type an operation of the given kind enters through, or
// nil if the schema does not support that kind of operation.
func (s *Schema) RootType(operation language.OperationType) *ObjectType {
	return asObjectType(s.DeclaredRootType(operation))
}

// DeclaredRootType is the type the schema names for an operation, whatever
// kind it turns out to be.
//
// [Schema.RootType] answers the narrower question — what an operation can
// actually enter through — and so answers with nothing where the schema named
// something that is not an object type. This is what prints such a schema,
// describes it to a client, and reports what is wrong with it.
func (s *Schema) DeclaredRootType(operation language.OperationType) NamedType {
	if s == nil {
		return nil
	}
	switch operation {
	case language.OperationQuery:
		return s.query
	case language.OperationMutation:
		return s.mutation
	case language.OperationSubscription:
		return s.subscription
	default:
		return nil
	}
}

// asObjectType narrows a named type to an object type, answering with nothing
// for anything else.
func asObjectType(t NamedType) *ObjectType {
	if o, isObject := t.(*ObjectType); isObject {
		return o
	}
	return nil
}

// presentType turns a type that is not there into a nil interface, so that a
// nil *ObjectType assigned to a NamedType field does not read as present.
func presentType(t NamedType) NamedType {
	if isAbsentType(t) {
		return nil
	}
	return t
}

// Types returns every named type in the schema, in the order they were reached
// from the roots. Callers must not modify the returned slice.
func (s *Schema) Types() []NamedType {
	if s == nil {
		return nil
	}
	return s.types
}

// Type returns the type with the given name, or nil if the schema has none.
func (s *Schema) Type(name string) NamedType {
	if s == nil {
		return nil
	}
	return s.typesByName[name]
}

// Directives returns the directives the schema allows. Callers must not modify
// the returned slice.
func (s *Schema) Directives() []*Directive {
	if s == nil {
		return nil
	}
	return s.directives
}

// Directive returns the directive with the given name, or nil.
func (s *Schema) Directive(name string) *Directive {
	if s == nil {
		return nil
	}
	return s.directiveByName[name]
}

// Implementations returns the types that implement an interface. It returns a
// zero value for an interface the schema does not know about.
func (s *Schema) Implementations(iface *InterfaceType) Implementations {
	if s == nil || iface == nil {
		return Implementations{}
	}
	if impl, ok := s.implementations[iface.Name()]; ok {
		return *impl
	}
	return Implementations{}
}

// PossibleTypes returns the object types a value of an abstract type could
// turn out to be.
func (s *Schema) PossibleTypes(abstract AbstractType) []*ObjectType {
	if s == nil {
		return nil
	}
	switch t := abstract.(type) {
	case *UnionType:
		// Only the members that are object types: nothing else is something a
		// value could turn out to be, whatever the union named.
		members := t.Types()
		out := make([]*ObjectType, 0, len(members))
		for _, member := range members {
			if o, isObject := member.Get(); isObject {
				out = append(out, o)
			}
		}
		return out
	case *InterfaceType:
		if impl, ok := s.implementations[t.Name()]; ok {
			return impl.Objects
		}
	}
	return nil
}

// IsSubType reports whether a type is one an abstract type could turn out to
// be. An interface counts as a subtype of another interface it implements,
// which is what lets a fragment on one spread into the other.
func (s *Schema) IsSubType(abstract AbstractType, maybeSubType NamedType) bool {
	if s == nil || abstract == nil || maybeSubType == nil {
		return false
	}
	switch t := abstract.(type) {
	case *UnionType:
		obj, ok := maybeSubType.(*ObjectType)
		return ok && t.HasType(obj)
	case *InterfaceType:
		impl, ok := s.implementations[t.Name()]
		if !ok {
			return false
		}
		switch candidate := maybeSubType.(type) {
		case *ObjectType:
			for _, o := range impl.Objects {
				if o == candidate {
					return true
				}
			}
		case *InterfaceType:
			for _, i := range impl.Interfaces {
				if i == candidate {
					return true
				}
			}
		}
	}
	return false
}

// Field returns a field of a composite type, or nil if there is none.
//
// The three meta-fields are answered here too, because a document may ask for
// them where no type declares them: __typename of any composite type, and
// __schema and __type of the query root.
func (s *Schema) Field(parent CompositeType, name string) *Field {
	switch name {
	case TypeNameMetaField.Name():
		if parent != nil {
			return TypeNameMetaField
		}
		return nil
	case SchemaMetaField.Name(), TypeMetaField.Name():
		// These two describe the schema itself, so they belong only to the
		// type a query enters through.
		if query := s.QueryType(); query != nil && parent != nil && parent == CompositeType(query) {
			if name == SchemaMetaField.Name() {
				return SchemaMetaField
			}
			return TypeMetaField
		}
		return nil
	}

	switch t := parent.(type) {
	case *ObjectType:
		return t.Field(name)
	case *InterfaceType:
		// A union has no fields of its own beyond the meta-fields above.
		return t.Field(name)
	default:
		return nil
	}
}

// isAbsentType reports whether a type is missing.
//
// A missing type is often a nil pointer of a concrete type rather than an
// untyped nil, and putting one in an interface produces a value that is not
// equal to nil but holds nothing. Comparing against nil alone would let it
// through to be dereferenced.
func isAbsentType(t Type) bool {
	if t == nil {
		return true
	}
	v := reflect.ValueOf(t)
	return v.Kind() == reflect.Pointer && v.IsNil()
}
