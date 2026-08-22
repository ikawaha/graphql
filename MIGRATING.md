# Migrating from graphql-go/graphql

[graphql-go/graphql](https://github.com/graphql-go/graphql) answers the same
requests this library does, and a server moving between them keeps its shape:
a schema, a resolver per field, one call per request.

What changes falls into two kinds. Most of it is spelling — a map becomes a
slice, a config struct gains a field, a parameter moves. The compiler finds all
of that. The rest is a handful of places where the two libraries give different
answers to the same request, because this one follows the specification and
graphql-js where graphql-go departs from them. Those do not stop a build, so
they are worth reading before starting: [Where the answers
change](#where-the-answers-change).

## The request loop

```go
// graphql-go/graphql
result := graphql.Do(graphql.Params{
    Schema:         schema,               // a Schema value
    RequestString:  query,
    VariableValues: vars,                 // map[string]any
    OperationName:  name,
    RootObject:     root,                 // map[string]any
    Context:        ctx,
})
```

```go
// this library
result := graphql.Do(ctx, graphql.Params{  // the context is the first argument
    Schema:        s,                      // a *Schema
    Query:         query,
    Variables:     vars,                   // map[string]graphql.Maybe[any]
    OperationName: name,
    RootValue:     root,                   // any, not just a map
})
```

Both return a value that marshals to the response the specification describes,
so the code that writes it out is unchanged.

`Variables` is the one parameter that is not a rename. A request's variables
are three-state — absent, null, or a value — so the map holds `Maybe`. A body
decoded straight from JSON already gets this right, since a key that was not
sent is not in the map:

```go
var body struct {
    Query         string                        `json:"query"`
    OperationName string                        `json:"operationName"`
    Variables     map[string]graphql.Maybe[any] `json:"variables"`
}
```

`graphql.Variables(m)` converts a `map[string]any` a Go caller built itself.
Every key in that map counts as supplied, including one holding nil, which
means null.

## The schema

graphql-go has no SDL, so every schema written against it is built in Go. Both
routes are open here, and the choice can be deferred: build it in Go as it is
now, get the server running, and move to SDL later if that reads better.

### Keeping it in Go

The `schema` package is the counterpart of graphql-go's top-level constructors.
Two shape changes run through all of it:

- **Fields and arguments are slices, not maps.** `graphql.Fields{"id": …}`
  becomes `[]*schema.Field{schema.NewField("id", …)}`. The order is kept, so
  introspection and a printed schema list fields in the order they were
  written rather than in whatever order a map ranged.
- **Descriptions are `value.Maybe[string]`.** `value.Just("…")` supplies one;
  the zero value means undocumented, which is a different thing from
  documented with the empty string.

```go
role := schema.NewEnum(schema.EnumConfig{
    Name: "Role",
    Values: []*schema.EnumValue{
        schema.NewEnumValue("ADMIN", schema.EnumValueConfig{Value: schema.InternalValue(1)}),
        schema.NewEnumValue("GUEST", schema.EnumValueConfig{Value: schema.InternalValue(2)}),
    },
})

user := schema.NewObject(schema.ObjectConfig{
    Name:       "User",
    Interfaces: schema.Implements(node),
    Fields: []*schema.Field{
        schema.NewField("id", schema.FieldConfig{Type: schema.NewNonNull(schema.ID)}),
        schema.NewField("role", schema.FieldConfig{Type: role}),
    },
})

query := schema.NewObject(schema.ObjectConfig{
    Name: "Query",
    Fields: []*schema.Field{
        schema.NewField("user", schema.FieldConfig{
            Type: user,
            Args: []*schema.Argument{
                schema.NewArgument("id", schema.ArgumentConfig{
                    Type:    schema.NewNonNull(schema.ID),
                    Default: schema.DefaultValue("1"),
                }),
            },
            Resolve: func(
                ctx context.Context, source any, args schema.Arguments, info *schema.ResolveInfo,
            ) (any, error) {
                id, _ := args.Get("id")
                return findUser(ctx, id.(string))
            },
        }),
    },
})

s := schema.New(schema.Config{Query: query, Types: []schema.NamedType{user}})
if errs := schema.ValidateSchema(s); len(errs) != 0 {
    log.Fatal(errs[0])
}
```

A type that refers to itself, or to one declared after it, uses the thunk
beside the list — `ObjectConfig.FieldsThunk` beside `ObjectConfig.Fields` —
where graphql-go uses `graphql.FieldsThunk`.

### Moving it to SDL

A schema already built in Go can be printed and then read back, which turns
this into a mechanical step rather than a retyping job:

```go
os.WriteFile("schema.graphql", []byte(utilities.PrintSchema(s)), 0o644)
```

From then on the schema is text, and the server reaches into it for the fields
it wants to answer:

```go
s, err := graphql.BuildSchema(sdl)
if err != nil {
    log.Fatal(err)
}
s.QueryType().Field("user").Resolve = resolveUser
```

`BuildSchema` checks the document against the rules a schema definition must
follow before building anything from it, so a type defined twice or a directive
nothing declares is reported rather than carried forward.

## Name by name

| graphql-go/graphql | here |
|---|---|
| `graphql.Do(Params{…, Context: ctx})` | `graphql.Do(ctx, Params{…})` |
| `Params.RequestString` | `Params.Query` |
| `Params.VariableValues map[string]any` | `Params.Variables map[string]Maybe[any]` |
| `Params.RootObject map[string]any` | `Params.RootValue any` |
| `graphql.NewSchema(SchemaConfig) (Schema, error)` | `schema.New(Config) *Schema` plus `schema.ValidateSchema` |
| `graphql.NewObject(ObjectConfig)` | `schema.NewObject(schema.ObjectConfig)` |
| `graphql.Fields{"id": &graphql.Field{…}}` | `[]*schema.Field{schema.NewField("id", schema.FieldConfig{…})}` |
| `graphql.FieldConfigArgument{"id": &ArgumentConfig{…}}` | `[]*schema.Argument{schema.NewArgument("id", schema.ArgumentConfig{…})}` |
| `ArgumentConfig.DefaultValue: v` | `ArgumentConfig.Default: schema.DefaultValue(v)` |
| `graphql.NewInterface` / `NewUnion` / `NewEnum` / `NewInputObject` / `NewScalar` | the same names in `schema` |
| `UnionConfig.Types: []*Object{a, b}` | `UnionConfig.Types: schema.Members(a, b)` |
| `ObjectConfig.Interfaces: []*Interface{n}` | `ObjectConfig.Interfaces: schema.Implements(n)` |
| `EnumValueConfig.Value: v` | `EnumValueConfig.Value: schema.InternalValue(v)` |
| `graphql.NewList` / `NewNonNull` | `schema.NewList` / `schema.NewNonNull` |
| `graphql.String` / `Int` / `Float` / `Boolean` / `ID` | the same names in `schema` |
| `graphql.DateTime` | not built in; see [What is not here](#what-is-not-here) |
| `func(p ResolveParams) (any, error)` | `func(ctx, source any, args schema.Arguments, info *schema.ResolveInfo) (any, error)` |
| `p.Args["x"]` | `args.Get("x")`, which also says whether it was supplied |
| `p.Source`, `p.Context`, `p.Info` | the `source`, `ctx` and `info` parameters |
| `ScalarConfig.Serialize` | `ScalarConfig.CoerceOutputValue` |
| `ScalarConfig.ParseValue` | `ScalarConfig.CoerceInputValue` |
| `ScalarConfig.ParseLiteral` | `ScalarConfig.CoerceInputLiteral` |
| `IsTypeOfFn func(IsTypeOfParams) bool` | `func(ctx, v any, info *ResolveInfo) (bool, error)` |
| `ResolveTypeFn` returning `*Object` | `TypeResolver` returning the type's name |
| `graphql.Subscribe(Params) chan *Result` | `graphql.Subscribe(ctx, Params) SubscriptionResult` |
| `gqlerrors.FormattedError` | `*gqlerror.Error` |

## Where the answers change

These compile either way. Each is a case where graphql-go and this library
disagree, and in each the answer here is graphql-js's.

### A `json` struct tag no longer names a field

graphql-go's default resolver reads a struct field by its name ignoring case,
then by a `json` or a `graphql` tag. This library reads the `graphql` tag, and
failing that the field name ignoring case. `json` is not consulted.

Most structs are unaffected, because the Go name and the GraphQL name usually
match ignoring case and the tag was never doing the work. The exception is a
tag that *renames*:

```go
type User struct {
    Name string `json:"displayName"`
}
```

Asked for `displayName`, graphql-go answers `"Ada"` and this library answers
`null` — no error, just null. **This is the one failure here that is silent**,
so it is worth grepping for `json:"` on any type a resolver returns before
switching. The fix is to add the tag this library reads:

```go
type User struct {
    Name string `json:"displayName" graphql:"displayName"`
}
```

A field kept out of the schema uses `graphql:"-"`.

### A variable sent as null

A variable with a default, sent explicitly as null:

```graphql
query ($n: String = "default") { echo(n: $n) }
```

with `{"n": null}`. graphql-go resolves `n` to `"default"`; the default is for
a variable that was *not supplied*, and null is a value. This library resolves
it to null, which is what the specification asks and what graphql-js does.

A server that relied on the old behaviour to mean "reset to the default" has to
say so another way.

### Response keys come back in the order asked

graphql-go builds its data as a `map[string]any`, so `encoding/json` writes the
keys in alphabetical order: `{ id fullName nickname }` comes back as
`fullName`, `id`, `nickname`. Here the response is a `value.OrderedMap` and the
keys follow the document, as the specification requires. A client that sorted
its own way is unaffected; a golden-file test comparing response bytes is not.

### `data` is absent when the request never ran

On a syntax error graphql-go answers `{"data":null,"errors":[…]}`. The
specification asks for `data` to be *absent* when the request failed before
execution could begin, and present-but-null only when a non-null field failed
during it. This library tells the two apart, which is why `Result.Data` is a
`value.Maybe` rather than a pointer that might be nil.

### Message wording

Every message a request can come back with is graphql-js's, checked against it
rather than assumed. They are not graphql-go's. A test asserting on message
text will need its expectations replaced — including syntax errors, which
graphql-go renders with an excerpt of the source inside the message and this
library keeps as one line.

### Errors are a type, not a formatted struct

`[]gqlerrors.FormattedError` becomes `[]*gqlerror.Error`. It implements `error`,
and `Unwrap` reaches the cause, so `errors.Is` and `errors.As` see through it to
whatever a resolver returned.

### A schema's mistakes are reported by `ValidateSchema`

graphql-go's constructors record an error inside the type and `NewSchema`
returns it. Here, building never fails — `schema.New` returns a schema — and
`schema.ValidateSchema` says what is wrong with it. Call it once at startup.
A schema read with `BuildSchema` is already checked.

### Subscriptions

graphql-go returns one `chan *Result` that carries both a failure to start and
the events. Here the two are separate, because a server does different things
with them:

```go
sub := graphql.Subscribe(ctx, graphql.Params{Schema: s, Query: q})
if sub.Events == nil {
    return sub.Errors      // could not start
}
for result := range sub.Events {
    send(result)
}
```

**The payload means something different.** graphql-go, like graphql-js, makes
each event the root value and resolves the root field from it, so a server
publishes envelopes — `{"messageAdded": msg}` — and the default resolver reads
the key out. Here the event *is* the root field's value, because a Go
subscriber naturally returns `<-chan Message` rather than a channel of
envelopes. A server that publishes envelopes keeps working by giving the root
field a resolver, which is called with the event exactly as in graphql-go:

```go
s.SubscriptionType().Field("messageAdded").Resolve = func(
    _ context.Context, event any, _ graphql.Arguments, _ *graphql.ResolveInfo,
) (any, error) {
    return event.(map[string]any)["messageAdded"], nil
}
```

graphql-go also requires the `Subscribe` resolver to return exactly a
`chan interface{}`; here any channel that can be received from will do,
including a typed one.

## What is not here

**Per-field and result-level extension data.** graphql-go's `Extension`
interface has four hooks. Three of them — `ParseDidStart`,
`ValidationDidStart`, `ExecutionDidStart` — are `Params.Harness` here, which
stands in for any of those stages:

```go
graphql.Do(ctx, graphql.Params{
    Schema: s, Query: q,
    Harness: &graphql.Harness{
        Execute: func(ctx context.Context, req execution.Request) execution.Result {
            defer trace(ctx, "execute")()
            return execution.Execute(ctx, req)
        },
    },
})
```

What is missing is the fourth, `ResolveFieldDidStart`, which means wrapping the
resolvers; and `Result` has no `Extensions` member, so a server wanting one in
the response adds it when it writes the response out.

**The `DateTime` scalar.** graphql-go ships one; graphql-js does not, so
neither does this. Written out, with the three things worth getting right:

```go
var DateTime = schema.NewScalar(schema.ScalarConfig{
    Name:        "DateTime",
    Description: value.Just("An RFC 3339 date and time, to nanosecond precision."),
    CoerceOutputValue: func(internal any) (value.Maybe[any], error) {
        t, ok := internal.(time.Time)
        if !ok {
            return value.Nothing[any](), nil
        }
        text, err := t.MarshalText()
        if err != nil {
            return value.Nothing[any](), err
        }
        return value.Just[any](string(text)), nil
    },
    CoerceInputValue: func(external any) (value.Maybe[any], error) {
        s, ok := external.(string)
        if !ok {
            return value.Nothing[any](), nil
        }
        var t time.Time
        if err := t.UnmarshalText([]byte(s)); err != nil {
            return value.Nothing[any](), err
        }
        return value.Just[any](t), nil
    },
    ValueToLiteral: func(external any, _ schema.Type) (language.Value, error) {
        s, ok := external.(string)
        if !ok {
            return nil, fmt.Errorf("DateTime: expected a string, got %T", external)
        }
        var t time.Time
        if err := t.UnmarshalText([]byte(s)); err != nil {
            return nil, err
        }
        return &language.StringValue{Value: s}, nil
    },
})
```

Answering with nothing says the value does not fit; returning an error says so
in the coercer's own words, which is what the caller sees.

**`MarshalText`, not `Format(time.RFC3339)`.** That layout carries no
fractional second, so formatting with it drops the sub-second part silently.
`MarshalText` writes RFC 3339 to nanoseconds and also refuses the timestamps Go
can hold but the format cannot carry — a year outside `[0,9999]`, an offset of
a day or more — which `Format` would emit as a string no client can parse. It
is what graphql-go's own scalar uses, and what `encoding/json` does for a
`time.Time`, so the wire format is unchanged by the move.

Trailing zeros are trimmed, so a whole second is written without a fraction and
a tenth as `.1`. A schema promising a constant width wants the layout spelt out
— `"2006-01-02T15:04:05.000000000Z07:00"` — on output only; parsing with it
would reject every timestamp that has no fraction.

**A default is the external value.** `ArgumentConfig.Default` holds the value
in the form a caller would send, not the form a resolver receives, so it is the
string:

```go
schema.NewArgument("since", schema.ArgumentConfig{
    Type:    DateTime,
    Default: schema.DefaultValue("2024-01-01T00:00:00Z"),
})
```

A default is checked by running it through the *input* coercer, so a
`time.Time` there is rejected by `ValidateSchema`, which names the string it
should have been. `ValueToLiteral` is what writes the default back out when the
schema is printed, and it too is handed the external value — omit it and the
default validates but goes missing from the printed schema.

**`*time.Time` needs no handling.** graphql-go's scalar has a case for it; here
the executor follows the pointer before the coercer is reached, and a nil one
is null.

**A `func() interface{}` in a source map.** graphql-go calls a map value of
that type. Here it is returned as it is; use a resolver.

## What comes with the move

- `@defer` and `@stream`, through `graphql.DoIncrementally`.
- Schemas as SDL: `BuildSchema`, `PrintSchema`, `ExtendSchema`, and a
  round trip through introspection with `Introspect` and `BuildClientSchema`.
- 32 rules a document is checked against, against graphql-go's 24, and 15 more
  for a schema definition. `Params.Rules` takes a rule of a server's own
  alongside them.
- `Params.Concurrency`, which bounds how many fields of an object and how many
  entries of a list are worked on at once.
- `language.MaxTokens`, which bounds what a request can cost before any of it
  runs, and `HideSuggestions`, which keeps "Did you mean …?" from naming
  schema members a server does not want introspected.
- `oneOf` input objects, and `@specifiedBy` on custom scalars.

See [COMPATIBILITY.md](COMPATIBILITY.md) for how this library stands to
graphql-js itself.
