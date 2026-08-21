package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/SingleFieldSubscriptionsRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_SingleFieldSubscriptions(t *testing.T) {
	runPorted(t, validation.SingleFieldSubscriptionsRule, []portedCase{
		{
			name: `valid subscription`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    importantEmails: [String]
    notImportantEmails: [String]
    moreImportantEmails: [String]
    spamEmails: [String]
    deletedEmails: [String]
    newMessage: Message
  }

  type QueryRoot {
    dummy: String
  }

  schema {
    query: QueryRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription ImportantEmails {
        importantEmails
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `valid subscription with fragment`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    importantEmails: [String]
    notImportantEmails: [String]
    moreImportantEmails: [String]
    spamEmails: [String]
    deletedEmails: [String]
    newMessage: Message
  }

  type QueryRoot {
    dummy: String
  }

  schema {
    query: QueryRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription sub {
        ...newMessageFields
      }

      fragment newMessageFields on SubscriptionRoot {
        newMessage {
          body
          sender
        }
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `valid subscription with fragment and field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    importantEmails: [String]
    notImportantEmails: [String]
    moreImportantEmails: [String]
    spamEmails: [String]
    deletedEmails: [String]
    newMessage: Message
  }

  type QueryRoot {
    dummy: String
  }

  schema {
    query: QueryRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription sub {
        newMessage {
          body
        }
        ...newMessageFields
      }

      fragment newMessageFields on SubscriptionRoot {
        newMessage {
          body
          sender
        }
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `fails with more than one root field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    importantEmails: [String]
    notImportantEmails: [String]
    moreImportantEmails: [String]
    spamEmails: [String]
    deletedEmails: [String]
    newMessage: Message
  }

  type QueryRoot {
    dummy: String
  }

  schema {
    query: QueryRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription ImportantEmails {
        importantEmails
        notImportantEmails
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 9}}},
					},
				},
			},
		},
		{
			name: `fails with more than one root field including introspection`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    importantEmails: [String]
    notImportantEmails: [String]
    moreImportantEmails: [String]
    spamEmails: [String]
    deletedEmails: [String]
    newMessage: Message
  }

  type QueryRoot {
    dummy: String
  }

  schema {
    query: QueryRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription ImportantEmails {
        importantEmails
        __typename
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 9}}},
						{At: []at{{4, 9}}},
					},
				},
			},
		},
		{
			name: `fails with more than one root field including aliased introspection via fragment`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    importantEmails: [String]
    notImportantEmails: [String]
    moreImportantEmails: [String]
    spamEmails: [String]
    deletedEmails: [String]
    newMessage: Message
  }

  type QueryRoot {
    dummy: String
  }

  schema {
    query: QueryRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription ImportantEmails {
        importantEmails
        ...Introspection
      }
      fragment Introspection on SubscriptionRoot {
        typename: __typename
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{7, 9}}},
						{At: []at{{7, 9}}},
					},
				},
			},
		},
		{
			name: `fails with many more than one root field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    importantEmails: [String]
    notImportantEmails: [String]
    moreImportantEmails: [String]
    spamEmails: [String]
    deletedEmails: [String]
    newMessage: Message
  }

  type QueryRoot {
    dummy: String
  }

  schema {
    query: QueryRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription ImportantEmails {
        importantEmails
        notImportantEmails
        spamEmails
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 9}, {5, 9}}},
					},
				},
			},
		},
		{
			name: `fails with many more than one root field via fragments`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    importantEmails: [String]
    notImportantEmails: [String]
    moreImportantEmails: [String]
    spamEmails: [String]
    deletedEmails: [String]
    newMessage: Message
  }

  type QueryRoot {
    dummy: String
  }

  schema {
    query: QueryRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription ImportantEmails {
        importantEmails
        ... {
          more: moreImportantEmails
        }
        ...NotImportantEmails
      }
      fragment NotImportantEmails on SubscriptionRoot {
        notImportantEmails
        deleted: deletedEmails
        ...SpamEmails
      }
      fragment SpamEmails on SubscriptionRoot {
        spamEmails
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{5, 11}, {10, 9}, {11, 9}, {15, 9}}},
					},
				},
			},
		},
		{
			name: `does not infinite loop on recursive fragments`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    importantEmails: [String]
    notImportantEmails: [String]
    moreImportantEmails: [String]
    spamEmails: [String]
    deletedEmails: [String]
    newMessage: Message
  }

  type QueryRoot {
    dummy: String
  }

  schema {
    query: QueryRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription NoInfiniteLoop {
        ...A
      }
      fragment A on SubscriptionRoot {
        ...A
      }
    `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `fails with many more than one root field via fragments (anonymous)`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    importantEmails: [String]
    notImportantEmails: [String]
    moreImportantEmails: [String]
    spamEmails: [String]
    deletedEmails: [String]
    newMessage: Message
  }

  type QueryRoot {
    dummy: String
  }

  schema {
    query: QueryRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription {
        importantEmails
        ... {
          more: moreImportantEmails
          ...NotImportantEmails
        }
        ...NotImportantEmails
      }
      fragment NotImportantEmails on SubscriptionRoot {
        notImportantEmails
        deleted: deletedEmails
        ... {
          ... {
            archivedEmails
          }
        }
        ...SpamEmails
      }
      fragment SpamEmails on SubscriptionRoot {
        spamEmails
        ...NonExistentFragment
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{5, 11}, {11, 9}, {12, 9}, {15, 13}, {21, 9}}},
					},
				},
			},
		},
		{
			name: `fails with more than one root field in anonymous subscriptions`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    importantEmails: [String]
    notImportantEmails: [String]
    moreImportantEmails: [String]
    spamEmails: [String]
    deletedEmails: [String]
    newMessage: Message
  }

  type QueryRoot {
    dummy: String
  }

  schema {
    query: QueryRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription {
        importantEmails
        notImportantEmails
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 9}}},
					},
				},
			},
		},
		{
			name: `fails with introspection field`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    importantEmails: [String]
    notImportantEmails: [String]
    moreImportantEmails: [String]
    spamEmails: [String]
    deletedEmails: [String]
    newMessage: Message
  }

  type QueryRoot {
    dummy: String
  }

  schema {
    query: QueryRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription ImportantEmails {
        __typename
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 9}}},
					},
				},
			},
		},
		{
			name: `fails with introspection field in anonymous subscription`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    importantEmails: [String]
    notImportantEmails: [String]
    moreImportantEmails: [String]
    spamEmails: [String]
    deletedEmails: [String]
    newMessage: Message
  }

  type QueryRoot {
    dummy: String
  }

  schema {
    query: QueryRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription {
        __typename
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 9}}},
					},
				},
			},
		},
		{
			name: `fails with @skip or @include directive`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    importantEmails: [String]
    notImportantEmails: [String]
    moreImportantEmails: [String]
    spamEmails: [String]
    deletedEmails: [String]
    newMessage: Message
  }

  type QueryRoot {
    dummy: String
  }

  schema {
    query: QueryRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription RequiredRuntimeValidation($bool: Boolean!) {
        newMessage @include(if: $bool) {
          body
          sender
        }
        disallowedSecondRootField @skip(if: $bool)
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 20}, {7, 35}}},
					},
				},
			},
		},
		{
			name: `fails with @skip or @include directive in anonymous subscription`,
			ownSchema: `
  type Message {
    body: String
    sender: String
  }

  type SubscriptionRoot {
    importantEmails: [String]
    notImportantEmails: [String]
    moreImportantEmails: [String]
    spamEmails: [String]
    deletedEmails: [String]
    newMessage: Message
  }

  type QueryRoot {
    dummy: String
  }

  schema {
    query: QueryRoot
    subscription: SubscriptionRoot
  }`,
			steps: []portedStep{
				{
					query: `
      subscription ($bool: Boolean!) {
        newMessage @include(if: $bool) {
          body
          sender
        }
        disallowedSecondRootField @skip(if: $bool)
      }
    `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{3, 20}, {7, 35}}},
					},
				},
			},
		},
	})
}

// Not ported, because each of these is written in a way this could not
// follow:
//   - skips if not subscription type: nothing to run
