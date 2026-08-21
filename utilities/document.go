package utilities

import "github.com/ikawaha/graphql/language"

// ConcatDocuments joins several parsed documents into one.
//
// A schema written across several files is one schema; parsing each file on
// its own and joining the results here gives the single document that
// [BuildSchema] and validation expect.
//
// Nothing is copied: the result points at the same definitions, so it must be
// treated as read-only. It carries no location, since its definitions come
// from more than one source; each definition keeps its own.
//
// This is graphql-js's concatAST.
func ConcatDocuments(docs ...*language.Document) *language.Document {
	out := &language.Document{}
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		out.Definitions = append(out.Definitions, doc.Definitions...)
		out.TokenCount += doc.TokenCount
	}
	return out
}

// FindOperation returns the operation a request asked for, or nil if the
// document does not settle on one.
//
// With a name, it returns the operation of that name. Without one, it returns
// the document's only operation, and nil if the document holds none or more
// than one — a request that names no operation is answerable only when there
// is nothing to choose between.
//
// Executing a document does this itself, and says what was wrong rather than
// answering nil. This is for a caller that wants to know before executing:
// which operation type is about to run, whether a persisted document still
// holds the operation a client asks for.
//
// This is graphql-js's getOperationAST.
func FindOperation(doc *language.Document, name string) *language.OperationDefinition {
	if doc == nil {
		return nil
	}
	var only *language.OperationDefinition
	for _, def := range doc.Definitions {
		operation, isOperation := def.(*language.OperationDefinition)
		if !isOperation {
			continue
		}
		if name != "" {
			if operation.Name != nil && operation.Name.Value == name {
				return operation
			}
			continue
		}
		// Without a name, a second operation makes the choice ambiguous.
		if only != nil {
			return nil
		}
		only = operation
	}
	if name != "" {
		return nil
	}
	return only
}
