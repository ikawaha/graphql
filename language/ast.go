package language

import "reflect"

// Location records where a node came from in the source document.
//
// Start and End are byte offsets into the source body, matching the offsets
// the lexer records on tokens.
type Location struct {
	// Start is the byte offset at which the node begins.
	Start int
	// End is the byte offset just past the end of the node.
	End int
	// StartToken is the first token of the node.
	StartToken *Token
	// EndToken is the last token of the node.
	EndToken *Token
	// Source is the document the node was parsed from.
	Source *Source
}

// newLocation builds a Location spanning two tokens.
func newLocation(start, end *Token, source *Source) *Location {
	return &Location{
		Start:      start.Start,
		End:        end.End,
		StartToken: start,
		EndToken:   end,
		Source:     source,
	}
}

// Node is implemented by every AST node.
//
// Nodes are represented as one Go type per kind, so a type switch over a Node
// mirrors the switch on node.kind that the reference implementation uses, and
// the fields available on each kind are the ones the type declares.
//
// An optional child node is a nil pointer and an optional list is a nil slice.
// This is the "omitted is expressed by the container" rule the rest of the
// library follows; an AST never crosses a JSON boundary, so it has no need for
// the three states value.Maybe exists to represent.
type Node interface {
	// Kind reports which kind of node this is.
	Kind() Kind
	// Location reports where the node came from, or nil when the document was
	// parsed without location information.
	Location() *Location
}

// Value is implemented by the nodes that can appear where the grammar expects
// a value.
//
// # Constant values
//
// The GraphQL grammar distinguishes a value from a constant value, which is a
// value containing no variable references. The reference implementation draws
// that distinction in its type declarations only: at run time both are the
// same objects with the same kinds. This port follows the run-time shape and
// uses one set of node types, so a default value or a directive argument in a
// schema is an ordinary Value. The parser rejects a variable where the grammar
// forbids one, and [ContainsVariable] can check a value that was built by hand.
type Value interface {
	Node
	isValue()
}

// Type is implemented by the nodes that can appear where the grammar expects a
// type reference.
type Type interface {
	Node
	isType()
}

// Selection is implemented by the nodes that may appear in a selection set.
type Selection interface {
	Node
	isSelection()
}

// Definition is implemented by every node that may appear at the top level of
// a document.
type Definition interface {
	Node
	isDefinition()
}

// ExecutableDefinition is an operation or a fragment, the definitions that a
// request may contain.
type ExecutableDefinition interface {
	Definition
	isExecutableDefinition()
}

// TypeSystemDefinition is a schema, type or directive definition.
type TypeSystemDefinition interface {
	Definition
	isTypeSystemDefinition()
}

// TypeDefinition is the definition of a named type.
type TypeDefinition interface {
	TypeSystemDefinition
	isTypeDefinition()
}

// TypeSystemExtension extends a schema, a type or a directive.
type TypeSystemExtension interface {
	Definition
	isTypeSystemExtension()
}

// TypeExtension extends a named type.
type TypeExtension interface {
	TypeSystemExtension
	isTypeExtension()
}

// SchemaCoordinate names an element of a schema, such as a type, a field, or a
// directive argument.
type SchemaCoordinate interface {
	Node
	isSchemaCoordinate()
}

// Name is an identifier.
type Name struct {
	Loc   *Location
	Value string
}

func (*Name) Kind() Kind            { return KindName }
func (n *Name) Location() *Location { return n.Loc }

// Document is a parsed GraphQL document: a list of definitions.
type Document struct {
	Loc         *Location
	Definitions []Definition
	// TokenCount is the number of tokens the document was parsed from, which
	// callers can use to bound the cost of accepting a request.
	TokenCount int
}

func (*Document) Kind() Kind            { return KindDocument }
func (n *Document) Location() *Location { return n.Loc }

// OperationDefinition is a query, mutation or subscription.
type OperationDefinition struct {
	Loc                 *Location
	Description         *StringValue
	Operation           OperationType
	Name                *Name
	VariableDefinitions []*VariableDefinition
	Directives          []*Directive
	SelectionSet        *SelectionSet
}

func (*OperationDefinition) Kind() Kind            { return KindOperationDefinition }
func (n *OperationDefinition) Location() *Location { return n.Loc }
func (*OperationDefinition) isDefinition()         {}
func (*OperationDefinition) isExecutableDefinition() {
}

// VariableDefinition declares a variable of an operation or a fragment.
type VariableDefinition struct {
	Loc          *Location
	Description  *StringValue
	Variable     *Variable
	Type         Type
	DefaultValue Value
	Directives   []*Directive
}

func (*VariableDefinition) Kind() Kind            { return KindVariableDefinition }
func (n *VariableDefinition) Location() *Location { return n.Loc }

// SelectionSet is a braced list of selections.
type SelectionSet struct {
	Loc        *Location
	Selections []Selection
}

func (*SelectionSet) Kind() Kind            { return KindSelectionSet }
func (n *SelectionSet) Location() *Location { return n.Loc }

// Field selects a single field, optionally under an alias.
type Field struct {
	Loc          *Location
	Alias        *Name
	Name         *Name
	Arguments    []*Argument
	Directives   []*Directive
	SelectionSet *SelectionSet
}

func (*Field) Kind() Kind            { return KindField }
func (n *Field) Location() *Location { return n.Loc }
func (*Field) isSelection()          {}

// ResponseKey returns the key this field takes in the response, which is its
// alias when it has one and otherwise its name.
func (n *Field) ResponseKey() string {
	if n.Alias != nil {
		return n.Alias.Value
	}
	if n.Name != nil {
		return n.Name.Value
	}
	return ""
}

// Argument is a named argument of a field or a directive.
type Argument struct {
	Loc   *Location
	Name  *Name
	Value Value
}

func (*Argument) Kind() Kind            { return KindArgument }
func (n *Argument) Location() *Location { return n.Loc }

// FragmentArgument is an argument passed to a fragment spread.
type FragmentArgument struct {
	Loc   *Location
	Name  *Name
	Value Value
}

func (*FragmentArgument) Kind() Kind            { return KindFragmentArgument }
func (n *FragmentArgument) Location() *Location { return n.Loc }

// FragmentSpread includes a named fragment in a selection set.
type FragmentSpread struct {
	Loc        *Location
	Name       *Name
	Arguments  []*FragmentArgument
	Directives []*Directive
}

func (*FragmentSpread) Kind() Kind            { return KindFragmentSpread }
func (n *FragmentSpread) Location() *Location { return n.Loc }
func (*FragmentSpread) isSelection()          {}

// InlineFragment is a fragment written in place, optionally narrowed to a type.
type InlineFragment struct {
	Loc           *Location
	TypeCondition *NamedType
	Directives    []*Directive
	SelectionSet  *SelectionSet
}

func (*InlineFragment) Kind() Kind            { return KindInlineFragment }
func (n *InlineFragment) Location() *Location { return n.Loc }
func (*InlineFragment) isSelection()          {}

// FragmentDefinition defines a named fragment.
type FragmentDefinition struct {
	Loc                 *Location
	Description         *StringValue
	Name                *Name
	VariableDefinitions []*VariableDefinition
	TypeCondition       *NamedType
	Directives          []*Directive
	SelectionSet        *SelectionSet
}

func (*FragmentDefinition) Kind() Kind            { return KindFragmentDefinition }
func (n *FragmentDefinition) Location() *Location { return n.Loc }
func (*FragmentDefinition) isDefinition()         {}
func (*FragmentDefinition) isExecutableDefinition() {
}

// Directive applies a directive to the element it follows.
type Directive struct {
	Loc       *Location
	Name      *Name
	Arguments []*Argument
}

func (*Directive) Kind() Kind            { return KindDirective }
func (n *Directive) Location() *Location { return n.Loc }

// isAbsentNode reports whether a node is missing.
//
// An optional child is written as a nil pointer, and putting one in a Node
// interface produces a value that is not equal to nil but holds nothing. Every
// node type is a pointer type, so the reflected nil check is always valid, and
// catching the case here keeps Print total: it never panics, whatever AST it
// is handed.
//
// Checking each case with a plain pointer comparison instead was measured at
// the same speed on a large schema, so this keeps the shorter form.
func isAbsentNode(node Node) bool {
	if node == nil {
		return true
	}
	// Every node this package defines is a pointer, but Node is an ordinary
	// interface and a caller may satisfy it with a value. IsNil would panic on
	// one of those, and the whole point here is not to panic.
	switch rv := reflect.ValueOf(node); rv.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice,
		reflect.Func, reflect.Chan:
		return rv.IsNil()
	default:
		return false
	}
}
