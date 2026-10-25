## Why

The LSP re-analyzes far more source than an edit actually changes. On a 98.5 KB workspace one `didChange` spends ~195 ms, ~77% of it in parser2, while only a ~0.7 KB file changed (99%+ redundant parsing). The cost scales with unchanged dependencies: adding a never-edited 73.7 KB package raises one edit to ~423 ms. Worse, the package fingerprint is order-sensitive (overlay paths come from map iteration), so it never matches: with five open documents in one package, every `publishDiagnostics` triggers a full `AnalyzeEditorPackage`, turning one edit into ~971 ms (A) / ~2.2 s (B) — a 5× amplification per edit.

This is the dominant responsiveness problem for the editor and is independent of parser2 throughput, so it is fixed separately from the parser2 performance work.

## What Changes

- Make package analysis identity deterministic: compute fingerprints from a canonical, order-independent set of source paths and stamps, so an unchanged package hashes identically across calls.
- Reuse analysis across requests: a package with an unchanged fingerprint SHALL return its cached snapshot instead of re-running `AnalyzeEditorPackage`, regardless of how many open documents are published.
- Add a per-file parse cache shared across analyses: parsed `BootstrapInputs` for an overlay or disk file are reused while that file's stamp (overlay version/size, or disk size/mtime) is unchanged. This removes redundant parsing of the Prelude, imported workspace packages, and unchanged sibling files on every edit.
- Scope invalidation to the changed file(s) and the existing dependency-graph invalidation, so edits do not discard unrelated parse results.
- Avoid parsing the edited document twice: package refresh SHALL consume the already available syntax/AST result for the current overlay version.
- Add measurable guards: cache-hit counters plus benchmarks and a regression test asserting that one edit to one open document performs one package analysis and one reparse of the changed file, independent of open-document count.
- No change to diagnostic wording, positions, ordering, or protocol behavior. No parser2-internal changes in this change.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `lsp-analysis`: analysis reuse becomes an explicit contract — deterministic package identity, one analysis per unchanged package per edit, per-file parse reuse across analyses, and invalidation limited to changed files and affected dependents.

## Impact

- LSP: `lsp/analysis_cache.mygo`, `lsp/package_sources.mygo`, `lsp/file_stamps.mygo`, `lsp/diagnostics.mygo`, `lsp/file_events.mygo`, `lsp/documents.mygo`, `lsp/dependency_graph.mygo`.
- Compiler entry points used by the editor: `internal/mygo/compiler/editor_analysis.mygo`, `internal/mygo/compiler/editor_loader.mygo` (accept a persistent `BootstrapState`/parse cache instead of creating one per call).
- Tests/benchmarks: `lsp/*_test.mygo`, `lsp/perf_benchmark_test.go` and a new multi-document/multi-package fixture that asserts cache hits and edit-time bounds.
- Out of scope: parser2 parser/lowering performance (separate change) and type inference caching across analyses (the remaining non-parse floor, currently reset by design).
