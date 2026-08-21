package language

// NamedType refers to a type by name.
type NamedType struct {
	Loc  *Location
	Name *Name
}

func (*NamedType) Kind() Kind            { return KindNamedType }
func (n *NamedType) Location() *Location { return n.Loc }
func (*NamedType) isType()               {}

// ListType is a list of another type, written in brackets.
type ListType struct {
	Loc  *Location
	Type Type
}

func (*ListType) Kind() Kind            { return KindListType }
func (n *ListType) Location() *Location { return n.Loc }
func (*ListType) isType()               {}

// NonNullType marks another type as not nullable, written with a trailing
// exclamation mark.
//
// The grammar does not allow a non-null type to wrap another non-null type, so
// Type here is always a [NamedType] or a [ListType]. The field is declared as
// the general Type interface because Go cannot express that restriction. The
// parser never builds a doubly wrapped type, and code that assembles types by
// hand should not either.
type NonNullType struct {
	Loc  *Location
	Type Type
}

func (*NonNullType) Kind() Kind            { return KindNonNullType }
func (n *NonNullType) Location() *Location { return n.Loc }
func (*NonNullType) isType()               {}

// NamedTypeOf unwraps list and non-null wrappers and returns the named type at
// the centre, or nil if there is none.
func NamedTypeOf(t Type) *NamedType {
	for {
		switch v := t.(type) {
		case *NamedType:
			return v
		case *ListType:
			t = v.Type
		case *NonNullType:
			t = v.Type
		default:
			return nil
		}
	}
}
