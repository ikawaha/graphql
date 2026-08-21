package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/KnownArgumentNamesRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_KnownArgumentNames(t *testing.T) {
	runPorted(t, validation.KnownArgumentNamesRule, []portedCase{
		{
			name: `single arg is known`,
			steps: []portedStep{
				{
					query: `
      fragment argOnRequiredArg on Dog {
        doesKnowCommand(dogCommand: SIT)
      }
    `,
				},
			},
		},
		{
			name: `multiple args are known`,
			steps: []portedStep{
				{
					query: `
      fragment multipleArgs on ComplicatedArgs {
        multipleReqs(req1: 1, req2: 2)
      }
    `,
				},
			},
		},
		{
			name: `ignores args of unknown fields`,
			steps: []portedStep{
				{
					query: `
      fragment argOnUnknownField on Dog {
        unknownField(unknownArg: SIT)
      }
    `,
				},
			},
		},
		{
			name: `multiple args in reverse order are known`,
			steps: []portedStep{
				{
					query: `
      fragment multipleArgsReverseOrder on ComplicatedArgs {
        multipleReqs(req2: 2, req1: 1)
      }
    `,
				},
			},
		},
		{
			name: `no args on optional arg`,
			steps: []portedStep{
				{
					query: `
      fragment noArgOnOptionalArg on Dog {
        isHouseTrained
      }
    `,
				},
			},
		},
		{
			name: `args are known deeply`,
			steps: []portedStep{
				{
					query: `
      {
        dog {
          doesKnowCommand(dogCommand: SIT)
        }
        human {
          pet {
            ... on Dog {
              doesKnowCommand(dogCommand: SIT)
            }
          }
        }
      }
    `,
				},
			},
		},
		{
			name: `directive args are known`,
			steps: []portedStep{
				{
					query: `
      {
        dog @skip(if: true)
      }
    `,
				},
			},
		},
		{
			name: `fragment args are known`,
			steps: []portedStep{
				{
					query: `
      {
        dog {
          ...withArg(dogCommand: SIT)
        }
      }
      fragment withArg($dogCommand: DogCommand) on Dog {
        doesKnowCommand(dogCommand: $dogCommand)
      }
    `,
				},
			},
		},
		{
			name: `field args are invalid`,
			steps: []portedStep{
				{
					query: `
      {
        dog @skip(unless: true)
      }
    `,
					want: []want{
						{At: []at{{3, 19}}},
					},
				},
			},
		},
		{
			name: `directive without args is valid`,
			steps: []portedStep{
				{
					query: `
      {
        dog @onField
      }
    `,
				},
			},
		},
		{
			name: `arg passed to directive without arg is reported`,
			steps: []portedStep{
				{
					query: `
      {
        dog @onField(if: true)
      }
    `,
					want: []want{
						{At: []at{{3, 22}}},
					},
				},
			},
		},
		{
			name: `misspelled directive args are reported`,
			steps: []portedStep{
				{
					query: `
      {
        dog @skip(iff: true)
      }
    `,
					want: []want{
						{At: []at{{3, 19}}},
					},
				},
			},
		},
		{
			name: `misspelled directive args are reported (no suggestions)`,
			steps: []portedStep{
				{
					query: `
      {
        dog @skip(iff: true)
      }
    `,
					want: []want{
						{At: []at{{3, 19}}},
					},
				},
			},
		},
		{
			name: `arg passed to fragment without arg is reported`,
			steps: []portedStep{
				{
					query: `
      {
        dog {
          ...withoutArg(unknown: true)
        }
      }
      fragment withoutArg on Dog {
        doesKnowCommand
      }
    `,
					want: []want{
						{At: []at{{4, 25}}},
					},
				},
			},
		},
		{
			name: `misspelled fragment args are reported`,
			steps: []portedStep{
				{
					query: `
      {
        dog {
          ...withArg(command: SIT)
        }
      }
      fragment withArg($dogCommand: DogCommand) on Dog {
        doesKnowCommand(dogCommand: $dogCommand)
      }
    `,
					want: []want{
						{At: []at{{4, 22}}},
					},
				},
			},
		},
		{
			name: `misspelled fragment args are reported (no suggestions)`,
			steps: []portedStep{
				{
					query: `
      {
        dog {
          ...withArg(command: SIT)
        }
      }
      fragment withArg($dogCommand: DogCommand) on Dog {
        doesKnowCommand(dogCommand: $dogCommand)
      }
    `,
					want: []want{
						{At: []at{{4, 22}}},
					},
				},
			},
		},
		{
			name: `invalid arg name`,
			steps: []portedStep{
				{
					query: `
      fragment invalidArgName on Dog {
        doesKnowCommand(unknown: true)
      }
    `,
					want: []want{
						{At: []at{{3, 25}}},
					},
				},
			},
		},
		{
			name: `misspelled arg name is reported`,
			steps: []portedStep{
				{
					query: `
      fragment invalidArgName on Dog {
        doesKnowCommand(DogCommand: true)
      }
    `,
					want: []want{
						{At: []at{{3, 25}}},
					},
				},
			},
		},
		{
			name: `misspelled arg name is reported (no suggestions)`,
			steps: []portedStep{
				{
					query: `
      fragment invalidArgName on Dog {
        doesKnowCommand(DogCommand: true)
      }
    `,
					want: []want{
						{At: []at{{3, 25}}},
					},
				},
			},
		},
		{
			name: `unknown args amongst known args`,
			steps: []portedStep{
				{
					query: `
      fragment oneGoodArgOneInvalidArg on Dog {
        doesKnowCommand(whoKnows: 1, dogCommand: SIT, unknown: true)
      }
    `,
					want: []want{
						{At: []at{{3, 25}}},
						{At: []at{{3, 55}}},
					},
				},
			},
		},
		{
			name: `unknown args deeply`,
			steps: []portedStep{
				{
					query: `
      {
        dog {
          doesKnowCommand(unknown: true)
        }
        human {
          pet {
            ... on Dog {
              doesKnowCommand(unknown: true)
            }
          }
        }
      }
    `,
					want: []want{
						{At: []at{{4, 27}}},
						{At: []at{{9, 31}}},
					},
				},
			},
		},
		{
			name: `known arg on directive defined inside SDL`,
			steps: []portedStep{
				{
					query: `
        type Query {
          foo: String @test(arg: "")
        }

        directive @test(arg: String) on FIELD_DEFINITION
      `,
					sdl: true,
				},
			},
		},
		{
			name: `unknown arg on directive defined inside SDL`,
			steps: []portedStep{
				{
					query: `
        type Query {
          foo: String @test(unknown: "")
        }

        directive @test(arg: String) on FIELD_DEFINITION
      `,
					sdl: true,
					want: []want{
						{At: []at{{3, 29}}},
					},
				},
			},
		},
		{
			name: `misspelled arg name is reported on directive defined inside SDL`,
			steps: []portedStep{
				{
					query: `
        type Query {
          foo: String @test(agr: "")
        }

        directive @test(arg: String) on FIELD_DEFINITION
      `,
					sdl: true,
					want: []want{
						{At: []at{{3, 29}}},
					},
				},
			},
		},
		{
			name: `unknown arg on standard directive`,
			steps: []portedStep{
				{
					query: `
        type Query {
          foo: String @deprecated(unknown: "")
        }
      `,
					sdl: true,
					want: []want{
						{At: []at{{3, 35}}},
					},
				},
			},
		},
		{
			name: `unknown arg on overridden standard directive`,
			steps: []portedStep{
				{
					query: `
        type Query {
          foo: String @deprecated(reason: "")
        }
        directive @deprecated(arg: String) on FIELD
      `,
					sdl: true,
					want: []want{
						{At: []at{{3, 35}}},
					},
				},
			},
		},
		{
			name: `unknown arg on directive defined in schema extension`,
			ownSchema: `
        type Query {
          foo: String
        }
      `,
			steps: []portedStep{
				{
					query: `
          directive @test(arg: String) on OBJECT

          extend type Query  @test(unknown: "")
        `,
					sdl:              true,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{4, 36}}},
					},
				},
			},
		},
		{
			name: `unknown arg on directive used in schema extension`,
			ownSchema: `
        directive @test(arg: String) on OBJECT

        type Query {
          foo: String
        }
      `,
			steps: []portedStep{
				{
					query: `
          extend type Query @test(unknown: "")
        `,
					sdl:              true,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{2, 35}}},
					},
				},
			},
		},
	})
}
