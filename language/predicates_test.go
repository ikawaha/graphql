package language

import "testing"

func TestPredicates_Classification(t *testing.T) {
	// Each predicate is listed with the nodes it must accept. Every other node
	// in allNodes must be rejected, which is what makes this table a complete
	// statement of the classification rather than a set of examples.
	tests := []struct {
		name      string
		predicate func(Node) bool
		accepts   []Kind
	}{
		{
			name:      "IsExecutableDefinition",
			predicate: IsExecutableDefinition,
			accepts:   []Kind{KindOperationDefinition, KindFragmentDefinition},
		},
		{
			name:      "IsSelection",
			predicate: IsSelection,
			accepts:   []Kind{KindField, KindFragmentSpread, KindInlineFragment},
		},
		{
			name:      "IsValue",
			predicate: IsValue,
			accepts: []Kind{
				KindVariable, KindIntValue, KindFloatValue, KindStringValue,
				KindBooleanValue, KindNullValue, KindEnumValue, KindListValue,
				KindObjectValue,
			},
		},
		{
			name:      "IsType",
			predicate: IsType,
			accepts:   []Kind{KindNamedType, KindListType, KindNonNullType},
		},
		{
			name:      "IsTypeDefinition",
			predicate: IsTypeDefinition,
			accepts: []Kind{
				KindScalarTypeDefinition, KindObjectTypeDefinition,
				KindInterfaceTypeDefinition, KindUnionTypeDefinition,
				KindEnumTypeDefinition, KindInputObjectTypeDefinition,
			},
		},
		{
			name:      "IsTypeSystemDefinition",
			predicate: IsTypeSystemDefinition,
			accepts: []Kind{
				KindSchemaDefinition, KindDirectiveDefinition,
				KindScalarTypeDefinition, KindObjectTypeDefinition,
				KindInterfaceTypeDefinition, KindUnionTypeDefinition,
				KindEnumTypeDefinition, KindInputObjectTypeDefinition,
			},
		},
		{
			name:      "IsTypeExtension",
			predicate: IsTypeExtension,
			accepts: []Kind{
				KindScalarTypeExtension, KindObjectTypeExtension,
				KindInterfaceTypeExtension, KindUnionTypeExtension,
				KindEnumTypeExtension, KindInputObjectTypeExtension,
			},
		},
		{
			name:      "IsTypeSystemExtension",
			predicate: IsTypeSystemExtension,
			accepts: []Kind{
				KindSchemaExtension, KindDirectiveExtension,
				KindScalarTypeExtension, KindObjectTypeExtension,
				KindInterfaceTypeExtension, KindUnionTypeExtension,
				KindEnumTypeExtension, KindInputObjectTypeExtension,
			},
		},
		{
			name:      "IsSchemaCoordinate",
			predicate: IsSchemaCoordinate,
			accepts: []Kind{
				KindTypeCoordinate, KindMemberCoordinate, KindArgumentCoordinate,
				KindDirectiveCoordinate, KindDirectiveArgumentCoordinate,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accepted := make(map[Kind]bool, len(tt.accepts))
			for _, k := range tt.accepts {
				accepted[k] = true
			}
			for _, node := range allNodes() {
				want := accepted[node.Kind()]
				if got := tt.predicate(node); got != want {
					t.Errorf("%s(%v) = %v, want %v", tt.name, node.Kind(), got, want)
				}
			}
		})
	}
}

// A definition is anything that may stand at the top level, which is the union
// of the three groups.
func TestIsDefinition(t *testing.T) {
	for _, node := range allNodes() {
		want := IsExecutableDefinition(node) ||
			IsTypeSystemDefinition(node) ||
			IsTypeSystemExtension(node)
		if got := IsDefinition(node); got != want {
			t.Errorf("IsDefinition(%v) = %v, want %v", node.Kind(), got, want)
		}
	}
}

func TestIsSubscriptionOperation(t *testing.T) {
	tests := []struct {
		body string
		want bool
	}{
		{"subscription S { a }", true},
		{"query Q { a }", false},
		{"mutation M { a }", false},
		{"{ a }", false},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			def := mustParse(t, tt.body).Definitions[0]
			if got := IsSubscriptionOperation(def); got != tt.want {
				t.Errorf("IsSubscriptionOperation() = %v, want %v", got, tt.want)
			}
		})
	}
	if IsSubscriptionOperation(&Name{}) {
		t.Error("IsSubscriptionOperation(Name) = true, want false")
	}
}

// A value is constant when nothing inside it references a variable. Mixed
// lists and objects are the interesting cases: graphql-js reports those as
// constant because its check asks whether any element is constant rather than
// all of them.
func TestIsConstValue(t *testing.T) {
	tests := []struct {
		body string
		want bool
	}{
		{"1", true},
		{"null", true},
		{`"s"`, true},
		{"ENUM", true},
		{"[]", true},
		{"{}", true},
		{"[1, 2]", true},
		{"{a: 1, b: [2]}", true},
		{"$var", false},
		{"[$var]", false},
		{"[$var, 1]", false},
		{"[1, $var]", false},
		{"{a: $var}", false},
		{"{a: $var, b: 1}", false},
		{"{a: 1, b: $var}", false},
		{"[[{a: $var}]]", false},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			v, err := ParseValue(NewSource(tt.body))
			if err != nil {
				t.Fatalf("ParseValue(%q): %v", tt.body, err)
			}
			if got := IsConstValue(v); got != tt.want {
				t.Errorf("IsConstValue(%s) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}

	// A node that is not a value at all is not a constant value either.
	if IsConstValue(&Name{Value: "x"}) {
		t.Error("IsConstValue(Name) = true, want false")
	}
}
