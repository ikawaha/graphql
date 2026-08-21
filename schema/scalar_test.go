package schema

import (
	"errors"
	"github.com/ikawaha/graphql/value"
	"testing"

	"github.com/ikawaha/graphql/language"
)

func TestNewScalar_Defaults(t *testing.T) {
	s := NewScalar(ScalarConfig{Name: "Custom", Description: value.Just("A custom scalar.")})

	if got := s.Name(); got != "Custom" {
		t.Errorf("Name() = %q, want %q", got, "Custom")
	}
	if got := s.Description(); got != "A custom scalar." {
		t.Errorf("Description() = %q, want %q", got, "A custom scalar.")
	}
	if got := s.String(); got != "Custom" {
		t.Errorf("String() = %q, want %q", got, "Custom")
	}

	// A scalar with nothing to convert passes values through both ways.
	out, err := s.CoerceOutputValue(42)
	if err != nil || out.Or(nil) != 42 {
		t.Errorf("CoerceOutputValue(42) = %v, %v, want 42, nil", out, err)
	}
	in, err := s.CoerceInputValue("x")
	if err != nil || in.Or(nil) != "x" {
		t.Errorf("CoerceInputValue(\"x\") = %v, %v, want x, nil", in, err)
	}

	// The two that have no sensible default stay nil, so callers can tell
	// whether the scalar handles them.
	if s.CoerceInputLiteral != nil {
		t.Error("CoerceInputLiteral was filled in, want nil")
	}
	if s.ValueToLiteral != nil {
		t.Error("ValueToLiteral was filled in, want nil")
	}
}

func TestNewScalar_Coercers(t *testing.T) {
	boom := errors.New("not an Int")
	s := NewScalar(ScalarConfig{
		Name: "Int",
		CoerceOutputValue: func(v any) (value.Maybe[any], error) {
			n, ok := v.(int)
			if !ok {
				return value.Nothing[any](), boom
			}
			return value.Just[any](n * 2), nil
		},
		CoerceInputValue: func(v any) (value.Maybe[any], error) {
			n, ok := v.(int)
			if !ok {
				return value.Nothing[any](), boom
			}
			return value.Just[any](n), nil
		},
		CoerceInputLiteral: func(literal language.Value) (value.Maybe[any], error) {
			lit, ok := literal.(*language.IntValue)
			if !ok {
				return value.Nothing[any](), boom
			}
			return value.Just[any](lit.Value), nil
		},
	})

	if got, err := s.CoerceOutputValue(21); err != nil || got.Or(nil) != 42 {
		t.Errorf("CoerceOutputValue(21) = %v, %v, want 42, nil", got, err)
	}
	if _, err := s.CoerceOutputValue("no"); !errors.Is(err, boom) {
		t.Errorf("CoerceOutputValue(\"no\") error = %v, want %v", err, boom)
	}
	if got, err := s.CoerceInputLiteral(&language.IntValue{Value: "5"}); err != nil || got.Or(nil) != "5" {
		t.Errorf("CoerceInputLiteral() = %v, %v, want 5, nil", got, err)
	}
}

func TestScalar_TypePredicates(t *testing.T) {
	s := testScalar("Int")

	if !IsScalarType(s) {
		t.Error("IsScalarType = false, want true")
	}
	if !IsNamedType(s) {
		t.Error("IsNamedType = false, want true")
	}
	// A scalar can appear on either side of a request, and a selection ends
	// at one.
	if !IsInputType(s) || !IsOutputType(s) || !IsLeafType(s) {
		t.Errorf("scalar: input=%v output=%v leaf=%v, want all true",
			IsInputType(s), IsOutputType(s), IsLeafType(s))
	}
	if IsWrappingType(s) || IsNonNullType(s) || IsListType(s) {
		t.Error("a scalar was reported as a wrapper")
	}
	if !IsNullableType(s) {
		t.Error("IsNullableType = false, want true")
	}

	// Wrapping keeps the leaf reachable but the wrapper itself is not a leaf.
	if IsLeafType(NewList(s)) {
		t.Error("a list was reported as a leaf")
	}
	if got := NamedTypeOf(NewNonNull(NewList(s))); got != NamedType(s) {
		t.Errorf("NamedTypeOf() = %v, want the scalar", got)
	}
}

func TestScalar_StringOnNil(t *testing.T) {
	var absent *ScalarType
	if got := absent.String(); got != "<nil>" {
		t.Errorf("String() on nil = %q, want %q", got, "<nil>")
	}
}

func TestScalar_CarriesASTAndExtensions(t *testing.T) {
	node := &language.ScalarTypeDefinition{Name: &language.Name{Value: "DateTime"}}
	ext := &language.ScalarTypeExtension{Name: &language.Name{Value: "DateTime"}}

	s := NewScalar(ScalarConfig{
		Name:              "DateTime",
		SpecifiedByURL:    "https://example.com/datetime",
		ASTNode:           node,
		ExtensionASTNodes: []*language.ScalarTypeExtension{ext},
		Extensions:        map[string]any{"internal": true},
	})

	if s.ASTNode != node {
		t.Error("ASTNode was not kept")
	}
	if len(s.ExtensionASTNodes) != 1 || s.ExtensionASTNodes[0] != ext {
		t.Error("ExtensionASTNodes were not kept")
	}
	if s.SpecifiedByURL != "https://example.com/datetime" {
		t.Errorf("SpecifiedByURL = %q", s.SpecifiedByURL)
	}
	if s.Extensions["internal"] != true {
		t.Errorf("Extensions = %v", s.Extensions)
	}
}
