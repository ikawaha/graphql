package validation

import (
	"sort"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// FieldsOnCorrectTypeRule reports a field selected from a type that does not
// have it.
//
// Where the field exists on some of the types an interface or union stands
// for, the mistake is usually a missing inline fragment rather than a typo, so
// that is suggested first; only when no type would help is the name itself
// treated as misspelt.
func FieldsOnCorrectTypeRule(ctx *Context) language.Visitor {
	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			field, isField := node.(*language.Field)
			if !isField || field.Name == nil {
				return language.VisitContinue
			}
			parent := ctx.ParentType()
			if parent == nil || ctx.FieldDef() != nil {
				return language.VisitContinue
			}

			name := field.Name.Value
			suggestion := ctx.DidYouMean("to use an inline fragment on",
				suggestedTypeNames(ctx.Schema(), parent, name))
			if suggestion == "" {
				suggestion = ctx.DidYouMean("", suggestedFieldNames(parent, name))
			}
			ctx.Reportf([]language.Node{field}, "Cannot query field %s on type %s.%s",
				quote(name), quote(parent.Name()), suggestion)
			return language.VisitContinue
		},
	}
}

// suggestedTypeNames returns the types a fragment could narrow to so that the
// field becomes selectable, most widely applicable first.
func suggestedTypeNames(s *schema.Schema, parent schema.CompositeType, fieldName string) []string {
	abstract, isAbstract := parent.(schema.AbstractType)
	if !isAbstract {
		return nil
	}

	// A type is worth suggesting once per possible type it would cover, so an
	// interface that several of them share is offered ahead of any one of them.
	var candidates []schema.NamedType
	usage := map[string]int{}
	seen := map[string]bool{}
	add := func(t schema.NamedType) {
		if !seen[t.Name()] {
			seen[t.Name()] = true
			candidates = append(candidates, t)
		}
		usage[t.Name()]++
	}

	for _, possible := range s.PossibleTypes(abstract) {
		if possible == nil || possible.Field(fieldName) == nil {
			continue
		}
		add(possible)
		for _, declared := range possible.Interfaces() {
			// A clause naming something that is not an interface suggests
			// nothing; the schema check is what reports it.
			if iface, isInterface := declared.Get(); isInterface && iface.Field(fieldName) != nil {
				add(iface)
			}
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if usage[a.Name()] != usage[b.Name()] {
			return usage[a.Name()] > usage[b.Name()]
		}
		// Between a type and an interface it implements, the interface comes
		// first: it is the more general way to write the same fragment.
		if ifaceA, ok := a.(*schema.InterfaceType); ok && s.IsSubType(ifaceA, b) {
			return true
		}
		if ifaceB, ok := b.(*schema.InterfaceType); ok && s.IsSubType(ifaceB, a) {
			return false
		}
		return a.Name() < b.Name()
	})

	names := make([]string, len(candidates))
	for i, t := range candidates {
		names[i] = t.Name()
	}
	return names
}

// suggestedFieldNames returns the fields of a type whose names are close to
// what was written.
func suggestedFieldNames(parent schema.CompositeType, fieldName string) []string {
	var options []string
	switch t := parent.(type) {
	case *schema.ObjectType:
		for _, f := range t.Fields() {
			if f != nil {
				options = append(options, f.Name())
			}
		}
	case *schema.InterfaceType:
		for _, f := range t.Fields() {
			if f != nil {
				options = append(options, f.Name())
			}
		}
	default:
		// A union has no fields of its own, so nothing here would help.
		return nil
	}
	return schema.SuggestionList(fieldName, options)
}
