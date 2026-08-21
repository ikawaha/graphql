package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

func TestLoneSchemaDefinition(t *testing.T) {
	rule := validation.LoneSchemaDefinitionRule

	t.Run("no schema definition", func(t *testing.T) {
		expectValidSDL(t, nil, rule, `type Query { a: String }`)
	})

	t.Run("one schema definition", func(t *testing.T) {
		expectValidSDL(t, nil, rule, `
			schema { query: Query }
			type Query { a: String }
		`)
	})

	t.Run("two schema definitions", func(t *testing.T) {
		expectSDLErrors(t, nil, rule, `
			schema { query: Query }
			schema { mutation: Mutation }
			type Query { a: String }
			type Mutation { b: String }
		`,
			want{Message: "Must provide only one schema definition.", At: []at{{2, 1}}},
		)
	})

	// A schema being extended already says where its operations start.
	t.Run("a schema definition where a schema exists", func(t *testing.T) {
		expectSDLErrors(t, testSchema(t), rule, `schema { query: Query }`,
			want{Message: "Cannot define a new schema within a schema extension.", At: []at{{1, 1}}},
		)
	})
}

func TestUniqueOperationTypes(t *testing.T) {
	rule := validation.UniqueOperationTypesRule

	t.Run("one of each", func(t *testing.T) {
		expectValidSDL(t, nil, rule, `
			schema { query: Query mutation: Mutation }
			type Query { a: String }
			type Mutation { b: String }
		`)
	})

	// A definition and an extension together must still name each root once.
	t.Run("split between a definition and an extension", func(t *testing.T) {
		expectValidSDL(t, nil, rule, `
			schema { query: Query }
			extend schema { mutation: Mutation }
			type Query { a: String }
			type Mutation { b: String }
		`)
	})

	t.Run("a root named twice", func(t *testing.T) {
		expectSDLErrors(t, nil, rule, `
			schema { query: Query }
			extend schema { query: OtherQuery }
			type Query { a: String }
			type OtherQuery { a: String }
		`,
			want{Message: "There can be only one query type in schema.", At: []at{{1, 10}, {2, 17}}},
		)
	})

	t.Run("a root the schema already has", func(t *testing.T) {
		expectSDLErrors(t, testSchema(t), rule, `extend schema { query: Human }`,
			want{Message: "Type for query already defined in the schema. It cannot be redefined.", At: []at{{1, 17}}},
		)
	})
}

func TestUniqueTypeNames(t *testing.T) {
	rule := validation.UniqueTypeNamesRule

	t.Run("differently named types", func(t *testing.T) {
		expectValidSDL(t, nil, rule, `
			type A { a: String }
			type B { b: String }
			scalar C
		`)
	})

	t.Run("a type defined twice", func(t *testing.T) {
		expectSDLErrors(t, nil, rule, `
			type A { a: String }
			scalar A
		`,
			want{Message: `There can be only one type named "A".`, At: []at{{1, 6}, {2, 8}}},
		)
	})

	t.Run("a type the schema already has", func(t *testing.T) {
		expectSDLErrors(t, testSchema(t), rule, `type Dog { other: String }`,
			want{Message: `Type "Dog" already exists in the schema.`, At: []at{{1, 6}}},
		)
	})
}

func TestUniqueEnumValueNames(t *testing.T) {
	rule := validation.UniqueEnumValueNamesRule

	t.Run("differently named members", func(t *testing.T) {
		expectValidSDL(t, nil, rule, `
			enum Colour { RED GREEN }
			extend enum Colour { BLUE }
		`)
	})

	// Two enums may each have a member of the same name.
	t.Run("the same name in different enums", func(t *testing.T) {
		expectValidSDL(t, nil, rule, `
			enum A { X }
			enum B { X }
		`)
	})

	t.Run("a member declared twice", func(t *testing.T) {
		expectSDLErrors(t, nil, rule, `enum Colour { RED RED }`,
			want{Message: `Enum value "Colour.RED" can only be defined once.`, At: []at{{1, 15}, {1, 19}}},
		)
	})

	// A definition and its extensions add to one tally.
	t.Run("a member declared by both a definition and an extension", func(t *testing.T) {
		expectSDLErrors(t, nil, rule, `
			enum Colour { RED }
			extend enum Colour { RED }
		`,
			want{Message: `Enum value "Colour.RED" can only be defined once.`, At: []at{{1, 15}, {2, 22}}},
		)
	})

	t.Run("a member the schema already has", func(t *testing.T) {
		expectSDLErrors(t, testSchema(t), rule, `extend enum FurColor { BROWN }`,
			want{Message: `Enum value "FurColor.BROWN" already exists in the schema.`, At: []at{{1, 24}}},
		)
	})
}

func TestUniqueFieldDefinitionNames(t *testing.T) {
	rule := validation.UniqueFieldDefinitionNamesRule

	t.Run("differently named fields", func(t *testing.T) {
		expectValidSDL(t, nil, rule, `
			type A { a: String b: String }
			interface B { a: String }
			input C { a: String }
		`)
	})

	t.Run("a field declared twice", func(t *testing.T) {
		expectSDLErrors(t, nil, rule, `type A { a: String a: Int }`,
			want{Message: `Field "A.a" can only be defined once.`, At: []at{{1, 10}, {1, 20}}},
		)
	})

	t.Run("an input field declared twice", func(t *testing.T) {
		expectSDLErrors(t, nil, rule, `input A { a: String a: Int }`,
			want{Message: `Field "A.a" can only be defined once.`, At: []at{{1, 11}, {1, 21}}},
		)
	})

	t.Run("a field declared by both a definition and an extension", func(t *testing.T) {
		expectSDLErrors(t, nil, rule, `
			type A { a: String }
			extend type A { a: Int }
		`,
			want{Message: `Field "A.a" can only be defined once.`, At: []at{{1, 10}, {2, 17}}},
		)
	})

	t.Run("a field the schema already has", func(t *testing.T) {
		expectSDLErrors(t, testSchema(t), rule, `extend type Dog { barks: Boolean }`,
			want{Message: `Field "Dog.barks" already exists in the schema.`, At: []at{{1, 19}}},
		)
	})
}

func TestUniqueArgumentDefinitionNames(t *testing.T) {
	rule := validation.UniqueArgumentDefinitionNamesRule

	t.Run("differently named arguments", func(t *testing.T) {
		expectValidSDL(t, nil, rule, `
			type A { f(a: Int, b: Int): String }
			directive @d(a: Int, b: Int) on FIELD
		`)
	})

	// Two fields may each take an argument of the same name.
	t.Run("the same name on different fields", func(t *testing.T) {
		expectValidSDL(t, nil, rule, `
			type A {
				f(a: Int): String
				g(a: Int): String
			}
		`)
	})

	t.Run("a field argument declared twice", func(t *testing.T) {
		expectSDLErrors(t, nil, rule, `type A { f(a: Int, a: Int): String }`,
			want{Message: `Argument "A.f(a:)" can only be defined once.`, At: []at{{1, 12}, {1, 20}}},
		)
	})

	t.Run("a directive argument declared twice", func(t *testing.T) {
		expectSDLErrors(t, nil, rule, `directive @d(a: Int, a: Int) on FIELD`,
			want{Message: `Argument "@d(a:)" can only be defined once.`, At: []at{{1, 14}, {1, 22}}},
		)
	})
}

func TestUniqueDirectiveNames(t *testing.T) {
	rule := validation.UniqueDirectiveNamesRule

	t.Run("differently named directives", func(t *testing.T) {
		expectValidSDL(t, nil, rule, `
			directive @a on FIELD
			directive @b on FIELD
		`)
	})

	t.Run("a directive declared twice", func(t *testing.T) {
		expectSDLErrors(t, nil, rule, `
			directive @a on FIELD
			directive @a on OBJECT
		`,
			want{Message: `There can be only one directive named "@a".`, At: []at{{1, 12}, {2, 12}}},
		)
	})

	t.Run("a directive the schema already has", func(t *testing.T) {
		expectSDLErrors(t, testSchema(t), rule, `directive @onField on OBJECT`,
			want{Message: `Directive "@onField" already exists in the schema. It cannot be redefined.`, At: []at{{1, 12}}},
		)
	})
}

func TestPossibleTypeExtensions(t *testing.T) {
	rule := validation.PossibleTypeExtensionsRule

	t.Run("extensions of the right kind", func(t *testing.T) {
		expectValidSDL(t, nil, rule, `
			scalar S
			extend scalar S @onScalar
			type O { a: String }
			extend type O { b: String }
			interface I { a: String }
			extend interface I { b: String }
			union U = O
			extend union U = O
			enum E { A }
			extend enum E { B }
			input In { a: String }
			extend input In { b: String }
			directive @onScalar on SCALAR
		`)
	})

	t.Run("extending a type of the schema", func(t *testing.T) {
		expectValidSDL(t, testSchema(t), rule, `extend type Dog { colour: String }`)
	})

	t.Run("extending what is not there", func(t *testing.T) {
		expectSDLErrors(t, nil, rule, `
			type Known { a: String }
			extend type Knwon { b: String }
		`,
			want{Message: `Cannot extend type "Knwon" because it is not defined. Did you mean "Known"?`, At: []at{{2, 13}}},
		)
	})

	t.Run("extending as the wrong kind", func(t *testing.T) {
		expectSDLErrors(t, nil, rule, `
			enum E { A }
			extend type E { b: String }
		`,
			want{Message: `Cannot extend non-enum type "E".`, At: []at{{1, 1}, {2, 1}}},
		)
	})

	t.Run("extending a type of the schema as the wrong kind", func(t *testing.T) {
		expectSDLErrors(t, testSchema(t), rule, `extend enum Dog { A }`,
			want{Message: `Cannot extend non-object type "Dog".`, At: []at{{1, 1}}},
		)
	})
}

// Every kind of type can be extended as the wrong kind, and each says which
// kind it really is. A switch that missed one would let a mismatch through.
func TestPossibleTypeExtensions_EveryKind(t *testing.T) {
	rule := validation.PossibleTypeExtensionsRule
	sdl := `
		scalar S
		type O { a: String }
		interface I { a: String }
		union U = O
		enum E { A }
		input In { a: String }
	`
	for _, tt := range []struct {
		extension string
		want      string
	}{
		{"extend scalar O @onScalar", `Cannot extend non-object type "O".`},
		{"extend type S { a: String }", `Cannot extend non-scalar type "S".`},
		{"extend interface E { a: String }", `Cannot extend non-enum type "E".`},
		{"extend union I = O", `Cannot extend non-interface type "I".`},
		{"extend enum U { B }", `Cannot extend non-union type "U".`},
		{"extend input O { b: String }", `Cannot extend non-object type "O".`},
		{"extend type In { b: String }", `Cannot extend non-input object type "In".`},
	} {
		t.Run(tt.extension, func(t *testing.T) {
			expectSDLErrors(t, nil, rule, sdl+"\n"+tt.extension+"\ndirective @onScalar on SCALAR",
				want{Message: tt.want})
		})
	}

	// And the same again against a schema rather than a document, which is the
	// other way the kind is discovered.
	t.Run("against the kinds of a schema", func(t *testing.T) {
		s := testSchema(t)
		for _, tt := range []struct {
			extension string
			want      string
		}{
			{"extend type FurColor { a: String }", `Cannot extend non-enum type "FurColor".`},
			{"extend enum Pet { A }", `Cannot extend non-interface type "Pet".`},
			{"extend type CatOrDog { a: String }", `Cannot extend non-union type "CatOrDog".`},
			{"extend type ComplexInput { a: String }", `Cannot extend non-input object type "ComplexInput".`},
			{"extend type String { a: String }", `Cannot extend non-scalar type "String".`},
		} {
			t.Run(tt.extension, func(t *testing.T) {
				expectSDLErrors(t, s, rule, tt.extension, want{Message: tt.want})
			})
		}
	})
}

// A directive can be written on any of the many places SDL allows, and the
// rule has to find the ones already there at each.
func TestUniqueDirectivesPerLocation_EveryPlace(t *testing.T) {
	rule := validation.UniqueDirectivesPerLocationRule
	expectSDLErrors(t, nil, rule, `
		directive @a on SCALAR | OBJECT | INTERFACE | UNION | ENUM | INPUT_OBJECT | SCHEMA
		directive @b on FIELD_DEFINITION | ARGUMENT_DEFINITION | INPUT_FIELD_DEFINITION | ENUM_VALUE

		schema @a @a { query: Query }
		scalar S @a @a
		type Query @a @a {
			f(arg: Int @b @b): String @b @b
		}
		interface I @a @a { g: String }
		union U @a @a = Query
		enum E @a @a { X @b @b }
		input In @a @a { h: String @b @b }
	`,
		want{Message: `"@a" can only be used once`},
		want{Message: `"@a" can only be used once`},
		want{Message: `"@a" can only be used once`},
		want{Message: `"@b" can only be used once`},
		want{Message: `"@b" can only be used once`},
		want{Message: `"@a" can only be used once`},
		want{Message: `"@a" can only be used once`},
		want{Message: `"@a" can only be used once`},
		want{Message: `"@b" can only be used once`},
		want{Message: `"@a" can only be used once`},
		want{Message: `"@b" can only be used once`},
	)
}
