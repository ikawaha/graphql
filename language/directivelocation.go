package language

// DirectiveLocation names a place in a document or a schema where a directive
// may be applied. A directive definition lists the locations it allows.
type DirectiveLocation string

// Locations that appear in an executable document.
const (
	DirectiveLocationQuery                      DirectiveLocation = "QUERY"
	DirectiveLocationMutation                   DirectiveLocation = "MUTATION"
	DirectiveLocationSubscription               DirectiveLocation = "SUBSCRIPTION"
	DirectiveLocationField                      DirectiveLocation = "FIELD"
	DirectiveLocationFragmentDefinition         DirectiveLocation = "FRAGMENT_DEFINITION"
	DirectiveLocationFragmentSpread             DirectiveLocation = "FRAGMENT_SPREAD"
	DirectiveLocationInlineFragment             DirectiveLocation = "INLINE_FRAGMENT"
	DirectiveLocationVariableDefinition         DirectiveLocation = "VARIABLE_DEFINITION"
	DirectiveLocationFragmentVariableDefinition DirectiveLocation = "FRAGMENT_VARIABLE_DEFINITION"
)

// Locations that appear in a schema.
const (
	DirectiveLocationSchema               DirectiveLocation = "SCHEMA"
	DirectiveLocationScalar               DirectiveLocation = "SCALAR"
	DirectiveLocationObject               DirectiveLocation = "OBJECT"
	DirectiveLocationFieldDefinition      DirectiveLocation = "FIELD_DEFINITION"
	DirectiveLocationArgumentDefinition   DirectiveLocation = "ARGUMENT_DEFINITION"
	DirectiveLocationInterface            DirectiveLocation = "INTERFACE"
	DirectiveLocationUnion                DirectiveLocation = "UNION"
	DirectiveLocationEnum                 DirectiveLocation = "ENUM"
	DirectiveLocationEnumValue            DirectiveLocation = "ENUM_VALUE"
	DirectiveLocationInputObject          DirectiveLocation = "INPUT_OBJECT"
	DirectiveLocationInputFieldDefinition DirectiveLocation = "INPUT_FIELD_DEFINITION"
	DirectiveLocationDirectiveDefinition  DirectiveLocation = "DIRECTIVE_DEFINITION"
)

// String returns the location as it is spelled in a directive definition.
func (l DirectiveLocation) String() string { return string(l) }

// executableDirectiveLocations holds the locations valid in a document.
var executableDirectiveLocations = map[DirectiveLocation]bool{
	DirectiveLocationQuery:                      true,
	DirectiveLocationMutation:                   true,
	DirectiveLocationSubscription:               true,
	DirectiveLocationField:                      true,
	DirectiveLocationFragmentDefinition:         true,
	DirectiveLocationFragmentSpread:             true,
	DirectiveLocationInlineFragment:             true,
	DirectiveLocationVariableDefinition:         true,
	DirectiveLocationFragmentVariableDefinition: true,
}

// typeSystemDirectiveLocations holds the locations valid in a schema.
var typeSystemDirectiveLocations = map[DirectiveLocation]bool{
	DirectiveLocationSchema:               true,
	DirectiveLocationScalar:               true,
	DirectiveLocationObject:               true,
	DirectiveLocationFieldDefinition:      true,
	DirectiveLocationArgumentDefinition:   true,
	DirectiveLocationInterface:            true,
	DirectiveLocationUnion:                true,
	DirectiveLocationEnum:                 true,
	DirectiveLocationEnumValue:            true,
	DirectiveLocationInputObject:          true,
	DirectiveLocationInputFieldDefinition: true,
	DirectiveLocationDirectiveDefinition:  true,
}

// IsExecutableDirectiveLocation reports whether l names a place in an
// executable document.
func IsExecutableDirectiveLocation(l DirectiveLocation) bool {
	return executableDirectiveLocations[l]
}

// IsTypeSystemDirectiveLocation reports whether l names a place in a schema.
func IsTypeSystemDirectiveLocation(l DirectiveLocation) bool {
	return typeSystemDirectiveLocations[l]
}

// IsDirectiveLocation reports whether l is a location the grammar recognises.
func IsDirectiveLocation(l DirectiveLocation) bool {
	return IsExecutableDirectiveLocation(l) || IsTypeSystemDirectiveLocation(l)
}
