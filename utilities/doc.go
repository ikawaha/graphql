// Package utilities works on schemas and documents that already exist.
//
// Nothing here is needed to answer a request. What is here is everything a
// server does around that: reading a schema someone wrote, printing one back
// out, changing one wholesale, asking a server what it can answer, and telling
// whether tomorrow's schema will still answer yesterday's queries.
//
// # Reading and writing a schema
//
// [BuildSchema] reads SDL; [BuildASTSchema] reads a document already parsed.
// Both check the document against the rules a schema definition must follow
// before building anything from it — [validation.SpecifiedSDLRules], which is
// what reports a type defined twice or a directive nothing declares — and
// [AssumeValidSDL] skips that check for a document already known to be sound.
// [PrintSchema] writes one back out, and [PrintType] a single type. What comes
// out is the schema as it stands rather than the text that went in: the
// definitions are in the order the schema holds them, an extension has been
// merged into what it extends, and the directives every schema has are left
// out. [PrintIntrospectionSchema] prints the other half — the types
// introspection itself is made of.
//
// # Changing one
//
// A schema's types point at one another, so changing one means rebuilding all
// of them. That is done once here rather than by every caller who wants a
// variation: [ExtendSchema] applies a document of definitions and extensions,
// [LexicographicSortSchema] puts everything in name order, and [MapSchema]
// takes a [SchemaMapper] saying what each part becomes — which is how a schema
// is filtered down to what a public API should show, or how coercers are put
// back onto one that came from a server's own answer. All three return a new
// schema whose types are new objects, so a caller holding a type from the
// original must look it up again.
//
// # Asking a server what it can answer
//
// [IntrospectionQuery] is the document to send, [IntrospectionQueryResult] the
// shape of the answer, and [BuildClientSchema] turns that answer back into a
// schema. The result is a schema that can be validated and printed against but
// not executed: an answer says what the fields are, not how their values are
// found. It may also be partial, since a server may refuse to describe parts
// of itself.
//
// # Whether a change is safe
//
// [FindBreakingChanges] and [FindDangerousChanges] compare two schemas and say
// what a client would notice. [FindSchemaChanges] returns both, along with the
// changes that are safe.
//
// # Reading a document
//
// [FindOperation] picks one operation out of a document, [SeparateOperations]
// splits a document into one per operation, and [ConcatDocuments] joins
// several into one. [TypeInfo] follows a walk of a document and says which
// type, field and argument each node is under, which is what a rule of one's
// own needs; [VisitWithTypeInfo] keeps it in step with a visitor.
// [ResolveSchemaCoordinate] answers what a coordinate such as `User.name`
// refers to.
package utilities
