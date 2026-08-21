package utilities_test

// Ported from graphql-js src/utilities/__tests__/introspectionFromSchema-test.ts:
// a schema described to a client, and the schema that description rebuilds
// into. The third case there is about a deprecated directive, which is an
// experimental part of introspection this does not answer.

import (
	"context"
	"testing"

	"github.com/ikawaha/graphql/utilities"
)

func TestPortedIntrospectionFromSchema(t *testing.T) {
	const sdl = `"""This is a simple schema"""
schema {
  query: Simple
}

"""This is a simple type"""
type Simple {
  """This is a string field"""
  string: String
}`
	const undocumented = `schema {
  query: Simple
}

type Simple {
  string: String
}`

	for _, tt := range []struct {
		name string
		opts []utilities.IntrospectionOption
		want string
	}{
		{name: "describes a simple schema", want: sdl},
		{
			name: "describes a simple schema without descriptions",
			opts: []utilities.IntrospectionOption{utilities.WithoutDescriptions()},
			want: undocumented,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			answer, err := utilities.IntrospectionFromSchema(
				context.Background(), mustBuild(t, sdl), tt.opts...)
			if err != nil {
				t.Fatalf("describing the schema: %v", err)
			}
			rebuilt, err := utilities.BuildClientSchema(answer)
			if err != nil {
				t.Fatalf("rebuilding from the description: %v", err)
			}
			if got := utilities.PrintSchema(rebuilt); got != tt.want {
				t.Errorf("came back as\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}
