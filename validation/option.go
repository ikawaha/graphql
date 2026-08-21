package validation

// Option changes how a document is checked.
type Option func(*options)

// options is what the options add up to.
type options struct {
	rules           []Rule
	rulesGiven      bool
	maxErrors       int
	hideSuggestions bool
}

// apply reads a list of options into the settings they describe.
func apply(opts []Option) options {
	settings := options{maxErrors: defaultMaxErrors}
	for _, set := range opts {
		if set != nil {
			set(&settings)
		}
	}
	return settings
}

// WithRules replaces the rules the document is checked against.
//
// Nothing said means [SpecifiedRules], which are the rules the specification
// requires and what almost every server wants. A rule of a server's own goes
// alongside them:
//
//	validation.WithRules(append(slices.Clone(validation.SpecifiedRules), myRule))
func WithRules(rules ...Rule) Option {
	return func(o *options) {
		o.rules = rules
		o.rulesGiven = true
	}
}

// WithMaxErrors bounds how many problems are reported before the check gives
// up. It defaults to 100.
//
// The cap is there because a document that is wrong in a thousand ways
// produces a response nobody reads to the end, and finding all thousand costs
// the server something. Raising it is for tooling that really does want them
// all; lowering it is for a server that would rather answer quickly.
func WithMaxErrors(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.maxErrors = n
		}
	}
}

// WithoutSuggestions leaves the "Did you mean …?" out of every message.
//
// A suggestion is worked out from the schema, so it names types, fields and
// enum members that the document got close to. A server that does not answer
// introspection is hiding those names on purpose, and a message that offers
// the nearest one hands them over anyway. Turning the suggestions off keeps
// what is wrong being said without saying what would have been right.
func WithoutSuggestions() Option {
	return func(o *options) { o.hideSuggestions = true }
}
