package typeinfo

// StackDepth is how far a walk is currently nested on each of the stacks
// TypeInfo keeps.
type StackDepth struct {
	Types, Parents, Inputs, Fields, Defaults int
}

// StackDepths reports where the walk currently is on each stack.
//
// It is exported for this package's external tests, which build a schema from
// SDL and so must sit above the builder, and which say what a walk left
// behind when it did not unwind.
func (t *TypeInfo) StackDepths() StackDepth {
	return StackDepth{
		Types:    len(t.typeStack),
		Parents:  len(t.parentTypeStack),
		Inputs:   len(t.inputTypeStack),
		Fields:   len(t.fieldDefStack),
		Defaults: len(t.defaultValueStack),
	}
}

// IsBalanced reports whether every level pushed on the way in was popped on
// the way out.
func (t *TypeInfo) IsBalanced() bool { return t.StackDepths() == StackDepth{} }
