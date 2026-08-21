package language

import "fmt"

// SyntaxError reports malformed GraphQL source.
//
// The lexer and the parser return this type rather than the richer error the
// gqlerror package defines. graphql-js has language and error import each
// other, which JavaScript permits and Go does not, so the dependency runs one
// way here: language stays at the bottom and returns a syntax error carrying
// everything needed to describe the failure, and gqlerror, which sits above,
// converts it into a GraphQL error.
type SyntaxError struct {
	// Source is the document the error was found in.
	Source *Source
	// Position is the byte offset in the source body at which the error was
	// found.
	Position int
	// Location is the one-indexed line and column of Position in the
	// coordinates of the file the source came from, that is, with the
	// source's LocationOffset already applied.
	//
	// [GetLocation] and [PrintSourceLocation] work in the coordinates of the
	// source body instead, without the offset, so do not pass this to them.
	// Use [SyntaxError.PrintLocation] to render the offending line.
	Location SourceLocation
	// Description explains the problem without the "Syntax Error: " prefix.
	Description string
}

// Error implements the error interface.
func (e *SyntaxError) Error() string {
	return "Syntax Error: " + e.Description
}

// PrintLocation renders the line the error was found on, with a caret under
// the position:
//
//	GraphQL request:1:8
//	1 | query {
//	  |        ^
func (e *SyntaxError) PrintLocation() string {
	if e == nil || e.Source == nil {
		return ""
	}
	return PrintSourceLocation(e.Source, GetLocation(e.Source, e.Position))
}

// newSyntaxError builds a SyntaxError for a byte offset in a source.
func newSyntaxError(source *Source, position int, format string, args ...any) *SyntaxError {
	description := format
	if len(args) > 0 {
		description = fmt.Sprintf(format, args...)
	}
	return &SyntaxError{
		Source:      source,
		Position:    position,
		Location:    source.FileLocation(GetLocation(source, position)),
		Description: description,
	}
}
