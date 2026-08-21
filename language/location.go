package language

import "unicode/utf8"

// GetLocation converts a byte offset within the source body into a one-indexed
// line and column.
//
// Lines are separated by "\r\n", "\n" or "\r". Column counts Unicode code
// points from the start of the line, so a multi-byte character advances the
// column by one rather than by its byte length. Offsets past the end of the
// body are clamped to the end.
func GetLocation(source *Source, position int) SourceLocation {
	body := source.Body
	if position > len(body) {
		position = len(body)
	}
	if position < 0 {
		position = 0
	}

	line := 1
	lineStart := 0
	for i := 0; i < position; i++ {
		switch body[i] {
		case '\r':
			// Treat "\r\n" as a single line terminator.
			if i+1 < len(body) && body[i+1] == '\n' {
				i++
			}
			line++
			lineStart = i + 1
		case '\n':
			line++
			lineStart = i + 1
		}
	}
	return SourceLocation{
		Line:   line,
		Column: utf8.RuneCountInString(body[lineStart:position]) + 1,
	}
}
