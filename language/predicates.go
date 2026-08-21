package language

// The predicates below classify a node by the role it can play in a document.
//
// In Go most of them are a type assertion against one of the AST interfaces,
// which the compiler already enforces wherever the type is known. They are
// worth having anyway for code that holds a plain [Node] and wants to ask.

// IsDefinition reports whether a node may appear at the top level of a
// document.
func IsDefinition(node Node) bool {
	_, ok := node.(Definition)
	return ok
}

// IsExecutableDefinition reports whether a node is an operation or a fragment
// definition, the two kinds a request may contain.
func IsExecutableDefinition(node Node) bool {
	_, ok := node.(ExecutableDefinition)
	return ok
}

// IsSubscriptionOperation reports whether a node is an operation definition
// for a subscription.
func IsSubscriptionOperation(node Node) bool {
	op, ok := node.(*OperationDefinition)
	return ok && op.Operation == OperationSubscription
}

// IsSelection reports whether a node may appear in a selection set.
func IsSelection(node Node) bool {
	_, ok := node.(Selection)
	return ok
}

// IsValue reports whether a node may appear where the grammar expects a value.
func IsValue(node Node) bool {
	_, ok := node.(Value)
	return ok
}

// IsConstValue reports whether a node is a value that references no variables,
// which is what the grammar requires of a default value or of an argument to a
// directive in a schema.
//
// A list or an input object is constant only when everything inside it is.
// graphql-js answers this with a check that asks whether *any* element is
// constant, so it reports a mixed list such as [$var, 1] as constant; this
// implementation requires all of them, which is what the grammar means.
func IsConstValue(node Node) bool {
	value, ok := node.(Value)
	return ok && !ContainsVariable(value)
}

// IsType reports whether a node is a type reference.
func IsType(node Node) bool {
	_, ok := node.(Type)
	return ok
}

// IsTypeSystemDefinition reports whether a node defines part of a schema.
func IsTypeSystemDefinition(node Node) bool {
	_, ok := node.(TypeSystemDefinition)
	return ok
}

// IsTypeDefinition reports whether a node defines a named type.
func IsTypeDefinition(node Node) bool {
	_, ok := node.(TypeDefinition)
	return ok
}

// IsTypeSystemExtension reports whether a node extends part of a schema.
func IsTypeSystemExtension(node Node) bool {
	_, ok := node.(TypeSystemExtension)
	return ok
}

// IsTypeExtension reports whether a node extends a named type.
func IsTypeExtension(node Node) bool {
	_, ok := node.(TypeExtension)
	return ok
}

// IsSchemaCoordinate reports whether a node names an element of a schema.
func IsSchemaCoordinate(node Node) bool {
	_, ok := node.(SchemaCoordinate)
	return ok
}
