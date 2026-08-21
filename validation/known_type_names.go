package validation

import (
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// KnownTypeNamesRule reports a reference to a type that neither the schema nor
// the document defines.
//
// It applies to executable documents and to SDL alike, since both name types:
// a fragment's type condition and a field's declared type are the same kind of
// mistake to make.
func KnownTypeNamesRule(ctx *Context) language.Visitor {
	known := map[string]bool{}
	for _, t := range ctx.Schema().Types() {
		if t != nil {
			known[t.Name()] = true
		}
	}
	for _, def := range ctx.Document().Definitions {
		if typeDef, isType := def.(language.TypeDefinition); isType {
			if name := typeDefinitionName(typeDef); name != "" {
				known[name] = true
			}
		}
	}

	names := make([]string, 0, len(known))
	for name := range known {
		names = append(names, name)
	}

	// A type definition may name a built-in scalar the schema does not list,
	// because SDL leaves the built-ins out; a query may not, since by then the
	// schema has them.
	// The introspection types are there too, and SDL leaves them out for the
	// same reason it leaves out the built-in scalars.
	standard := make([]string, 0, len(schema.SpecifiedScalars)+len(schema.IntrospectionTypes))
	isStandard := map[string]bool{}
	for _, t := range schema.SpecifiedScalars {
		standard = append(standard, t.Name())
		isStandard[t.Name()] = true
	}
	for _, t := range schema.IntrospectionTypes {
		standard = append(standard, t.Name())
		isStandard[t.Name()] = true
	}

	// Whether a name is being read in SDL depends on which definition encloses
	// it, so the walk keeps track of what it is inside.
	inSDL := 0

	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			switch n := node.(type) {
			case *language.SchemaDefinition, *language.SchemaExtension, *language.DirectiveDefinition:
				inSDL++
			case language.TypeDefinition:
				inSDL++
			case language.TypeExtension:
				inSDL++
			case *language.NamedType:
				if n.Name == nil || known[n.Name.Value] {
					return language.VisitContinue
				}
				name := n.Name.Value
				if inSDL > 0 && isStandard[name] {
					return language.VisitContinue
				}
				options := names
				if inSDL > 0 {
					options = append(append([]string{}, names...), standard...)
				}
				ctx.Reportf([]language.Node{n}, "Unknown type %s.%s",
					quote(name), ctx.DidYouMean("", schema.SuggestionList(name, options)))
			}
			return language.VisitContinue
		},
		Leave: func(node language.Node, _ language.VisitContext) language.VisitAction {
			switch node.(type) {
			case *language.SchemaDefinition, *language.SchemaExtension, *language.DirectiveDefinition:
				inSDL--
			case language.TypeDefinition:
				inSDL--
			case language.TypeExtension:
				inSDL--
			}
			return language.VisitContinue
		},
	}
}
