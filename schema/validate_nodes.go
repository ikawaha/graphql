package schema

import "github.com/ikawaha/graphql/language"

// This file finds the part of a schema's source responsible for a problem, so
// that validation can point at it.
//
// A schema built in Go has no source at all, and one built from SDL may still
// be missing a node here and there, so every one of these may come back empty;
// reporting handles that by dropping what is not there.

// nodesOf gathers a definition and the extensions of it into one list.
func nodesOf[D language.Node, E language.Node](definition D, extensions []E) []language.Node {
	nodes := make([]language.Node, 0, 1+len(extensions))
	nodes = append(nodes, definition)
	for _, e := range extensions {
		nodes = append(nodes, e)
	}
	return nodes
}

// definitionNodes returns everywhere a type is written: its definition and
// every extension of it.
func definitionNodes(t Type) []language.Node {
	switch n := t.(type) {
	case *ScalarType:
		return nodesOf(n.ASTNode, n.ExtensionASTNodes)
	case *ObjectType:
		return nodesOf(n.ASTNode, n.ExtensionASTNodes)
	case *InterfaceType:
		return nodesOf(n.ASTNode, n.ExtensionASTNodes)
	case *UnionType:
		return nodesOf(n.ASTNode, n.ExtensionASTNodes)
	case *EnumType:
		return nodesOf(n.ASTNode, n.ExtensionASTNodes)
	case *InputObjectType:
		return nodesOf(n.ASTNode, n.ExtensionASTNodes)
	}
	return nil
}

// implementsNodes returns the places a type names an interface in an
// implements clause. A type may be extended more than once, so there may be
// more than one, which is how declaring the same interface twice is reported
// at both of them.
func implementsNodes(t Type, ifaceName string) []language.Node {
	var clauses [][]*language.NamedType
	switch n := t.(type) {
	case *ObjectType:
		clauses = append(clauses, clauseOf(n.ASTNode))
		for _, e := range n.ExtensionASTNodes {
			clauses = append(clauses, clauseOf(e))
		}
	case *InterfaceType:
		clauses = append(clauses, clauseOf(n.ASTNode))
		for _, e := range n.ExtensionASTNodes {
			clauses = append(clauses, clauseOf(e))
		}
	}
	var nodes []language.Node
	for _, clause := range clauses {
		for _, named := range clause {
			if named != nil && named.Name != nil && named.Name.Value == ifaceName {
				nodes = append(nodes, named)
			}
		}
	}
	return nodes
}

// clauseOf reads the implements clause of one definition or extension.
func clauseOf(node language.Node) []*language.NamedType {
	switch n := node.(type) {
	case *language.ObjectTypeDefinition:
		if n != nil {
			return n.Interfaces
		}
	case *language.ObjectTypeExtension:
		if n != nil {
			return n.Interfaces
		}
	case *language.InterfaceTypeDefinition:
		if n != nil {
			return n.Interfaces
		}
	case *language.InterfaceTypeExtension:
		if n != nil {
			return n.Interfaces
		}
	}
	return nil
}

// memberNodes returns the places a union names one of its members.
func memberNodes(u *UnionType, memberName string) []language.Node {
	var lists [][]*language.NamedType
	if u.ASTNode != nil {
		lists = append(lists, u.ASTNode.Types)
	}
	for _, e := range u.ExtensionASTNodes {
		if e != nil {
			lists = append(lists, e.Types)
		}
	}
	var nodes []language.Node
	for _, list := range lists {
		for _, named := range list {
			if named != nil && named.Name != nil && named.Name.Value == memberName {
				nodes = append(nodes, named)
			}
		}
	}
	return nodes
}

// operationTypeNode returns where a schema names the root type an operation
// enters through, which is the type's name rather than the whole binding.
func operationTypeNode(s *Schema, operation language.OperationType) language.Node {
	var lists [][]*language.OperationTypeDefinition
	if s.ASTNode != nil {
		lists = append(lists, s.ASTNode.OperationTypes)
	}
	for _, e := range s.ExtensionASTNodes {
		if e != nil {
			lists = append(lists, e.OperationTypes)
		}
	}
	for _, list := range lists {
		for _, bound := range list {
			if bound != nil && bound.Operation == operation {
				return bound.Type
			}
		}
	}
	return nil
}

// deprecatedNode returns where something was marked deprecated, which is what
// a complaint about the deprecation should point at rather than the whole
// definition.
func deprecatedNode(node language.Node) language.Node {
	var directives []*language.Directive
	switch n := node.(type) {
	case *language.FieldDefinition:
		if n != nil {
			directives = n.Directives
		}
	case *language.InputValueDefinition:
		if n != nil {
			directives = n.Directives
		}
	case *language.EnumValueDefinition:
		if n != nil {
			directives = n.Directives
		}
	}
	for _, d := range directives {
		if d != nil && d.Name != nil && d.Name.Value == Deprecated.Name() {
			return d
		}
	}
	return nil
}

// typeNodeOf returns where a field or argument writes its type, which is what
// a complaint about that type should point at.
func typeNodeOf(node language.Node) language.Node {
	switch n := node.(type) {
	case *language.FieldDefinition:
		if n != nil {
			return n.Type
		}
	case *language.InputValueDefinition:
		if n != nil {
			return n.Type
		}
	}
	return nil
}

// at is shorthand for blaming a single node.
func at(nodes ...language.Node) []language.Node { return nodes }
