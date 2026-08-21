package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/KnownDirectivesRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_KnownDirectives(t *testing.T) {
	runPorted(t, validation.KnownDirectivesRule, []portedCase{
		{
			name: `with directive defined in schema extension`,
			ownSchema: `
        type Query {
          foo: String
        }
      `,
			steps: []portedStep{
				{
					query: `
          directive @test on OBJECT

          extend type Query @test
        `,
					sdl:              true,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `with directive used in schema extension`,
			ownSchema: `
        directive @test on OBJECT

        type Query {
          foo: String
        }
      `,
			steps: []portedStep{
				{
					query: `
          extend type Query @test
        `,
					sdl:              true,
					againstOwnSchema: true,
				},
			},
		},
		{
			name: `with unknown directive in schema extension`,
			ownSchema: `
        type Query {
          foo: String
        }
      `,
			steps: []portedStep{
				{
					query: `
          extend type Query @unknown
        `,
					sdl:              true,
					againstOwnSchema: true,
					want: []want{
						{At: []at{{2, 29}}},
					},
				},
			},
		},
	})
}

// Not ported, because each of these is written in a way this could not
// follow:
//   - with no directives: the file sets up more than one schema
//   - with standard directives: the file sets up more than one schema
//   - with unknown directive: the file sets up more than one schema
//   - with many unknown directives: the file sets up more than one schema
//   - with well placed directives: the file sets up more than one schema
//   - with misplaced directives: the file sets up more than one schema
//   - with directive defined inside SDL: the file sets up more than one schema
//   - with standard directive: the file sets up more than one schema
//   - with overridden standard directive: the file sets up more than one schema
//   - with well placed directives: the file sets up more than one schema
//   - with misplaced directives: the file sets up more than one schema

// The cases the extractor could not take, because the file sets up two schemas
// and picks between them per case. Written out rather than guessed at.
func TestPorted_KnownDirectives_Rest(t *testing.T) {
	s := upstreamHarness(t)
	rule := validation.KnownDirectivesRule

	t.Run("with many unknown directives", func(t *testing.T) {
		expectErrorsAsWritten(t, s, rule, `
      {
        __typename @unknown
        human @unknown {
          name
          pets @unknown {
            name
          }
        }
      }
    `,
			want{Message: `Unknown directive "@unknown".`, At: []at{{3, 20}}},
			want{Message: `Unknown directive "@unknown".`, At: []at{{4, 15}}},
			want{Message: `Unknown directive "@unknown".`, At: []at{{6, 16}}},
		)
	})

	// A directive the document declares itself is known to the rest of it.
	t.Run("with directive defined inside SDL", func(t *testing.T) {
		expectValidSDL(t, nil, rule, `
			type Query {
				foo: String @test
			}

			directive @test on FIELD_DEFINITION
		`)
	})

	t.Run("with standard directive", func(t *testing.T) {
		expectValidSDL(t, nil, rule, `
			type Query {
				foo: String @deprecated
			}
		`)
	})

	// A document may redeclare a standard directive to mean something else,
	// and the rule reads its own declaration rather than the built-in one.
	t.Run("with overridden standard directive", func(t *testing.T) {
		expectValidSDL(t, nil, rule, `
			schema @deprecated {
				query: Query
			}
			directive @deprecated on SCHEMA
		`)
	})
}
