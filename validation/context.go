package validation

import (
	"fmt"

	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/internal/typeinfo"
	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/value"
)

// Rule inspects a document and reports what is wrong with it.
//
// A rule is called once per validation and returns a visitor, which the walk
// then drives. State a rule needs across nodes lives in the closure the
// visitor captures, so each validation starts afresh.
//
// A rule that needs to know about types reads them from the context, which
// follows the walk. Rules that only read the document work whether or not
// there is a schema to check against.
type Rule func(*Context) language.Visitor

// Context is what a rule is given: the document being checked, the schema to
// check it against, where in the schema the walk currently is, and somewhere
// to report what it finds.
//
// The questions a rule asks about a document tend to be the same few, and
// answering them means walking the document again. The answers are worked out
// once and kept, because with several rules asking the same question of the
// same document the repeated walks would otherwise dominate.
//
// A Context belongs to one validation and is not safe for concurrent use.
type Context struct {
	schema   *schema.Schema
	document *language.Document
	typeInfo *typeinfo.TypeInfo

	errors    []*gqlerror.Error
	maxErrors int
	// hideSuggestions leaves the "Did you mean …?" out of every message.
	hideSuggestions bool
	// full records that the cap has been reached, which ends the walk.
	full bool

	// fragments indexes the document's fragment definitions by name.
	fragments map[string]*language.FragmentDefinition
	// fragmentsCollected guards the index, since a document with no fragments
	// would otherwise be indexed on every question.
	fragmentsCollected bool

	fragmentSpreads         map[*language.SelectionSet][]*language.FragmentSpread
	referencedFragments     map[*language.OperationDefinition][]*language.FragmentDefinition
	variableUsages          map[language.Node][]VariableUsage
	recursiveVariableUsages map[*language.OperationDefinition][]VariableUsage
}

// VariableUsage is one place a variable is used, together with what the
// document expects of it there.
type VariableUsage struct {
	// Node is the variable as it appears in the document.
	Node *language.Variable
	// Type is the input type the variable is used at, or nil where the
	// document does not match the schema.
	Type schema.Type
	// ParentType is the input type one level out: for a variable feeding a
	// field of an input object, the object itself.
	ParentType schema.Type
	// Default is the default of the argument or input field the variable
	// stands in for. Unset means there is no default, which is not the same as
	// a default of null.
	Default value.Maybe[schema.DefaultInput]
	// FragmentVarDef is the fragment's own declaration of this variable, where
	// the use is inside a fragment that takes arguments. Such a variable is
	// supplied by the spread rather than by the operation, so a rule looking
	// for variables the operation must declare has to leave it out.
	FragmentVarDef *language.VariableDefinition
}

// defaultMaxErrors is how many problems are reported before the walk gives up.
//
// A document can be wrong in unboundedly many ways, and a client cannot act on
// a thousand errors any better than on a hundred. The cap keeps a hostile or
// merely broken document from costing more to reject than to run.
const defaultMaxErrors = 100

// NewContext returns a context for checking a document against a schema.
//
// The schema may be nil, which is the state SDL is checked in before there is
// a schema to check it against. Questions about types then have no answer,
// which rules written for that case already cope with.
func NewContext(s *schema.Schema, doc *language.Document) *Context {
	return &Context{
		schema:    s,
		document:  doc,
		typeInfo:  typeinfo.NewTypeInfoForDocument(s, doc),
		maxErrors: defaultMaxErrors,
	}
}

// Schema is the schema the document is being checked against, or nil.
func (c *Context) Schema() *schema.Schema { return c.schema }

// Document is the document being checked.
func (c *Context) Document() *language.Document { return c.document }

// TypeInfo follows the walk, and is what the type questions below read from.
func (c *Context) TypeInfo() *typeinfo.TypeInfo { return c.typeInfo }

// Errors returns what has been reported so far.
func (c *Context) Errors() []*gqlerror.Error { return c.errors }

// CheckOptions are the settings a rule passes on when it asks the schema
// package to check a value or a literal, so that what it says agrees with what
// the rule itself would have said.
func (c *Context) CheckOptions() []schema.CheckOption {
	if c.hideSuggestions {
		return []schema.CheckOption{schema.WithoutSuggestions()}
	}
	return nil
}

// DidYouMean renders a suggestion, or nothing where the caller asked for none.
func (c *Context) DidYouMean(subMessage string, suggestions []string) string {
	if c.hideSuggestions {
		return ""
	}
	return schema.DidYouMean(subMessage, suggestions)
}

// The questions below are about where in the schema the walk is. Each has no
// answer wherever the document does not match the schema, which is ordinary
// during validation rather than exceptional.

// Type is the type of the field the walk is inside, or nil.
func (c *Context) Type() schema.Type { return c.typeInfo.Type() }

// ParentType is the composite type whose selection set the walk is inside.
func (c *Context) ParentType() schema.CompositeType { return c.typeInfo.ParentType() }

// InputType is the type of the value the walk is inside.
func (c *Context) InputType() schema.Type { return c.typeInfo.InputType() }

// ParentInputType is the input type one level out.
func (c *Context) ParentInputType() schema.Type { return c.typeInfo.ParentInputType() }

// FieldDef is the definition of the field the walk is inside.
func (c *Context) FieldDef() *schema.Field { return c.typeInfo.FieldDef() }

// Directive is the directive the walk is inside.
func (c *Context) Directive() *schema.Directive { return c.typeInfo.Directive() }

// Argument is the argument the walk is inside.
func (c *Context) Argument() *schema.Argument { return c.typeInfo.Argument() }

// EnumValue is the enum member the walk is looking at.
func (c *Context) EnumValue() *schema.EnumValue { return c.typeInfo.EnumValue() }

// Report records a problem with the document, blaming the given nodes.
func (c *Context) Report(message string, nodes ...language.Node) {
	c.ReportError(gqlerror.New(message, gqlerror.WithNodes(nodes...)))
}

// Reportf records a problem with a formatted message.
func (c *Context) Reportf(nodes []language.Node, format string, args ...any) {
	c.Report(fmt.Sprintf(format, args...), nodes...)
}

// ReportError records an error a rule has built for itself, which is what a
// rule does when it has more to say than a message.
func (c *Context) ReportError(err *gqlerror.Error) {
	if c.full || err == nil {
		return
	}
	// A negative bound is no bound, which is how a caller asks for all of
	// them; graphql-js passes Infinity, which Go has no int for.
	if c.maxErrors >= 0 && len(c.errors) >= c.maxErrors {
		c.errors = append(c.errors, gqlerror.New(
			"Too many validation errors, error limit reached. Validation aborted."))
		c.full = true
		return
	}
	c.errors = append(c.errors, err)
}

// Fragment returns the fragment of the given name, or nil if the document
// defines none.
func (c *Context) Fragment(name string) *language.FragmentDefinition {
	if !c.fragmentsCollected {
		c.fragments = map[string]*language.FragmentDefinition{}
		c.fragmentsCollected = true
		for _, def := range c.document.Definitions {
			fragment, ok := def.(*language.FragmentDefinition)
			if !ok || fragment.Name == nil {
				continue
			}
			// A name defined more than once is a separate complaint; the first
			// definition is the one used here, so that the rest of validation
			// has something to work with either way.
			if _, taken := c.fragments[fragment.Name.Value]; !taken {
				c.fragments[fragment.Name.Value] = fragment
			}
		}
	}
	return c.fragments[name]
}

// FragmentSpreads returns every fragment spread within a selection set,
// including those nested inside fields and inline fragments.
func (c *Context) FragmentSpreads(set *language.SelectionSet) []*language.FragmentSpread {
	if set == nil {
		return nil
	}
	if spreads, known := c.fragmentSpreads[set]; known {
		return spreads
	}

	var spreads []*language.FragmentSpread
	toVisit := []*language.SelectionSet{set}
	for len(toVisit) > 0 {
		current := toVisit[len(toVisit)-1]
		toVisit = toVisit[:len(toVisit)-1]
		for _, selection := range current.Selections {
			switch node := selection.(type) {
			case *language.FragmentSpread:
				spreads = append(spreads, node)
			case *language.Field:
				if node.SelectionSet != nil {
					toVisit = append(toVisit, node.SelectionSet)
				}
			case *language.InlineFragment:
				if node.SelectionSet != nil {
					toVisit = append(toVisit, node.SelectionSet)
				}
			}
		}
	}

	if c.fragmentSpreads == nil {
		c.fragmentSpreads = map[*language.SelectionSet][]*language.FragmentSpread{}
	}
	c.fragmentSpreads[set] = spreads
	return spreads
}

// RecursivelyReferencedFragments returns every fragment an operation can
// reach: those it spreads, those they spread, and so on.
//
// A fragment is listed once however many times it is reached, and a cycle
// terminates rather than spinning, so this is safe on a document that has not
// yet been checked for cycles.
func (c *Context) RecursivelyReferencedFragments(op *language.OperationDefinition) []*language.FragmentDefinition {
	if op == nil {
		return nil
	}
	if fragments, known := c.referencedFragments[op]; known {
		return fragments
	}

	var fragments []*language.FragmentDefinition
	collected := map[string]bool{}
	toVisit := []*language.SelectionSet{op.SelectionSet}
	for len(toVisit) > 0 {
		current := toVisit[len(toVisit)-1]
		toVisit = toVisit[:len(toVisit)-1]
		for _, spread := range c.FragmentSpreads(current) {
			if spread.Name == nil {
				continue
			}
			name := spread.Name.Value
			if collected[name] {
				continue
			}
			collected[name] = true
			if fragment := c.Fragment(name); fragment != nil {
				fragments = append(fragments, fragment)
				toVisit = append(toVisit, fragment.SelectionSet)
			}
		}
	}

	if c.referencedFragments == nil {
		c.referencedFragments = map[*language.OperationDefinition][]*language.FragmentDefinition{}
	}
	c.referencedFragments[op] = fragments
	return fragments
}

// VariableUsages returns every variable used within a definition, together
// with the type each is used at.
//
// A variable in the definition's own variable definitions is a declaration
// rather than a use, and is not listed.
func (c *Context) VariableUsages(node language.Node) []VariableUsage {
	if node == nil {
		return nil
	}
	if usages, known := c.variableUsages[node]; known {
		return usages
	}

	var usages []VariableUsage
	// A fragment may declare variables of its own, which its spreads supply.
	var declaredByFragment []*language.VariableDefinition
	if fragment, isFragment := node.(*language.FragmentDefinition); isFragment {
		declaredByFragment = fragment.VariableDefinitions
	}

	// A separate TypeInfo, because this walk is its own and must not disturb
	// the position of the one following the validation walk.
	info := typeinfo.NewTypeInfoForDocument(c.schema, c.document)
	language.Visit(node, typeinfo.VisitWithTypeInfo(info, language.Visitor{
		Enter: func(n language.Node, _ language.VisitContext) language.VisitAction {
			switch v := n.(type) {
			case *language.VariableDefinition:
				// The variable being declared is not a use of it.
				return language.VisitSkip
			case *language.Variable:
				usages = append(usages, VariableUsage{
					Node:           v,
					Type:           info.InputType(),
					ParentType:     info.ParentInputType(),
					Default:        info.DefaultValue(),
					FragmentVarDef: findVariableDefinition(declaredByFragment, v),
				})
			}
			return language.VisitContinue
		},
	}))

	if c.variableUsages == nil {
		c.variableUsages = map[language.Node][]VariableUsage{}
	}
	c.variableUsages[node] = usages
	return usages
}

// RecursiveVariableUsages returns every variable an operation uses, including
// those used by the fragments it reaches.
func (c *Context) RecursiveVariableUsages(op *language.OperationDefinition) []VariableUsage {
	if op == nil {
		return nil
	}
	if usages, known := c.recursiveVariableUsages[op]; known {
		return usages
	}

	usages := append([]VariableUsage(nil), c.VariableUsages(op)...)
	for _, fragment := range c.RecursivelyReferencedFragments(op) {
		usages = append(usages, c.VariableUsages(fragment)...)
	}

	if c.recursiveVariableUsages == nil {
		c.recursiveVariableUsages = map[*language.OperationDefinition][]VariableUsage{}
	}
	c.recursiveVariableUsages[op] = usages
	return usages
}

// findVariableDefinition returns the declaration of a variable among those
// given, or nil if none of them declares it.
func findVariableDefinition(definitions []*language.VariableDefinition, v *language.Variable) *language.VariableDefinition {
	if v == nil || v.Name == nil {
		return nil
	}
	for _, def := range definitions {
		if def != nil && def.Variable != nil && def.Variable.Name != nil && def.Variable.Name.Value == v.Name.Value {
			return def
		}
	}
	return nil
}
