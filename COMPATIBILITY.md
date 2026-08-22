# Compatibility with graphql-js

This is a port of [graphql-js](https://github.com/graphql/graphql-js) v17. It
follows the GraphQL specification, and follows graphql-js wherever the
specification leaves a choice — except where doing so would produce code that
is wrong or unpleasant in Go.

Every deliberate difference is listed here, with why. If you find one that is
not listed, it is a bug.

## Not a difference

Error message wording. It matches, and is checked rather than assumed: one
corpus goes through both implementations and the answers are compared as text,
1,197 validation messages among them, along with every response the execution
corpora produce. Structure — which rule reported the problem, where in the
document it points, and the response path it happened at — is compared the
same way.

The few that do differ are listed below, each with why, and each is asserted
to *still* differ, so closing one cannot go unnoticed.

## Because Go has no `undefined`

GraphQL distinguishes three states — a value not supplied, one supplied as
null, and one supplied — and JavaScript spells them `undefined`, `null`, and
the value. Go has two.

`nil` always means GraphQL null. Omission is expressed by the absence of a map
key, by a second return value, or by `value.Maybe[T]`. There is no sentinel
value standing for null: one would have to be special-cased by `reflect`, by
`encoding/json`, and by every `== nil` in user code.

Request variables therefore arrive as `map[string]value.Maybe[any]`. A map
decoded from a request body already has the distinction right, since a key that
was not sent is not in the map.

Go 1.24 or later is required: the `omitzero` JSON tag is what lets an omitted
field be written back out as omitted.

| | graphql-js | here |
|---|---|---|
| A variable not supplied | `undefined` | absent from the map |
| A variable supplied as null | `null` | `value.Just[any](nil)` |
| Response `data` when the request never ran | key absent | `value.Maybe` holding nothing |
| Nothing documented | `description: undefined` | `Description` unset |
| Documented with the empty string | `description: ''` | `Description: value.Just("")` |
| An enum member whose value is its name | `value: undefined` | `Value` unset |
| An enum member whose value is null | `value: null` | `Value: schema.InternalValue(nil)` |
| A scalar saying a value does not fit | `return undefined` | `return value.Nothing[any](), nil` |

One state Go still cannot hold is `undefined` sitting *inside* an object as a
field's value: a Go map either has a key or it does not. graphql-js can be
handed `{ value: undefined }`, and the nearest thing here is `{}`, so a message
naming such a value names the empty object instead.

## Because Go has no promises

A resolver is a plain function. It cannot return a promise, because there is
nothing to return; work that would be asynchronous in JavaScript is either done
before returning or arranged with a goroutine of the resolver's own.

Sibling fields therefore run one after another by default, and so do the
entries of a list. Setting `Request.Concurrency` above one lets both run
alongside each other, which is worth a great deal when resolvers wait on a
database — but it means resolvers must be safe to call from several goroutines
at once, which graphql-js never requires of them. That is why it is not the
default. The bound is per object and per list rather than over the request as a
whole, so a list of objects reaches it at both levels.

graphql-js needs no such setting: an asynchronous resolver hands back a promise
that is already in flight, so its siblings and the entries beside it overlap
without anyone asking. What a goroutine buys here is the same overlap.

A mutation's root fields always run in order, as the specification requires.

**Error order** is the order the document wrote the fields in, not the order
resolvers finished. Results and errors are written into slots reserved before
the work starts, so a response does not change depending on which resolver was
quickest. An error that nulls an object is reported after the errors collected
from inside that object, since that is when it comes to rest.

Every field of an object is resolved even once one of them has failed in a way
that will null the object. graphql-js has no choice about this, having already
started the work; here it is a choice, made so that a caller with no data at
all is told everything that went wrong rather than only the first thing.

**Cancellation** is `context.Context`. There is no separate abort signal:
cancelling the context ends a request, a subscription, or an incremental
delivery, and whatever was already produced is still returned.

## Because Go strings are bytes

| | graphql-js | here |
|---|---|---|
| `Location.Start` / `End` | UTF-16 code units | byte offsets |
| `Location.Column` | UTF-16 code units | runes (code points) |

Byte offsets are what indexes a Go string. Columns are counted in runes because
that is what a person reading an error message means by a column.

A lone surrogate in a source document is a syntax error in both, but here it is
detected as its WTF-8 byte sequence (`ED A0 80`–`ED BF BF`), because a Go string
can hold one and a JavaScript string cannot.

## Because JavaScript numbers are floats

A variable holding an integer too large for a float64 keeps its precision here.
`value.Maybe` reads a number as a [`json.Number`] rather than as a float64, so
a request body decoded into `map[string]value.Maybe[any]` — which is what a
server hands to `Do` — arrives with its digits intact, and `json.Number` is
accepted wherever a scalar takes an integer. graphql-js loses precision above
2^53.

[`json.Number`]: https://pkg.go.dev/encoding/json#Number

## Because Go has no `undefined`, part two

`ValueFromASTUntyped` refuses a literal that names a variable the request did
not supply, wherever the variable sits. graphql-js leaves `undefined` in its
place — inside a list or an object, the item or the field stays, holding
nothing. Go has no third thing to put there: `nil` would say null, which is a
different answer, so the literal is refused instead and whatever asked for it
falls back the way an omitted value does.

A literal handed to a custom scalar does not reach this: the variables are
replaced first — by [`schema.ReplaceVariables`], which puts the supplied values
in place and leaves out any input object field whose variable was omitted, as
graphql-js does — so what the scalar and the generic conversion both see is a
constant.

## Because a coercer has three answers

graphql-js lets a scalar's coercion answer `undefined` to mean "this value does
not fit", which is a third answer beside a value and null. A coercer here
returns `(value.Maybe[any], error)` and has the same three:

| | graphql-js | here |
|---|---|---|
| the value, null among them | `return v` | `return value.Just[any](v), nil` |
| does not fit, nothing more | `return undefined` | `return value.Nothing[any](), nil` |
| does not fit, and why | `throw` | `return value.Nothing[any](), err` |

Answering with nothing is answered as graphql-js answers undefined — the
complaint names the type and the value, `Expected value of type "Foo", found:
{}.` — while an error says it in the coercer's own words, which is usually
clearer and is what a Go coercer should reach for first. Output coercion has
the same three, and its complaint says which of nothing and null came back.

Which of the two kinds of error a coercer returns decides how it is reported.
A `*gqlerror.Error` is one the type meant as a message: it is passed on as it
is, carrying its own extensions and whatever it points at. Any other error is
one the type did not mean as a message, and it is wrapped in one naming the
type and the value — `Expected value of type "X", but encountered error "…";
found: ….` graphql-js draws the line in the same place, between a thrown
`GraphQLError` and a thrown `Error`.

A type's own complaints are therefore `gqlerror.New(…)`, and the built-in
scalars use it. One about a literal points at the literal, as graphql-js's
does; one about a value points nowhere, because a value has no place in a
document.

A message may suggest what the request might have meant — "Did you mean
\"barkVolume\"?" — and a server can turn that off. A suggestion is worked out
from the schema, so it names types, fields and enum members the request got
close to, and a server that does not answer introspection is hiding those names
on purpose.

`Params.HideSuggestions` covers a whole request. Below it,
`validation.WithoutSuggestions()` and `schema.WithoutSuggestions()` say the same
thing to the two halves separately, and `execution.Request.HideSuggestions` to
the executor. This is graphql-js's `hideSuggestions`.

SDL validation keeps its suggestions either way, as graphql-js's does: a
document of type definitions is the author's own, so there is nobody to hide
the names from.

## Deliberate simplifications

**No `ConstValueNode`.** graphql-js distinguishes constant values from ones
that may hold a variable at the type level only; at runtime they are the same
object with the same kind. Here there is one `Value`, and the parser refuses a
variable where none is allowed. `language.ContainsVariable` answers the
question at runtime.

While porting this, graphql-js's `isConstValueNode` turned out to use `.some`
where it means `.every`, so it calls `[$var, 1]` constant. This implementation
treats a value holding any variable as non-constant.

**No deprecated API.** `astFromValue` and the rest carry
`@deprecated … will be removed in v18` upstream, and are left out. That
includes the shape of a scalar's literal coercion: `InputLiteralCoercer` is
handed a constant literal and nothing else, as graphql-js's
`coerceInputLiteral` is, rather than the literal and the variables its
deprecated `parseLiteral` took. The older incremental payload format is *not*
among them — graphql-js keeps it and so does this; see the incremental
delivery section below.

**An introspection depth beyond a hundred is brought back rather than
refused.** `getIntrospectionQuery({ typeDepth })` throws above a hundred;
`utilities.WithTypeDepth` is an option on a function that returns a query and
nothing else, so it has no way to fail. It answers with a hundred — the most
graphql-js would ever have allowed — rather than quietly with the default,
which would hand back a query the caller did not ask for and could not tell
apart from one they did. Below zero the two agree: graphql-js unfolds nothing
once the level it counts down reaches zero, and so does this.

**No tracing channels, no dev mode, no version constant.** graphql-js's
`diagnostics` publishes to Node's `diagnostics_channel`, and `devMode` turns on
the cross-realm `instanceof` check that a Go build cannot need. Neither has a
counterpart. A Go program that wants to time a request reaches for
`runtime/trace`, or stands in for a stage with `Params.Harness` — graphql-js's
own harness, which is ported and does cover parse, validate and execute. There
is no `version` either: a Go module's version is the one the module system
resolved, and a constant beside it could only disagree.

## Walking a tree

**Reading a tree and rewriting one are two functions.** graphql-js has a single
`visit` that both walks and edits, told apart by what the callbacks return.
Here `Visit` walks and `Transform` rewrites, with `VisitInParallel` and
`TransformInParallel` beside them. Everything `visit` can do can be done, and
a read-only walk — which is nearly all of them — is not made to carry the
return values a rewrite needs.

`Transform` answers with the tree the transformer made, and an error. Nothing
is modified in place: the tree that comes back shares every node the
transformer left alone with the one that went in, and a rewrite that changes
nothing gives back the very tree it was handed.

Where graphql-js returns `null` from a callback to delete a node, `Transform`
has `TransformRemove`; where it returns a node to replace one, `Transform`
returns the node. Returning nil — of any type, including the nil a Go helper
hands back when it has nothing to offer — leaves the node as it was, so a tree
cannot be given a hole by accident.

A replacement that cannot go where it was offered, an `*IntValue` where the
tree holds a `*Name`, is an error rather than a corrupted tree. JavaScript has
no type to check, so graphql-js puts it in and lets whatever reads it next
fail.

**A skip ends even when another transformer edits.** `TransformInParallel` and
`VisitInParallel` show each node to every transformer in turn, and stop at the
first that answers with a replacement — what a later one would have said about
a node that is no longer there cannot be acted on. graphql-js stops there too,
but it also stops resetting the others' skip state, so a transformer that
skipped a subtree can be left skipping for the rest of the walk depending on
the order the transformers were given in. Here the skip state is settled before
anything is asked.

One difference remains. `VisitContext.Ancestors` runs from the root down to and
including the node's own parent, where graphql-js stops one short and hands the
parent separately; here the parent is both the last ancestor and the `Parent`
field, which saves a caller that wants the whole chain from putting it back
together.

## Differences you may notice in output

**`error.locations` are file coordinates.** graphql-js applies
`Source.LocationOffset` when it prints the excerpt under an error but not when
it fills in `locations`, so one error can name two different lines. Here the
offset is applied in both, which is the only reading on which a `locations`
entry means what the offset was set for. The two implementations agree unless
an offset was set.

## Differences in behaviour

**A subscription's event is the root field's value.** graphql-js makes the
event the root value and resolves the root field from it, so a server must
write `resolve: (payload) => payload` even when the event already is the value.
In Go a subscriber naturally returns `<-chan Message`, so the event is used
directly.

A server publishing envelopes — `{ messageAdded: msg }`, the graphql-js idiom —
keeps working: give the root field a resolver and it is called with the event,
exactly as in graphql-js.

**Extending or mapping a schema invalidates the old types.** A schema's types
point at one another, so changing one means rebuilding all of them.
`ExtendSchema`, `MapSchema` and `LexicographicSortSchema` return a new schema
whose types are new objects; a caller holding a type from the old schema must
look it up again. graphql-js rebuilds too, so this is not new, but it is worth
stating.

**A schema mapper is shaped differently.** graphql-js's `mapSchemaConfig` —
internal there, public here as `MapSchema` — has one hook per kind of element,
each given that element's configuration. `SchemaMapper` splits the same ground
in two: a list hook (`Fields`, `Arguments`, `EnumValues`, …) says what a type
holds, so reordering, leaving out and adding are one hook rather than three;
a configuration hook (`Object`, `Enum`, `Schema`, …) says what the element
itself is. Everything graphql-js's mappers reach is reachable, but the order
differs in one place: a list hook runs before the things in the list are
rebuilt, so `Fields` runs before `Arguments` and a field that `Fields` put
there has its arguments mapped like any other. To look at a field once its
arguments are mapped, call the thunk from the `Object` hook, which runs after
both.

**A type takes its configuration rather than borrowing it.** The lists are
copied, and so are the fields and enum members, because those record which type
they belong to; graphql-js builds its own for the same reason. Reaching a field
through the type and setting its resolver works as it reads — that is the copy
the type holds — but a field built in Go and put into a type is not the object
the type ends up with.

**An empty description is the same as none.** A description is held as a Go
string, which cannot tell `""` from unset, so a type documented with an empty
string prints as one with no documentation. graphql-js keeps the two apart.

A deprecation reason is not held this way: it is a `value.Maybe[string]`,
because whether something is deprecated and what was said about it are two
questions. `@deprecated(reason: "")` deprecates without saying why, and a bare
`@deprecated` takes the directive's own default; the reason is declared
`String!`, so writing it as null is refused when the schema is assembled, as
graphql-js refuses it. Only a schema built in Go can hold a deprecation with
nothing in it: say `schema.DeprecatedFor("…")` or `schema.NotDeprecated()`.

**A schema holds what it names, of the right kind or not.** A document may name
anything where a particular kind belongs — `schema { query: SomeInput }`,
`union U = String`, `type T implements SomeInput` all parse — and a schema
built from one holds what was named rather than refusing to be built, leaving
`ValidateSchema` to say what is wrong with it. graphql-js does the same, and
its own validator guards against the kind it declared but did not get.

Where graphql-js declares `ReadonlyArray<GraphQLObjectType>` and then lies to
its own compiler, this says what it means:

| written | held as | asked for as |
|---|---|---|
| a root type | `schema.NamedType` | `Schema.QueryType` and its siblings, which answer with nothing for a root a request cannot enter through; `Schema.DeclaredRootType` answers with whatever was named |
| a union's members | `[]schema.Declared[*schema.ObjectType]` | `UnionType.Types`, and `Schema.PossibleTypes` for the ones a value could turn out to be |
| an `implements` clause | `[]schema.Declared[*schema.InterfaceType]` | `ObjectType.Interfaces` and `InterfaceType.Interfaces` |

`schema.Declared` records what belonged there, so that reading a union's
members as interfaces will not compile, and so that a schema written in Go
cannot name the wrong kind at all: `schema.Members` and `schema.Implements`
take the kind itself. Only `schema.DeclareNamed` puts something else there,
which is what the schema builder reaches for while reading a document.
`Declared.Get` is how a reader asks for the kind that belongs, and answers
true for every one in a schema `ValidateSchema` has passed.

A document naming a type nothing defines is still refused, as graphql-js
refuses it: there is nothing to hold.

**Which object type a value is has more ways to be answered.** graphql-js asks
the abstract type's own `resolveType`, then the one given to the request, and
failing both its `defaultTypeResolver`, which reads `__typename` off the value
and otherwise asks each possible type's `isTypeOf`. A resolver that answers
with nothing ends the field there.

The same chain runs here, with two additions Go needs. A resolver answering
with the empty string has not answered — `""` is what a Go function returns
when it has nothing to say — so the next step is tried rather than the field
failing; that is also how a resolver delegates, since graphql-js's
`defaultTypeResolver` has no exported counterpart to call. And after
`__typename` and `isTypeOf` have been tried, the value's own Go type name is
taken as the answer, so a schema whose types are named after the structs
behind them needs no resolver at all. A value says its own name through a
`GraphQLTypeName() string` method, a `__typename` key, or the name of its
struct, in that order.

**A subscription's stream is a channel.** graphql-js asks a subscribe resolver
for an async iterable; here it is a channel, and the error for a resolver that
returns something else says so rather than repeating a name for something Go
does not have.

**An error is how one entry of a list fails.** A resolver says a field failed
by returning an error, but a list comes back from one resolver and there is no
second return value per entry. A `*gqlerror.Error` among the entries is a
failed entry: the entry is null and the error is reported at its path.
graphql-js reads any `Error` this way; here only `gqlerror.Error` counts, since
any Go value may happen to have an `Error` method and a server returning one as
data should get it back as data.

**A Go zero value is not null.** A struct field of type `string` answers `""`
for a field that can be null, because Go has no way to tell an unset string
from an empty one. A field that can be null needs a Go type that can be nil —
a pointer is the usual choice, and the executor follows one to the value it
points at.

## Incremental delivery

`@defer` and `@stream` are implemented, in the response format of the current
specification (`pending` / `incremental` / `completed` / `hasNext`). The
machinery is not a port: graphql-js's `WorkQueue`, `Queue` and `Computation`
exist to schedule promises, and Go has no such problem to solve.

What a client sees is what graphql-js sends, down to the identifiers: a
deferred fragment is announced once, where the `@defer` was written, and its
fields arrive under that identifier however deep they turn out to be, with
`subPath` saying how much further down. A fragment written inside another is
announced only once the enclosing one has been delivered, and what a fragment
asked for arrives together or not at all — where a piece of it fails, the whole
fragment completes with that error and none of its data is sent.

One thing follows from Go rather than from a choice:

- **A synchronous run has one payload after the first.** A resolver here
  answers rather than promising to answer, so everything a run defers is ready
  the moment it is asked for, and everything ready at the same moment goes in
  one payload. graphql-js does the same when nothing it is waiting on is a
  promise; where it is, it sends what has settled and keeps the rest for the
  next payload. A streamed list is the same story: its remaining entries are
  already in hand, so they go out together rather than one at a time.

**The older payload format is available too.** graphql-js keeps the payload
shape that came before the current one — each payload naming its own path
rather than an identifier announced beforehand — and so does this, as
`execution.ExecuteLegacyIncrementally` and `graphql.DoLegacyIncrementally`.
The two differ in more than shape: the current format works out which set of
deferred fragments each field belongs to, so a field asked for both inside a
fragment and outside it is sent once, while the older one answers each fragment
on its own and sends the field again for each fragment that asked.

`Execute` refuses a document that uses either directive, rather than answering
it with everything at once: the client is waiting for a different shape of
response. Use `ExecuteIncrementally`.

graphql-js refuses more, and differently. It refuses on what the schema
declares rather than on what the document asked: a schema declaring `@defer` or
`@stream` cannot be passed to `execute` at all, even for a document that uses
neither. Here the document decides, so such a schema still answers an ordinary
request.

And it refuses by raising rather than by answering. graphql-js throws a plain
`Error` — "The provided schema unexpectedly contains experimental directives",
or "Executing this GraphQL operation would unexpectedly produce multiple
payloads" where the schema did not declare them — so there is no response to
read. Here the refusal is a response like any other: an error, and no data.

Neither directive is one of the directives every schema has. A schema must
declare them — `schema.Defer` and `schema.Stream` are the definitions — and
their arguments are read from the schema's own declaration, so a default the
author chose is honoured.

## Schema validation

`schema.ValidateSchema` reports what is wrong with a schema and points at the
SDL responsible, as graphql-js does. A schema built in Go has no source to
point at, so its errors carry no location.

A default given in Go as a value rather than written as a literal is checked
too, and where the value looks like an already coerced one the same suggested
fix is offered — "Did you mean: …?". The fix is written out with the fields of
an input object in name order, because a Go map has none of its own and a
message that differed from one run to the next would be worse than one that
differs from graphql-js; graphql-js writes them in the order the value was
built.

graphql-js's cases for the differences above are in the test suite, listed as
known gaps, so closing one cannot go unnoticed.

A document is also checked before a schema is built from it, against the rules
graphql-js's `assertValidSDL` applies — a type or field defined twice, a
directive nothing declares, an extension of something that is not there. The
messages are graphql-js's own, unprefixed, joined as `ValidateSchema` joins
its own.

`utilities.AssumeValidSDL` skips that check, which is graphql-js's
`assumeValidSDL`. A document that would not have passed then reaches machinery
never meant to see it, and what the two make of it agrees case for case: a name
defined twice, a field or an argument written twice, a root type that is not an
object, a schema definition written twice, an extension of something nothing
defines, and a directive redeclared over one the schema already has.

## Turning off error propagation

An operation written with `@experimental_disableErrorPropagation` asks that a
field which may not be null be answered with null when it fails, rather than
the failure travelling up to the nearest place that can hold one. The response
then keeps whatever did resolve, and the promise the schema made about that one
field is broken instead — so a client asking for it has to be ready to read a
null where the schema said there would not be one.

It applies wherever a failure would otherwise travel: a field of an object, an
entry of a list of non-nulls, an event of a subscription, and a deferred
payload.

The directive is experimental and is not one of the directives every schema
has. A schema must declare it — `schema.DisableErrorPropagation` is the
definition — and **a schema that has not declared it does not honour it**.
graphql-js reads the name off the operation without asking the schema;
validation refuses such a document either way, so the two differ only for a
document that skipped validation.

## Fragment arguments

Fragment arguments — `fragment F($x: Int) on T` and `...F(x: 1)` — are
experimental, and the parser reads them only when asked
(`language.ExperimentalFragmentArguments`).

They are validated in part. A spread giving an argument the fragment does not
declare is reported, a fragment's own variable is kept apart from an operation
variable of the same name when deciding whether two fields can be merged, and a
fragment's unused variables are reported.

What a spread supplies is checked against what the fragment declares, as
graphql-js does: a required argument left out, an argument of the wrong type,
and a variable that does not fit the declared one are all reported.

They are bound at execution time, so a field inside the fragment sees what the
spread passed in. A fragment's body may name the request's own variables and
the ones the fragment declares, and nothing else: a name the fragment declared
is answered by the spread, by the fragment's own default, or by nothing at all,
never by a request variable that happens to share it — and a scope does not
travel through a fragment that declares none of its own.

A spread that gets its arguments wrong fails the request before anything runs,
so the response has no `data`. graphql-js answers `data: null` for some of
these and nothing for others, depending on which of its two checks caught the
problem; here the answer is the same either way.

## Package layout

`type` and `error` are Go keywords, so those two packages are named `schema`
and `gqlerror`.

graphql-js's modules import each other in cycles, which Go does not allow.
Five had to be given a direction:

| Cycle in graphql-js | Direction here |
|---|---|
| `language` ↔ `error` | `language` is below `gqlerror`; the lexer and parser return `*language.SyntaxError` |
| `utilities` ↔ `type` (for `astFromValue`) | `LiteralFromValue` lives in `schema` |
| `utilities` ↔ `type` (for `coerceInputValue` and `validateInputValue`) | both live in `schema`, along with `SuggestionList`, `DidYouMean`, `NaturalCompare`, `ValueFromASTUntyped` and `ReplaceVariables`: a type is what decides which values it accepts, and validating a schema's default values needs to ask |
| `utilities` ↔ `execution` (for `introspectionFromSchema`) | `IntrospectionFromSchema` lives in `utilities`, as it does upstream; what pointed the other way was `TypeFromAST`, moved as below |
| `utilities` ↔ `validation` (the schema builder checks the document, and the rules need to know where in the schema they are) | `TypeInfo` and `TypeFromAST` live in `internal/typeinfo`, below both, and are re-exported from `utilities` under the names graphql-js gives them |
