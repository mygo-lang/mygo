## Why

parser2 currently constructs `ast2` first and reconstructs lossless metadata
afterward through a separate source scan and span/event heuristics.  This
duplicates lexical knowledge, makes source preservation fragile, and leaves the
formatter to reconcile independently-derived layout data.

The parser needs one authoritative lossless syntax representation that can
recover a complete tree after malformed input while keeping compiler and
formatter entry points strict about syntax errors.

## What Changes

- Add a parser2 lossless-CST capability: one lexer-owned token/trivia stream,
  immutable CST nodes, and navigable Syntax/Token Tree views whose ranges cover
  every input byte exactly once.
- Add CST error recovery: malformed regions become error nodes and diagnostics,
  while parsing continues far enough to retain following declarations and
  tokens in the lossless tree.
- Add lowering from valid Syntax Tree nodes to `ast2`, retaining source spans
  and preserving the existing AST-only parse API contract.
- **BREAKING (internal parser2 source model):** replace post-parse
  `scanLossless`/reconstructed node-span and layout-event metadata as the
  formatter's structural authority with CST/Syntax Tree traversal.
- Move formatter structure and source preservation decisions to the Syntax
  Tree, while retaining the public behavior that invalid input returns an
  error and no formatted source.

## Capabilities

### New Capabilities

- `parser2-lossless-cst`: lossless CST, Syntax/Token Tree navigation, recovery
  diagnostics, and strict AST lowering for parser2.

### Modified Capabilities

- `formatter-cli`: formatter input and layout ownership move from reconstructed
  parser metadata to the lossless Syntax Tree while invalid input remains an
  error.

## Impact

- `internal/mygo/parser2/` lexer, parser, lossless model, tests, and generated
  bootstrap outputs.
- `internal/mygo/ast2/` source association used by CST-to-AST lowering.
- `internal/mygo/formatter/` rendering and formatter tests.
- Bootstrap synchronization and parser2/formatter/cmd regression gates.
