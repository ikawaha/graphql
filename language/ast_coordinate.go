package language

// A schema coordinate names a single element of a schema, in the syntax used
// by tooling and documentation. The five node types below cover the forms the
// grammar allows:
//
//	Type                      a type
//	Type.field                a field, or an enum member, or an input field
//	Type.field(arg:)          an argument of a field
//	@directive                a directive
//	@directive(arg:)          an argument of a directive

// TypeCoordinate names a type.
type TypeCoordinate struct {
	Loc  *Location
	Name *Name
}

func (*TypeCoordinate) Kind() Kind            { return KindTypeCoordinate }
func (n *TypeCoordinate) Location() *Location { return n.Loc }
func (*TypeCoordinate) isSchemaCoordinate()   {}

// MemberCoordinate names a member of a type: a field, an enum member or an
// input object field.
type MemberCoordinate struct {
	Loc        *Location
	Name       *Name
	MemberName *Name
}

func (*MemberCoordinate) Kind() Kind            { return KindMemberCoordinate }
func (n *MemberCoordinate) Location() *Location { return n.Loc }
func (*MemberCoordinate) isSchemaCoordinate()   {}

// ArgumentCoordinate names an argument of a field.
type ArgumentCoordinate struct {
	Loc          *Location
	Name         *Name
	FieldName    *Name
	ArgumentName *Name
}

func (*ArgumentCoordinate) Kind() Kind            { return KindArgumentCoordinate }
func (n *ArgumentCoordinate) Location() *Location { return n.Loc }
func (*ArgumentCoordinate) isSchemaCoordinate()   {}

// DirectiveCoordinate names a directive.
type DirectiveCoordinate struct {
	Loc  *Location
	Name *Name
}

func (*DirectiveCoordinate) Kind() Kind            { return KindDirectiveCoordinate }
func (n *DirectiveCoordinate) Location() *Location { return n.Loc }
func (*DirectiveCoordinate) isSchemaCoordinate()   {}

// DirectiveArgumentCoordinate names an argument of a directive.
type DirectiveArgumentCoordinate struct {
	Loc          *Location
	Name         *Name
	ArgumentName *Name
}

func (*DirectiveArgumentCoordinate) Kind() Kind            { return KindDirectiveArgumentCoordinate }
func (n *DirectiveArgumentCoordinate) Location() *Location { return n.Loc }
func (*DirectiveArgumentCoordinate) isSchemaCoordinate()   {}
