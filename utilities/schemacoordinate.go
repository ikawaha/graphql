package utilities

import (
	"fmt"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// ResolvedCoordinate is what a schema coordinate names.
//
// Which members are set depends on the form of the coordinate: `Type` sets
// only Type, `Type.field` sets Type and one of Field, InputField or EnumValue,
// and the argument forms set Argument as well. A caller that knows which form
// it asked about can read the member it wants; one that does not can switch on
// what is set.
type ResolvedCoordinate struct {
	// Type is the type named, for every form but the directive ones.
	Type schema.NamedType
	// Field is the field named, where the type has fields.
	Field *schema.Field
	// InputField is the input object field named.
	InputField *schema.InputField
	// EnumValue is the enum member named.
	EnumValue *schema.EnumValue
	// Directive is the directive named, for the directive forms.
	Directive *schema.Directive
	// Argument is the argument named, for the two argument forms.
	Argument *schema.Argument
}

// ResolveSchemaCoordinate finds what a coordinate names in a schema.
//
// A coordinate is how tooling and documentation refer to one element of a
// schema — `User.name`, `User.friends(first:)`, `@deprecated(reason:)` — and
// resolving one is how a link in a document becomes the thing it points at.
//
// It returns nil where the schema has no such element, which is an ordinary
// answer rather than a fault: a coordinate may be written about a schema that
// has since changed. An error is for a coordinate that is not a question about
// this schema at all — one leading with a type it does not have, or with a
// type of the wrong sort for what follows the dot.
func ResolveSchemaCoordinate(s *schema.Schema, coordinate string) (*ResolvedCoordinate, error) {
	node, err := language.ParseSchemaCoordinate(language.NewSource(coordinate))
	if err != nil {
		return nil, err
	}
	return ResolveSchemaCoordinateNode(s, node)
}

// ResolveSchemaCoordinateNode finds what an already parsed coordinate names.
func ResolveSchemaCoordinateNode(
	s *schema.Schema,
	node language.SchemaCoordinate,
) (*ResolvedCoordinate, error) {
	// A schema that is not there has nothing to name, which is an answer
	// rather than a fault.
	if s == nil {
		return nil, nil
	}

	switch n := node.(type) {
	case *language.TypeCoordinate:
		t := s.Type(nameOf(n.Name))
		if t == nil {
			return nil, nil
		}
		return &ResolvedCoordinate{Type: t}, nil

	case *language.MemberCoordinate:
		t, err := namedType(s, nameOf(n.Name))
		if err != nil {
			return nil, err
		}
		return resolveMember(s, t, nameOf(n.MemberName))

	case *language.ArgumentCoordinate:
		t, err := namedType(s, nameOf(n.Name))
		if err != nil {
			return nil, err
		}
		field, err := fieldOf(s, t, nameOf(n.FieldName))
		if err != nil || field == nil {
			return nil, err
		}
		arg := field.Arg(nameOf(n.ArgumentName))
		if arg == nil {
			return nil, nil
		}
		return &ResolvedCoordinate{Type: t, Field: field, Argument: arg}, nil

	case *language.DirectiveCoordinate:
		d := s.Directive(nameOf(n.Name))
		if d == nil {
			return nil, nil
		}
		return &ResolvedCoordinate{Directive: d}, nil

	case *language.DirectiveArgumentCoordinate:
		name := nameOf(n.Name)
		d := s.Directive(name)
		if d == nil {
			return nil, fmt.Errorf("expected %q to be defined as a directive in the schema", name)
		}
		arg := d.Arg(nameOf(n.ArgumentName))
		if arg == nil {
			return nil, nil
		}
		return &ResolvedCoordinate{Directive: d, Argument: arg}, nil

	default:
		return nil, nil
	}
}

// namedType finds the type a coordinate leads with, which has to be there for
// what follows the dot to mean anything.
func namedType(s *schema.Schema, name string) (schema.NamedType, error) {
	t := s.Type(name)
	if t == nil {
		return nil, fmt.Errorf("expected %q to be defined as a type in the schema", name)
	}
	return t, nil
}

// resolveMember finds a member of a type, which is a field, an enum member or
// an input object field depending on what kind of type it is.
func resolveMember(s *schema.Schema, t schema.NamedType, name string) (*ResolvedCoordinate, error) {
	switch typed := t.(type) {
	// The schema is asked rather than the type, so that the fields every type
	// has — __typename, and __schema and __type on the query root — can be
	// pointed at like any other. Objects and interfaces are written out
	// separately so that the compiler, rather than a type assertion, is what
	// says each of them has fields.
	case *schema.ObjectType:
		if field := s.Field(typed, name); field != nil {
			return &ResolvedCoordinate{Type: t, Field: field}, nil
		}
	case *schema.InterfaceType:
		if field := s.Field(typed, name); field != nil {
			return &ResolvedCoordinate{Type: t, Field: field}, nil
		}
	case *schema.InputObjectType:
		if field := typed.Field(name); field != nil {
			return &ResolvedCoordinate{Type: t, InputField: field}, nil
		}
	case *schema.EnumType:
		if member := typed.Value(name); member != nil {
			return &ResolvedCoordinate{Type: t, EnumValue: member}, nil
		}
	default:
		// A scalar and a union have no members, so nothing can follow the dot.
		return nil, fmt.Errorf(
			"expected %q to be an Enum, Input Object, Object or Interface type", t.Name())
	}
	return nil, nil
}

// fieldOf reads a field of whichever kinds of type have them.
func fieldOf(s *schema.Schema, t schema.NamedType, name string) (*schema.Field, error) {
	var composite schema.CompositeType
	switch typed := t.(type) {
	case *schema.ObjectType:
		composite = typed
	case *schema.InterfaceType:
		composite = typed
	default:
		return nil, fmt.Errorf("expected %q to be an object type or interface type", t.Name())
	}
	field := s.Field(composite, name)
	if field == nil {
		return nil, fmt.Errorf(
			"expected %q to exist as a field of type %q in the schema", name, t.Name())
	}
	return field, nil
}

// String renders a coordinate for what was resolved, which is the form it
// would have been written in.
func (r *ResolvedCoordinate) String() string {
	switch {
	case r == nil:
		return ""
	case r.Directive != nil && r.Argument != nil:
		return fmt.Sprintf("@%s(%s:)", r.Directive.Name(), r.Argument.Name())
	case r.Directive != nil:
		return "@" + r.Directive.Name()
	case r.Field != nil && r.Argument != nil:
		return fmt.Sprintf("%s.%s(%s:)", r.Type.Name(), r.Field.Name(), r.Argument.Name())
	case r.Field != nil:
		return r.Type.Name() + "." + r.Field.Name()
	case r.InputField != nil:
		return r.Type.Name() + "." + r.InputField.Name()
	case r.EnumValue != nil:
		return r.Type.Name() + "." + r.EnumValue.Name()
	case r.Type != nil:
		return r.Type.Name()
	default:
		return ""
	}
}
