package graphql

import (
	"context"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/validation"
)

// Harness replaces the stages a request goes through.
//
// [Do] is the composition of parsing, checking and running, and a server
// sometimes wants to reach between them: to time each one, to log a document
// that failed, to cache parses across requests, or to swap a stage for one of
// its own. Setting [Params.Harness] does that without giving up the rest of
// what Do arranges.
//
// A nil member falls back to the standard one, so a harness that only cares
// about parsing sets only Parse. [DefaultHarness] holds the four as they are
// used when nothing is said.
//
// graphql-js has the same idea under the same name, where it covers parse,
// validate, execute and subscribe. This adds the two incremental runs, which
// graphql-js's own graphql() does not reach.
type Harness struct {
	// Parse turns the request's text into a document. It is not called when
	// the request already carries one in [Params.Document].
	Parse func(query string, opts ...language.ParseOption) (*language.Document, error)
	// Validate checks a document against a schema and answers with what is
	// wrong with it. Answering with nothing runs the document.
	Validate func(s *schema.Schema, doc *language.Document, opts ...validation.Option) []*gqlerror.Error
	// Execute answers a request.
	Execute func(ctx context.Context, req execution.Request) execution.Result
	// ExecuteIncrementally answers one using @defer and @stream.
	ExecuteIncrementally func(ctx context.Context, req execution.Request) execution.IncrementalResult
	// ExecuteLegacyIncrementally is the same in the earlier payload format.
	ExecuteLegacyIncrementally func(ctx context.Context, req execution.Request) execution.LegacyIncrementalResult
	// Subscribe starts a subscription.
	Subscribe func(ctx context.Context, req execution.Request) execution.SubscriptionResult
}

// DefaultHarness is what each stage is when a request says nothing, which is
// the packages underneath called in order.
var DefaultHarness = Harness{
	Parse:                      language.ParseString,
	Validate:                   validation.ValidateWithOptions,
	Execute:                    execution.Execute,
	ExecuteIncrementally:       execution.ExecuteIncrementally,
	ExecuteLegacyIncrementally: execution.ExecuteLegacyIncrementally,
	Subscribe:                  execution.Subscribe,
}

// harness returns the stages this request runs through, standing in the
// default for anything it left unset.
func (p Params) harness() Harness {
	h := DefaultHarness
	if p.Harness == nil {
		return h
	}
	if p.Harness.Parse != nil {
		h.Parse = p.Harness.Parse
	}
	if p.Harness.Validate != nil {
		h.Validate = p.Harness.Validate
	}
	if p.Harness.Execute != nil {
		h.Execute = p.Harness.Execute
	}
	if p.Harness.ExecuteIncrementally != nil {
		h.ExecuteIncrementally = p.Harness.ExecuteIncrementally
	}
	if p.Harness.ExecuteLegacyIncrementally != nil {
		h.ExecuteLegacyIncrementally = p.Harness.ExecuteLegacyIncrementally
	}
	if p.Harness.Subscribe != nil {
		h.Subscribe = p.Harness.Subscribe
	}
	return h
}
