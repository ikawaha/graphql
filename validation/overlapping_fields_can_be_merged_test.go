package validation_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
	"github.com/ikawaha/graphql/validation"
)

// The schema below exists to make fields collide in every way they can: two
// object types with a field of the same name and different types, the same
// name at different nullability, and types deep enough to disagree several
// levels down.
const boxSDL = `
	interface SomeBox {
		a: String
		deepBox: SomeBox
		unrelatedField: String
	}

	type StringBox implements SomeBox {
		a: String
		scalar: String
		deepBox: StringBox
		unrelatedField: String
		listStringBox: [StringBox]
		stringBox: StringBox
		intBox: IntBox
	}

	type IntBox implements SomeBox {
		a: String
		scalar: Int
		deepBox: IntBox
		unrelatedField: String
		listStringBox: [StringBox]
		stringBox: StringBox
		intBox: IntBox
	}

	interface NonNullStringBox1 { scalar: String! }
	type NonNullStringBox1Impl implements SomeBox & NonNullStringBox1 {
		a: String
		scalar: String!
		deepBox: SomeBox
		unrelatedField: String
	}

	interface NonNullStringBox2 { scalar: String! }
	type NonNullStringBox2Impl implements SomeBox & NonNullStringBox2 {
		a: String
		scalar: String!
		deepBox: SomeBox
		unrelatedField: String
	}

	type Connection { edges: [Edge] }
	type Edge { node: Node }
	type Node { id: ID name: String }

	type Query {
		someBox: SomeBox
		connection: Connection
	}`

var (
	boxSchemaOnce sync.Once
	boxSchemaVal  *schema.Schema
	boxSchemaErr  error
)

func boxSchema(t testing.TB) *schema.Schema {
	t.Helper()
	boxSchemaOnce.Do(func() {
		boxSchemaVal, boxSchemaErr = utilities.BuildSchema(boxSDL)
		if boxSchemaErr == nil {
			boxSchemaErr = schema.AssertValidSchema(boxSchemaVal)
		}
	})
	if boxSchemaErr != nil {
		t.Fatalf("building the box schema: %v", boxSchemaErr)
	}
	return boxSchemaVal
}

func TestOverlappingFieldsCanBeMerged_Simple(t *testing.T) {
	s := testSchema(t)
	rule := validation.OverlappingFieldsCanBeMergedRule

	t.Run("unique fields", func(t *testing.T) {
		expectValid(t, s, rule, `fragment f on Dog { name nickname }`)
	})

	t.Run("the same field twice", func(t *testing.T) {
		expectValid(t, s, rule, `fragment f on Dog { name name }`)
	})

	t.Run("the same field with the same arguments", func(t *testing.T) {
		expectValid(t, s, rule, `fragment f on Dog { doesKnowCommand(dogCommand: SIT) doesKnowCommand(dogCommand: SIT) }`)
	})

	// The order arguments are written in is not part of what they mean.
	t.Run("the same arguments written in a different order", func(t *testing.T) {
		expectValid(t, s, rule, `fragment f on Dog { isAtLocation(x: 0, y: 1) isAtLocation(y: 1, x: 0) }`)
	})

	t.Run("different fields under different aliases", func(t *testing.T) {
		expectValid(t, s, rule, `fragment f on Dog { one: name two: nickname }`)
	})

	t.Run("different fields under the same alias", func(t *testing.T) {
		expectErrors(t, s, rule, `fragment f on Dog { one: name one: nickname }`,
			want{
				Message: `Fields "one" conflict because "name" and "nickname" are different fields.`,
				At:      []at{{1, 21}, {1, 31}},
			},
		)
	})

	// An alias that takes the name of another field lands in the same place as
	// that field selected directly.
	t.Run("an alias masking a field of the same name", func(t *testing.T) {
		expectErrors(t, s, rule, `fragment f on Dog { name: nickname name }`,
			want{Message: `Fields "name" conflict because "nickname" and "name" are different fields.`, At: []at{{1, 21}, {1, 36}}},
		)
	})

	t.Run("the same field with differing arguments", func(t *testing.T) {
		expectErrors(t, s, rule, `fragment f on Dog { doesKnowCommand(dogCommand: SIT) doesKnowCommand(dogCommand: HEEL) }`,
			want{Message: `Fields "doesKnowCommand" conflict because they have differing arguments.`, At: []at{{1, 21}, {1, 54}}},
		)
	})

	t.Run("an argument given to only one of two selections", func(t *testing.T) {
		expectErrors(t, s, rule, `fragment f on Dog { isHouseTrained isHouseTrained(atOtherHomes: false) }`,
			want{Message: `they have differing arguments`, At: []at{{1, 21}, {1, 36}}},
		)
	})
}

func TestOverlappingFieldsCanBeMerged_ReturnTypes(t *testing.T) {
	s := boxSchema(t)
	rule := validation.OverlappingFieldsCanBeMergedRule

	// Two selections that can never both apply are free to differ in what they
	// name and what they are given.
	t.Run("the same alias on fields of different object types", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				someBox {
					... on IntBox { name: unrelatedField }
					... on StringBox { name: a }
				}
			}
		`)
	})

	// Return types are the exception: whatever object comes back, the client
	// reads one key, and it cannot be an Int in one case and a String in
	// another.
	t.Run("differing return types even where the fields never both apply", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				someBox {
					... on IntBox { scalar }
					... on StringBox { scalar }
				}
			}
		`,
			want{Message: `Fields "scalar" conflict because they return conflicting types "Int" and "String".`},
		)
	})

	t.Run("differing nullability", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				someBox {
					... on NonNullStringBox1Impl { scalar }
					... on StringBox { scalar }
				}
			}
		`,
			want{Message: `conflicting types "String!" and "String"`},
		)
	})

	t.Run("a list against a single value", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				someBox {
					... on IntBox { box: listStringBox { scalar } }
					... on StringBox { box: stringBox { scalar } }
				}
			}
		`,
			want{Message: `conflicting types "[StringBox]" and "StringBox"`},
		)
	})

	// Two fields of the same wrapped type agree, however they were reached.
	t.Run("the same wrapped return type", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				someBox {
					... on NonNullStringBox1Impl { scalar }
					... on NonNullStringBox2Impl { scalar }
				}
			}
		`)
	})
}

func TestOverlappingFieldsCanBeMerged_Deep(t *testing.T) {
	s := boxSchema(t)
	rule := validation.OverlappingFieldsCanBeMergedRule

	// A conflict found several levels down is reported where the reader wrote
	// the selections that collide, with the path to the disagreement spelt
	// out, rather than at the leaf with no context.
	t.Run("a conflict one level down", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				connection {
					edges { node { id: name } }
					edges { node { id } }
				}
			}
		`,
			want{Message: `Fields "edges" conflict because subfields "node" conflict because subfields "id" conflict because "name" and "id" are different fields.`},
		)
	})

	t.Run("more than one conflict beneath the same fields", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				connection {
					edges { node { id: name, name: id } }
					edges { node { id, name } }
				}
			}
		`,
			want{Message: `subfields "id" conflict because "name" and "id" are different fields and subfields "name" conflict because "id" and "name" are different fields`},
		)
	})
}

func TestOverlappingFieldsCanBeMerged_Fragments(t *testing.T) {
	s := testSchema(t)
	rule := validation.OverlappingFieldsCanBeMergedRule

	t.Run("a conflict between a field and a fragment", func(t *testing.T) {
		expectErrors(t, s, rule, `
			fragment f on Dog {
				name: nickname
				...A
			}
			fragment A on Dog {
				name
			}
		`,
			want{Message: `Fields "name" conflict because "nickname" and "name" are different fields.`},
		)
	})

	t.Run("a conflict between two fragments", func(t *testing.T) {
		expectErrors(t, s, rule, `
			fragment f on Dog {
				...A
				...B
			}
			fragment A on Dog { name: nickname }
			fragment B on Dog { name }
		`,
			want{Message: `Fields "name" conflict`},
		)
	})

	// The same pair of fragments is reached by many routes through a document
	// that spreads them freely, and the reader wants to be told once.
	t.Run("reported once however many routes reach it", func(t *testing.T) {
		expectErrors(t, s, rule, `
			fragment f on Dog {
				...A
				...B
				...A
				...B
			}
			fragment A on Dog { name: nickname }
			fragment B on Dog { name }
		`,
			want{Message: `Fields "name" conflict`},
		)
	})

	// A cycle between fragments is a separate complaint, and following one
	// here must terminate rather than spin.
	t.Run("a fragment that spreads itself", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				dog { ...A }
			}
			fragment A on Dog {
				name
				...A
			}
		`)
	})

	t.Run("two fragments that spread each other", func(t *testing.T) {
		expectValid(t, s, rule, `
			{
				dog { ...A }
			}
			fragment A on Dog { name ...B }
			fragment B on Dog { nickname ...A }
		`)
	})

	// Terminating on a cycle must not mean giving up on what the cycle
	// contains.
	t.Run("a conflict inside a recursive fragment", func(t *testing.T) {
		expectErrors(t, s, rule, `
			{
				dog { ...A }
			}
			fragment A on Dog {
				name: nickname
				name
				...A
			}
		`,
			want{Message: `Fields "name" conflict`},
		)
	})
}

// A rule this involved has to hold up on a document the size of a real one.
func TestOverlappingFieldsCanBeMerged_IsNotQuadratic(t *testing.T) {
	s := testSchema(t)

	// Many fragments spread into one place is the shape that makes a naive
	// implementation compare every pair again for each route to it.
	var b strings.Builder
	const fragments = 40
	b.WriteString("{ dog { ")
	for i := range fragments {
		b.WriteString("...F")
		b.WriteString(string(rune('a' + i%26)))
		b.WriteString(string(rune('a' + i/26)))
		b.WriteString(" ")
	}
	b.WriteString("} }\n")
	for i := range fragments {
		b.WriteString("fragment F")
		b.WriteString(string(rune('a' + i%26)))
		b.WriteString(string(rune('a' + i/26)))
		b.WriteString(" on Dog { name barkVolume }\n")
	}

	expectValid(t, s, validation.OverlappingFieldsCanBeMergedRule, b.String())
}
