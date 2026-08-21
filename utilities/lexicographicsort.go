package utilities

import (
	"sort"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// LexicographicSortSchema returns the same schema with everything in it put in
// alphabetical order: the types, their fields, the arguments of each field,
// the members of each enum, and so on.
//
// A schema's order is otherwise whatever its author wrote or its builder
// happened to produce, which makes two schemas hard to compare. Sorting both
// first means a diff of their printed forms shows what actually differs rather
// than where things moved to.
//
// The order is by Unicode code point, so it does not depend on a locale: the
// same schema sorts the same way everywhere.
//
// As with [ExtendSchema] and [MapSchema], the result is a new schema and the
// original is left alone.
func LexicographicSortSchema(s *schema.Schema) *schema.Schema {
	return MapSchema(s, SchemaMapper{
		Types: func(types []schema.NamedType) []schema.NamedType {
			return sortedBy(types, func(t schema.NamedType) string { return t.Name() })
		},
		Fields: func(_ schema.NamedType, fields []*schema.Field) []*schema.Field {
			return sortedBy(fields, func(f *schema.Field) string { return f.Name() })
		},
		Arguments: func(_ string, args []*schema.Argument) []*schema.Argument {
			return sortedBy(args, func(a *schema.Argument) string { return a.Name() })
		},
		InputFields: func(_ schema.NamedType, fields []*schema.InputField) []*schema.InputField {
			return sortedBy(fields, func(f *schema.InputField) string { return f.Name() })
		},
		EnumValues: func(_ schema.NamedType, values []*schema.EnumValue) []*schema.EnumValue {
			return sortedBy(values, func(v *schema.EnumValue) string { return v.Name() })
		},
		UnionMembers: func(_ schema.NamedType, members []schema.Declared[*schema.ObjectType]) []schema.Declared[*schema.ObjectType] {
			return sortedBy(members, func(m schema.Declared[*schema.ObjectType]) string { return m.Name() })
		},
		Interfaces: func(_ schema.NamedType, ifaces []schema.Declared[*schema.InterfaceType]) []schema.Declared[*schema.InterfaceType] {
			return sortedBy(ifaces, func(i schema.Declared[*schema.InterfaceType]) string { return i.Name() })
		},
		Directives: func(directives []*schema.Directive) []*schema.Directive {
			sorted := sortedBy(directives, func(d *schema.Directive) string { return d.Name() })
			for i, d := range sorted {
				// Where a directive may be written is part of what it says, so
				// that is put in order too. A built-in is left alone: it is
				// shared rather than rebuilt, and nothing prints it.
				if d != nil && !schema.IsSpecifiedDirective(d) {
					sorted[i] = withSortedLocations(d)
				}
			}
			return sorted
		},
	})
}

// withSortedLocations returns the same directive with the places it may be
// written put in order.
func withSortedLocations(d *schema.Directive) *schema.Directive {
	locations := make([]language.DirectiveLocation, len(d.Locations))
	copy(locations, d.Locations)
	sort.Slice(locations, func(i, j int) bool {
		return schema.NaturalCompare(string(locations[i]), string(locations[j])) < 0
	})
	return schema.NewDirective(schema.DirectiveConfig{
		Name:              d.Name(),
		Description:       d.DescribedAs(),
		Locations:         locations,
		Args:              d.Args,
		IsRepeatable:      d.IsRepeatable,
		DeprecationReason: d.DeprecationReason,
		ASTNode:           d.ASTNode,
		Extensions:        d.Extensions,
	})
}

// sortedBy returns a copy of a list in order of the name each entry gives.
//
// A copy, because the list belongs to the schema being sorted and that one is
// left alone. A nil entry keeps its place rather than being dropped, since
// leaving things out is not what sorting is for; the rebuilder discards it.
func sortedBy[T any](items []T, name func(T) string) []T {
	out := make([]T, len(items))
	copy(out, items)
	sort.SliceStable(out, func(i, j int) bool {
		return schema.NaturalCompare(nameOrEmpty(out[i], name), nameOrEmpty(out[j], name)) < 0
	})
	return out
}

// nameOrEmpty reads an entry's name, coping with there being no entry.
func nameOrEmpty[T any](item T, name func(T) string) string {
	if isMissing(item) {
		return ""
	}
	return name(item)
}
