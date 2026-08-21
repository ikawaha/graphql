package schema

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// The two below are how a message offers a correction. A misspelt field name
// is the commonest thing to get wrong in a query, and whatever knows the real
// names can say which was probably meant.

// maxSuggestions is how many alternatives a message offers. Past a handful the
// list stops being a hint and becomes another thing to read.
const maxSuggestions = 5

// SuggestionList returns the options closest to what was written, nearest
// first, leaving out anything too far away to be a plausible typo.
//
// A misspelt field name is the commonest thing to get wrong in a query, and
// the schema already knows every name it could have been, so saying which is
// nearly always more use than saying the name is unknown.
func SuggestionList(input string, options []string) []string {
	threshold := utf8.RuneCountInString(input)*2/5 + 1
	type scored struct {
		option   string
		distance int
	}
	var found []scored
	seen := map[string]bool{}
	for _, option := range options {
		// An option identical to what was written is measured like any other
		// and comes out nearest, as graphql-js measures it. Callers reach here
		// because a name was not found, so the two rarely coincide, but when
		// they do the caller decides what to make of it.
		if seen[option] {
			continue
		}
		seen[option] = true
		if d, within := lexicalDistance(input, option, threshold); within {
			found = append(found, scored{option, d})
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].distance != found[j].distance {
			return found[i].distance < found[j].distance
		}
		return NaturalCompare(found[i].option, found[j].option) < 0
	})
	out := make([]string, len(found))
	for i, f := range found {
		out[i] = f.option
	}
	return out
}

// lexicalDistance measures how far apart two names are, counting an insertion,
// a deletion, a substitution or a transposition of neighbours as one step.
//
// Case is ignored to the extent that a name differing only in case counts as
// one step away, so "dogname" is offered for "dogName". A pair further apart
// than the threshold is reported as out of range rather than measured, since
// the caller only wants the near ones and the work can stop early.
func lexicalDistance(a, b string, threshold int) (int, bool) {
	if a == b {
		return 0, true
	}
	lowerA, lowerB := strings.ToLower(a), strings.ToLower(b)
	if lowerA == lowerB {
		return 1, threshold >= 1
	}

	x := []rune(lowerA)
	y := []rune(lowerB)
	// A difference in length is a lower bound on the distance, so a pair too
	// different in size can be dismissed without measuring.
	if len(x)-len(y) > threshold || len(y)-len(x) > threshold {
		return 0, false
	}

	// Only three rows of the matrix are ever read: the current one, the one
	// above for insertions and substitutions, and the one above that for a
	// transposition.
	rows := [3][]int{
		make([]int, len(y)+1),
		make([]int, len(y)+1),
		make([]int, len(y)+1),
	}
	for j := range rows[0] {
		rows[0][j] = j
	}

	for i := 1; i <= len(x); i++ {
		current := rows[i%3]
		above := rows[(i-1)%3]
		twoAbove := rows[(i-2+3)%3]

		current[0] = i
		smallest := i
		for j := 1; j <= len(y); j++ {
			cost := 1
			if x[i-1] == y[j-1] {
				cost = 0
			}
			d := min(above[j]+1, current[j-1]+1, above[j-1]+cost)
			if i > 1 && j > 1 && x[i-1] == y[j-2] && x[i-2] == y[j-1] {
				// The two names have a neighbouring pair swapped, which is one
				// slip of the fingers rather than two separate mistakes.
				d = min(d, twoAbove[j-2]+1)
			}
			current[j] = d
			smallest = min(smallest, d)
		}
		// Every remaining row can only add to the distance, so once a whole
		// row is beyond the threshold the answer cannot come back under it.
		if smallest > threshold {
			return 0, false
		}
	}

	d := rows[len(x)%3][len(y)]
	return d, d <= threshold
}

// DidYouMean renders suggestions as the tail of a message, or nothing when
// there are none.
func DidYouMean(subMessage string, suggestions []string) string {
	if len(suggestions) == 0 {
		return ""
	}
	quoted := make([]string, 0, maxSuggestions)
	for _, s := range suggestions[:min(len(suggestions), maxSuggestions)] {
		quoted = append(quoted, quote(s))
	}

	message := " Did you mean "
	if subMessage != "" {
		message += subMessage + " "
	}
	return message + joinList("or", quoted) + "?"
}

// joinList writes a list of things the way a sentence does: "a", "a or b",
// "a, b, or c". It is graphql-js's formatList, and the conjunction is what
// tells its orList and andList apart.
func joinList(conjunction string, items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " " + conjunction + " " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", " + conjunction + " " + items[len(items)-1]
	}
}
