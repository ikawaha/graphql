package validation

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ikawaha/graphql/internal/typeinfo"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// OverlappingFieldsCanBeMergedRule reports two selections that would have to
// share one place in the response but disagree about what belongs there.
//
// A response is keyed by alias, or by field name where there is none, so
// selecting `name` twice through different fragments means both have to
// produce the same thing. Where they name different fields, take different
// arguments, or return types that cannot be reconciled, there is no single
// answer to put under the key.
//
// Two selections that can never both apply are exempt: fields of different
// object types are reached only through a fragment on one or the other, and
// exactly one of those fragments matches any given object.
func OverlappingFieldsCanBeMergedRule(ctx *Context) language.Visitor {
	c := &overlapChecker{
		ctx:                ctx,
		cache:              map[*language.SelectionSet]map[string]*fieldsAndFragments{},
		comparedFragments:  pairSet{},
		comparedWithFields: pairSet{},
	}
	return language.Visitor{
		Enter: func(node language.Node, _ language.VisitContext) language.VisitAction {
			set, isSelectionSet := node.(*language.SelectionSet)
			if !isSelectionSet {
				return language.VisitContinue
			}
			for _, found := range c.conflictsWithin(ctx.ParentType(), set) {
				blamed := append(append([]language.Node{}, found.fields1...), found.fields2...)
				ctx.Reportf(blamed,
					"Fields %s conflict because %s. Use different aliases on the fields to fetch both if this was intentional.",
					quote(found.responseName), found.reason.String())
			}
			return language.VisitContinue
		},
	}
}

// overlapChecker holds what one run of the rule has worked out so far.
//
// The same selection set and the same pair of fragments are reached again and
// again through different routes, and comparing them afresh each time is what
// makes the naive form of this check cost more than the document is worth.
type overlapChecker struct {
	ctx *Context
	// cache holds what a selection set contributes, per scope: a fragment
	// spread twice with different arguments brings different values, so the
	// same selection set has more than one answer.
	cache map[*language.SelectionSet]map[string]*fieldsAndFragments
	// comparedFragments remembers which pairs of spreads have been compared,
	// and comparedWithFields which spreads have been compared against which
	// group of fields.
	comparedFragments  pairSet
	comparedWithFields pairSet
}

// scope is the renaming in force where a selection was found.
//
// A fragment that declares variables of its own has them renamed to something
// unique to the spread that supplied them, so that a fragment's `$x` is never
// mistaken for an operation's `$x`, and two spreads given the same arguments
// still agree. The zero scope is the document's own, where nothing is renamed.
type scope struct {
	// key identifies the scope: the fragment and the arguments it was given.
	key string
	// rename maps a fragment's own variable names to the unique ones.
	rename map[string]string
}

// selectedField is one selection of a field, together with the type it was
// selected from, the definition it resolved to, and the scope it was found in.
type selectedField struct {
	parent schema.CompositeType
	node   *language.Field
	def    *schema.Field
	scope  scope
}

// fragmentSpread is one spread of a fragment, and what it supplied.
//
// Two spreads of one fragment given different arguments bring different
// values, so what identifies a spread is the fragment together with its
// arguments rather than the fragment alone.
type fragmentSpread struct {
	name string
	// node is the spread as the document wrote it, which is where a conflict
	// between two spreads of one fragment is reported.
	node *language.FragmentSpread
	// key identifies this spread, for remembering what it has been compared
	// against. Two spreads of one fragment given the same arguments share it.
	key string
	// inner is the scope of everything the fragment brings. It is the zero
	// scope for a fragment that declares no variables of its own: nothing is
	// renamed, so what it brings is the same wherever it is spread, and the
	// same answer can be reused.
	inner scope
}

// fieldsAndFragments is what a selection set contributes: its fields, grouped
// by the key each takes in the response, and the fragments it spreads.
type fieldsAndFragments struct {
	// order keeps the response keys in the order they were written, so that
	// what is reported does not shift between runs.
	order     []string
	byKey     map[string][]selectedField
	fragments []fragmentSpread
	// id names this group, for remembering what it has been compared against.
	id string
}

// add records a field under the key it will take in the response.
func (f *fieldsAndFragments) add(key string, field selectedField) {
	if _, seen := f.byKey[key]; !seen {
		f.order = append(f.order, key)
	}
	f.byKey[key] = append(f.byKey[key], field)
}

// addFragment records a fragment spread, once however often the same one is
// written.
func (f *fieldsAndFragments) addFragment(spread fragmentSpread) {
	for _, known := range f.fragments {
		if known.key == spread.key {
			return
		}
	}
	f.fragments = append(f.fragments, spread)
}

// conflict is one disagreement about what belongs under a response key.
type conflict struct {
	responseName string
	reason       conflictReason
	fields1      []language.Node
	fields2      []language.Node
}

// conflictReason says why two selections disagree. A conflict found among
// subfields is reported at the level the reader wrote, so the reason carries
// the path down to where the disagreement actually is.
type conflictReason struct {
	text     string
	children []namedReason
}

// namedReason is a conflict under a response key, one level down.
type namedReason struct {
	responseName string
	reason       conflictReason
}

// String renders a reason, unfolding nested ones into the path they were
// found at.
func (r conflictReason) String() string {
	if len(r.children) == 0 {
		return r.text
	}
	parts := make([]string, len(r.children))
	for i, child := range r.children {
		parts[i] = "subfields " + quote(child.responseName) + " conflict because " + child.reason.String()
	}
	return strings.Join(parts, " and ")
}

// conflictsWithin finds every disagreement inside one selection set.
func (c *overlapChecker) conflictsWithin(parent schema.CompositeType, set *language.SelectionSet) []conflict {
	var found []conflict
	contents := c.fieldsAndFragmentsOf(parent, set, scope{})

	// Two selections written in the same set always both apply.
	c.collectWithin(&found, contents)

	// A field written here and one reached through a fragment also both apply.
	for i, spread := range contents.fragments {
		c.collectBetweenFieldsAndFragment(&found, false, contents, spread)
		// And two fragments spread side by side both apply.
		for _, other := range contents.fragments[i+1:] {
			c.collectBetweenFragments(&found, false, spread, other)
		}
	}
	return found
}

// collectWithin compares the selections written in one place against one
// another.
func (c *overlapChecker) collectWithin(found *[]conflict, contents *fieldsAndFragments) {
	for _, key := range contents.order {
		fields := contents.byKey[key]
		if len(fields) <= 1 {
			continue
		}
		for i := range fields {
			for j := i + 1; j < len(fields); j++ {
				if got, is := c.findConflict(false, key, fields[i], fields[j]); is {
					*found = append(*found, got)
				}
			}
		}
	}
}

// collectBetweenFieldsAndFragment compares selections written in one place
// against those a fragment brings, following the fragments it spreads in turn.
func (c *overlapChecker) collectBetweenFieldsAndFragment(
	found *[]conflict,
	mutuallyExclusive bool,
	contents *fieldsAndFragments,
	spread fragmentSpread,
) {
	brought := c.fragmentContents(spread)
	if brought == nil || brought == contents {
		return
	}
	// The same group and spread are reached by many routes through a document
	// that spreads fragments freely; comparing them once is enough, and it is
	// also what stops a cycle from spinning here.
	if c.comparedWithFields.has(contents.id, spread.key, mutuallyExclusive) {
		return
	}
	c.comparedWithFields.add(contents.id, spread.key, mutuallyExclusive)

	c.collectBetween(found, mutuallyExclusive, contents, brought)
	for _, nested := range brought.fragments {
		c.collectBetweenFieldsAndFragment(found, mutuallyExclusive, contents, nested)
	}
}

// collectBetweenFragments compares what two fragment spreads bring, and what
// the fragments they spread bring in turn.
func (c *overlapChecker) collectBetweenFragments(
	found *[]conflict,
	mutuallyExclusive bool,
	a, b fragmentSpread,
) {
	if a.key == b.key {
		return
	}
	// One fragment spread twice with different arguments brings different
	// values under the same keys. What is wrong is the pair of spreads, not
	// anything written inside the fragment, so it is said of them and the
	// fragment is not descended into.
	if a.name == b.name {
		if c.comparedFragments.has(a.key, b.key, false) {
			return
		}
		c.comparedFragments.add(a.key, b.key, false)
		// Reported here rather than collected: what is wrong is the pair of
		// spreads themselves, so it must not be folded into the "subfields …
		// conflict" chain of whatever they were written inside.
		c.ctx.Reportf([]language.Node{a.node, b.node},
			"Spreads %s conflict because %s and %s have different fragment arguments.",
			quote(a.name), spreadAsWritten(a.node), spreadAsWritten(b.node))
		return
	}
	if c.comparedFragments.has(a.key, b.key, mutuallyExclusive) {
		return
	}
	c.comparedFragments.add(a.key, b.key, mutuallyExclusive)

	contentsA, contentsB := c.fragmentContents(a), c.fragmentContents(b)
	if contentsA == nil || contentsB == nil {
		return
	}

	c.collectBetween(found, mutuallyExclusive, contentsA, contentsB)
	for _, nested := range contentsB.fragments {
		c.collectBetweenFragments(found, mutuallyExclusive, a, nested)
	}
	for _, nested := range contentsA.fragments {
		c.collectBetweenFragments(found, mutuallyExclusive, nested, b)
	}
}

// collectBetween compares two groups of selections key by key. Only keys they
// share can disagree, since only those land in the same place.
func (c *overlapChecker) collectBetween(
	found *[]conflict,
	mutuallyExclusive bool,
	a, b *fieldsAndFragments,
) {
	for _, key := range a.order {
		others, shared := b.byKey[key]
		if !shared {
			continue
		}
		for _, fieldA := range a.byKey[key] {
			for _, fieldB := range others {
				if got, is := c.findConflict(mutuallyExclusive, key, fieldA, fieldB); is {
					*found = append(*found, got)
				}
			}
		}
	}
}

// conflictsBetweenSubSelections compares what two fields select beneath them.
func (c *overlapChecker) conflictsBetweenSubSelections(
	mutuallyExclusive bool,
	parentA schema.CompositeType, setA *language.SelectionSet, scopeA scope,
	parentB schema.CompositeType, setB *language.SelectionSet, scopeB scope,
) []conflict {
	var found []conflict
	a := c.fieldsAndFragmentsOf(parentA, setA, scopeA)
	b := c.fieldsAndFragmentsOf(parentB, setB, scopeB)

	c.collectBetween(&found, mutuallyExclusive, a, b)
	for _, spread := range b.fragments {
		c.collectBetweenFieldsAndFragment(&found, mutuallyExclusive, a, spread)
	}
	for _, spread := range a.fragments {
		c.collectBetweenFieldsAndFragment(&found, mutuallyExclusive, b, spread)
	}
	for _, spreadA := range a.fragments {
		for _, spreadB := range b.fragments {
			c.collectBetweenFragments(&found, mutuallyExclusive, spreadA, spreadB)
		}
	}
	return found
}

// findConflict decides whether two selections under one response key disagree.
func (c *overlapChecker) findConflict(
	parentsMutuallyExclusive bool,
	responseName string,
	a, b selectedField,
) (conflict, bool) {
	// Fields of two different object types are reached only through fragments
	// on each, and an object matches at most one of those, so they never both
	// apply and are free to differ.
	mutuallyExclusive := parentsMutuallyExclusive ||
		(a.parent != b.parent && schema.IsObjectType(a.parent) && schema.IsObjectType(b.parent))

	if !mutuallyExclusive {
		nameA, nameB := nameOf(a.node.Name), nameOf(b.node.Name)
		if nameA != nameB {
			return conflict{
				responseName: responseName,
				reason:       conflictReason{text: quote(nameA) + " and " + quote(nameB) + " are different fields"},
				fields1:      []language.Node{a.node},
				fields2:      []language.Node{b.node},
			}, true
		}
		// The same field asked two different ways has two different answers.
		if !sameArguments(a.node.Arguments, a.scope, b.node.Arguments, b.scope) {
			return conflict{
				responseName: responseName,
				reason:       conflictReason{text: "they have differing arguments"},
				fields1:      []language.Node{a.node},
				fields2:      []language.Node{b.node},
			}, true
		}
	}

	// A streamed field is delivered in pieces of its own, so two selections
	// that would land in one place cannot both be streamed — not even when
	// they ask for the same thing, since that would deliver it twice — and a
	// field cannot be streamed on one selection and not the other.
	if why, overlap := overlappingStreams(
		a.node.Directives, a.scope, b.node.Directives, b.scope,
	); overlap {
		return conflict{
			responseName: responseName,
			reason:       conflictReason{text: why},
			fields1:      []language.Node{a.node},
			fields2:      []language.Node{b.node},
		}, true
	}

	var typeA, typeB schema.Type
	if a.def != nil {
		typeA = a.def.Type
	}
	if b.def != nil {
		typeB = b.def.Type
	}
	if typeA != nil && typeB != nil && typesConflict(typeA, typeB) {
		return conflict{
			responseName: responseName,
			reason: conflictReason{
				text: "they return conflicting types " + quote(typeA.String()) + " and " + quote(typeB.String()),
			},
			fields1: []language.Node{a.node},
			fields2: []language.Node{b.node},
		}, true
	}

	// Where both select subfields, the disagreement may be further down.
	if a.node.SelectionSet != nil && b.node.SelectionSet != nil {
		parentA, _ := schema.NamedTypeOf(typeA).(schema.CompositeType)
		parentB, _ := schema.NamedTypeOf(typeB).(schema.CompositeType)
		nested := c.conflictsBetweenSubSelections(mutuallyExclusive,
			parentA, a.node.SelectionSet, a.scope,
			parentB, b.node.SelectionSet, b.scope)
		if len(nested) == 0 {
			return conflict{}, false
		}
		return combineSubfieldConflicts(nested, responseName, a.node, b.node), true
	}
	return conflict{}, false
}

// fieldsAndFragmentsOf reads what a selection set contributes in one scope,
// remembering the answer.
func (c *overlapChecker) fieldsAndFragmentsOf(
	parent schema.CompositeType,
	set *language.SelectionSet,
	sc scope,
) *fieldsAndFragments {
	byScope, started := c.cache[set]
	if !started {
		byScope = map[string]*fieldsAndFragments{}
		c.cache[set] = byScope
	}
	if known, seen := byScope[sc.key]; seen {
		return known
	}

	contents := &fieldsAndFragments{
		byKey: map[string][]selectedField{},
		id:    fmt.Sprintf("%p|%s", set, sc.key),
	}
	// The group is recorded before it is filled, so that a fragment reaching
	// back to it finds the same one rather than building it again.
	byScope[sc.key] = contents
	c.collectFrom(parent, set, contents, sc)
	return contents
}

// fragmentContents reads what a fragment spread brings, against the type the
// fragment is conditioned on.
func (c *overlapChecker) fragmentContents(spread fragmentSpread) *fieldsAndFragments {
	fragment := c.ctx.Fragment(spread.name)
	if fragment == nil {
		return nil
	}
	var parent schema.CompositeType
	if fragment.TypeCondition != nil {
		if t, known := typeinfo.TypeFromAST(c.ctx.Schema(), fragment.TypeCondition); known {
			parent, _ = t.(schema.CompositeType)
		}
	}
	return c.fieldsAndFragmentsOf(parent, fragment.SelectionSet, spread.inner)
}

// collectFrom gathers a selection set's fields and spreads, descending through
// inline fragments because those contribute to the same place.
func (c *overlapChecker) collectFrom(
	parent schema.CompositeType,
	set *language.SelectionSet,
	into *fieldsAndFragments,
	sc scope,
) {
	if set == nil {
		return
	}
	for _, selection := range set.Selections {
		switch node := selection.(type) {
		case *language.Field:
			var def *schema.Field
			switch t := parent.(type) {
			case *schema.ObjectType:
				def = t.Field(nameOf(node.Name))
			case *schema.InterfaceType:
				def = t.Field(nameOf(node.Name))
			}
			into.add(node.ResponseKey(),
				selectedField{parent: parent, node: node, def: def, scope: sc})

		case *language.FragmentSpread:
			if node.Name != nil {
				into.addFragment(c.spreadOf(node, sc))
			}

		case *language.InlineFragment:
			inner := parent
			if node.TypeCondition != nil {
				inner = nil
				if t, known := typeinfo.TypeFromAST(c.ctx.Schema(), node.TypeCondition); known {
					inner, _ = t.(schema.CompositeType)
				}
			}
			c.collectFrom(inner, node.SelectionSet, into, sc)
		}
	}
}

// spreadOf reads one spread of a fragment: which fragment, and what the spread
// gave the variables the fragment declares.
//
// A fragment that declares variables has them renamed to something unique to
// this spread. That is what keeps a fragment's own `$x` apart from an
// operation's `$x`, and what makes two spreads given the same arguments agree
// while two given different ones do not.
func (c *overlapChecker) spreadOf(node *language.FragmentSpread, outer scope) fragmentSpread {
	name := node.Name.Value
	fragment := c.ctx.Fragment(name)
	if fragment == nil {
		return fragmentSpread{name: name, node: node, key: name + "?"}
	}

	given := make(map[string]language.Value, len(node.Arguments))
	for _, arg := range node.Arguments {
		if arg != nil && arg.Name != nil {
			given[arg.Name.Value] = arg.Value
		}
	}

	var b strings.Builder
	b.WriteString(name + "(")
	declared := make([]string, 0, len(fragment.VariableDefinitions))
	for _, def := range fragment.VariableDefinitions {
		if def == nil || def.Variable == nil || def.Variable.Name == nil {
			continue
		}
		variable := def.Variable.Name.Value
		declared = append(declared, variable)
		// An argument the spread did not give leaves the variable to whatever
		// the fragment's own default says, which is the same for every such
		// spread and so contributes nothing to what tells them apart.
		if value, supplied := given[variable]; supplied {
			b.WriteString(variable + ": " + canonicalValue(value, outer) + " ")
		}
	}
	b.WriteString(")")
	key := b.String()

	if len(declared) == 0 {
		// Nothing is renamed, so the fragment brings the same fields wherever
		// it is spread and the enclosing scope carries on unchanged.
		return fragmentSpread{name: name, node: node, key: key}
	}
	rename := make(map[string]string, len(declared))
	for _, variable := range declared {
		rename[variable] = key + "\x00" + variable
	}
	return fragmentSpread{name: name, node: node, key: key, inner: scope{key: key, rename: rename}}
}

// combineSubfieldConflicts folds conflicts found beneath two fields into one
// reported at the fields themselves, so that a reader is pointed at what they
// wrote rather than at the innermost leaf.
func combineSubfieldConflicts(nested []conflict, responseName string, a, b *language.Field) conflict {
	combined := conflict{
		responseName: responseName,
		fields1:      []language.Node{a},
		fields2:      []language.Node{b},
	}
	for _, found := range nested {
		combined.reason.children = append(combined.reason.children,
			namedReason{responseName: found.responseName, reason: found.reason})
		combined.fields1 = append(combined.fields1, found.fields1...)
		combined.fields2 = append(combined.fields2, found.fields2...)
	}
	return combined
}

// typesConflict reports whether two field types could not both describe one
// value in the response.
//
// The shape has to match exactly, because a list and a single value, or a
// nullable and a non-null, are different things to a client reading the
// response. Beneath the wrappers, two different leaf types conflict while two
// composite ones do not: what they have in common is settled by comparing the
// subfields.
func typesConflict(a, b schema.Type) bool {
	if listA, isListA := a.(*schema.List); isListA {
		listB, isListB := b.(*schema.List)
		return !isListB || typesConflict(listA.OfType, listB.OfType)
	}
	if _, isListB := b.(*schema.List); isListB {
		return true
	}
	if nonNullA, isNonNullA := a.(*schema.NonNull); isNonNullA {
		nonNullB, isNonNullB := b.(*schema.NonNull)
		return !isNonNullB || typesConflict(nonNullA.OfType, nonNullB.OfType)
	}
	if _, isNonNullB := b.(*schema.NonNull); isNonNullB {
		return true
	}
	if schema.IsLeafType(a) || schema.IsLeafType(b) {
		return a != b
	}
	return false
}

// sameArguments reports whether two selections were given the same arguments.
//
// Order is not part of what an argument list means, so the comparison is by
// name; the values are compared as written, with object fields put in a
// settled order so that two spellings of the same value match. A variable is
// compared under the scope it was written in, so a fragment's own variable is
// never taken for an operation variable of the same name.
func sameArguments(a []*language.Argument, scopeA scope, b []*language.Argument, scopeB scope) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	values := make(map[string]language.Value, len(b))
	for _, arg := range b {
		if arg != nil && arg.Name != nil {
			values[arg.Name.Value] = arg.Value
		}
	}
	for _, arg := range a {
		if arg == nil || arg.Name == nil {
			continue
		}
		other, given := values[arg.Name.Value]
		if !given || canonicalValue(arg.Value, scopeA) != canonicalValue(other, scopeB) {
			return false
		}
	}
	return true
}

// overlappingStreams reports whether two selections of one field cannot both
// be delivered, and why.
func overlappingStreams(
	a []*language.Directive, scopeA scope, b []*language.Directive, scopeB scope,
) (string, bool) {
	streamA := findDirectiveNamed(a, schema.Stream.Name())
	streamB := findDirectiveNamed(b, schema.Stream.Name())
	switch {
	case streamA == nil && streamB == nil:
		return "", false
	case streamA != nil && streamB != nil:
		if sameArguments(streamA.Arguments, scopeA, streamB.Arguments, scopeB) {
			// Two identical streams of one field were allowed by an early
			// draft, so the message says where the change was argued out.
			return "they have overlapping stream directives. " +
				"See https://github.com/graphql/defer-stream-wg/discussions/100", true
		}
		return "they have overlapping stream directives", true
	default:
		return "they have overlapping stream directives", true
	}
}

// findDirectiveNamed returns the directive of the given name, or nil.
func findDirectiveNamed(directives []*language.Directive, name string) *language.Directive {
	for _, d := range directives {
		if d != nil && d.Name != nil && d.Name.Value == name {
			return d
		}
	}
	return nil
}

// canonicalValue renders a literal in a settled form, so that two ways of
// writing the same value compare equal.
func canonicalValue(v language.Value, sc scope) string {
	switch node := v.(type) {
	case nil:
		return ""
	case *language.Variable:
		name := nameOf(node.Name)
		if renamed, isFragmentVariable := sc.rename[name]; isFragmentVariable {
			return "$" + renamed
		}
		return "$" + name
	case *language.ObjectValue:
		// The fields of an input object are unordered, so they are put in a
		// settled order before being compared.
		type named struct{ name, written string }
		parts := make([]named, 0, len(node.Fields))
		for _, field := range node.Fields {
			if field != nil {
				parts = append(parts, named{nameOf(field.Name), canonicalValue(field.Value, sc)})
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
		// A list is ordered, so its entries stay as written.
		parts := make([]string, len(node.Values))
		for i, entry := range node.Values {
			parts[i] = canonicalValue(entry, sc)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return language.Print(v)
	}
}

// pairSet remembers which pairs have been compared.
//
// A pair compared without assuming exclusivity has been compared more
// thoroughly than one compared with it, so the first answer covers the second
// but not the other way round.
type pairSet map[string]bool

func (p pairSet) key(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "\x00" + b
}

func (p pairSet) has(a, b string, mutuallyExclusive bool) bool {
	comparedExclusively, compared := p[p.key(a, b)]
	if !compared {
		return false
	}
	if mutuallyExclusive {
		return true
	}
	return !comparedExclusively
}

func (p pairSet) add(a, b string, mutuallyExclusive bool) {
	p[p.key(a, b)] = mutuallyExclusive
}

// spreadAsWritten renders a spread the way a message names it: the fragment
// and the arguments it was given, without the leading dots or any directives.
func spreadAsWritten(node *language.FragmentSpread) string {
	if node == nil {
		return ""
	}
	bare := *node
	bare.Directives = nil
	return strings.TrimPrefix(language.Print(&bare), "...")
}
