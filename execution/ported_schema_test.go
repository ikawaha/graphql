package execution_test

// Ported from graphql-js src/execution/__tests__/schema-test.ts: one query
// against a small blog schema, deliberately not validated first, so that what
// the executor does with a document asking for fields that do not exist is
// part of what is checked.

import (
	"context"
	"fmt"
	"testing"

	"github.com/ikawaha/graphql/schema"
)

func TestPortedSchema(t *testing.T) {
	runPorted(t, blogSchema(t), nil, nil, []portedCase{{
		name: `defines a query only schema`,
		query: `
      {
        feed {
          id,
          title
        },
        article(id: "1") {
          ...articleFields,
          author {
            id,
            name,
            pic(width: 640, height: 480) {
              url,
              width,
              height
            },
            recentArticle {
              ...articleFields,
              keywords
            }
          }
        }
      }

      fragment articleFields on Article {
        id,
        isPublished,
        title,
        body,
        hidden,
        notDefined
      }
    `,
		want: `{"data": {
			"feed": [
				{"id": "1", "title": "My Article 1"}, {"id": "2", "title": "My Article 2"},
				{"id": "3", "title": "My Article 3"}, {"id": "4", "title": "My Article 4"},
				{"id": "5", "title": "My Article 5"}, {"id": "6", "title": "My Article 6"},
				{"id": "7", "title": "My Article 7"}, {"id": "8", "title": "My Article 8"},
				{"id": "9", "title": "My Article 9"}, {"id": "10", "title": "My Article 10"}],
			"article": {
				"id": "1", "isPublished": true, "title": "My Article 1", "body": "This is a post",
				"author": {
					"id": "123", "name": "John Smith",
					"pic": {"url": "cdn://123", "width": 640, "height": 480},
					"recentArticle": {
						"id": "1", "isPublished": true, "title": "My Article 1",
						"body": "This is a post",
						"keywords": ["foo", "bar", "1", "true", null]}}}}}`,
	}})
}

// blogSchema is graphql-js's own schema from schema-test.ts.
func blogSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s := buildPorted(t, `
		type Image { url: String width: Int height: Int }
		type Author {
			id: String
			name: String
			pic(width: Int, height: Int): Image
			recentArticle: Article
		}
		type Article {
			id: String!
			isPublished: Boolean
			author: Author
			title: String
			body: String
			keywords: [String]
		}
		type Query {
			article(id: ID): Article
			feed: [Article]
		}
	`)

	s.QueryType().Field("article").Resolve = func(
		_ context.Context, _ any, args schema.Arguments, _ *schema.ResolveInfo,
	) (any, error) {
		id, _ := args.Get("id")
		return blogArticle(id), nil
	}
	s.QueryType().Field("feed").Resolve = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		feed := make([]any, 0, 10)
		for id := 1; id <= 10; id++ {
			feed = append(feed, blogArticle(id))
		}
		return feed, nil
	}
	// An author's recent article is written lazily in graphql-js, since an
	// article holds an author who holds an article. A resolver is what stands
	// in for that here; building it eagerly would never finish.
	objectOf(t, s, "Author").Field("recentArticle").Resolve = func(
		context.Context, any, schema.Arguments, *schema.ResolveInfo,
	) (any, error) {
		return blogArticle(1), nil
	}
	// graphql-js reads pic from the value by calling it with the arguments; a
	// Go value has no way to be called with them, so the field says how.
	objectOf(t, s, "Author").Field("pic").Resolve = func(
		_ context.Context, source any, args schema.Arguments, _ *schema.ResolveInfo,
	) (any, error) {
		width, _ := args.Get("width")
		height, _ := args.Get("height")
		author, _ := source.(map[string]any)
		return map[string]any{
			"url":    fmt.Sprintf("cdn://%v", author["id"]),
			"width":  fmt.Sprintf("%v", width),
			"height": fmt.Sprintf("%v", height),
		}, nil
	}
	return s
}

// blogArticle is one article, with an author who wrote another one. The values
// are deliberately of the wrong Go types here and there — a number where the
// schema says String, a string where it says Int — because coercing them on
// the way out is part of what the case is about.
func blogArticle(id any) map[string]any {
	return map[string]any{
		"id":          id,
		"isPublished": true,
		"author": map[string]any{
			"id":   123,
			"name": "John Smith",
		},
		"title":    fmt.Sprintf("My Article %v", id),
		"body":     "This is a post",
		"hidden":   "This data is not exposed in the schema",
		"keywords": []any{"foo", "bar", 1, true, nil},
	}
}
