package value

import (
	"strconv"
	"strings"
)

// Path is a linked response path from a field back to the root of the
// response. Each segment names either an object field or an index into a list.
//
// The list is linked towards the root, so appending a segment never mutates or
// copies the existing path. A nil *Path is the root and is ready to use:
//
//	p := (*Path)(nil).WithField("hero", "Query").WithIndex(0)
//	p.String() // ".hero[0]"
type Path struct {
	// Prev is the segment closer to the root, or nil at the root.
	Prev *Path
	// Key is the field name for an object segment, or the index for a list
	// segment. Exactly one of Key and Index is meaningful, as reported by
	// IsIndex.
	Key string
	// Index is the list index for a list segment.
	Index int
	// TypeName is the name of the object type that owns this segment, empty
	// when it is not known, which is the case for list indices.
	TypeName string

	isIndex bool
}

// WithField returns a new path with an object field segment appended.
// typeName is the object type that owns the field and may be empty.
func (p *Path) WithField(name, typeName string) *Path {
	return &Path{Prev: p, Key: name, TypeName: typeName}
}

// WithIndex returns a new path with a list index segment appended.
func (p *Path) WithIndex(i int) *Path {
	return &Path{Prev: p, Index: i, isIndex: true}
}

// IsIndex reports whether this segment is a list index rather than a field.
func (p *Path) IsIndex() bool {
	return p != nil && p.isIndex
}

// Len returns the number of segments, zero at the root.
func (p *Path) Len() int {
	n := 0
	for cur := p; cur != nil; cur = cur.Prev {
		n++
	}
	return n
}

// AsSlice flattens the path from root to leaf. Field segments contribute a
// string and list segments an int, which is the shape the errors entry of a
// GraphQL response requires. The root returns nil.
func (p *Path) AsSlice() []any {
	n := p.Len()
	if n == 0 {
		return nil
	}
	out := make([]any, n)
	for cur, i := p, n-1; cur != nil; cur, i = cur.Prev, i-1 {
		if cur.isIndex {
			out[i] = cur.Index
		} else {
			out[i] = cur.Key
		}
	}
	return out
}

// String renders the path for use in error messages, as in ".hero[0].name".
// The root renders as the empty string.
func (p *Path) String() string {
	if p == nil {
		return ""
	}
	var b strings.Builder
	writePath(&b, p)
	return b.String()
}

// writePath appends the segments to b in root-to-leaf order by recursing
// towards the root first.
func writePath(b *strings.Builder, p *Path) {
	if p == nil {
		return
	}
	writePath(b, p.Prev)
	if p.isIndex {
		b.WriteByte('[')
		b.WriteString(strconv.Itoa(p.Index))
		b.WriteByte(']')
		return
	}
	b.WriteByte('.')
	b.WriteString(p.Key)
}
