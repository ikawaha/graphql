package utilities

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// Severity says how much a change asks of the clients already using a schema.
type Severity int

const (
	// SafeChange breaks nothing: a client written against the old schema keeps
	// working, and one written against the new schema has more to work with.
	SafeChange Severity = iota
	// DangerousChange breaks no query, but may still surprise a client: a new
	// enum member arrives in a response the client has a switch over, or a
	// default value changes under a query that relied on it.
	DangerousChange
	// BreakingChange stops some query that used to work.
	BreakingChange
)

// String names a severity.
func (s Severity) String() string {
	switch s {
	case SafeChange:
		return "safe"
	case DangerousChange:
		return "dangerous"
	case BreakingChange:
		return "breaking"
	default:
		return "unknown"
	}
}

// Change is one difference between two schemas.
type Change struct {
	// Severity says what the change asks of existing clients.
	Severity Severity
	// Kind names the sort of change, in the vocabulary graphql-js uses, so
	// that a tool can act on it without reading the message.
	Kind string
	// Coordinate names what changed, written as a schema coordinate where
	// there is one for it.
	Coordinate string
	// Message says what happened, for a person to read.
	Message string
}

// String renders a change for a report.
func (c Change) String() string { return c.Message }

// The kinds a change can be, in the vocabulary graphql-js uses.
const (
	TypeRemoved                 = "TYPE_REMOVED"
	TypeChangedKind             = "TYPE_CHANGED_KIND"
	TypeRemovedFromUnion        = "TYPE_REMOVED_FROM_UNION"
	ValueRemovedFromEnum        = "VALUE_REMOVED_FROM_ENUM"
	RequiredInputFieldAdded     = "REQUIRED_INPUT_FIELD_ADDED"
	ImplementedInterfaceRemoved = "IMPLEMENTED_INTERFACE_REMOVED"
	FieldRemoved                = "FIELD_REMOVED"
	FieldChangedKind            = "FIELD_CHANGED_KIND"
	RequiredArgAdded            = "REQUIRED_ARG_ADDED"
	ArgRemoved                  = "ARG_REMOVED"
	ArgChangedKind              = "ARG_CHANGED_KIND"
	DirectiveRemoved            = "DIRECTIVE_REMOVED"
	DirectiveArgRemoved         = "DIRECTIVE_ARG_REMOVED"
	RequiredDirectiveArgAdded   = "REQUIRED_DIRECTIVE_ARG_ADDED"
	DirectiveRepeatableRemoved  = "DIRECTIVE_REPEATABLE_REMOVED"
	DirectiveLocationRemoved    = "DIRECTIVE_LOCATION_REMOVED"

	ValueAddedToEnum             = "VALUE_ADDED_TO_ENUM"
	TypeAddedToUnion             = "TYPE_ADDED_TO_UNION"
	OptionalInputFieldAdded      = "OPTIONAL_INPUT_FIELD_ADDED"
	OptionalArgAdded             = "OPTIONAL_ARG_ADDED"
	ImplementedInterfaceAdded    = "IMPLEMENTED_INTERFACE_ADDED"
	ArgDefaultValueChange        = "ARG_DEFAULT_VALUE_CHANGE"
	InputFieldDefaultValueChange = "INPUT_FIELD_DEFAULT_VALUE_CHANGE"

	TypeAdded                   = "TYPE_ADDED"
	FieldAdded                  = "FIELD_ADDED"
	DirectiveAdded              = "DIRECTIVE_ADDED"
	DescriptionChanged          = "DESCRIPTION_CHANGED"
	DirectiveRepeatableAdded    = "DIRECTIVE_REPEATABLE_ADDED"
	DirectiveLocationAdded      = "DIRECTIVE_LOCATION_ADDED"
	OptionalDirectiveArgAdded   = "OPTIONAL_DIRECTIVE_ARG_ADDED"
	FieldChangedKindSafe        = "FIELD_CHANGED_KIND_SAFE"
	ArgChangedKindSafe          = "ARG_CHANGED_KIND_SAFE"
	ArgDefaultValueAdded        = "ARG_DEFAULT_VALUE_ADDED"
	InputFieldDefaultValueAdded = "INPUT_FIELD_DEFAULT_VALUE_ADDED"
)

// FindSchemaChanges reports every difference between two schemas, in the order
// the new schema declares things.
//
// This is what a server runs before deploying a schema and what a registry
// runs on a proposed one: it says which changes will stop a query that works
// today, which may surprise a client without stopping it, and which are simply
// additions.
//
// Only what a client can observe is compared. Descriptions, the order things
// are written in, and anything a resolver does are not differences a query
// would notice.
func FindSchemaChanges(before, after *schema.Schema) []Change {
	var changes []Change
	changes = append(changes, findTypeChanges(before, after)...)
	changes = append(changes, findDirectiveChanges(before, after)...)
	return changes
}

// FindBreakingChanges reports only the changes that stop a query which works
// against the old schema.
func FindBreakingChanges(before, after *schema.Schema) []Change {
	return withSeverity(FindSchemaChanges(before, after), BreakingChange)
}

// FindDangerousChanges reports only the changes that break no query but may
// still surprise a client.
func FindDangerousChanges(before, after *schema.Schema) []Change {
	return withSeverity(FindSchemaChanges(before, after), DangerousChange)
}

// withSeverity keeps the changes of one severity.
func withSeverity(changes []Change, severity Severity) []Change {
	var out []Change
	for _, change := range changes {
		if change.Severity == severity {
			out = append(out, change)
		}
	}
	return out
}

// findTypeChanges compares the types of two schemas.
func findTypeChanges(before, after *schema.Schema) []Change {
	var changes []Change

	// Every type is looked at three times over — gone, new, still here — so
	// that what a reader sees is grouped the same way whatever order the
	// schema happens to hold its types in.
	for _, was := range before.Types() {
		if was == nil || schema.IsIntrospectionType(was) || after.Type(was.Name()) != nil {
			continue
		}
		// A built-in scalar is in the schema only while something names it, so
		// one dropping out is worth saying plainly: nothing was deleted, the
		// last reference to it went.
		if schema.IsSpecifiedScalarType(was) {
			changes = append(changes, Change{
				Severity:   BreakingChange,
				Kind:       TypeRemoved,
				Coordinate: was.Name(),
				Message: fmt.Sprintf(
					"Standard scalar %s was removed because it is not referenced anymore.",
					was.Name()),
			})
			continue
		}
		changes = append(changes, Change{
			Severity:   BreakingChange,
			Kind:       TypeRemoved,
			Coordinate: was.Name(),
			Message:    fmt.Sprintf("%s was removed.", was.Name()),
		})
	}

	for _, is := range after.Types() {
		if is == nil || !isRebuildable(is) || before.Type(is.Name()) != nil {
			continue
		}
		changes = append(changes, Change{
			Severity:   SafeChange,
			Kind:       TypeAdded,
			Coordinate: is.Name(),
			Message:    fmt.Sprintf("%s was added.", is.Name()),
		})
	}

	for _, was := range before.Types() {
		if was == nil || !isRebuildable(was) {
			continue
		}
		is := after.Type(was.Name())
		if is == nil {
			continue
		}
		if was.Description() != is.Description() {
			changes = append(changes, Change{
				Severity:   SafeChange,
				Kind:       DescriptionChanged,
				Coordinate: was.Name(),
				Message: fmt.Sprintf("Description of %s has changed to %q.",
					was.Name(), is.Description()),
			})
		}
		if typeKindName(was) != typeKindName(is) {
			changes = append(changes, Change{
				Severity:   BreakingChange,
				Kind:       TypeChangedKind,
				Coordinate: was.Name(),
				Message: fmt.Sprintf("%s changed from %s to %s.",
					was.Name(), typeKindName(was), typeKindName(is)),
			})
			continue
		}
		changes = append(changes, findChangesWithinType(was, is)...)
	}
	return changes
}

// findChangesWithinType compares two types of the same name and kind.
//
// The caller has already reported a type that changed kind, so the two are the
// same kind here; the second is still matched rather than asserted, so that a
// caller reaching this another way gets no changes instead of a panic.
func findChangesWithinType(was, is schema.NamedType) []Change {
	switch old := was.(type) {
	case *schema.EnumType:
		if new, sameKind := is.(*schema.EnumType); sameKind {
			return findEnumChanges(old, new)
		}
	case *schema.UnionType:
		if new, sameKind := is.(*schema.UnionType); sameKind {
			return findUnionChanges(old, new)
		}
	case *schema.InputObjectType:
		if new, sameKind := is.(*schema.InputObjectType); sameKind {
			return findInputObjectChanges(old, new)
		}
	case *schema.ObjectType:
		if new, sameKind := is.(*schema.ObjectType); sameKind {
			changes := findInterfaceChanges(old, old.Interfaces(), new.Interfaces())
			return append(changes, findFieldChanges(old, old.Fields(), new.Fields())...)
		}
	case *schema.InterfaceType:
		if new, sameKind := is.(*schema.InterfaceType); sameKind {
			changes := findInterfaceChanges(old, old.Interfaces(), new.Interfaces())
			return append(changes, findFieldChanges(old, old.Fields(), new.Fields())...)
		}
	}
	// A scalar has nothing a query can observe beyond its name.
	return nil
}

// findEnumChanges compares the members of two enums.
//
// Removing a member stops any query that sends it as an argument. Adding one
// is not breaking, but a client with a switch over the members will meet a
// value it has never seen, so it is worth saying.
func findEnumChanges(was, is *schema.EnumType) []Change {
	var changes []Change
	for _, member := range was.Values() {
		if member == nil || is.Value(member.Name()) != nil {
			continue
		}
		changes = append(changes, Change{
			Severity:   BreakingChange,
			Kind:       ValueRemovedFromEnum,
			Coordinate: was.Name() + "." + member.Name(),
			Message:    fmt.Sprintf("Enum value %s.%s was removed.", was.Name(), member.Name()),
		})
	}
	for _, member := range is.Values() {
		if member == nil || was.Value(member.Name()) != nil {
			continue
		}
		changes = append(changes, Change{
			Severity:   DangerousChange,
			Kind:       ValueAddedToEnum,
			Coordinate: is.Name() + "." + member.Name(),
			Message:    fmt.Sprintf("Enum value %s.%s was added.", is.Name(), member.Name()),
		})
	}
	for _, member := range was.Values() {
		now := is.Value(member.Name())
		if member == nil || now == nil || member.Description() == now.Description() {
			continue
		}
		changes = append(changes, Change{
			Severity:   SafeChange,
			Kind:       DescriptionChanged,
			Coordinate: was.Name() + "." + member.Name(),
			Message: fmt.Sprintf("Description of enum value %s.%s has changed to %q.",
				was.Name(), member.Name(), now.Description()),
		})
	}
	return changes
}

// findUnionChanges compares what two unions stand for.
func findUnionChanges(was, is *schema.UnionType) []Change {
	var changes []Change
	for _, member := range was.Types() {
		if !member.IsSet() || hasMember(is, member.Name()) {
			continue
		}
		changes = append(changes, Change{
			Severity:   BreakingChange,
			Kind:       TypeRemovedFromUnion,
			Coordinate: was.Name(),
			Message:    fmt.Sprintf("%s was removed from union type %s.", member.Name(), was.Name()),
		})
	}
	// A new member means a response may hold a type the client's fragments do
	// not cover, which is surprising rather than breaking.
	for _, member := range is.Types() {
		if !member.IsSet() || hasMember(was, member.Name()) {
			continue
		}
		changes = append(changes, Change{
			Severity:   DangerousChange,
			Kind:       TypeAddedToUnion,
			Coordinate: is.Name(),
			Message:    fmt.Sprintf("%s was added to union type %s.", member.Name(), is.Name()),
		})
	}
	return changes
}

// hasMember reports whether a union stands for a type of the given name.
func hasMember(union *schema.UnionType, name string) bool {
	for _, member := range union.Types() {
		if member.IsSet() && member.Name() == name {
			return true
		}
	}
	return false
}

// findInputObjectChanges compares the fields of two input objects.
func findInputObjectChanges(was, is *schema.InputObjectType) []Change {
	var changes []Change
	for _, field := range was.Fields() {
		if field == nil || is.Field(field.Name()) != nil {
			continue
		}
		at := was.Name() + "." + field.Name()
		changes = append(changes, Change{
			Severity:   BreakingChange,
			Kind:       FieldRemoved,
			Coordinate: at,
			Message:    fmt.Sprintf("Field %s was removed.", at),
		})
	}
	for _, field := range was.Fields() {
		if field == nil {
			continue
		}
		now := is.Field(field.Name())
		if now == nil {
			continue
		}
		at := was.Name() + "." + field.Name()
		before, hadDefault := defaultAsWritten(field.Default, field.Type)
		after, hasDefault := defaultAsWritten(now.Default, now.Type)
		// An input travels from the client, so its type may become more
		// forgiving but not less. A change to what fills the field when the
		// client leaves it out changes what a request means without changing
		// what it says.
		switch {
		case !inputChangeIsSafe(field.Type, now.Type):
			changes = append(changes, Change{
				Severity:   BreakingChange,
				Kind:       FieldChangedKind,
				Coordinate: at,
				Message: fmt.Sprintf("Field %s changed type from %s to %s.",
					at, field.Type.String(), now.Type.String()),
			})
		case hadDefault && !hasDefault:
			changes = append(changes, Change{
				Severity:   DangerousChange,
				Kind:       InputFieldDefaultValueChange,
				Coordinate: at,
				Message:    fmt.Sprintf("%s defaultValue was removed.", at),
			})
		case hadDefault && before != after:
			changes = append(changes, Change{
				Severity:   DangerousChange,
				Kind:       InputFieldDefaultValueChange,
				Coordinate: at,
				Message: fmt.Sprintf("%s has changed defaultValue from %s to %s.",
					at, before, after),
			})
		case !hadDefault && hasDefault:
			changes = append(changes, Change{
				Severity:   SafeChange,
				Kind:       InputFieldDefaultValueAdded,
				Coordinate: at,
				Message:    fmt.Sprintf("%s added a defaultValue %s.", at, after),
			})
		case field.Type.String() != now.Type.String():
			changes = append(changes, Change{
				Severity:   SafeChange,
				Kind:       FieldChangedKindSafe,
				Coordinate: at,
				Message: fmt.Sprintf("Field %s changed type from %s to %s.",
					at, field.Type.String(), now.Type.String()),
			})
		}
		if field.Description() != now.Description() {
			changes = append(changes, Change{
				Severity:   SafeChange,
				Kind:       DescriptionChanged,
				Coordinate: at,
				Message: fmt.Sprintf("Description of input-field %s has changed to %q.",
					at, now.Description()),
			})
		}
	}
	for _, field := range is.Fields() {
		if field == nil || was.Field(field.Name()) != nil {
			continue
		}
		at := is.Name() + "." + field.Name()
		// A new field the client must supply stops every query that does not.
		if schema.IsRequiredInputField(field) {
			changes = append(changes, Change{
				Severity:   BreakingChange,
				Kind:       RequiredInputFieldAdded,
				Coordinate: at,
				Message:    fmt.Sprintf("A required field %s.%s was added.", is.Name(), field.Name()),
			})
			continue
		}
		changes = append(changes, Change{
			Severity:   DangerousChange,
			Kind:       OptionalInputFieldAdded,
			Coordinate: at,
			Message:    fmt.Sprintf("An optional field %s.%s was added.", is.Name(), field.Name()),
		})
	}
	return changes
}

// findInterfaceChanges compares what two types implement.
func findInterfaceChanges(owner schema.NamedType, was, is []schema.Declared[*schema.InterfaceType]) []Change {
	var changes []Change
	for _, iface := range was {
		if !iface.IsSet() || implementsNamed(is, iface.Name()) {
			continue
		}
		changes = append(changes, Change{
			Severity:   BreakingChange,
			Kind:       ImplementedInterfaceRemoved,
			Coordinate: owner.Name(),
			Message: fmt.Sprintf("%s no longer implements interface %s.",
				owner.Name(), iface.Name()),
		})
	}
	// Newly implementing an interface means the type turns up in places a
	// client did not expect it, which its fragments may not cover.
	for _, iface := range is {
		if !iface.IsSet() || implementsNamed(was, iface.Name()) {
			continue
		}
		changes = append(changes, Change{
			Severity:   DangerousChange,
			Kind:       ImplementedInterfaceAdded,
			Coordinate: owner.Name(),
			Message: fmt.Sprintf("%s added to interfaces implemented by %s.",
				iface.Name(), owner.Name()),
		})
	}
	return changes
}

// implementsNamed reports whether an interface of the given name is in the
// list.
func implementsNamed(interfaces []schema.Declared[*schema.InterfaceType], name string) bool {
	for _, iface := range interfaces {
		if iface.IsSet() && iface.Name() == name {
			return true
		}
	}
	return false
}

// findFieldChanges compares the fields of two object or interface types.
func findFieldChanges(owner schema.NamedType, was, is []*schema.Field) []Change {
	byName := make(map[string]*schema.Field, len(is))
	for _, field := range is {
		if field != nil {
			byName[field.Name()] = field
		}
	}

	var changes []Change
	for _, field := range was {
		if field == nil {
			continue
		}
		at := owner.Name() + "." + field.Name()
		now := byName[field.Name()]
		if now == nil {
			changes = append(changes, Change{
				Severity:   BreakingChange,
				Kind:       FieldRemoved,
				Coordinate: at,
				Message:    fmt.Sprintf("Field %s was removed.", at),
			})
			continue
		}
		changes = append(changes, findArgumentChanges(at, field.Args, now.Args)...)
		// An output travels to the client, so its type may become more
		// specific but not less. Becoming more specific is still worth
		// naming, since a client reading the schema will see it.
		switch {
		case !outputChangeIsSafe(field.Type, now.Type):
			changes = append(changes, Change{
				Severity:   BreakingChange,
				Kind:       FieldChangedKind,
				Coordinate: at,
				Message: fmt.Sprintf("Field %s changed type from %s to %s.",
					at, field.Type.String(), now.Type.String()),
			})
		case field.Type.String() != now.Type.String():
			changes = append(changes, Change{
				Severity:   SafeChange,
				Kind:       FieldChangedKindSafe,
				Coordinate: at,
				Message: fmt.Sprintf("Field %s changed type from %s to %s.",
					at, field.Type.String(), now.Type.String()),
			})
		}
		if field.Description() != now.Description() {
			changes = append(changes, Change{
				Severity:   SafeChange,
				Kind:       DescriptionChanged,
				Coordinate: at,
				Message: fmt.Sprintf("Description of field %s has changed to %q.",
					at, now.Description()),
			})
		}
	}

	wasByName := make(map[string]bool, len(was))
	for _, field := range was {
		if field != nil {
			wasByName[field.Name()] = true
		}
	}
	for _, field := range is {
		if field == nil || wasByName[field.Name()] {
			continue
		}
		changes = append(changes, Change{
			Severity:   SafeChange,
			Kind:       FieldAdded,
			Coordinate: owner.Name() + "." + field.Name(),
			Message:    fmt.Sprintf("Field %s.%s was added.", owner.Name(), field.Name()),
		})
	}
	return changes
}

// findArgumentChanges compares the arguments of two fields or directives.
//
// The owner is written as it would be in a schema coordinate, so that the
// coordinates this produces are ones [ResolveSchemaCoordinate] can resolve.
func findArgumentChanges(owner string, was, is []*schema.Argument) []Change {
	byName := make(map[string]*schema.Argument, len(is))
	for _, arg := range is {
		if arg != nil {
			byName[arg.Name()] = arg
		}
	}

	var changes []Change
	for _, arg := range was {
		if arg == nil {
			continue
		}
		at := fmt.Sprintf("%s(%s:)", owner, arg.Name())
		now := byName[arg.Name()]
		if now == nil {
			changes = append(changes, Change{
				Severity:   BreakingChange,
				Kind:       ArgRemoved,
				Coordinate: at,
				Message:    fmt.Sprintf("Argument %s was removed.", at),
			})
			continue
		}
		changes = append(changes, findOneArgumentChange(at, "Argument "+at, arg, now)...)
	}

	wasByName := make(map[string]bool, len(was))
	for _, arg := range was {
		if arg != nil {
			wasByName[arg.Name()] = true
		}
	}
	for _, arg := range is {
		if arg == nil || wasByName[arg.Name()] {
			continue
		}
		at := fmt.Sprintf("%s(%s:)", owner, arg.Name())
		if schema.IsRequiredArgument(arg) {
			changes = append(changes, Change{
				Severity:   BreakingChange,
				Kind:       RequiredArgAdded,
				Coordinate: at,
				Message:    fmt.Sprintf("A required argument %s was added.", at),
			})
			continue
		}
		changes = append(changes, Change{
			Severity:   DangerousChange,
			Kind:       OptionalArgAdded,
			Coordinate: at,
			Message:    fmt.Sprintf("An optional argument %s was added.", at),
		})
	}
	return changes
}

// findOneArgumentChange compares an argument that is in both schemas.
//
// at is the argument's coordinate and named is how a message calls it, which
// differs between a field's argument and a directive's.
func findOneArgumentChange(at, named string, was, is *schema.Argument) []Change {
	var changes []Change
	before, hadDefault := defaultAsWritten(was.Default, was.Type)
	after, hasDefault := defaultAsWritten(is.Default, is.Type)

	switch {
	case !inputChangeIsSafe(was.Type, is.Type):
		changes = append(changes, Change{
			Severity:   BreakingChange,
			Kind:       ArgChangedKind,
			Coordinate: at,
			Message: fmt.Sprintf("%s has changed type from %s to %s.",
				named, was.Type.String(), is.Type.String()),
		})
	case hadDefault && !hasDefault:
		// A query that left the argument out will now be answered
		// differently, which is a change in what it means without a change in
		// what it says.
		changes = append(changes, Change{
			Severity:   DangerousChange,
			Kind:       ArgDefaultValueChange,
			Coordinate: at,
			Message:    fmt.Sprintf("%s defaultValue was removed.", at),
		})
	case hadDefault && before != after:
		changes = append(changes, Change{
			Severity:   DangerousChange,
			Kind:       ArgDefaultValueChange,
			Coordinate: at,
			Message: fmt.Sprintf("%s has changed defaultValue from %s to %s.",
				at, before, after),
		})
	case !hadDefault && hasDefault:
		changes = append(changes, Change{
			Severity:   SafeChange,
			Kind:       ArgDefaultValueAdded,
			Coordinate: at,
			Message:    fmt.Sprintf("%s added a defaultValue %s.", at, after),
		})
	case was.Type.String() != is.Type.String():
		changes = append(changes, Change{
			Severity:   SafeChange,
			Kind:       ArgChangedKindSafe,
			Coordinate: at,
			Message: fmt.Sprintf("%s has changed type from %s to %s.",
				named, was.Type.String(), is.Type.String()),
		})
	}

	if was.Description() != is.Description() {
		changes = append(changes, Change{
			Severity:   SafeChange,
			Kind:       DescriptionChanged,
			Coordinate: at,
			Message: fmt.Sprintf("Description of argument %s has changed to %q.",
				at, is.Description()),
		})
	}
	return changes
}

// findDirectiveChanges compares the directives two schemas allow.
func findDirectiveChanges(before, after *schema.Schema) []Change {
	var changes []Change

	for _, was := range before.Directives() {
		if was != nil && after.Directive(was.Name()) == nil {
			changes = append(changes, Change{
				Severity:   BreakingChange,
				Kind:       DirectiveRemoved,
				Coordinate: "@" + was.Name(),
				Message:    fmt.Sprintf("Directive @%s was removed.", was.Name()),
			})
		}
	}
	for _, is := range after.Directives() {
		if is != nil && before.Directive(is.Name()) == nil {
			changes = append(changes, Change{
				Severity:   SafeChange,
				Kind:       DirectiveAdded,
				Coordinate: "@" + is.Name(),
				Message:    fmt.Sprintf("Directive @%s was added.", is.Name()),
			})
		}
	}

	for _, was := range before.Directives() {
		if was == nil {
			continue
		}
		is := after.Directive(was.Name())
		if is == nil {
			continue
		}
		at := "@" + was.Name()
		changes = append(changes, findDirectiveArgumentChanges(at, was.Args, is.Args)...)

		// A directive that may no longer be repeated stops any document that
		// repeats it; one that newly may is no trouble for anyone.
		switch {
		case was.IsRepeatable && !is.IsRepeatable:
			changes = append(changes, Change{
				Severity:   BreakingChange,
				Kind:       DirectiveRepeatableRemoved,
				Coordinate: at,
				Message:    fmt.Sprintf("Repeatable flag was removed from %s.", at),
			})
		case !was.IsRepeatable && is.IsRepeatable:
			changes = append(changes, Change{
				Severity:   SafeChange,
				Kind:       DirectiveRepeatableAdded,
				Coordinate: at,
				Message:    fmt.Sprintf("Repeatable flag was added to %s.", at),
			})
		}

		if was.Description() != is.Description() {
			changes = append(changes, Change{
				Severity:   SafeChange,
				Kind:       DescriptionChanged,
				Coordinate: at,
				Message: fmt.Sprintf("Description of %s has changed to %q.",
					at, is.Description()),
			})
		}

		for _, location := range was.Locations {
			if !allowsLocation(is, location) {
				changes = append(changes, Change{
					Severity:   BreakingChange,
					Kind:       DirectiveLocationRemoved,
					Coordinate: at,
					Message:    fmt.Sprintf("%s was removed from %s.", location, at),
				})
			}
		}
		for _, location := range is.Locations {
			if !allowsLocation(was, location) {
				changes = append(changes, Change{
					Severity:   SafeChange,
					Kind:       DirectiveLocationAdded,
					Coordinate: at,
					Message:    fmt.Sprintf("%s was added to %s.", location, at),
				})
			}
		}
	}
	return changes
}

// findDirectiveArgumentChanges compares the arguments of two directives.
//
// A directive's arguments change in the same ways a field's do, but the
// vocabulary for them is its own and a message names them differently.
func findDirectiveArgumentChanges(at string, was, is []*schema.Argument) []Change {
	byName := make(map[string]*schema.Argument, len(is))
	for _, arg := range is {
		if arg != nil {
			byName[arg.Name()] = arg
		}
	}
	wasByName := make(map[string]bool, len(was))
	for _, arg := range was {
		if arg != nil {
			wasByName[arg.Name()] = true
		}
	}

	var changes []Change
	for _, arg := range is {
		if arg == nil || wasByName[arg.Name()] {
			continue
		}
		where := fmt.Sprintf("%s(%s:)", at, arg.Name())
		if schema.IsRequiredArgument(arg) {
			changes = append(changes, Change{
				Severity:   BreakingChange,
				Kind:       RequiredDirectiveArgAdded,
				Coordinate: where,
				Message:    fmt.Sprintf("A required argument %s was added.", where),
			})
			continue
		}
		changes = append(changes, Change{
			Severity:   SafeChange,
			Kind:       OptionalDirectiveArgAdded,
			Coordinate: where,
			Message:    fmt.Sprintf("An optional argument %s was added.", where),
		})
	}
	for _, arg := range was {
		if arg == nil {
			continue
		}
		where := fmt.Sprintf("%s(%s:)", at, arg.Name())
		if byName[arg.Name()] == nil {
			changes = append(changes, Change{
				Severity:   BreakingChange,
				Kind:       DirectiveArgRemoved,
				Coordinate: where,
				Message:    fmt.Sprintf("Argument %s was removed.", where),
			})
		}
	}
	for _, arg := range was {
		now := byName[arg.Name()]
		if arg == nil || now == nil {
			continue
		}
		where := fmt.Sprintf("%s(%s:)", at, arg.Name())
		changes = append(changes, findOneArgumentChange(where, "Argument "+where, arg, now)...)
	}
	return changes
}

// allowsLocation reports whether a directive may be written at a location.
func allowsLocation(d *schema.Directive, location language.DirectiveLocation) bool {
	for _, allowed := range d.Locations {
		if allowed == location {
			return true
		}
	}
	return false
}

// outputChangeIsSafe reports whether a field's type may change this way
// without stopping a query.
//
// A value travelling to the client may become more specific: a nullable type
// may become non-null, since the client already copes with a value being
// there. It may not become less specific, and it may not change its name or
// its list structure, because the client's selections were written against
// what it was.
func outputChangeIsSafe(was, is schema.Type) bool {
	if list, wasList := was.(*schema.List); wasList {
		if nowList, isList := is.(*schema.List); isList {
			return outputChangeIsSafe(list.OfType, nowList.OfType)
		}
		// A list may gain a non-null wrapper around it.
		if nonNull, isNonNull := is.(*schema.NonNull); isNonNull {
			return outputChangeIsSafe(was, nonNull.OfType)
		}
		return false
	}
	if nonNull, wasNonNull := was.(*schema.NonNull); wasNonNull {
		nowNonNull, isNonNull := is.(*schema.NonNull)
		return isNonNull && outputChangeIsSafe(nonNull.OfType, nowNonNull.OfType)
	}
	if named, isNamed := is.(schema.NamedType); isNamed {
		wasNamed, ok := was.(schema.NamedType)
		return ok && wasNamed.Name() == named.Name()
	}
	if nonNull, isNonNull := is.(*schema.NonNull); isNonNull {
		return outputChangeIsSafe(was, nonNull.OfType)
	}
	return false
}

// inputChangeIsSafe reports whether an argument or input field's type may
// change this way without stopping a query.
//
// A value travelling from the client may become more forgiving: a non-null
// type may become nullable, since everything that was being sent still is. It
// may not become stricter.
func inputChangeIsSafe(was, is schema.Type) bool {
	if list, wasList := was.(*schema.List); wasList {
		nowList, isList := is.(*schema.List)
		return isList && inputChangeIsSafe(list.OfType, nowList.OfType)
	}
	if nonNull, wasNonNull := was.(*schema.NonNull); wasNonNull {
		if nowNonNull, isNonNull := is.(*schema.NonNull); isNonNull {
			return inputChangeIsSafe(nonNull.OfType, nowNonNull.OfType)
		}
		// Dropping the non-null asks less of the client than before.
		return inputChangeIsSafe(nonNull.OfType, is)
	}
	named, isNamed := is.(schema.NamedType)
	if !isNamed {
		return false
	}
	wasNamed, ok := was.(schema.NamedType)
	return ok && wasNamed.Name() == named.Name()
}

// defaultAsWritten renders a default value the way it would be written, so
// that two of them can be compared.
//
// A default supplied in code and the same one written in a schema are the same
// default, so both are rendered rather than compared as they are held.
func defaultAsWritten(def value.Maybe[schema.DefaultInput], t schema.Type) (string, bool) {
	if literal, ok := defaultLiteralOf(def, t); ok {
		return canonicalLiteral(literal), true
	}
	// A value with no literal form still has to be compared with something.
	// Printing it is not what a schema would say, but two schemas holding the
	// same unprintable default do agree, which is the question being asked.
	if input, has := def.Get(); has {
		return fmt.Sprint(input.Value), true
	}
	return "", false
}

// canonicalLiteral renders a literal in a settled form, so that two ways of
// writing the same value compare equal.
func canonicalLiteral(v language.Value) string {
	switch node := v.(type) {
	case *language.ObjectValue:
		// The fields of an input object are unordered, so they are put in
		// order before being compared.
		type named struct{ name, written string }
		parts := make([]named, 0, len(node.Fields))
		for _, field := range node.Fields {
			if field != nil && field.Name != nil {
				parts = append(parts, named{field.Name.Value, canonicalLiteral(field.Value)})
			}
		}
		// By name, as graphql-js's sortValueNode does: the field's own value
		// has no say in where the field goes.
		sort.SliceStable(parts, func(i, j int) bool {
			return schema.NaturalCompare(parts[i].name, parts[j].name) < 0
		})
		written := make([]string, len(parts))
		for i, part := range parts {
			written[i] = part.name + ": " + part.written
		}
		return "{" + strings.Join(written, ", ") + "}"
	case *language.ListValue:
		// A list is ordered, so its entries stay where they are; what is
		// inside each of them is still settled.
		parts := make([]string, len(node.Values))
		for i, entry := range node.Values {
			parts[i] = canonicalLiteral(entry)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return language.Print(v)
	}
}

// typeKindName names what kind of thing a type is, for a message about one
// changing into another.
func typeKindName(t schema.NamedType) string {
	switch t.(type) {
	case *schema.ScalarType:
		return "a Scalar type"
	case *schema.ObjectType:
		return "an Object type"
	case *schema.InterfaceType:
		return "an Interface type"
	case *schema.UnionType:
		return "a Union type"
	case *schema.EnumType:
		return "an Enum type"
	case *schema.InputObjectType:
		return "an Input type"
	default:
		return "an Unknown type"
	}
}
