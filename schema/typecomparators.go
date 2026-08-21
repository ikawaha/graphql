package schema

// IsTypeSubTypeOf reports whether a value of maybeSubType is always acceptable
// where superType is expected.
//
// This is what lets an object type implement an interface with a narrower
// field type, and what lets a variable of one type be passed where another is
// wanted. Three things make a type acceptable in place of another: being the
// same type, being a non-null version of it, and being one of the types an
// abstract type could turn out to be.
func IsTypeSubTypeOf(s *Schema, maybeSubType, superType Type) bool {
	if maybeSubType == nil || superType == nil {
		return false
	}
	if maybeSubType == superType {
		return true
	}

	// A non-null is acceptable where a nullable is wanted, but not the other
	// way round: promising a value is stronger than promising maybe a value.
	if super, ok := superType.(*NonNull); ok {
		sub, ok := maybeSubType.(*NonNull)
		if !ok {
			return false
		}
		return IsTypeSubTypeOf(s, sub.OfType, super.OfType)
	}
	if sub, ok := maybeSubType.(*NonNull); ok {
		return IsTypeSubTypeOf(s, sub.OfType, superType)
	}

	// A list is acceptable only where a list is wanted, and then only if its
	// elements are.
	if super, ok := superType.(*List); ok {
		sub, ok := maybeSubType.(*List)
		if !ok {
			return false
		}
		return IsTypeSubTypeOf(s, sub.OfType, super.OfType)
	}
	if _, ok := maybeSubType.(*List); ok {
		return false
	}

	// Finally, a type is acceptable where an abstract type is wanted if it is
	// one of the things that abstract type could be.
	abstract, ok := superType.(AbstractType)
	if !ok || s == nil {
		return false
	}
	named, ok := maybeSubType.(NamedType)
	if !ok {
		return false
	}
	return s.IsSubType(abstract, named)
}

// DoTypesOverlap reports whether a value could be of both types at once.
//
// Validation uses this to decide whether a fragment can be spread into a
// selection: a fragment on a type that could never be the type being selected
// is pointless and is reported rather than silently never matching.
func DoTypesOverlap(s *Schema, a, b CompositeType) bool {
	if a == nil || b == nil {
		return false
	}
	if a == b {
		return true
	}

	abstractA, aIsAbstract := a.(AbstractType)
	abstractB, bIsAbstract := b.(AbstractType)

	switch {
	case aIsAbstract && bIsAbstract:
		// Two abstract types overlap if anything at all could be both.
		for _, possible := range s.PossibleTypes(abstractA) {
			if s.IsSubType(abstractB, possible) {
				return true
			}
		}
		return false
	case aIsAbstract:
		return s.IsSubType(abstractA, b)
	case bIsAbstract:
		return s.IsSubType(abstractB, a)
	default:
		// Two different object types never overlap: a value is one or the
		// other.
		return false
	}
}

// IsEqualType reports whether two types are written identically.
//
// It is stricter than [IsTypeSubTypeOf]: Int! is a sub-type of Int but is not
// the same type, and an argument's type has to match exactly where a field's
// may narrow.
func IsEqualType(a, b Type) bool {
	switch left := a.(type) {
	case nil:
		return b == nil
	case *NonNull:
		right, ok := b.(*NonNull)
		return ok && IsEqualType(left.OfType, right.OfType)
	case *List:
		right, ok := b.(*List)
		return ok && IsEqualType(left.OfType, right.OfType)
	default:
		return a == b
	}
}
