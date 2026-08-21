package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
	"github.com/ikawaha/graphql/validation"
)

func TestKnownOperationTypes(t *testing.T) {
	rule := validation.KnownOperationTypesRule

	t.Run("supported operations", func(t *testing.T) {
		expectValid(t, testSchema(t), rule, `
			query { dog { name } }
			mutation { testMutation(arg: "x") }
			subscription { newMessage { body } }
		`)
	})

	t.Run("an unsupported operation", func(t *testing.T) {
		queryOnly, err := utilities.BuildSchema(`type Query { a: String }`)
		if err != nil {
			t.Fatalf("building the schema: %v", err)
		}
		if err := schema.AssertValidSchema(queryOnly); err != nil {
			t.Fatalf("the schema is not sound: %v", err)
		}
		expectErrors(t, queryOnly, rule, `
			query { a }
			mutation { b }
			subscription { c }
		`,
			want{Message: "The mutation operation is not supported by the schema.", At: []at{{2, 1}}},
			want{Message: "The subscription operation is not supported by the schema.", At: []at{{3, 1}}},
		)
	})
}
