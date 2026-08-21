package schema

// keepLastByName collapses the entries of a list that share a name.
//
// graphql-js builds a type's fields, its arguments and an enum's members from
// an object literal, so a name written twice is one entry: the last of them is
// what the object ends up holding, and it holds it under the key's first
// position. This is that rule, and it is applied wherever this package takes a
// list where graphql-js takes an object.
//
// A union's members are the exception, because they are a list on both sides:
// `union U = A | B | A` keeps all three.
//
// An entry that is not there has no name and is left where it is, for the
// benefit of a list assembled in Go with a hole in it: the schema check
// reports the hole rather than this quietly closing it.
func keepLastByName[T comparable](items []T, nameOf func(T) string) []T {
	var absent T
	var out []T
	at := make(map[string]int, len(items))
	for _, held := range items {
		if held == absent {
			out = append(out, held)
			continue
		}
		name := nameOf(held)
		if where, seen := at[name]; seen {
			out[where] = held
			continue
		}
		at[name] = len(out)
		out = append(out, held)
	}
	return out
}
