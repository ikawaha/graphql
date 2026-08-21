# Differential tests against graphql-js

These run one corpus through both implementations and compare what each
answered. Reading graphql-js and reasoning about what it would do has been
wrong often enough; this runs it.

`go test .` needs nothing but Go. The answers graphql-js gave are
recorded under `expected/`, and the Go side compares against the recording.

## Regenerating the recording

Needed when the pinned graphql-js moves, or when a case is added to `corpus/`.

graphql-js is pinned at commit `9c24501` (v17.0.2). It asks for Node 22 or
later, but only to strip its own types, so esbuild is enough to run it on an
older Node:

```sh
npm install --no-save esbuild
npx esbuild <graphql-js checkout>/src/index.ts \
    --bundle --format=esm --platform=node --outfile=testdata/differential/pinned.mjs
node testdata/differential/runner.mjs
```

The bundle is generated and is not committed. `GRAPHQL_BUNDLE` points the
runner at one elsewhere.

The npm package `graphql@17.0.0-alpha.10` also installs and runs, but it is
older than the pinned commit — it has no `legacyExecuteIncrementally` — so it
is not the reference.

## What is compared

| | input | compared |
|---|---|---|
| `documents` | a document | `print(parse(x))`, and printing what was printed |
| `schemas` | SDL | `printSchema`, and what `validateSchema` says |
| `validations` | a document, against graphql-js's own harness schema | every complaint, message and place |
| `executions` | SDL, a document, variables, a root value of plain data | the whole response |
| `coercions` | generated: every type × every shape of value, through a variable and through a literal | the whole response, with each field answering what its arguments came to |
| `introspections` | SDL, and one introspection query frozen in the corpus | the whole answer, key order included, and the schema rebuilt from it |
| `incremental` | a document using `@defer` or `@stream`, against a schema of its own | every payload, in order |
| `legacy-incremental` | the same documents | every payload in the format that came before |
| `subscriptions` | a document, a schema of its own, and a list of events | one response per event, in order |

`validations` is the layer the ported suites leave least covered: they compare
how many errors there are and where each points, and what each says is checked
only here.

`executions` resolves every field from plain data in the root value, so each
implementation uses its own default resolver and neither is handed one the
other could not have. Nothing is asynchronous; what is compared is the input
and the output.

`introspections` puts *one* query to both: asking each its own would compare
two different questions. Key order is compared too — an answer that agrees
field for field but writes an empty list where the other writes nothing is not
the same answer, and a client rebuilds a different schema from it.

`coercions` is written by `generate.mjs` rather than by hand: input coercion is
where a port drifts, and enumerating every scalar, wrapper and value by hand
would miss cases. Both implementations install their own resolver that answers
with what the arguments came to, so a coercion that succeeds is compared and
not only one that fails. Run `node generate.mjs` after changing it, then
regenerate the recording.

`incremental` was taken from graphql-js's defer and stream tests by reading the
input out of each one — the document and the root value — and throwing the
timing away: a promise that is already settled is the value it holds, a thunk
answering with a constant is that constant, and a value the test names but
writes as one of graphql-js's own type objects is written out as plain data.
The test's own expectation is not used, since what graphql-js answers is
recorded by running it.

That reading was a one-off, as it was for the other corpora taken the same way,
and what it produced is the corpus itself. Doing it again against a later
graphql-js means reading whatever its tests look like then.

A field that fails is written in the corpus as `{"__throw": "message"}`, since
a corpus holds data and graphql-js's tests write a resolver that throws. Each
runner installs a resolver that raises it its own way, over its own default
resolver so everything else is unchanged.

Five cases were dropped after being extracted. Their names say what they are
about — a slower sibling, a fragment that resolves later — and with the timing
removed the two implementations batch the payloads differently, which is not a
difference in what either does but in what was asked of them.

The whole run of payloads is compared, since how many there are and in what
order is the shape a client is waiting for. Both sides skip document
validation, because graphql-js's own runner calls the executor straight; what a
document is wrong about is compared by `validations`.

`subscriptions` carries each event as the root field's value, which is what
this executor hands a subscriber. graphql-js makes the event the root value and
resolves the root field from it, so the runner wraps each one before yielding.
That difference is deliberate and recorded in COMPATIBILITY.md; what is
compared is everything after it. Neither side writes a subscriber: the channel
and the async iterable are both put in the root value, where both look.

`legacy-incremental` reuses the same corpus: graphql-js still answers in the
earlier draft's format and so does this, and a client written against it sees
these payloads.

`documents` holds every SDL in the schema corpus as well as its own queries,
since a schema is a document too, and the three fixtures under
`language/testdata` — the largest of which is a real 368KB schema.

`schemas` is mostly graphql-js's own test SDL, kept only where it stands alone
as a schema: the extendSchema tests are full of fragments like
`extend type Query { … }` that mean nothing on their own. `introspections`
asks the same corpus a second question, so growing one grows both.

Most of `validations` comes from graphql-js's own test files, extracted by
taking the document out of every `expectErrors(` and `expectValid(` call. What
each upstream test expected is not used: both implementations are run over the
document with the full rule set and the answers compared.

## Adding a case

Add it to the corpus file, regenerate the recording, and read the diff before
committing it: the recording is graphql-js's answer, so a surprise in it is
worth understanding rather than accepting.

## A difference on purpose

`known` in the repository root's `differential_test.go` holds the cases this
implementation answers differently, each with its reason. A case listed there is asserted to *still*
differ, so closing one cannot go unnoticed.

Today it holds one: a single payload of one incremental case whose pieces come
out in a different order. The same fragments, ids, subPaths and data are
delivered in the same payload; graphql-js arrives at its order through the
order its own queue released them, and sorting to match it moved three other
cases the wrong way.
