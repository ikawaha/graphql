package schema

// Declared is a type a schema names where a particular kind belongs.
//
// A document may name anything. `union U = String` and
// `type T implements SomeInput` both parse, and a schema built from either
// holds what was named rather than refusing to be built: a client asking a
// broken schema what it is must be told what it says, and [ValidateSchema]
// is what says what is wrong with it. graphql-js does the same, and its own
// validator guards against the kind it declared but did not get.
//
// The type parameter records what belonged there, so that reading a union's
// members as interfaces will not compile, and so that a schema written in Go
// cannot name the wrong kind at all: [Members] and [Implements] take the kind
// itself. Only [DeclareNamed] can put something else there, and that is what
// the schema builder reaches for while reading a document.
type Declared[T NamedType] struct {
	named NamedType
}

// Declare records a type of the kind that belongs. The compiler checks it
// here, which is the ordinary case.
func Declare[T NamedType](t T) Declared[T] {
	if isAbsentType(t) {
		return Declared[T]{}
	}
	return Declared[T]{named: t}
}

// DeclareNamed records whatever a document named, which may not be of the
// kind that belongs at all. [ValidateSchema] reports the ones that are not.
func DeclareNamed[T NamedType](named NamedType) Declared[T] {
	if isAbsentType(named) {
		return Declared[T]{}
	}
	return Declared[T]{named: named}
}

// Members is the members of a union, as a schema written in Go gives them.
func Members(types ...*ObjectType) []Declared[*ObjectType] {
	return declareAll[*ObjectType](types)
}

// Implements is the interfaces a type implements, as a schema written in Go
// gives them.
func Implements(interfaces ...*InterfaceType) []Declared[*InterfaceType] {
	return declareAll[*InterfaceType](interfaces)
}

// declareAll wraps a list of the kind that belongs. A nil among them is kept
// as one, since a list assembled in Go may have a hole in it and the schema
// check reports the hole rather than this quietly closing it.
func declareAll[T NamedType](types []T) []Declared[T] {
	if types == nil {
		return nil
	}
	out := make([]Declared[T], 0, len(types))
	for _, t := range types {
		out = append(out, Declare(t))
	}
	return out
}

// Name is the name that was written, or the empty string where nothing was.
func (d Declared[T]) Name() string {
	if d.named == nil {
		return ""
	}
	return d.named.Name()
}

// Named is what was written, whatever kind it turned out to be, or nil where
// nothing was written at all.
func (d Declared[T]) Named() NamedType { return d.named }

// IsSet reports whether anything was named here.
func (d Declared[T]) IsSet() bool { return d.named != nil }

// Get is the type where it is of the kind that belongs, and a false ok where
// it is not. A schema [ValidateSchema] has passed answers true for every one.
func (d Declared[T]) Get() (T, bool) {
	held, ok := d.named.(T)
	return held, ok
}

// String renders what was named, which is its name.
func (d Declared[T]) String() string {
	if d.named == nil {
		return "<nil>"
	}
	return d.named.String()
}

// namedTypes is what a list of declarations amounts to for whoever only wants
// the types, holes and all.
func namedTypes[T NamedType](declared []Declared[T]) []NamedType {
	out := make([]NamedType, 0, len(declared))
	for _, d := range declared {
		out = append(out, d.named)
	}
	return out
}
