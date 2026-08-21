package language

import (
	"fmt"
	"reflect"
)

// TransformAction tells a rewrite what to do with the node it has just been
// shown.
type TransformAction int

const (
	// TransformContinue rewrites the node's children too. It is the zero
	// value, so a function that returns nothing meaningful still walks the
	// whole tree.
	TransformContinue TransformAction = iota
	// TransformSkip leaves the node's children as they are. Leave is not
	// called for a node that was skipped.
	TransformSkip
	// TransformRemove drops the node from the tree. Where it is one of a list
	// the list closes up; where it is a single child the child is gone, which
	// may leave the parent without something it needs — a rewrite that removes
	// a field's name produces a tree that will not print.
	TransformRemove
	// TransformBreak ends the rewrite at once. What has been rewritten so far
	// stands; the rest of the tree is left as it was.
	TransformBreak
)

// Transformer holds the functions a rewrite calls. Either may be nil.
//
// Each returns the node that should stand in the place of the one it was
// shown, or nil to leave that node as it is. A nil of a node type — the value
// a helper returns when it has nothing to put there — counts as nil, so a
// transformer cannot put a hole in the tree by accident; removing a node is
// TransformRemove and nothing else.
//
// Returning a node of a different type than the place allows — an *IntValue
// where the tree wants a *Name — panics, since nothing downstream could make
// sense of it.
type Transformer struct {
	// Enter is called on the way down, before a node's children. A node it
	// returns is put in place first, and it is that node's children the
	// rewrite goes on to.
	Enter func(node Node, ctx VisitContext) (Node, TransformAction)
	// Leave is called on the way back up, once the node's children have been
	// rewritten. The node it is shown is the one holding the new children.
	//
	// TransformSkip means nothing here: the children are already done.
	Leave func(node Node, ctx VisitContext) (Node, TransformAction)
}

// Transform walks a tree and returns what the transformer made of it.
//
// Nothing is modified in place. The tree that comes back shares every node the
// transformer left alone with the tree that went in: only the nodes on the
// path from a changed one up to the root are new, so rewriting one field of a
// large document leaves the rest of it shared rather than copied. A rewrite
// that changes nothing gives back the very tree it was handed.
//
// That is about the tree, not about the walk. Reaching a node's children means
// copying the node in case they change, and the copy is thrown away when they
// do not, so a rewrite costs roughly one allocation per node that has children
// whether or not anything comes of it. Reading a tree is [Visit], which costs
// nothing to speak of.
//
// The tree that comes back is the transformer's answer for the root, which is
// nil if the root itself was removed.
//
// It fails when the transformer answers with a node that cannot go where it
// was offered — an *IntValue where the tree holds a *Name. Nothing is returned
// in that case, since a half-rewritten tree is no use to anyone.
//
// This is graphql-js's editing visitor, where the walking and the editing are
// one function told apart by what the callbacks return.
func Transform(root Node, t Transformer) (Node, error) {
	w := &rewriter{transformer: t}
	out := w.rewrite(root, VisitContext{Index: -1})
	if w.failed != nil {
		return nil, w.failed
	}
	return out, nil
}

// rewriter carries the state of one rewrite.
type rewriter struct {
	transformer Transformer
	ancestors   []Node
	stopped     bool
	// failed is the first replacement that did not fit where it was offered.
	failed error
}

// rewrite handles one node and, unless told otherwise, its children.
func (w *rewriter) rewrite(node Node, ctx VisitContext) Node {
	if w.stopped || w.failed != nil || isAbsentNode(node) {
		return node
	}
	ctx.Ancestors = w.ancestors

	current := node
	if w.transformer.Enter != nil {
		replacement, action := w.transformer.Enter(current, ctx)
		if !isAbsentNode(replacement) {
			current = replacement
		}
		switch action {
		case TransformRemove:
			return nil
		case TransformBreak:
			w.stopped = true
			return current
		case TransformSkip:
			return current
		}
	}

	w.ancestors = append(w.ancestors, current)
	rewritten := transformChildren(current, func(child Node, key string, index int) Node {
		return w.rewrite(child, VisitContext{Parent: current, Key: key, Index: index})
	}, &w.failed)
	w.ancestors = w.ancestors[:len(w.ancestors)-1]
	current = rewritten

	if w.stopped || w.failed != nil || w.transformer.Leave == nil {
		return current
	}
	ctx.Ancestors = w.ancestors
	replacement, action := w.transformer.Leave(current, ctx)
	if !isAbsentNode(replacement) {
		current = replacement
	}
	switch action {
	case TransformRemove:
		return nil
	case TransformBreak:
		w.stopped = true
	}
	return current
}

// TransformInParallel combines transformers into one that runs them all over a
// single rewrite.
//
// They are shown each node in turn, and a node one of them changes or removes
// is not shown to the rest: what a later transformer would have said about a
// node that is no longer there cannot be acted on. That is the rule graphql-js
// follows. Where transformers have to see each other's work, run them one
// after another instead.
//
// Each keeps its own idea of where it is, the same as [VisitInParallel]: one
// that skips a subtree stops seeing nodes until the rewrite leaves it, and one
// that breaks stops seeing nodes altogether while the others carry on. The
// rewrite itself descends as long as any of them still wants to see what is
// down there, and ends only when they have all broken.
func TransformInParallel(transformers ...Transformer) Transformer {
	// skipping records, for each transformer, the node whose subtree it asked
	// to skip; it is nil while that transformer is being shown nodes.
	skipping := make([]Node, len(transformers))
	broken := make([]bool, len(transformers))

	// live counts the transformers still interested in what comes next.
	live := func() int {
		n := 0
		for i := range transformers {
			if !broken[i] {
				n++
			}
		}
		return n
	}

	return Transformer{
		Enter: func(node Node, ctx VisitContext) (Node, TransformAction) {
			for i, t := range transformers {
				if broken[i] || skipping[i] != nil || t.Enter == nil {
					continue
				}
				replacement, action := t.Enter(node, ctx)
				switch action {
				case TransformSkip:
					skipping[i] = node
				case TransformBreak:
					broken[i] = true
				}
				if !isAbsentNode(replacement) || action == TransformRemove {
					return replacement, action
				}
			}
			if live() == 0 {
				return nil, TransformBreak
			}
			return nil, TransformContinue
		},
		Leave: func(node Node, ctx VisitContext) (Node, TransformAction) {
			// The rewrite is leaving the node, so every transformer that was
			// skipping this subtree starts seeing nodes again. This happens
			// before anything is asked, so that one transformer editing the
			// node cannot leave another stuck skipping for the rest of the
			// walk — graphql-js leaves it stuck, which makes the answer depend
			// on the order the transformers were given in.
			asking := make([]bool, len(transformers))
			for i := range transformers {
				switch {
				case broken[i]:
				case skipping[i] == nil:
					asking[i] = true
				case skipping[i] == node:
					skipping[i] = nil
				}
			}

			for i, t := range transformers {
				if !asking[i] || t.Leave == nil {
					continue
				}
				replacement, action := t.Leave(node, ctx)
				if action == TransformBreak {
					broken[i] = true
				}
				if !isAbsentNode(replacement) || action == TransformRemove {
					return replacement, action
				}
			}
			if live() == 0 {
				return nil, TransformBreak
			}
			return nil, TransformContinue
		},
	}
}

// transformFn is what a rewrite does with one child: it answers with the node
// that should stand in its place, or nil for one that should be dropped.
//
// It is passed to the helpers below rather than held in a struct so that the
// closure the walk builds for each node stays on the stack.
type transformFn func(child Node, key string, index int) Node

// transformChild replaces one optional child, reporting whether it changed.
//
// A replacement that cannot go here is recorded in failed, and nothing further
// is attempted. Go has no generic methods, so the sink is a parameter.
func transformChild[T Node](f transformFn, failed *error, key string, dst *T) bool {
	if *failed != nil || isAbsentNode(*dst) {
		return false
	}
	replacement := f(*dst, key, -1)
	if replacement == Node(*dst) {
		return false
	}
	var zero T
	if replacement == nil {
		*dst = zero
		return true
	}
	typed, isRight := replacement.(T)
	if !isRight {
		*failed = fmt.Errorf("graphql/language: %s must be a %s, not a %T",
			key, reflect.TypeFor[T](), replacement)
		return false
	}
	*dst = typed
	return true
}

// transformList replaces the children of a list, reporting whether any
// changed. A child dropped closes the gap behind it.
func transformList[T Node](f transformFn, failed *error, key string, dst *[]T) bool {
	if *failed != nil {
		return false
	}
	// The list is copied only once something in it actually changes, so a walk
	// that rewrites nothing allocates nothing.
	var kept []T
	for i, child := range *dst {
		var replacement Node
		if !isAbsentNode(child) {
			replacement = f(child, key, i)
		} else {
			replacement = child
		}

		if kept == nil {
			if replacement == Node(child) {
				continue // still the list that came in
			}
			kept = make([]T, 0, len(*dst))
			kept = append(kept, (*dst)[:i]...)
		}
		if replacement == nil {
			continue // dropped; the list closes up behind it
		}
		typed, isRight := replacement.(T)
		if !isRight {
			*failed = fmt.Errorf("graphql/language: %s must hold a %s, not a %T",
				key, reflect.TypeFor[T](), replacement)
			return false
		}
		kept = append(kept, typed)
	}
	if kept == nil {
		return false
	}
	*dst = kept
	return true
}

// transformChildren rebuilds a node around whatever the rewrite made of its
// children, or returns it unchanged when nothing did.
//
// This mirrors visitChildren field for field: the same table read the other
// way round. A node type added to one has to be added to the other, which is
// what the test comparing them against every kind is for.
func transformChildren(node Node, f transformFn, failed *error) Node {
	switch n := node.(type) {
	case *Document:
		copied := *n
		changed := false
		changed = transformList(f, failed, "Definitions", &copied.Definitions) || changed
		if !changed {
			return node
		}
		return &copied
	case *OperationDefinition:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Description", &copied.Description) || changed
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "VariableDefinitions", &copied.VariableDefinitions) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		changed = transformChild(f, failed, "SelectionSet", &copied.SelectionSet) || changed
		if !changed {
			return node
		}
		return &copied
	case *VariableDefinition:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Description", &copied.Description) || changed
		changed = transformChild(f, failed, "Variable", &copied.Variable) || changed
		changed = transformChild(f, failed, "Type", &copied.Type) || changed
		changed = transformChild(f, failed, "DefaultValue", &copied.DefaultValue) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		if !changed {
			return node
		}
		return &copied
	case *Variable:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		if !changed {
			return node
		}
		return &copied
	case *SelectionSet:
		copied := *n
		changed := false
		changed = transformList(f, failed, "Selections", &copied.Selections) || changed
		if !changed {
			return node
		}
		return &copied
	case *Field:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Alias", &copied.Alias) || changed
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "Arguments", &copied.Arguments) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		changed = transformChild(f, failed, "SelectionSet", &copied.SelectionSet) || changed
		if !changed {
			return node
		}
		return &copied
	case *Argument:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformChild(f, failed, "Value", &copied.Value) || changed
		if !changed {
			return node
		}
		return &copied
	case *FragmentArgument:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformChild(f, failed, "Value", &copied.Value) || changed
		if !changed {
			return node
		}
		return &copied
	case *FragmentSpread:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "Arguments", &copied.Arguments) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		if !changed {
			return node
		}
		return &copied
	case *InlineFragment:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "TypeCondition", &copied.TypeCondition) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		changed = transformChild(f, failed, "SelectionSet", &copied.SelectionSet) || changed
		if !changed {
			return node
		}
		return &copied
	case *FragmentDefinition:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Description", &copied.Description) || changed
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "VariableDefinitions", &copied.VariableDefinitions) || changed
		changed = transformChild(f, failed, "TypeCondition", &copied.TypeCondition) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		changed = transformChild(f, failed, "SelectionSet", &copied.SelectionSet) || changed
		if !changed {
			return node
		}
		return &copied
	case *ListValue:
		copied := *n
		changed := false
		changed = transformList(f, failed, "Values", &copied.Values) || changed
		if !changed {
			return node
		}
		return &copied
	case *ObjectValue:
		copied := *n
		changed := false
		changed = transformList(f, failed, "Fields", &copied.Fields) || changed
		if !changed {
			return node
		}
		return &copied
	case *ObjectField:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformChild(f, failed, "Value", &copied.Value) || changed
		if !changed {
			return node
		}
		return &copied
	case *Directive:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "Arguments", &copied.Arguments) || changed
		if !changed {
			return node
		}
		return &copied
	case *NamedType:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		if !changed {
			return node
		}
		return &copied
	case *ListType:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Type", &copied.Type) || changed
		if !changed {
			return node
		}
		return &copied
	case *NonNullType:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Type", &copied.Type) || changed
		if !changed {
			return node
		}
		return &copied
	case *SchemaDefinition:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Description", &copied.Description) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		changed = transformList(f, failed, "OperationTypes", &copied.OperationTypes) || changed
		if !changed {
			return node
		}
		return &copied
	case *OperationTypeDefinition:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Type", &copied.Type) || changed
		if !changed {
			return node
		}
		return &copied
	case *ScalarTypeDefinition:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Description", &copied.Description) || changed
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		if !changed {
			return node
		}
		return &copied
	case *ObjectTypeDefinition:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Description", &copied.Description) || changed
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "Interfaces", &copied.Interfaces) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		changed = transformList(f, failed, "Fields", &copied.Fields) || changed
		if !changed {
			return node
		}
		return &copied
	case *FieldDefinition:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Description", &copied.Description) || changed
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "Arguments", &copied.Arguments) || changed
		changed = transformChild(f, failed, "Type", &copied.Type) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		if !changed {
			return node
		}
		return &copied
	case *InputValueDefinition:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Description", &copied.Description) || changed
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformChild(f, failed, "Type", &copied.Type) || changed
		changed = transformChild(f, failed, "DefaultValue", &copied.DefaultValue) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		if !changed {
			return node
		}
		return &copied
	case *InterfaceTypeDefinition:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Description", &copied.Description) || changed
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "Interfaces", &copied.Interfaces) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		changed = transformList(f, failed, "Fields", &copied.Fields) || changed
		if !changed {
			return node
		}
		return &copied
	case *UnionTypeDefinition:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Description", &copied.Description) || changed
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		changed = transformList(f, failed, "Types", &copied.Types) || changed
		if !changed {
			return node
		}
		return &copied
	case *EnumTypeDefinition:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Description", &copied.Description) || changed
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		changed = transformList(f, failed, "Values", &copied.Values) || changed
		if !changed {
			return node
		}
		return &copied
	case *EnumValueDefinition:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Description", &copied.Description) || changed
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		if !changed {
			return node
		}
		return &copied
	case *InputObjectTypeDefinition:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Description", &copied.Description) || changed
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		changed = transformList(f, failed, "Fields", &copied.Fields) || changed
		if !changed {
			return node
		}
		return &copied
	case *DirectiveDefinition:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Description", &copied.Description) || changed
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "Arguments", &copied.Arguments) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		changed = transformList(f, failed, "Locations", &copied.Locations) || changed
		if !changed {
			return node
		}
		return &copied
	case *SchemaExtension:
		copied := *n
		changed := false
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		changed = transformList(f, failed, "OperationTypes", &copied.OperationTypes) || changed
		if !changed {
			return node
		}
		return &copied
	case *ScalarTypeExtension:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		if !changed {
			return node
		}
		return &copied
	case *ObjectTypeExtension:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "Interfaces", &copied.Interfaces) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		changed = transformList(f, failed, "Fields", &copied.Fields) || changed
		if !changed {
			return node
		}
		return &copied
	case *InterfaceTypeExtension:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "Interfaces", &copied.Interfaces) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		changed = transformList(f, failed, "Fields", &copied.Fields) || changed
		if !changed {
			return node
		}
		return &copied
	case *UnionTypeExtension:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		changed = transformList(f, failed, "Types", &copied.Types) || changed
		if !changed {
			return node
		}
		return &copied
	case *EnumTypeExtension:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		changed = transformList(f, failed, "Values", &copied.Values) || changed
		if !changed {
			return node
		}
		return &copied
	case *InputObjectTypeExtension:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		changed = transformList(f, failed, "Fields", &copied.Fields) || changed
		if !changed {
			return node
		}
		return &copied
	case *DirectiveExtension:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformList(f, failed, "Directives", &copied.Directives) || changed
		if !changed {
			return node
		}
		return &copied
	case *TypeCoordinate:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		if !changed {
			return node
		}
		return &copied
	case *MemberCoordinate:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformChild(f, failed, "MemberName", &copied.MemberName) || changed
		if !changed {
			return node
		}
		return &copied
	case *ArgumentCoordinate:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformChild(f, failed, "FieldName", &copied.FieldName) || changed
		changed = transformChild(f, failed, "ArgumentName", &copied.ArgumentName) || changed
		if !changed {
			return node
		}
		return &copied
	case *DirectiveCoordinate:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		if !changed {
			return node
		}
		return &copied
	case *DirectiveArgumentCoordinate:
		copied := *n
		changed := false
		changed = transformChild(f, failed, "Name", &copied.Name) || changed
		changed = transformChild(f, failed, "ArgumentName", &copied.ArgumentName) || changed
		if !changed {
			return node
		}
		return &copied
	}

	// Names, numbers, strings, booleans, nulls and enum values are leaves.
	return node
}
