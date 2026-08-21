package utilities

import (
	"errors"
	"strings"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/validation"
)

// BuildOption changes how a schema is built from a document.
//
// graphql-js takes a single bag of options here, holding both what the parser
// should do and what the builder may take on trust; this is that bag.
type BuildOption func(*buildConfig)

// buildConfig is what a set of options amounts to.
type buildConfig struct {
	parse          []language.ParseOption
	assumeValid    bool
	assumeValidSDL bool
}

func newBuildConfig(opts []BuildOption) *buildConfig {
	c := &buildConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

// skipSDLCheck reports whether the document may be taken as a sound schema
// definition without being checked. Assuming the schema sound assumes the
// document describing it sound too, which is how graphql-js reads the two
// options together.
func (c *buildConfig) skipSDLCheck() bool { return c.assumeValid || c.assumeValidSDL }

// WithParseOptions passes options through to the parser.
//
// Only the calls handed source rather than a document parse anything; a
// document the caller parsed has had its options applied already.
func WithParseOptions(opts ...language.ParseOption) BuildOption {
	return func(c *buildConfig) { c.parse = append(c.parse, opts...) }
}

// AssumeValidSDL builds without first checking that the document is a sound
// schema definition.
//
// That check is what reports a type defined twice, a field defined twice, an
// unknown directive and the rest. Skipping it is for a document already known
// to be sound; one that is not will either fail further in or produce a schema
// that [schema.ValidateSchema] then complains about.
func AssumeValidSDL() BuildOption {
	return func(c *buildConfig) { c.assumeValidSDL = true }
}

// AssumeValid marks the built schema as one that need not be validated, and
// with it the document it was built from.
//
// [schema.ValidateSchema] then reports nothing about it, so the schema is
// taken on trust. Use it for one already known to be sound, where checking it
// again is work done twice.
func AssumeValid() BuildOption {
	return func(c *buildConfig) { c.assumeValid = true }
}

// assertValidSDL reports what is wrong with a document of type definitions,
// which is the check graphql-js makes before building anything from one.
//
// A schema to extend is passed along, since what may be defined depends on
// what is already there: a document defining a type the schema already has is
// wrong, and the same document on its own would not be.
//
// The messages are joined as [schema.AssertValidSchema] joins its own, so a
// caller receives the whole of what is wrong rather than the first of it.
func assertValidSDL(doc *language.Document, toExtend *schema.Schema) error {
	errs := validation.ValidateSDL(doc, toExtend)
	if len(errs) == 0 {
		return nil
	}
	messages := make([]string, len(errs))
	for i, err := range errs {
		messages[i] = err.Message
	}
	return errors.New(strings.Join(messages, "\n\n"))
}
