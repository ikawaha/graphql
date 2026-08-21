package language

// ParseOption configures a parse.
type ParseOption func(*parseOptions)

type parseOptions struct {
	noLocation                    bool
	maxTokens                     int
	hasMaxTokens                  bool
	experimentalFragmentArguments bool
}

// NoLocation drops source locations from the nodes that are produced.
//
// Locations are what let errors point at the offending part of a document, so
// this is for callers that only need the shape of a document and want to save
// the memory.
func NoLocation() ParseOption {
	return func(o *parseOptions) { o.noLocation = true }
}

// MaxTokens caps how many tokens a document may contain.
//
// Parsing costs time and memory in proportion to the number of tokens, and it
// happens before validation, so an invalid document can still be expensive.
// A server accepting documents from untrusted callers should set a limit.
func MaxTokens(n int) ParseOption {
	return func(o *parseOptions) { o.maxTokens, o.hasMaxTokens = n, true }
}

// ExperimentalFragmentArguments enables variable definitions on fragment
// definitions and arguments on fragment spreads.
//
// This is not part of the specification yet. Definitions parse into the
// VariableDefinitions field of [FragmentDefinition] and spread arguments into
// the Arguments field of [FragmentSpread].
func ExperimentalFragmentArguments() ParseOption {
	return func(o *parseOptions) { o.experimentalFragmentArguments = true }
}

// Parse reads a GraphQL document.
func Parse(source *Source, opts ...ParseOption) (*Document, error) {
	return withParser(source, opts, func(p *parser) *Document {
		return p.parseDocument()
	})
}

// ParseString reads a GraphQL document from a string, naming the source
// "GraphQL request".
func ParseString(body string, opts ...ParseOption) (*Document, error) {
	return Parse(NewSource(body), opts...)
}

// ParseValue reads a single GraphQL value, such as "[42]".
//
// It is meant for tools that work with values on their own, outside a
// document. The whole input must be one value.
func ParseValue(source *Source, opts ...ParseOption) (Value, error) {
	return withParser(source, opts, func(p *parser) Value {
		p.expect(TokenSOF)
		v := p.parseValueLiteral(false)
		p.expect(TokenEOF)
		return v
	})
}

// ParseConstValue reads a single GraphQL value that may not contain variables.
func ParseConstValue(source *Source, opts ...ParseOption) (Value, error) {
	return withParser(source, opts, func(p *parser) Value {
		p.expect(TokenSOF)
		v := p.parseValueLiteral(true)
		p.expect(TokenEOF)
		return v
	})
}

// ParseType reads a single GraphQL type reference, such as "[Int!]!".
func ParseType(source *Source, opts ...ParseOption) (Type, error) {
	return withParser(source, opts, func(p *parser) Type {
		p.expect(TokenSOF)
		t := p.parseTypeReference()
		p.expect(TokenEOF)
		return t
	})
}

// ParseSchemaCoordinate reads a single schema coordinate, such as
// "Query.field(arg:)".
func ParseSchemaCoordinate(source *Source, opts ...ParseOption) (SchemaCoordinate, error) {
	return withParser(source, opts, func(p *parser) SchemaCoordinate {
		// A coordinate is written in its own restricted grammar, so it needs
		// the lexer that reads that grammar rather than the document one.
		p.lexer = newSchemaCoordinateLexer(source)
		p.expect(TokenSOF)
		c := p.parseSchemaCoordinate()
		p.expect(TokenEOF)
		return c
	})
}

// parser holds the state of one parse.
//
// The parse functions do not return errors. They report a problem by panicking
// with a bailout value, which withParser recovers and turns into an error.
// A recursive descent parser has a failure path through almost every function,
// and threading an error return through all of them would bury the grammar;
// the standard library's go/parser takes the same approach.
type parser struct {
	lexer      *Lexer
	opts       parseOptions
	tokenCount int
}

// bailout carries a syntax error up to withParser.
type bailout struct{ err error }

// withParser runs a parse function and converts a bailout into an error.
func withParser[T Node](source *Source, opts []ParseOption, parse func(*parser) T) (result T, err error) {
	p := &parser{lexer: NewLexer(source)}
	for _, opt := range opts {
		opt(&p.opts)
	}
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		b, ok := r.(bailout)
		if !ok {
			panic(r)
		}
		var zero T
		result, err = zero, b.err
	}()
	return parse(p), nil
}

// fail reports a syntax error at a byte offset and unwinds the parse.
func (p *parser) fail(pos int, format string, args ...any) {
	panic(bailout{newSyntaxError(p.lexer.Source, pos, format, args...)})
}

// bail unwinds the parse with an error the lexer produced.
func (p *parser) bail(err error) {
	panic(bailout{err})
}

// token returns the token the parser is looking at.
func (p *parser) token() *Token { return p.lexer.Token }

// advance consumes the current token, enforcing the token limit.
func (p *parser) advance() {
	tok, err := p.lexer.Advance()
	if err != nil {
		p.bail(err)
	}
	if tok.Kind == TokenEOF {
		return
	}
	p.tokenCount++
	if p.opts.hasMaxTokens && p.tokenCount > p.opts.maxTokens {
		p.fail(tok.Start, "Document contains more than %d tokens. Parsing aborted.", p.opts.maxTokens)
	}
}

// lookahead returns the token after the current one without consuming it.
func (p *parser) lookahead() *Token {
	tok, err := p.lexer.Lookahead()
	if err != nil {
		p.bail(err)
	}
	return tok
}

// peek reports whether the current token is of the given kind.
func (p *parser) peek(kind TokenKind) bool { return p.token().Kind == kind }

// expect consumes and returns the current token, which must be of the given
// kind.
func (p *parser) expect(kind TokenKind) *Token {
	tok := p.token()
	if tok.Kind != kind {
		p.fail(tok.Start, "Expected %s, found %s.", tokenKindDesc(kind), tokenDesc(tok))
	}
	p.advance()
	return tok
}

// skip consumes the current token if it is of the given kind, reporting
// whether it did.
func (p *parser) skip(kind TokenKind) bool {
	if p.token().Kind != kind {
		return false
	}
	p.advance()
	return true
}

// expectKeyword consumes the current token, which must be the given name.
func (p *parser) expectKeyword(word string) {
	tok := p.token()
	if tok.Kind != TokenName || tok.Value != word {
		p.fail(tok.Start, "Expected %q, found %s.", word, tokenDesc(tok))
	}
	p.advance()
}

// skipKeyword consumes the current token if it is the given name, reporting
// whether it did.
func (p *parser) skipKeyword(word string) bool {
	tok := p.token()
	if tok.Kind != TokenName || tok.Value != word {
		return false
	}
	p.advance()
	return true
}

// unexpected reports that a token has no place here.
func (p *parser) unexpected(tok *Token) {
	if tok == nil {
		tok = p.token()
	}
	p.fail(tok.Start, "Unexpected %s.", tokenDesc(tok))
}

// loc returns the location spanning from start to the last consumed token, or
// nil when locations are turned off.
//
// Call this only after the node's children have been parsed, so that the last
// consumed token really is the node's last token.
func (p *parser) loc(start *Token) *Location {
	if p.opts.noLocation {
		return nil
	}
	return newLocation(start, p.lexer.LastToken, p.lexer.Source)
}

// tokenDesc describes a token for an error message.
func tokenDesc(t *Token) string {
	d := tokenKindDesc(t.Kind)
	if tokenKindHasValue(t.Kind) {
		d += ` "` + t.Value + `"`
	}
	return d
}

// tokenKindDesc describes a token kind for an error message, quoting
// punctuation so that it reads as the character it is.
func tokenKindDesc(k TokenKind) string {
	if IsPunctuatorToken(k) {
		return `"` + string(k) + `"`
	}
	return string(k)
}

// tokenKindHasValue reports whether tokens of this kind carry a value. It is
// keyed on the kind rather than on the value being empty, because an empty
// string literal is a token with a value that happens to be empty.
func tokenKindHasValue(k TokenKind) bool {
	switch k {
	case TokenName, TokenInt, TokenFloat, TokenString, TokenBlockString, TokenComment:
		return true
	default:
		return false
	}
}

// many parses one or more items between an opening and a closing token.
func many[T any](p *parser, open TokenKind, parse func() T, close TokenKind) []T {
	p.expect(open)
	var items []T
	for {
		items = append(items, parse())
		if p.skip(close) {
			return items
		}
	}
}

// anyOf parses zero or more items between an opening and a closing token.
func anyOf[T any](p *parser, open TokenKind, parse func() T, close TokenKind) []T {
	p.expect(open)
	var items []T
	for !p.skip(close) {
		items = append(items, parse())
	}
	return items
}

// optionalMany parses a bracketed list that may be absent altogether. A nil
// result means the opening token was not there, which is different from an
// empty list.
func optionalMany[T any](p *parser, open TokenKind, parse func() T, close TokenKind) []T {
	if !p.skip(open) {
		return nil
	}
	items := []T{}
	for {
		items = append(items, parse())
		if p.skip(close) {
			return items
		}
	}
}

// delimitedMany parses one or more items separated by a delimiter, which may
// also appear before the first item.
func delimitedMany[T any](p *parser, delimiter TokenKind, parse func() T) []T {
	p.skip(delimiter)
	var items []T
	for {
		items = append(items, parse())
		if !p.skip(delimiter) {
			return items
		}
	}
}

// parseName parses an identifier.
func (p *parser) parseName() *Name {
	tok := p.expect(TokenName)
	return &Name{Loc: p.loc(tok), Value: tok.Value}
}

// parseDocument parses a whole document.
func (p *parser) parseDocument() *Document {
	start := p.token()
	definitions := many(p, TokenSOF, p.parseDefinition, TokenEOF)
	return &Document{
		Loc:         p.loc(start),
		Definitions: definitions,
		TokenCount:  p.tokenCount,
	}
}

// parseDefinition parses one top-level definition.
func (p *parser) parseDefinition() Definition {
	if p.peek(TokenBraceL) {
		return p.parseOperationDefinition()
	}

	// Most definitions may be preceded by a description, so deciding which one
	// this is can need a look past the description.
	hasDescription := p.peekDescription()
	keyword := p.token()
	if hasDescription {
		keyword = p.lookahead()
	}

	if hasDescription && keyword.Kind == TokenBraceL {
		p.fail(p.token().Start,
			"Unexpected description, descriptions are not supported on shorthand queries.")
	}

	if keyword.Kind == TokenName {
		switch keyword.Value {
		case "schema":
			return p.parseSchemaDefinition()
		case "scalar":
			return p.parseScalarTypeDefinition()
		case "type":
			return p.parseObjectTypeDefinition()
		case "interface":
			return p.parseInterfaceTypeDefinition()
		case "union":
			return p.parseUnionTypeDefinition()
		case "enum":
			return p.parseEnumTypeDefinition()
		case "input":
			return p.parseInputObjectTypeDefinition()
		case "directive":
			return p.parseDirectiveDefinition()
		case "query", "mutation", "subscription":
			return p.parseOperationDefinition()
		case "fragment":
			return p.parseFragmentDefinition()
		}
		if hasDescription {
			p.fail(p.token().Start,
				"Unexpected description, only GraphQL definitions support descriptions.")
		}
		if keyword.Value == "extend" {
			return p.parseTypeSystemExtension()
		}
	}

	p.unexpected(keyword)
	return nil
}

// parseOperationDefinition parses an operation, in either the shorthand form
// that is just a selection set or the full form.
func (p *parser) parseOperationDefinition() *OperationDefinition {
	start := p.token()
	if p.peek(TokenBraceL) {
		selectionSet := p.parseSelectionSet()
		return &OperationDefinition{
			Loc:          p.loc(start),
			Operation:    OperationQuery,
			SelectionSet: selectionSet,
		}
	}
	description := p.parseDescription()
	operation := p.parseOperationType()
	var name *Name
	if p.peek(TokenName) {
		name = p.parseName()
	}
	variableDefinitions := p.parseVariableDefinitions()
	directives := p.parseDirectives(false)
	selectionSet := p.parseSelectionSet()
	return &OperationDefinition{
		Loc:                 p.loc(start),
		Operation:           operation,
		Description:         description,
		Name:                name,
		VariableDefinitions: variableDefinitions,
		Directives:          directives,
		SelectionSet:        selectionSet,
	}
}

// parseOperationType parses the query, mutation or subscription keyword.
func (p *parser) parseOperationType() OperationType {
	tok := p.expect(TokenName)
	switch tok.Value {
	case "query":
		return OperationQuery
	case "mutation":
		return OperationMutation
	case "subscription":
		return OperationSubscription
	}
	p.unexpected(tok)
	return ""
}

// parseVariableDefinitions parses a parenthesised list of variable
// definitions, which may be absent.
func (p *parser) parseVariableDefinitions() []*VariableDefinition {
	return optionalMany(p, TokenParenL, p.parseVariableDefinition, TokenParenR)
}

// parseVariableDefinition parses one variable declaration.
func (p *parser) parseVariableDefinition() *VariableDefinition {
	start := p.token()
	description := p.parseDescription()
	variable := p.parseVariable()
	p.expect(TokenColon)
	typ := p.parseTypeReference()
	var defaultValue Value
	if p.skip(TokenEquals) {
		defaultValue = p.parseValueLiteral(true)
	}
	directives := p.parseDirectives(true)
	return &VariableDefinition{
		Loc:          p.loc(start),
		Description:  description,
		Variable:     variable,
		Type:         typ,
		DefaultValue: defaultValue,
		Directives:   directives,
	}
}

// parseVariable parses a variable reference.
func (p *parser) parseVariable() *Variable {
	start := p.token()
	p.expect(TokenDollar)
	name := p.parseName()
	return &Variable{Loc: p.loc(start), Name: name}
}

// parseSelectionSet parses a braced list of selections.
func (p *parser) parseSelectionSet() *SelectionSet {
	start := p.token()
	selections := many(p, TokenBraceL, p.parseSelection, TokenBraceR)
	return &SelectionSet{Loc: p.loc(start), Selections: selections}
}

// parseSelection parses one selection, which is a field or a fragment.
func (p *parser) parseSelection() Selection {
	if p.peek(TokenSpread) {
		return p.parseFragment()
	}
	return p.parseField()
}

// parseField parses a field selection.
func (p *parser) parseField() *Field {
	start := p.token()
	nameOrAlias := p.parseName()

	var alias, name *Name
	if p.skip(TokenColon) {
		alias, name = nameOrAlias, p.parseName()
	} else {
		name = nameOrAlias
	}

	arguments := p.parseArguments(false)
	directives := p.parseDirectives(false)
	var selectionSet *SelectionSet
	if p.peek(TokenBraceL) {
		selectionSet = p.parseSelectionSet()
	}
	return &Field{
		Loc:          p.loc(start),
		Alias:        alias,
		Name:         name,
		Arguments:    arguments,
		Directives:   directives,
		SelectionSet: selectionSet,
	}
}

// parseArguments parses a parenthesised argument list, which may be absent.
func (p *parser) parseArguments(isConst bool) []*Argument {
	parse := func() *Argument { return p.parseArgument(isConst) }
	return optionalMany(p, TokenParenL, parse, TokenParenR)
}

// parseArgument parses one argument.
func (p *parser) parseArgument(isConst bool) *Argument {
	start := p.token()
	name := p.parseName()
	p.expect(TokenColon)
	value := p.parseValueLiteral(isConst)
	return &Argument{Loc: p.loc(start), Name: name, Value: value}
}

// parseFragmentArguments parses the arguments of a fragment spread.
func (p *parser) parseFragmentArguments() []*FragmentArgument {
	return optionalMany(p, TokenParenL, p.parseFragmentArgument, TokenParenR)
}

// parseFragmentArgument parses one argument of a fragment spread.
func (p *parser) parseFragmentArgument() *FragmentArgument {
	start := p.token()
	name := p.parseName()
	p.expect(TokenColon)
	value := p.parseValueLiteral(false)
	return &FragmentArgument{Loc: p.loc(start), Name: name, Value: value}
}

// parseFragment parses either a fragment spread or an inline fragment, which
// are told apart by what follows the spread punctuation.
func (p *parser) parseFragment() Selection {
	start := p.token()
	p.expect(TokenSpread)

	hasTypeCondition := p.skipKeyword("on")
	if !hasTypeCondition && p.peek(TokenName) {
		name := p.parseFragmentName()
		var arguments []*FragmentArgument
		if p.peek(TokenParenL) && p.opts.experimentalFragmentArguments {
			arguments = p.parseFragmentArguments()
		}
		directives := p.parseDirectives(false)
		return &FragmentSpread{
			Loc:        p.loc(start),
			Name:       name,
			Arguments:  arguments,
			Directives: directives,
		}
	}

	var typeCondition *NamedType
	if hasTypeCondition {
		typeCondition = p.parseNamedType()
	}
	directives := p.parseDirectives(false)
	selectionSet := p.parseSelectionSet()
	return &InlineFragment{
		Loc:           p.loc(start),
		TypeCondition: typeCondition,
		Directives:    directives,
		SelectionSet:  selectionSet,
	}
}

// parseFragmentDefinition parses a named fragment definition.
func (p *parser) parseFragmentDefinition() *FragmentDefinition {
	start := p.token()
	description := p.parseDescription()
	p.expectKeyword("fragment")
	name := p.parseFragmentName()
	var variableDefinitions []*VariableDefinition
	if p.opts.experimentalFragmentArguments {
		variableDefinitions = p.parseVariableDefinitions()
	}
	p.expectKeyword("on")
	typeCondition := p.parseNamedType()
	directives := p.parseDirectives(false)
	selectionSet := p.parseSelectionSet()
	return &FragmentDefinition{
		Loc:                 p.loc(start),
		Description:         description,
		Name:                name,
		VariableDefinitions: variableDefinitions,
		TypeCondition:       typeCondition,
		Directives:          directives,
		SelectionSet:        selectionSet,
	}
}

// parseFragmentName parses a fragment's name, which may not be "on" because
// that would be ambiguous with an inline fragment's type condition.
func (p *parser) parseFragmentName() *Name {
	if p.token().Value == "on" {
		p.unexpected(nil)
	}
	return p.parseName()
}

// parseValueLiteral parses a value. When isConst is set, a variable reference
// is rejected, which is what the grammar requires of a default value or of an
// argument to a directive in a schema.
func (p *parser) parseValueLiteral(isConst bool) Value {
	tok := p.token()
	switch tok.Kind {
	case TokenBracketL:
		return p.parseListValue(isConst)
	case TokenBraceL:
		return p.parseObjectValue(isConst)
	case TokenInt:
		p.advance()
		return &IntValue{Loc: p.loc(tok), Value: tok.Value}
	case TokenFloat:
		p.advance()
		return &FloatValue{Loc: p.loc(tok), Value: tok.Value}
	case TokenString, TokenBlockString:
		return p.parseStringLiteral()
	case TokenName:
		p.advance()
		switch tok.Value {
		case "true":
			return &BooleanValue{Loc: p.loc(tok), Value: true}
		case "false":
			return &BooleanValue{Loc: p.loc(tok), Value: false}
		case "null":
			return &NullValue{Loc: p.loc(tok)}
		default:
			return &EnumValue{Loc: p.loc(tok), Value: tok.Value}
		}
	case TokenDollar:
		if isConst {
			p.expect(TokenDollar)
			if p.token().Kind == TokenName {
				p.fail(tok.Start, "Unexpected variable %q in constant value.", "$"+p.token().Value)
			}
			p.unexpected(tok)
		}
		return p.parseVariable()
	}
	p.unexpected(nil)
	return nil
}

// parseStringLiteral parses a string, remembering whether it was written in
// the block form.
func (p *parser) parseStringLiteral() *StringValue {
	tok := p.token()
	p.advance()
	return &StringValue{
		Loc:   p.loc(tok),
		Value: tok.Value,
		Block: tok.Kind == TokenBlockString,
	}
}

// parseListValue parses a bracketed list of values.
func (p *parser) parseListValue(isConst bool) *ListValue {
	start := p.token()
	parse := func() Value { return p.parseValueLiteral(isConst) }
	values := anyOf(p, TokenBracketL, parse, TokenBracketR)
	return &ListValue{Loc: p.loc(start), Values: values}
}

// parseObjectValue parses a braced list of input object fields.
func (p *parser) parseObjectValue(isConst bool) *ObjectValue {
	start := p.token()
	parse := func() *ObjectField { return p.parseObjectField(isConst) }
	fields := anyOf(p, TokenBraceL, parse, TokenBraceR)
	return &ObjectValue{Loc: p.loc(start), Fields: fields}
}

// parseObjectField parses one field of an object value.
func (p *parser) parseObjectField(isConst bool) *ObjectField {
	start := p.token()
	name := p.parseName()
	p.expect(TokenColon)
	value := p.parseValueLiteral(isConst)
	return &ObjectField{Loc: p.loc(start), Name: name, Value: value}
}

// parseDirectives parses however many directives follow.
func (p *parser) parseDirectives(isConst bool) []*Directive {
	var directives []*Directive
	for p.peek(TokenAt) {
		directives = append(directives, p.parseDirective(isConst))
	}
	return directives
}

// parseConstDirectives parses directives whose arguments may not contain
// variables.
func (p *parser) parseConstDirectives() []*Directive {
	return p.parseDirectives(true)
}

// parseDirective parses one directive application.
func (p *parser) parseDirective(isConst bool) *Directive {
	start := p.token()
	p.expect(TokenAt)
	name := p.parseName()
	arguments := p.parseArguments(isConst)
	return &Directive{Loc: p.loc(start), Name: name, Arguments: arguments}
}

// parseTypeReference parses a type, including any list and non-null wrappers.
func (p *parser) parseTypeReference() Type {
	start := p.token()
	var typ Type
	if p.skip(TokenBracketL) {
		inner := p.parseTypeReference()
		p.expect(TokenBracketR)
		typ = &ListType{Loc: p.loc(start), Type: inner}
	} else {
		typ = p.parseNamedType()
	}
	if p.skip(TokenBang) {
		return &NonNullType{Loc: p.loc(start), Type: typ}
	}
	return typ
}

// parseNamedType parses a reference to a type by name.
func (p *parser) parseNamedType() *NamedType {
	start := p.token()
	name := p.parseName()
	return &NamedType{Loc: p.loc(start), Name: name}
}

// parseSchemaCoordinate parses a coordinate naming one element of a schema.
func (p *parser) parseSchemaCoordinate() SchemaCoordinate {
	start := p.token()
	ofDirective := p.skip(TokenAt)
	name := p.parseName()

	var memberName *Name
	if !ofDirective && p.skip(TokenDot) {
		memberName = p.parseName()
	}

	var argumentName *Name
	if (ofDirective || memberName != nil) && p.skip(TokenParenL) {
		argumentName = p.parseName()
		p.expect(TokenColon)
		p.expect(TokenParenR)
	}

	switch {
	case ofDirective && argumentName != nil:
		return &DirectiveArgumentCoordinate{Loc: p.loc(start), Name: name, ArgumentName: argumentName}
	case ofDirective:
		return &DirectiveCoordinate{Loc: p.loc(start), Name: name}
	case memberName != nil && argumentName != nil:
		return &ArgumentCoordinate{
			Loc: p.loc(start), Name: name, FieldName: memberName, ArgumentName: argumentName,
		}
	case memberName != nil:
		return &MemberCoordinate{Loc: p.loc(start), Name: name, MemberName: memberName}
	default:
		return &TypeCoordinate{Loc: p.loc(start), Name: name}
	}
}
