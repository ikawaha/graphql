package gqlerror_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ikawaha/graphql/gqlerror"
	"github.com/ikawaha/graphql/language"
)

func mustParse(t *testing.T, body string) *language.Document {
	t.Helper()
	doc, err := language.ParseString(body)
	if err != nil {
		t.Fatalf("parsing %q: %v", body, err)
	}
	return doc
}

// firstField returns the first field selection of a shorthand query.
func firstField(t *testing.T, doc *language.Document) language.Node {
	t.Helper()
	op, ok := doc.Definitions[0].(*language.OperationDefinition)
	if !ok {
		t.Fatalf("definition is %T, want an operation", doc.Definitions[0])
	}
	return op.SelectionSet.Selections[0]
}

func TestNew_Minimal(t *testing.T) {
	err := gqlerror.New("boom")
	if err.Message != "boom" {
		t.Errorf("Message = %q, want %q", err.Message, "boom")
	}
	if got := err.Error(); got != "boom" {
		t.Errorf("Error() = %q, want %q", got, "boom")
	}
	if err.Locations != nil {
		t.Errorf("Locations = %v, want nil", err.Locations)
	}
	if err.Unwrap() != nil {
		t.Errorf("Unwrap() = %v, want nil", err.Unwrap())
	}
}

// Most of the messages a response carries name the type or the field they are
// about, so working one out from a format is the common case.
func TestNewf(t *testing.T) {
	err := gqlerror.Newf("Cannot return null for non-nullable field %s.%s.", "User", "name")
	want := "Cannot return null for non-nullable field User.name."
	if err.Message != want {
		t.Errorf("Message = %q, want %q", err.Message, want)
	}
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	// Nothing else is said, which is what tells it from New.
	if err.Locations != nil || err.Nodes != nil || err.Path != nil {
		t.Errorf("Newf said more than the message: %+v", err)
	}

	t.Run("a format with nothing to fill in", func(t *testing.T) {
		if got := gqlerror.Newf("boom").Message; got != "boom" {
			t.Errorf("Message = %q, want %q", got, "boom")
		}
	})
}

func TestNew_LocationsFromPositions(t *testing.T) {
	source := language.NewSource("{ name }")
	err := gqlerror.New("Cannot query field.",
		gqlerror.WithSource(source),
		gqlerror.WithPositions(2))

	want := []language.SourceLocation{{Line: 1, Column: 3}}
	if len(err.Locations) != 1 || err.Locations[0] != want[0] {
		t.Errorf("Locations = %v, want %v", err.Locations, want)
	}
}

// With no positions given, the error takes them from the nodes it blames,
// along with the document they came from.
func TestNew_LocationsFromNodes(t *testing.T) {
	doc := mustParse(t, "{ hero }")
	field := firstField(t, doc)

	err := gqlerror.New("Cannot query field.", gqlerror.WithNodes(field))

	if err.Source == nil {
		t.Fatal("Source was not taken from the node")
	}
	if want := []int{2}; len(err.Positions) != 1 || err.Positions[0] != want[0] {
		t.Errorf("Positions = %v, want %v", err.Positions, want)
	}
	if want := (language.SourceLocation{Line: 1, Column: 3}); len(err.Locations) != 1 || err.Locations[0] != want {
		t.Errorf("Locations = %v, want [%v]", err.Locations, want)
	}
}

func TestNew_LocationsFromSeveralNodes(t *testing.T) {
	doc := mustParse(t, "{ a b }")
	op := doc.Definitions[0].(*language.OperationDefinition)
	first := op.SelectionSet.Selections[0]
	second := op.SelectionSet.Selections[1]

	err := gqlerror.New("Fields conflict.", gqlerror.WithNodes(first, second))
	if len(err.Locations) != 2 {
		t.Fatalf("%d locations, want 2", len(err.Locations))
	}
	if err.Locations[0].Column != 3 || err.Locations[1].Column != 5 {
		t.Errorf("Locations = %v, want columns 3 and 5", err.Locations)
	}
}

// A node without a location contributes nothing rather than a bogus position.
func TestNew_NodesWithoutLocations(t *testing.T) {
	doc, parseErr := language.ParseString("{ hero }", language.NoLocation())
	if parseErr != nil {
		t.Fatalf("parsing: %v", parseErr)
	}
	err := gqlerror.New("boom", gqlerror.WithNodes(firstField(t, doc)))
	if err.Locations != nil {
		t.Errorf("Locations = %v, want nil", err.Locations)
	}
	if got := err.Error(); got != "boom" {
		t.Errorf("Error() = %q, want %q", got, "boom")
	}
}

// Locations are reported in the coordinates of the file the source came from.
func TestNew_LocationsApplyTheSourceOffset(t *testing.T) {
	source := language.NewSource("{ name }",
		language.SourceName("Foo.graphql"),
		language.SourceLocationOffset(40, 3))

	err := gqlerror.New("boom", gqlerror.WithSource(source), gqlerror.WithPositions(0))
	if want := (language.SourceLocation{Line: 40, Column: 3}); err.Locations[0] != want {
		t.Errorf("Locations[0] = %v, want %v", err.Locations[0], want)
	}
}

func TestError_IncludesSourceExcerpt(t *testing.T) {
	source := language.NewSource("{ name }")
	err := gqlerror.New("Cannot query field.",
		gqlerror.WithSource(source),
		gqlerror.WithPositions(2))

	want := strings.Join([]string{
		"Cannot query field.",
		"",
		"GraphQL request:1:3",
		"1 | { name }",
		"  |   ^",
	}, "\n")
	if got := err.Error(); got != want {
		t.Errorf("Error() =\n%s\nwant\n%s", got, want)
	}
}

// The excerpt applies the source offset exactly once, even though the reported
// locations already carry it.
func TestError_ExcerptWithOffsetIsNotDoubled(t *testing.T) {
	source := language.NewSource("{ name }",
		language.SourceName("Foo.graphql"),
		language.SourceLocationOffset(40, 1))
	err := gqlerror.New("boom", gqlerror.WithSource(source), gqlerror.WithPositions(2))

	if !strings.Contains(err.Error(), "Foo.graphql:40:3") {
		t.Errorf("Error() does not point at 40:3:\n%s", err.Error())
	}
	if strings.Contains(err.Error(), ":79:") {
		t.Errorf("the offset was applied twice:\n%s", err.Error())
	}
}

func TestError_Unwrap(t *testing.T) {
	cause := errors.New("underlying")
	err := gqlerror.New("wrapped", gqlerror.WithCause(cause))

	if !errors.Is(err, cause) {
		t.Error("errors.Is did not find the cause")
	}
	if got := err.Unwrap(); got != cause {
		t.Errorf("Unwrap() = %v, want %v", got, cause)
	}
}

func TestError_AsThroughWrapping(t *testing.T) {
	err := gqlerror.New("boom")
	wrapped := errors.Join(errors.New("other"), err)

	var found *gqlerror.Error
	if !errors.As(wrapped, &found) {
		t.Fatal("errors.As did not find the GraphQL error")
	}
	if found.Message != "boom" {
		t.Errorf("Message = %q, want %q", found.Message, "boom")
	}
}

func TestError_JSON(t *testing.T) {
	tests := []struct {
		name string
		err  *gqlerror.Error
		want string
	}{
		{
			name: "message only",
			err:  gqlerror.New("boom"),
			want: `{"message":"boom"}`,
		},
		{
			name: "with a path",
			err:  gqlerror.New("boom", gqlerror.WithPath("viewer", "name")),
			want: `{"message":"boom","path":["viewer","name"]}`,
		},
		{
			name: "with a list index in the path",
			err:  gqlerror.New("boom", gqlerror.WithPath("friends", 0, "name")),
			want: `{"message":"boom","path":["friends",0,"name"]}`,
		},
		{
			name: "with locations",
			err: gqlerror.New("boom",
				gqlerror.WithSource(language.NewSource("{ name }")),
				gqlerror.WithPositions(2)),
			want: `{"message":"boom","locations":[{"line":1,"column":3}]}`,
		},
		{
			name: "with extensions",
			err:  gqlerror.New("boom", gqlerror.WithExtensions(map[string]any{"code": "OOPS"})),
			want: `{"message":"boom","extensions":{"code":"OOPS"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.err)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Marshal() = %s, want %s", got, tt.want)
			}
		})
	}
}

// The fields that only matter on the server never reach a response.
func TestError_JSONOmitsServerOnlyFields(t *testing.T) {
	doc := mustParse(t, "{ hero }")
	err := gqlerror.New("boom", gqlerror.WithNodes(firstField(t, doc)))

	encoded, marshalErr := json.Marshal(err)
	if marshalErr != nil {
		t.Fatalf("Marshal() error = %v", marshalErr)
	}
	for _, absent := range []string{"nodes", "source", "positions"} {
		if strings.Contains(string(encoded), absent) {
			t.Errorf("response contains %q: %s", absent, encoded)
		}
	}
	if !strings.Contains(string(encoded), "locations") {
		t.Errorf("response has no locations: %s", encoded)
	}
}

func TestFromSyntaxError(t *testing.T) {
	_, parseErr := language.ParseString("{")
	var syntaxErr *language.SyntaxError
	if !errors.As(parseErr, &syntaxErr) {
		t.Fatalf("parse error is %T, want *language.SyntaxError", parseErr)
	}

	err := gqlerror.FromSyntaxError(syntaxErr)
	if !strings.HasPrefix(err.Message, "Syntax Error:") {
		t.Errorf("Message = %q, want it to begin with %q", err.Message, "Syntax Error:")
	}
	if len(err.Locations) != 1 {
		t.Fatalf("%d locations, want 1", len(err.Locations))
	}
	if !errors.Is(err, syntaxErr) {
		t.Error("the syntax error is not reachable through the cause chain")
	}

	if gqlerror.FromSyntaxError(nil) != nil {
		t.Error("FromSyntaxError(nil) is not nil")
	}
}

func TestEnsure(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if gqlerror.Ensure(nil) != nil {
			t.Error("Ensure(nil) is not nil")
		}
	})

	t.Run("already a GraphQL error", func(t *testing.T) {
		original := gqlerror.New("boom")
		if got := gqlerror.Ensure(original); got != original {
			t.Error("Ensure returned a different error for one that was already a GraphQL error")
		}
	})

	t.Run("syntax error keeps its position", func(t *testing.T) {
		_, parseErr := language.ParseString("{")
		got := gqlerror.Ensure(parseErr)
		if len(got.Locations) != 1 {
			t.Errorf("%d locations, want the syntax error's position to survive", len(got.Locations))
		}
	})

	t.Run("plain error", func(t *testing.T) {
		cause := errors.New("plain")
		got := gqlerror.Ensure(cause)
		if got.Message != "plain" {
			t.Errorf("Message = %q, want %q", got.Message, "plain")
		}
		if !errors.Is(got, cause) {
			t.Error("the original error is not reachable")
		}
	})
}

func TestLocated(t *testing.T) {
	doc := mustParse(t, "{ hero }")
	field := firstField(t, doc)

	t.Run("nil", func(t *testing.T) {
		if gqlerror.Located(nil, nil, nil) != nil {
			t.Error("Located(nil, ...) is not nil")
		}
	})

	t.Run("adds a position and a path to a plain error", func(t *testing.T) {
		got := gqlerror.Located(errors.New("resolver failed"),
			[]language.Node{field}, []any{"hero"})

		if got.Message != "resolver failed" {
			t.Errorf("Message = %q, want %q", got.Message, "resolver failed")
		}
		if want := (language.SourceLocation{Line: 1, Column: 3}); len(got.Locations) != 1 || got.Locations[0] != want {
			t.Errorf("Locations = %v, want [%v]", got.Locations, want)
		}
		if len(got.Path) != 1 || got.Path[0] != "hero" {
			t.Errorf("Path = %v, want [hero]", got.Path)
		}
	})

	t.Run("keeps a path that was already set", func(t *testing.T) {
		deep := gqlerror.New("already located", gqlerror.WithPath("a", "b"))
		got := gqlerror.Located(deep, []language.Node{field}, []any{"hero"})
		if got != deep {
			t.Error("an error that already had a path was rebuilt")
		}
		if len(got.Path) != 2 {
			t.Errorf("Path = %v, want the original two entries", got.Path)
		}
	})

	t.Run("prefers the nodes the error already blamed", func(t *testing.T) {
		other := mustParse(t, "{ other }")
		otherField := firstField(t, other)

		original := gqlerror.New("boom", gqlerror.WithNodes(otherField))
		got := gqlerror.Located(original, []language.Node{field}, []any{"hero"})

		if len(got.Nodes) != 1 || got.Nodes[0] != otherField {
			t.Errorf("Nodes = %v, want the node the error already blamed", got.Nodes)
		}
	})

	t.Run("carries extensions across", func(t *testing.T) {
		original := gqlerror.New("boom", gqlerror.WithExtensions(map[string]any{"code": "OOPS"}))
		got := gqlerror.Located(original, []language.Node{field}, []any{"hero"})
		if got.Extensions["code"] != "OOPS" {
			t.Errorf("Extensions = %v, want the code to survive", got.Extensions)
		}
	})
}
