# Test fixtures

`kitchen-sink.graphql` and `github-schema.graphql` are copied unmodified from
the graphql-js repository (`benchmark/`), and `kitchen-sink-query.graphql` and
`kitchen-sink-sdl.graphql` from its `src/__testUtils__/`. All four are MIT
licensed, Copyright (c) GraphQL Contributors. See the repository NOTICE file.

They exercise the lexer, parser, printer and visitor against real-world
documents: the kitchen sinks cover every syntactic construct, and the GitHub
schema is a large SDL document.

`kitchen-sink-sdl-printed.graphql` is what graphql-js's own printer test says
`kitchen-sink-sdl.graphql` prints as. Comparing against it is what says the two
printers agree, byte for byte, on a document using every type-system
construct.
