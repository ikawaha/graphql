package typeinfo

import (
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// TypeFromAST turns a type reference written in a document into the type it
// names, looking the name up in a schema.
//
// A false ok means the schema has no type of that name. That is a mistake in
// the document rather than in this call, so it is reported by whoever asked
// rather than raised here.
func TypeFromAST(s *schema.Schema, node language.Type) (schema.Type, bool) {
	return TypeFromASTWith(node, func(name string) schema.NamedType {
		if s == nil {
			return nil
		}
		return s.Type(name)
	})
}

// TypeFromASTWith resolves a type reference against any way of finding a named
// type, which is what lets a schema being built resolve references to types
// that do not exist yet.
func TypeFromASTWith(node language.Type, lookup func(string) schema.NamedType) (schema.Type, bool) {
	switch n := node.(type) {
	case *language.NamedType:
		if n.Name == nil {
			return nil, false
		}
		found := lookup(n.Name.Value)
		if found == nil {
			return nil, false
		}
		return found, true

	case *language.ListType:
		inner, ok := TypeFromASTWith(n.Type, lookup)
		if !ok {
			return nil, false
		}
		return schema.NewList(inner), true

	case *language.NonNullType:
		inner, ok := TypeFromASTWith(n.Type, lookup)
		if !ok {
			return nil, false
		}
		if schema.IsNonNullType(inner) {
			// The grammar does not allow this, so the parser never produces
			// it; a reference assembled by hand still might.
			return nil, false
		}
		return schema.NewNonNull(inner), true

	default:
		return nil, false
	}
}
