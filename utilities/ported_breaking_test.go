package utilities_test

// Ported from graphql-js src/utilities/__tests__/findBreakingChanges-test.ts:
// the same comparison as the other file, read through the filter that keeps
// only what would break a client, or only what would surprise one.

import (
	"testing"

	"github.com/ikawaha/graphql/utilities"
)

// knownBreakingDivergences are the cases this implementation does not match,
// and why. Each is asserted to *still* diverge, so that closing one cannot go
// unnoticed.
var knownBreakingDivergences = map[string]string{}

func TestPortedFindBreakingChanges(t *testing.T) {
	for _, tt := range []struct {
		name          string
		severity      utilities.Severity
		before, after string
		want          []portedChange
	}{
		{
			name:     `should detect if a type was removed or not`,
			severity: utilities.BreakingChange,
			before: `type Type1
type Type2`,
			after: `type Type2
`,
			want: []portedChange{
				{kind: "TYPE_REMOVED", says: "Type1 was removed."},
			},
		},
		{
			name:     `should detect if a standard scalar was removed`,
			severity: utilities.BreakingChange,
			before: `type Query {
  foo: Float
}`,
			after: `type Query {
  foo: String
}`,
			want: []portedChange{
				{kind: "TYPE_REMOVED", says: "Standard scalar Float was removed because it is not referenced anymore."},
				{kind: "FIELD_CHANGED_KIND", says: "Field Query.foo changed type from Float to String."},
			},
		},
		{
			name:     `should detect if a type changed its type`,
			severity: utilities.BreakingChange,
			before: `scalar TypeWasScalarBecomesEnum
interface TypeWasInterfaceBecomesUnion
type TypeWasObjectBecomesInputObject`,
			after: `enum TypeWasScalarBecomesEnum
union TypeWasInterfaceBecomesUnion
input TypeWasObjectBecomesInputObject`,
			want: []portedChange{
				{kind: "TYPE_CHANGED_KIND", says: "TypeWasScalarBecomesEnum changed from a Scalar type to an Enum type."},
				{kind: "TYPE_CHANGED_KIND", says: "TypeWasInterfaceBecomesUnion changed from an Interface type to a Union type."},
				{kind: "TYPE_CHANGED_KIND", says: "TypeWasObjectBecomesInputObject changed from an Object type to an Input type."},
			},
		},
		{
			name:     `should detect if fields on input types changed kind or were removed`,
			severity: utilities.BreakingChange,
			before: `input InputType1 {
  field1: String
  field2: Boolean
  field3: [String]
  field4: String!
  field5: String
  field6: [Int]
  field7: [Int]!
  field8: Int
  field9: [Int]
  field10: [Int!]
  field11: [Int]
  field12: [[Int]]
  field13: Int!
  field14: [[Int]!]
  field15: [[Int]!]
}`,
			after: `input InputType1 {
  field1: Int
  field3: String
  field4: String
  field5: String!
  field6: [Int]!
  field7: [Int]
  field8: [Int]!
  field9: [Int!]
  field10: [Int]
  field11: [[Int]]
  field12: [Int]
  field13: [Int]!
  field14: [[Int]]
  field15: [[Int!]!]
}`,
			want: []portedChange{
				{kind: "FIELD_REMOVED", says: "Field InputType1.field2 was removed."},
				{kind: "FIELD_CHANGED_KIND", says: "Field InputType1.field1 changed type from String to Int."},
				{kind: "FIELD_CHANGED_KIND", says: "Field InputType1.field3 changed type from [String] to String."},
				{kind: "FIELD_CHANGED_KIND", says: "Field InputType1.field5 changed type from String to String!."},
				{kind: "FIELD_CHANGED_KIND", says: "Field InputType1.field6 changed type from [Int] to [Int]!."},
				{kind: "FIELD_CHANGED_KIND", says: "Field InputType1.field8 changed type from Int to [Int]!."},
				{kind: "FIELD_CHANGED_KIND", says: "Field InputType1.field9 changed type from [Int] to [Int!]."},
				{kind: "FIELD_CHANGED_KIND", says: "Field InputType1.field11 changed type from [Int] to [[Int]]."},
				{kind: "FIELD_CHANGED_KIND", says: "Field InputType1.field12 changed type from [[Int]] to [Int]."},
				{kind: "FIELD_CHANGED_KIND", says: "Field InputType1.field13 changed type from Int! to [Int]!."},
				{kind: "FIELD_CHANGED_KIND", says: "Field InputType1.field15 changed type from [[Int]!] to [[Int!]!]."},
			},
		},
		{
			name:     `should detect if a required field is added to an input type`,
			severity: utilities.BreakingChange,
			before: `input InputType1 {
  field1: String
}`,
			after: `input InputType1 {
  field1: String
  requiredField: Int!
  optionalField1: Boolean
  optionalField2: Boolean! = false
}`,
			want: []portedChange{
				{kind: "REQUIRED_INPUT_FIELD_ADDED", says: "A required field InputType1.requiredField was added."},
			},
		},
		{
			name:     `should detect if a type was removed from a union type`,
			severity: utilities.BreakingChange,
			before: `type Type1
type Type2
type Type3

union UnionType1 = Type1 | Type2`,
			after: `type Type1
type Type2
type Type3

union UnionType1 = Type1 | Type3`,
			want: []portedChange{
				{kind: "TYPE_REMOVED_FROM_UNION", says: "Type2 was removed from union type UnionType1."},
			},
		},
		{
			name:     `should detect if a value was removed from an enum type`,
			severity: utilities.BreakingChange,
			before: `enum EnumType1 {
  VALUE0
  VALUE1
  VALUE2
}`,
			after: `enum EnumType1 {
  VALUE0
  VALUE2
  VALUE3
}`,
			want: []portedChange{
				{kind: "VALUE_REMOVED_FROM_ENUM", says: "Enum value EnumType1.VALUE1 was removed."},
			},
		},
		{
			name:     `should detect if a field argument was removed`,
			severity: utilities.BreakingChange,
			before: `interface Interface1 {
  field1(arg1: Boolean, objectArg: String): String
}

type Type1 {
  field1(name: String): String
}`,
			after: `interface Interface1 {
  field1: String
}

type Type1 {
  field1: String
}`,
			want: []portedChange{
				{kind: "ARG_REMOVED", says: "Argument Interface1.field1(arg1:) was removed."},
				{kind: "ARG_REMOVED", says: "Argument Interface1.field1(objectArg:) was removed."},
				{kind: "ARG_REMOVED", says: "Argument Type1.field1(name:) was removed."},
			},
		},
		{
			name:     `should detect if a field argument has changed type`,
			severity: utilities.BreakingChange,
			before: `type Type1 {
  field1(
    arg1: String
    arg2: String
    arg3: [String]
    arg4: String
    arg5: String!
    arg6: String!
    arg7: [Int]!
    arg8: Int
    arg9: [Int]
    arg10: [Int!]
    arg11: [Int]
    arg12: [[Int]]
    arg13: Int!
    arg14: [[Int]!]
    arg15: [[Int]!]
  ): String
}`,
			after: `type Type1 {
  field1(
    arg1: Int
    arg2: [String]
    arg3: String
    arg4: String!
    arg5: Int
    arg6: Int!
    arg7: [Int]
    arg8: [Int]!
    arg9: [Int!]
    arg10: [Int]
    arg11: [[Int]]
    arg12: [Int]
    arg13: [Int]!
    arg14: [[Int]]
    arg15: [[Int!]!]
   ): String
}`,
			want: []portedChange{
				{kind: "ARG_CHANGED_KIND", says: "Argument Type1.field1(arg1:) has changed type from String to Int."},
				{kind: "ARG_CHANGED_KIND", says: "Argument Type1.field1(arg2:) has changed type from String to [String]."},
				{kind: "ARG_CHANGED_KIND", says: "Argument Type1.field1(arg3:) has changed type from [String] to String."},
				{kind: "ARG_CHANGED_KIND", says: "Argument Type1.field1(arg4:) has changed type from String to String!."},
				{kind: "ARG_CHANGED_KIND", says: "Argument Type1.field1(arg5:) has changed type from String! to Int."},
				{kind: "ARG_CHANGED_KIND", says: "Argument Type1.field1(arg6:) has changed type from String! to Int!."},
				{kind: "ARG_CHANGED_KIND", says: "Argument Type1.field1(arg8:) has changed type from Int to [Int]!."},
				{kind: "ARG_CHANGED_KIND", says: "Argument Type1.field1(arg9:) has changed type from [Int] to [Int!]."},
				{kind: "ARG_CHANGED_KIND", says: "Argument Type1.field1(arg11:) has changed type from [Int] to [[Int]]."},
				{kind: "ARG_CHANGED_KIND", says: "Argument Type1.field1(arg12:) has changed type from [[Int]] to [Int]."},
				{kind: "ARG_CHANGED_KIND", says: "Argument Type1.field1(arg13:) has changed type from Int! to [Int]!."},
				{kind: "ARG_CHANGED_KIND", says: "Argument Type1.field1(arg15:) has changed type from [[Int]!] to [[Int!]!]."},
			},
		},
		{
			name:     `should detect if a required field argument was added`,
			severity: utilities.BreakingChange,
			before: `type Type1 {
  field1(arg1: String): String
}`,
			after: `type Type1 {
  field1(
    arg1: String,
    newRequiredArg: String!
    newOptionalArg1: Int
    newOptionalArg2: Int! = 0
  ): String
}`,
			want: []portedChange{
				{kind: "REQUIRED_ARG_ADDED", says: "A required argument Type1.field1(newRequiredArg:) was added."},
			},
		},
		{
			name:     `should not flag args with the same type signature as breaking`,
			severity: utilities.BreakingChange,
			before: `input InputType1 {
  field1: String
}

type Type1 {
  field1(arg1: Int!, arg2: InputType1): Int
}`,
			after: `input InputType1 {
  field1: String
}

type Type1 {
  field1(arg1: Int!, arg2: InputType1): Int
}`,
			want: []portedChange{},
		},
		{
			name:     `should consider args that move away from NonNull as non-breaking`,
			severity: utilities.BreakingChange,
			before: `type Type1 {
  field1(name: String!): String
}`,
			after: `type Type1 {
  field1(name: String): String
}`,
			want: []portedChange{},
		},
		{
			name:     `should detect interfaces removed from types`,
			severity: utilities.BreakingChange,
			before: `interface Interface1

type Type1 implements Interface1`,
			after: `interface Interface1

type Type1`,
			want: []portedChange{
				{kind: "IMPLEMENTED_INTERFACE_REMOVED", says: "Type1 no longer implements interface Interface1."},
			},
		},
		{
			name:     `should detect interfaces removed from interfaces`,
			severity: utilities.BreakingChange,
			before: `interface Interface1

interface Interface2 implements Interface1`,
			after: `interface Interface1

interface Interface2`,
			want: []portedChange{
				{kind: "IMPLEMENTED_INTERFACE_REMOVED", says: "Interface2 no longer implements interface Interface1."},
			},
		},
		{
			name:     `should ignore changes in order of interfaces`,
			severity: utilities.BreakingChange,
			before: `interface FirstInterface
interface SecondInterface

type Type1 implements FirstInterface & SecondInterface`,
			after: `interface FirstInterface
interface SecondInterface

type Type1 implements SecondInterface & FirstInterface`,
			want: []portedChange{},
		},
		{
			name:     `should detect all breaking changes`,
			severity: utilities.BreakingChange,
			before: `directive @DirectiveThatIsRemoved on FIELD_DEFINITION

directive @DirectiveThatRemovesArg(arg1: String) on FIELD_DEFINITION

directive @NonNullDirectiveAdded on FIELD_DEFINITION

directive @DirectiveThatWasRepeatable repeatable on FIELD_DEFINITION

directive @DirectiveName on FIELD_DEFINITION | QUERY

type ArgThatChanges {
  field1(id: Float): String
}

enum EnumTypeThatLosesAValue {
  VALUE0
  VALUE1
  VALUE2
}

interface Interface1
type TypeThatLooseInterface1 implements Interface1

type TypeInUnion1
type TypeInUnion2
union UnionTypeThatLosesAType = TypeInUnion1 | TypeInUnion2

type TypeThatChangesType

type TypeThatGetsRemoved

interface TypeThatHasBreakingFieldChanges {
  field1: String
  field2: String
}`,
			after: `directive @DirectiveThatRemovesArg on FIELD_DEFINITION

directive @NonNullDirectiveAdded(arg1: Boolean!) on FIELD_DEFINITION

directive @DirectiveThatWasRepeatable on FIELD_DEFINITION

directive @DirectiveName on FIELD_DEFINITION

type ArgThatChanges {
  field1(id: String): String
}

enum EnumTypeThatLosesAValue {
  VALUE1
  VALUE2
}

interface Interface1
type TypeThatLooseInterface1

type TypeInUnion1
type TypeInUnion2
union UnionTypeThatLosesAType = TypeInUnion1

interface TypeThatChangesType

interface TypeThatHasBreakingFieldChanges {
  field2: Boolean
}`,
			want: []portedChange{
				{kind: "TYPE_REMOVED", says: "Standard scalar Float was removed because it is not referenced anymore."},
				{kind: "TYPE_REMOVED", says: "TypeThatGetsRemoved was removed."},
				{kind: "ARG_CHANGED_KIND", says: "Argument ArgThatChanges.field1(id:) has changed type from Float to String."},
				{kind: "VALUE_REMOVED_FROM_ENUM", says: "Enum value EnumTypeThatLosesAValue.VALUE0 was removed."},
				{kind: "IMPLEMENTED_INTERFACE_REMOVED", says: "TypeThatLooseInterface1 no longer implements interface Interface1."},
				{kind: "TYPE_REMOVED_FROM_UNION", says: "TypeInUnion2 was removed from union type UnionTypeThatLosesAType."},
				{kind: "TYPE_CHANGED_KIND", says: "TypeThatChangesType changed from an Object type to an Interface type."},
				{kind: "FIELD_REMOVED", says: "Field TypeThatHasBreakingFieldChanges.field1 was removed."},
				{kind: "FIELD_CHANGED_KIND", says: "Field TypeThatHasBreakingFieldChanges.field2 changed type from String to Boolean."},
				{kind: "DIRECTIVE_REMOVED", says: "Directive @DirectiveThatIsRemoved was removed."},
				{kind: "DIRECTIVE_ARG_REMOVED", says: "Argument @DirectiveThatRemovesArg(arg1:) was removed."},
				{kind: "REQUIRED_DIRECTIVE_ARG_ADDED", says: "A required argument @NonNullDirectiveAdded(arg1:) was added."},
				{kind: "DIRECTIVE_REPEATABLE_REMOVED", says: "Repeatable flag was removed from @DirectiveThatWasRepeatable."},
				{kind: "DIRECTIVE_LOCATION_REMOVED", says: "QUERY was removed from @DirectiveName."},
			},
		},
		{
			name:     `should detect if a directive was explicitly removed`,
			severity: utilities.BreakingChange,
			before: `directive @DirectiveThatIsRemoved on FIELD_DEFINITION
directive @DirectiveThatStays on FIELD_DEFINITION`,
			after: `directive @DirectiveThatStays on FIELD_DEFINITION
`,
			want: []portedChange{
				{kind: "DIRECTIVE_REMOVED", says: "Directive @DirectiveThatIsRemoved was removed."},
			},
		},
		{
			name:     `should detect if a directive argument was removed`,
			severity: utilities.BreakingChange,
			before: `directive @DirectiveWithArg(arg1: String) on FIELD_DEFINITION
`,
			after: `directive @DirectiveWithArg on FIELD_DEFINITION
`,
			want: []portedChange{
				{kind: "DIRECTIVE_ARG_REMOVED", says: "Argument @DirectiveWithArg(arg1:) was removed."},
			},
		},
		{
			name:     `should detect if an optional directive argument was added`,
			severity: utilities.BreakingChange,
			before: `directive @DirectiveName on FIELD_DEFINITION
`,
			after: `directive @DirectiveName(
  newRequiredArg: String!
  newOptionalArg1: Int
  newOptionalArg2: Int! = 0
) on FIELD_DEFINITION`,
			want: []portedChange{
				{kind: "REQUIRED_DIRECTIVE_ARG_ADDED", says: "A required argument @DirectiveName(newRequiredArg:) was added."},
			},
		},
		{
			name:     `should detect removal of repeatable flag`,
			severity: utilities.BreakingChange,
			before: `directive @DirectiveName repeatable on OBJECT
`,
			after: `directive @DirectiveName on OBJECT
`,
			want: []portedChange{
				{kind: "DIRECTIVE_REPEATABLE_REMOVED", says: "Repeatable flag was removed from @DirectiveName."},
			},
		},
		{
			name:     `should detect locations removed from a directive`,
			severity: utilities.BreakingChange,
			before: `directive @DirectiveName on FIELD_DEFINITION | QUERY
`,
			after: `directive @DirectiveName on FIELD_DEFINITION
`,
			want: []portedChange{
				{kind: "DIRECTIVE_LOCATION_REMOVED", says: "QUERY was removed from @DirectiveName."},
			},
		},
		{
			name:     `should ignore changes in field order of defaultValue`,
			severity: utilities.DangerousChange,
			before: `input Input1 {
  a: String
  b: String
  c: String
}

type Type1 {
  field1(
    arg1: Input1 = { a: "a", b: "b", c: "c" }
  ): String
}`,
			after: `input Input1 {
  a: String
  b: String
  c: String
}

type Type1 {
  field1(
    arg1: Input1 = { c: "c", b: "b", a: "a" }
  ): String
}`,
			want: []portedChange{},
		},
		{
			name:     `should ignore changes in field definitions order`,
			severity: utilities.DangerousChange,
			before: `input Input1 {
  a: String
  b: String
  c: String
}

type Type1 {
  field1(
    arg1: Input1 = { a: "a", b: "b", c: "c" }
  ): String
}`,
			after: `input Input1 {
  c: String
  b: String
  a: String
}

type Type1 {
  field1(
    arg1: Input1 = { a: "a", b: "b", c: "c" }
  ): String
}`,
			want: []portedChange{},
		},
		{
			name:     `should detect if a value was added to an enum type`,
			severity: utilities.DangerousChange,
			before: `enum EnumType1 {
  VALUE0
  VALUE1
}`,
			after: `enum EnumType1 {
  VALUE0
  VALUE1
  VALUE2
}`,
			want: []portedChange{
				{kind: "VALUE_ADDED_TO_ENUM", says: "Enum value EnumType1.VALUE2 was added."},
			},
		},
		{
			name:     `should detect interfaces added to types`,
			severity: utilities.DangerousChange,
			before: `interface OldInterface
interface NewInterface

type Type1 implements OldInterface`,
			after: `interface OldInterface
interface NewInterface

type Type1 implements OldInterface & NewInterface`,
			want: []portedChange{
				{kind: "IMPLEMENTED_INTERFACE_ADDED", says: "NewInterface added to interfaces implemented by Type1."},
			},
		},
		{
			name:     `should detect interfaces added to interfaces`,
			severity: utilities.DangerousChange,
			before: `interface OldInterface
interface NewInterface

interface Interface1 implements OldInterface`,
			after: `interface OldInterface
interface NewInterface

interface Interface1 implements OldInterface & NewInterface`,
			want: []portedChange{
				{kind: "IMPLEMENTED_INTERFACE_ADDED", says: "NewInterface added to interfaces implemented by Interface1."},
			},
		},
		{
			name:     `should detect if a type was added to a union type`,
			severity: utilities.DangerousChange,
			before: `type Type1
type Type2

union UnionType1 = Type1`,
			after: `type Type1
type Type2

union UnionType1 = Type1 | Type2`,
			want: []portedChange{
				{kind: "TYPE_ADDED_TO_UNION", says: "Type2 was added to union type UnionType1."},
			},
		},
		{
			name:     `should detect if an optional field was added to an input`,
			severity: utilities.DangerousChange,
			before: `input InputType1 {
  field1: String
}`,
			after: `input InputType1 {
  field1: String
  field2: Int
}`,
			want: []portedChange{
				{kind: "OPTIONAL_INPUT_FIELD_ADDED", says: "An optional field InputType1.field2 was added."},
			},
		},
		{
			name:     `should find all dangerous changes`,
			severity: utilities.DangerousChange,
			before: `enum EnumType1 {
  VALUE0
  VALUE1
}

type Type1 {
  field1(argThatChangesDefaultValue: String = "test"): String
}

interface Interface1
type TypeThatGainsInterface1

type TypeInUnion1
union UnionTypeThatGainsAType = TypeInUnion1`,
			after: `enum EnumType1 {
  VALUE0
  VALUE1
  VALUE2
}

type Type1 {
  field1(argThatChangesDefaultValue: String = "Test"): String
}

interface Interface1
type TypeThatGainsInterface1 implements Interface1

type TypeInUnion1
type TypeInUnion2
union UnionTypeThatGainsAType = TypeInUnion1 | TypeInUnion2`,
			want: []portedChange{
				{kind: "VALUE_ADDED_TO_ENUM", says: "Enum value EnumType1.VALUE2 was added."},
				{kind: "ARG_DEFAULT_VALUE_CHANGE", says: "Type1.field1(argThatChangesDefaultValue:) has changed defaultValue from \"test\" to \"Test\"."},
				{kind: "IMPLEMENTED_INTERFACE_ADDED", says: "Interface1 added to interfaces implemented by TypeThatGainsInterface1."},
				{kind: "TYPE_ADDED_TO_UNION", says: "TypeInUnion2 was added to union type UnionTypeThatGainsAType."},
			},
		},
		{
			name:     `should detect if an optional field argument was added`,
			severity: utilities.DangerousChange,
			before: `type Type1 {
  field1(arg1: String): String
}`,
			after: `type Type1 {
  field1(arg1: String, arg2: String): String
}`,
			want: []portedChange{
				{kind: "OPTIONAL_ARG_ADDED", says: "An optional argument Type1.field1(arg2:) was added."},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before, after := mustBuild(t, tt.before), mustBuild(t, tt.after)
			var found []utilities.Change
			if tt.severity == utilities.BreakingChange {
				found = utilities.FindBreakingChanges(before, after)
			} else {
				found = utilities.FindDangerousChanges(before, after)
			}

			got := make([]portedChange, len(found))
			for i, change := range found {
				got[i] = portedChange{kind: change.Kind, says: change.Message}
			}
			same := len(got) == len(tt.want)
			if same {
				for i := range got {
					if got[i] != tt.want[i] {
						same = false
						break
					}
				}
			}
			if why, listed := knownBreakingDivergences[tt.name]; listed {
				if same {
					t.Errorf("this case now matches graphql-js; remove it from the known divergences (%s)", why)
				} else {
					t.Logf("known divergence: %s", why)
				}
				return
			}
			if !same {
				t.Errorf("found %v, want %v", got, tt.want)
			}
		})
	}
}
