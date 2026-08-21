// Package language reads and writes GraphQL documents.
//
// It holds the lexer, the parser, the abstract syntax tree those produce, a
// walker over that tree, and a printer that turns it back into source. Nothing
// here knows about schemas or execution, so it can be used on its own by
// tooling that only needs to work with GraphQL text.
//
// # Reading a document
//
//	doc, err := language.ParseString("{ hero { name } }")
//	if err != nil {
//		return err
//	}
//	fmt.Println(language.Print(doc))
//
// [Parse] reads a whole document. [ParseValue], [ParseType] and
// [ParseSchemaCoordinate] read the smaller fragments of the grammar that
// tooling often needs on their own.
//
// # Errors
//
// Reading a malformed document returns a [SyntaxError], which carries the
// source, the byte offset and the line and column of the problem. Use
// [PrintLocation] to show the offending line with a caret under it.
//
// The reference implementation raises the richer error type that its error
// module defines, but that module needs this one, and Go does not allow two
// packages to import each other. The dependency therefore runs one way: this
// package returns a syntax error describing what went wrong, and the gqlerror
// package above it converts that into a GraphQL error.
//
// # Positions
//
// Offsets are byte offsets into the source, which is the natural unit for a Go
// string. Lines and columns are one-indexed, and a column counts Unicode code
// points rather than bytes. graphql-js counts UTF-16 code units instead; the
// two agree for ASCII, which is what a document is outside its string literals
// and comments.
//
// A source body is not required to be valid UTF-8. A Go string is a byte
// string, which lets this package hold and then reject the malformed input the
// specification requires implementations to refuse, such as an unpaired
// surrogate.
//
// # The tree
//
// Every kind of node is its own Go type implementing [Node], so a type switch
// over a node covers the same ground as a switch on its kind, and each type
// declares exactly the fields that kind has. An optional child is a nil
// pointer and an optional list is a nil slice, so an absent argument list and
// an empty one stay distinguishable.
//
// [Visit] walks a tree, calling a visitor as it enters and leaves each node.
// The walk only reads: utilities that reshape a document do so with plain
// recursion rather than by rewriting nodes during a walk.
package language
