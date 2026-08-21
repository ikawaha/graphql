package utilities

import (
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// BuildASTSchema builds a schema from a parsed SDL document.
//
// Types refer to one another freely and in any order, so nothing is resolved
// while the definitions are read: each type is created with its fields
// deferred, and a reference is looked up by name only once every type exists.
// That is the same mechanism a schema written in Go uses for its own recursive
// types.
//
// The root types come from the schema definition if the document has one, and
// otherwise from the conventional names Query, Mutation and Subscription.
//
// A document may extend what it defines, so definitions and extensions can be
// read together. Extending something the document does not define is an error;
// use [ExtendSchema] to apply a document to a schema that already exists.
//
// The document is checked against the rules a schema definition must follow
// before anything is built from it, which is what reports a type defined
// twice or a directive nothing declares. [AssumeValidSDL] skips that check.
func BuildASTSchema(doc *language.Document, opts ...BuildOption) (*schema.Schema, error) {
	return ExtendSchema(nil, doc, opts...)
}

// BuildSchema parses SDL and builds a schema from it.
//
// Options for the parser are passed with [WithParseOptions]; graphql-js takes
// both kinds in one bag here and so does this.
func BuildSchema(source string, opts ...BuildOption) (*schema.Schema, error) {
	doc, err := language.ParseString(source, newBuildConfig(opts).parse...)
	if err != nil {
		return nil, err
	}
	return BuildASTSchema(doc, opts...)
}

// typeDefinitionName reads the name a definition declares.
func typeDefinitionName(node language.TypeDefinition) string {
	switch def := node.(type) {
	case *language.ScalarTypeDefinition:
		return nameOf(def.Name)
	case *language.ObjectTypeDefinition:
		return nameOf(def.Name)
	case *language.InterfaceTypeDefinition:
		return nameOf(def.Name)
	case *language.UnionTypeDefinition:
		return nameOf(def.Name)
	case *language.EnumTypeDefinition:
		return nameOf(def.Name)
	case *language.InputObjectTypeDefinition:
		return nameOf(def.Name)
	default:
		return ""
	}
}

func nameOf(n *language.Name) string {
	if n == nil {
		return ""
	}
	return n.Value
}

// descriptionOf reads a description, which is absent more often than not.
func descriptionOf(n *language.StringValue) value.Maybe[string] {
	if n == nil {
		return value.Nothing[string]()
	}
	return value.Just(n.Value)
}

// defaultOf keeps a default value as the literal it was written as, so that
// printing the schema gives back the same text.
func defaultOf(node language.Value) value.Maybe[schema.DefaultInput] {
	if node == nil {
		return schema.NoDefault()
	}
	return schema.DefaultLiteral(node)
}

// hasDirective reports whether a directive of the given name was applied.
func hasDirective(directives []*language.Directive, name string) bool {
	return findDirective(directives, name) != nil
}

// findDirective returns the first application of a directive, or nil.
func findDirective(directives []*language.Directive, name string) *language.Directive {
	for _, d := range directives {
		if d != nil && d.Name != nil && d.Name.Value == name {
			return d
		}
	}
	return nil
}

// directiveArgumentString reads a string argument of a directive.
func directiveArgumentString(directives []*language.Directive, directive, argument string) string {
	d := findDirective(directives, directive)
	if d == nil {
		return ""
	}
	for _, arg := range d.Arguments {
		if arg == nil || arg.Name == nil || arg.Name.Value != argument {
			continue
		}
		if s, isString := arg.Value.(*language.StringValue); isString {
			return s.Value
		}
	}
	return ""
}

// deprecationReason reads the reason from an applied @deprecated, falling back
// to the directive's own default when the argument was left out.
//
// The reason is a String!, so writing it as null, or as anything a string will
// not take, is not a smaller answer but a wrong one; refused is reported so
// that the caller can say so rather than guessing what was meant. graphql-js
// refuses the same schema, from the same coercion.
func deprecationReason(directives []*language.Directive) (reason value.Maybe[string], refused language.Value) {
	d := findDirective(directives, "deprecated")
	if d == nil {
		return schema.NotDeprecated(), nil
	}
	for _, arg := range d.Arguments {
		if arg == nil || arg.Name == nil || arg.Name.Value != "reason" {
			continue
		}
		written, isString := arg.Value.(*language.StringValue)
		if !isString {
			return schema.NotDeprecated(), arg.Value
		}
		return schema.DeprecatedFor(written.Value), nil
	}
	return schema.DeprecatedFor(schema.DefaultDeprecationReason), nil
}
