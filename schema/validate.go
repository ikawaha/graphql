package schema

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/value"
)

// ValidateSchema checks that a schema is well formed and returns everything
// wrong with it.
//
// This is where the checking lives. Building a type or a schema never fails,
// so that assembling one reads as a description rather than a sequence of
// error handling; the price is that nothing is verified until this is called.
// A server should call it once at start-up.
//
// An empty result means the schema is sound.
func ValidateSchema(s *Schema) []*gqlerror.Error {
	if s == nil {
		return []*gqlerror.Error{gqlerror.New("Expected a schema, got nothing.")}
	}
	// Worked out once: a schema does not change after it is built, and an
	// executor asks this of every request. A schema told to assume it is
	// sound has already had this run with nothing in it.
	s.checked.Do(func() { s.problems = validateSchema(s) })
	return s.problems
}

func validateSchema(s *Schema) []*gqlerror.Error {
	v := &schemaValidator{schema: s}

	// Problems noticed while the schema was being assembled, such as two types
	// sharing a name, are reported here rather than at the point they were
	// found.
	for _, err := range s.collectErrors {
		v.report(nil, "%s", err)
	}

	v.validateRootTypes()
	v.validateDirectives()
	v.validateTypes()
	return v.errors
}

// AssertValidSchema returns an error describing everything wrong with a
// schema, or nil if it is sound.
func AssertValidSchema(s *Schema) error {
	errs := ValidateSchema(s)
	if len(errs) == 0 {
		return nil
	}
	// What is wrong with it, and nothing else: graphql-js joins the messages
	// the same way, and an executor puts this straight into a response.
	messages := make([]string, len(errs))
	for i, err := range errs {
		messages[i] = err.Message
	}
	return errors.New(strings.Join(messages, "\n\n"))
}

// schemaValidator gathers the problems found in one schema.
type schemaValidator struct {
	schema *Schema
	errors []*gqlerror.Error
	// satisfiable says, for each input object, whether a value can be written
	// for it. It is worked out once, the first time it is wanted.
	satisfiable map[*InputObjectType]bool
	// walkedForCycles keeps the input objects already looked at for cycles, so
	// that one cycle is reported once rather than once for every type it
	// passes through.
	walkedForCycles map[*InputObjectType]bool
	// walkedDefaults keeps the fields whose defaults have already been
	// followed, for the same reason.
	walkedDefaults map[string]bool
}

// report records a problem, blaming the part of the schema's source
// responsible for it.
//
// A schema is often written as SDL, and a complaint about one that does not say
// where is much less use than one that does. Where a schema was built in Go
// there is no source to point at, and the nodes are simply absent.
func (v *schemaValidator) report(nodes []language.Node, format string, args ...any) {
	message := format
	if len(args) > 0 {
		message = fmt.Sprintf(format, args...)
	}
	v.errors = append(v.errors, gqlerror.New(message, gqlerror.WithNodes(present(nodes)...)))
}

// present keeps the nodes that are really there, since a schema built in Go
// has none and a malformed document may be missing one.
func present(nodes []language.Node) []language.Node {
	out := make([]language.Node, 0, len(nodes))
	for _, n := range nodes {
		if !isAbsentNode(n) {
			out = append(out, n)
		}
	}
	return out
}

// isAbsentNode reports whether a node is missing.
//
// A missing node is often a nil pointer of a concrete type rather than an
// untyped nil, and putting one in an interface gives a value that is not equal
// to nil.
func isAbsentNode(n language.Node) bool {
	if n == nil {
		return true
	}
	rv := reflect.ValueOf(n)
	return rv.Kind() == reflect.Pointer && rv.IsNil()
}

// validateRootTypes checks the types a request enters through.
func (v *schemaValidator) validateRootTypes() {
	if v.schema.DeclaredRootType(language.OperationQuery) == nil {
		v.report(at(v.schema.ASTNode), "Query root type must be provided.")
	}

	// Two operations entering through the same type would make the schema
	// ambiguous about which one a request is. A type used for more than two is
	// still one thing wrong, so it is said once naming all of them.
	var order []*ObjectType
	sharing := map[*ObjectType][]language.OperationType{}
	for _, operation := range []language.OperationType{
		language.OperationQuery,
		language.OperationMutation,
		language.OperationSubscription,
	} {
		declared := v.schema.DeclaredRootType(operation)
		if declared == nil {
			continue
		}
		object, isObject := declared.(*ObjectType)
		if !isObject {
			// A request enters through an object and nothing else. The schema
			// holds whatever was named — graphql-js does too — and what is
			// wrong with it is said here rather than by refusing to build it.
			says := "%s root type must be Object type if provided, it cannot be %s."
			if operation == language.OperationQuery {
				says = "%s root type must be Object type, it cannot be %s."
			}
			v.report(rootTypeNodes(v.schema, operation, declared),
				says, capitalized(operation), declared.String())
			continue
		}
		if _, seen := sharing[object]; !seen {
			order = append(order, object)
		}
		sharing[object] = append(sharing[object], operation)
	}
	for _, typ := range order {
		operations := sharing[typ]
		if len(operations) < 2 {
			continue
		}
		named := make([]string, len(operations))
		blamed := make([]language.Node, len(operations))
		for i, operation := range operations {
			named[i] = string(operation)
			blamed[i] = operationTypeNode(v.schema, operation)
		}
		v.report(blamed, "All root types must be different, %q type is used as %s root types.",
			typ.Name(), joinList("and", named))
	}
}

// rootTypeNodes says where to point a complaint about a root type: at the
// place the schema named it, or, where the type was taken up by its
// conventional name and never written in a schema definition, at the type.
func rootTypeNodes(s *Schema, operation language.OperationType, declared NamedType) []language.Node {
	if named := operationTypeNode(s, operation); named != nil {
		return at(named)
	}
	if defined := definitionNodes(declared); len(defined) > 0 {
		return defined[:1]
	}
	return nil
}

// capitalized names an operation as a message opens with it.
func capitalized(operation language.OperationType) string {
	name := string(operation)
	if name == "" {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// validateDirectives checks the directives a schema allows.
func (v *schemaValidator) validateDirectives() {
	for _, d := range v.schema.Directives() {
		if d == nil {
			v.report(nil, "Expected a directive, got nothing.")
			continue
		}
		v.validateName(d.Name(), "@"+d.Name(), d.ASTNode)
		if len(d.Locations) == 0 {
			v.report(at(d.ASTNode), "Directive @%s must include 1 or more locations.", d.Name())
		}
		for _, arg := range d.Args {
			if arg == nil {
				continue
			}
			v.validateArgument(arg, "@"+d.Name())
		}
	}
}

// validateArgument checks one argument of a field or a directive.
func (v *schemaValidator) validateArgument(arg *Argument, owner string) {
	where := owner + "(" + arg.Name() + ":)"
	v.validateName(arg.Name(), where, arg.ASTNode)

	if !IsInputType(arg.Type) {
		v.report(at(typeNodeOf(arg.ASTNode)), "The type of %s must be Input Type but got: %s.",
			where, describeTypeForError(arg.Type))
	}
	// A caller has to supply a required argument, so deprecating it leaves
	// them no way to comply without using something deprecated.
	if IsRequiredArgument(arg) && arg.IsDeprecated() {
		v.report(at(deprecatedNode(arg.ASTNode), typeNodeOf(arg.ASTNode)),
			"Required argument %s cannot be deprecated.", where)
	}

	v.validateDefaultValue(where, arg.Default, arg.Type, arg.ASTNode)
}

// validateDefaultValue checks that a default fits the type it is a default
// for.
//
// A default written in SDL is a literal and one given in Go is a value, and
// each is checked by the half of the checking that reads that form. Nothing
// else in the schema is checked against a type this way: a default is the one
// place a schema holds a value rather than describing one.
func (v *schemaValidator) validateDefaultValue(
	where string,
	def value.Maybe[DefaultInput],
	t Type,
	node *language.InputValueDefinition,
) {
	input, has := def.Get()
	if !has {
		return
	}
	var written language.Node
	if node != nil {
		written = node.DefaultValue
	}

	if input.Literal != nil {
		for _, why := range ValidateInputLiteral(input.Literal, t, VariableValues{}) {
			v.report(at(blameForDefault(why.Cause, why.Node, written)),
				"%s has invalid default value%s: %s", where, atPath(why.Path), why.Message)
		}
		return
	}
	why := ValidateInputValue(input.Value, t)
	if len(why) == 0 {
		return
	}

	// A default that does not fit may be one written in the form a resolver
	// receives rather than the form a caller supplies. Where turning it back
	// gives something that does fit, saying so is more use than repeating
	// what is wrong with each part of it.
	if fixed, err := uncoerceDefaultValue(input.Value, t); err == nil {
		if len(ValidateInputValue(fixed, t)) == 0 {
			v.report(at(written), "%s has invalid default value: %s. Did you mean: %s?",
				where, value.Describe(input.Value), value.Describe(fixed))
			return
		}
	}

	for _, why := range why {
		v.report(at(written),
			"%s has invalid default value%s: %s", where, atPath(why.Path), why.Message)
	}
}

// uncoerceDefaultValue turns a default held in the internal form into the
// external one, which is the reverse of what a caller's value goes through on
// its way in.
//
// A default is meant to be written the way a caller would write it, but a
// schema built in code can just as easily hold the form a resolver would
// receive: the name of an enum member is what a caller writes, while the value
// standing behind that name is what a resolver gets. Where the two differ the
// default is refused, and this is the only way to tell whether the refusal is
// that mistake or a real one.
//
// An error means the value cannot be turned back at all, which is answer
// enough: there is no fix to suggest.
func uncoerceDefaultValue(v any, t Type) (any, error) {
	if nonNull, isNonNull := t.(*NonNull); isNonNull {
		return uncoerceDefaultValue(v, nonNull.OfType)
	}
	if v == nil {
		return nil, nil
	}

	switch typ := t.(type) {
	case *List:
		items, isList := asList(v)
		if !isList {
			// A lone value stands for a list of one on the way in, so it does
			// on the way back out.
			item, err := uncoerceDefaultValue(v, typ.OfType)
			if err != nil {
				return nil, err
			}
			return []any{item}, nil
		}
		out := make([]any, len(items))
		for i, item := range items {
			back, err := uncoerceDefaultValue(item, typ.OfType)
			if err != nil {
				return nil, err
			}
			out[i] = back
		}
		return out, nil

	case *InputObjectType:
		fields, isObject := asObject(v)
		if !isObject {
			return nil, fmt.Errorf("expected an object for %s, found: %s", typ.Name(), value.Describe(v))
		}
		out := make(map[string]any, len(fields))
		for name, given := range fields {
			field := typ.Field(name)
			if field == nil {
				return nil, fmt.Errorf("%s has no field %q", typ.Name(), name)
			}
			back, err := uncoerceDefaultValue(given, field.Type)
			if err != nil {
				return nil, err
			}
			out[name] = back
		}
		return out, nil

	case *ScalarType:
		// Output coercion is what input coercion undoes, for every scalar that
		// converts at all.
		if typ.CoerceOutputValue == nil {
			return v, nil
		}
		coerced, err := typ.CoerceOutputValue(v)
		if err != nil {
			return nil, err
		}
		held, represented := coerced.Get()
		if !represented {
			return nil, fmt.Errorf("%s cannot represent the value %s",
				typ.Name(), value.Describe(v))
		}
		return held, nil

	case *EnumType:
		member := typ.ValueFor(v)
		if member == nil {
			return nil, fmt.Errorf("%s has no member with the value %s", typ.Name(), value.Describe(v))
		}
		return member.Name(), nil
	}

	return nil, fmt.Errorf("%s is not an input type", describeTypeName(t))
}

// blameForDefault picks where a complaint about a default points.
//
// A type that refused the value may have named the part of the document
// itself, and where it named nowhere there is nothing better than the default
// as a whole. Any other refusal is simply what a type said while the literal
// was being read, so the literal is the place.
func blameForDefault(cause error, literal, written language.Node) language.Node {
	var raised *gqlerror.Error
	if cause != nil && errors.As(cause, &raised) {
		if len(raised.Nodes) > 0 {
			return raised.Nodes[0]
		}
		return written
	}
	if literal != nil {
		return literal
	}
	return written
}

// atPath renders where inside a value a complaint is about, or nothing when it
// is about the value as a whole.
func atPath(path []any) string {
	if len(path) == 0 {
		return ""
	}
	var b strings.Builder
	for _, step := range path {
		switch key := step.(type) {
		case string:
			b.WriteString("." + key)
		case int:
			b.WriteString("[" + strconv.Itoa(key) + "]")
		}
	}
	return " at " + b.String()
}

// validateName rejects a name reserved for introspection.
func (v *schemaValidator) validateName(name, where string, nodes ...language.Node) {
	if err := ValidateName(name); err != nil {
		v.report(nodes, "%s: %s.", where, err)
		return
	}
	if strings.HasPrefix(name, "__") {
		v.report(nodes, "Name %q must not begin with %q, which is reserved by GraphQL introspection.", name, "__")
	}
}

// validateTypes checks every type in the schema.
func (v *schemaValidator) validateTypes() {
	for _, t := range v.schema.Types() {
		// The introspection types are the one place names may begin with two
		// underscores, since that prefix is reserved for them.
		if IsIntrospectionType(t) {
			continue
		}
		v.validateName(t.Name(), "Type "+t.Name(), definitionNodes(t)...)

		switch n := t.(type) {
		case *ObjectType:
			v.validateFields(n, n.Fields())
			v.validateInterfaces(n, n.Interfaces())
		case *InterfaceType:
			v.validateFields(n, n.Fields())
			v.validateInterfaces(n, n.Interfaces())
		case *UnionType:
			v.validateUnionMembers(n)
		case *EnumType:
			v.validateEnumValues(n)
		case *InputObjectType:
			v.validateInputFields(n)
			v.validateInputObjectCycles(n)
			v.validateDefaultValueCycles(n)
		}
	}
}

// validateFields checks the fields of an object or interface type.
func (v *schemaValidator) validateFields(owner NamedType, fields []*Field) {
	if len(fields) == 0 {
		v.report(definitionNodes(owner), "Type %s must define one or more fields.", owner.Name())
		return
	}
	// A name written twice is one field by the time the type holds it, as it
	// is in graphql-js, so there is nothing here to say about one.
	for _, f := range fields {
		if f == nil {
			v.report(definitionNodes(owner), "Type %s has a field that is missing.", owner.Name())
			continue
		}
		where := owner.Name() + "." + f.Name()
		v.validateName(f.Name(), where, f.ASTNode)
		if !IsOutputType(f.Type) {
			v.report(at(typeNodeOf(f.ASTNode)), "The type of %s must be Output Type but got: %s.",
				where, describeTypeForError(f.Type))
		}

		for _, arg := range f.Args {
			if arg == nil {
				continue
			}
			v.validateArgument(arg, where)
		}
	}
}

// fielded is what an object and an interface have in common for the purpose of
// checking that one implements the other.
type fielded interface {
	NamedType
	Field(name string) *Field
	Fields() []*Field
	Interfaces() []Declared[*InterfaceType]
}

// validateInterfaces checks the interfaces a type declares it implements.
func (v *schemaValidator) validateInterfaces(self fielded, interfaces []Declared[*InterfaceType]) {
	owner := self.Name()
	// What the type declares, whatever else is wrong with the list. An
	// ancestor already among them is one the type implements, so there is
	// nothing to say about it.
	declared := make(map[*InterfaceType]bool, len(interfaces))
	for _, held := range interfaces {
		if iface, isInterface := held.Get(); isInterface {
			declared[iface] = true
		}
	}
	seen := map[string]bool{}
	for _, held := range interfaces {
		if !held.IsSet() {
			v.report(definitionNodes(self), "Type %s declares an interface that is missing.", owner)
			continue
		}
		// Only an interface can be implemented. graphql-js says this here
		// rather than refusing to build the schema, and so does this.
		iface, isInterface := held.Get()
		if !isInterface {
			v.report(implementsNodes(self, held.Name()),
				"Type %s must only implement Interface types, it cannot implement %s.",
				owner, held.Name())
			continue
		}
		if iface == Type(self) {
			v.report(implementsNodes(self, owner),
				"Type %s cannot implement itself because it would create a circular reference.", owner)
			continue
		}
		if seen[iface.Name()] {
			v.report(implementsNodes(self, iface.Name()),
				"Type %s can only implement %s once.", owner, iface.Name())
			continue
		}
		seen[iface.Name()] = true

		// Implementing an interface means implementing what it implements
		// too, so that anything written against the ancestor still works.
		// Only an interface that got this far is asked about its own, since
		// one already reported has nothing more to say.
		for _, heldAncestor := range iface.Interfaces() {
			ancestor, isInterface := heldAncestor.Get()
			if !isInterface || declared[ancestor] {
				continue
			}
			// Both ends of the chain are worth pointing at: where the
			// interface declares the ancestor, and where this type declares
			// the interface.
			blamed := append(implementsNodes(iface, ancestor.Name()),
				implementsNodes(self, iface.Name())...)
			if ancestor == Type(self) {
				v.report(blamed,
					"Type %s cannot implement %s because it would create a circular reference.",
					owner, iface.Name())
				continue
			}
			v.report(blamed, "Type %s must implement %s because it is implemented by %s.",
				owner, ancestor.Name(), iface.Name())
		}

		v.validateImplements(self, iface)
	}
}

// validateImplements checks that a type really provides what an interface
// promises.
func (v *schemaValidator) validateImplements(self fielded, iface *InterfaceType) {
	for _, promised := range iface.Fields() {
		if promised == nil {
			continue
		}
		where := self.Name() + "." + promised.Name()
		provided := self.Field(promised.Name())
		if provided == nil {
			v.report(append(at(promised.ASTNode), definitionNodes(self)...),
				"Interface field %s.%s expected but %s does not provide it.",
				iface.Name(), promised.Name(), self.Name())
			continue
		}

		// The field may narrow the promised type but not widen it.
		if !IsTypeSubTypeOf(v.schema, provided.Type, promised.Type) {
			v.report(at(typeNodeOf(promised.ASTNode), typeNodeOf(provided.ASTNode)),
				"Interface field %s.%s expects type %s but %s is type %s.",
				iface.Name(), promised.Name(), describeTypeForError(promised.Type),
				where, describeTypeForError(provided.Type))
		}

		for _, promisedArg := range promised.Args {
			if promisedArg == nil {
				continue
			}
			providedArg := provided.Arg(promisedArg.Name())
			if providedArg == nil {
				v.report(at(promisedArg.ASTNode, provided.ASTNode),
					"Interface field argument %s.%s(%s:) expected but %s does not provide it.",
					iface.Name(), promised.Name(), promisedArg.Name(), where)
				continue
			}
			// An argument's type must match exactly. Narrowing it would reject
			// a value the interface says is allowed.
			if !IsEqualType(providedArg.Type, promisedArg.Type) {
				v.report(at(typeNodeOf(promisedArg.ASTNode), typeNodeOf(providedArg.ASTNode)),
					"Interface field argument %s.%s(%s:) expects type %s but %s(%s:) is type %s.",
					iface.Name(), promised.Name(), promisedArg.Name(),
					describeTypeForError(promisedArg.Type), where, promisedArg.Name(),
					describeTypeForError(providedArg.Type))
			}
		}

		// An argument the interface did not ask for must be optional, or a
		// caller writing against the interface could not call the field.
		for _, extra := range provided.Args {
			if extra == nil || promised.Arg(extra.Name()) != nil {
				continue
			}
			if IsRequiredArgument(extra) {
				v.report(at(extra.ASTNode, promised.ASTNode),
					"Argument %q must not be required type %q if not provided by the Interface field %q.",
					where+"("+extra.Name()+":)", describeTypeForError(extra.Type),
					iface.Name()+"."+promised.Name())
			}
		}

		// Deprecating an implementation of a field the interface still
		// recommends leaves a caller nowhere to go.
		if provided.IsDeprecated() && !promised.IsDeprecated() {
			v.report(at(deprecatedNode(provided.ASTNode), typeNodeOf(provided.ASTNode)),
				"Interface field %s.%s is not deprecated, so implementation field %s must not be deprecated.",
				iface.Name(), promised.Name(), where)
		}
	}
}

// validateUnionMembers checks the members of a union.
func (v *schemaValidator) validateUnionMembers(u *UnionType) {
	members := u.Types()
	if len(members) == 0 {
		v.report(definitionNodes(u), "Union type %s must define one or more member types.", u.Name())
		return
	}
	seen := map[string]bool{}
	for _, member := range members {
		if !member.IsSet() {
			v.report(definitionNodes(u), "Union type %s has a member that is missing.", u.Name())
			continue
		}
		if seen[member.Name()] {
			v.report(memberNodes(u, member.Name()),
				"Union type %s can only include type %s once.", u.Name(), member.Name())
			continue
		}
		seen[member.Name()] = true
		// A value turns out to be an object and nothing else, so a union of
		// anything else could never be answered. graphql-js says this here
		// rather than refusing to build the schema, and so does this.
		if _, isObject := member.Get(); !isObject {
			v.report(memberNodes(u, member.Name()),
				"Union type %s can only include Object types, it cannot include %s.",
				u.Name(), member.Name())
		}
	}
}

// validateEnumValues checks the members of an enum.
func (v *schemaValidator) validateEnumValues(e *EnumType) {
	values := e.Values()
	if len(values) == 0 {
		v.report(definitionNodes(e), "Enum type %s must define one or more values.", e.Name())
		return
	}
	seen := map[string]bool{}
	for _, value := range values {
		if value == nil {
			v.report(definitionNodes(e), "Enum type %s has a value that is missing.", e.Name())
			continue
		}
		if seen[value.Name()] {
			v.report(at(value.ASTNode), "Enum type %s can only include value %s once.", e.Name(), value.Name())
			continue
		}
		seen[value.Name()] = true
		if err := ValidateEnumValueName(value.Name()); err != nil {
			v.report(at(value.ASTNode), "%s.%s: %s.", e.Name(), value.Name(), err)
			continue
		}
		if strings.HasPrefix(value.Name(), "__") {
			v.report(at(value.ASTNode),
				"Name %q must not begin with %q, which is reserved by GraphQL introspection.",
				value.Name(), "__")
		}
	}
}

// validateInputFields checks the fields of an input object.
func (v *schemaValidator) validateInputFields(in *InputObjectType) {
	fields := in.Fields()
	if len(fields) == 0 {
		v.report(definitionNodes(in), "Input Object type %s must define one or more fields.", in.Name())
		return
	}
	for _, f := range fields {
		if f == nil {
			v.report(definitionNodes(in), "Input Object type %s has a field that is missing.", in.Name())
			continue
		}
		where := in.Name() + "." + f.Name()
		v.validateName(f.Name(), where, f.ASTNode)
		if !IsInputType(f.Type) {
			v.report(at(typeNodeOf(f.ASTNode)), "The type of %s must be Input Type but got: %s.",
				where, describeTypeForError(f.Type))
		}
		if IsRequiredInputField(f) && f.IsDeprecated() {
			v.report(at(deprecatedNode(f.ASTNode), typeNodeOf(f.ASTNode)),
				"Required input field %s cannot be deprecated.", where)
		}

		v.validateDefaultValue(where, f.Default, f.Type, f.ASTNode)

		// Exactly one field of a oneOf input object is supplied, so no field
		// can be required and none can have a default: either would force or
		// imply a second value.
		if in.IsOneOf {
			if IsNonNullType(f.Type) {
				v.report(at(typeNodeOf(f.ASTNode)), "OneOf input field %s must be nullable.", where)
			}
			if hasDefault(f.Default) {
				v.report(at(f.ASTNode), "OneOf input field %s cannot have a default value.", where)
			}
		}
	}
}

// validateInputObjectCycles reports an input object no one could ever write a
// value for.
//
// A value is finite only if the chain of fields it must contain comes to an
// end. A field that may be left out ends it, and so does a list, which may be
// empty, and so does a leaf. A field that must be given and holds an input
// object continues it, and a OneOf input object continues it whatever the
// field is written as, since exactly one field must be given a value.
//
// Where such a chain leads back to where it started with nothing to end it,
// the chain is reported. Each one is reported once, however many types it
// passes through.
func (v *schemaValidator) validateInputObjectCycles(root *InputObjectType) {
	if v.satisfiable == nil {
		v.satisfiable = satisfiableInputObjects(v.schema)
	}
	if v.satisfiable[root] {
		return
	}
	if v.walkedForCycles == nil {
		v.walkedForCycles = map[*InputObjectType]bool{}
	}

	// path is the chain of fields followed to get here, and enteredAt says
	// where on it each type was entered, which is what turns meeting a type
	// again into the part of the chain that loops.
	var path []*InputField
	enteredAt := map[*InputObjectType]int{}

	var walk func(in *InputObjectType)
	walk = func(in *InputObjectType) {
		if v.walkedForCycles[in] {
			return
		}
		v.walkedForCycles[in] = true
		enteredAt[in] = len(path)
		defer delete(enteredAt, in)

		for _, f := range in.Fields() {
			next := finiteValueTarget(in, f)
			if next == nil || v.satisfiable[next] {
				continue
			}
			path = append(path, f)
			if entered, looping := enteredAt[next]; looping {
				v.reportCycle(next, path[entered:])
			} else {
				walk(next)
			}
			path = path[:len(path)-1]
		}
	}
	walk(root)
}

// reportCycle describes one chain of fields that leads back to where it
// started.
func (v *schemaValidator) reportCycle(root *InputObjectType, cycle []*InputField) {
	named := make([]string, len(cycle))
	blamed := make([]language.Node, len(cycle))
	for i, f := range cycle {
		// Each field names the type it belongs to, since the chain walks
		// through several of them.
		named[i] = f.String()
		blamed[i] = f.ASTNode
	}
	v.report(blamed,
		"Input Object %s cannot be provided a finite value because it references itself through fields: %s.",
		root.Name(), strings.Join(named, ", "))
}

// satisfiableInputObjects works out which input objects a value can be written
// for.
//
// Nothing is taken to be satisfiable to begin with, and a type is admitted once
// its fields say so; repeating that until nothing changes leaves unsatisfied
// exactly the types that only ever lead to one another. Deciding in one pass
// instead would make the answer depend on where the walk began.
func satisfiableInputObjects(s *Schema) map[*InputObjectType]bool {
	var inputs []*InputObjectType
	for _, t := range s.Types() {
		if in, isInputObject := t.(*InputObjectType); isInputObject {
			inputs = append(inputs, in)
		}
	}
	satisfiable := make(map[*InputObjectType]bool, len(inputs))
	for changed := true; changed; {
		changed = false
		for _, in := range inputs {
			if satisfiable[in] || !satisfiedBy(in, satisfiable) {
				continue
			}
			satisfiable[in] = true
			changed = true
		}
	}
	return satisfiable
}

// satisfiedBy reports whether an input object can be written, given what is
// known to be satisfiable so far.
//
// Exactly one field of a OneOf input object is given a value, so one that can
// be written is enough; anywhere else every field that must be given has to be
// writable.
func satisfiedBy(in *InputObjectType, satisfiable map[*InputObjectType]bool) bool {
	fields := in.Fields()
	if in.IsOneOf {
		// A oneOf is given exactly one of its fields, so it can be given a
		// value as soon as any one of them can be.
		for _, f := range fields {
			if target := finiteValueTarget(in, f); target == nil || satisfiable[target] {
				return true
			}
		}
		return len(fields) == 0
	}
	// Every field that has to be given a value must be one that can be.
	for _, f := range fields {
		if target := finiteValueTarget(in, f); target != nil && !satisfiable[target] {
			return false
		}
	}
	return true
}

// finiteValueTarget returns the input object a field leads to when giving that
// field a value means giving one to another input object, and nil otherwise.
//
// A field of a list type never leads anywhere, because an empty list is a
// value. Which fields have to be given one differs: every field of a oneOf may
// be the one chosen, while elsewhere only a non-null field must be written.
func finiteValueTarget(in *InputObjectType, f *InputField) *InputObjectType {
	if f == nil {
		return nil
	}
	if in.IsOneOf {
		target, isInputObject := f.Type.(*InputObjectType)
		if !isInputObject {
			return nil
		}
		return target
	}
	wrapper, isNonNull := f.Type.(*NonNull)
	if !isNonNull {
		return nil
	}
	target, isInputObject := wrapper.OfType.(*InputObjectType)
	if !isInputObject {
		return nil
	}
	return target
}

// defaultStep is one field along a chain of defaults, kept so that a chain
// that leads back to where it started can name every step of it.
type defaultStep struct {
	// where names the field, as Type.field.
	where string
	// node is where the default is written, or nil for a schema built in Go.
	node language.Node
}

// validateDefaultValueCycles reports a default value that contains itself.
//
// A default may leave out a field, and the field's own default then fills the
// gap; that default may leave out a field in turn. Where following those
// defaults leads back to the field it started from, no finite value is being
// described, however innocent each default looks on its own.
//
// The walk starts from an empty value, which is the way to reach every field
// of the type at once and apply every default it has.
func (v *schemaValidator) validateDefaultValueCycles(root *InputObjectType) {
	if v.walkedDefaults == nil {
		v.walkedDefaults = map[string]bool{}
	}
	var path []defaultStep
	// enteredAt says where on the path each field was entered, counting from
	// one so that zero means "not on it".
	enteredAt := map[string]int{}

	var throughValue func(in *InputObjectType, held any)
	var throughLiteral func(in *InputObjectType, literal language.Value)

	// throughField follows one field's own default, which is what happens
	// where the value being walked says nothing about that field.
	throughField := func(f *InputField, fieldType *InputObjectType, where string) {
		input, has := f.Default.Get()
		if !has {
			return
		}
		if entered, looping := enteredAt[where]; looping {
			v.reportDefaultCycle(where, path[entered-1:], path[entered:])
			return
		}
		// A field is followed once however many ways there are to reach it,
		// so that one cycle is reported once.
		if v.walkedDefaults[where] {
			return
		}
		v.walkedDefaults[where] = true

		var written language.Node
		if f.ASTNode != nil {
			written = f.ASTNode.DefaultValue
		}
		path = append(path, defaultStep{where: where, node: written})
		enteredAt[where] = len(path)
		if input.Literal != nil {
			throughLiteral(fieldType, input.Literal)
		} else {
			throughValue(fieldType, input.Value)
		}
		path = path[:len(path)-1]
		delete(enteredAt, where)
	}

	// follow visits the fields of a type against what a value says about them.
	// A field the value mentions is followed into that value; a field it does
	// not mention falls back to the field's own default.
	follow := func(
		in *InputObjectType,
		mentions func(name string) bool,
		into func(f *InputField, fieldType *InputObjectType, name string),
	) {
		for _, f := range in.Fields() {
			if f == nil {
				continue
			}
			// Only a field holding an input object can carry on the chain.
			fieldType, isInputObject := NamedTypeOf(f.Type).(*InputObjectType)
			if !isInputObject {
				continue
			}
			if mentions(f.Name()) {
				into(f, fieldType, f.Name())
				continue
			}
			throughField(f, fieldType, in.Name()+"."+f.Name())
		}
	}

	throughValue = func(in *InputObjectType, held any) {
		if items, isList := asList(held); isList {
			for _, item := range items {
				throughValue(in, item)
			}
			return
		}
		fields, isObject := asObject(held)
		if !isObject {
			return
		}
		follow(in,
			func(name string) bool { _, given := fields[name]; return given },
			func(_ *InputField, fieldType *InputObjectType, name string) {
				throughValue(fieldType, fields[name])
			})
	}

	throughLiteral = func(in *InputObjectType, literal language.Value) {
		if list, isList := literal.(*language.ListValue); isList {
			for _, item := range list.Values {
				throughLiteral(in, item)
			}
			return
		}
		object, isObject := literal.(*language.ObjectValue)
		if !isObject {
			return
		}
		written := make(map[string]language.Value, len(object.Fields))
		for _, f := range object.Fields {
			if f != nil && f.Name != nil {
				written[f.Name.Value] = f.Value
			}
		}
		follow(in,
			func(name string) bool { _, given := written[name]; return given },
			func(_ *InputField, fieldType *InputObjectType, name string) {
				throughLiteral(fieldType, written[name])
			})
	}

	throughValue(root, map[string]any{})
}

// reportDefaultCycle describes one chain of defaults that leads back to where
// it started. cycle is the whole chain and via is what it passed through on
// the way, which is empty when a default refers to its own field directly.
func (v *schemaValidator) reportDefaultCycle(where string, cycle, via []defaultStep) {
	blamed := make([]language.Node, len(cycle))
	for i, step := range cycle {
		blamed[i] = step.node
	}
	through := ""
	if len(via) > 0 {
		names := make([]string, len(via))
		for i, step := range via {
			names[i] = step.where
		}
		through = " via the default values of: " + strings.Join(names, ", ")
	}
	v.report(blamed,
		"Invalid circular reference. The default value of Input Object field %s references itself%s.",
		where, through)
}

// describeTypeForError names a type for a message, saying so plainly when
// there is none.
func describeTypeForError(t Type) string {
	if t == nil {
		return "nothing"
	}
	return t.String()
}
