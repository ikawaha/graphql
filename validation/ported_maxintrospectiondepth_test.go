package validation_test

import (
	"testing"

	"github.com/ikawaha/graphql/validation"
)

// Ported from graphql-js src/validation/__tests__/MaxIntrospectionDepthRule-test.ts
// (MIT, Copyright (c) GraphQL Contributors; see the NOTICE file).
func TestPorted_MaxIntrospectionDepth(t *testing.T) {
	runPorted(t, validation.MaxIntrospectionDepthRule, []portedCase{
		{
			name: `3 flat fields introspection query`,
			steps: []portedStep{
				{
					query: `
    {
      __type(name: "Query") {
        trueFields: fields(includeDeprecated: true) {
          name
        }
        falseFields: fields(includeDeprecated: false) {
          name
        }
        omittedFields: fields {
          name
        }
      }
    }
    `,
				},
			},
		},
		{
			name: `3 fields deep introspection query from __schema`,
			steps: []portedStep{
				{
					query: `
    {
      __schema {
        types {
          fields {
            type {
              fields {
                type {
                  fields {
                    name
                  }
                }
              }
            }
          }
        }
      }
    }
    `,
					want: []want{
						{At: []at{{3, 7}}},
					},
				},
			},
		},
		{
			name: `3 interfaces deep introspection query from __schema`,
			steps: []portedStep{
				{
					query: `
    {
      __schema {
        types {
          interfaces {
            interfaces {
              interfaces {
                name
              }
            }
          }
        }
      }
    }
    `,
					want: []want{
						{At: []at{{3, 7}}},
					},
				},
			},
		},
		{
			name: `3 possibleTypes deep introspection query from __schema`,
			steps: []portedStep{
				{
					query: `
    {
      __schema {
        types {
          possibleTypes {
            possibleTypes {
              possibleTypes {
                name
              }
            }
          }
        }
      }
    }
    `,
					want: []want{
						{At: []at{{3, 7}}},
					},
				},
			},
		},
		{
			name: `3 inputFields deep introspection query from __schema`,
			steps: []portedStep{
				{
					query: `
    {
      __schema {
        types {
          inputFields {
            type {
              inputFields {
                type {
                  inputFields {
                    type {
                      name
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
    `,
					want: []want{
						{At: []at{{3, 7}}},
					},
				},
			},
		},
		{
			name: `3 fields deep introspection query from multiple __schema`,
			steps: []portedStep{
				{
					query: `
    {
      one: __schema {
        types {
          fields {
            type {
              fields {
                type {
                  fields {
                    name
                  }
                }
              }
            }
          }
        }
      }
      two: __schema {
        types {
          fields {
            type {
              fields {
                type {
                  fields {
                    name
                  }
                }
              }
            }
          }
        }
      }
      three: __schema {
        types {
          fields {
            type {
              fields {
                type {
                  fields {
                    name
                  }
                }
              }
            }
          }
        }
      }
    }
    `,
					want: []want{
						{At: []at{{3, 7}}},
						{At: []at{{18, 7}}},
						{At: []at{{33, 7}}},
					},
				},
			},
		},
		{
			name: `3 fields deep introspection query from __type`,
			steps: []portedStep{
				{
					query: `
    {
      __type(name: "Query") {
        types {
          fields {
            type {
              fields {
                type {
                  fields {
                    name
                  }
                }
              }
            }
          }
        }
      }
    }
    `,
					want: []want{
						{At: []at{{3, 7}}},
					},
				},
			},
		},
		{
			name: `3 fields deep introspection query from multiple __type`,
			steps: []portedStep{
				{
					query: `
    {
      one: __type(name: "Query") {
        types {
          fields {
            type {
              fields {
                type {
                  fields {
                    name
                  }
                }
              }
            }
          }
        }
      }
      two: __type(name: "Query") {
        types {
          fields {
            type {
              fields {
                type {
                  fields {
                    name
                  }
                }
              }
            }
          }
        }
      }
      three: __type(name: "Query") {
        types {
          fields {
            type {
              fields {
                type {
                  fields {
                    name
                  }
                }
              }
            }
          }
        }
      }
    }
    `,
					want: []want{
						{At: []at{{3, 7}}},
						{At: []at{{18, 7}}},
						{At: []at{{33, 7}}},
					},
				},
			},
		},
		{
			name: `1 fields deep with 3 fields introspection query`,
			steps: []portedStep{
				{
					query: `
    {
      __schema {
        types {
          fields {
            type {
              oneFields: fields {
                name
              }
              twoFields: fields {
                name
              }
              threeFields: fields {
                name
              }
            }
          }
        }
      }
    }
    `,
				},
			},
		},
		{
			name: `3 fields deep from varying parents introspection query`,
			steps: []portedStep{
				{
					query: `
    {
      __schema {
        types {
          fields {
            type {
              fields {
                type {
                  ofType {
                    fields {
                      name
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
    `,
					want: []want{
						{At: []at{{3, 7}}},
					},
				},
			},
		},
		{
			name: `3 fields deep introspection query with inline fragments`,
			steps: []portedStep{
				{
					query: `
    query test {
      __schema {
        types {
          ... on __Type {
            fields {
              type {
                ... on __Type {
                  ofType {
                    fields {
                      type {
                        ... on __Type {
                          fields {
                            name
                          }
                        }
                      }
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
    `,
					want: []want{
						{At: []at{{3, 7}}},
					},
				},
			},
		},
		{
			name: `3 fields deep introspection query with fragments`,
			steps: []portedStep{
				{
					query: `
    query test {
      __schema {
        types {
          ...One
        }
      }
    }

    fragment One on __Type {
      fields {
        type {
          ...Two
        }
      }
    }

    fragment Two on __Type {
      fields {
        type {
          ...Three
        }
      }
    }

    fragment Three on __Type {
      fields {
        name
      }
    }
    `,
					want: []want{
						{At: []at{{3, 7}}},
					},
				},
			},
		},
		{
			name: `3 fields deep inside inline fragment on query`,
			steps: []portedStep{
				{
					query: `
    {
      ... {
        __schema { types { fields { type { fields { type { fields { name } } } } } } }
      }
    }
    `,
					want: []want{
						{At: []at{{4, 9}}},
					},
				},
			},
		},
		{
			name: `opts out if fragment is missing`,
			steps: []portedStep{
				{
					query: `
    query test {
      __schema {
        types {
          ...Missing
        }
      }
    }
    `,
				},
			},
		},
		{
			name: `doesn't infinitely recurse on fragment cycle`,
			steps: []portedStep{
				{
					query: `
    query test {
      __schema {
        types {
          ...Cycle
        }
      }
    }
    fragment Cycle on __Type {
      ...Cycle
    }
    `,
				},
			},
		},
	})
}

// Not ported, because each of these is written in a way this could not
// follow:
//   - default introspection query: a document that is not written out
//   - all options introspection query: a document that is not written out
