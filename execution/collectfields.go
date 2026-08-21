package execution

import (
	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/internal/typeinfo"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// DeferUsage is one @defer written in a document.
//
// A deferred fragment's fields are not part of the first response; they arrive
// afterwards, in a payload of their own. A @defer inside another one is
// deferred again, so each records what encloses it.
type DeferUsage struct {
	// Label is what the document called this one, which is how a client tells
	// two deferred payloads apart. It may be empty.
	Label string
	// Parent is the deferred fragment this one is written inside, or nil.
	Parent *DeferUsage
	// order is where in the walk this one was met. Deferrals are only ever
	// compared with others found in the same walk, and this is what puts a set
	// of them in a settled order so that which one a payload is delivered
	// under does not vary between runs.
	order int
}

// FieldSelection is one place a document asked for a field, and whether it
// asked inside a deferred fragment.
type FieldSelection struct {
	// Node is the field as the document wrote it.
	Node *language.Field
	// Defer is the deferred fragment the field was found inside, or nil where
	// it belongs to the first response.
	Defer *DeferUsage
	// Variables is the variable scope the field was found under: the request's
	// own variables, with a fragment's arguments over the top where the field
	// was reached through a fragment that declares any. It is nil where the
	// caller built the selection rather than collecting it.
	Variables schema.VariableValues
}

// GroupedFieldSet is the fields of a selection set, gathered by the key each
// takes in the response.
//
// The order is the order the document wrote them in, because that is the order
// the response has to be in: the specification asks for the keys of an object
// in the response to follow the query, and a client reading a JSON object as
// an ordered list of pairs would otherwise see them shuffled.
//
// Several selections share a key when the same field is asked for more than
// once, whether written twice or reached through two fragments. They are one
// field with one value, so they are resolved once and their subselections are
// merged.
type GroupedFieldSet struct {
	order []string
	byKey map[string]fieldGroup
	errs  []*gqlerror.Error
}

// fieldGroup is what one response key asks for.
type fieldGroup struct {
	selections []FieldSelection
	// nodes are the field nodes of those selections. A resolver is given them,
	// and so is every error reported against the field, which means they would
	// otherwise be read out again for every object the set is used for — once
	// per entry of a list, and once per entry of every list inside that. They
	// are read out once instead, when the set stops being added to.
	nodes []*language.Field
	// args are the field's arguments after coercion, and argsErr is what was
	// wrong with them. Both are worked out once for the object type the set
	// was collected against, since a field written once asks for the same
	// arguments however many objects it is resolved against — which is once
	// per entry of a list, and once per entry of every list inside that.
	//
	// coerced says they were worked out at all. A set collected without an
	// object type to hand, or one built while planning a deferred payload,
	// has none, and the executor works them out as it goes.
	args    schema.Arguments
	argsErr *gqlerror.Error
	coerced bool
}

// Len returns how many response keys the set has.
func (g *GroupedFieldSet) Len() int { return len(g.order) }

// Keys returns the response keys in the order the document wrote them.
// Callers must not modify the returned slice.
func (g *GroupedFieldSet) Keys() []string { return g.order }

// Fields returns the selections that share a response key.
// Callers must not modify the returned slice.
func (g *GroupedFieldSet) Fields(key string) []FieldSelection { return g.byKey[key].selections }

// Errors is what went wrong while the fields were being gathered: an argument
// a fragment spread supplied that does not fit what the fragment declared.
// A document that has passed validation produces none.
func (g *GroupedFieldSet) Errors() []*gqlerror.Error { return g.errs }

// Nodes returns the field nodes that share a response key, which is what a
// caller with no interest in deferral wants.
// Callers must not modify the returned slice.
func (g *GroupedFieldSet) Nodes(key string) []*language.Field {
	group := g.byKey[key]
	if group.nodes != nil {
		return group.nodes
	}
	return nodesOf(group.selections)
}

// arguments returns the field's coerced arguments where they have been worked
// out already, and says whether they have.
func (g *GroupedFieldSet) arguments(key string) (schema.Arguments, *gqlerror.Error, bool) {
	group := g.byKey[key]
	return group.args, group.argsErr, group.coerced
}

// coerce records what a response key's arguments came to.
func (g *GroupedFieldSet) coerce(key string, args schema.Arguments, err *gqlerror.Error) {
	group := g.byKey[key]
	group.args, group.argsErr, group.coerced = args, err, true
	g.byKey[key] = group
}

// seal says the set is finished, and reads out the field nodes of each
// response key while it is still the only thing looking at them.
//
// A set that is not sealed still answers [GroupedFieldSet.Nodes] correctly, by
// reading them out again each time. Sealing is what makes that once rather
// than once per object.
func (g *GroupedFieldSet) seal() *GroupedFieldSet {
	for _, key := range g.order {
		group := g.byKey[key]
		group.nodes = nodesOf(group.selections)
		g.byKey[key] = group
	}
	return g
}

// nodesOf reads the field nodes out of a group of selections.
func nodesOf(selections []FieldSelection) []*language.Field {
	nodes := make([]*language.Field, 0, len(selections))
	for _, selection := range selections {
		nodes = append(nodes, selection.Node)
	}
	return nodes
}

// add records a selection under the key it takes in the response.
func (g *GroupedFieldSet) add(key string, selection FieldSelection) {
	if g.byKey == nil {
		g.byKey = map[string]fieldGroup{}
	}
	group, seen := g.byKey[key]
	if !seen {
		g.order = append(g.order, key)
	}
	group.selections = append(group.selections, selection)
	g.byKey[key] = group
}

// fieldCollector gathers the fields of a selection set against one object
// type.
type fieldCollector struct {
	schema    *schema.Schema
	fragments map[string]*language.FragmentDefinition
	variables schema.VariableValues
	// deferrals collects the @defer usages met along the way, in the order
	// they were met, so that a payload can be planned for each.
	deferrals []*DeferUsage
	// incremental says whether @defer and @stream are being honoured. Where
	// they are not, a deferred fragment is treated as an ordinary one.
	incremental bool
	// errs collects what a fragment spread got wrong about the arguments the
	// fragment declares.
	errs []*gqlerror.Error
}

// CollectFields returns the fields a selection set asks of an object type.
//
// Fragments are followed and inline fragments flattened, because both describe
// fields of the same object; a fragment conditioned on a type this object is
// not contributes nothing. @skip and @include are applied here, so a field
// switched off never reaches a resolver.
//
// @defer is not honoured: every field comes back as part of one set. Use
// [CollectFieldsIncrementally] where deferred fields are to be delivered
// separately.
func CollectFields(
	s *schema.Schema,
	fragments map[string]*language.FragmentDefinition,
	variables schema.VariableValues,
	objectType *schema.ObjectType,
	selectionSet *language.SelectionSet,
) *GroupedFieldSet {
	fields, _ := CollectFieldsIncrementally(s, fragments, variables, objectType, selectionSet, false)
	return fields
}

// CollectFieldsIncrementally is [CollectFields] with a say over whether @defer
// is honoured.
//
// When it is, a field written inside a deferred fragment is marked with the
// deferral it was found under, and the deferrals met are returned alongside,
// in the order the document wrote them.
func CollectFieldsIncrementally(
	s *schema.Schema,
	fragments map[string]*language.FragmentDefinition,
	variables schema.VariableValues,
	objectType *schema.ObjectType,
	selectionSet *language.SelectionSet,
	incremental bool,
) (*GroupedFieldSet, []*DeferUsage) {
	c := &fieldCollector{schema: s, fragments: fragments, variables: variables, incremental: incremental}
	fields := &GroupedFieldSet{}
	c.collect(objectType, selectionSet, fields, nil, variables, map[string]spreadState{})
	fields.errs = c.errs
	return fields.seal(), c.deferrals
}

// CollectSubfields returns the fields asked of an object beneath several
// selections that share a response key.
//
// The selections are one field, so what they ask for underneath is one
// selection set: `a { x } a { y }` asks for `a` once, with both x and y.
func CollectSubfields(
	s *schema.Schema,
	fragments map[string]*language.FragmentDefinition,
	variables schema.VariableValues,
	objectType *schema.ObjectType,
	fields []*language.Field,
) *GroupedFieldSet {
	selections := make([]FieldSelection, len(fields))
	for i, field := range fields {
		selections[i] = FieldSelection{Node: field}
	}
	collected, _ := collectSubfields(s, fragments, variables, objectType, selections, false)
	return collected
}

// collectSubfields gathers what several selections of one field ask for
// underneath, keeping track of which deferral each was found under.
func collectSubfields(
	s *schema.Schema,
	fragments map[string]*language.FragmentDefinition,
	variables schema.VariableValues,
	objectType *schema.ObjectType,
	selections []FieldSelection,
	incremental bool,
) (*GroupedFieldSet, []*DeferUsage) {
	c := &fieldCollector{schema: s, fragments: fragments, variables: variables, incremental: incremental}
	collected := &GroupedFieldSet{}
	for _, selection := range selections {
		if selection.Node == nil || selection.Node.SelectionSet == nil {
			continue
		}
		// A selection reached through a fragment that declares arguments
		// carries the scope it was found under, and what it asks for
		// underneath is read under that same scope.
		scope := selection.Variables
		if !scope.IsSet() {
			scope = variables
		}
		// Each selection brings its own record of which fragments have been
		// followed: the same fragment reached from two of them contributes to
		// both, because each may sit under a different deferral.
		c.collect(objectType, selection.Node.SelectionSet, collected, selection.Defer, scope, map[string]spreadState{})
	}
	collected.errs = c.errs
	return collected.seal(), c.deferrals
}

// collect walks a selection set, adding what it asks of the object type.
//
// within is the deferred fragment the walk is already inside, which anything
// found here belongs to unless a further @defer says otherwise.
func (c *fieldCollector) collect(
	objectType *schema.ObjectType,
	selectionSet *language.SelectionSet,
	into *GroupedFieldSet,
	within *DeferUsage,
	scope schema.VariableValues,
	visited map[string]spreadState,
) {
	if selectionSet == nil {
		return
	}
	for _, selection := range selectionSet.Selections {
		switch node := selection.(type) {
		case *language.Field:
			if !c.included(node.Directives, scope) {
				continue
			}
			into.add(node.ResponseKey(),
				FieldSelection{Node: node, Defer: within, Variables: scope})

		case *language.FragmentSpread:
			if node.Name == nil || !c.included(node.Directives, scope) {
				continue
			}
			name := node.Name.Value
			fragment := c.fragments[name]
			if fragment == nil || !c.applies(objectType, fragment.TypeCondition) {
				continue
			}

			// How a fragment was spread before decides whether spreading it
			// again contributes anything. Spreading it plainly twice asks for
			// the same fields twice, so the second is dropped, and so is a
			// cycle. Deferring it twice would announce two payloads holding
			// the same thing, so the second is dropped as well. But spreading
			// it plainly after deferring it is a real request: what the
			// deferred payload holds is not part of the first response.
			inner := within
			if deferral := c.deferralOn(node.Directives, within, scope); deferral == nil {
				if visited[name] == spreadPlainly {
					continue
				}
				visited[name] = spreadPlainly
			} else {
				if visited[name] != notSpread {
					continue
				}
				visited[name] = spreadDeferred
				inner = c.record(deferral)
			}
			// A fragment that declares arguments reads its body under a
			// scope of its own: what the spread supplied, over the top of
			// whatever the spread itself was written under.
			c.collect(objectType, fragment.SelectionSet, into, inner,
				c.scopeFor(fragment, node, scope), visited)

		case *language.InlineFragment:
			if !c.included(node.Directives, scope) || !c.applies(objectType, node.TypeCondition) {
				continue
			}
			inner := within
			if deferral := c.deferralOn(node.Directives, within, scope); deferral != nil {
				inner = c.record(deferral)
			}
			c.collect(objectType, node.SelectionSet, into, inner, scope, visited)
		}
	}
}

// scopeFor works out the variables a fragment's body is read under.
//
// A fragment's body may name the request's own variables and the ones the
// fragment declares, and nothing else — which is what validation requires of
// it — so the scope is the request's, with the declared ones over the top. A
// scope does not travel through a fragment that declares nothing: what an
// enclosing fragment was reading under is not visible inside one that did not
// ask for it.
//
// A declared name always comes from the spread, or from the fragment's own
// default, or from nowhere. It is never answered by a request variable that
// happens to share the name, which is why each is taken out first.
//
// What the spread supplied is read under the scope the spread itself was
// written in, since that is where those values were named.
//
// Fragment arguments are experimental, so nothing here runs for a document
// that did not opt in to the syntax: without it a fragment cannot declare a
// variable in the first place.
func (c *fieldCollector) scopeFor(
	fragment *language.FragmentDefinition,
	spread *language.FragmentSpread,
	outer schema.VariableValues,
) schema.VariableValues {
	if len(fragment.VariableDefinitions) == 0 {
		return c.variables
	}

	supplied := make(map[string]*language.FragmentArgument, len(spread.Arguments))
	for _, arg := range spread.Arguments {
		if arg != nil && arg.Name != nil {
			supplied[arg.Name.Value] = arg
		}
	}

	scope := c.variables.Clone()
	for _, def := range fragment.VariableDefinitions {
		if def == nil || def.Variable == nil || def.Variable.Name == nil {
			continue
		}
		name := def.Variable.Name.Value
		// A declared name is answered by the spread or not at all. Taking it
		// out first is what keeps an operation variable of the same name from
		// showing through where the spread said nothing.
		scope.Delete(name)

		declared, known := typeinfo.TypeFromAST(c.schema, def.Type)
		if !known || !schema.IsInputType(declared) {
			c.errs = append(c.errs, gqlerror.New(
				"Variable "+quote("$"+name)+" expected value of type "+
					quote(language.Print(def.Type))+" which cannot be used as an input type.",
				gqlerror.WithNodes(def.Type)))
			continue
		}
		if v, ok := c.fragmentArgument(fragment, spread, name, def, declared, supplied[name], outer); ok {
			scope.Set(name, v, declared)
		}
	}
	return scope
}

// fragmentArgument works out one of a fragment's declared variables from what
// the spread supplied, and reports what is wrong where nothing fits.
//
// The rules are an argument's rules, because that is what this is: a value the
// spread left out falls back to the fragment's own default, and so does one
// written as a variable the request did not supply.
func (c *fieldCollector) fragmentArgument(
	fragment *language.FragmentDefinition,
	spread *language.FragmentSpread,
	name string,
	def *language.VariableDefinition,
	declared schema.Type,
	arg *language.FragmentArgument,
	outer schema.VariableValues,
) (any, bool) {
	required := schema.IsNonNullType(declared) && def.DefaultValue == nil
	blamed := "Variable " + quote("$"+name) + " defined by fragment " + quote(nameOf(spread.Name))

	written := arg != nil
	if written && !required {
		if variable, isVariable := arg.Value.(*language.Variable); isVariable {
			if _, supplied := outer.Get(nameOf(variable.Name)); !supplied {
				written = false
			}
		}
	}

	if !written {
		if arg == nil && required {
			c.errs = append(c.errs, gqlerror.New(
				blamed+" of required type "+quote(declared.String())+" was not provided.",
				gqlerror.WithNodes(spread)))
			return nil, false
		}
		if def.DefaultValue == nil {
			return nil, false
		}
		fallback, ok := schema.CoerceInputLiteral(def.DefaultValue, declared, schema.VariableValues{})
		if !ok {
			// The spread is what is blamed, as it is for a value the spread
			// left out: graphql-js reports both against the node that reached
			// the fragment, not against the fragment's own declaration.
			c.errs = append(c.errs, explain(blamed+" has invalid default value",
				literalReasons(def.DefaultValue, declared, schema.VariableValues{}), spread)...)
			return nil, false
		}
		return fallback, true
	}

	converted, ok := schema.CoerceInputLiteral(arg.Value, declared, outer)
	if !ok {
		errs := explainWritten(blamed+" has invalid value",
			literalReasons(arg.Value, declared, outer))
		if len(errs) == 0 {
			errs = []*gqlerror.Error{gqlerror.New(
				blamed+" has invalid value "+language.Print(arg.Value)+".",
				gqlerror.WithNodes(arg))}
		}
		c.errs = append(c.errs, errs...)
		return nil, false
	}
	return converted, true
}

// spreadState says how a fragment has been spread so far, which is what
// decides whether spreading it again contributes anything.
type spreadState int

const (
	// notSpread means the fragment has not been reached yet.
	notSpread spreadState = iota
	// spreadPlainly means it has been spread into the response being built.
	spreadPlainly
	// spreadDeferred means it has been spread into a payload of its own.
	spreadDeferred
)

// deferralOn reads a @defer written on a fragment, returning the deferral
// anything inside it would belong to, or nil where there is none.
//
// A @defer switched off with `if: false` asks for nothing to be deferred, so
// what it holds stays where it was written. Nothing is recorded here: whether
// the deferral is real depends on whether the fragment is followed at all.
func (c *fieldCollector) deferralOn(
	directives []*language.Directive, within *DeferUsage, scope schema.VariableValues,
) *DeferUsage {
	if !c.incremental {
		return nil
	}
	args, written := directiveValues(declaredDirective(c.schema, schema.Defer), directives, scope)
	if !written {
		return nil
	}
	if on, given := args.Get("if"); given && on != true {
		return nil
	}
	label, _ := args.Get("label")
	text, _ := label.(string)
	return &DeferUsage{Label: text, Parent: within}
}

// record keeps a deferral that is really going to be delivered, in the order
// the document wrote it.
func (c *fieldCollector) record(deferral *DeferUsage) *DeferUsage {
	deferral.order = len(c.deferrals)
	c.deferrals = append(c.deferrals, deferral)
	return deferral
}

// included applies @skip and @include, which decide whether a selection is
// part of the request at all.
//
// @skip wins over @include where they disagree, which is what the
// specification says and is the safer way round: asking for something to be
// left out is the more specific instruction.
func (c *fieldCollector) included(directives []*language.Directive, scope schema.VariableValues) bool {
	// A condition that cannot be settled — a variable with no value, which
	// only happens while a document is being checked rather than run — is read
	// as not true. @skip then does not skip, and @include does not include.
	if args, written := directiveValues(declaredDirective(c.schema, schema.Skip), directives, scope); written {
		if skip, _ := args.Get("if"); skip == true {
			return false
		}
	} else if findDirectiveNamed(directives, schema.Skip.Name()) != nil {
		return true
	}
	if args, written := directiveValues(declaredDirective(c.schema, schema.Include), directives, scope); written {
		if include, _ := args.Get("if"); include != true {
			return false
		}
	} else if findDirectiveNamed(directives, schema.Include.Name()) != nil {
		return false
	}
	return true
}

// findDirectiveNamed returns a directive of the given name, or nil.
func findDirectiveNamed(directives []*language.Directive, name string) *language.Directive {
	for _, d := range directives {
		if d != nil && d.Name != nil && d.Name.Value == name {
			return d
		}
	}
	return nil
}

// applies reports whether a fragment's type condition holds for the object
// being resolved. A fragment with no condition applies wherever it is written.
func (c *fieldCollector) applies(objectType *schema.ObjectType, condition *language.NamedType) bool {
	if condition == nil {
		return true
	}
	conditional, known := typeinfo.TypeFromAST(c.schema, condition)
	if !known {
		return false
	}
	if named, isNamed := conditional.(schema.NamedType); isNamed && named == schema.NamedType(objectType) {
		return true
	}
	abstract, isAbstract := conditional.(schema.AbstractType)
	return isAbstract && c.schema.IsSubType(abstract, objectType)
}
