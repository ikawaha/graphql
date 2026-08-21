package language

// Variable references a variable declared by the enclosing operation or
// fragment.
type Variable struct {
	Loc  *Location
	Name *Name
}

func (*Variable) Kind() Kind            { return KindVariable }
func (n *Variable) Location() *Location { return n.Loc }
func (*Variable) isValue()              {}

// IntValue is an integer literal.
//
// The digits are kept as written. Turning them into a Go number is the job of
// the type that receives the value, since the range that is acceptable depends
// on the scalar it is being coerced to.
type IntValue struct {
	Loc   *Location
	Value string
}

func (*IntValue) Kind() Kind            { return KindIntValue }
func (n *IntValue) Location() *Location { return n.Loc }
func (*IntValue) isValue()              {}

// FloatValue is a floating point literal. As with [IntValue] the text is kept
// as written.
type FloatValue struct {
	Loc   *Location
	Value string
}

func (*FloatValue) Kind() Kind            { return KindFloatValue }
func (n *FloatValue) Location() *Location { return n.Loc }
func (*FloatValue) isValue()              {}

// StringValue is a string literal, with escape sequences already resolved.
type StringValue struct {
	Loc   *Location
	Value string
	// Block reports whether the literal was written in the triple-quoted block
	// form. The value is the same either way; this only records how it was
	// spelled so that printing can preserve the choice.
	Block bool
}

func (*StringValue) Kind() Kind            { return KindStringValue }
func (n *StringValue) Location() *Location { return n.Loc }
func (*StringValue) isValue()              {}

// BooleanValue is true or false.
type BooleanValue struct {
	Loc   *Location
	Value bool
}

func (*BooleanValue) Kind() Kind            { return KindBooleanValue }
func (n *BooleanValue) Location() *Location { return n.Loc }
func (*BooleanValue) isValue()              {}

// NullValue is the null literal.
//
// A null literal is an explicit null, which the specification distinguishes
// from a value being left out altogether. In the AST that difference is the
// difference between a NullValue node and a nil Value.
type NullValue struct {
	Loc *Location
}

func (*NullValue) Kind() Kind            { return KindNullValue }
func (n *NullValue) Location() *Location { return n.Loc }
func (*NullValue) isValue()              {}

// EnumValue is an unquoted enum member name.
type EnumValue struct {
	Loc   *Location
	Value string
}

func (*EnumValue) Kind() Kind            { return KindEnumValue }
func (n *EnumValue) Location() *Location { return n.Loc }
func (*EnumValue) isValue()              {}

// ListValue is a bracketed list of values.
type ListValue struct {
	Loc    *Location
	Values []Value
}

func (*ListValue) Kind() Kind            { return KindListValue }
func (n *ListValue) Location() *Location { return n.Loc }
func (*ListValue) isValue()              {}

// ObjectValue is a braced list of input object fields.
type ObjectValue struct {
	Loc    *Location
	Fields []*ObjectField
}

func (*ObjectValue) Kind() Kind            { return KindObjectValue }
func (n *ObjectValue) Location() *Location { return n.Loc }
func (*ObjectValue) isValue()              {}

// ObjectField is one field of an [ObjectValue].
type ObjectField struct {
	Loc   *Location
	Name  *Name
	Value Value
}

func (*ObjectField) Kind() Kind            { return KindObjectField }
func (n *ObjectField) Location() *Location { return n.Loc }

// ContainsVariable reports whether a value references a variable anywhere
// within it.
//
// The grammar forbids variables in a constant value, such as a default value
// or an argument to a directive in a schema. The parser enforces that while
// reading a document; use this to check a value assembled in code.
func ContainsVariable(v Value) bool {
	switch v := v.(type) {
	case nil:
		return false
	case *Variable:
		return true
	case *ListValue:
		for _, item := range v.Values {
			if ContainsVariable(item) {
				return true
			}
		}
		return false
	case *ObjectValue:
		for _, f := range v.Fields {
			if f != nil && ContainsVariable(f.Value) {
				return true
			}
		}
		return false
	default:
		return false
	}
}
