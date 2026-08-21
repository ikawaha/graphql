// Package value provides the foundational types needed to represent GraphQL
// values in Go.
//
// # Why this package exists
//
// GraphQL distinguishes three states at the specification level: "omitted"
// (undefined), "explicit null", and "a value". JavaScript expresses these
// naturally with undefined / null / value, but Go has no equivalent of
// undefined.
//
// This implementation expresses them with the following rules:
//
//   - A nil stored in an any always means GraphQL null.
//     Never use nil to mean "omitted".
//   - Anything that may be omitted is represented in one of exactly three
//     ways, and nothing else:
//     1. presence of a map key (v, ok := m[k])
//     2. a multi-value return (v, ok)
//     3. [Maybe]
//
// The key point is that the zero value of [Maybe] means "omitted", so a field
// left out of a struct literal defaults to undefined, matching JavaScript.
//
// # Why this is a leaf package
//
// [Maybe] appears in the public API (the data of an execution result, argument
// default values, and so on), so it cannot live under internal. At the same
// time the root graphql package will eventually import the subpackages, so
// placing it there would create an import cycle. It is therefore split out as
// a dependency-free leaf package and re-exported from the root via type
// aliases.
package value
