package language

import (
	"strings"
	"unicode/utf8"
)

// dedentBlockStringLines produces the value of a block string from its raw
// lines, in the manner of Python's docstring trimming or Ruby's strip_heredoc.
//
// It implements the BlockStringValue() static algorithm from the GraphQL
// specification: the common indentation shared by every line after the first
// is removed, then leading and trailing blank lines are dropped.
func dedentBlockStringLines(lines []string) []string {
	commonIndent := -1 // -1 stands in for "no common indent seen yet"
	firstNonEmptyLine := -1
	lastNonEmptyLine := -1

	for i, line := range lines {
		indent := leadingWhitespace(line)
		if indent == len(line) {
			continue // skip lines that are entirely whitespace
		}
		if firstNonEmptyLine < 0 {
			firstNonEmptyLine = i
		}
		lastNonEmptyLine = i
		// The first line is not indented in the source, so it never
		// contributes to the common indent.
		if i != 0 && (commonIndent < 0 || indent < commonIndent) {
			commonIndent = indent
		}
	}

	out := make([]string, 0, len(lines))
	for i, line := range lines {
		if i == 0 || commonIndent < 0 {
			out = append(out, line)
			continue
		}
		if commonIndent >= len(line) {
			out = append(out, "")
			continue
		}
		out = append(out, line[commonIndent:])
	}

	start := firstNonEmptyLine
	if start < 0 {
		start = 0
	}
	return out[start : lastNonEmptyLine+1]
}

// leadingWhitespace returns the number of leading tab or space bytes in s.
func leadingWhitespace(s string) int {
	i := 0
	for i < len(s) && isWhiteSpace(s[i]) {
		i++
	}
	return i
}

// splitLines splits s on the line terminators "\r\n", "\n" and "\r". A source
// with no terminator yields a single line, and unlike strings.Split on a
// single separator this never merges "\r\n" into two empty lines.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\r':
			lines = append(lines, s[start:i])
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			start = i + 1
		case '\n':
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	return append(lines, s[start:])
}

// IsPrintableAsBlockString reports whether a string can be written as a block
// string without changing its meaning.
//
// Printing a description reaches for the block form when it can, because a
// multi-line description reads far better that way than as one escaped line.
func IsPrintableAsBlockString(value string) bool { return isPrintableAsBlockString(value) }

// isPrintableAsBlockString reports whether value can be printed as a block
// string without changing its meaning.
//
// A value is not printable this way if it contains characters a block string
// cannot carry, if it would gain or lose leading or trailing blank lines, or
// if every line shares an indent that dedenting would strip.
func isPrintableAsBlockString(value string) bool {
	if value == "" {
		return true // the empty string is printable
	}

	isEmptyLine := true
	hasIndent := false
	hasCommonIndent := true
	seenNonEmptyLine := false

	for _, r := range value {
		switch r {
		case 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
			0x0b, 0x0c, 0x0e, 0x0f:
			return false // contains characters that cannot be printed
		case '\r':
			return false // \r or \r\n would be rewritten as \n
		case '\n':
			if isEmptyLine && !seenNonEmptyLine {
				return false // would gain a leading blank line
			}
			seenNonEmptyLine = true
			isEmptyLine = true
			hasIndent = false
		case '\t', ' ':
			hasIndent = hasIndent || isEmptyLine
		default:
			hasCommonIndent = hasCommonIndent && hasIndent
			isEmptyLine = false
		}
	}

	if isEmptyLine {
		return false // has trailing blank lines
	}
	if hasCommonIndent && seenNonEmptyLine {
		return false // every line is indented, which dedenting would strip
	}
	return true
}

// printBlockString renders value as a block string literal, including the
// surrounding triple quotes.
//
// Leading and trailing blank lines are added when they improve readability,
// and are required when the value would otherwise be misread: a value ending
// in a quote or backslash, or one whose every subsequent line begins with
// whitespace that dedenting would strip. Setting minimize suppresses the
// purely cosmetic blank lines but keeps the required ones.
//
// The readability heuristic measures length in Unicode code points, where
// graphql-js measures UTF-16 code units. This only affects where a long
// non-ASCII single line is broken across lines, never the value itself.
func printBlockString(value string, minimize bool) string {
	escaped := strings.ReplaceAll(value, `"""`, `\"""`)

	lines := splitLines(escaped)
	isSingleLine := len(lines) == 1

	// Dedenting would strip the indent of every line after the first, so a
	// leading blank line is needed to make the first line share their fate.
	forceLeadingNewLine := false
	if len(lines) > 1 {
		forceLeadingNewLine = true
		for _, line := range lines[1:] {
			if line != "" && !isWhiteSpace(line[0]) {
				forceLeadingNewLine = false
				break
			}
		}
	}

	// Trailing triple quotes merely look confusing; they do not force a
	// trailing blank line the way a lone quote or a backslash does.
	hasTrailingTripleQuotes := strings.HasSuffix(escaped, `\"""`)
	hasTrailingQuote := strings.HasSuffix(value, `"`) && !hasTrailingTripleQuotes
	hasTrailingSlash := strings.HasSuffix(value, `\`)
	forceTrailingNewline := hasTrailingQuote || hasTrailingSlash

	printAsMultipleLines := !minimize &&
		(!isSingleLine ||
			utf8.RuneCountInString(value) > 70 ||
			forceTrailingNewline ||
			forceLeadingNewLine ||
			hasTrailingTripleQuotes)

	var b strings.Builder
	b.WriteString(`"""`)

	// A leading blank line before a single line that starts with whitespace
	// would strip that whitespace, so it is skipped.
	skipLeadingNewLine := isSingleLine && value != "" && isWhiteSpace(value[0])
	if (printAsMultipleLines && !skipLeadingNewLine) || forceLeadingNewLine {
		b.WriteByte('\n')
	}
	b.WriteString(escaped)
	if printAsMultipleLines || forceTrailingNewline {
		b.WriteByte('\n')
	}

	b.WriteString(`"""`)
	return b.String()
}

// PrintBlockString renders a value as a block string.
//
// Setting minimize asks for the shortest form with the same value, which is
// what [utilities.StripIgnoredCharacters] wants; leaving it unset produces the
// readable form a printed schema uses.
func PrintBlockString(value string, minimize bool) string {
	return printBlockString(value, minimize)
}
