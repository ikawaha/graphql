package validation

// RecommendedRules are not required by the specification but are strongly
// encouraged, and are part of [SpecifiedRules]. graphql-js keeps the same
// list under the same name.
var RecommendedRules = []Rule{
	MaxIntrospectionDepthRule,
}

// SpecifiedRules are the rules the specification requires of an executable
// document, followed by [RecommendedRules]. A document that passes them all
// may be executed.
//
// The order is the order errors come out in, and is graphql-js's: two rules
// that both have something to say about the same place say it in this order.
var SpecifiedRules = append([]Rule{
	ExecutableDefinitionsRule,
	KnownOperationTypesRule,
	UniqueOperationNamesRule,
	LoneAnonymousOperationRule,
	SingleFieldSubscriptionsRule,
	KnownTypeNamesRule,
	FragmentsOnCompositeTypesRule,
	VariablesAreInputTypesRule,
	ScalarLeafsRule,
	FieldsOnCorrectTypeRule,
	UniqueFragmentNamesRule,
	KnownFragmentNamesRule,
	NoUnusedFragmentsRule,
	PossibleFragmentSpreadsRule,
	NoFragmentCyclesRule,
	UniqueVariableNamesRule,
	NoUndefinedVariablesRule,
	NoUnusedVariablesRule,
	KnownDirectivesRule,
	UniqueDirectivesPerLocationRule,
	DeferStreamDirectiveOnRootFieldRule,
	DeferStreamDirectiveOnValidOperationsRule,
	DeferStreamDirectiveLabelRule,
	StreamDirectiveOnListFieldRule,
	KnownArgumentNamesRule,
	UniqueArgumentNamesRule,
	ValuesOfCorrectTypeRule,
	ProvidedRequiredArgumentsRule,
	VariablesInAllowedPositionRule,
	OverlappingFieldsCanBeMergedRule,
	UniqueInputFieldNamesRule,
}, RecommendedRules...)

// SpecifiedSDLRules are the rules the specification requires of a document of
// type definitions, which is what a schema is built from.
//
// Several are shared with [SpecifiedRules]: a type name is a type name whether
// it is written in a query or in a definition.
var SpecifiedSDLRules = []Rule{
	LoneSchemaDefinitionRule,
	UniqueOperationTypesRule,
	UniqueTypeNamesRule,
	UniqueEnumValueNamesRule,
	UniqueFieldDefinitionNamesRule,
	UniqueArgumentDefinitionNamesRule,
	UniqueDirectiveNamesRule,
	KnownTypeNamesRule,
	KnownDirectivesRule,
	UniqueDirectivesPerLocationRule,
	PossibleTypeExtensionsRule,
	KnownArgumentNamesOnDirectivesRule,
	UniqueArgumentNamesRule,
	UniqueInputFieldNamesRule,
	ProvidedRequiredArgumentsOnDirectivesRule,
}
