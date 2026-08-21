package language

// VisitAction tells the walk what to do after entering a node.
type VisitAction int

const (
	// VisitContinue descends into the node's children. It is the zero value,
	// so an Enter function that returns nothing meaningful still walks the
	// whole tree.
	VisitContinue VisitAction = iota
	// VisitSkip leaves the node's children unvisited. Leave is not called for
	// a node that was skipped, and returning it from Leave means nothing.
	VisitSkip
	// VisitBreak ends the whole walk at once.
	VisitBreak
)

// VisitContext says where in the tree a node sits.
type VisitContext struct {
	// Parent is the node holding this one, or nil at the root of the walk.
	Parent Node
	// Key names the field of Parent that this node came from, such as
	// "SelectionSet" or "Directives". It is empty at the root.
	Key string
	// Index is the position of this node within Key when that field holds a
	// list, and -1 when it holds a single node.
	Index int
	// Ancestors lists the enclosing nodes from the root down to Parent. The
	// slice is reused as the walk proceeds, so copy it to keep it.
	Ancestors []Node
}

// Visitor holds the functions a walk calls. Either may be nil.
//
// This walker only reads the tree. graphql-js can also rewrite nodes as it
// visits them, but nothing in the library itself uses that, so it is left out
// here; utilities that reshape a document do so with plain recursion.
type Visitor struct {
	// Enter is called on the way down, before a node's children.
	Enter func(node Node, ctx VisitContext) VisitAction
	// Leave is called on the way back up, after a node's children. It is not
	// called for a node whose Enter returned VisitSkip, nor once the walk has
	// been ended by VisitBreak.
	//
	// Only VisitBreak means anything here: the children have already been
	// walked, so there is nothing left to skip. Returning VisitSkip is the
	// same as returning VisitContinue.
	Leave func(node Node, ctx VisitContext) VisitAction
}

// Visit walks a tree depth first, calling the visitor on each node.
//
// Children are visited in the order the grammar writes them, so a walk of a
// document sees its nodes in source order.
//
// The walk recurses. That adds no risk a caller does not already have, because
// building the tree in the first place recurses just as deeply; a document
// nested deeply enough to trouble this would not have parsed.
func Visit(root Node, v Visitor) {
	w := &walker{visitor: v}
	w.walk(root, VisitContext{Index: -1})
}

// walker carries the state of one walk.
type walker struct {
	visitor   Visitor
	ancestors []Node
	stopped   bool
}

// walk visits one node and, unless told otherwise, its children.
func (w *walker) walk(node Node, ctx VisitContext) {
	if w.stopped || isAbsentNode(node) {
		return
	}
	ctx.Ancestors = w.ancestors

	if w.visitor.Enter != nil {
		switch w.visitor.Enter(node, ctx) {
		case VisitBreak:
			w.stopped = true
			return
		case VisitSkip:
			return
		}
	}

	w.ancestors = append(w.ancestors, node)
	visitChildren(node, func(child Node, key string, index int) bool {
		w.walk(child, VisitContext{Parent: node, Key: key, Index: index})
		return !w.stopped
	})
	w.ancestors = w.ancestors[:len(w.ancestors)-1]

	if w.stopped {
		return
	}
	if w.visitor.Leave != nil {
		ctx.Ancestors = w.ancestors
		if w.visitor.Leave(node, ctx) == VisitBreak {
			w.stopped = true
		}
	}
}

// VisitInParallel combines visitors into one that runs them all over a single
// walk, which is cheaper than walking once per visitor.
//
// Each visitor keeps its own idea of where it is: one that returns VisitSkip
// stops seeing nodes until the walk leaves the skipped subtree, and one that
// returns VisitBreak stops seeing nodes altogether, while the others carry on.
// The walk itself only ends when every visitor has broken.
func VisitInParallel(visitors ...Visitor) Visitor {
	// skipping records, for each visitor, the node whose subtree it asked to
	// skip; it is nil while that visitor is being shown nodes.
	skipping := make([]Node, len(visitors))
	broken := make([]bool, len(visitors))

	return Visitor{
		Enter: func(node Node, ctx VisitContext) VisitAction {
			live := 0
			for i, v := range visitors {
				if broken[i] {
					continue
				}
				live++
				if skipping[i] != nil {
					continue
				}
				if v.Enter == nil {
					continue
				}
				switch v.Enter(node, ctx) {
				case VisitSkip:
					skipping[i] = node
				case VisitBreak:
					broken[i] = true
					live--
				}
			}
			if live == 0 {
				return VisitBreak
			}
			return VisitContinue
		},
		Leave: func(node Node, ctx VisitContext) VisitAction {
			live := 0
			for i, v := range visitors {
				if broken[i] {
					continue
				}
				live++
				if skipping[i] != nil {
					// The walk is leaving the node this visitor skipped, so it
					// starts seeing nodes again.
					if skipping[i] == node {
						skipping[i] = nil
					}
					continue
				}
				if v.Leave == nil {
					continue
				}
				if v.Leave(node, ctx) == VisitBreak {
					broken[i] = true
					live--
				}
			}
			if live == 0 {
				return VisitBreak
			}
			return VisitContinue
		},
	}
}

// visitChild passes one optional child to yield, skipping it if absent.
func visitChild[T Node](yield func(Node, string, int) bool, key string, child T) bool {
	if isAbsentNode(child) {
		return true
	}
	return yield(child, key, -1)
}

// visitList passes each child of a list to yield, with its index.
func visitList[T Node](yield func(Node, string, int) bool, key string, children []T) bool {
	for i, child := range children {
		if isAbsentNode(child) {
			continue
		}
		if !yield(child, key, i) {
			return false
		}
	}
	return true
}

// visitChildren passes each child of a node to yield, in the order the grammar
// writes them, stopping early if yield returns false.
//
// This table is the whole of what the walker needs to know about the shape of
// the tree. A node type added without a case here is walked as a leaf, which
// is why the tests check every kind against it.
func visitChildren(node Node, yield func(child Node, key string, index int) bool) bool {
	switch n := node.(type) {
	case *Document:
		return visitList(yield, "Definitions", n.Definitions)

	case *OperationDefinition:
		return visitChild(yield, "Description", n.Description) &&
			visitChild(yield, "Name", n.Name) &&
			visitList(yield, "VariableDefinitions", n.VariableDefinitions) &&
			visitList(yield, "Directives", n.Directives) &&
			visitChild(yield, "SelectionSet", n.SelectionSet)
	case *VariableDefinition:
		return visitChild(yield, "Description", n.Description) &&
			visitChild(yield, "Variable", n.Variable) &&
			visitChild(yield, "Type", n.Type) &&
			visitChild(yield, "DefaultValue", n.DefaultValue) &&
			visitList(yield, "Directives", n.Directives)
	case *Variable:
		return visitChild(yield, "Name", n.Name)
	case *SelectionSet:
		return visitList(yield, "Selections", n.Selections)
	case *Field:
		return visitChild(yield, "Alias", n.Alias) &&
			visitChild(yield, "Name", n.Name) &&
			visitList(yield, "Arguments", n.Arguments) &&
			visitList(yield, "Directives", n.Directives) &&
			visitChild(yield, "SelectionSet", n.SelectionSet)
	case *Argument:
		return visitChild(yield, "Name", n.Name) &&
			visitChild(yield, "Value", n.Value)
	case *FragmentArgument:
		return visitChild(yield, "Name", n.Name) &&
			visitChild(yield, "Value", n.Value)
	case *FragmentSpread:
		return visitChild(yield, "Name", n.Name) &&
			visitList(yield, "Arguments", n.Arguments) &&
			visitList(yield, "Directives", n.Directives)
	case *InlineFragment:
		return visitChild(yield, "TypeCondition", n.TypeCondition) &&
			visitList(yield, "Directives", n.Directives) &&
			visitChild(yield, "SelectionSet", n.SelectionSet)
	case *FragmentDefinition:
		return visitChild(yield, "Description", n.Description) &&
			visitChild(yield, "Name", n.Name) &&
			visitList(yield, "VariableDefinitions", n.VariableDefinitions) &&
			visitChild(yield, "TypeCondition", n.TypeCondition) &&
			visitList(yield, "Directives", n.Directives) &&
			visitChild(yield, "SelectionSet", n.SelectionSet)

	case *ListValue:
		return visitList(yield, "Values", n.Values)
	case *ObjectValue:
		return visitList(yield, "Fields", n.Fields)
	case *ObjectField:
		return visitChild(yield, "Name", n.Name) &&
			visitChild(yield, "Value", n.Value)

	case *Directive:
		return visitChild(yield, "Name", n.Name) &&
			visitList(yield, "Arguments", n.Arguments)

	case *NamedType:
		return visitChild(yield, "Name", n.Name)
	case *ListType:
		return visitChild(yield, "Type", n.Type)
	case *NonNullType:
		return visitChild(yield, "Type", n.Type)

	case *SchemaDefinition:
		return visitChild(yield, "Description", n.Description) &&
			visitList(yield, "Directives", n.Directives) &&
			visitList(yield, "OperationTypes", n.OperationTypes)
	case *OperationTypeDefinition:
		return visitChild(yield, "Type", n.Type)
	case *ScalarTypeDefinition:
		return visitChild(yield, "Description", n.Description) &&
			visitChild(yield, "Name", n.Name) &&
			visitList(yield, "Directives", n.Directives)
	case *ObjectTypeDefinition:
		return visitChild(yield, "Description", n.Description) &&
			visitChild(yield, "Name", n.Name) &&
			visitList(yield, "Interfaces", n.Interfaces) &&
			visitList(yield, "Directives", n.Directives) &&
			visitList(yield, "Fields", n.Fields)
	case *FieldDefinition:
		return visitChild(yield, "Description", n.Description) &&
			visitChild(yield, "Name", n.Name) &&
			visitList(yield, "Arguments", n.Arguments) &&
			visitChild(yield, "Type", n.Type) &&
			visitList(yield, "Directives", n.Directives)
	case *InputValueDefinition:
		return visitChild(yield, "Description", n.Description) &&
			visitChild(yield, "Name", n.Name) &&
			visitChild(yield, "Type", n.Type) &&
			visitChild(yield, "DefaultValue", n.DefaultValue) &&
			visitList(yield, "Directives", n.Directives)
	case *InterfaceTypeDefinition:
		return visitChild(yield, "Description", n.Description) &&
			visitChild(yield, "Name", n.Name) &&
			visitList(yield, "Interfaces", n.Interfaces) &&
			visitList(yield, "Directives", n.Directives) &&
			visitList(yield, "Fields", n.Fields)
	case *UnionTypeDefinition:
		return visitChild(yield, "Description", n.Description) &&
			visitChild(yield, "Name", n.Name) &&
			visitList(yield, "Directives", n.Directives) &&
			visitList(yield, "Types", n.Types)
	case *EnumTypeDefinition:
		return visitChild(yield, "Description", n.Description) &&
			visitChild(yield, "Name", n.Name) &&
			visitList(yield, "Directives", n.Directives) &&
			visitList(yield, "Values", n.Values)
	case *EnumValueDefinition:
		return visitChild(yield, "Description", n.Description) &&
			visitChild(yield, "Name", n.Name) &&
			visitList(yield, "Directives", n.Directives)
	case *InputObjectTypeDefinition:
		return visitChild(yield, "Description", n.Description) &&
			visitChild(yield, "Name", n.Name) &&
			visitList(yield, "Directives", n.Directives) &&
			visitList(yield, "Fields", n.Fields)
	case *DirectiveDefinition:
		return visitChild(yield, "Description", n.Description) &&
			visitChild(yield, "Name", n.Name) &&
			visitList(yield, "Arguments", n.Arguments) &&
			visitList(yield, "Directives", n.Directives) &&
			visitList(yield, "Locations", n.Locations)

	case *SchemaExtension:
		return visitList(yield, "Directives", n.Directives) &&
			visitList(yield, "OperationTypes", n.OperationTypes)
	case *ScalarTypeExtension:
		return visitChild(yield, "Name", n.Name) &&
			visitList(yield, "Directives", n.Directives)
	case *ObjectTypeExtension:
		return visitChild(yield, "Name", n.Name) &&
			visitList(yield, "Interfaces", n.Interfaces) &&
			visitList(yield, "Directives", n.Directives) &&
			visitList(yield, "Fields", n.Fields)
	case *InterfaceTypeExtension:
		return visitChild(yield, "Name", n.Name) &&
			visitList(yield, "Interfaces", n.Interfaces) &&
			visitList(yield, "Directives", n.Directives) &&
			visitList(yield, "Fields", n.Fields)
	case *UnionTypeExtension:
		return visitChild(yield, "Name", n.Name) &&
			visitList(yield, "Directives", n.Directives) &&
			visitList(yield, "Types", n.Types)
	case *EnumTypeExtension:
		return visitChild(yield, "Name", n.Name) &&
			visitList(yield, "Directives", n.Directives) &&
			visitList(yield, "Values", n.Values)
	case *InputObjectTypeExtension:
		return visitChild(yield, "Name", n.Name) &&
			visitList(yield, "Directives", n.Directives) &&
			visitList(yield, "Fields", n.Fields)
	case *DirectiveExtension:
		return visitChild(yield, "Name", n.Name) &&
			visitList(yield, "Directives", n.Directives)

	case *TypeCoordinate:
		return visitChild(yield, "Name", n.Name)
	case *MemberCoordinate:
		return visitChild(yield, "Name", n.Name) &&
			visitChild(yield, "MemberName", n.MemberName)
	case *ArgumentCoordinate:
		return visitChild(yield, "Name", n.Name) &&
			visitChild(yield, "FieldName", n.FieldName) &&
			visitChild(yield, "ArgumentName", n.ArgumentName)
	case *DirectiveCoordinate:
		return visitChild(yield, "Name", n.Name)
	case *DirectiveArgumentCoordinate:
		return visitChild(yield, "Name", n.Name) &&
			visitChild(yield, "ArgumentName", n.ArgumentName)
	}

	// Names, numbers, strings, booleans, nulls and enum values are leaves.
	return true
}
