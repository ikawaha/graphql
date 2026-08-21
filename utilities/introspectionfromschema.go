package utilities

import (
	"context"
	"fmt"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// IntrospectionFromSchema returns what a client would learn by asking a schema
// to describe itself.
//
// The answer is produced by running the introspection query rather than by
// reading the schema directly, so it is exactly what a client would receive;
// a second way of describing a schema would be a second thing to keep in step.
//
// Everything a schema can say about itself is asked for, since an answer that
// left something out would rebuild into a schema missing it. An option passed
// here is applied afterwards, so a caller who wants a smaller answer — one
// with no documentation in it, say — can still ask for one.
func IntrospectionFromSchema(
	ctx context.Context,
	s *schema.Schema,
	opts ...IntrospectionOption,
) (*IntrospectionQueryResult, error) {
	asked := append([]IntrospectionOption{WithEverything()}, opts...)
	doc, err := language.ParseString(IntrospectionQuery(asked...))
	if err != nil {
		return nil, fmt.Errorf("parsing the introspection query: %w", err)
	}

	result := execution.Execute(ctx, execution.Request{Schema: s, Document: doc})
	if len(result.Errors) > 0 {
		// The message is the schema's own, worded as a GraphQL response words
		// one, so it is given a line of its own rather than run on from this
		// one: a sentence does not read well inside another.
		return nil, fmt.Errorf("the introspection query failed:\n\t%w", result.Errors[0])
	}
	data, present := result.Data.Get()
	if !present || data == nil {
		return nil, fmt.Errorf("the introspection query returned no data")
	}
	return IntrospectionResultFrom(data)
}
