package gqlerror

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ikawaha/graphql/language"
)

// Error is a GraphQL error: something that went wrong, together with enough
// context to point at what caused it.
//
// It implements the error interface, and Unwrap exposes the underlying cause
// so that errors.Is and errors.As reach through it.
//
// # What ends up in a response
//
// Only Message, Locations, Path and Extensions are serialised, because those
// are the members the specification gives an error entry. The remaining fields
// are there for tooling on the server side and never reach the client.
type Error struct {
	// Message describes what went wrong.
	Message string

	// Locations point at the parts of the document responsible, in the
	// coordinates of the file the source came from.
	Locations []language.SourceLocation

	// Path is the response path of the field that failed, made of field names
	// and list indices. It is empty for an error raised before execution.
	Path []any

	// Extensions carries anything else a server wants to report.
	Extensions map[string]any

	// Nodes are the AST nodes blamed for the error.
	Nodes []language.Node

	// Source is the document the error was found in.
	Source *language.Source

	// Positions are byte offsets into Source, one per entry in Locations.
	Positions []int

	cause error
}

// Option configures an [Error].
type Option func(*Error)

// WithNodes blames the given AST nodes. The source and positions of the error
// are taken from them unless set explicitly.
func WithNodes(nodes ...language.Node) Option {
	return func(e *Error) { e.Nodes = append(e.Nodes, nodes...) }
}

// WithSource names the document the error was found in.
func WithSource(source *language.Source) Option {
	return func(e *Error) { e.Source = source }
}

// WithPositions gives byte offsets into the source, which are turned into
// locations.
func WithPositions(positions ...int) Option {
	return func(e *Error) { e.Positions = append(e.Positions, positions...) }
}

// WithPath sets the response path of the field that failed. Entries are field
// names and list indices, which is what [value.Path.AsSlice] produces.
func WithPath(path ...any) Option {
	return func(e *Error) { e.Path = path }
}

// WithCause records the error underneath this one, which Unwrap returns.
func WithCause(err error) Option {
	return func(e *Error) { e.cause = err }
}

// WithExtensions attaches extra fields to report alongside the error.
func WithExtensions(extensions map[string]any) Option {
	return func(e *Error) { e.Extensions = extensions }
}

// Newf builds an error whose message is worked out from a format, which most
// of them are: a message a response carries usually names the type or the
// field it is about.
//
// It takes no options, unlike [New]: a message and nothing else is the common
// case, and where there is more to say [New] is there.
func Newf(format string, args ...any) *Error {
	return New(fmt.Sprintf(format, args...))
}

// New builds an error and works out where it points.
//
// Locations come from the positions given, or failing that from the locations
// of the nodes blamed, so a caller only has to supply whichever it has.
func New(message string, opts ...Option) *Error {
	e := &Error{Message: message}
	for _, opt := range opts {
		opt(e)
	}
	e.resolveLocations()
	return e
}

// resolveLocations fills in the source, positions and locations from whichever
// of them the caller supplied.
//
// A pair of positions and a source given together are read against each other.
// Otherwise the nodes are the authority, and each is read against the source it
// came from, so an error blaming a schema and the document that extended it
// points into both.
func (e *Error) resolveLocations() {
	givenSource, givenPositions := e.Source, e.Positions

	var nodeLocations []*language.Location
	for _, node := range e.Nodes {
		if loc := node.Location(); loc != nil {
			nodeLocations = append(nodeLocations, loc)
		}
	}
	if e.Source == nil && len(nodeLocations) > 0 {
		e.Source = nodeLocations[0].Source
	}
	if len(givenPositions) == 0 {
		for _, loc := range nodeLocations {
			e.Positions = append(e.Positions, loc.Start)
		}
	}

	if len(givenPositions) > 0 && givenSource != nil {
		for _, pos := range givenPositions {
			e.Locations = append(e.Locations, givenSource.FileLocation(language.GetLocation(givenSource, pos)))
		}
		return
	}
	for _, loc := range nodeLocations {
		e.Locations = append(e.Locations, loc.Source.FileLocation(language.GetLocation(loc.Source, loc.Start)))
	}
}

// Error returns the message together with an excerpt of each place the error
// points at.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	var b strings.Builder
	b.WriteString(e.Message)
	for _, excerpt := range e.excerpts() {
		b.WriteString("\n\n")
		b.WriteString(excerpt)
	}
	return b.String()
}

// excerpts renders the source lines the error points at.
//
// They are rendered from byte offsets rather than from Locations, because
// printing applies the source's offset itself and Locations already have it
// applied.
func (e *Error) excerpts() []string {
	var out []string
	if len(e.Nodes) > 0 {
		for _, node := range e.Nodes {
			if loc := node.Location(); loc != nil {
				out = append(out, language.PrintLocation(loc))
			}
		}
		return out
	}
	if e.Source != nil {
		for _, pos := range e.Positions {
			out = append(out, language.PrintSourceLocation(e.Source, language.GetLocation(e.Source, pos)))
		}
	}
	return out
}

// Unwrap returns the error underneath this one, if there is one.
func (e *Error) Unwrap() error { return e.cause }

// FormattedError is the shape an error takes in a GraphQL response.
type FormattedError struct {
	Message    string                    `json:"message"`
	Locations  []language.SourceLocation `json:"locations,omitempty"`
	Path       []any                     `json:"path,omitempty"`
	Extensions map[string]any            `json:"extensions,omitempty"`
}

// Formatted reduces the error to the members that belong in a response.
func (e *Error) Formatted() FormattedError {
	return FormattedError{
		Message:    e.Message,
		Locations:  e.Locations,
		Path:       e.Path,
		Extensions: e.Extensions,
	}
}

// MarshalJSON writes the error in the form a response uses.
func (e *Error) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.Formatted())
}
