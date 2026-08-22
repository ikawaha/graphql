# graphql

A Go port of [graphql-js](https://github.com/graphql/graphql-js), the reference
implementation of GraphQL, targeting v17.

Parsing, validation, execution, subscriptions, introspection and `@defer` /
`@stream`, with no dependencies outside the standard library.

## Requirements

Go 1.24 or later. The `omitzero` JSON tag is what lets a field that was not
supplied be told apart from one supplied as null, which GraphQL requires.

## Answering a request

```go
s, err := graphql.BuildSchema(`
    type Query {
        user(id: ID!): User
    }
    type User {
        id: ID!
        name: String
        nickname: String
    }
`)
if err != nil {
    log.Fatal(err)
}

s.QueryType().Field("user").Resolve = func(
    ctx context.Context, _ any, args graphql.Arguments, _ *graphql.ResolveInfo,
) (any, error) {
    id, _ := args.Get("id")
    return findUser(ctx, id.(string))
}

result := graphql.Do(ctx, graphql.Params{
    Schema:    s,
    Query:     `query ($id: ID!) { user(id: $id) { name nickname } }`,
    Variables: graphql.Variables(map[string]any{"id": "1"}),
})
json.NewEncoder(w).Encode(result)
```

`BuildSchema` checks the document before building anything from it — a type
defined twice, a field defined twice, a directive nothing declares — and says
what is wrong in graphql-js's own words. A document already known to be sound
can skip the check through `utilities.BuildSchema`, which takes
`utilities.AssumeValidSDL`.

`Do` parses the text, checks that the schema can answer it, and runs it. The
result marshals to the response the specification describes, so a server can
write it out as it is.

## Where values come from

A field with no resolver reads its value from whatever the enclosing field
returned. JavaScript makes that a property access; Go has no such thing, so
what stands in for it is spelt out:

```go
type User struct {
    ID       string  `graphql:"id"`
    Name     string
    Nickname *string          // a field that can be null needs a type that can be nil
    Email    string  `graphql:"-"`   // not exposed
}

func (u *User) Greeting() string { return "hello, " + u.Name }
```

- a `map[string]any` is read by the field's name;
- a struct is read by a field tagged `graphql:"name"`, or one whose name
  matches ignoring case;
- a method of the matching name is called, optionally taking a
  `context.Context`, the field's `graphql.Arguments`, or both, and optionally
  returning an error;
- a pointer is followed, and a nil one is null;
- a `__typename` key names which object type a value is, where the schema has
  no other way of telling.

`Params.FieldResolver` replaces this for every field at once, which is what a
server whose values all come from one place wants.

Resolvers run one after another, in document order, until asked otherwise.
`Params.Concurrency` bounds how many fields of an object, and how many entries
of a list, are worked on at once; above one, resolvers must be safe to call
from several goroutines. A list of eight entries that each wait 100 µs takes
about 1 ms one at a time and about 140 µs at eight.

## Omitted, null, and a value

GraphQL distinguishes a variable that was not supplied from one supplied as
null: the first falls back to a default and the second does not. Go's `nil`
cannot tell them apart, so this library keeps them apart itself.

`nil` always means GraphQL null. Omission is the absence of a map key, or
`value.Maybe[T]`. A resolver asks with the second return value:

```go
if v, supplied := args.Get("filter"); !supplied {
    // the caller left it out
} else if v == nil {
    // the caller sent null
}
```

A request's variables come in as `map[string]graphql.Maybe[any]`, which a body
decoded from JSON already gets right — a key that was not sent is not in the
map:

```go
var body struct {
    Query         string                        `json:"query"`
    OperationName string                        `json:"operationName"`
    Variables     map[string]graphql.Maybe[any] `json:"variables"`
}
```

An object *inside* a variable arrives as `*value.OrderedMap` rather than as a
Go map, so that the order the request wrote its keys in survives: a message
naming the value writes them back in that order, as graphql-js does. Coercion
reads it as an object like any other, so a schema written against
`map[string]any` is unaffected — only a resolver taking a custom scalar's raw
value sees the difference.

Response objects are `value.OrderedMap` too, because the specification asks for
the keys of an object to follow the query and a Go map has no order.

## Subscriptions

The root field's `Subscribe` resolver returns a channel of whatever the
server's own machinery produces. Each event is answered as though the
operation were a query against it.

```go
s.SubscriptionType().Field("messageAdded").Subscribe = func(
    ctx context.Context, _ any, args graphql.Arguments, _ *graphql.ResolveInfo,
) (any, error) {
    return broker.Subscribe(ctx), nil    // <-chan Message
}

sub := graphql.Subscribe(ctx, graphql.Params{Schema: s, Query: q})
if sub.Events == nil {
    return sub.Errors
}
for result := range sub.Events {
    send(result)
}
```

A field with no `Subscribe` of its own falls back to `Params.SubscribeResolver`
and then to the default resolver, so putting the channel in the root value is
enough when a server has nothing else to say:

```go
graphql.Subscribe(ctx, graphql.Params{
    Schema: s, Query: q,
    RootValue: map[string]any{"messageAdded": broker.Subscribe(ctx)},
})
```

The field's ordinary `Resolve` is not in that list. A subscription root field
has two jobs — producing the stream, and turning each event into the field's
value — and the specification gives it a separate function for each. `Resolve`
is the second, so a server can have both: the channel from the root value, and
a resolver called once per event.

Cancelling the context ends the subscription.

## @defer and @stream

A schema declares the two directives; `graphql.DoIncrementally` then delivers
what they ask to be delayed after the rest.

```go
result := graphql.DoIncrementally(ctx, graphql.Params{Schema: s, Query: q})
if result.Subsequent == nil {
    return send(result.Initial)   // nothing was deferred: an ordinary response
}
send(result.Initial)
for payload := range result.Subsequent {
    send(payload)
}
```

A nil `Subsequent` is how an ordinary response is told from one that has more
to come, which is the fork a server takes before it decides what to write.

The deferred part does not run until it is ranged over, and it runs on the
goroutine that ranges. A server that writes the first response and stops —
because the client hung up — has not paid for the rest.

`graphql.DoLegacyIncrementally` answers in the payload format that came before
the current one, for a client written against the earlier draft.

## Beyond one request at a time

`Do` is the way in. Everything it does is done by packages that can also be
used on their own, which is what a server wants once it parses once and runs
many times, or wants a schema tool of its own:

| package | what it is for |
|---|---|
| `schema` | describing a schema, in Go or from SDL; coercing and checking input values against it |
| `language` | parsing and printing documents |
| `validation` | checking a document against a schema — 44 rules |
| `execution` | running one; the resolver API lives here |
| `utilities` | building, printing, extending, sorting and comparing schemas |
| `gqlerror` | the error a response carries |
| `value` | `Maybe`, `OrderedMap`, response paths and `Describe` |

A schema written in Go rather than read from SDL uses the `schema` package
directly. Two things there are worth knowing before starting: `schema.Members`
and `schema.Implements` are how a union is given its members and a type its
interfaces, and `Description` takes a `value.Maybe[string]`, so that a type
documented with the empty string can be told from one documented not at all.

Things a server is likely to want from them:

- `language.MaxTokens` bounds what a request can cost before any of it runs.
- `utilities.StripIgnoredCharacters` turns two spellings of one request into
  one string, which is what makes a cache key.
- `utilities.FindOperation` says which operation a request would run, and
  `utilities.ConcatDocuments` joins a schema written across several files into
  the one document that builds it.
- `utilities.FindSchemaChanges` says which changes between two schemas would
  break a client, naming each by its schema coordinate.
- `graphql.Introspect` and `utilities.BuildClientSchema` are the two halves of
  a schema round trip: what a server says about itself, and the schema a client
  rebuilds from it.

## Differences from graphql-js

Every deliberate one is listed in [COMPATIBILITY.md](COMPATIBILITY.md), with
why. The short version: three-state values are explicit, positions are byte
offsets rather than UTF-16 code units, and where graphql-js's answer depends on
the order its promises settled there is nothing here to reproduce it with.

Message wording is not among the differences. One corpus goes through both
implementations and the answers are compared as text — 1,197 validation
messages, and every response the execution corpora produce. What still differs
is listed, and asserted to *still* differ, so closing one cannot go unnoticed.

## Coming from graphql-go/graphql

[MIGRATING.md](MIGRATING.md) maps the API across name by name, and lists the
places where the two libraries give different answers to the same request —
which is the part worth reading first, since none of them stop a build.

## Licence

MIT, as graphql-js is. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
