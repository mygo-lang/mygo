## Why

The production and self-hosted bootstrap compiler pipelines accept different
MyGO programs and expose inconsistent workflow behavior. This weakens
bootstrap as a verification path and leaves parts of the language contract
ambiguous, notably string literal semantics and loop control.

## What Changes

- Make the language specification the shared behavioral contract for both
  compiler pipelines.
- Add `break` and `continue` as specified MyGO loop-control statements and
  implement them in the production compiler as well as bootstrap.
- Preserve multiline raw-string behavior, which is already implemented and
  tested by both parsers, and remove the contradictory same-line restriction
  from the language documentation.
- Preserve the established verbatim content behavior of triple-quoted strings,
  including in inline Go `code` fields, and document it consistently.
- Bring bootstrap parity for no-prelude compilation, nested/discarding tuple
  destructuring, statement-context switch patterns, external test packages,
  and the self-contained `GenerateSource` APIs.
- Add differential conformance tests and require separate bootstrap and
  production commits so either lane can be cherry-picked independently.

## Capabilities

### New Capabilities

- `language-semantics-parity`: Defines the source-language behavior shared by
  production and bootstrap, including strings, tuple bindings, switch
  patterns, and loop control.
- `bootstrap-workflow-parity`: Defines bootstrap CLI and generation workflow
  behavior for prelude selection, test packages, and convenience APIs.

### Modified Capabilities

- None.

## Impact

- Documentation: `docs/spec.md`, `docs/compiler/semantics.md`, and
  `docs/compiler2/differences.md`.
- Production: parser grammar/lexer, AST validation and Go code generation.
- Bootstrap: `parser2`, `ast2`, `typeinference2`, `codegen2`, and bootstrap
  orchestration in `internal/mygo/compiler`.
- Tests: parser, inference, codegen, compiler end-to-end tests, plus shared
  differential fixtures.
