package utilities_test

import (
	"strings"
	"testing"

	"github.com/ikawaha/graphql/utilities"
)

// changesBetween builds two schemas and reports what differs, which is the
// shape every test here takes.
func changesBetween(t *testing.T, before, after string) []utilities.Change {
	t.Helper()
	return utilities.FindSchemaChanges(mustBuild(t, before), mustBuild(t, after))
}

// assertChange checks that a change of a kind was reported against a
// coordinate, and returns it.
func assertChange(t *testing.T, changes []utilities.Change, kind, coordinate string) utilities.Change {
	t.Helper()
	for _, change := range changes {
		if change.Kind == kind && change.Coordinate == coordinate {
			return change
		}
	}
	t.Errorf("nothing reported %s at %s; got:\n%s", kind, coordinate, describeChanges(changes))
	return utilities.Change{}
}

// assertNoChange checks that nothing of a kind was reported.
func assertNoChange(t *testing.T, changes []utilities.Change, kind string) {
	t.Helper()
	for _, change := range changes {
		if change.Kind == kind {
			t.Errorf("%s was reported but should not have been: %s", kind, change.Message)
		}
	}
}

func describeChanges(changes []utilities.Change) string {
	if len(changes) == 0 {
		return "  (nothing)"
	}
	var b strings.Builder
	for _, change := range changes {
		b.WriteString("  [" + change.Severity.String() + "] " + change.Kind + " " +
			change.Coordinate + ": " + change.Message + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// A schema compared with itself has changed in no way at all, which is the
// case everything else is measured against.
func TestFindSchemaChanges_Unchanged(t *testing.T) {
	const sdl = `
		enum Colour { RED GREEN }
		interface Node { id: ID! }
		input Filter { term: String limit: Int = 5 }
		type User implements Node { id: ID! name(upper: Boolean = false): String }
		type Photo implements Node { id: ID! }
		union Media = User | Photo
		directive @auth(role: String! = "user") repeatable on FIELD_DEFINITION
		type Query { me: User media: Media colour: Colour search(f: Filter): String }
	`
	if changes := changesBetween(t, sdl, sdl); len(changes) != 0 {
		t.Errorf("a schema differs from itself:\n%s", describeChanges(changes))
	}

	// Documentation is not something a query can observe, so a change to it
	// breaks nothing — but it is still a change, and a tool showing what
	// happened between two versions of a schema wants to say so.
	documented := `
		"Now documented."
		enum Colour { RED GREEN }
		interface Node { id: ID! }
		input Filter { term: String limit: Int = 5 }
		"A person."
		type User implements Node { id: ID! name(upper: Boolean = false): String }
		type Photo implements Node { id: ID! }
		union Media = User | Photo
		directive @auth(role: String! = "user") repeatable on FIELD_DEFINITION
		type Query { me: User media: Media colour: Colour search(f: Filter): String }
	`
	changes := changesBetween(t, sdl, documented)
	if len(changes) != 2 {
		t.Fatalf("%d changes, want 2:\n%s", len(changes), describeChanges(changes))
	}
	for _, change := range changes {
		if change.Kind != utilities.DescriptionChanged {
			t.Errorf("a change of kind %s was reported, want only descriptions", change.Kind)
		}
		if change.Severity != utilities.SafeChange {
			t.Errorf("a description change was called %s, want safe", change.Severity)
		}
	}
}

func TestFindSchemaChanges_Breaking(t *testing.T) {
	tests := []struct {
		name       string
		before     string
		after      string
		kind       string
		coordinate string
	}{
		{
			name:   "a type removed",
			before: `type Gone { a: String } type Query { a: String }`,
			after:  `type Query { a: String }`,
			kind:   utilities.TypeRemoved, coordinate: "Gone",
		},
		{
			name:   "a type changed kind",
			before: `interface Thing { a: String } type Query { t: Thing }`,
			after:  `type Thing { a: String } type Query { t: Thing }`,
			kind:   utilities.TypeChangedKind, coordinate: "Thing",
		},
		{
			name:   "a field removed",
			before: `type Query { a: String b: String }`,
			after:  `type Query { a: String }`,
			kind:   utilities.FieldRemoved, coordinate: "Query.b",
		},
		{
			name:   "a field's type changed",
			before: `type Query { a: String }`,
			after:  `type Query { a: Int }`,
			kind:   utilities.FieldChangedKind, coordinate: "Query.a",
		},
		{
			// A client selecting a list writes different selections from one
			// selecting a single value.
			name:   "a field became a list",
			before: `type Query { a: String }`,
			after:  `type Query { a: [String] }`,
			kind:   utilities.FieldChangedKind, coordinate: "Query.a",
		},
		{
			name:   "a field stopped being non-null",
			before: `type Query { a: String! }`,
			after:  `type Query { a: String }`,
			kind:   utilities.FieldChangedKind, coordinate: "Query.a",
		},
		{
			name:   "an argument removed",
			before: `type Query { f(a: Int, b: Int): String }`,
			after:  `type Query { f(a: Int): String }`,
			kind:   utilities.ArgRemoved, coordinate: "Query.f(b:)",
		},
		{
			name:   "a required argument added",
			before: `type Query { f(a: Int): String }`,
			after:  `type Query { f(a: Int, b: Int!): String }`,
			kind:   utilities.RequiredArgAdded, coordinate: "Query.f(b:)",
		},
		{
			// An argument may become more forgiving, not stricter.
			name:   "an argument became non-null",
			before: `type Query { f(a: Int): String }`,
			after:  `type Query { f(a: Int!): String }`,
			kind:   utilities.ArgChangedKind, coordinate: "Query.f(a:)",
		},
		{
			name:   "an enum member removed",
			before: `enum Colour { RED GREEN } type Query { c: Colour }`,
			after:  `enum Colour { RED } type Query { c: Colour }`,
			kind:   utilities.ValueRemovedFromEnum, coordinate: "Colour.GREEN",
		},
		{
			name:   "a union member removed",
			before: `type A { a: String } type B { b: String } union U = A | B type Query { u: U }`,
			after:  `type A { a: String } type B { b: String } union U = A type Query { u: U }`,
			kind:   utilities.TypeRemovedFromUnion, coordinate: "U",
		},
		{
			name:   "an interface no longer implemented",
			before: `interface N { id: ID! } type T implements N { id: ID! } type Query { t: T }`,
			after:  `interface N { id: ID! } type T { id: ID! } type Query { t: T n: N }`,
			kind:   utilities.ImplementedInterfaceRemoved, coordinate: "T",
		},
		{
			name:   "an input field removed",
			before: `input F { a: String b: String } type Query { f(x: F): String }`,
			after:  `input F { a: String } type Query { f(x: F): String }`,
			kind:   utilities.FieldRemoved, coordinate: "F.b",
		},
		{
			name:   "a required input field added",
			before: `input F { a: String } type Query { f(x: F): String }`,
			after:  `input F { a: String b: Int! } type Query { f(x: F): String }`,
			kind:   utilities.RequiredInputFieldAdded, coordinate: "F.b",
		},
		{
			name:   "a directive removed",
			before: `directive @d on FIELD type Query { a: String }`,
			after:  `type Query { a: String }`,
			kind:   utilities.DirectiveRemoved, coordinate: "@d",
		},
		{
			name:   "a directive argument removed",
			before: `directive @d(a: Int) on FIELD type Query { a: String }`,
			after:  `directive @d on FIELD type Query { a: String }`,
			kind:   utilities.DirectiveArgRemoved, coordinate: "@d(a:)",
		},
		{
			name:   "a required directive argument added",
			before: `directive @d on FIELD type Query { a: String }`,
			after:  `directive @d(a: Int!) on FIELD type Query { a: String }`,
			kind:   utilities.RequiredDirectiveArgAdded, coordinate: "@d(a:)",
		},
		{
			name:   "a directive is no longer repeatable",
			before: `directive @d repeatable on FIELD type Query { a: String }`,
			after:  `directive @d on FIELD type Query { a: String }`,
			kind:   utilities.DirectiveRepeatableRemoved, coordinate: "@d",
		},
		{
			name:   "a directive location removed",
			before: `directive @d on FIELD | OBJECT type Query { a: String }`,
			after:  `directive @d on FIELD type Query { a: String }`,
			kind:   utilities.DirectiveLocationRemoved, coordinate: "@d",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := changesBetween(t, tt.before, tt.after)
			change := assertChange(t, changes, tt.kind, tt.coordinate)
			if change.Severity != utilities.BreakingChange {
				t.Errorf("%s is %s, want breaking", tt.kind, change.Severity)
			}
			// And it turns up in the shorter list too.
			breaking := utilities.FindBreakingChanges(mustBuild(t, tt.before), mustBuild(t, tt.after))
			if len(breaking) == 0 {
				t.Error("FindBreakingChanges reported nothing")
			}
		})
	}
}

func TestFindSchemaChanges_Dangerous(t *testing.T) {
	tests := []struct {
		name       string
		before     string
		after      string
		kind       string
		coordinate string
	}{
		{
			name:   "an enum member added",
			before: `enum Colour { RED } type Query { c: Colour }`,
			after:  `enum Colour { RED GREEN } type Query { c: Colour }`,
			kind:   utilities.ValueAddedToEnum, coordinate: "Colour.GREEN",
		},
		{
			name:   "a union member added",
			before: `type A { a: String } type B { b: String } union U = A type Query { u: U b: B }`,
			after:  `type A { a: String } type B { b: String } union U = A | B type Query { u: U b: B }`,
			kind:   utilities.TypeAddedToUnion, coordinate: "U",
		},
		{
			name:   "an optional argument added",
			before: `type Query { f: String }`,
			after:  `type Query { f(a: Int): String }`,
			kind:   utilities.OptionalArgAdded, coordinate: "Query.f(a:)",
		},
		{
			name:   "an optional input field added",
			before: `input F { a: String } type Query { f(x: F): String }`,
			after:  `input F { a: String b: Int } type Query { f(x: F): String }`,
			kind:   utilities.OptionalInputFieldAdded, coordinate: "F.b",
		},
		{
			name:   "an interface newly implemented",
			before: `interface N { id: ID! } type T { id: ID! } type Query { t: T n: N }`,
			after:  `interface N { id: ID! } type T implements N { id: ID! } type Query { t: T n: N }`,
			kind:   utilities.ImplementedInterfaceAdded, coordinate: "T",
		},
		{
			// A query that leaves the argument out will be answered
			// differently than before, without saying anything different.
			name:   "a default value changed",
			before: `type Query { f(a: Int = 1): String }`,
			after:  `type Query { f(a: Int = 2): String }`,
			kind:   utilities.ArgDefaultValueChange, coordinate: "Query.f(a:)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := changesBetween(t, tt.before, tt.after)
			change := assertChange(t, changes, tt.kind, tt.coordinate)
			if change.Severity != utilities.DangerousChange {
				t.Errorf("%s is %s, want dangerous", tt.kind, change.Severity)
			}
			breaking := utilities.FindBreakingChanges(mustBuild(t, tt.before), mustBuild(t, tt.after))
			if len(breaking) != 0 {
				t.Errorf("a dangerous change was also reported as breaking:\n%s", describeChanges(breaking))
			}
		})
	}
}

// A type may become more specific on the way out and more forgiving on the way
// in. Getting either direction backwards would either refuse a safe change or
// let a breaking one through, so both are pinned.
func TestFindSchemaChanges_SafeTypeChanges(t *testing.T) {
	tests := []struct{ name, before, after string }{
		// Out: a value the client already copes with may now always be there.
		{"a field became non-null", `type Query { a: String }`, `type Query { a: String! }`},
		{"a list entry became non-null", `type Query { a: [String] }`, `type Query { a: [String!] }`},
		{"a list became non-null", `type Query { a: [String] }`, `type Query { a: [String]! }`},
		// In: everything the client was sending is still accepted.
		{"an argument became nullable", `type Query { f(a: Int!): String }`, `type Query { f(a: Int): String }`},
		{"an input field became nullable",
			`input F { a: Int! } type Query { f(x: F): String }`,
			`input F { a: Int } type Query { f(x: F): String }`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := changesBetween(t, tt.before, tt.after)
			assertNoChange(t, changes, utilities.FieldChangedKind)
			assertNoChange(t, changes, utilities.ArgChangedKind)
		})
	}
}

func TestFindSchemaChanges_Safe(t *testing.T) {
	changes := changesBetween(t,
		`type Query { a: String }`,
		`type Query { a: String b: String } type Added { c: String } directive @d on FIELD`)

	assertChange(t, changes, utilities.FieldAdded, "Query.b")
	assertChange(t, changes, utilities.TypeAdded, "Added")
	assertChange(t, changes, utilities.DirectiveAdded, "@d")
	for _, change := range changes {
		if change.Severity != utilities.SafeChange {
			t.Errorf("%s is %s, want safe", change.Kind, change.Severity)
		}
	}
	if len(utilities.FindBreakingChanges(
		mustBuild(t, `type Query { a: String }`),
		mustBuild(t, `type Query { a: String b: String }`))) != 0 {
		t.Error("adding a field was reported as breaking")
	}
}

// The coordinate a change names has to be one that can be resolved, or it is
// not much use to a tool reporting where the problem is.
func TestFindSchemaChanges_CoordinatesResolve(t *testing.T) {
	after := mustBuild(t, `
		enum Colour { RED GREEN }
		input F { a: String b: Int }
		directive @d(a: Int) on FIELD
		type Query { f(a: Int, b: Int): String x(y: F): Colour }
	`)
	changes := utilities.FindSchemaChanges(mustBuild(t, `
		enum Colour { RED }
		input F { a: String }
		directive @d on FIELD
		type Query { f(a: Int): String x(y: F): Colour }
	`), after)

	if len(changes) == 0 {
		t.Fatal("nothing changed; the test proves nothing")
	}
	for _, change := range changes {
		if change.Coordinate == "" {
			t.Errorf("%s named nothing: %s", change.Kind, change.Message)
			continue
		}
		resolved, err := utilities.ResolveSchemaCoordinate(after, change.Coordinate)
		if err != nil {
			t.Errorf("%s named %q, which is not a coordinate: %v", change.Kind, change.Coordinate, err)
			continue
		}
		if resolved == nil {
			t.Errorf("%s named %q, which the new schema does not have", change.Kind, change.Coordinate)
		}
	}
}

// The result is meant to be reported to a person and acted on by a tool, so
// both forms have to say something useful.
func TestChange_Describing(t *testing.T) {
	for _, tt := range []struct {
		severity utilities.Severity
		want     string
	}{
		{utilities.SafeChange, "safe"},
		{utilities.DangerousChange, "dangerous"},
		{utilities.BreakingChange, "breaking"},
		{utilities.Severity(99), "unknown"},
	} {
		if got := tt.severity.String(); got != tt.want {
			t.Errorf("Severity(%d) = %q, want %q", tt.severity, got, tt.want)
		}
	}

	changes := changesBetween(t, `type Query { a: String }`, `type Query { b: String }`)
	if len(changes) == 0 {
		t.Fatal("nothing changed")
	}
	for _, change := range changes {
		if change.String() != change.Message {
			t.Errorf("String() = %q, want the message %q", change.String(), change.Message)
		}
	}

	// The narrower views agree with the whole.
	dangerous := utilities.FindDangerousChanges(
		mustBuild(t, `enum C { A } type Query { c: C }`),
		mustBuild(t, `enum C { A B } type Query { c: C }`))
	if len(dangerous) != 1 || dangerous[0].Kind != utilities.ValueAddedToEnum {
		t.Errorf("FindDangerousChanges gave %s", describeChanges(dangerous))
	}
	if len(utilities.FindDangerousChanges(
		mustBuild(t, `type Query { a: String }`),
		mustBuild(t, `type Query { a: String }`))) != 0 {
		t.Error("an unchanged schema has dangerous changes")
	}
}

// A default that is an input object is unordered, so two spellings of the same
// one must not read as a change.
func TestFindSchemaChanges_ObjectDefaults(t *testing.T) {
	t.Run("the same object written another way round", func(t *testing.T) {
		changes := changesBetween(t,
			`input F { a: Int b: Int } type Query { f(x: F = { a: 1, b: 2 }): String }`,
			`input F { a: Int b: Int } type Query { f(x: F = { b: 2, a: 1 }): String }`)
		assertNoChange(t, changes, utilities.ArgDefaultValueChange)
	})

	t.Run("an object default that really differs", func(t *testing.T) {
		changes := changesBetween(t,
			`input F { a: Int b: Int } type Query { f(x: F = { a: 1 }): String }`,
			`input F { a: Int b: Int } type Query { f(x: F = { a: 2 }): String }`)
		assertChange(t, changes, utilities.ArgDefaultValueChange, "Query.f(x:)")
	})

	// A list is ordered, so its order is part of the value.
	t.Run("a list default reordered", func(t *testing.T) {
		changes := changesBetween(t,
			`type Query { f(x: [Int] = [1, 2]): String }`,
			`type Query { f(x: [Int] = [2, 1]): String }`)
		assertChange(t, changes, utilities.ArgDefaultValueChange, "Query.f(x:)")
	})
}

// Every kind of type has to be named when it turns into another, or the
// message says "Unknown".
func TestFindSchemaChanges_EveryKindChanged(t *testing.T) {
	for _, tt := range []struct{ before, after, want string }{
		{`scalar T type Query { a: String }`, `type T { a: String } type Query { a: String }`, "a Scalar type to an Object type"},
		{`type T { a: String } type Query { t: T }`, `interface T { a: String } type Query { t: T }`, "an Object type to an Interface type"},
		{`interface T { a: String } type Query { t: T }`, `enum T { A } type Query { t: T }`, "an Interface type to an Enum type"},
		{`enum T { A } type Query { t: T }`, `input T { a: Int } type Query { a(t: T): String }`, "an Enum type to an Input type"},
		{`union T = Q type Q { a: String } type Query { t: T q: Q }`,
			`scalar T type Q { a: String } type Query { t: T q: Q }`, "a Union type to a Scalar type"},
	} {
		changes := changesBetween(t, tt.before, tt.after)
		change := assertChange(t, changes, utilities.TypeChangedKind, "T")
		if !strings.Contains(change.Message, tt.want) {
			t.Errorf("message = %q, want it to say %q", change.Message, tt.want)
		}
	}
}
