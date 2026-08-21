package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/DeferStreamDirectiveLabelRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_DeferStreamDirectiveLabel(t *testing.T) {
	runPorted(t, validation.DeferStreamDirectiveLabelRule, []portedCase{
		{
			name: `Defer fragments with no label`,
			steps: []portedStep{
				{
					query: `
      {
        dog {
          ...dogFragmentA @defer
          ...dogFragmentB @defer
        }
      }
      fragment dogFragmentA on Dog {
        name
      }
      fragment dogFragmentB on Dog {
        nickname
      }
    `,
				},
			},
		},
		{
			name: `Defer fragments, one with label, one without`,
			steps: []portedStep{
				{
					query: `
      {
        dog {
          ...dogFragmentA @defer(label: "fragA")
          ...dogFragmentB @defer
        }
      }
      fragment dogFragmentA on Dog {
        name
      }
      fragment dogFragmentB on Dog {
        nickname
      }
    `,
				},
			},
		},
		{
			name: `Defer fragment with null label`,
			steps: []portedStep{
				{
					query: `
      {
        dog {
          ...dogFragmentA @defer(label: null)
        }
      }
      fragment dogFragmentA on Dog {
        name
      }
    `,
				},
			},
		},
		{
			name: `Defer fragment with variable label`,
			steps: []portedStep{
				{
					query: `
    query($label: String) {
      dog {
        ...dogFragmentA @defer(label: $label)
        ...dogFragmentB @defer(label: "fragA")
      }
    }
    fragment dogFragmentA on Dog {
      name
    }
    fragment dogFragmentB on Dog {
      nickname
    }
    `,
					want: []want{
						{At: []at{{4, 25}}},
					},
				},
			},
		},
		{
			name: `Defer fragments with different labels`,
			steps: []portedStep{
				{
					query: `
    {
      dog {
        ...dogFragmentA @defer(label: "fragB")
        ...dogFragmentB @defer(label: "fragA")
      }
    }
    fragment dogFragmentA on Dog {
      name
    }
    fragment dogFragmentB on Dog {
      nickname
    }
    `,
				},
			},
		},
		{
			name: `Defer fragments with same label`,
			steps: []portedStep{
				{
					query: `
    {
      dog {
        ...dogFragmentA @defer(label: "fragA")
        ...dogFragmentB @defer(label: "fragA")
      }
    }
    fragment dogFragmentA on Dog {
      name
    }
    fragment dogFragmentB on Dog {
      nickname
    }
    `,
					want: []want{
						{At: []at{{4, 25}, {5, 25}}},
					},
				},
			},
		},
		{
			name: `Defer and stream with no label`,
			steps: []portedStep{
				{
					query: `
      {
        dog {
          ...dogFragment @defer
        }
        pets @stream(initialCount: 0) @stream {
          name
        }
      }
      fragment dogFragment on Dog {
        name
      }
    `,
				},
			},
		},
		{
			name: `Stream with null label`,
			steps: []portedStep{
				{
					query: `
      {
        pets @stream(label: null) {
          name
        }
      }
    `,
				},
			},
		},
		{
			name: `Stream with variable label`,
			steps: []portedStep{
				{
					query: `
      query ($label: String!) {
        dog {
          ...dogFragment @defer
        }
        pets @stream(initialCount: 0) @stream(label: $label) {
          name
        }
      }
      fragment dogFragment on Dog {
        name
      }
      `,
					want: []want{
						{At: []at{{6, 39}}},
					},
				},
			},
		},
		{
			name: `Defer and stream with the same label`,
			steps: []portedStep{
				{
					query: `
      {
        dog {
          ...dogFragment @defer(label: "MyLabel")
        }
        pets @stream(initialCount: 0) @stream(label: "MyLabel") {
          name
        }
      }
      fragment dogFragment on Dog {
        name
      }
      `,
					want: []want{
						{At: []at{{4, 26}, {6, 39}}},
					},
				},
			},
		},
	})
}
