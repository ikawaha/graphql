package language

// SchemaDefinition declares the root operation types of a schema.
type SchemaDefinition struct {
	Loc            *Location
	Description    *StringValue
	Directives     []*Directive
	OperationTypes []*OperationTypeDefinition
}

func (*SchemaDefinition) Kind() Kind            { return KindSchemaDefinition }
func (n *SchemaDefinition) Location() *Location { return n.Loc }
func (*SchemaDefinition) isDefinition()         {}
func (*SchemaDefinition) isTypeSystemDefinition() {
}

// OperationTypeDefinition binds one operation type to a named object type.
type OperationTypeDefinition struct {
	Loc       *Location
	Operation OperationType
	Type      *NamedType
}

func (*OperationTypeDefinition) Kind() Kind            { return KindOperationTypeDefinition }
func (n *OperationTypeDefinition) Location() *Location { return n.Loc }

// ScalarTypeDefinition defines a scalar type.
type ScalarTypeDefinition struct {
	Loc         *Location
	Description *StringValue
	Name        *Name
	Directives  []*Directive
}

func (*ScalarTypeDefinition) Kind() Kind            { return KindScalarTypeDefinition }
func (n *ScalarTypeDefinition) Location() *Location { return n.Loc }
func (*ScalarTypeDefinition) isDefinition()         {}
func (*ScalarTypeDefinition) isTypeSystemDefinition() {
}
func (*ScalarTypeDefinition) isTypeDefinition() {}

// ObjectTypeDefinition defines an object type.
type ObjectTypeDefinition struct {
	Loc         *Location
	Description *StringValue
	Name        *Name
	Interfaces  []*NamedType
	Directives  []*Directive
	Fields      []*FieldDefinition
}

func (*ObjectTypeDefinition) Kind() Kind            { return KindObjectTypeDefinition }
func (n *ObjectTypeDefinition) Location() *Location { return n.Loc }
func (*ObjectTypeDefinition) isDefinition()         {}
func (*ObjectTypeDefinition) isTypeSystemDefinition() {
}
func (*ObjectTypeDefinition) isTypeDefinition() {}

// FieldDefinition defines one field of an object or interface type.
type FieldDefinition struct {
	Loc         *Location
	Description *StringValue
	Name        *Name
	Arguments   []*InputValueDefinition
	Type        Type
	Directives  []*Directive
}

func (*FieldDefinition) Kind() Kind            { return KindFieldDefinition }
func (n *FieldDefinition) Location() *Location { return n.Loc }

// InputValueDefinition defines an argument or an input object field.
type InputValueDefinition struct {
	Loc         *Location
	Description *StringValue
	Name        *Name
	Type        Type
	// DefaultValue is the value used when the caller leaves this input out.
	// A nil DefaultValue means there is no default at all, which is different
	// from a default of null, written as a [NullValue] node.
	DefaultValue Value
	Directives   []*Directive
}

func (*InputValueDefinition) Kind() Kind            { return KindInputValueDefinition }
func (n *InputValueDefinition) Location() *Location { return n.Loc }

// InterfaceTypeDefinition defines an interface type.
type InterfaceTypeDefinition struct {
	Loc         *Location
	Description *StringValue
	Name        *Name
	Interfaces  []*NamedType
	Directives  []*Directive
	Fields      []*FieldDefinition
}

func (*InterfaceTypeDefinition) Kind() Kind            { return KindInterfaceTypeDefinition }
func (n *InterfaceTypeDefinition) Location() *Location { return n.Loc }
func (*InterfaceTypeDefinition) isDefinition()         {}
func (*InterfaceTypeDefinition) isTypeSystemDefinition() {
}
func (*InterfaceTypeDefinition) isTypeDefinition() {}

// UnionTypeDefinition defines a union type.
type UnionTypeDefinition struct {
	Loc         *Location
	Description *StringValue
	Name        *Name
	Directives  []*Directive
	Types       []*NamedType
}

func (*UnionTypeDefinition) Kind() Kind            { return KindUnionTypeDefinition }
func (n *UnionTypeDefinition) Location() *Location { return n.Loc }
func (*UnionTypeDefinition) isDefinition()         {}
func (*UnionTypeDefinition) isTypeSystemDefinition() {
}
func (*UnionTypeDefinition) isTypeDefinition() {}

// EnumTypeDefinition defines an enum type.
type EnumTypeDefinition struct {
	Loc         *Location
	Description *StringValue
	Name        *Name
	Directives  []*Directive
	Values      []*EnumValueDefinition
}

func (*EnumTypeDefinition) Kind() Kind            { return KindEnumTypeDefinition }
func (n *EnumTypeDefinition) Location() *Location { return n.Loc }
func (*EnumTypeDefinition) isDefinition()         {}
func (*EnumTypeDefinition) isTypeSystemDefinition() {
}
func (*EnumTypeDefinition) isTypeDefinition() {}

// EnumValueDefinition defines one member of an enum type.
type EnumValueDefinition struct {
	Loc         *Location
	Description *StringValue
	Name        *Name
	Directives  []*Directive
}

func (*EnumValueDefinition) Kind() Kind            { return KindEnumValueDefinition }
func (n *EnumValueDefinition) Location() *Location { return n.Loc }

// InputObjectTypeDefinition defines an input object type.
type InputObjectTypeDefinition struct {
	Loc         *Location
	Description *StringValue
	Name        *Name
	Directives  []*Directive
	Fields      []*InputValueDefinition
}

func (*InputObjectTypeDefinition) Kind() Kind            { return KindInputObjectTypeDefinition }
func (n *InputObjectTypeDefinition) Location() *Location { return n.Loc }
func (*InputObjectTypeDefinition) isDefinition()         {}
func (*InputObjectTypeDefinition) isTypeSystemDefinition() {
}
func (*InputObjectTypeDefinition) isTypeDefinition() {}

// DirectiveDefinition defines a directive and where it may be applied.
type DirectiveDefinition struct {
	Loc         *Location
	Description *StringValue
	Name        *Name
	Arguments   []*InputValueDefinition
	Directives  []*Directive
	// Repeatable reports whether the directive may be applied more than once
	// in the same location.
	Repeatable bool
	// Locations names the places the directive may be applied. Each name must
	// be one of the [DirectiveLocation] values.
	Locations []*Name
}

func (*DirectiveDefinition) Kind() Kind            { return KindDirectiveDefinition }
func (n *DirectiveDefinition) Location() *Location { return n.Loc }
func (*DirectiveDefinition) isDefinition()         {}
func (*DirectiveDefinition) isTypeSystemDefinition() {
}

// SchemaExtension adds directives or root operation types to a schema.
type SchemaExtension struct {
	Loc            *Location
	Directives     []*Directive
	OperationTypes []*OperationTypeDefinition
}

func (*SchemaExtension) Kind() Kind            { return KindSchemaExtension }
func (n *SchemaExtension) Location() *Location { return n.Loc }
func (*SchemaExtension) isDefinition()         {}
func (*SchemaExtension) isTypeSystemExtension() {
}

// ScalarTypeExtension extends a scalar type.
type ScalarTypeExtension struct {
	Loc        *Location
	Name       *Name
	Directives []*Directive
}

func (*ScalarTypeExtension) Kind() Kind            { return KindScalarTypeExtension }
func (n *ScalarTypeExtension) Location() *Location { return n.Loc }
func (*ScalarTypeExtension) isDefinition()         {}
func (*ScalarTypeExtension) isTypeSystemExtension() {
}
func (*ScalarTypeExtension) isTypeExtension() {}

// ObjectTypeExtension extends an object type.
type ObjectTypeExtension struct {
	Loc        *Location
	Name       *Name
	Interfaces []*NamedType
	Directives []*Directive
	Fields     []*FieldDefinition
}

func (*ObjectTypeExtension) Kind() Kind            { return KindObjectTypeExtension }
func (n *ObjectTypeExtension) Location() *Location { return n.Loc }
func (*ObjectTypeExtension) isDefinition()         {}
func (*ObjectTypeExtension) isTypeSystemExtension() {
}
func (*ObjectTypeExtension) isTypeExtension() {}

// InterfaceTypeExtension extends an interface type.
type InterfaceTypeExtension struct {
	Loc        *Location
	Name       *Name
	Interfaces []*NamedType
	Directives []*Directive
	Fields     []*FieldDefinition
}

func (*InterfaceTypeExtension) Kind() Kind            { return KindInterfaceTypeExtension }
func (n *InterfaceTypeExtension) Location() *Location { return n.Loc }
func (*InterfaceTypeExtension) isDefinition()         {}
func (*InterfaceTypeExtension) isTypeSystemExtension() {
}
func (*InterfaceTypeExtension) isTypeExtension() {}

// UnionTypeExtension extends a union type.
type UnionTypeExtension struct {
	Loc        *Location
	Name       *Name
	Directives []*Directive
	Types      []*NamedType
}

func (*UnionTypeExtension) Kind() Kind            { return KindUnionTypeExtension }
func (n *UnionTypeExtension) Location() *Location { return n.Loc }
func (*UnionTypeExtension) isDefinition()         {}
func (*UnionTypeExtension) isTypeSystemExtension() {
}
func (*UnionTypeExtension) isTypeExtension() {}

// EnumTypeExtension extends an enum type.
type EnumTypeExtension struct {
	Loc        *Location
	Name       *Name
	Directives []*Directive
	Values     []*EnumValueDefinition
}

func (*EnumTypeExtension) Kind() Kind            { return KindEnumTypeExtension }
func (n *EnumTypeExtension) Location() *Location { return n.Loc }
func (*EnumTypeExtension) isDefinition()         {}
func (*EnumTypeExtension) isTypeSystemExtension() {
}
func (*EnumTypeExtension) isTypeExtension() {}

// InputObjectTypeExtension extends an input object type.
type InputObjectTypeExtension struct {
	Loc        *Location
	Name       *Name
	Directives []*Directive
	Fields     []*InputValueDefinition
}

func (*InputObjectTypeExtension) Kind() Kind            { return KindInputObjectTypeExtension }
func (n *InputObjectTypeExtension) Location() *Location { return n.Loc }
func (*InputObjectTypeExtension) isDefinition()         {}
func (*InputObjectTypeExtension) isTypeSystemExtension() {
}
func (*InputObjectTypeExtension) isTypeExtension() {}

// DirectiveExtension extends a directive definition.
type DirectiveExtension struct {
	Loc        *Location
	Name       *Name
	Directives []*Directive
}

func (*DirectiveExtension) Kind() Kind            { return KindDirectiveExtension }
func (n *DirectiveExtension) Location() *Location { return n.Loc }
func (*DirectiveExtension) isDefinition()         {}
func (*DirectiveExtension) isTypeSystemExtension() {
}
