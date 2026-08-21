package execution

import (
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/schema"
)

// TestSubfieldsAreWorkedOutOnce is graphql-js's "memoizes collectSubfields
// results". Two objects of the same type reached through the same selections
// are answered with the same grouped field set rather than with two equal
// ones, which is what saves the work of collecting them again.
func TestSubfieldsAreWorkedOutOnce(t *testing.T) {
	deep := schema.NewObject(schema.ObjectConfig{
		Name:   "DeepType",
		Fields: []*schema.Field{schema.NewField("name", schema.FieldConfig{Type: schema.String})},
	})
	s := schema.New(schema.Config{Query: schema.NewObject(schema.ObjectConfig{
		Name:   "Query",
		Fields: []*schema.Field{schema.NewField("deep", schema.FieldConfig{Type: deep})},
	})})
	doc, err := language.ParseString("{ deep { name } }")
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	operation, isOperation := doc.Definitions[0].(*language.OperationDefinition)
	if !isOperation {
		t.Fatal("the first definition is not an operation")
	}
	field, isField := operation.SelectionSet.Selections[0].(*language.Field)
	if !isField {
		t.Fatal("the first selection is not a field")
	}

	e := &executor{schema: s, shared: &runState{}}
	selections := []FieldSelection{{Node: field}}

	first, _ := e.subfieldsOf(deep, selections)
	second, _ := e.subfieldsOf(deep, selections)
	if first != second {
		t.Error("asking twice for the same selections worked them out twice")
	}

	// A different list of the same selections is a different question, as it
	// is in graphql-js, whose table is keyed by the list itself.
	third, _ := e.subfieldsOf(deep, []FieldSelection{{Node: field}})
	if third == first {
		t.Error("a separate list of selections was answered from the table")
	}
}
