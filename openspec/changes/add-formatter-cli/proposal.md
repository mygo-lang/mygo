## Why

MyGO currently lacks a unified source-formatting capability, so developers cannot consistently normalize `.mygo` files through the command line or CI. Establishing an independent formatter and CLI now will validate formatting rules and provide a reusable foundation for future LSP formatting support.

## What Changes

- Add an independent MyGO source-formatting core, authored in `.mygo` and compiled to Go, that combines parser2/ast2 structure with positioned token/trivia information to produce stable, idempotent formatted text.
- Add the `mygo fmt` CLI subcommand for formatting files and standard input/output.
- Add a check mode so CI can detect unformatted files without modifying them.
- Preserve the exact source content of comments, strings, inline Go, delimiters, and other trivia while formatting structure; report errors and avoid writing incomplete output when parsing fails.
- Do not add LSP protocol handling in this change; integrate LSP later as another caller of the formatter core.

## Capabilities

### New Capabilities

- `formatter-cli`: Provides the MyGO source-formatting core and command-line interface.

### Modified Capabilities

<!-- No existing capability requirements are changed. -->

## Impact

- Add MyGO formatter source and generated formatter code/tests using parser2/ast2 plus positioned token/trivia data. AST supplies abstract structure and layout decisions; token/trivia supplies exact source preservation. Keep the Go formatter package limited to the stable bridge API.
- Extend the `cmd/mygo` command-line interface with the `fmt` subcommand, its arguments, exit codes, and file-write behavior.
- Do not change compiler-generated code, the LSP protocol, or existing `sync`/`build` behavior.
