package language

// peekDescription reports whether the current token could be a description,
// which is a string literal written before a definition.
func (p *parser) peekDescription() bool {
	return p.peek(TokenString) || p.peek(TokenBlockString)
}

// parseDescription parses a description if one is present.
func (p *parser) parseDescription() *StringValue {
	if p.peekDescription() {
		return p.parseStringLiteral()
	}
	return nil
}

// parseSchemaDefinition parses a schema definition.
func (p *parser) parseSchemaDefinition() *SchemaDefinition {
	start := p.token()
	description := p.parseDescription()
	p.expectKeyword("schema")
	directives := p.parseConstDirectives()
	operationTypes := many(p, TokenBraceL, p.parseOperationTypeDefinition, TokenBraceR)
	return &SchemaDefinition{
		Loc:            p.loc(start),
		Description:    description,
		Directives:     directives,
		OperationTypes: operationTypes,
	}
}

// parseOperationTypeDefinition binds one operation type to a named type.
func (p *parser) parseOperationTypeDefinition() *OperationTypeDefinition {
	start := p.token()
	operation := p.parseOperationType()
	p.expect(TokenColon)
	typ := p.parseNamedType()
	return &OperationTypeDefinition{Loc: p.loc(start), Operation: operation, Type: typ}
}

// parseScalarTypeDefinition parses a scalar type definition.
func (p *parser) parseScalarTypeDefinition() *ScalarTypeDefinition {
	start := p.token()
	description := p.parseDescription()
	p.expectKeyword("scalar")
	name := p.parseName()
	directives := p.parseConstDirectives()
	return &ScalarTypeDefinition{
		Loc: p.loc(start), Description: description, Name: name, Directives: directives,
	}
}

// parseObjectTypeDefinition parses an object type definition.
func (p *parser) parseObjectTypeDefinition() *ObjectTypeDefinition {
	start := p.token()
	description := p.parseDescription()
	p.expectKeyword("type")
	name := p.parseName()
	interfaces := p.parseImplementsInterfaces()
	directives := p.parseConstDirectives()
	fields := p.parseFieldsDefinition()
	return &ObjectTypeDefinition{
		Loc:         p.loc(start),
		Description: description,
		Name:        name,
		Interfaces:  interfaces,
		Directives:  directives,
		Fields:      fields,
	}
}

// parseImplementsInterfaces parses an implements clause, which may be absent.
func (p *parser) parseImplementsInterfaces() []*NamedType {
	if !p.skipKeyword("implements") {
		return nil
	}
	return delimitedMany(p, TokenAmp, p.parseNamedType)
}

// parseFieldsDefinition parses a braced list of field definitions, which may
// be absent.
func (p *parser) parseFieldsDefinition() []*FieldDefinition {
	return optionalMany(p, TokenBraceL, p.parseFieldDefinition, TokenBraceR)
}

// parseFieldDefinition parses one field definition.
func (p *parser) parseFieldDefinition() *FieldDefinition {
	start := p.token()
	description := p.parseDescription()
	name := p.parseName()
	args := p.parseArgumentDefs()
	p.expect(TokenColon)
	typ := p.parseTypeReference()
	directives := p.parseConstDirectives()
	return &FieldDefinition{
		Loc:         p.loc(start),
		Description: description,
		Name:        name,
		Arguments:   args,
		Type:        typ,
		Directives:  directives,
	}
}

// parseArgumentDefs parses a parenthesised list of argument definitions, which
// may be absent.
func (p *parser) parseArgumentDefs() []*InputValueDefinition {
	return optionalMany(p, TokenParenL, p.parseInputValueDef, TokenParenR)
}

// parseInputValueDef parses an argument or input object field definition.
func (p *parser) parseInputValueDef() *InputValueDefinition {
	start := p.token()
	description := p.parseDescription()
	name := p.parseName()
	p.expect(TokenColon)
	typ := p.parseTypeReference()
	var defaultValue Value
	if p.skip(TokenEquals) {
		defaultValue = p.parseValueLiteral(true)
	}
	directives := p.parseConstDirectives()
	return &InputValueDefinition{
		Loc:          p.loc(start),
		Description:  description,
		Name:         name,
		Type:         typ,
		DefaultValue: defaultValue,
		Directives:   directives,
	}
}

// parseInterfaceTypeDefinition parses an interface type definition.
func (p *parser) parseInterfaceTypeDefinition() *InterfaceTypeDefinition {
	start := p.token()
	description := p.parseDescription()
	p.expectKeyword("interface")
	name := p.parseName()
	interfaces := p.parseImplementsInterfaces()
	directives := p.parseConstDirectives()
	fields := p.parseFieldsDefinition()
	return &InterfaceTypeDefinition{
		Loc:         p.loc(start),
		Description: description,
		Name:        name,
		Interfaces:  interfaces,
		Directives:  directives,
		Fields:      fields,
	}
}

// parseUnionTypeDefinition parses a union type definition.
func (p *parser) parseUnionTypeDefinition() *UnionTypeDefinition {
	start := p.token()
	description := p.parseDescription()
	p.expectKeyword("union")
	name := p.parseName()
	directives := p.parseConstDirectives()
	types := p.parseUnionMemberTypes()
	return &UnionTypeDefinition{
		Loc:         p.loc(start),
		Description: description,
		Name:        name,
		Directives:  directives,
		Types:       types,
	}
}

// parseUnionMemberTypes parses the member list of a union, which may be
// absent.
func (p *parser) parseUnionMemberTypes() []*NamedType {
	if !p.skip(TokenEquals) {
		return nil
	}
	return delimitedMany(p, TokenPipe, p.parseNamedType)
}

// parseEnumTypeDefinition parses an enum type definition.
func (p *parser) parseEnumTypeDefinition() *EnumTypeDefinition {
	start := p.token()
	description := p.parseDescription()
	p.expectKeyword("enum")
	name := p.parseName()
	directives := p.parseConstDirectives()
	values := p.parseEnumValuesDefinition()
	return &EnumTypeDefinition{
		Loc:         p.loc(start),
		Description: description,
		Name:        name,
		Directives:  directives,
		Values:      values,
	}
}

// parseEnumValuesDefinition parses a braced list of enum members, which may be
// absent.
func (p *parser) parseEnumValuesDefinition() []*EnumValueDefinition {
	return optionalMany(p, TokenBraceL, p.parseEnumValueDefinition, TokenBraceR)
}

// parseEnumValueDefinition parses one enum member.
func (p *parser) parseEnumValueDefinition() *EnumValueDefinition {
	start := p.token()
	description := p.parseDescription()
	name := p.parseEnumValueName()
	directives := p.parseConstDirectives()
	return &EnumValueDefinition{
		Loc: p.loc(start), Description: description, Name: name, Directives: directives,
	}
}

// parseEnumValueName parses an enum member's name, which may not be true,
// false or null because those spell other kinds of value.
func (p *parser) parseEnumValueName() *Name {
	tok := p.token()
	switch tok.Value {
	case "true", "false", "null":
		p.fail(tok.Start, "%s is reserved and cannot be used for an enum value.", tokenDesc(tok))
	}
	return p.parseName()
}

// parseInputObjectTypeDefinition parses an input object type definition.
func (p *parser) parseInputObjectTypeDefinition() *InputObjectTypeDefinition {
	start := p.token()
	description := p.parseDescription()
	p.expectKeyword("input")
	name := p.parseName()
	directives := p.parseConstDirectives()
	fields := p.parseInputFieldsDefinition()
	return &InputObjectTypeDefinition{
		Loc:         p.loc(start),
		Description: description,
		Name:        name,
		Directives:  directives,
		Fields:      fields,
	}
}

// parseInputFieldsDefinition parses a braced list of input fields, which may
// be absent.
func (p *parser) parseInputFieldsDefinition() []*InputValueDefinition {
	return optionalMany(p, TokenBraceL, p.parseInputValueDef, TokenBraceR)
}

// parseDirectiveDefinition parses a directive definition.
func (p *parser) parseDirectiveDefinition() *DirectiveDefinition {
	start := p.token()
	description := p.parseDescription()
	p.expectKeyword("directive")
	p.expect(TokenAt)
	name := p.parseName()
	args := p.parseArgumentDefs()
	directives := p.parseConstDirectives()
	repeatable := p.skipKeyword("repeatable")
	p.expectKeyword("on")
	locations := p.parseDirectiveLocations()
	return &DirectiveDefinition{
		Loc:         p.loc(start),
		Description: description,
		Name:        name,
		Arguments:   args,
		Directives:  directives,
		Repeatable:  repeatable,
		Locations:   locations,
	}
}

// parseDirectiveLocations parses the pipe-separated list of locations a
// directive may be applied in.
func (p *parser) parseDirectiveLocations() []*Name {
	return delimitedMany(p, TokenPipe, p.parseDirectiveLocation)
}

// parseDirectiveLocation parses one location name, which must be one the
// grammar recognises.
func (p *parser) parseDirectiveLocation() *Name {
	start := p.token()
	name := p.parseName()
	if IsDirectiveLocation(DirectiveLocation(name.Value)) {
		return name
	}
	p.unexpected(start)
	return nil
}

// parseTypeSystemExtension parses an extend declaration, dispatching on the
// keyword that follows.
func (p *parser) parseTypeSystemExtension() TypeSystemExtension {
	keyword := p.lookahead()
	if keyword.Kind == TokenName {
		switch keyword.Value {
		case "schema":
			return p.parseSchemaExtension()
		case "scalar":
			return p.parseScalarTypeExtension()
		case "type":
			return p.parseObjectTypeExtension()
		case "interface":
			return p.parseInterfaceTypeExtension()
		case "union":
			return p.parseUnionTypeExtension()
		case "enum":
			return p.parseEnumTypeExtension()
		case "input":
			return p.parseInputObjectTypeExtension()
		case "directive":
			return p.parseDirectiveExtension()
		}
	}
	p.unexpected(keyword)
	return nil
}

// An extension has to add something. Each of the parse functions below checks
// that at least one part was present, because "extend type Foo" on its own
// says nothing and is a mistake rather than a no-op.

// parseSchemaExtension parses a schema extension.
func (p *parser) parseSchemaExtension() *SchemaExtension {
	start := p.token()
	p.expectKeyword("extend")
	p.expectKeyword("schema")
	directives := p.parseConstDirectives()
	operationTypes := optionalMany(p, TokenBraceL, p.parseOperationTypeDefinition, TokenBraceR)
	if directives == nil && operationTypes == nil {
		p.unexpected(nil)
	}
	return &SchemaExtension{
		Loc: p.loc(start), Directives: directives, OperationTypes: operationTypes,
	}
}

// parseScalarTypeExtension parses a scalar type extension.
func (p *parser) parseScalarTypeExtension() *ScalarTypeExtension {
	start := p.token()
	p.expectKeyword("extend")
	p.expectKeyword("scalar")
	name := p.parseName()
	directives := p.parseConstDirectives()
	if directives == nil {
		p.unexpected(nil)
	}
	return &ScalarTypeExtension{Loc: p.loc(start), Name: name, Directives: directives}
}

// parseObjectTypeExtension parses an object type extension.
func (p *parser) parseObjectTypeExtension() *ObjectTypeExtension {
	start := p.token()
	p.expectKeyword("extend")
	p.expectKeyword("type")
	name := p.parseName()
	interfaces := p.parseImplementsInterfaces()
	directives := p.parseConstDirectives()
	fields := p.parseFieldsDefinition()
	if interfaces == nil && directives == nil && fields == nil {
		p.unexpected(nil)
	}
	return &ObjectTypeExtension{
		Loc:        p.loc(start),
		Name:       name,
		Interfaces: interfaces,
		Directives: directives,
		Fields:     fields,
	}
}

// parseInterfaceTypeExtension parses an interface type extension.
func (p *parser) parseInterfaceTypeExtension() *InterfaceTypeExtension {
	start := p.token()
	p.expectKeyword("extend")
	p.expectKeyword("interface")
	name := p.parseName()
	interfaces := p.parseImplementsInterfaces()
	directives := p.parseConstDirectives()
	fields := p.parseFieldsDefinition()
	if interfaces == nil && directives == nil && fields == nil {
		p.unexpected(nil)
	}
	return &InterfaceTypeExtension{
		Loc:        p.loc(start),
		Name:       name,
		Interfaces: interfaces,
		Directives: directives,
		Fields:     fields,
	}
}

// parseUnionTypeExtension parses a union type extension.
func (p *parser) parseUnionTypeExtension() *UnionTypeExtension {
	start := p.token()
	p.expectKeyword("extend")
	p.expectKeyword("union")
	name := p.parseName()
	directives := p.parseConstDirectives()
	types := p.parseUnionMemberTypes()
	if directives == nil && types == nil {
		p.unexpected(nil)
	}
	return &UnionTypeExtension{
		Loc: p.loc(start), Name: name, Directives: directives, Types: types,
	}
}

// parseEnumTypeExtension parses an enum type extension.
func (p *parser) parseEnumTypeExtension() *EnumTypeExtension {
	start := p.token()
	p.expectKeyword("extend")
	p.expectKeyword("enum")
	name := p.parseName()
	directives := p.parseConstDirectives()
	values := p.parseEnumValuesDefinition()
	if directives == nil && values == nil {
		p.unexpected(nil)
	}
	return &EnumTypeExtension{
		Loc: p.loc(start), Name: name, Directives: directives, Values: values,
	}
}

// parseInputObjectTypeExtension parses an input object type extension.
func (p *parser) parseInputObjectTypeExtension() *InputObjectTypeExtension {
	start := p.token()
	p.expectKeyword("extend")
	p.expectKeyword("input")
	name := p.parseName()
	directives := p.parseConstDirectives()
	fields := p.parseInputFieldsDefinition()
	if directives == nil && fields == nil {
		p.unexpected(nil)
	}
	return &InputObjectTypeExtension{
		Loc: p.loc(start), Name: name, Directives: directives, Fields: fields,
	}
}

// parseDirectiveExtension parses a directive extension.
func (p *parser) parseDirectiveExtension() *DirectiveExtension {
	start := p.token()
	p.expectKeyword("extend")
	p.expectKeyword("directive")
	p.expect(TokenAt)
	name := p.parseName()
	directives := p.parseConstDirectives()
	if directives == nil {
		p.unexpected(nil)
	}
	return &DirectiveExtension{Loc: p.loc(start), Name: name, Directives: directives}
}
