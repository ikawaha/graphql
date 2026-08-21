package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/OverlappingFieldsCanBeMergedRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_OverlappingFieldsCanBeMerged(t *testing.T) {
	runPorted(t, validation.OverlappingFieldsCanBeMergedRule, []portedCase{
		{
			name: `unique fields`,
			steps: []portedStep{
				{
					query: `
      fragment uniqueFields on Dog {
        name
        nickname
      }
    `,
				},
			},
		},
		{
			name: `identical fields`,
			steps: []portedStep{
				{
					query: `
      fragment mergeIdenticalFields on Dog {
        name
        name
      }
    `,
				},
			},
		},
		{
			name: `identical fields with identical args`,
			steps: []portedStep{
				{
					query: `
      fragment mergeIdenticalFieldsWithIdenticalArgs on Dog {
        doesKnowCommand(dogCommand: SIT)
        doesKnowCommand(dogCommand: SIT)
      }
    `,
				},
			},
		},
		{
			name: `identical fields with identical directives`,
			steps: []portedStep{
				{
					query: `
      fragment mergeSameFieldsWithSameDirectives on Dog {
        name @include(if: true)
        name @include(if: true)
      }
    `,
				},
			},
		},
		{
			name: `different args with different aliases`,
			steps: []portedStep{
				{
					query: `
      fragment differentArgsWithDifferentAliases on Dog {
        knowsSit: doesKnowCommand(dogCommand: SIT)
        knowsDown: doesKnowCommand(dogCommand: DOWN)
      }
    `,
				},
			},
		},
		{
			name: `different directives with different aliases`,
			steps: []portedStep{
				{
					query: `
      fragment differentDirectivesWithDifferentAliases on Dog {
        nameIfTrue: name @include(if: true)
        nameIfFalse: name @include(if: false)
      }
    `,
				},
			},
		},
		{
			name: `different skip/include directives accepted`,
			steps: []portedStep{
				{
					query: `
      fragment differentDirectivesWithDifferentAliases on Dog {
        name @include(if: true)
        name @include(if: false)
      }
    `,
				},
			},
		},
		{
			name: `stream directive used on different instances of the same field`,
			steps: []portedStep{
				{
					query: `
      fragment differentDirectivesWithDifferentAliases on Dog {
        name @stream(label: "streamLabel", initialCount: 1)
        name @stream(label: "streamLabel", initialCount: 1)
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {4, 9}}},
					},
				},
			},
		},
		{
			name: `different stream directive label`,
			steps: []portedStep{
				{
					query: `
      fragment conflictingArgs on Dog {
        name @stream(label: "streamLabel", initialCount: 1)
        name @stream(label: "anotherLabel", initialCount: 1)
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {4, 9}}},
					},
				},
			},
		},
		{
			name: `different stream directive initialCount`,
			steps: []portedStep{
				{
					query: `
      fragment conflictingArgs on Dog {
        name @stream(label: "streamLabel", initialCount: 1)
        name @stream(label: "streamLabel", initialCount: 2)
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {4, 9}}},
					},
				},
			},
		},
		{
			name: `different stream directive first missing args`,
			steps: []portedStep{
				{
					query: `
      fragment conflictingArgs on Dog {
        name @stream
        name @stream(label: "streamLabel", initialCount: 1)
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {4, 9}}},
					},
				},
			},
		},
		{
			name: `different stream directive second missing args`,
			steps: []portedStep{
				{
					query: `
      fragment conflictingArgs on Dog {
        name @stream(label: "streamLabel", initialCount: 1)
        name @stream
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {4, 9}}},
					},
				},
			},
		},
		{
			name: `different stream directive extra argument`,
			steps: []portedStep{
				{
					query: `
      fragment conflictingArgs on Dog {
        name @stream(label: "streamLabel", initialCount: 1)
        name @stream(label: "streamLabel", initialCount: 1, extraArg: true)
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {4, 9}}},
					},
				},
			},
		},
		{
			name: `mix of stream and no stream`,
			steps: []portedStep{
				{
					query: `
      fragment conflictingArgs on Dog {
        name @stream
        name
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {4, 9}}},
					},
				},
			},
		},
		{
			name: `different stream directive both missing args`,
			steps: []portedStep{
				{
					query: `
      fragment conflictingArgs on Dog {
        name @stream
        name @stream
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {4, 9}}},
					},
				},
			},
		},
		{
			name: `Same aliases with different field targets`,
			steps: []portedStep{
				{
					query: `
      fragment sameAliasesWithDifferentFieldTargets on Dog {
        fido: name
        fido: nickname
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {4, 9}}},
					},
				},
			},
		},
		{
			name: `Same aliases allowed on non-overlapping fields`,
			steps: []portedStep{
				{
					query: `
      fragment sameAliasesWithDifferentFieldTargets on Pet {
        ... on Dog {
          name
        }
        ... on Cat {
          name: nickname
        }
      }
    `,
				},
			},
		},
		{
			name: `Alias masking direct field access`,
			steps: []portedStep{
				{
					query: `
      fragment aliasMaskingDirectFieldAccess on Dog {
        name: nickname
        name
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {4, 9}}},
					},
				},
			},
		},
		{
			name: `different args, second adds an argument`,
			steps: []portedStep{
				{
					query: `
      fragment conflictingArgs on Dog {
        doesKnowCommand
        doesKnowCommand(dogCommand: HEEL)
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {4, 9}}},
					},
				},
			},
		},
		{
			name: `different args, second missing an argument`,
			steps: []portedStep{
				{
					query: `
      fragment conflictingArgs on Dog {
        doesKnowCommand(dogCommand: SIT)
        doesKnowCommand
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {4, 9}}},
					},
				},
			},
		},
		{
			name: `different args, first has two, second missing one`,
			steps: []portedStep{
				{
					query: `
      fragment conflictingArgs on Dog {
        doesKnowCommand(dogCommand: SIT, atExpertLevel: true)
        doesKnowCommand(dogCommand: SIT)
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {4, 9}}},
					},
				},
			},
		},
		{
			name: `conflicting arg values`,
			steps: []portedStep{
				{
					query: `
      fragment conflictingArgs on Dog {
        doesKnowCommand(dogCommand: SIT)
        doesKnowCommand(dogCommand: HEEL)
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {4, 9}}},
					},
				},
			},
		},
		{
			name: `conflicting arg names`,
			steps: []portedStep{
				{
					query: `
      fragment conflictingArgs on Dog {
        isAtLocation(x: 0)
        isAtLocation(y: 0)
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {4, 9}}},
					},
				},
			},
		},
		{
			name: `allows different args where no conflict is possible`,
			steps: []portedStep{
				{
					query: `
      fragment conflictingArgs on Pet {
        ... on Dog {
          name(surname: true)
        }
        ... on Cat {
          name
        }
      }
    `,
				},
			},
		},
		{
			name: `allows different order of args`,
			ownSchema: `
      type Query {
        someField(a: String, b: String): String
      }
    `,
			steps: []portedStep{
				{
					query: `
        {
          someField(a: null, b: null)
          someField(b: null, a: null)
        }
      `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `allows different order of input object fields in arg values`,
			ownSchema: `
      input SomeInput {
        a: String
        b: String
      }

      type Query {
        someField(arg: SomeInput): String
      }
    `,
			steps: []portedStep{
				{
					query: `
        {
          someField(arg: { a: null, b: null })
          someField(arg: { b: null, a: null })
        }
      `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `encounters conflict in fragments`,
			steps: []portedStep{
				{
					query: `
      {
        ...A
        ...B
      }
      fragment A on Type {
        x: a
      }
      fragment B on Type {
        x: b
      }
    `,
					want: []want{
						{At: []at{{7, 9}, {10, 9}}},
					},
				},
			},
		},
		{
			name: `reports each conflict once`,
			steps: []portedStep{
				{
					query: `
      {
        f1 {
          ...A
          ...B
        }
        f2 {
          ...B
          ...A
        }
        f3 {
          ...A
          ...B
          x: c
        }
      }
      fragment A on Type {
        x: a
      }
      fragment B on Type {
        x: b
      }
    `,
					want: []want{
						{At: []at{{18, 9}, {21, 9}}},
						{At: []at{{14, 11}, {18, 9}}},
						{At: []at{{14, 11}, {21, 9}}},
					},
				},
			},
		},
		{
			name: `deep conflict`,
			steps: []portedStep{
				{
					query: `
      {
        field {
          x: a
        },
        field {
          x: b
        }
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {4, 11}, {6, 9}, {7, 11}}},
					},
				},
			},
		},
		{
			name: `deep conflict with multiple issues`,
			steps: []portedStep{
				{
					query: `
      {
        field {
          x: a
          y: c
        },
        field {
          x: b
          y: d
        }
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {4, 11}, {5, 11}, {7, 9}, {8, 11}, {9, 11}}},
					},
				},
			},
		},
		{
			name: `does not compare subfields of incompatible parent calls`,
			steps: []portedStep{
				{
					query: `
      fragment incompatibleParents on Dog {
        parent: mother {
          value: name
        }
        parent: father {
          value: mother { name }
        }
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {6, 9}}},
					},
				},
			},
		},
		{
			name: `still compares subfields of each compatible parent-call group`,
			steps: []portedStep{
				{
					query: `
      fragment mixedParents on Dog {
        parent: mother {
          value: name
        }
        parent: mother {
          value: mother { name }
        }
        parent: father { name }
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {4, 11}, {6, 9}, {7, 11}}},
						{At: []at{{3, 9}, {9, 9}}},
						{At: []at{{6, 9}, {9, 9}}},
					},
				},
			},
		},
		{
			name: `does not compare subfields after a stream conflict`,
			steps: []portedStep{
				{
					query: `
      fragment streamBarrier on Dog {
        parent: mother @stream { value: name }
        parent: mother { value: father }
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {4, 9}}},
					},
				},
			},
		},
		{
			name: `very deep conflict`,
			steps: []portedStep{
				{
					query: `
      {
        field {
          deepField {
            x: a
          }
        },
        field {
          deepField {
            x: b
          }
        }
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {4, 11}, {5, 13}, {8, 9}, {9, 11}, {10, 13}}},
					},
				},
			},
		},
		{
			name: `reports deep conflict to nearest common ancestor`,
			steps: []portedStep{
				{
					query: `
      {
        field {
          deepField {
            x: a
          }
          deepField {
            x: b
          }
        },
        field {
          deepField {
            y
          }
        }
      }
    `,
					want: []want{
						{At: []at{{4, 11}, {5, 13}, {7, 11}, {8, 13}}},
					},
				},
			},
		},
		{
			name: `reports deep conflict to nearest common ancestor in fragments`,
			steps: []portedStep{
				{
					query: `
      {
        field {
          ...F
        }
        field {
          ...F
        }
      }
      fragment F on T {
        deepField {
          deeperField {
            x: a
          }
          deeperField {
            x: b
          }
        },
        deepField {
          deeperField {
            y
          }
        }
      }
    `,
					want: []want{
						{At: []at{{12, 11}, {13, 13}, {15, 11}, {16, 13}}},
					},
				},
			},
		},
		{
			name: `reports deep conflict in nested fragments`,
			steps: []portedStep{
				{
					query: `
      {
        field {
          ...I
        }
        field {
          ...F
        }
      }
      fragment F on T {
        x: a
        ...G
      }
      fragment G on T {
        y: c
      }
      fragment I on T {
        y: d
        ...J
      }
      fragment J on T {
        x: b
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {18, 9}, {22, 9}, {6, 9}, {15, 9}, {11, 9}}},
					},
				},
			},
		},
		{
			name: `reports deep conflict after nested fragments`,
			steps: []portedStep{
				{
					query: `
      fragment F on T {
        ...G
      }
      fragment G on T {
        ...H
      }
      fragment H on T {
        x: a
      }
      {
        x: b
        ...F
      }
    `,
					want: []want{
						{At: []at{{12, 9}, {9, 9}}},
					},
				},
			},
		},
		{
			name: `ignores unknown fragments`,
			steps: []portedStep{
				{
					query: `
      {
        field
        ...Unknown
        ...Known
      }

      fragment Known on T {
        field
        ...OtherUnknown
      }
    `,
				},
			},
		},
		{
			name: `conflicting return types which potentially overlap`,
			ownSchema: `
      interface SomeBox {
        deepBox: SomeBox
        unrelatedField: String
      }

      type StringBox implements SomeBox {
        scalar: String
        deepBox: StringBox
        unrelatedField: String
        listNode: [Node]
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      type IntBox implements SomeBox {
        scalar: Int
        deepBox: IntBox
        unrelatedField: String
        node: Node
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      interface NonNullStringBox1 {
        scalar: String!
      }

      type NonNullStringBox1Impl implements SomeBox & NonNullStringBox1 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      interface NonNullStringBox2 {
        scalar: String!
      }

      type NonNullStringBox2Impl implements SomeBox & NonNullStringBox2 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      type Connection {
        edges: [Edge]
      }

      type Edge {
        node: Node
      }

      type Node {
        id: ID
        name: String
      }

      type Query {
        someBox: SomeBox
        connection: Connection
      }
    `,
			steps: []portedStep{
				{
					query: `
          {
            someBox {
              ...on IntBox {
                scalar
              }
              ...on NonNullStringBox1 {
                scalar
              }
            }
          }
        `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{5, 17}, {8, 17}}},
					},
				},
			},
		},
		{
			name: `compatible return shapes on different return types`,
			ownSchema: `
      interface SomeBox {
        deepBox: SomeBox
        unrelatedField: String
      }

      type StringBox implements SomeBox {
        scalar: String
        deepBox: StringBox
        unrelatedField: String
        listNode: [Node]
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      type IntBox implements SomeBox {
        scalar: Int
        deepBox: IntBox
        unrelatedField: String
        node: Node
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      interface NonNullStringBox1 {
        scalar: String!
      }

      type NonNullStringBox1Impl implements SomeBox & NonNullStringBox1 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      interface NonNullStringBox2 {
        scalar: String!
      }

      type NonNullStringBox2Impl implements SomeBox & NonNullStringBox2 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      type Connection {
        edges: [Edge]
      }

      type Edge {
        node: Node
      }

      type Node {
        id: ID
        name: String
      }

      type Query {
        someBox: SomeBox
        connection: Connection
      }
    `,
			steps: []portedStep{
				{
					query: `
          {
            someBox {
              ... on SomeBox {
                deepBox {
                  unrelatedField
                }
              }
              ... on StringBox {
                deepBox {
                  unrelatedField
                }
              }
            }
          }
        `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `disallows differing return types despite no overlap`,
			ownSchema: `
      interface SomeBox {
        deepBox: SomeBox
        unrelatedField: String
      }

      type StringBox implements SomeBox {
        scalar: String
        deepBox: StringBox
        unrelatedField: String
        listNode: [Node]
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      type IntBox implements SomeBox {
        scalar: Int
        deepBox: IntBox
        unrelatedField: String
        node: Node
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      interface NonNullStringBox1 {
        scalar: String!
      }

      type NonNullStringBox1Impl implements SomeBox & NonNullStringBox1 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      interface NonNullStringBox2 {
        scalar: String!
      }

      type NonNullStringBox2Impl implements SomeBox & NonNullStringBox2 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      type Connection {
        edges: [Edge]
      }

      type Edge {
        node: Node
      }

      type Node {
        id: ID
        name: String
      }

      type Query {
        someBox: SomeBox
        connection: Connection
      }
    `,
			steps: []portedStep{
				{
					query: `
          {
            someBox {
              ... on IntBox {
                scalar
              }
              ... on StringBox {
                scalar
              }
            }
          }
        `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{5, 17}, {8, 17}}},
					},
				},
			},
		},
		{
			name: `does not compare subfields after a response-type conflict`,
			ownSchema: `
      interface SomeBox {
        deepBox: SomeBox
        unrelatedField: String
      }

      type StringBox implements SomeBox {
        scalar: String
        deepBox: StringBox
        unrelatedField: String
        listNode: [Node]
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      type IntBox implements SomeBox {
        scalar: Int
        deepBox: IntBox
        unrelatedField: String
        node: Node
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      interface NonNullStringBox1 {
        scalar: String!
      }

      type NonNullStringBox1Impl implements SomeBox & NonNullStringBox1 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      interface NonNullStringBox2 {
        scalar: String!
      }

      type NonNullStringBox2Impl implements SomeBox & NonNullStringBox2 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      type Connection {
        edges: [Edge]
      }

      type Edge {
        node: Node
      }

      type Node {
        id: ID
        name: String
      }

      type Query {
        someBox: SomeBox
        connection: Connection
      }
    `,
			steps: []portedStep{
				{
					query: `
          {
            someBox {
              ... on StringBox {
                parent: listNode { value: name }
              }
              ... on IntBox {
                parent: node { value: id }
              }
            }
          }
        `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{5, 17}, {8, 17}}},
					},
				},
			},
		},
		{
			name: `reports correctly when a non-exclusive follows an exclusive`,
			ownSchema: `
      interface SomeBox {
        deepBox: SomeBox
        unrelatedField: String
      }

      type StringBox implements SomeBox {
        scalar: String
        deepBox: StringBox
        unrelatedField: String
        listNode: [Node]
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      type IntBox implements SomeBox {
        scalar: Int
        deepBox: IntBox
        unrelatedField: String
        node: Node
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      interface NonNullStringBox1 {
        scalar: String!
      }

      type NonNullStringBox1Impl implements SomeBox & NonNullStringBox1 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      interface NonNullStringBox2 {
        scalar: String!
      }

      type NonNullStringBox2Impl implements SomeBox & NonNullStringBox2 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      type Connection {
        edges: [Edge]
      }

      type Edge {
        node: Node
      }

      type Node {
        id: ID
        name: String
      }

      type Query {
        someBox: SomeBox
        connection: Connection
      }
    `,
			steps: []portedStep{
				{
					query: `
          {
            someBox {
              ... on IntBox {
                deepBox {
                  ...X
                }
              }
            }
            someBox {
              ... on StringBox {
                deepBox {
                  ...Y
                }
              }
            }
            memoed: someBox {
              ... on IntBox {
                deepBox {
                  ...X
                }
              }
            }
            memoed: someBox {
              ... on StringBox {
                deepBox {
                  ...Y
                }
              }
            }
            other: someBox {
              ...X
            }
            other: someBox {
              ...Y
            }
          }
          fragment X on SomeBox {
            scalar
          }
          fragment Y on SomeBox {
            scalar: unrelatedField
          }
        `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{31, 13}, {39, 13}, {34, 13}, {42, 13}}},
					},
				},
			},
		},
		{
			name: `disallows differing return type nullability despite no overlap`,
			ownSchema: `
      interface SomeBox {
        deepBox: SomeBox
        unrelatedField: String
      }

      type StringBox implements SomeBox {
        scalar: String
        deepBox: StringBox
        unrelatedField: String
        listNode: [Node]
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      type IntBox implements SomeBox {
        scalar: Int
        deepBox: IntBox
        unrelatedField: String
        node: Node
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      interface NonNullStringBox1 {
        scalar: String!
      }

      type NonNullStringBox1Impl implements SomeBox & NonNullStringBox1 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      interface NonNullStringBox2 {
        scalar: String!
      }

      type NonNullStringBox2Impl implements SomeBox & NonNullStringBox2 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      type Connection {
        edges: [Edge]
      }

      type Edge {
        node: Node
      }

      type Node {
        id: ID
        name: String
      }

      type Query {
        someBox: SomeBox
        connection: Connection
      }
    `,
			steps: []portedStep{
				{
					query: `
          {
            someBox {
              ... on NonNullStringBox1 {
                scalar
              }
              ... on StringBox {
                scalar
              }
            }
          }
        `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{5, 17}, {8, 17}}},
					},
				},
			},
		},
		{
			name: `disallows differing return type list despite no overlap`,
			ownSchema: `
      interface SomeBox {
        deepBox: SomeBox
        unrelatedField: String
      }

      type StringBox implements SomeBox {
        scalar: String
        deepBox: StringBox
        unrelatedField: String
        listNode: [Node]
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      type IntBox implements SomeBox {
        scalar: Int
        deepBox: IntBox
        unrelatedField: String
        node: Node
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      interface NonNullStringBox1 {
        scalar: String!
      }

      type NonNullStringBox1Impl implements SomeBox & NonNullStringBox1 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      interface NonNullStringBox2 {
        scalar: String!
      }

      type NonNullStringBox2Impl implements SomeBox & NonNullStringBox2 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      type Connection {
        edges: [Edge]
      }

      type Edge {
        node: Node
      }

      type Node {
        id: ID
        name: String
      }

      type Query {
        someBox: SomeBox
        connection: Connection
      }
    `,
			steps: []portedStep{
				{
					query: `
          {
            someBox {
              ... on IntBox {
                box: listStringBox {
                  scalar
                }
              }
              ... on StringBox {
                box: stringBox {
                  scalar
                }
              }
            }
          }
        `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{5, 17}, {10, 17}}},
					},
				},
				{
					query: `
          {
            someBox {
              ... on IntBox {
                box: stringBox {
                  scalar
                }
              }
              ... on StringBox {
                box: listStringBox {
                  scalar
                }
              }
            }
          }
        `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{5, 17}, {10, 17}}},
					},
				},
			},
		},
		{
			name: `disallows differing subfields`,
			ownSchema: `
      interface SomeBox {
        deepBox: SomeBox
        unrelatedField: String
      }

      type StringBox implements SomeBox {
        scalar: String
        deepBox: StringBox
        unrelatedField: String
        listNode: [Node]
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      type IntBox implements SomeBox {
        scalar: Int
        deepBox: IntBox
        unrelatedField: String
        node: Node
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      interface NonNullStringBox1 {
        scalar: String!
      }

      type NonNullStringBox1Impl implements SomeBox & NonNullStringBox1 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      interface NonNullStringBox2 {
        scalar: String!
      }

      type NonNullStringBox2Impl implements SomeBox & NonNullStringBox2 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      type Connection {
        edges: [Edge]
      }

      type Edge {
        node: Node
      }

      type Node {
        id: ID
        name: String
      }

      type Query {
        someBox: SomeBox
        connection: Connection
      }
    `,
			steps: []portedStep{
				{
					query: `
          {
            someBox {
              ... on IntBox {
                box: stringBox {
                  val: scalar
                  val: unrelatedField
                }
              }
              ... on StringBox {
                box: stringBox {
                  val: scalar
                }
              }
            }
          }
        `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{6, 19}, {7, 19}}},
					},
				},
			},
		},
		{
			name: `disallows differing deep return types despite no overlap`,
			ownSchema: `
      interface SomeBox {
        deepBox: SomeBox
        unrelatedField: String
      }

      type StringBox implements SomeBox {
        scalar: String
        deepBox: StringBox
        unrelatedField: String
        listNode: [Node]
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      type IntBox implements SomeBox {
        scalar: Int
        deepBox: IntBox
        unrelatedField: String
        node: Node
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      interface NonNullStringBox1 {
        scalar: String!
      }

      type NonNullStringBox1Impl implements SomeBox & NonNullStringBox1 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      interface NonNullStringBox2 {
        scalar: String!
      }

      type NonNullStringBox2Impl implements SomeBox & NonNullStringBox2 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      type Connection {
        edges: [Edge]
      }

      type Edge {
        node: Node
      }

      type Node {
        id: ID
        name: String
      }

      type Query {
        someBox: SomeBox
        connection: Connection
      }
    `,
			steps: []portedStep{
				{
					query: `
          {
            someBox {
              ... on IntBox {
                box: stringBox {
                  scalar
                }
              }
              ... on StringBox {
                box: intBox {
                  scalar
                }
              }
            }
          }
        `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{5, 17}, {6, 19}, {10, 17}, {11, 19}}},
					},
				},
			},
		},
		{
			name: `allows non-conflicting overlapping types`,
			ownSchema: `
      interface SomeBox {
        deepBox: SomeBox
        unrelatedField: String
      }

      type StringBox implements SomeBox {
        scalar: String
        deepBox: StringBox
        unrelatedField: String
        listNode: [Node]
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      type IntBox implements SomeBox {
        scalar: Int
        deepBox: IntBox
        unrelatedField: String
        node: Node
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      interface NonNullStringBox1 {
        scalar: String!
      }

      type NonNullStringBox1Impl implements SomeBox & NonNullStringBox1 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      interface NonNullStringBox2 {
        scalar: String!
      }

      type NonNullStringBox2Impl implements SomeBox & NonNullStringBox2 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      type Connection {
        edges: [Edge]
      }

      type Edge {
        node: Node
      }

      type Node {
        id: ID
        name: String
      }

      type Query {
        someBox: SomeBox
        connection: Connection
      }
    `,
			steps: []portedStep{
				{
					query: `
          {
            someBox {
              ... on IntBox {
                scalar: unrelatedField
              }
              ... on StringBox {
                scalar
              }
            }
          }
        `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `same wrapped scalar return types`,
			ownSchema: `
      interface SomeBox {
        deepBox: SomeBox
        unrelatedField: String
      }

      type StringBox implements SomeBox {
        scalar: String
        deepBox: StringBox
        unrelatedField: String
        listNode: [Node]
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      type IntBox implements SomeBox {
        scalar: Int
        deepBox: IntBox
        unrelatedField: String
        node: Node
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      interface NonNullStringBox1 {
        scalar: String!
      }

      type NonNullStringBox1Impl implements SomeBox & NonNullStringBox1 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      interface NonNullStringBox2 {
        scalar: String!
      }

      type NonNullStringBox2Impl implements SomeBox & NonNullStringBox2 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      type Connection {
        edges: [Edge]
      }

      type Edge {
        node: Node
      }

      type Node {
        id: ID
        name: String
      }

      type Query {
        someBox: SomeBox
        connection: Connection
      }
    `,
			steps: []portedStep{
				{
					query: `
          {
            someBox {
              ...on NonNullStringBox1 {
                scalar
              }
              ...on NonNullStringBox2 {
                scalar
              }
            }
          }
        `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `allows inline fragments without type condition`,
			ownSchema: `
      interface SomeBox {
        deepBox: SomeBox
        unrelatedField: String
      }

      type StringBox implements SomeBox {
        scalar: String
        deepBox: StringBox
        unrelatedField: String
        listNode: [Node]
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      type IntBox implements SomeBox {
        scalar: Int
        deepBox: IntBox
        unrelatedField: String
        node: Node
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      interface NonNullStringBox1 {
        scalar: String!
      }

      type NonNullStringBox1Impl implements SomeBox & NonNullStringBox1 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      interface NonNullStringBox2 {
        scalar: String!
      }

      type NonNullStringBox2Impl implements SomeBox & NonNullStringBox2 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      type Connection {
        edges: [Edge]
      }

      type Edge {
        node: Node
      }

      type Node {
        id: ID
        name: String
      }

      type Query {
        someBox: SomeBox
        connection: Connection
      }
    `,
			steps: []portedStep{
				{
					query: `
          {
            a
            ... {
              a
            }
          }
        `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `compares deep types including list`,
			ownSchema: `
      interface SomeBox {
        deepBox: SomeBox
        unrelatedField: String
      }

      type StringBox implements SomeBox {
        scalar: String
        deepBox: StringBox
        unrelatedField: String
        listNode: [Node]
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      type IntBox implements SomeBox {
        scalar: Int
        deepBox: IntBox
        unrelatedField: String
        node: Node
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      interface NonNullStringBox1 {
        scalar: String!
      }

      type NonNullStringBox1Impl implements SomeBox & NonNullStringBox1 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      interface NonNullStringBox2 {
        scalar: String!
      }

      type NonNullStringBox2Impl implements SomeBox & NonNullStringBox2 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      type Connection {
        edges: [Edge]
      }

      type Edge {
        node: Node
      }

      type Node {
        id: ID
        name: String
      }

      type Query {
        someBox: SomeBox
        connection: Connection
      }
    `,
			steps: []portedStep{
				{
					query: `
          {
            connection {
              ...edgeID
              edges {
                node {
                  id: name
                }
              }
            }
          }

          fragment edgeID on Connection {
            edges {
              node {
                id
              }
            }
          }
        `,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{5, 15}, {6, 17}, {7, 19}, {14, 13}, {15, 15}, {16, 17}}},
					},
				},
			},
		},
		{
			name: `ignores unknown types`,
			ownSchema: `
      interface SomeBox {
        deepBox: SomeBox
        unrelatedField: String
      }

      type StringBox implements SomeBox {
        scalar: String
        deepBox: StringBox
        unrelatedField: String
        listNode: [Node]
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      type IntBox implements SomeBox {
        scalar: Int
        deepBox: IntBox
        unrelatedField: String
        node: Node
        listStringBox: [StringBox]
        stringBox: StringBox
        intBox: IntBox
      }

      interface NonNullStringBox1 {
        scalar: String!
      }

      type NonNullStringBox1Impl implements SomeBox & NonNullStringBox1 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      interface NonNullStringBox2 {
        scalar: String!
      }

      type NonNullStringBox2Impl implements SomeBox & NonNullStringBox2 {
        scalar: String!
        unrelatedField: String
        deepBox: SomeBox
      }

      type Connection {
        edges: [Edge]
      }

      type Edge {
        node: Node
      }

      type Node {
        id: ID
        name: String
      }

      type Query {
        someBox: SomeBox
        connection: Connection
      }
    `,
			steps: []portedStep{
				{
					query: `
          {
            someBox {
              ...on UnknownType {
                scalar
              }
              ...on NonNullStringBox2 {
                scalar
              }
            }
          }
        `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `works for field names that are JS keywords`,
			ownSchema: `
        type Foo {
          constructor: String
        }

        type Query {
          foo: Foo
        }
      `,
			steps: []portedStep{
				{
					query: `
          {
            foo {
              constructor
            }
          }
        `,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `does not infinite loop on recursive fragment`,
			steps: []portedStep{
				{
					query: `
      {
        ...fragA
      }

      fragment fragA on Human { name, relatives { name, ...fragA } }
    `,
				},
			},
		},
		{
			name: `does not infinite loop on immediately recursive fragment`,
			steps: []portedStep{
				{
					query: `
      {
        ...fragA
      }

      fragment fragA on Human { name, ...fragA }
    `,
				},
			},
		},
		{
			name: `does not infinite loop on recursive fragment with a field named after fragment`,
			steps: []portedStep{
				{
					query: `
      {
        ...fragA
        fragA
      }

      fragment fragA on Query { ...fragA }
    `,
				},
			},
		},
		{
			name: `finds invalid cases even with field named after fragment`,
			steps: []portedStep{
				{
					query: `
      {
        fragA
        ...fragA
      }

      fragment fragA on Type {
        fragA: b
      }
    `,
					want: []want{
						{At: []at{{3, 9}, {8, 9}}},
					},
				},
			},
		},
		{
			name: `does not infinite loop on transitively recursive fragment`,
			steps: []portedStep{
				{
					query: `
      {
        ...fragA
        fragB
      }

      fragment fragA on Human { name, ...fragB }
      fragment fragB on Human { name, ...fragC }
      fragment fragC on Human { name, ...fragA }
    `,
				},
			},
		},
		{
			name: `finds invalid case even with immediately recursive fragment`,
			steps: []portedStep{
				{
					query: `
      fragment sameAliasesWithDifferentFieldTargets on Dog {
        ...sameAliasesWithDifferentFieldTargets
        fido: name
        fido: nickname
      }
    `,
					want: []want{
						{At: []at{{4, 9}, {5, 9}}},
					},
				},
			},
		},
		{
			name: `does not infinite loop on recursive fragments separated by fields`,
			steps: []portedStep{
				{
					query: `
      {
        ...fragA
        ...fragB
      }

      fragment fragA on T {
        x {
          ...fragA
          x {
            ...fragA
          }
        }
      }

      fragment fragB on T {
        x {
          ...fragB
          x {
            ...fragB
          }
        }
      }
    `,
				},
			},
		},
		{
			name: `allows conflicting spreads at different depths`,
			steps: []portedStep{
				{
					query: `
        query ValidDifferingFragmentArgs($command1: DogCommand, $command2: DogCommand) {
          dog {
            ...DoesKnowCommand(command: $command1)
            mother {
              ...DoesKnowCommand(command: $command2)
            }
          }
        }
        fragment DoesKnowCommand($command: DogCommand) on Dog {
          doesKnowCommand(dogCommand: $command)
        }
      `,
				},
			},
		},
		{
			name: `encounters conflict in fragments (2)`,
			steps: []portedStep{
				{
					query: `
        {
          ...WithArgs(x: 3)
          ...WithArgs(x: 4)
        }
        fragment WithArgs($x: Int) on Type {
          a(x: $x)
        }
      `,
					want: []want{
						{At: []at{{3, 11}, {4, 11}}},
					},
				},
			},
		},
		{
			name: `allows operations with overlapping fields with arguments using identical operation variables`,
			steps: []portedStep{
				{
					query: `
        query ($y: Int = 1) {
          a(x: $y)
          ...WithArgs(x: 1)
        }
        fragment WithArgs($x: Int = 1) on Type {
          a(x: $y)
        }
      `,
				},
			},
		},
		{
			name: `rejects overlapping fields whose variables have different definitions`,
			steps: []portedStep{
				{
					query: `
        query ($y: Int = 1) {
          a(x: $y)
          ...WithArgs(x: $y)
        }
        fragment WithArgs($x: Int) on Type {
          a(x: $x)
        }
      `,
					want: []want{
						{At: []at{{3, 11}, {7, 11}}},
					},
				},
			},
		},
		{
			name: `rejects overlapping fields with different nested variable definitions`,
			steps: []portedStep{
				{
					query: `
        query ($z: Int = 1) {
          a(x: $z)
          ...WithArgs(y: $z)
        }
        fragment WithArgs($y: Int) on Type {
          ...NestedWithArgs(x: $y)
        }
        fragment NestedWithArgs($x: Int) on Type {
          a(x: $x)
        }
      `,
					want: []want{
						{At: []at{{3, 11}, {10, 11}}},
					},
				},
			},
		},
		{
			name: `rejects a fragment variable even when its default matches a literal`,
			steps: []portedStep{
				{
					query: `
        query {
          a(x: 1)
          ...WithArgs
        }
        fragment WithArgs($x: Int = 1) on Type {
          a(x: $x)
        }
      `,
					want: []want{
						{At: []at{{3, 11}, {7, 11}}},
					},
				},
			},
		},
		{
			name: `keeps an omitted fragment variable distinct from a shadowed operation variable`,
			steps: []portedStep{
				{
					query: `
        query ($x: Int) {
          a(x: $x)
          ...WithArgs
        }
        fragment WithArgs($x: Int) on Type {
          a(x: $x)
        }
      `,
					want: []want{
						{At: []at{{3, 11}, {7, 11}}},
					},
				},
			},
		},
		{
			name: `raises errors with overlapping fields with arguments that conflict via operation variables even with defaults and fragment variable defaults`,
			steps: []portedStep{
				{
					query: `
        query ($y: Int = 1) {
          a(x: $y)
          ...WithArgs
        }
        fragment WithArgs($x: Int = 1) on Type {
          a(x: $x)
        }
      `,
					want: []want{
						{At: []at{{3, 11}, {7, 11}}},
					},
				},
			},
		},
		{
			name: `rejects overlapping fields using list variables with different definitions`,
			steps: []portedStep{
				{
					query: `
        query Query($stringListVarY: [String]) {
          complicatedArgs {
            stringListArgField(stringListArg: $stringListVarY)
            ...WithArgs(stringListVarX: $stringListVarY)
          }
        }
        fragment WithArgs($stringListVarX: [String]) on Type {
          stringListArgField(stringListArg: $stringListVarX)
        }
      `,
					want: []want{
						{At: []at{{4, 13}, {9, 11}}},
					},
				},
			},
		},
		{
			name: `rejects nested list values with different variable definitions`,
			steps: []portedStep{
				{
					query: `
        query Query($stringListVarY: [String]) {
          complicatedArgs {
            stringListArgField(stringListArg: [$stringListVarY, "fixed"])
            ...WithArgs(stringListVarX: $stringListVarY)
          }
        }
        fragment WithArgs($stringListVarX: [String]) on Type {
          stringListArgField(stringListArg: [$stringListVarX, "fixed"])
        }
      `,
					want: []want{
						{At: []at{{4, 13}, {9, 11}}},
					},
				},
			},
		},
		{
			name: `rejects overlapping fields using input object variables with different definitions`,
			steps: []portedStep{
				{
					query: `
        query Query($complexVarY: ComplexInput) {
          complicatedArgs {
            complexArgField(complexArg: $complexVarY)
            ...WithArgs(complexVarX: $complexVarY)
          }
        }
        fragment WithArgs($complexVarX: ComplexInput) on Type {
          complexArgField(complexArg: $complexVarX)
        }
      `,
					want: []want{
						{At: []at{{4, 13}, {9, 11}}},
					},
				},
			},
		},
		{
			name: `rejects nested input values with different variable definitions`,
			steps: []portedStep{
				{
					query: `
        query Query($boolVarY: Boolean) {
          complicatedArgs {
            complexArgField(complexArg: {requiredArg: $boolVarY})
            ...WithArgs(boolVarX: $boolVarY)
          }
        }
        fragment WithArgs($boolVarX: Boolean) on Type {
          complexArgField(complexArg: {requiredArg: $boolVarX})
        }
      `,
					want: []want{
						{At: []at{{4, 13}, {9, 11}}},
					},
				},
			},
		},
		{
			name: `rejects fragment spreads whose variables have different definitions`,
			steps: []portedStep{
				{
					query: `
        query Query($value: Int) {
          ...A(value: $value)
          ...B(value: $value)
        }
        fragment A($value: Int) on Type {
          ...Shared(size: $value)
        }
        fragment B($value: Int) on Type {
          ...Shared(size: $value)
        }
        fragment Shared($size: Int) on Type {
          a(x: $size)
        }
      `,
					want: []want{
						{At: []at{{7, 11}, {10, 11}}},
					},
				},
			},
		},
		{
			name: `allows repeated spreads with the same local variable definitions`,
			steps: []portedStep{
				{
					query: `
        query Query($size: Int) {
          ...Wrapper(size: $size)
          ...Wrapper(size: $size)
        }
        fragment Wrapper($size: Int) on Type {
          a(x: $size)
        }
      `,
				},
			},
		},
		{
			name: `encounters nested field conflict in fragments that could otherwise merge`,
			steps: []portedStep{
				{
					query: `
        query ValidDifferingFragmentArgs($command1: DogCommand, $command2: DogCommand) {
          dog {
            ...DoesKnowCommandNested(command: $command1)
            mother {
              ...DoesKnowCommandNested(command: $command2)
            }
          }
        }
        fragment DoesKnowCommandNested($command: DogCommand) on Dog {
          doesKnowCommand(dogCommand: $command)
          mother {
            doesKnowCommand(dogCommand: $command)
          }
        }
      `,
					want: []want{
						{At: []at{{5, 13}, {13, 13}, {12, 11}, {11, 11}}},
					},
				},
			},
		},
		{
			name: `encounters nested conflict in fragments`,
			steps: []portedStep{
				{
					query: `
        {
          connection {
            edges {
              ...WithArgs(x: 3)
            }
          }
          ...Connection
        }
        fragment Connection on Type {
          connection {
            edges {
              ...WithArgs(x: 4)
            }
          }
        }
        fragment WithArgs($x: Int) on Type {
          a(x: $x)
        }
      `,
					want: []want{
						{At: []at{{5, 15}, {13, 15}}},
					},
				},
			},
		},
	})
}
