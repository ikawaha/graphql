package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestProvidedRequiredArguments(t *testing.T) {
	s := testSchema(t)
	rule := validation.ProvidedRequiredArgumentsRule

	t.Run("all required arguments given", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				complicatedArgs {
					multipleReqs(req1: 1, req2: 2)
					nonNullIntArgField(nonNullIntArg: 1)
				}
			}
		`)
	})

	// An optional argument may be left out, and so may one with a default even
	// where its type is non-null.
	t.Run("optional arguments left out", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				complicatedArgs {
					multipleOpts
					intArgField
					nonNullFieldWithDefault
				}
				dog { isHouseTrained }
			}
		`)
	})

	t.Run("a required argument left out", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				complicatedArgs {
					multipleReqs(req1: 1)
				}
			}
		`,
			want{Message: `Argument "ComplicatedArgs.multipleReqs(req2:)" of type "Int!" is required, but it was not provided.`, At: []at{{3, 3}}},
		)
	})

	t.Run("all required arguments left out", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				complicatedArgs {
					multipleReqs
				}
			}
		`,
			want{Message: `Argument "ComplicatedArgs.multipleReqs(req1:)" of type "Int!" is required`, At: []at{{3, 3}}},
			want{Message: `Argument "ComplicatedArgs.multipleReqs(req2:)" of type "Int!" is required`, At: []at{{3, 3}}},
		)
	})

	t.Run("a required directive argument left out", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				dog @include { name }
			}
		`,
			want{Message: `Argument "@include(if:)" of type "Boolean!" is required, but it was not provided.`, At: []at{{2, 6}}},
		)
	})

	t.Run("a required argument of a declared directive", func(t *testing.T) {
		expectSDLErrors(t, nil, rule, `
			directive @tag(name: String!) on OBJECT
			type Query @tag { a: String }
		`,
			want{Message: `Argument "@tag(name:)" of type "String!" is required`, At: []at{{2, 12}}},
		)
	})
}
