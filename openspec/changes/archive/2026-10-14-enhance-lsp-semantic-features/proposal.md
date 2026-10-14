## Why

Currently, MyGO LSP suffers from protocol gaps in message transmission, document synchronization, and JSON boundaries. Semantic functionality relies primarily on string search, which cannot reliably support daily editing. The first phase simultaneously establishes a reliable protocol foundation, type completion, and cross-file navigation, enabling existing compiler syntax and type information to serve the editor.

## What Changes

- Fixed standard input/output message framing, JSON-RPC encoding/decoding, request/notification differentiation, initialization, and exit lifecycle issues.

- Established versioned document caching, UTF-16 location conversion, and standard file URI conversion; released and cleaned up syntax and type diagnostics.

- Reused parser2 and typeinference2, providing scope-aware completion, type and member completion, type hover, and hierarchical document symbols.

- Included cross-file definition jumps, reference lookups, and workspace symbols for MyGO packages imported within the same package and workspace, including unopened files.

- Supported syntax-level degradation for unsaved content overwriting disks, dependency failures, and erroneous input.

- Split source files by protocol, documentation, workspace, analysis, and feature processor. Handwritten files are targeted to be no more than 400 lines; those exceeding 500 lines must be further split. Generated files are not subject to this limit.

## Capabilities

### New Capabilities

- `lsp-protocol`: Message passing, JSON-RPC, lifecycle, document synchronization, and location transformation.

- `lsp-analysis`: Workspace and package analysis, caching, diagnostics, and incomplete input degradation.

- `lsp-language-features`: Type completion, hover, documentation symbols, cross-file definitions and references, and workspace symbols.

### Modified Capabilities

None. Existing compiler and language specification behavioral contracts remain in effect.

## Impact

- Primarily affects `lsp/`, `cmd/mygo-lsp/`, and the LSP descriptions of related generated Go files and READMEs.

- Reuse compiler package parsing, import parsing, Prelude, and Go FFI type information; add read-only analysis adapter interfaces as needed, without changing language semantics.

- Maintain the MyGO-based service core; Go only handles boundary adaptations such as streams, JSON, URIs, location conversions, and file systems.

- Use a standard LSP client for acceptance testing; this round will not modify the independent VSCode extension repository, and will not add rename, formatting, code manipulation, or automatic import functionality.