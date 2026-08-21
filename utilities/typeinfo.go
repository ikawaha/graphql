package utilities

import (
	"github.com/ikawaha/graphql/internal/typeinfo"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// TypeInfo keeps track of where in a schema a walk of a document currently is.
//
// A document on its own says almost nothing about types: a field is a name,
// and which field it names depends on the type enclosing it. TypeInfo follows
// a walk and answers that question at every step, which is what lets a
// validation rule be written in terms of types rather than in terms of names.
//
// The implementation lives in an internal package because validation needs it
// and the schema builder needs validation; this is where it is meant to be
// read from.
type TypeInfo = typeinfo.TypeInfo

// FragmentSignature is what a fragment declares it takes, which is how a
// variable used inside one is understood.
type FragmentSignature = typeinfo.FragmentSignature

// NewTypeInfo returns a TypeInfo that reads a walk against a schema.
func NewTypeInfo(s *schema.Schema) *TypeInfo { return typeinfo.NewTypeInfo(s) }

// NewTypeInfoForDocument returns a TypeInfo that also knows the fragments a
// document defines, so a spread can be followed.
func NewTypeInfoForDocument(s *schema.Schema, doc *language.Document) *TypeInfo {
	return typeinfo.NewTypeInfoForDocument(s, doc)
}

// VisitWithTypeInfo wraps a visitor so that a TypeInfo is kept in step with
// the walk, entering and leaving each node around the visitor's own calls.
func VisitWithTypeInfo(info *TypeInfo, visitor language.Visitor) language.Visitor {
	return typeinfo.VisitWithTypeInfo(info, visitor)
}

// TypeFromAST turns a type reference written in a document into the type it
// names, looking the name up in a schema.
//
// A false ok means the schema has no type of that name. That is a mistake in
// the document rather than in this call, so it is reported by whoever asked
// rather than raised here.
func TypeFromAST(s *schema.Schema, node language.Type) (schema.Type, bool) {
	return typeinfo.TypeFromAST(s, node)
}
