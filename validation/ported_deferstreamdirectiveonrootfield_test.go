package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/DeferStreamDirectiveOnRootFieldRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_DeferStreamDirectiveOnRootField(t *testing.T) {
	runPorted(t, validation.DeferStreamDirectiveOnRootFieldRule, []portedCase{
		{
			name: `Defer fragment spread on root query field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  interface Root {
    rootField: Message
  }

  type SubscriptionRoot implements Root {
    subscriptionField: Message
    subscriptionListField: [Message]
    rootField: Message
  }

  type MutationRoot implements Root {
    mutationField: Message
    mutationListField: [Message]
    rootField: Message
  }

  type QueryRoot implements Root {
    message: Message
    messages: [Message]
    rootField: Message
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
        ...rootQueryFragment @defer
      }
      fragment rootQueryFragment on QueryRoot {
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
			name: `Defer inline fragment spread on root query field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  interface Root {
    rootField: Message
  }

  type SubscriptionRoot implements Root {
    subscriptionField: Message
    subscriptionListField: [Message]
    rootField: Message
  }

  type MutationRoot implements Root {
    mutationField: Message
    mutationListField: [Message]
    rootField: Message
  }

  type QueryRoot implements Root {
    message: Message
    messages: [Message]
    rootField: Message
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
			name: `Defer fragment spread on root mutation field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  interface Root {
    rootField: Message
  }

  type SubscriptionRoot implements Root {
    subscriptionField: Message
    subscriptionListField: [Message]
    rootField: Message
  }

  type MutationRoot implements Root {
    mutationField: Message
    mutationListField: [Message]
    rootField: Message
  }

  type QueryRoot implements Root {
    message: Message
    messages: [Message]
    rootField: Message
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
        ...rootFragment @defer
        ...otherFragment
      }
      fragment otherFragment on MutationRoot {
        ...rootFragment
        mutationListField {
          body
        }
      }
      fragment rootFragment on MutationRoot {
        mutationField {
          body
        }
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 25}}},
					},
				},
			},
		},
		{
			name: `Fragment spread cycle on root mutation field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  interface Root {
    rootField: Message
  }

  type SubscriptionRoot implements Root {
    subscriptionField: Message
    subscriptionListField: [Message]
    rootField: Message
  }

  type MutationRoot implements Root {
    mutationField: Message
    mutationListField: [Message]
    rootField: Message
  }

  type QueryRoot implements Root {
    message: Message
    messages: [Message]
    rootField: Message
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
        ...rootFragment
      }
      fragment rootFragment on MutationRoot {
        ...otherFragment
      }
      fragment otherFragment on MutationRoot {
        ...rootFragment
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Self-referencing fragment spread on root mutation field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  interface Root {
    rootField: Message
  }

  type SubscriptionRoot implements Root {
    subscriptionField: Message
    subscriptionListField: [Message]
    rootField: Message
  }

  type MutationRoot implements Root {
    mutationField: Message
    mutationListField: [Message]
    rootField: Message
  }

  type QueryRoot implements Root {
    message: Message
    messages: [Message]
    rootField: Message
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
        ...rootFragment
      }
      fragment rootFragment on MutationRoot {
        ...rootFragment
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Defer inline fragment spread on root mutation field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  interface Root {
    rootField: Message
  }

  type SubscriptionRoot implements Root {
    subscriptionField: Message
    subscriptionListField: [Message]
    rootField: Message
  }

  type MutationRoot implements Root {
    mutationField: Message
    mutationListField: [Message]
    rootField: Message
  }

  type QueryRoot implements Root {
    message: Message
    messages: [Message]
    rootField: Message
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
        ... @defer {
          mutationField {
            body
          }
        }
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 13}}},
					},
				},
			},
		},
		{
			name: `Defer fragment spread on root mutation field interface`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  interface Root {
    rootField: Message
  }

  type SubscriptionRoot implements Root {
    subscriptionField: Message
    subscriptionListField: [Message]
    rootField: Message
  }

  type MutationRoot implements Root {
    mutationField: Message
    mutationListField: [Message]
    rootField: Message
  }

  type QueryRoot implements Root {
    message: Message
    messages: [Message]
    rootField: Message
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
        ...rootFragment
      }
      fragment rootFragment on Root {
        ... @defer {
          rootField {
            body
          }
        }
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{6, 13}}},
					},
				},
			},
		},
		{
			name: `Defer fragment spread on nested mutation field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  interface Root {
    rootField: Message
  }

  type SubscriptionRoot implements Root {
    subscriptionField: Message
    subscriptionListField: [Message]
    rootField: Message
  }

  type MutationRoot implements Root {
    mutationField: Message
    mutationListField: [Message]
    rootField: Message
  }

  type QueryRoot implements Root {
    message: Message
    messages: [Message]
    rootField: Message
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
			name: `Defer fragment spread on root subscription field interface`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  interface Root {
    rootField: Message
  }

  type SubscriptionRoot implements Root {
    subscriptionField: Message
    subscriptionListField: [Message]
    rootField: Message
  }

  type MutationRoot implements Root {
    mutationField: Message
    mutationListField: [Message]
    rootField: Message
  }

  type QueryRoot implements Root {
    message: Message
    messages: [Message]
    rootField: Message
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
        ...rootFragment
      }
      fragment rootFragment on Root {
        ... @defer {
            rootField {
              body
            }
        }
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{6, 13}}},
					},
				},
			},
		},
		{
			name: `Defer fragment spread on root subscription field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  interface Root {
    rootField: Message
  }

  type SubscriptionRoot implements Root {
    subscriptionField: Message
    subscriptionListField: [Message]
    rootField: Message
  }

  type MutationRoot implements Root {
    mutationField: Message
    mutationListField: [Message]
    rootField: Message
  }

  type QueryRoot implements Root {
    message: Message
    messages: [Message]
    rootField: Message
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
        ...rootFragment @defer
      }
      fragment rootFragment on SubscriptionRoot {
        subscriptionField {
          body
        }
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 25}}},
					},
				},
			},
		},
		{
			name: `Defer inline fragment spread on root subscription field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  interface Root {
    rootField: Message
  }

  type SubscriptionRoot implements Root {
    subscriptionField: Message
    subscriptionListField: [Message]
    rootField: Message
  }

  type MutationRoot implements Root {
    mutationField: Message
    mutationListField: [Message]
    rootField: Message
  }

  type QueryRoot implements Root {
    message: Message
    messages: [Message]
    rootField: Message
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
        ... @defer {
          subscriptionField {
            body
          }
        }
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 13}}},
					},
				},
			},
		},
		{
			name: `Defer fragment spread on nested subscription field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  interface Root {
    rootField: Message
  }

  type SubscriptionRoot implements Root {
    subscriptionField: Message
    subscriptionListField: [Message]
    rootField: Message
  }

  type MutationRoot implements Root {
    mutationField: Message
    mutationListField: [Message]
    rootField: Message
  }

  type QueryRoot implements Root {
    message: Message
    messages: [Message]
    rootField: Message
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
          ...nestedFragment @defer
        }
      }
      fragment nestedFragment on Message {
        body
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `Stream field on root query field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  interface Root {
    rootField: Message
  }

  type SubscriptionRoot implements Root {
    subscriptionField: Message
    subscriptionListField: [Message]
    rootField: Message
  }

  type MutationRoot implements Root {
    mutationField: Message
    mutationListField: [Message]
    rootField: Message
  }

  type QueryRoot implements Root {
    message: Message
    messages: [Message]
    rootField: Message
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
			name: `Stream field on fragment on root query field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  interface Root {
    rootField: Message
  }

  type SubscriptionRoot implements Root {
    subscriptionField: Message
    subscriptionListField: [Message]
    rootField: Message
  }

  type MutationRoot implements Root {
    mutationField: Message
    mutationListField: [Message]
    rootField: Message
  }

  type QueryRoot implements Root {
    message: Message
    messages: [Message]
    rootField: Message
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
        ...rootFragment
      }
      fragment rootFragment on QueryType {
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
			name: `Stream field on root mutation field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  interface Root {
    rootField: Message
  }

  type SubscriptionRoot implements Root {
    subscriptionField: Message
    subscriptionListField: [Message]
    rootField: Message
  }

  type MutationRoot implements Root {
    mutationField: Message
    mutationListField: [Message]
    rootField: Message
  }

  type QueryRoot implements Root {
    message: Message
    messages: [Message]
    rootField: Message
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
        mutationListField @stream {
          name
        }
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 27}}},
					},
				},
			},
		},
		{
			name: `Stream field on fragment on root mutation field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  interface Root {
    rootField: Message
  }

  type SubscriptionRoot implements Root {
    subscriptionField: Message
    subscriptionListField: [Message]
    rootField: Message
  }

  type MutationRoot implements Root {
    mutationField: Message
    mutationListField: [Message]
    rootField: Message
  }

  type QueryRoot implements Root {
    message: Message
    messages: [Message]
    rootField: Message
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
        ...rootFragment
      }
      fragment rootFragment on MutationRoot {
        mutationListField @stream {
          name
        }
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{6, 27}}},
					},
				},
			},
		},
		{
			name: `Stream field on root subscription field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  interface Root {
    rootField: Message
  }

  type SubscriptionRoot implements Root {
    subscriptionField: Message
    subscriptionListField: [Message]
    rootField: Message
  }

  type MutationRoot implements Root {
    mutationField: Message
    mutationListField: [Message]
    rootField: Message
  }

  type QueryRoot implements Root {
    message: Message
    messages: [Message]
    rootField: Message
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
        subscriptionListField @stream {
          name
        }
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 31}}},
					},
				},
			},
		},
		{
			name: `Stream field on fragment on root subscription field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  interface Root {
    rootField: Message
  }

  type SubscriptionRoot implements Root {
    subscriptionField: Message
    subscriptionListField: [Message]
    rootField: Message
  }

  type MutationRoot implements Root {
    mutationField: Message
    mutationListField: [Message]
    rootField: Message
  }

  type QueryRoot implements Root {
    message: Message
    messages: [Message]
    rootField: Message
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
        ...rootFragment
      }
      fragment rootFragment on SubscriptionRoot {
        subscriptionListField @stream {
          name
        }
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{6, 31}}},
					},
				},
			},
		},
	})
}
