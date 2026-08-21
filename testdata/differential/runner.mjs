// Runs the corpus through graphql-js and records what it answered.
//
// The recording is what the Go side compares against, so that running the
// tests needs no Node. Regenerate it when the pinned commit moves; see
// README.md for how.

import { readFileSync, writeFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const graphql = await import(process.env.GRAPHQL_BUNDLE ?? join(here, 'pinned.mjs'));

const corpus = (name) =>
  JSON.parse(readFileSync(join(here, 'corpus', name + '.json'), 'utf8'));

// record writes the answers out with their keys in a fixed order, so that a
// regeneration that changed nothing produces no diff.
const record = (name, answers) =>
  writeFileSync(join(here, 'expected', name + '.json'),
    JSON.stringify(answers, null, 2) + '\n');

// caught turns whatever a call threw into the message alone: the two
// implementations agree about what is wrong, not about how a stack reads.
function caught(run) {
  try {
    return { ok: run() };
  } catch (error) {
    return { error: error.message };
  }
}

// ---- L1: a document read and written back out.
record('documents', corpus('documents').map(({ name, source }) => {
  const printed = caught(() => graphql.print(graphql.parse(source)));
  if (printed.error) {
    return { name, error: printed.error };
  }
  // Printing what was printed must give the same text: a printer that is not
  // idempotent loses something on the first pass.
  const again = caught(() => graphql.print(graphql.parse(printed.ok)));
  return { name, printed: printed.ok, reprinted: again.ok ?? null, error: again.error ?? null };
}));

// ---- L2: a schema built from SDL and written back out.
record('schemas', corpus('schemas').map(({ name, sdl }) => {
  const built = caught(() => graphql.buildSchema(sdl));
  if (built.error) {
    return { name, error: built.error };
  }
  const printed = caught(() => graphql.printSchema(built.ok));
  const problems = caught(() => graphql.validateSchema(built.ok).map((e) => e.message));
  return {
    name,
    printed: printed.ok ?? null,
    problems: problems.ok ?? null,
    error: printed.error ?? problems.error ?? null,
  };
}));

// ---- L3: a document checked against a schema.
//
// The message is what matters here: the ported validation suites compare only
// how many errors there are and where they point, so wording drifts unnoticed.
const harness = graphql.buildSchema(
  readFileSync(join(here, 'corpus', 'harness.graphql'), 'utf8'), { assumeValidSDL: true });

record('validations', corpus('validations').map(({ name, query }) => {
  const parsed = caught(() => graphql.parse(query, { experimentalFragmentArguments: true }));
  if (parsed.error) {
    return { name, parseError: parsed.error, problems: null };
  }
  const problems = caught(() => graphql.validate(harness, parsed.ok).map((e) => ({
    message: e.message,
    locations: (e.locations ?? []).map(({ line, column }) => ({ line, column })),
  })));
  return { name, parseError: null, problems: problems.ok ?? null, error: problems.error ?? null };
}));

// ---- L4: a document run against a schema.
//
// Every field resolves from plain data in the root value, so both
// implementations use their own default resolver and neither is given a
// resolver the other cannot have. What is compared is the response.
// echoArgs answers every field with what its arguments came to, so that the
// result of coercing an input is visible and not merely its failure. The keys
// are sorted, since one implementation's map has an order and the other's does
// not.
const canonical = (v) => {
  if (Array.isArray(v)) return v.map(canonical);
  if (v === null || typeof v !== 'object') return v;
  const out = {};
  for (const k of Object.keys(v).sort()) out[k] = canonical(v[k]);
  return out;
};
const echoResolver = (_source, args) => JSON.stringify(canonical(args));

const runExecutions = (name) => record(name, corpus(name).map((c) => {
  const answer = caught(() => {
    const schema = graphql.buildSchema(c.sdl, { assumeValidSDL: true });
    const result = graphql.graphqlSync({
      schema,
      source: c.query,
      rootValue: c.root ?? {},
      variableValues: c.variables,
      operationName: c.operationName,
      fieldResolver: c.echoArgs ? echoResolver : undefined,
    });
    return {
      data: result.data === undefined ? undefined : result.data,
      errors: (result.errors ?? []).map((e) => ({
        message: e.message,
        path: e.path ?? null,
        locations: (e.locations ?? []).map(({ line, column }) => ({ line, column })),
      })),
    };
  });
  return { name: c.name, ...(answer.ok ?? { error: answer.error }) };
}));

runExecutions('executions');
runExecutions('coercions');

// ---- L5: what a schema says about itself, and what that rebuilds into.
//
// One introspection query, frozen in the corpus, is put to both
// implementations: asking each its own question would compare two different
// answers. The response is compared whole — key order included, since that is
// the order the document asked for — and then the schema is rebuilt from it
// and printed, which is the round trip a client makes.
const introspectionQuery = readFileSync(
  join(here, 'corpus', 'introspection-query.graphql'), 'utf8');

record('introspections', corpus('schemas').map(({ name, sdl }) => {
  const answer = caught(() => {
    const schema = graphql.buildSchema(sdl);
    const asked = graphql.graphqlSync({ schema, source: introspectionQuery });
    if (asked.errors?.length) {
      return { answered: null, rebuilt: null, error: asked.errors[0].message };
    }
    const rebuilt = graphql.printSchema(graphql.buildClientSchema(asked.data));
    return { answered: asked.data, rebuilt, error: null };
  });
  return { name, ...(answer.ok ?? { answered: null, rebuilt: null, error: answer.error }) };
}));

// ---- L6: a document that asks for parts of the response to arrive later.
//
// The payloads are collected into one list and compared as a whole: what
// matters is not only what each says but how many there are and in what order.
// Only documents whose data is already there are taken, so neither
// implementation is waiting on anything and the sequence is settled.
const incrementalSchema = graphql.buildSchema(
  readFileSync(join(here, 'corpus', 'incremental.graphql'), 'utf8'));

// A field whose value is the sentinel fails. graphql-js's tests write a
// resolver that throws; a corpus holds data, so the corpus says which field
// fails and each implementation raises it its own way.
const failing = (source, args, context, info) => {
  const held = graphql.defaultFieldResolver(source, args, context, info);
  if (held !== null && typeof held === 'object' && typeof held.__throw === 'string') {
    throw new Error(held.__throw);
  }
  return held;
};

const payloads = async (c) => {
  const result = await graphql.experimentalExecuteIncrementally({
    schema: incrementalSchema,
    document: graphql.parse(c.query),
    rootValue: c.root ?? {},
    fieldResolver: failing,
  });
  if (!('initialResult' in result)) {
    return [result];
  }
  const all = [result.initialResult];
  for await (const part of result.subsequentResults) {
    all.push(part);
  }
  return all;
};

const incremental = [];
for (const c of corpus('incremental')) {
  try {
    incremental.push({ name: c.name, payloads: await payloads(c) });
  } catch (error) {
    incremental.push({ name: c.name, payloads: null, error: error.message });
  }
}
record('incremental', incremental);

// ---- The same documents in the payload format that came before.
const legacyPayloads = async (c) => {
  const result = await graphql.legacyExecuteIncrementally({
    schema: incrementalSchema,
    document: graphql.parse(c.query),
    rootValue: c.root ?? {},
    fieldResolver: failing,
  });
  if (!('initialResult' in result)) {
    return [result];
  }
  const all = [result.initialResult];
  for await (const part of result.subsequentResults) {
    all.push(part);
  }
  return all;
};

const legacy = [];
for (const c of corpus('incremental')) {
  try {
    legacy.push({ name: c.name, payloads: await legacyPayloads(c) });
  } catch (error) {
    legacy.push({ name: c.name, payloads: null, error: error.message });
  }
}
record('legacy-incremental', legacy);

// ---- Subscriptions: one response per event.
//
// The corpus carries the events as the root field's value, which is what this
// port's executor hands a subscriber. graphql-js instead makes each event the
// root value and resolves the root field from it, so the runner wraps each one
// — that difference is deliberate and recorded in COMPATIBILITY.md; what is
// compared here is everything else.
const subscriptionSchema = graphql.buildSchema(
  readFileSync(join(here, 'corpus', 'subscription.graphql'), 'utf8'));

const subscriptions = [];
for (const c of corpus('subscriptions')) {
  const source = (async function* () {
    for (const event of c.events) {
      yield { [c.field]: event };
    }
  })();
  try {
    const started = await graphql.subscribe({
      schema: subscriptionSchema,
      document: graphql.parse(c.query),
      rootValue: { [c.field]: source },
    });
    if (started.errors) {
      subscriptions.push({ name: c.name, payloads: null, error: started.errors[0].message });
      continue;
    }
    const all = [];
    for await (const payload of started) {
      all.push(payload);
    }
    subscriptions.push({ name: c.name, payloads: all });
  } catch (error) {
    subscriptions.push({ name: c.name, payloads: null, error: error.message });
  }
}
record('subscriptions', subscriptions);

console.log('recorded', readdirSync(join(here, 'expected')).join(', '));
