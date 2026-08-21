package language

import (
	"strings"
	"unicode/utf8"
)

// maxLineLength is the width past which the printer breaks an argument list or
// a list value across several lines.
//
// The length is measured in Unicode code points. graphql-js measures UTF-16
// code units; the two agree for ASCII, and where they differ the only effect
// is on where a long line is broken.
const maxLineLength = 80

// Print renders an AST node as GraphQL source.
//
// The output is formatted rather than reproduced: whatever whitespace and
// comments the original document had are gone, since the AST does not record
// them. Parsing the result gives back an equal AST, so printing is stable
// under a second round trip.
//
// A nil node prints as the empty string.
func Print(node Node) string {
	if isAbsentNode(node) {
		return ""
	}
	switch n := node.(type) {
	case *Name:
		return n.Value
	case *Variable:
		return "$" + Print(n.Name)

	case *Document:
		return join(printEach(n.Definitions), "\n\n")

	case *OperationDefinition:
		return printOperationDefinition(n)
	case *VariableDefinition:
		return wrap("", Print(n.Description), "\n") +
			Print(n.Variable) + ": " + Print(n.Type) +
			wrap(" = ", Print(n.DefaultValue), "") +
			wrap(" ", join(printEach(n.Directives), " "), "")
	case *SelectionSet:
		return braceBlock(printEach(n.Selections))
	case *Field:
		return printField(n)
	case *Argument:
		return Print(n.Name) + ": " + Print(n.Value)
	case *FragmentArgument:
		return Print(n.Name) + ": " + Print(n.Value)

	case *FragmentSpread:
		return withArgs("..."+Print(n.Name), printEach(n.Arguments)) +
			wrap(" ", join(printEach(n.Directives), " "), "")
	case *InlineFragment:
		return join([]string{
			"...",
			wrap("on ", Print(n.TypeCondition), ""),
			join(printEach(n.Directives), " "),
			Print(n.SelectionSet),
		}, " ")
	case *FragmentDefinition:
		return printFragmentDefinition(n)

	case *IntValue:
		return n.Value
	case *FloatValue:
		return n.Value
	case *StringValue:
		if n.Block {
			return printBlockString(n.Value, false)
		}
		return PrintString(n.Value)
	case *BooleanValue:
		if n.Value {
			return "true"
		}
		return "false"
	case *NullValue:
		return "null"
	case *EnumValue:
		return n.Value
	case *ListValue:
		return printListValue(n)
	case *ObjectValue:
		return printObjectValue(n)
	case *ObjectField:
		return Print(n.Name) + ": " + Print(n.Value)

	case *Directive:
		return "@" + Print(n.Name) +
			wrap("(", join(printEach(n.Arguments), ", "), ")")

	case *NamedType:
		return Print(n.Name)
	case *ListType:
		return "[" + Print(n.Type) + "]"
	case *NonNullType:
		return Print(n.Type) + "!"

	case *SchemaDefinition:
		return wrap("", Print(n.Description), "\n") +
			join([]string{
				"schema",
				join(printEach(n.Directives), " "),
				braceBlock(printEach(n.OperationTypes)),
			}, " ")
	case *OperationTypeDefinition:
		return string(n.Operation) + ": " + Print(n.Type)
	case *ScalarTypeDefinition:
		return wrap("", Print(n.Description), "\n") +
			join([]string{"scalar", Print(n.Name), join(printEach(n.Directives), " ")}, " ")
	case *ObjectTypeDefinition:
		return wrap("", Print(n.Description), "\n") +
			printFielded("type", n.Name, n.Interfaces, n.Directives, printEach(n.Fields))
	case *InterfaceTypeDefinition:
		return wrap("", Print(n.Description), "\n") +
			printFielded("interface", n.Name, n.Interfaces, n.Directives, printEach(n.Fields))
	case *FieldDefinition:
		return wrap("", Print(n.Description), "\n") +
			Print(n.Name) + printArgumentDefs(printEach(n.Arguments)) +
			": " + Print(n.Type) +
			wrap(" ", join(printEach(n.Directives), " "), "")
	case *InputValueDefinition:
		return wrap("", Print(n.Description), "\n") +
			join([]string{
				Print(n.Name) + ": " + Print(n.Type),
				wrap("= ", Print(n.DefaultValue), ""),
				join(printEach(n.Directives), " "),
			}, " ")
	case *UnionTypeDefinition:
		return wrap("", Print(n.Description), "\n") +
			printUnion("union", n.Name, n.Directives, n.Types)
	case *EnumTypeDefinition:
		return wrap("", Print(n.Description), "\n") +
			join([]string{
				"enum", Print(n.Name),
				join(printEach(n.Directives), " "),
				braceBlock(printEach(n.Values)),
			}, " ")
	case *EnumValueDefinition:
		return wrap("", Print(n.Description), "\n") +
			join([]string{Print(n.Name), join(printEach(n.Directives), " ")}, " ")
	case *InputObjectTypeDefinition:
		return wrap("", Print(n.Description), "\n") +
			join([]string{
				"input", Print(n.Name),
				join(printEach(n.Directives), " "),
				braceBlock(printEach(n.Fields)),
			}, " ")
	case *DirectiveDefinition:
		return printDirectiveDefinition(n)

	case *SchemaExtension:
		return join([]string{
			"extend schema",
			join(printEach(n.Directives), " "),
			braceBlock(printEach(n.OperationTypes)),
		}, " ")
	case *ScalarTypeExtension:
		return join([]string{"extend scalar", Print(n.Name), join(printEach(n.Directives), " ")}, " ")
	case *ObjectTypeExtension:
		return printFielded("extend type", n.Name, n.Interfaces, n.Directives, printEach(n.Fields))
	case *InterfaceTypeExtension:
		return printFielded("extend interface", n.Name, n.Interfaces, n.Directives, printEach(n.Fields))
	case *UnionTypeExtension:
		return printUnion("extend union", n.Name, n.Directives, n.Types)
	case *EnumTypeExtension:
		return join([]string{
			"extend enum", Print(n.Name),
			join(printEach(n.Directives), " "),
			braceBlock(printEach(n.Values)),
		}, " ")
	case *InputObjectTypeExtension:
		return join([]string{
			"extend input", Print(n.Name),
			join(printEach(n.Directives), " "),
			braceBlock(printEach(n.Fields)),
		}, " ")
	case *DirectiveExtension:
		return join([]string{
			"extend directive @" + Print(n.Name),
			join(printEach(n.Directives), " "),
		}, " ")

	case *TypeCoordinate:
		return Print(n.Name)
	case *MemberCoordinate:
		return Print(n.Name) + wrap(".", Print(n.MemberName), "")
	case *ArgumentCoordinate:
		return Print(n.Name) + wrap(".", Print(n.FieldName), "") +
			wrap("(", Print(n.ArgumentName), ":)")
	case *DirectiveCoordinate:
		return "@" + Print(n.Name)
	case *DirectiveArgumentCoordinate:
		return "@" + Print(n.Name) + wrap("(", Print(n.ArgumentName), ":)")
	}
	return ""
}

// printOperationDefinition renders an operation, using the shorthand form when
// there is nothing but a selection set to print.
func printOperationDefinition(n *OperationDefinition) string {
	varDefs := printEach(n.VariableDefinitions)
	var varDefsText string
	if hasMultilineItem(varDefs) {
		varDefsText = wrap("(\n", join(varDefs, "\n"), "\n)")
	} else {
		varDefsText = wrap("(", join(varDefs, ", "), ")")
	}

	prefix := wrap("", Print(n.Description), "\n") +
		join([]string{
			string(n.Operation),
			join([]string{Print(n.Name), varDefsText}, ""),
			join(printEach(n.Directives), " "),
		}, " ")

	// An unnamed query with nothing else to say is written as just its
	// selection set.
	if prefix == string(OperationQuery) {
		return Print(n.SelectionSet)
	}
	return prefix + " " + Print(n.SelectionSet)
}

// printField renders a field selection.
func printField(n *Field) string {
	prefix := join([]string{wrap("", Print(n.Alias), ": "), Print(n.Name)}, "")
	return join([]string{
		withArgs(prefix, printEach(n.Arguments)),
		wrap(" ", join(printEach(n.Directives), " "), ""),
		wrap(" ", Print(n.SelectionSet), ""),
	}, "")
}

// printFragmentDefinition renders a named fragment definition.
func printFragmentDefinition(n *FragmentDefinition) string {
	return wrap("", Print(n.Description), "\n") +
		"fragment " + Print(n.Name) +
		wrap("(", join(printEach(n.VariableDefinitions), ", "), ")") +
		" on " + Print(n.TypeCondition) + " " +
		wrap("", join(printEach(n.Directives), " "), " ") +
		Print(n.SelectionSet)
}

// printListValue renders a list, breaking it across lines when it grows long.
func printListValue(n *ListValue) string {
	values := printEach(n.Values)
	line := "[" + join(values, ", ") + "]"
	if utf8.RuneCountInString(line) > maxLineLength {
		return "[\n" + indentBlock(join(values, "\n")) + "\n]"
	}
	return line
}

// printObjectValue renders an object, breaking it across lines when it grows
// long.
func printObjectValue(n *ObjectValue) string {
	fields := printEach(n.Fields)
	line := "{ " + join(fields, ", ") + " }"
	if utf8.RuneCountInString(line) > maxLineLength {
		return braceBlock(fields)
	}
	return line
}

// printFielded renders a type definition or extension that has interfaces and
// a block of fields, which covers objects and interfaces in both forms.
func printFielded(keyword string, name *Name, interfaces []*NamedType, directives []*Directive, fields []string) string {
	return join([]string{
		keyword,
		Print(name),
		wrap("implements ", join(printEach(interfaces), " & "), ""),
		join(printEach(directives), " "),
		braceBlock(fields),
	}, " ")
}

// printUnion renders a union definition or extension.
func printUnion(keyword string, name *Name, directives []*Directive, types []*NamedType) string {
	return join([]string{
		keyword,
		Print(name),
		join(printEach(directives), " "),
		wrap("= ", join(printEach(types), " | "), ""),
	}, " ")
}

// printDirectiveDefinition renders a directive definition.
func printDirectiveDefinition(n *DirectiveDefinition) string {
	repeatable := ""
	if n.Repeatable {
		repeatable = " repeatable"
	}
	return wrap("", Print(n.Description), "\n") +
		"directive @" + Print(n.Name) +
		printArgumentDefs(printEach(n.Arguments)) +
		wrap(" ", join(printEach(n.Directives), " "), "") +
		repeatable +
		" on " + join(printEach(n.Locations), " | ")
}

// printArgumentDefs renders a definition's argument list, putting each
// argument on its own line if any of them is itself multi-line, which happens
// when an argument carries a description.
func printArgumentDefs(args []string) string {
	if hasMultilineItem(args) {
		return wrap("(\n", indentBlock(join(args, "\n")), "\n)")
	}
	return wrap("(", join(args, ", "), ")")
}

// withArgs appends an argument list to a prefix, breaking it across lines if
// the result would be too wide.
func withArgs(prefix string, args []string) string {
	line := prefix + wrap("(", join(args, ", "), ")")
	if utf8.RuneCountInString(line) > maxLineLength {
		return prefix + wrap("(\n", indentBlock(join(args, "\n")), "\n)")
	}
	return line
}

// printEach renders every node of a slice. The element type is a type
// parameter so that this accepts the concrete slices the AST declares, such as
// []*Directive, rather than requiring a conversion to []Node at each call.
func printEach[T Node](nodes []T) []string {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, Print(n))
	}
	return out
}

// join concatenates the items that are not empty, separated by sep. Empty
// items are dropped rather than producing runs of separators, which is what
// lets the callers above list optional parts unconditionally.
func join(items []string, sep string) string {
	var b strings.Builder
	first := true
	for _, item := range items {
		if item == "" {
			continue
		}
		if !first {
			b.WriteString(sep)
		}
		b.WriteString(item)
		first = false
	}
	return b.String()
}

// wrap surrounds a string with a prefix and a suffix, or returns nothing at
// all if the string is empty.
func wrap(start, s, end string) string {
	if s == "" {
		return ""
	}
	return start + s + end
}

// indentBlock indents every line of a string by two spaces.
func indentBlock(s string) string {
	if s == "" {
		return ""
	}
	return "  " + strings.ReplaceAll(s, "\n", "\n  ")
}

// braceBlock renders items one per line inside an indented brace block, or
// nothing at all when there are none.
func braceBlock(items []string) string {
	return wrap("{\n", indentBlock(join(items, "\n")), "\n}")
}

// hasMultilineItem reports whether any item spans more than one line.
func hasMultilineItem(items []string) bool {
	for _, item := range items {
		if strings.Contains(item, "\n") {
			return true
		}
	}
	return false
}
