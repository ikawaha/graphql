package validation

import (
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// UniqueFieldDefinitionNamesRule reports a field declared twice on one type.
//
// A field is selected by name, so two of a name leave no way to say which is
// being asked for.
func UniqueFieldDefinitionNamesRule(ctx *Context) language.Visitor {
	// Fields are counted per type, and a definition and its extensions add to
	// the same tally.
	known := map[string]map[string]*language.Name{}

	// hasField reports whether the schema already declares the field.
	hasField := func(typeName, fieldName string) bool {
		switch t := ctx.Schema().Type(typeName).(type) {
		case *schema.ObjectType:
			return t.Field(fieldName) != nil
		case *schema.InterfaceType:
			return t.Field(fieldName) != nil
		case *schema.InputObjectType:
			return t.Field(fieldName) != nil
		default:
			return false
		}
	}

	check := func(typeName string, names []*language.Name) {
		fields, started := known[typeName]
		if !started {
			fields = map[string]*language.Name{}
			known[typeName] = fields
		}
		for _, field := range names {
			if field == nil {
				continue
			}
			switch {
			case hasField(typeName, field.Value):
				ctx.Reportf([]language.Node{field},
					"Field %s already exists in the schema. It cannot also be defined in this type extension.",
					quote(typeName+"."+field.Value))
			case fields[field.Value] != nil:
				ctx.Reportf([]language.Node{fields[field.Value], field},
					"Field %s can only be defined once.", quote(typeName+"."+field.Value))
			default:
				fields[field.Value] = field
			}
		}
	}

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			switch n := node.(type) {
			case *language.ObjectTypeDefinition:
				check(nameOf(n.Name), fieldDefinitionNames(n.Fields))
			case *language.ObjectTypeExtension:
				check(nameOf(n.Name), fieldDefinitionNames(n.Fields))
			case *language.InterfaceTypeDefinition:
				check(nameOf(n.Name), fieldDefinitionNames(n.Fields))
			case *language.InterfaceTypeExtension:
				check(nameOf(n.Name), fieldDefinitionNames(n.Fields))
			case *language.InputObjectTypeDefinition:
				check(nameOf(n.Name), inputValueNames(n.Fields))
			case *language.InputObjectTypeExtension:
				check(nameOf(n.Name), inputValueNames(n.Fields))
			default:
				return language.VisitContinue
			}
			return language.VisitSkip
		},
	}
}

// fieldDefinitionNames reads the names a list of field definitions declares.
func fieldDefinitionNames(fields []*language.FieldDefinition) []*language.Name {
	out := make([]*language.Name, 0, len(fields))
	for _, f := range fields {
		if f != nil && f.Name != nil {
			out = append(out, f.Name)
		}
	}
	return out
}

// inputValueNames reads the names a list of input value definitions declares.
func inputValueNames(values []*language.InputValueDefinition) []*language.Name {
	out := make([]*language.Name, 0, len(values))
	for _, v := range values {
		if v != nil && v.Name != nil {
			out = append(out, v.Name)
		}
	}
	return out
}
