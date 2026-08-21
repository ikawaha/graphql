// Package typeinfo says where in a schema a walk of a document currently is.
//
// It sits below both validation and the schema builder because each needs it
// and they need each other: the builder checks every document it is given
// against the rules a schema definition must follow, and those rules are
// written in terms of the types a walk is passing through. graphql-js has the
// same pair pointing at one another and leaves the cycle to the module loader;
// Go does not allow one, so what they share lives here.
//
// [TypeInfo] and [TypeFromAST] are re-exported from the utilities package
// under the names graphql-js gives them, which is where a caller reaches for
// them.
package typeinfo

import (
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// TypeInfo keeps track of where in a schema a walk of a document currently is.
//
// A document on its own says almost nothing about types: a field is a name,
// and what it returns depends on the type it was selected from, which depends
// in turn on everything above it. TypeInfo follows a walk and answers those
// questions at each step, which is what lets a validation rule work in terms
// of types rather than of names.
//
// It is driven by [VisitWithTypeInfo], which keeps it in step with the walk.
// Driving it by hand means calling Enter and Leave for every node, in the same
// order the walk does; the two must be balanced or the answers drift.
//
// A question with no answer at the current position comes back nil. That
// happens whenever the document does not match the schema, which is exactly
// the situation validation exists to report, so it is ordinary rather than
// exceptional.
type TypeInfo struct {
	schema *schema.Schema

	// Each stack rises and falls with the walk. A level is pushed even when
	// nothing could be resolved for it, so that the matching pop always finds
	// its own entry.
	typeStack         []schema.Type
	parentTypeStack   []schema.CompositeType
	inputTypeStack    []schema.Type
	fieldDefStack     []*schema.Field
	defaultValueStack []value.Maybe[schema.DefaultInput]

	directive *schema.Directive
	argument  *schema.Argument
	enumValue *schema.EnumValue

	// signatures says what each of the document's fragments declares it takes,
	// which is what lets a spread's arguments be answered for. It is nil where
	// the TypeInfo was made without a document.
	signatures map[string]*FragmentSignature
	// signature is the fragment the walk is inside a spread of, and
	// fragmentArgument the declaration the argument it is inside answers to.
	signature        *FragmentSignature
	fragmentArgument *language.VariableDefinition
}

// FragmentSignature is what a fragment declares it takes.
//
// A fragment may take arguments the way an operation takes variables, which is
// experimental: the parser reads them only when asked with
// [language.ExperimentalFragmentArguments].
type FragmentSignature struct {
	// Definition is the fragment itself.
	Definition *language.FragmentDefinition
	// Variables are what it declares, by name.
	Variables map[string]*language.VariableDefinition
}

// NewTypeInfo returns a TypeInfo for walking a document against a schema.
//
// It cannot answer for the arguments of a fragment spread, since it does not
// know what the document's fragments declare; use [NewTypeInfoForDocument] for
// that.
func NewTypeInfo(s *schema.Schema) *TypeInfo { return &TypeInfo{schema: s} }

// NewTypeInfoForDocument returns a TypeInfo that also knows what the
// document's fragments declare, so that it can answer for the arguments a
// spread gives one.
func NewTypeInfoForDocument(s *schema.Schema, doc *language.Document) *TypeInfo {
	info := &TypeInfo{schema: s, signatures: map[string]*FragmentSignature{}}
	if doc == nil {
		return info
	}
	for _, def := range doc.Definitions {
		fragment, isFragment := def.(*language.FragmentDefinition)
		if !isFragment || fragment.Name == nil {
			continue
		}
		if _, taken := info.signatures[fragment.Name.Value]; taken {
			// A name declared twice is a separate complaint; the first is the
			// one used, so that the rest has something to work with.
			continue
		}
		variables := map[string]*language.VariableDefinition{}
		for _, v := range fragment.VariableDefinitions {
			if v != nil && v.Variable != nil && v.Variable.Name != nil {
				variables[v.Variable.Name.Value] = v
			}
		}
		info.signatures[fragment.Name.Value] = &FragmentSignature{
			Definition: fragment,
			Variables:  variables,
		}
	}
	return info
}

// FragmentSignature is what the fragment the walk is inside a spread of
// declares, or nil.
func (t *TypeInfo) FragmentSignature() *FragmentSignature { return t.signature }

// FragmentArgument is the declaration the fragment argument the walk is inside
// answers to, or nil.
func (t *TypeInfo) FragmentArgument() *language.VariableDefinition { return t.fragmentArgument }

// Schema is the schema the walk is being read against.
func (t *TypeInfo) Schema() *schema.Schema { return t.schema }

// Type is the type of the field the walk is inside, or nil.
func (t *TypeInfo) Type() schema.Type { return lastOf(t.typeStack) }

// ParentType is the composite type whose selection set the walk is inside, or
// nil.
func (t *TypeInfo) ParentType() schema.CompositeType { return lastOf(t.parentTypeStack) }

// InputType is the type of the value the walk is inside: an argument, a
// variable's declared type, a list element or an input object field.
func (t *TypeInfo) InputType() schema.Type { return lastOf(t.inputTypeStack) }

// ParentInputType is the input type one level out, which is what a list or an
// input object field sits inside.
func (t *TypeInfo) ParentInputType() schema.Type {
	if len(t.inputTypeStack) < 2 {
		return nil
	}
	return t.inputTypeStack[len(t.inputTypeStack)-2]
}

// FieldDef is the definition of the field the walk is inside, or nil.
func (t *TypeInfo) FieldDef() *schema.Field { return lastOf(t.fieldDefStack) }

// DefaultValue is the default of the argument or input field the walk is
// inside. An unset result means there is no default, which is not the same as
// a default of null.
func (t *TypeInfo) DefaultValue() value.Maybe[schema.DefaultInput] {
	if len(t.defaultValueStack) == 0 {
		return value.Nothing[schema.DefaultInput]()
	}
	return t.defaultValueStack[len(t.defaultValueStack)-1]
}

// Directive is the directive the walk is inside, or nil.
func (t *TypeInfo) Directive() *schema.Directive { return t.directive }

// Argument is the argument the walk is inside, or nil.
func (t *TypeInfo) Argument() *schema.Argument { return t.argument }

// EnumValue is the enum member the walk is looking at, or nil.
func (t *TypeInfo) EnumValue() *schema.EnumValue { return t.enumValue }

// lastOf returns the top of a stack, or the zero value when it is empty.
func lastOf[T any](stack []T) T {
	if len(stack) == 0 {
		var zero T
		return zero
	}
	return stack[len(stack)-1]
}

// popOf removes the top of a stack, doing nothing when it is already empty.
func popOf[T any](stack []T) []T {
	if len(stack) == 0 {
		return stack
	}
	return stack[:len(stack)-1]
}

// Enter updates the position on the way into a node.
func (t *TypeInfo) Enter(node language.Node) {
	switch n := node.(type) {
	case *language.SelectionSet:
		// A selection set is written against whatever the enclosing field
		// returns, with its wrappers stripped.
		named := schema.NamedTypeOf(t.Type())
		composite, _ := named.(schema.CompositeType)
		t.parentTypeStack = append(t.parentTypeStack, composite)

	case *language.Field:
		var fieldDef *schema.Field
		var fieldType schema.Type
		if parent := t.ParentType(); parent != nil && n.Name != nil {
			fieldDef = t.schema.Field(parent, n.Name.Value)
			if fieldDef != nil {
				fieldType = fieldDef.Type
			}
		}
		t.fieldDefStack = append(t.fieldDefStack, fieldDef)
		t.typeStack = append(t.typeStack, outputOrNil(fieldType))

	case *language.Directive:
		t.directive = nil
		if t.schema != nil && n.Name != nil {
			t.directive = t.schema.Directive(n.Name.Value)
		}

	case *language.OperationDefinition:
		var root schema.Type
		if t.schema != nil {
			if r := t.schema.RootType(n.Operation); r != nil {
				root = r
			}
		}
		t.typeStack = append(t.typeStack, root)

	case *language.InlineFragment:
		t.typeStack = append(t.typeStack, t.typeConditionOf(n.TypeCondition))

	case *language.FragmentDefinition:
		t.typeStack = append(t.typeStack, t.typeConditionOf(n.TypeCondition))

	case *language.VariableDefinition:
		t.inputTypeStack = append(t.inputTypeStack, t.inputTypeOf(n.Type))

	case *language.Argument:
		t.enterArgument(n)

	case *language.FragmentSpread:
		// Inside a spread, an argument is answered for by what the fragment
		// declares.
		t.signature = nil
		if n.Name != nil {
			t.signature = t.signatures[n.Name.Value]
		}

	case *language.FragmentArgument:
		t.enterFragmentArgument(n)

	case *language.ListValue:
		// Inside a list, values are of its element type. Where the expected
		// type is not a list at all there is no element type, and the position
		// is left empty; whatever reports that a list does not belong here
		// reads the type one level out.
		var itemType schema.Type
		if list, isList := schema.NullableTypeOf(t.InputType()).(*schema.List); isList {
			itemType = list.OfType
		}
		t.defaultValueStack = append(t.defaultValueStack, value.Nothing[schema.DefaultInput]())
		t.inputTypeStack = append(t.inputTypeStack, inputOrNil(itemType))

	case *language.ObjectField:
		t.enterObjectField(n)

	case *language.EnumValue:
		t.enumValue = nil
		if enum, isEnum := schema.NamedTypeOf(t.InputType()).(*schema.EnumType); isEnum {
			t.enumValue = enum.Value(n.Value)
		}
	}
}

// enterArgument resolves the argument of whichever field or directive the walk
// is inside.
func (t *TypeInfo) enterArgument(n *language.Argument) {
	var argDef *schema.Argument
	if n.Name != nil {
		// A directive's arguments take precedence: inside @skip(if:) the walk
		// is in the directive, not in the field it is attached to.
		if d := t.Directive(); d != nil {
			argDef = d.Arg(n.Name.Value)
		} else if f := t.FieldDef(); f != nil {
			argDef = f.Arg(n.Name.Value)
		}
	}
	t.argument = argDef

	if argDef == nil {
		t.defaultValueStack = append(t.defaultValueStack, value.Nothing[schema.DefaultInput]())
		t.inputTypeStack = append(t.inputTypeStack, nil)
		return
	}
	t.defaultValueStack = append(t.defaultValueStack, argDef.Default)
	t.inputTypeStack = append(t.inputTypeStack, inputOrNil(argDef.Type))
}

// enterFragmentArgument resolves an argument a spread gives a fragment against
// what the fragment declares.
func (t *TypeInfo) enterFragmentArgument(n *language.FragmentArgument) {
	var declared *language.VariableDefinition
	if t.signature != nil && n.Name != nil {
		declared = t.signature.Variables[n.Name.Value]
	}
	t.fragmentArgument = declared

	if declared == nil {
		t.defaultValueStack = append(t.defaultValueStack, value.Nothing[schema.DefaultInput]())
		t.inputTypeStack = append(t.inputTypeStack, nil)
		return
	}
	t.defaultValueStack = append(t.defaultValueStack, defaultOf(declared.DefaultValue))
	t.inputTypeStack = append(t.inputTypeStack, t.inputTypeOf(declared.Type))
}

// enterObjectField resolves a field of the input object the walk is inside.
func (t *TypeInfo) enterObjectField(n *language.ObjectField) {
	var field *schema.InputField
	if object, isObject := schema.NamedTypeOf(t.InputType()).(*schema.InputObjectType); isObject && n.Name != nil {
		field = object.Field(n.Name.Value)
	}
	if field == nil {
		t.defaultValueStack = append(t.defaultValueStack, value.Nothing[schema.DefaultInput]())
		t.inputTypeStack = append(t.inputTypeStack, nil)
		return
	}
	t.defaultValueStack = append(t.defaultValueStack, field.Default)
	t.inputTypeStack = append(t.inputTypeStack, inputOrNil(field.Type))
}

// typeConditionOf resolves a fragment's type condition, falling back to the
// enclosing type when the fragment has none.
func (t *TypeInfo) typeConditionOf(condition *language.NamedType) schema.Type {
	if condition == nil {
		return outputOrNil(schema.NamedTypeOf(t.Type()))
	}
	resolved, ok := TypeFromAST(t.schema, condition)
	if !ok {
		return nil
	}
	return outputOrNil(resolved)
}

// inputTypeOf resolves a type reference that must be usable as input.
func (t *TypeInfo) inputTypeOf(node language.Type) schema.Type {
	resolved, ok := TypeFromAST(t.schema, node)
	if !ok {
		return nil
	}
	return inputOrNil(resolved)
}

// outputOrNil keeps a type only if a field could return it.
func outputOrNil(t schema.Type) schema.Type {
	if t == nil || !schema.IsOutputType(t) {
		return nil
	}
	return t
}

// inputOrNil keeps a type only if a value could be given for it.
func inputOrNil(t schema.Type) schema.Type {
	if t == nil || !schema.IsInputType(t) {
		return nil
	}
	return t
}

// Leave undoes what Enter did for the same node.
func (t *TypeInfo) Leave(node language.Node) {
	switch node.(type) {
	case *language.SelectionSet:
		t.parentTypeStack = popOf(t.parentTypeStack)

	case *language.Field:
		t.fieldDefStack = popOf(t.fieldDefStack)
		t.typeStack = popOf(t.typeStack)

	case *language.Directive:
		t.directive = nil

	case *language.OperationDefinition, *language.InlineFragment, *language.FragmentDefinition:
		t.typeStack = popOf(t.typeStack)

	case *language.VariableDefinition:
		t.inputTypeStack = popOf(t.inputTypeStack)

	case *language.Argument:
		t.argument = nil
		t.defaultValueStack = popOf(t.defaultValueStack)
		t.inputTypeStack = popOf(t.inputTypeStack)

	case *language.FragmentSpread:
		t.signature = nil

	case *language.FragmentArgument:
		t.fragmentArgument = nil
		t.defaultValueStack = popOf(t.defaultValueStack)
		t.inputTypeStack = popOf(t.inputTypeStack)

	case *language.ListValue, *language.ObjectField:
		t.defaultValueStack = popOf(t.defaultValueStack)
		t.inputTypeStack = popOf(t.inputTypeStack)

	case *language.EnumValue:
		t.enumValue = nil
	}
}

// VisitWithTypeInfo wraps a visitor so that the given [TypeInfo] follows the
// walk, letting the visitor ask what type it is looking at.
//
// A visitor that skips a subtree, or ends the walk, is accounted for: the
// position is unwound for that node, because the walker will not leave a node
// it never went into.
func VisitWithTypeInfo(info *TypeInfo, visitor language.Visitor) language.Visitor {
	return language.Visitor{
		Enter: func(node language.Node, ctx language.VisitContext) language.VisitAction {
			info.Enter(node)
			if visitor.Enter == nil {
				return language.VisitContinue
			}
			action := visitor.Enter(node, ctx)
			if action != language.VisitContinue {
				// The walk will not descend, and so will not leave this node.
				info.Leave(node)
			}
			return action
		},
		Leave: func(node language.Node, ctx language.VisitContext) language.VisitAction {
			action := language.VisitContinue
			if visitor.Leave != nil {
				action = visitor.Leave(node, ctx)
			}
			// The position is dropped either way: a walk that ends here has
			// still finished with this node.
			info.Leave(node)
			return action
		},
	}
}

// defaultOf reads a default written as a literal, which is how a document
// holds one. The schema builder has the same reading of it; a default is a
// literal or it is absent, and there is nothing else to decide.
func defaultOf(node language.Value) value.Maybe[schema.DefaultInput] {
	if node == nil {
		return schema.NoDefault()
	}
	return schema.DefaultLiteral(node)
}
