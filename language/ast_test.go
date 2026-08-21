package language

import (
	"reflect"
	"testing"
)

// allNodes returns one zero value of every node type in the package. Tests use
// it to check properties that must hold for all of them.
func allNodes() []Node {
	return []Node{
		&Name{},
		&Document{},
		&OperationDefinition{},
		&VariableDefinition{},
		&SelectionSet{},
		&Field{},
		&Argument{},
		&FragmentArgument{},
		&FragmentSpread{},
		&InlineFragment{},
		&FragmentDefinition{},
		&Variable{},
		&IntValue{},
		&FloatValue{},
		&StringValue{},
		&BooleanValue{},
		&NullValue{},
		&EnumValue{},
		&ListValue{},
		&ObjectValue{},
		&ObjectField{},
		&Directive{},
		&NamedType{},
		&ListType{},
		&NonNullType{},
		&SchemaDefinition{},
		&OperationTypeDefinition{},
		&ScalarTypeDefinition{},
		&ObjectTypeDefinition{},
		&FieldDefinition{},
		&InputValueDefinition{},
		&InterfaceTypeDefinition{},
		&UnionTypeDefinition{},
		&EnumTypeDefinition{},
		&EnumValueDefinition{},
		&InputObjectTypeDefinition{},
		&DirectiveDefinition{},
		&SchemaExtension{},
		&ScalarTypeExtension{},
		&ObjectTypeExtension{},
		&InterfaceTypeExtension{},
		&UnionTypeExtension{},
		&EnumTypeExtension{},
		&InputObjectTypeExtension{},
		&DirectiveExtension{},
		&TypeCoordinate{},
		&MemberCoordinate{},
		&ArgumentCoordinate{},
		&DirectiveCoordinate{},
		&DirectiveArgumentCoordinate{},
	}
}

// allKinds lists every kind the package declares.
func allKinds() []Kind {
	return []Kind{
		KindName, KindDocument, KindOperationDefinition, KindVariableDefinition,
		KindSelectionSet, KindField, KindArgument, KindFragmentArgument,
		KindFragmentSpread, KindInlineFragment, KindFragmentDefinition,
		KindVariable, KindIntValue, KindFloatValue, KindStringValue,
		KindBooleanValue, KindNullValue, KindEnumValue, KindListValue,
		KindObjectValue, KindObjectField, KindDirective, KindNamedType,
		KindListType, KindNonNullType, KindSchemaDefinition,
		KindOperationTypeDefinition, KindScalarTypeDefinition,
		KindObjectTypeDefinition, KindFieldDefinition, KindInputValueDefinition,
		KindInterfaceTypeDefinition, KindUnionTypeDefinition,
		KindEnumTypeDefinition, KindEnumValueDefinition,
		KindInputObjectTypeDefinition, KindDirectiveDefinition,
		KindSchemaExtension, KindScalarTypeExtension, KindObjectTypeExtension,
		KindInterfaceTypeExtension, KindUnionTypeExtension,
		KindEnumTypeExtension, KindInputObjectTypeExtension,
		KindDirectiveExtension, KindTypeCoordinate, KindMemberCoordinate,
		KindArgumentCoordinate, KindDirectiveCoordinate,
		KindDirectiveArgumentCoordinate,
	}
}

// Every kind must be reported by exactly one node type, and every node type
// must report a distinct kind. A mismatch means a node was added without a
// kind, or two nodes were given the same one, which would break every type
// switch that dispatches on Kind.
func TestAST_KindsAndNodeTypesCorrespond(t *testing.T) {
	nodes := allNodes()
	kinds := allKinds()

	byKind := make(map[Kind]Node, len(nodes))
	for _, n := range nodes {
		k := n.Kind()
		if prev, dup := byKind[k]; dup {
			t.Errorf("kind %v is reported by both %T and %T", k, prev, n)
			continue
		}
		byKind[k] = n
	}

	for _, k := range kinds {
		if _, ok := byKind[k]; !ok {
			t.Errorf("kind %v has no node type", k)
		}
	}
	if len(byKind) != len(kinds) {
		t.Errorf("%d node types for %d kinds", len(byKind), len(kinds))
	}
}

// Every node exposes its Loc field through the Location method. Getting this
// wrong on one node would silently drop error positions for that construct.
func TestAST_LocationReturnsTheLocField(t *testing.T) {
	for _, n := range allNodes() {
		if got := n.Location(); got != nil {
			t.Errorf("%T: zero value Location() = %v, want nil", n, got)
		}
		loc := &Location{Start: 1, End: 2}
		field := reflect.ValueOf(n).Elem().FieldByName("Loc")
		if !field.IsValid() {
			t.Errorf("%T has no Loc field", n)
			continue
		}
		field.Set(reflect.ValueOf(loc))
		if got := n.Location(); got != loc {
			t.Errorf("%T: Location() did not return the Loc field", n)
		}
	}
}

func TestAST_ValueInterface(t *testing.T) {
	values := []Value{
		&Variable{}, &IntValue{}, &FloatValue{}, &StringValue{},
		&BooleanValue{}, &NullValue{}, &EnumValue{}, &ListValue{},
		&ObjectValue{},
	}
	if len(values) != 9 {
		t.Fatalf("expected 9 value kinds, listed %d", len(values))
	}
	// A node that is not a value must not satisfy the interface.
	var n Node = &Name{}
	if _, ok := n.(Value); ok {
		t.Error("Name satisfies Value, want it not to")
	}
}

func TestAST_TypeInterface(t *testing.T) {
	types := []Type{&NamedType{}, &ListType{}, &NonNullType{}}
	if len(types) != 3 {
		t.Fatalf("expected 3 type kinds, listed %d", len(types))
	}
	var n Node = &Name{}
	if _, ok := n.(Type); ok {
		t.Error("Name satisfies Type, want it not to")
	}
}

func TestAST_SelectionInterface(t *testing.T) {
	selections := []Selection{&Field{}, &FragmentSpread{}, &InlineFragment{}}
	if len(selections) != 3 {
		t.Fatalf("expected 3 selection kinds, listed %d", len(selections))
	}
	var n Node = &Argument{}
	if _, ok := n.(Selection); ok {
		t.Error("Argument satisfies Selection, want it not to")
	}
}

func TestAST_DefinitionInterfaces(t *testing.T) {
	executable := []ExecutableDefinition{&OperationDefinition{}, &FragmentDefinition{}}
	if len(executable) != 2 {
		t.Fatalf("expected 2 executable definition kinds, listed %d", len(executable))
	}

	typeDefs := []TypeDefinition{
		&ScalarTypeDefinition{}, &ObjectTypeDefinition{}, &InterfaceTypeDefinition{},
		&UnionTypeDefinition{}, &EnumTypeDefinition{}, &InputObjectTypeDefinition{},
	}
	if len(typeDefs) != 6 {
		t.Fatalf("expected 6 type definition kinds, listed %d", len(typeDefs))
	}

	typeExts := []TypeExtension{
		&ScalarTypeExtension{}, &ObjectTypeExtension{}, &InterfaceTypeExtension{},
		&UnionTypeExtension{}, &EnumTypeExtension{}, &InputObjectTypeExtension{},
	}
	if len(typeExts) != 6 {
		t.Fatalf("expected 6 type extension kinds, listed %d", len(typeExts))
	}

	// Every type definition is also a type system definition and a definition,
	// and the same for extensions.
	for _, d := range typeDefs {
		var _ TypeSystemDefinition = d
		var _ Definition = d
	}
	for _, e := range typeExts {
		var _ TypeSystemExtension = e
		var _ Definition = e
	}

	// Directive and schema definitions are type system definitions but not
	// type definitions.
	var tsd TypeSystemDefinition = &DirectiveDefinition{}
	if _, ok := tsd.(TypeDefinition); ok {
		t.Error("DirectiveDefinition satisfies TypeDefinition, want it not to")
	}
	tsd = &SchemaDefinition{}
	if _, ok := tsd.(TypeDefinition); ok {
		t.Error("SchemaDefinition satisfies TypeDefinition, want it not to")
	}
}

func TestAST_SchemaCoordinateInterface(t *testing.T) {
	coords := []SchemaCoordinate{
		&TypeCoordinate{}, &MemberCoordinate{}, &ArgumentCoordinate{},
		&DirectiveCoordinate{}, &DirectiveArgumentCoordinate{},
	}
	if len(coords) != 5 {
		t.Fatalf("expected 5 coordinate kinds, listed %d", len(coords))
	}
}

func TestField_ResponseKey(t *testing.T) {
	tests := []struct {
		name  string
		field *Field
		want  string
	}{
		{"name only", &Field{Name: &Name{Value: "hero"}}, "hero"},
		{"alias wins", &Field{Alias: &Name{Value: "h"}, Name: &Name{Value: "hero"}}, "h"},
		{"no name", &Field{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.field.ResponseKey(); got != tt.want {
				t.Errorf("ResponseKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNamedTypeOf(t *testing.T) {
	named := &NamedType{Name: &Name{Value: "Int"}}
	tests := []struct {
		name string
		in   Type
		want *NamedType
	}{
		{"named", named, named},
		{"list", &ListType{Type: named}, named},
		{"non-null", &NonNullType{Type: named}, named},
		{"non-null list of non-null", &NonNullType{Type: &ListType{Type: &NonNullType{Type: named}}}, named},
		{"nil", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NamedTypeOf(tt.in); got != tt.want {
				t.Errorf("NamedTypeOf() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainsVariable(t *testing.T) {
	variable := &Variable{Name: &Name{Value: "x"}}
	tests := []struct {
		name string
		in   Value
		want bool
	}{
		{"nil", nil, false},
		{"int", &IntValue{Value: "1"}, false},
		{"null", &NullValue{}, false},
		{"variable", variable, true},
		{"list without a variable", &ListValue{Values: []Value{&IntValue{Value: "1"}}}, false},
		{"list with a variable", &ListValue{Values: []Value{&IntValue{Value: "1"}, variable}}, true},
		{"nested list", &ListValue{Values: []Value{&ListValue{Values: []Value{variable}}}}, true},
		{
			name: "object without a variable",
			in:   &ObjectValue{Fields: []*ObjectField{{Name: &Name{Value: "a"}, Value: &IntValue{Value: "1"}}}},
			want: false,
		},
		{
			name: "object with a variable",
			in:   &ObjectValue{Fields: []*ObjectField{{Name: &Name{Value: "a"}, Value: variable}}},
			want: true,
		},
		{
			name: "object holding a list holding a variable",
			in:   &ObjectValue{Fields: []*ObjectField{{Value: &ListValue{Values: []Value{variable}}}}},
			want: true,
		},
		{"empty list", &ListValue{}, false},
		{"empty object", &ObjectValue{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsVariable(tt.in); got != tt.want {
				t.Errorf("ContainsVariable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDirectiveLocation(t *testing.T) {
	if !IsExecutableDirectiveLocation(DirectiveLocationQuery) {
		t.Error("QUERY is not reported as an executable location")
	}
	if IsTypeSystemDirectiveLocation(DirectiveLocationQuery) {
		t.Error("QUERY is reported as a type system location")
	}
	if !IsTypeSystemDirectiveLocation(DirectiveLocationObject) {
		t.Error("OBJECT is not reported as a type system location")
	}
	if IsExecutableDirectiveLocation(DirectiveLocationObject) {
		t.Error("OBJECT is reported as an executable location")
	}
	if IsDirectiveLocation("NOT_A_LOCATION") {
		t.Error("an unknown name is reported as a directive location")
	}
	// The two sets must be disjoint and together account for every location.
	if got := len(executableDirectiveLocations) + len(typeSystemDirectiveLocations); got != 21 {
		t.Errorf("%d directive locations, want 21", got)
	}
	for l := range executableDirectiveLocations {
		if typeSystemDirectiveLocations[l] {
			t.Errorf("%v is in both location sets", l)
		}
	}
}
