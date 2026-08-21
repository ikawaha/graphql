package execution_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ikawaha/graphql/execution"
	"github.com/ikawaha/graphql/schema"
)

type weird struct {
	Exported string
	hidden   int
	Self     *weird
	Fn       func()
	Ch       chan int
}

// Whatever a resolver hands back, the response is a response — never a crash.
// A field declared as an enum given a map is the one that used to take the
// process down: the lookup compared the value with ==, and a map cannot be
// compared.
func TestExecute_SurvivesAnyResolverValue(t *testing.T) {
	self := &weird{Exported: "x", hidden: 1}
	self.Self = self
	cyclicSlice := make([]any, 1)
	cyclicSlice[0] = cyclicSlice

	values := map[string]any{
		"nil":              nil,
		"typed nil":        (*weird)(nil),
		"nil in interface": any((*weird)(nil)),
		"a struct":         weird{Exported: "x", hidden: 1},
		"a cyclic struct":  self,
		"a channel":        make(chan int),
		"a function":       func() {},
		"a cyclic slice":   cyclicSlice,
		"a map":            map[string]any{"a": 1},
		"a non-string map": map[int]any{1: 2},
		"an array":         [2]int{1, 2},
		"a nil slice":      []any(nil),
		"a nil map":        map[string]any(nil),
		"a pointer chain":  &self,
		"an error value":   errors.New("boom"),
		"a complex number": complex(1, 2),
		"a uintptr":        uintptr(1),
		"an unsafe-ish":    struct{ A [0]byte }{},
	}

	fields := map[string]string{
		"scalar":      "String",
		"nonNull":     "String!",
		"list":        "[String]",
		"listNonNull": "[String!]!",
		"int":         "Int",
		"enum":        "Colour",
		"object":      "Thing",
		"iface":       "Named",
		"union":       "Any",
	}

	for fieldName, fieldType := range fields {
		s := buildSchema(t, `
			enum Colour { RED }
			type Thing implements Named { name: String }
			interface Named { name: String }
			union Any = Thing
			type Query { f: `+fieldType+` }
		`)
		for valueName, held := range values {
			t.Run(fieldName+"/"+valueName, func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("PANIC: %v", r)
					}
				}()
				s.QueryType().Field("f").Resolve = func(
					context.Context, any, schema.Arguments, *schema.ResolveInfo,
				) (any, error) {
					return held, nil
				}
				result := execution.Execute(context.Background(), execution.Request{
					Schema: s, Document: mustParse(t, `{ f }`),
				})
				_ = jsonOf(t, result)
			})
		}
	}
}
