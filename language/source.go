package language

import "fmt"

// SourceLocation is a one-indexed line and column in a [Source].
//
// Column counts Unicode code points, not bytes and not UTF-16 code units. This
// differs from graphql-js, which counts UTF-16 code units; the two agree
// whenever a line contains only ASCII.
type SourceLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// String renders the location as "line:column".
func (l SourceLocation) String() string {
	return fmt.Sprintf("%d:%d", l.Line, l.Column)
}

// Source is a GraphQL source document together with the metadata needed to
// report useful locations in errors.
//
// Name and LocationOffset matter for clients that embed GraphQL documents in
// other files. If a document starts at line 40 of Foo.graphql, setting Name to
// "Foo.graphql" and LocationOffset to line 40, column 1 makes errors point at
// the right place in the original file.
//
// # Encoding
//
// Body is not required to be valid UTF-8. Go strings are byte strings, which
// lets a Source hold the malformed input the GraphQL specification requires
// implementations to reject, such as an unpaired surrogate encoded as the
// three bytes ED BA AD. The lexer scans Body byte by byte and reports such
// input as a syntax error.
type Source struct {
	// Body is the GraphQL source text.
	Body string
	// Name identifies the source in diagnostics, typically a file path or a
	// request name.
	Name string
	// LocationOffset is the one-indexed line and column at which this source
	// begins within its enclosing file.
	LocationOffset SourceLocation
}

// SourceOption configures a [Source] built by [NewSource].
type SourceOption func(*Source)

// SourceName sets the name used to identify the source in diagnostics.
func SourceName(name string) SourceOption {
	return func(s *Source) { s.Name = name }
}

// SourceLocationOffset sets the one-indexed line and column at which the
// source begins within its enclosing file. Both values must be positive.
func SourceLocationOffset(line, column int) SourceOption {
	return func(s *Source) { s.LocationOffset = SourceLocation{Line: line, Column: column} }
}

// NewSource returns a Source for the given body. By default the name is
// "GraphQL request" and the source is taken to begin at line 1, column 1.
//
// It panics if an option sets a non-positive line or column, since both are
// one-indexed.
func NewSource(body string, opts ...SourceOption) *Source {
	s := &Source{
		Body:           body,
		Name:           "GraphQL request",
		LocationOffset: SourceLocation{Line: 1, Column: 1},
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.LocationOffset.Line <= 0 {
		panic("graphql/language: line in LocationOffset is 1-indexed and must be positive")
	}
	if s.LocationOffset.Column <= 0 {
		panic("graphql/language: column in LocationOffset is 1-indexed and must be positive")
	}
	return s
}

// FileLocation converts a location within this source into the coordinates of
// the file the source came from, by applying LocationOffset.
//
// Only the first line is shifted horizontally, because every later line begins
// at column 1 wherever the document was embedded.
func (s *Source) FileLocation(loc SourceLocation) SourceLocation {
	out := SourceLocation{
		Line:   loc.Line + s.LocationOffset.Line - 1,
		Column: loc.Column,
	}
	if loc.Line == 1 {
		out.Column += s.LocationOffset.Column - 1
	}
	return out
}
