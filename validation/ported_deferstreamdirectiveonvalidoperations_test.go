package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/DeferStreamDirectiveOnValidOperationsRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_DeferStreamDirectiveOnValidOperations(t *testing.T) {
	runPorted(t, validation.DeferStreamDirectiveOnValidOperationsRule, []portedCase{
		{
			name: `Defer fragment spread nested in query operation`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      {
        message {
          ...myFragment @defer
        }
      }
      fragment myFragment on Message {
        message {
          body
        }
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Defer inline fragment spread in query operation`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      {
        ... @defer {
          message {
            body
          }
        }
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Defer fragment spread on mutation field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      mutation {
        mutationField {
          ...myFragment @defer
        }
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Defer inline fragment spread on mutation field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      mutation {
        mutationField {
          ... @defer {
            body
          }
        }
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Defer fragment spread on subscription field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription {
        subscriptionField {
          ...myFragment @defer
        }
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 25}}},
					},
				},
			},
		},
		{
			name: `Defer fragment spread with boolean true if argument`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription {
        subscriptionField {
          ...myFragment @defer(if: true)
        }
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 25}}},
					},
				},
			},
		},
		{
			name: `Defer fragment spread with boolean false if argument`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription {
        subscriptionField {
          ...myFragment @defer(if: false)
        }
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Defer fragment spread on query in multi operation document`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription MySubscription {
        subscriptionField {
          ...myFragment
        }
      }
      query MyQuery {
        message {
          ...myFragment @defer
        }
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Defer fragment spread on subscription in multi operation document`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription MySubscription {
        subscriptionField {
          ...myFragment @defer
        }
      }
      query MyQuery {
        message {
          ...myFragment @defer
        }
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 25}}},
					},
				},
			},
		},
		{
			name: `Defer fragment spread with invalid if argument`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription MySubscription {
        subscriptionField {
          ...myFragment @defer(if: "Oops")
        }
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 25}}},
					},
				},
			},
		},
		{
			name: `Defer fragment spread with @skip directive`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription MySubscription {
        subscriptionField {
          ...myFragment @skip @defer
        }
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Defer fragment spread with @skip(if: true) directive`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription MySubscription {
        subscriptionField {
          ...myFragment @skip(if: true) @defer
        }
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Defer fragment spread with @skip(if: false) directive`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription MySubscription {
        subscriptionField {
          ...myFragment @skip(if: false) @defer
        }
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 42}}},
					},
				},
			},
		},
		{
			name: `Defer in fragment spread nested under @skip(if: true) directive`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription MySubscription {
        subscriptionField {
          ...outerFragment @skip(if: true)
        }
      }
      fragment outerFragment on Message {
        ...myFragment @defer
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Defer in fragment spread nested under @skip(if: false) directive`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription MySubscription {
        subscriptionField {
          ...outerFragment @skip(if: false)
        }
      }
      fragment outerFragment on Message {
        ...myFragment @defer
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{8, 23}, {4, 11}}},
					},
				},
			},
		},
		{
			name: `Defer in fragment spread nested under @skip(if: $variable) directive`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription MySubscription($variable: Boolean) {
        subscriptionField {
          ...outerFragment @skip(if: $variable)
        }
      }
      fragment outerFragment on Message {
        ...myFragment @defer
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Defer fragment spread with @skip(if: $variable) directive`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription MySubscription($variable: Boolean) {
        subscriptionField {
          ...myFragment @skip(if: $variable) @defer
        }
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Defer fragment spread with @include directive`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription MySubscription {
        subscriptionField {
          ...myFragment @include @defer
        }
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 34}}},
					},
				},
			},
		},
		{
			name: `Defer fragment spread with @include(if: true) directive`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription MySubscription {
        subscriptionField {
          ...myFragment @include(if: true) @defer
        }
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 44}}},
					},
				},
			},
		},
		{
			name: `Defer fragment spread with @include(if: false) directive`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription MySubscription {
        subscriptionField {
          ...myFragment @include(if: false) @defer
        }
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Defer in fragment spread nested under @include(if: true) directive`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription MySubscription {
        subscriptionField {
          ...outerFragment @include(if: true)
        }
      }
      fragment outerFragment on Message {
        ...myFragment @defer
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{8, 23}, {4, 11}}},
					},
				},
			},
		},
		{
			name: `Defer in fragment spread nested under @include(if: false) directive`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription MySubscription {
        subscriptionField {
          ...outerFragment @include(if: false)
        }
      }
      fragment outerFragment on Message {
        ...myFragment @defer
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Defer in fragment spread nested under @include(if: $variable) directive`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription MySubscription($variable: Boolean) {
        subscriptionField {
          ...outerFragment @include(if: $variable)
        }
      }
      fragment outerFragment on Message {
        ...myFragment @defer
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Defer fragment spread with @include(if: $variable) directive`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription MySubscription ($variable: Boolean) {
        subscriptionField {
          ...myFragment @include(if: $variable) @defer
        }
      }
      fragment myFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Stream on query field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      {
        messages @stream {
          name
        }
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Stream on mutation field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      mutation {
        mutationField {
          messages @stream
        }
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Stream on fragment on mutation field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      mutation {
        mutationField {
          ...myFragment
        }
      }
      fragment myFragment on Message {
        messages @stream
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Stream on subscription field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription {
        subscriptionField {
          messages @stream
        }
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 20}}},
					},
				},
			},
		},
		{
			name: `Stream on fragment on subscription field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription {
        subscriptionField {
          ...myFragment
        }
      }
      fragment myFragment on Message {
        messages @stream
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{8, 18}, {4, 11}}},
					},
				},
			},
		},
		{
			name: `Stream on fragment on query in multi operation document`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription MySubscription {
        subscriptionField {
          message
        }
      }
      query MyQuery {
        message {
          ...myFragment
        }
      }
      fragment myFragment on Message {
        messages @stream
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Stream on subscription in multi operation document`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      query MyQuery {
        message {
          ...myFragment
        }
      }
      subscription MySubscription {
        subscriptionField {
          message {
            ...myFragment
          }
        }
      }
      fragment myFragment on Message {
        messages @stream
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{15, 18}, {10, 13}}},
					},
				},
			},
		},
		{
			name: `Stream on subscription in document with fragment used multiple times`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    subscriptionField: Message
    subscriptionListField: [Message]
  }

  type MutationRoot {
    mutationField: Message
    mutationListField: [Message]
  }

  type QueryRoot {
    message: Message
    messages: [Message]
  }

  schema {
    query: QueryRoot
    mutation: MutationRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription MySubscription {
        subscriptionField {
          message {
            ...myOtherFragment
            ...myFragment  # not visited twice
          }
        }
      }
      fragment myOtherFragment on Message {
        ...myFragment
      }
      fragment myFragment on Message {
        messages @stream
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{14, 18}, {11, 9}, {5, 13}}},
					},
				},
			},
		},
	})
}
