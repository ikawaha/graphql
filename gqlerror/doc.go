// Package gqlerror describes what went wrong in a GraphQL request.
//
// An [Error] carries a message together with where it came from: the parts of
// the document to blame, the response path of the field that failed, and any
// extra fields a server wants to report. [Error.Formatted] reduces it to the
// members that belong in a response.
//
// # Building one
//
//	err := gqlerror.New("Cannot query field \"name\".",
//		gqlerror.WithNodes(fieldNode),
//		gqlerror.WithPath("viewer", "name"))
//
// Locations are worked out from whichever of the positions or the blamed nodes
// the caller supplied.
//
// # Errors from elsewhere
//
// [Ensure] turns any error into a GraphQL error, and [Located] adds the
// document position and response path to an error a resolver returned.
// [FromSyntaxError] converts a parse failure, which the language package
// reports with its own type because it sits below this package and Go does not
// allow two packages to import each other.
package gqlerror
