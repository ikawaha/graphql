package utilities_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/utilities"
)

// TestIntrospectionResult_EmptyListsSurviveJSON pins the distinction a client
// schema is rebuilt from: a type that answers a list with none of them writes
// [], and one that has no such list at all writes nothing. Collapsing the two
// leaves a rebuilt schema quietly missing what it was told.
func TestIntrospectionResult_EmptyListsSurviveJSON(t *testing.T) {
	s, err := utilities.BuildSchema(`
		type Query { plain: String }
		scalar Custom
	`)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	answer, err := utilities.IntrospectionFromSchema(context.Background(), s)
	if err != nil {
		t.Fatalf("asking the schema about itself: %v", err)
	}

	encoded, err := json.Marshal(answer)
	if err != nil {
		t.Fatalf("writing it out: %v", err)
	}
	text := string(encoded)
	if !strings.Contains(text, `"name":"Query","interfaces":[]`) &&
		!strings.Contains(text, `"interfaces":[]`) {
		t.Error("an object implementing nothing did not write an empty interfaces list")
	}
	if strings.Contains(text, `"name":"Custom","interfaces":`) {
		t.Error("a scalar wrote an interfaces list, which it has none of")
	}

	var again utilities.IntrospectionQueryResult
	if err := json.Unmarshal(encoded, &again); err != nil {
		t.Fatalf("reading it back: %v", err)
	}
	for _, ty := range again.Schema.Types {
		switch ty.Name {
		case "Query":
			if ty.Interfaces == nil {
				t.Error("Query came back with no interfaces list at all")
			}
			if len(ty.Interfaces) != 0 {
				t.Errorf("Query came back implementing %d interfaces", len(ty.Interfaces))
			}
		case "Custom":
			if ty.Interfaces != nil || ty.Fields != nil {
				t.Error("a scalar came back with lists it cannot have")
			}
		}
	}

	// And what came back still builds, which is the point of keeping them apart.
	if _, err := utilities.BuildClientSchema(&again); err != nil {
		t.Errorf("rebuilding from the round trip: %v", err)
	}
}
