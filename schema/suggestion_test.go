package schema

import (
	"reflect"
	"testing"
)

func TestSuggestionList(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		options []string
		want    []string
	}{
		{"nothing to suggest", "input", nil, nil},
		{"nothing close enough", "input", []string{"a", "b"}, nil},
		// graphql-js measures an option identical to what was written as zero
		// steps away, so it comes out first rather than being left out.
		{"an exact match is nearest", "abc", []string{"a", "ab", "abc"}, []string{"abc", "ab", "a"}},
		{"nothing to suggest for an empty name", "", []string{"a"}, []string{"a"}},
		{"one character off", "abc", []string{"abd"}, []string{"abd"}},
		// A case change counts as one step, the same as inserting a letter, so
		// these tie and the name decides the order.
		{"case only", "dogName", []string{"dogname", "dogNames"}, []string{"dogNames", "dogname"}},
		// Two neighbours swapped is one slip, not two mistakes.
		{"transposition", "hlelo", []string{"hello", "hallo"}, []string{"hello", "hallo"}},
		{
			name:    "nearest first",
			input:   "GraphQL",
			options: []string{"graphql", "GraphSQL", "SomethingElse", "GraphQ"},
			// All three are one step away, so they come out in name order.
			want: []string{"GraphQ", "GraphSQL", "graphql"},
		},
		// A name much longer than what was written is not a plausible typo.
		{"far too long", "id", []string{"identifier"}, nil},
		// Ported from graphql-js's suggestionList tests.
		{"beyond the threshold", "aaaa", []string{"aaab"}, []string{"aaab"}},
		{"two steps is still within", "aaaa", []string{"aabb"}, []string{"aabb"}},
		{"three steps is not", "aaaa", []string{"abbb"}, nil},
		{"a short name far away", "ab", []string{"ca"}, nil},
		{"long names differing only in case", "verylongstring", []string{"VERYLONGSTRING"}, []string{"VERYLONGSTRING"}},
		{"digits transposed", "214365879", []string{"123456789"}, []string{"123456789"}},
		{
			name:    "nearest first, then by name",
			input:   "GraphQl",
			options: []string{"graphics", "SQL", "GraphQL", "quarks", "mark"},
			want:    []string{"GraphQL", "graphics"},
		},
		{"a tie is broken naturally", "a", []string{"az", "ax", "ay"}, []string{"ax", "ay", "az"}},
		{"another tie", "boo", []string{"moo", "foo", "zoo"}, []string{"foo", "moo", "zoo"}},
		{"a tie broken by natural order", "abc", []string{"a1", "a12", "a2"}, []string{"a1", "a2", "a12"}},
		{
			name:    "distance first, then naturally",
			input:   "csutomer",
			options: []string{"store", "customer", "stomer", "some", "more"},
			want:    []string{"customer", "stomer", "some", "store"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SuggestionList(tt.input, tt.options)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SuggestionList(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDidYouMean(t *testing.T) {
	tests := []struct {
		name        string
		subMessage  string
		suggestions []string
		want        string
	}{
		{"nothing to say", "", nil, ""},
		{"one", "", []string{"a"}, ` Did you mean "a"?`},
		{"two", "", []string{"a", "b"}, ` Did you mean "a" or "b"?`},
		{"three", "", []string{"a", "b", "c"}, ` Did you mean "a", "b", or "c"?`},
		{"with a sub-message", "the type", []string{"a"}, ` Did you mean the type "a"?`},
		{
			name:        "more than can be read",
			suggestions: []string{"a", "b", "c", "d", "e", "f", "g"},
			want:        ` Did you mean "a", "b", "c", "d", or "e"?`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DidYouMean(tt.subMessage, tt.suggestions); got != tt.want {
				t.Errorf("didYouMean = %q, want %q", got, tt.want)
			}
		})
	}
}
