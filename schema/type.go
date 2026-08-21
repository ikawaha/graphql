package schema

// Type is implemented by every GraphQL type.
//
// A type is either named, such as a scalar or an object, or a wrapper around
// another type: a list, or a non-null.
type Type interface {
	// String renders the type as it is written in a schema, such as "[Int!]!".
	String() string
	isType()
}

// NamedType is a type that has a name: a scalar, an object, an interface, a
// union, an enum or an input object.
type NamedType interface {
	Type
	// Name is the name the type is declared under.
	Name() string
	// Description is the documentation written for the type, if any.
	Description() string
	isNamedType()
}

// WrappingType wraps another type. [List] and [NonNull] are the only two.
type WrappingType interface {
	Type
	// Unwrap returns the type inside the wrapper.
	Unwrap() Type
	isWrappingType()
}

// namedInputType is implemented by the named types that may be used as input:
// scalars, enums and input objects.
type namedInputType interface {
	NamedType
	isInputType()
}

// namedOutputType is implemented by the named types that may be the type of a
// field: scalars, enums, objects, interfaces and unions.
type namedOutputType interface {
	NamedType
	isOutputType()
}

// LeafType is a type a selection ends at, meaning a scalar or an enum. Every
// leaf of a response is a value of one of these.
type LeafType interface {
	NamedType
	isLeafType()
}

// List is a type whose values are lists of another type.
type List struct {
	// OfType is the type of the list's elements.
	OfType Type
}

// NewList returns a list of the given type.
func NewList(of Type) *List { return &List{OfType: of} }

func (l *List) String() string {
	if l == nil || l.OfType == nil {
		return "[?]"
	}
	return "[" + l.OfType.String() + "]"
}

// Unwrap returns the element type.
func (l *List) Unwrap() Type { return l.OfType }

func (*List) isType()         {}
func (*List) isWrappingType() {}

// NonNull is a type whose values may not be null.
//
// The specification does not allow a non-null to wrap another non-null, so
// OfType is always a named type or a list. Go cannot express that in the field
// declaration, so [NewNonNull] enforces it instead.
type NonNull struct {
	// OfType is the type being made non-null.
	OfType Type
}

// NewNonNull returns a non-null of the given type.
//
// It panics if given a non-null, because a doubly non-null type has no meaning
// and no way to be written.
func NewNonNull(of Type) *NonNull {
	if _, ok := of.(*NonNull); ok {
		panic("graphql/schema: NewNonNull called on a type that is already non-null")
	}
	return &NonNull{OfType: of}
}

func (n *NonNull) String() string {
	if n == nil || n.OfType == nil {
		return "?!"
	}
	return n.OfType.String() + "!"
}

// Unwrap returns the type inside the non-null.
func (n *NonNull) Unwrap() Type { return n.OfType }

func (*NonNull) isType()         {}
func (*NonNull) isWrappingType() {}

// IsWrappingType reports whether a type is a list or a non-null.
func IsWrappingType(t Type) bool {
	_, ok := t.(WrappingType)
	return ok
}

// IsListType reports whether a type is a list.
func IsListType(t Type) bool {
	_, ok := t.(*List)
	return ok
}

// IsNonNullType reports whether a type is a non-null.
func IsNonNullType(t Type) bool {
	_, ok := t.(*NonNull)
	return ok
}

// IsNamedType reports whether a type is named rather than a wrapper.
func IsNamedType(t Type) bool {
	_, ok := t.(NamedType)
	return ok
}

// IsNullableType reports whether a value of the type may be null, which is
// true of everything except a non-null.
func IsNullableType(t Type) bool {
	return t != nil && !IsNonNullType(t)
}

// NullableTypeOf strips a non-null wrapper, if there is one.
func NullableTypeOf(t Type) Type {
	if n, ok := t.(*NonNull); ok {
		return n.OfType
	}
	return t
}

// NamedTypeOf strips every list and non-null wrapper and returns the named
// type at the centre, or nil if there is none.
func NamedTypeOf(t Type) NamedType {
	for {
		switch v := t.(type) {
		case WrappingType:
			t = v.Unwrap()
		case NamedType:
			return v
		default:
			return nil
		}
	}
}

// IsInputType reports whether a type may be used for an argument, a variable
// or an input object field.
//
// A wrapper is an input type when what it wraps is, so this has to look
// through the wrappers rather than being a single type assertion.
func IsInputType(t Type) bool {
	if w, ok := t.(WrappingType); ok {
		return IsInputType(w.Unwrap())
	}
	_, ok := t.(namedInputType)
	return ok
}

// IsOutputType reports whether a type may be the type of a field.
func IsOutputType(t Type) bool {
	if w, ok := t.(WrappingType); ok {
		return IsOutputType(w.Unwrap())
	}
	_, ok := t.(namedOutputType)
	return ok
}

// IsLeafType reports whether a type is one a selection ends at.
func IsLeafType(t Type) bool {
	_, ok := t.(LeafType)
	return ok
}
