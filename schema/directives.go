package schema

import (
	"slices"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/value"
)

// DefaultDeprecationReason is what @deprecated says when no reason is given.
const DefaultDeprecationReason = "No longer supported"

// DirectiveConfig describes a directive.
type DirectiveConfig struct {
	Name        string
	Description value.Maybe[string]
	// Locations are the places the directive may be applied.
	Locations []language.DirectiveLocation
	// Args are the directive's arguments, in the order they should be written.
	Args []*Argument
	// IsRepeatable allows the directive to be applied more than once in the
	// same place.
	IsRepeatable bool
	// DeprecationReason marks the directive deprecated when it is not empty.
	DeprecationReason value.Maybe[string]
	ASTNode           *language.DirectiveDefinition
	Extensions        map[string]any
}

// Directive is something a document or a schema can apply to change how an
// element is treated.
type Directive struct {
	name        string
	description value.Maybe[string]

	// Locations are the places the directive may be applied.
	Locations []language.DirectiveLocation
	// Args are the directive's arguments, in declaration order.
	Args []*Argument
	// IsRepeatable reports whether the directive may be applied more than once
	// in the same place.
	IsRepeatable bool
	// DeprecationReason is why the directive should no longer be used, or
	// empty if it still should be.
	DeprecationReason value.Maybe[string]

	ASTNode    *language.DirectiveDefinition
	Extensions map[string]any
}

// NewDirective returns a directive.
func NewDirective(config DirectiveConfig) *Directive {
	d := &Directive{
		name:              config.Name,
		description:       config.Description,
		Locations:         config.Locations,
		IsRepeatable:      config.IsRepeatable,
		DeprecationReason: config.DeprecationReason,
		ASTNode:           config.ASTNode,
		Extensions:        config.Extensions,
	}
	d.Args = cloneArguments(config.Args, d)
	return d
}

// Name is the name the directive is declared under, without the leading at
// sign.
func (d *Directive) Name() string { return d.name }

// Description is the documentation written for the directive, if any.
func (d *Directive) Description() string { return d.description.Or("") }

// DescribedAs is the documentation written for it, telling one written
// as the empty string from none at all. graphql-js keeps the two apart
// and prints and describes them differently.
func (d *Directive) DescribedAs() value.Maybe[string] { return d.description }

// IsDeprecated reports whether the directive has been marked deprecated.
func (d *Directive) IsDeprecated() bool { return d.DeprecationReason.IsSet() }

// Arg returns the argument with the given name, or nil if the directive has
// none.
func (d *Directive) Arg(name string) *Argument {
	if d == nil {
		return nil
	}
	for _, arg := range d.Args {
		if arg.name == name {
			return arg
		}
	}
	return nil
}

// HasLocation reports whether the directive may be applied at a place.
func (d *Directive) HasLocation(loc language.DirectiveLocation) bool {
	return d != nil && slices.Contains(d.Locations, loc)
}

// String renders the directive as it is written in a document.
func (d *Directive) String() string {
	if d == nil {
		return "<nil>"
	}
	return "@" + d.name
}

// Include leaves a selection in the response only when its argument is true.
var Include = NewDirective(DirectiveConfig{
	Name:        "include",
	Description: value.Just("Directs the executor to include this field or fragment only when the `if` argument is true."),
	Locations: []language.DirectiveLocation{
		language.DirectiveLocationField,
		language.DirectiveLocationFragmentSpread,
		language.DirectiveLocationInlineFragment,
	},
	Args: []*Argument{
		NewArgument("if", ArgumentConfig{
			Type:        NewNonNull(Boolean),
			Description: value.Just("Included when true."),
		}),
	},
})

// Skip leaves a selection out of the response when its argument is true.
var Skip = NewDirective(DirectiveConfig{
	Name:        "skip",
	Description: value.Just("Directs the executor to skip this field or fragment when the `if` argument is true."),
	Locations: []language.DirectiveLocation{
		language.DirectiveLocationField,
		language.DirectiveLocationFragmentSpread,
		language.DirectiveLocationInlineFragment,
	},
	Args: []*Argument{
		NewArgument("if", ArgumentConfig{
			Type:        NewNonNull(Boolean),
			Description: value.Just("Skipped when true."),
		}),
	},
})

// Deprecated marks part of a schema as no longer recommended.
//
// Its reason is non-null yet has a default, so a caller may leave it out and
// still get a reason. That is the distinction between having no default and
// having one that happens to be null, made concrete.
var Deprecated = NewDirective(DirectiveConfig{
	Name:        "deprecated",
	Description: value.Just("Marks an element of a GraphQL schema as no longer supported."),
	Locations: []language.DirectiveLocation{
		language.DirectiveLocationFieldDefinition,
		language.DirectiveLocationArgumentDefinition,
		language.DirectiveLocationInputFieldDefinition,
		language.DirectiveLocationEnumValue,
		language.DirectiveLocationDirectiveDefinition,
	},
	Args: []*Argument{
		NewArgument("reason", ArgumentConfig{
			Type: NewNonNull(String),
			Description: value.Just("Explains why this element was deprecated, usually also " +
				"including a suggestion for how to access supported similar " +
				"data. Formatted using the Markdown syntax, as specified by " +
				"[CommonMark](https://commonmark.org/)."),
			Default: DefaultValue(DefaultDeprecationReason),
		}),
	},
})

// SpecifiedBy points at the specification a custom scalar follows.
var SpecifiedBy = NewDirective(DirectiveConfig{
	Name:        "specifiedBy",
	Description: value.Just("Exposes a URL that specifies the behavior of this scalar."),
	Locations:   []language.DirectiveLocation{language.DirectiveLocationScalar},
	Args: []*Argument{
		NewArgument("url", ArgumentConfig{
			Type:        NewNonNull(String),
			Description: value.Just("The URL that specifies the behavior of this scalar."),
		}),
	},
})

// OneOf marks an input object as one where exactly one field must be supplied.
var OneOf = NewDirective(DirectiveConfig{
	Name:        "oneOf",
	Description: value.Just("Indicates exactly one field must be supplied and this field must not be `null`."),
	Locations:   []language.DirectiveLocation{language.DirectiveLocationInputObject},
})

// Defer asks for a fragment to be delivered after the rest of the response.
//
// It is acted on by [execution.ExecuteIncrementally], and only by a schema
// that declares it: it is not one of the directives every schema has.
var Defer = NewDirective(DirectiveConfig{
	Name:        "defer",
	Description: value.Just("Directs the executor to defer this fragment when the `if` argument is true or undefined."),
	Locations: []language.DirectiveLocation{
		language.DirectiveLocationFragmentSpread,
		language.DirectiveLocationInlineFragment,
	},
	Args: []*Argument{
		NewArgument("if", ArgumentConfig{
			Type:        NewNonNull(Boolean),
			Description: value.Just("Deferred when true or undefined."),
			Default:     DefaultValue(true),
		}),
		NewArgument("label", ArgumentConfig{
			Type:        String,
			Description: value.Just("Unique name"),
		}),
	},
})

// Stream asks for a list field to be delivered in pieces: the entries past
// the first few arrive after the rest of the response.
//
// It is acted on by [execution.ExecuteIncrementally], and only by a schema
// that declares it: it is not one of the directives every schema has.
var Stream = NewDirective(DirectiveConfig{
	Name:        "stream",
	Description: value.Just("Directs the executor to stream plural fields when the `if` argument is true or undefined."),
	Locations:   []language.DirectiveLocation{language.DirectiveLocationField},
	Args: []*Argument{
		NewArgument("if", ArgumentConfig{
			Type:        NewNonNull(Boolean),
			Description: value.Just("Stream when true or undefined."),
			Default:     DefaultValue(true),
		}),
		NewArgument("label", ArgumentConfig{
			Type:        String,
			Description: value.Just("Unique name"),
		}),
		NewArgument("initialCount", ArgumentConfig{
			Type:        Int,
			Description: value.Just("Number of items to return immediately"),
			Default:     DefaultValue(int32(0)),
		}),
	},
})

// DisableErrorPropagation asks that a field which may not be null be answered
// with null when it fails, rather than the failure travelling up to the
// nearest place that can hold one.
//
// It is written on the operation and takes no arguments: a request either asks
// for it or does not. What it buys is a response that still carries the data
// that did resolve, at the cost of a `null` where the schema promised there
// would not be one — so a client asking for it has to be ready to read one.
//
// It is experimental, and acted on only by a schema that declares it: it is
// not one of the directives every schema has.
var DisableErrorPropagation = NewDirective(DirectiveConfig{
	Name:        "experimental_disableErrorPropagation",
	Description: value.Just("Disables error propagation."),
	Locations: []language.DirectiveLocation{
		language.DirectiveLocationQuery,
		language.DirectiveLocationMutation,
		language.DirectiveLocationSubscription,
	},
})

// SpecifiedDirectives are the directives every schema has.
//
// The experimental [Defer], [Stream] and [DisableErrorPropagation] are
// deliberately not among them, the same as in the reference implementation: a
// schema only gains them when it asks for them.
var SpecifiedDirectives = []*Directive{Include, Skip, Deprecated, SpecifiedBy, OneOf}

// IsSpecifiedDirective reports whether a directive is one of the built-in
// ones.
func IsSpecifiedDirective(d *Directive) bool {
	if d == nil {
		return false
	}
	// By name, not by identity, as graphql-js asks it: a schema that declares
	// @include for itself has declared the directive the specification names,
	// whether or not it is the same object.
	return slices.ContainsFunc(SpecifiedDirectives, func(specified *Directive) bool {
		return specified.Name() == d.Name()
	})
}
