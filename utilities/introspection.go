package utilities

import (
	"encoding/json"
	"fmt"

	"github.com/ikawaha/graphql/value"
)

// The types below are the shape of an answer to [IntrospectionQuery]. They are
// what a client receives as JSON, so they can be unmarshalled into directly:
//
//	var answer utilities.IntrospectionQueryResult
//	if err := json.Unmarshal(body, &answer); err != nil { ... }
//	s, err := utilities.BuildClientSchema(&answer)
//
// A description that was not asked for, or that a schema does not have, is the
// empty string: null and "" both mean there is none, and nothing in a schema
// tells them apart. A default value is a *string, because there null means no
// default at all while the string "null" means a default of null, and those
// are different things.

// IntrospectionQueryResult is what an introspection query answers with.
type IntrospectionQueryResult struct {
	Schema IntrospectionSchema `json:"__schema"`
}

// IntrospectionSchema describes a schema.
type IntrospectionSchema struct {
	Description      *string                   `json:"description"`
	QueryType        *IntrospectionTypeRef     `json:"queryType"`
	MutationType     *IntrospectionTypeRef     `json:"mutationType"`
	SubscriptionType *IntrospectionTypeRef     `json:"subscriptionType"`
	Types            []*IntrospectionType      `json:"types"`
	Directives       []*IntrospectionDirective `json:"directives"`
}

// IntrospectionType describes one named type.
type IntrospectionType struct {
	Kind           string  `json:"kind"`
	Name           string  `json:"name"`
	Description    *string `json:"description"`
	SpecifiedByURL string  `json:"specifiedByURL,omitempty"`
	IsOneOf        bool    `json:"isOneOf,omitempty"`
	// The five lists below are omitzero rather than omitempty, because a list
	// a kind does not have and a list that is empty are different answers: an
	// object with no interfaces reports none, while a scalar has no such
	// question to answer. Only the first is nil, and only the second is
	// written back out as [].
	Fields        []*IntrospectionField      `json:"fields,omitzero"`
	InputFields   []*IntrospectionInputValue `json:"inputFields,omitzero"`
	Interfaces    []*IntrospectionTypeRef    `json:"interfaces,omitzero"`
	EnumValues    []*IntrospectionEnumValue  `json:"enumValues,omitzero"`
	PossibleTypes []*IntrospectionTypeRef    `json:"possibleTypes,omitzero"`
}

// IntrospectionTypeRef names a type, wrapped in as many lists and non-nulls as
// it was written with.
type IntrospectionTypeRef struct {
	Kind   string                `json:"kind"`
	Name   string                `json:"name,omitempty"`
	OfType *IntrospectionTypeRef `json:"ofType,omitempty"`
}

// IntrospectionField describes a field of an object or interface.
type IntrospectionField struct {
	Name              string                     `json:"name"`
	Description       *string                    `json:"description"`
	Args              []*IntrospectionInputValue `json:"args"`
	Type              *IntrospectionTypeRef      `json:"type"`
	IsDeprecated      bool                       `json:"isDeprecated"`
	DeprecationReason *string                    `json:"deprecationReason"`
}

// IntrospectionInputValue describes an argument or an input object field.
type IntrospectionInputValue struct {
	Name        string                `json:"name"`
	Description *string               `json:"description"`
	Type        *IntrospectionTypeRef `json:"type"`
	// DefaultValue is the default as it would be written in a document. It is
	// nil where there is no default, which is not the same as a default of
	// null: that arrives as the string "null".
	DefaultValue      *string `json:"defaultValue"`
	IsDeprecated      bool    `json:"isDeprecated"`
	DeprecationReason *string `json:"deprecationReason"`
}

// IntrospectionEnumValue describes a member of an enum.
type IntrospectionEnumValue struct {
	Name              string  `json:"name"`
	Description       *string `json:"description"`
	IsDeprecated      bool    `json:"isDeprecated"`
	DeprecationReason *string `json:"deprecationReason"`
}

// IntrospectionDirective describes a directive a schema allows.
type IntrospectionDirective struct {
	Name              string                     `json:"name"`
	Description       *string                    `json:"description"`
	IsRepeatable      bool                       `json:"isRepeatable,omitempty"`
	IsDeprecated      bool                       `json:"isDeprecated,omitempty"`
	DeprecationReason *string                    `json:"deprecationReason"`
	Locations         []string                   `json:"locations"`
	Args              []*IntrospectionInputValue `json:"args"`
}

// IntrospectionResultFrom reads an executed introspection query into the typed
// form.
//
// It goes through JSON, which is the form a client receives an answer in
// anyway; reading the response map by hand would be a second way of
// interpreting the same thing, and the two would have to be kept in step.
func IntrospectionResultFrom(data *value.OrderedMap) (*IntrospectionQueryResult, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("reading the introspection result: %w", err)
	}
	var result IntrospectionQueryResult
	if err := json.Unmarshal(encoded, &result); err != nil {
		return nil, fmt.Errorf("reading the introspection result: %w", err)
	}
	return &result, nil
}
