package schema

// CheckOption changes how a value or a literal is checked against a type.
type CheckOption func(*checkOptions)

// checkOptions is what the options add up to.
type checkOptions struct {
	hideSuggestions bool
}

// applyCheckOptions reads a list of options into the settings they describe.
func applyCheckOptions(opts []CheckOption) checkOptions {
	var settings checkOptions
	for _, apply := range opts {
		if apply != nil {
			apply(&settings)
		}
	}
	return settings
}

// WithoutSuggestions leaves the "Did you mean …?" out of what a check says.
//
// A suggestion is worked out from the schema, so it names types, fields and
// enum members that the request got close to. A server that does not answer
// introspection is hiding those names on purpose, and a message that offers
// the nearest one hands them over anyway. Turning the suggestions off keeps
// what is wrong being said without saying what would have been right.
func WithoutSuggestions() CheckOption {
	return func(o *checkOptions) { o.hideSuggestions = true }
}

// didYouMean renders a suggestion, or nothing when the caller asked for none.
func (o checkOptions) didYouMean(subMessage string, suggestions []string) string {
	if o.hideSuggestions {
		return ""
	}
	return DidYouMean(subMessage, suggestions)
}
