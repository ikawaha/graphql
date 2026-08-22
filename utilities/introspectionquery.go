package utilities

import "strings"

// IntrospectionOption configures the query [IntrospectionQuery] produces.
type IntrospectionOption func(*introspectionOptions)

// introspectionOptions records which parts of a schema to ask about.
//
// The defaults ask for what every server can answer. The rest is opt-in
// because a server built against an older specification will reject a query
// naming a field its introspection schema does not have, and a client that
// cannot talk to older servers is less useful than one that asks for less.
type introspectionOptions struct {
	descriptions          bool
	specifiedByURL        bool
	directiveIsRepeatable bool
	schemaDescription     bool
	inputValueDeprecation bool
	directiveDeprecation  bool
	oneOf                 bool
	typeDepth             int
}

// WithoutDescriptions leaves the documentation out, which is worth doing when
// the answer is only being compared rather than read.
func WithoutDescriptions() IntrospectionOption {
	return func(o *introspectionOptions) { o.descriptions = false }
}

// WithSpecifiedByURL asks where each custom scalar is specified.
func WithSpecifiedByURL() IntrospectionOption {
	return func(o *introspectionOptions) { o.specifiedByURL = true }
}

// WithDirectiveIsRepeatable asks whether each directive may be applied more
// than once in one place.
func WithDirectiveIsRepeatable() IntrospectionOption {
	return func(o *introspectionOptions) { o.directiveIsRepeatable = true }
}

// WithSchemaDescription asks for the documentation of the schema itself.
func WithSchemaDescription() IntrospectionOption {
	return func(o *introspectionOptions) { o.schemaDescription = true }
}

// WithInputValueDeprecation asks which arguments and input fields are
// deprecated, and includes them in the answer.
func WithInputValueDeprecation() IntrospectionOption {
	return func(o *introspectionOptions) { o.inputValueDeprecation = true }
}

// WithDirectiveDeprecation asks which directives are deprecated, and includes
// them in the answer. It is experimental in graphql-js, so a server built
// against an older specification may not answer it.
func WithDirectiveDeprecation() IntrospectionOption {
	return func(o *introspectionOptions) { o.directiveDeprecation = true }
}

// WithOneOf asks which input objects take exactly one of their fields.
func WithOneOf() IntrospectionOption {
	return func(o *introspectionOptions) { o.oneOf = true }
}

// WithEverything asks for all of it, which is what a round trip needs: a
// schema rebuilt from an answer that left something out would be missing it.
func WithEverything() IntrospectionOption {
	return func(o *introspectionOptions) {
		// Each part is turned on in place rather than the whole being
		// replaced, so that how deep to unfold a type reference — which is
		// not a part of the schema but a shape of the question — survives
		// whichever order the options were given in.
		o.descriptions = true
		o.specifiedByURL = true
		o.directiveIsRepeatable = true
		o.schemaDescription = true
		o.inputValueDeprecation = true
		o.directiveDeprecation = true
		o.oneOf = true
	}
}

// typeRefDepth is how many wrappers a type reference is asked to unfold by
// default.
//
// A reference is a chain of list and non-null wrappers around a name, and the
// query has to spell out one level for each. Nine allows [[[[Type!]!]!]!]!,
// which is already further than any real schema goes.
const typeRefDepth = 9

// maxTypeRefDepth is as far as [WithTypeDepth] will go. Past this the query
// says more about the client than about the schema, and a server reading it
// pays for every level. graphql-js draws the line at the same hundred.
const maxTypeRefDepth = 100

// WithTypeDepth sets how many wrappers a type reference is asked to unfold,
// which defaults to nine.
//
// A depth of zero asks for none, which suits a client that only wants the
// names of things, and so does any negative depth: graphql-js unfolds nothing
// once the level it is counting down reaches zero, whichever side of it the
// caller started on.
//
// A depth beyond a hundred is brought back to a hundred. graphql-js refuses
// one outright, but this returns a query rather than an error or nothing, so
// there is no way to refuse; answering with the most that was ever going to be
// useful is the nearest thing to it, and is at least more than was asked for
// rather than less.
func WithTypeDepth(n int) IntrospectionOption {
	return func(o *introspectionOptions) {
		o.typeDepth = min(max(n, 0), maxTypeRefDepth)
	}
}

// IntrospectionQuery returns the query a client sends to discover a schema.
//
// The result is what [IntrospectionFromSchema] runs and what
// [BuildClientSchema] expects an answer to; a client asking by hand should
// send this rather than write its own, since the two have to agree about what
// was asked for.
func IntrospectionQuery(opts ...IntrospectionOption) string {
	o := introspectionOptions{descriptions: true, typeDepth: typeRefDepth}
	for _, opt := range opts {
		opt(&o)
	}

	// only writes a line when the option asking for it is on.
	only := func(on bool, line string) string {
		if !on {
			return ""
		}
		return line + "\n"
	}
	deprecated := ""
	if o.inputValueDeprecation {
		deprecated = "(includeDeprecated: true)"
	}
	// The documentation is asked for at whatever depth the field sits at.
	describe := func(indent string) string { return only(o.descriptions, indent+"description") }

	var b strings.Builder
	b.WriteString("query IntrospectionQuery {\n  __schema {\n")
	// Asking for the schema's own documentation only makes sense when the
	// documentation is being asked for at all.
	b.WriteString(only(o.descriptions && o.schemaDescription, "    description"))
	b.WriteString("    queryType { name }\n")
	b.WriteString("    mutationType { name }\n")
	b.WriteString("    subscriptionType { name }\n")
	b.WriteString("    types { ...FullType }\n")
	directivesAsked := ""
	if o.directiveDeprecation {
		directivesAsked = "(includeDeprecated: true)"
	}
	b.WriteString("    directives" + directivesAsked + " {\n      name\n")
	b.WriteString(only(o.descriptions, "      description"))
	b.WriteString(only(o.directiveIsRepeatable, "      isRepeatable"))
	b.WriteString(only(o.directiveDeprecation, "      isDeprecated"))
	b.WriteString(only(o.directiveDeprecation, "      deprecationReason"))
	b.WriteString("      locations\n")
	b.WriteString("      args" + deprecated + " { ...InputValue }\n")
	b.WriteString("    }\n  }\n}\n\n")

	b.WriteString("fragment FullType on __Type {\n  kind\n  name\n")
	b.WriteString(only(o.descriptions, "  description"))
	b.WriteString(only(o.specifiedByURL, "  specifiedByURL"))
	b.WriteString(only(o.oneOf, "  isOneOf"))
	b.WriteString("  fields(includeDeprecated: true) {\n    name\n")
	b.WriteString(describe("    "))
	b.WriteString("    args" + deprecated + " { ...InputValue }\n")
	b.WriteString("    type { ...TypeRef }\n    isDeprecated\n    deprecationReason\n  }\n")
	b.WriteString("  inputFields" + deprecated + " { ...InputValue }\n")
	b.WriteString("  interfaces { ...TypeRef }\n")
	b.WriteString("  enumValues(includeDeprecated: true) {\n    name\n")
	b.WriteString(describe("    "))
	b.WriteString("    isDeprecated\n    deprecationReason\n  }\n")
	b.WriteString("  possibleTypes { ...TypeRef }\n}\n\n")

	b.WriteString("fragment InputValue on __InputValue {\n  name\n")
	b.WriteString(describe("  "))
	b.WriteString("  type { ...TypeRef }\n  defaultValue\n")
	b.WriteString(only(o.inputValueDeprecation, "  isDeprecated"))
	b.WriteString(only(o.inputValueDeprecation, "  deprecationReason"))
	b.WriteString("}\n\n")

	b.WriteString("fragment TypeRef on __Type {\n  kind\n  name\n")
	// Each level of wrapping needs a level of nesting to unfold it.
	for i := range o.typeDepth {
		b.WriteString(strings.Repeat("  ", i+1) + "ofType {\n")
		b.WriteString(strings.Repeat("  ", i+2) + "kind\n")
		b.WriteString(strings.Repeat("  ", i+2) + "name\n")
	}
	for i := o.typeDepth; i > 0; i-- {
		b.WriteString(strings.Repeat("  ", i) + "}\n")
	}
	b.WriteString("}\n")

	return b.String()
}
