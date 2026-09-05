## Context

The repository already has parser2/ast2 support, a `cmd/mygo` CLI, and an independent `lsp` service. Formatting should reuse parser2 syntax-layer capabilities without depending on type inference, code generation, or LSP document state. Formatting rules should be centralized in a source-preserving token/layout printer.

## Goals / Non-Goals

**Goals:**

- Establish a pure formatter core reusable by the CLI and future LSP integration.
- Provide stable, idempotent output for valid source while preserving comments and special text content as far as possible.
- Support file writeback, stdin/stdout, and CI-friendly exit statuses in the CLI.

**Non-Goals:**

- This change does not implement LSP `textDocument/formatting` or range formatting.
- It does not perform type checking, import organization, code generation, or semantic rewriting.
- It does not promise error-recovery formatting for unparseable source.

## Decisions

### 1. Separate formatter core and entry points

Author the formatter core in MyGO source under the formatter implementation area and compile it to Go as part of the repository workflow. Expose its stable source-to-formatted-source-or-error API through a thin Go bridge package. `mygo fmt` handles argument parsing, file I/O, diagnostics, and exit codes. Future LSP support can reuse the bridge without emulating the CLI.

The alternatives are placing the logic directly in `cmd/mygo` or `lsp`; the former limits reuse, while the latter makes the protocol layer an unnecessary core dependency. Keeping the core in MyGO also exercises the language's own parser/AST-facing implementation boundary.

### 6. Keep formatter behavior authored in MyGO

Formatter traversal, rendering rules, and formatter-specific tests should be written in MyGO where the project toolchain supports them. Generated Go is a build artifact and must not become the hand-maintained source of formatter behavior. Small Go code is permitted only for the public bridge, CLI integration, filesystem operations, and generated-code plumbing.

### 2. Use protected-token layout instead of AST reconstruction

The formatting pipeline combines parser2/ast2 structure with positioned tokens and trivia. AST supplies abstract structure and layout decisions; positioned tokens/trivia preserve comments, strings, inline Go, delimiters, whitespace, and source locations. Protected spans are emitted without interpreting their internal text as MyGO formatting syntax. AST reconstruction alone is never the sole source for whole-file output.

Token-level layout preserves unknown syntax and source trivia more reliably than rebuilding from the compact ast2 representation. Nested structure handling combines AST boundaries, syntax delimiters, and parser2 validation.

### 3. Use safe file writeback

The default mode parses and formats completely before writing. Any file failure results in a failing command. `--check` only compares the original and formatted content and never writes. Multi-file processing retains each file's path in diagnostics.

### 4. Start with whole-file formatting

The first version defines only whole-file formatting. Range formatting must handle parent nodes outside the selection, indentation baselines, and comments crossing boundaries, so it is deferred to a later design.

### 5. Choose conditional syntax from rendered shape

The printer will decide whether an `if` or `case` is single-line or multiline based on the rendered body shape. Single-line `if` uses `if cond => xxx else yyy`; multiline `if` uses parser2-compatible `if cond then` block syntax with `else` and `end` on their own lines. Single-line cases use `case XXX => xxx`; multiline cases use `case XXX then`, one body statement per line, and `end` on its own line.

The parser's ability to accept `case XXX => xxx` in a multiline form does not affect printer output. The formatter intentionally canonicalizes that form to `then` block syntax for readability and consistency.

### 7. Prefer a bounded line width

The initial preferred line width is 100 columns. The printer renders a candidate compact form first; if it exceeds the width, it selects the corresponding multiline form. Protected strings and inline Go bodies are never split internally.

## Risks / Trade-offs

- [Comment positions may not be fully recoverable from the existing AST] -> Inspect parser/AST comment retention; use positioned tokens if needed and lock behavior with golden tests.
- [Parser and formatter may use different AST versions] -> Explicitly choose the current primary parser/AST and isolate parser adaptation at the formatter boundary.
- [Formatting rules may change frequently] -> Establish a minimal rule set with golden fixtures and idempotence tests; record later rule changes separately.
- [A body may be difficult to classify as single-line before rendering] -> Render into a temporary representation first, then choose compact or block syntax using explicit line-count and statement-count rules.
- [Atomic writeback may leave temporary files after failure] -> Use same-directory temporary files, clean them up after successful replacement, and preserve the original on write failure.

## Migration Plan

No migration is required. Users can opt into `mygo fmt`; existing `sync`, `build`, and LSP behavior remains unchanged. If formatting rules need to be rolled back, users can disable the CLI or revert the formatter version; source files are not formatted automatically.

## Open Questions

- The initial indentation width and line-breaking rules can be finalized during implementation using existing MyGO examples and golden fixtures without changing the architecture or CLI contract.
