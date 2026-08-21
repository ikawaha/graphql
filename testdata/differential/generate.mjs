// Writes the generated part of the execution corpus.
//
// Input coercion is where a port drifts: every scalar, every wrapper, every
// shape of value, reached once through a variable and once through a literal
// written in the document. Enumerating that by hand would miss cases and
// reading it would be worse; it is generated here and the result committed, so
// that the corpus is the same text for both implementations.
//
// Nothing here imports graphql: a corpus built with one implementation's help
// would be shaped by it.

import { writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));

const SDL = `
type Query { f(a: T): String }
enum E { A B }
input In { x: Int  y: String = "d"  z: [Int!] }
scalar Custom
`;

// The types an argument is given, each standing for a different rule about
// what may be written.
const types = [
  'Int', 'Int!', 'Float', 'String', 'Boolean', 'ID',
  '[Int]', '[Int!]', '[Int]!', '[Int!]!', '[[Int]]',
  'E', 'E!', 'In', 'In!', 'Custom',
];

// What a request supplies for it. Written as JSON, so these are the values a
// request body can carry.
const values = [
  null, 0, 1, -1, 1.5, 9007199254740993, '', 'A', '1', true, false,
  [], [1], [1, null], [[1]], {}, { x: 1 }, { x: null }, { x: 1, nope: 2 },
];

// What is written in the document instead of a variable.
const literals = [
  'null', '0', '1', '-1', '1.5', '1e3', '""', '"A"', '"1"', 'true', 'false',
  'A', 'B', 'Z', '[]', '[1]', '[1, null]', '[[1]]', '{}', '{x: 1}',
  '{x: null}', '{x: 1, nope: 2}', '{y: "s"}', '{z: [1]}',
];

const cases = [];

for (const type of types) {
  // Through a variable, supplied.
  for (const [i, value] of values.entries()) {
    cases.push({
      name: `variable ${type} given ${JSON.stringify(value)}#${i}`,
      sdl: SDL.replace('T', type),
      query: `query ($v: ${type}) { f(a: $v) }`,
      variables: { v: value },
      echoArgs: true,
    });
  }
  // Through a variable the request left out, which is not the same as null.
  cases.push({
    name: `variable ${type} not supplied`,
    sdl: SDL.replace('T', type),
    query: `query ($v: ${type}) { f(a: $v) }`,
    echoArgs: true,
  });
  // Written out in the document.
  for (const [i, literal] of literals.entries()) {
    cases.push({
      name: `literal ${type} = ${literal}#${i}`,
      sdl: SDL.replace('T', type),
      query: `{ f(a: ${literal}) }`,
      echoArgs: true,
    });
  }
  // Left out of the document altogether.
  cases.push({
    name: `literal ${type} left out`,
    sdl: SDL.replace('T', type),
    query: '{ f }',
    echoArgs: true,
  });
}

// A default on the argument, reached the three ways an argument can be.
for (const def of ['1', 'null', '[1]', '{x: 1}']) {
  for (const [name, query, variables] of [
    ['left out', '{ f }', undefined],
    ['written as null', '{ f(a: null) }', undefined],
    ['a variable not supplied', 'query ($v: Int) { f(a: $v) }', undefined],
    ['a variable given null', 'query ($v: Int) { f(a: $v) }', { v: null }],
  ]) {
    cases.push({
      name: `default ${def}, ${name}`,
      sdl: `type Query { f(a: Int = ${def}): String }`,
      query,
      variables,
      echoArgs: true,
    });
  }
}

writeFileSync(join(here, 'corpus', 'coercions.json'),
  JSON.stringify(cases, null, 1) + '\n');
console.log('wrote', cases.length, 'cases');
