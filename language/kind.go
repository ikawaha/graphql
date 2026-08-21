package language

// Kind identifies the concrete type of an AST node.
//
// Every node type reports its kind from the Kind method of the [Node]
// interface, which lets code switch on the kind where a Go type switch would
// be awkward, and gives errors a name to print.
type Kind string

// The kinds of AST node.
const (
	KindName Kind = "Name"

	KindDocument            Kind = "Document"
	KindOperationDefinition Kind = "OperationDefinition"
	KindVariableDefinition  Kind = "VariableDefinition"
	KindSelectionSet        Kind = "SelectionSet"
	KindField               Kind = "Field"
	KindArgument            Kind = "Argument"
	KindFragmentArgument    Kind = "FragmentArgument"
	KindFragmentSpread      Kind = "FragmentSpread"
	KindInlineFragment      Kind = "InlineFragment"
	KindFragmentDefinition  Kind = "FragmentDefinition"

	KindVariable     Kind = "Variable"
	KindIntValue     Kind = "IntValue"
	KindFloatValue   Kind = "FloatValue"
	KindStringValue  Kind = "StringValue"
	KindBooleanValue Kind = "BooleanValue"
	KindNullValue    Kind = "NullValue"
	KindEnumValue    Kind = "EnumValue"
	KindListValue    Kind = "ListValue"
	KindObjectValue  Kind = "ObjectValue"
	KindObjectField  Kind = "ObjectField"

	KindDirective Kind = "Directive"

	KindNamedType   Kind = "NamedType"
	KindListType    Kind = "ListType"
	KindNonNullType Kind = "NonNullType"

	KindSchemaDefinition          Kind = "SchemaDefinition"
	KindOperationTypeDefinition   Kind = "OperationTypeDefinition"
	KindScalarTypeDefinition      Kind = "ScalarTypeDefinition"
	KindObjectTypeDefinition      Kind = "ObjectTypeDefinition"
	KindFieldDefinition           Kind = "FieldDefinition"
	KindInputValueDefinition      Kind = "InputValueDefinition"
	KindInterfaceTypeDefinition   Kind = "InterfaceTypeDefinition"
	KindUnionTypeDefinition       Kind = "UnionTypeDefinition"
	KindEnumTypeDefinition        Kind = "EnumTypeDefinition"
	KindEnumValueDefinition       Kind = "EnumValueDefinition"
	KindInputObjectTypeDefinition Kind = "InputObjectTypeDefinition"
	KindDirectiveDefinition       Kind = "DirectiveDefinition"

	KindSchemaExtension          Kind = "SchemaExtension"
	KindScalarTypeExtension      Kind = "ScalarTypeExtension"
	KindObjectTypeExtension      Kind = "ObjectTypeExtension"
	KindInterfaceTypeExtension   Kind = "InterfaceTypeExtension"
	KindUnionTypeExtension       Kind = "UnionTypeExtension"
	KindEnumTypeExtension        Kind = "EnumTypeExtension"
	KindInputObjectTypeExtension Kind = "InputObjectTypeExtension"
	KindDirectiveExtension       Kind = "DirectiveExtension"

	KindTypeCoordinate              Kind = "TypeCoordinate"
	KindMemberCoordinate            Kind = "MemberCoordinate"
	KindArgumentCoordinate          Kind = "ArgumentCoordinate"
	KindDirectiveCoordinate         Kind = "DirectiveCoordinate"
	KindDirectiveArgumentCoordinate Kind = "DirectiveArgumentCoordinate"
)

// String returns the kind as it appears in messages.
func (k Kind) String() string { return string(k) }

// OperationType is the kind of operation an executable definition describes.
type OperationType string

// The operation types a document may define.
const (
	OperationQuery        OperationType = "query"
	OperationMutation     OperationType = "mutation"
	OperationSubscription OperationType = "subscription"
)

// String returns the operation type as it is spelled in a document.
func (o OperationType) String() string { return string(o) }
