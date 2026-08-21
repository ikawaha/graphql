package typeinfo_test

import (
	"testing"

	"github.com/ikawaha/graphql/schema"
	"github.com/ikawaha/graphql/utilities"
)

// mustBuild builds a schema from SDL. The builder sits above this package, so
// these tests are external ones that reach up to it.
func mustBuild(t *testing.T, sdl string) *schema.Schema {
	t.Helper()
	s, err := utilities.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("building the schema: %v", err)
	}
	return s
}
