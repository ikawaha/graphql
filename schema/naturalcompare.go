package schema

// NaturalCompare orders two strings the way a person reading them would: a run
// of digits counts as the number it spells, so "a2" comes before "a10".
//
// It returns a negative number when a comes first, a positive one when b does,
// and zero when they are the same. It is what settles the order of everything
// a reader sees in name order — the suggestions in a "Did you mean …?", the
// types and fields of a sorted schema, the fields of an input object being
// compared — and it is exported because those places live in three packages.
//
// The rule is graphql-js's, quirks and all. A run of digits that starts with a
// zero counts as just that zero: "02" is read as 0 followed by "2", which is
// what makes "02" come before "11". Reproducing that matters more than
// improving on it, since the point is to agree with graphql-js about order.
//
// Bytes are compared where digits are not, which is the same as graphql-js
// comparing UTF-16 code units for everything a GraphQL name may hold.
func NaturalCompare(a, b string) int {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if isDigitByte(a[i]) && isDigitByte(b[j]) {
			var left, right int
			left, i = readNumber(a, i)
			right, j = readNumber(b, j)
			if left != right {
				if left < right {
					return -1
				}
				return 1
			}
			continue
		}
		if a[i] != b[j] {
			if a[i] < b[j] {
				return -1
			}
			return 1
		}
		i++
		j++
	}
	return len(a) - len(b)
}

// readNumber reads the run of digits starting at i and says what it spells and
// where it ended.
//
// The run stops as soon as what has been read is still zero, which is how a
// leading zero ends up standing on its own.
func readNumber(s string, i int) (int, int) {
	number := 0
	for {
		// Guard against a run of digits longer than an int can hold. A name
		// with nineteen digits in it is not something to be exact about, but
		// it is something not to wrap around on.
		if number <= (1<<62)/10 {
			number = number*10 + int(s[i]-'0')
		}
		i++
		if i >= len(s) || !isDigitByte(s[i]) || number == 0 {
			return number, i
		}
	}
}

func isDigitByte(c byte) bool { return '0' <= c && c <= '9' }
