// Package validation checks that a document is one the schema can answer.
//
// A GraphQL request arrives as text. Parsing settles that it is a document;
// validation settles that it is a document this schema could execute. The
// division matters because execution is written to assume its input has passed
// here: it does not check that a field exists, that an argument is of the
// right type, or that a fragment can apply, because a rule below has already
// said so. Running an unvalidated document is therefore not merely risky but
// outside what execution promises to cope with.
//
// [Validate] checks an executable document, and [ValidateSDL] a document of
// type definitions. Both take the rules to apply, defaulting to the ones the
// specification requires: [SpecifiedRules] and [SpecifiedSDLRules]. A server
// that wants a check of its own writes a [Rule] and passes it alongside them.
//
// A server rarely calls [ValidateSDL] itself: the schema builder runs it over
// every document it is given, so a schema read from SDL has been through it
// already.
//
// Every rule sees the document in one walk rather than one walk each, and the
// questions rules ask about a document are answered once and kept, so the cost
// is of the document's size rather than of its size times the number of rules.
//
// Two further rules are provided but left out of the specified set, because
// what they report is allowed by the specification and only some servers want
// it refused: [NoDeprecatedCustomRule] and [NoSchemaIntrospectionCustomRule].
package validation
