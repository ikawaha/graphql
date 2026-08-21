package language

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// minifiedLineLength is the width past which a line is treated as minified and
// shown in chunks rather than whole.
const minifiedLineLength = 120

// minifiedChunkLength is the width of each chunk a minified line is shown in.
const minifiedChunkLength = 80

// PrintLocation renders the part of a document a node came from, with the
// surrounding lines and a caret under the position.
//
//	GraphQL request:1:14
//	1 | type Query { hello: String }
//	  |              ^
func PrintLocation(location *Location) string {
	if location == nil || location.Source == nil {
		return ""
	}
	return PrintSourceLocation(location.Source, GetLocation(location.Source, location.Start))
}

// PrintSourceLocation renders a line and column of a source in the same form
// as [PrintLocation].
func PrintSourceLocation(source *Source, at SourceLocation) string {
	// A source embedded in a larger file starts partway along its first line,
	// so that line is padded to sit where it really does.
	firstLineIndent := source.LocationOffset.Column - 1
	body := strings.Repeat(" ", firstLineIndent) + source.Body

	lineNum := at.Line + source.LocationOffset.Line - 1
	columnNum := at.Column
	if at.Line == 1 {
		columnNum += firstLineIndent
	}

	lines := splitLines(body)
	lineIndex := at.Line - 1
	if lineIndex < 0 || lineIndex >= len(lines) {
		return fmt.Sprintf("%s:%d:%d\n", source.Name, lineNum, columnNum)
	}
	locationLine := lines[lineIndex]

	header := fmt.Sprintf("%s:%d:%d\n", source.Name, lineNum, columnNum)
	if utf8.RuneCountInString(locationLine) > minifiedLineLength {
		return header + printMinifiedLine(lineNum, columnNum, locationLine)
	}

	before, hasBefore := lineAt(lines, lineIndex-1)
	after, hasAfter := lineAt(lines, lineIndex+1)

	var out prefixedLines
	out.add(lineLabel(lineNum-1), before, hasBefore)
	out.addPresent(lineLabel(lineNum), locationLine)
	out.addPresent("|", caret(columnNum))
	out.add(lineLabel(lineNum+1), after, hasAfter)
	return header + out.String()
}

// printMinifiedLine renders one very long line as a run of chunks, showing the
// chunks up to the one holding the position and the one just after it.
func printMinifiedLine(lineNum, columnNum int, line string) string {
	chunks := chunkRunes(line, minifiedChunkLength)
	chunkIndex := columnNum / minifiedChunkLength
	chunkColumn := columnNum % minifiedChunkLength

	var out prefixedLines
	out.addPresent(lineLabel(lineNum), chunks[0])
	for i := 1; i <= chunkIndex && i < len(chunks); i++ {
		out.addPresent("|", chunks[i])
	}
	out.addPresent("|", caret(chunkColumn))
	if next := chunkIndex + 1; next < len(chunks) {
		out.addPresent("|", chunks[next])
	}
	return out.String()
}

// chunkRunes splits a string into pieces of at most n code points.
func chunkRunes(s string, n int) []string {
	runes := []rune(s)
	var chunks []string
	for i := 0; i < len(runes); i += n {
		chunks = append(chunks, string(runes[i:min(i+n, len(runes))]))
	}
	if len(chunks) == 0 {
		chunks = append(chunks, "")
	}
	return chunks
}

// caret returns a run of spaces with a caret at the given one-indexed column.
func caret(column int) string {
	if column < 1 {
		return "^"
	}
	return strings.Repeat(" ", column-1) + "^"
}

// lineLabel returns the prefix that introduces a numbered line.
func lineLabel(n int) string { return strconv.Itoa(n) + " |" }

// lineAt returns the line at an index, and whether there is one there. The
// lines before and after the position are shown only when they exist, which is
// different from their being empty.
func lineAt(lines []string, i int) (string, bool) {
	if i < 0 || i >= len(lines) {
		return "", false
	}
	return lines[i], true
}

// prefixedLines collects the lines of an excerpt so that their prefixes can be
// aligned once they are all known.
type prefixedLines struct {
	prefixes []string
	lines    []string
}

// add appends a line that may not exist.
func (p *prefixedLines) add(prefix string, line string, present bool) {
	if present {
		p.addPresent(prefix, line)
	}
}

// addPresent appends a line that does exist, empty or not.
func (p *prefixedLines) addPresent(prefix, line string) {
	p.prefixes = append(p.prefixes, prefix)
	p.lines = append(p.lines, line)
}

// String renders the collected lines with their prefixes right-aligned.
func (p *prefixedLines) String() string {
	width := 0
	for _, prefix := range p.prefixes {
		width = max(width, utf8.RuneCountInString(prefix))
	}
	var b strings.Builder
	for i, prefix := range p.prefixes {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(strings.Repeat(" ", width-utf8.RuneCountInString(prefix)))
		b.WriteString(prefix)
		if p.lines[i] != "" {
			b.WriteByte(' ')
			b.WriteString(p.lines[i])
		}
	}
	return b.String()
}
