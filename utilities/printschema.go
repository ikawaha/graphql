package utilities

import (
	"strings"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// PrintSchema renders a schema as SDL.
//
// What is written is the schema as it now stands, not the text it may have
// been read from: whitespace and the order definitions were written in are not
// kept. Reading the result back gives the same schema, and printing that again
// gives the same text, so the output is stable.
//
// The built-in scalars, the built-in directives and the introspection types
// are left out. Every schema has them, so writing them down would only add
// noise; [PrintIntrospectionSchema] renders those on their own.
func PrintSchema(s *schema.Schema) string {
	return printFiltered(s, isDefinedType, isDefinedDirective)
}

// PrintIntrospectionSchema renders only the introspection types, which is
// useful for seeing what a client can ask about a schema.
func PrintIntrospectionSchema(s *schema.Schema) string {
	return printFiltered(s, schema.IsIntrospectionType, func(*schema.Directive) bool { return false })
}

// PrintType renders one type as it would appear in a schema.
func PrintType(t schema.NamedType) string {
	switch typ := t.(type) {
	case *schema.ScalarType:
		return printScalarType(typ)
	case *schema.ObjectType:
		return printObjectType(typ)
	case *schema.InterfaceType:
		return printInterfaceType(typ)
	case *schema.UnionType:
		return printUnionType(typ)
	case *schema.EnumType:
		return printEnumType(typ)
	case *schema.InputObjectType:
		return printInputObjectType(typ)
	default:
		return ""
	}
}

// isDefinedType reports whether a type is one the schema's author wrote,
// rather than one every schema has.
func isDefinedType(t schema.Type) bool {
	return !schema.IsSpecifiedScalarType(t) && !schema.IsIntrospectionType(t)
}

// isDefinedDirective reports whether a directive is one the schema's author
// declared.
func isDefinedDirective(d *schema.Directive) bool {
	return !schema.IsSpecifiedDirective(d)
}

// printFiltered renders the parts of a schema that pass the given tests.
func printFiltered(
	s *schema.Schema,
	keepType func(schema.Type) bool,
	keepDirective func(*schema.Directive) bool,
) string {
	if s == nil {
		return ""
	}
	var parts []string
	if header := printSchemaDefinition(s); header != "" {
		parts = append(parts, header)
	}
	for _, d := range s.Directives() {
		if d != nil && keepDirective(d) {
			parts = append(parts, printDirectiveDefinition(d))
		}
	}
	for _, t := range s.Types() {
		if t != nil && keepType(t) {
			if text := PrintType(t); text != "" {
				parts = append(parts, text)
			}
		}
	}
	if len(parts) == 0 {
		return ""
	}
	// No trailing newline: what comes back is a string, and whether it ends a
	// file is the caller's to decide. graphql-js joins the same way.
	return strings.Join(parts, "\n\n")
}

// printSchemaDefinition renders the schema definition, or nothing when it
// would say only what the conventional names already say.
func printSchemaDefinition(s *schema.Schema) string {
	// What the schema named, whatever kind it is: a root a request cannot
	// enter through is still what the schema says, and printing it is what
	// lets the schema be read back as it was written.
	query := s.DeclaredRootType(language.OperationQuery)
	mutation := s.DeclaredRootType(language.OperationMutation)
	subscription := s.DeclaredRootType(language.OperationSubscription)
	if query == nil && mutation == nil && subscription == nil {
		return ""
	}
	if !s.DescribedAs().IsSet() && hasConventionalRoots(s) {
		return ""
	}

	var b strings.Builder
	b.WriteString(describe(s.DescribedAs(), "", true))
	b.WriteString("schema {\n")
	for _, root := range []struct {
		operation language.OperationType
		typ       schema.NamedType
	}{
		{language.OperationQuery, query},
		{language.OperationMutation, mutation},
		{language.OperationSubscription, subscription},
	} {
		if root.typ != nil {
			b.WriteString("  " + string(root.operation) + ": " + root.typ.Name() + "\n")
		}
	}
	b.WriteString("}")
	return b.String()
}

// hasConventionalRoots reports whether each root is the type of the name that
// would be assumed for it anyway.
func hasConventionalRoots(s *schema.Schema) bool {
	return sameRoot(s.DeclaredRootType(language.OperationQuery), s.Type("Query")) &&
		sameRoot(s.DeclaredRootType(language.OperationMutation), s.Type("Mutation")) &&
		sameRoot(s.DeclaredRootType(language.OperationSubscription), s.Type("Subscription"))
}

// sameRoot compares a root against the type of the conventional name, counting
// "neither exists" as agreement.
func sameRoot(root, named schema.NamedType) bool {
	return root == named
}

func printScalarType(t *schema.ScalarType) string {
	out := describe(t.DescribedAs(), "", true) + "scalar " + t.Name()
	if t.SpecifiedByURL != "" {
		out += " @specifiedBy(url: " + language.PrintString(t.SpecifiedByURL) + ")"
	}
	return out
}

func printObjectType(t *schema.ObjectType) string {
	return describe(t.DescribedAs(), "", true) +
		"type " + t.Name() +
		printImplements(t.Interfaces()) +
		printFieldBlock(t.Fields())
}

func printInterfaceType(t *schema.InterfaceType) string {
	return describe(t.DescribedAs(), "", true) +
		"interface " + t.Name() +
		printImplements(t.Interfaces()) +
		printFieldBlock(t.Fields())
}

func printUnionType(t *schema.UnionType) string {
	out := describe(t.DescribedAs(), "", true) + "union " + t.Name()
	members := t.Types()
	if len(members) == 0 {
		return out
	}
	names := make([]string, 0, len(members))
	for _, m := range members {
		if m.IsSet() {
			names = append(names, m.Name())
		}
	}
	return out + " = " + strings.Join(names, " | ")
}

func printEnumType(t *schema.EnumType) string {
	out := describe(t.DescribedAs(), "", true) + "enum " + t.Name()
	lines := make([]string, 0, len(t.Values()))
	for i, member := range t.Values() {
		if member == nil {
			continue
		}
		lines = append(lines, describe(member.DescribedAs(), "  ", i == 0)+
			"  "+member.Name()+printDeprecated(member.DeprecationReason))
	}
	return out + block(lines)
}

func printInputObjectType(t *schema.InputObjectType) string {
	out := describe(t.DescribedAs(), "", true) + "input " + t.Name()
	if t.IsOneOf {
		out += " @oneOf"
	}
	lines := make([]string, 0, len(t.Fields()))
	for i, f := range t.Fields() {
		if f == nil {
			continue
		}
		lines = append(lines, describe(f.DescribedAs(), "  ", i == 0)+
			"  "+printInputValue(f.Name(), f.Type, f.Default, f.DeprecationReason))
	}
	return out + block(lines)
}

// printImplements renders an implements clause, or nothing when there is none.
func printImplements(interfaces []schema.Declared[*schema.InterfaceType]) string {
	names := make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		if iface.IsSet() {
			names = append(names, iface.Name())
		}
	}
	if len(names) == 0 {
		return ""
	}
	return " implements " + strings.Join(names, " & ")
}

// printFieldBlock renders the braced list of a type's fields.
func printFieldBlock(fields []*schema.Field) string {
	lines := make([]string, 0, len(fields))
	for i, f := range fields {
		if f == nil {
			continue
		}
		lines = append(lines, describe(f.DescribedAs(), "  ", i == 0)+
			"  "+f.Name()+printArguments(f.Args, "  ")+": "+f.Type.String()+
			printDeprecated(f.DeprecationReason))
	}
	return block(lines)
}

// printArguments renders an argument list.
//
// The list goes on one line unless an argument carries a description, which
// needs a line of its own; then each argument gets one.
func printArguments(args []*schema.Argument, indent string) string {
	present := make([]*schema.Argument, 0, len(args))
	described := false
	for _, arg := range args {
		if arg == nil {
			continue
		}
		present = append(present, arg)
		if arg.DescribedAs().IsSet() {
			described = true
		}
	}
	if len(present) == 0 {
		return ""
	}

	if !described {
		parts := make([]string, len(present))
		for i, arg := range present {
			parts[i] = printInputValue(arg.Name(), arg.Type, arg.Default, arg.DeprecationReason)
		}
		return "(" + strings.Join(parts, ", ") + ")"
	}

	lines := make([]string, len(present))
	for i, arg := range present {
		lines[i] = describe(arg.DescribedAs(), "  "+indent, i == 0) +
			"  " + indent + printInputValue(arg.Name(), arg.Type, arg.Default, arg.DeprecationReason)
	}
	return "(\n" + strings.Join(lines, "\n") + "\n" + indent + ")"
}

// printInputValue renders an argument or an input object field.
func printInputValue(
	name string,
	t schema.Type,
	def value.Maybe[schema.DefaultInput],
	deprecationReason value.Maybe[string],
) string {
	out := name + ": " + t.String()
	if literal, ok := defaultLiteralOf(def, t); ok {
		out += " = " + language.Print(literal)
	}
	return out + printDeprecated(deprecationReason)
}

// defaultLiteralOf returns the literal a default should be written as.
//
// A default read from a document is written back exactly as it was, which is
// what keeps a schema's text stable across a round trip. One supplied in code
// is rendered from its value.
func defaultLiteralOf(def value.Maybe[schema.DefaultInput], t schema.Type) (language.Value, bool) {
	input, has := def.Get()
	if !has {
		return nil, false
	}
	if input.Literal != nil {
		return input.Literal, true
	}
	return schema.LiteralFromValue(input.Value, t)
}

// printDirectiveDefinition renders a directive definition.
func printDirectiveDefinition(d *schema.Directive) string {
	out := describe(d.DescribedAs(), "", true) + "directive @" + d.Name() + printArguments(d.Args, "")
	out += printDeprecated(d.DeprecationReason)
	if d.IsRepeatable {
		out += " repeatable"
	}
	names := make([]string, 0, len(d.Locations))
	for _, loc := range d.Locations {
		names = append(names, string(loc))
	}
	return out + " on " + strings.Join(names, " | ")
}

// printDeprecated renders a deprecation, leaving out the reason when it is the
// one that would be assumed anyway.
func printDeprecated(reason value.Maybe[string]) string {
	said, deprecated := reason.Get()
	switch {
	case !deprecated:
		return ""
	case said == schema.DefaultDeprecationReason:
		return " @deprecated"
	default:
		// An empty reason still deprecates, and is written out as it was
		// given rather than being read as no reason at all.
		return " @deprecated(reason: " + language.PrintString(said) + ")"
	}
}

// block renders a braced list of lines, or nothing when there are none.
func block(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return " {\n" + strings.Join(lines, "\n") + "\n}"
}

// describe renders a description above whatever it documents.
//
// A description that can be written as a block string is, because a multi-line
// one reads far better that way than as a single escaped line. firstInBlock
// says whether this is the first entry of a braced list; the ones after it get
// a blank line above, so that a run of documented fields does not read as one
// wall of text.
func describe(described value.Maybe[string], indent string, firstInBlock bool) string {
	description, written := described.Get()
	if !written {
		return ""
	}
	literal := language.Print(&language.StringValue{
		Value: description,
		Block: language.IsPrintableAsBlockString(description),
	})
	prefix := indent
	if indent != "" && !firstInBlock {
		prefix = "\n" + indent
	}
	return prefix + strings.ReplaceAll(literal, "\n", "\n"+indent) + "\n"
}
