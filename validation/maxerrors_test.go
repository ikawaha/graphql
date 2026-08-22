package validation_test

import (
	"strings"
	"testing"

	"github.com/ikawaha/graphql/language"
	"github.com/ikawaha/graphql/utilities"
	"github.com/ikawaha/graphql/validation"
)

// TestWithMaxErrors covers the bound on how many problems a check reports.
//
// The counts are graphql-js's, taken from running it on the same document:
// a bound of n gives n problems and then the message saying there were more,
// zero gives that message alone, and a negative bound gives everything, which
// is what graphql-js's Infinity does.
func TestWithMaxErrors(t *testing.T) {
	s, err := utilities.BuildSchema(`type Query { a: String b: String }`)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := language.ParseString(`{ nope1 nope2 nope3 }`)
	if err != nil {
		t.Fatal(err)
	}

	const tooMany = "Too many validation errors, error limit reached. Validation aborted."

	tests := []struct {
		name    string
		opts    []validation.Option
		want    int
		lastIs  string
		aborted bool
	}{
		{name: "no bound asked for, so the default", want: 3},
		{name: "a bound above what there is", opts: []validation.Option{validation.WithMaxErrors(9)}, want: 3},
		{name: "a bound equal to what there is", opts: []validation.Option{validation.WithMaxErrors(3)}, want: 3},
		{
			name: "a bound below what there is",
			opts: []validation.Option{validation.WithMaxErrors(2)},
			want: 3, lastIs: tooMany, aborted: true,
		},
		{
			name: "a bound of zero",
			opts: []validation.Option{validation.WithMaxErrors(0)},
			want: 1, lastIs: tooMany, aborted: true,
		},
		{name: "no bound at all", opts: []validation.Option{validation.WithMaxErrors(-1)}, want: 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := validation.ValidateWithOptions(s, doc, test.opts...)
			if len(errs) != test.want {
				t.Fatalf("%d errors, wanted %d: %v", len(errs), test.want, errs)
			}
			if test.lastIs != "" && errs[len(errs)-1].Message != test.lastIs {
				t.Errorf("the last error was %q", errs[len(errs)-1].Message)
			}
			if !test.aborted {
				for _, e := range errs {
					if strings.Contains(e.Message, "Too many") {
						t.Errorf("the check gave up when it had no reason to: %v", errs)
					}
				}
			}
		})
	}
}
