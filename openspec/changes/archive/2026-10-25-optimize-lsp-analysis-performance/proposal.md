## Why

The MyGO language server re-reads and re-fingerprints every watched source file's full text on each analysis check, deduplicates paths with linear scans, re-parses the whole workspace for workspace symbols and references, and re-parses open documents several times per request. Those costs grow with total source bytes and file count, and because message handling is single-threaded they stall hover, completion, and diagnostics on large workspaces. No benchmark currently proves or guards any of this.

## What Changes

- Add a Go benchmark suite under `lsp/` that isolates fingerprinting, workspace source discovery, source-index construction, warm package analysis, and package diagnostics, with allocation reporting.
- Replace content-bearing analysis fingerprints with metadata stamps: overlay document version plus on-disk size and modification time. Unchanged files are validated with `os.Stat` instead of `os.ReadFile`.
- Replace `Slice.Contains`-based path deduplication with set-based membership in workspace discovery, package source listing, dependency recording, and dependency closure expansion.
- Cache the flattened workspace source list and invalidate it through a directory epoch bumped whenever a directory listing is refreshed.
- Cache the workspace-level `SourceIndex` keyed by the workspace fingerprint and Prelude directory, and reuse it for workspace symbols, references, and the completion fallback.
- Cache open-document `SyntaxSnapshot` values by document version and reuse them across diagnostics, completion, document symbols, and hover fallback so one request parses a file at most once.
- Remove the unused `discoverMyGoSources` duplicate of the directory cache.

Explicit non-goals: no LSP protocol, lifecycle, or capability changes; diagnostics are still published on every open/change notification; no concurrency, request cancellation, or incremental compiler work in this change.

## Capabilities

### New Capabilities

None. This is an implementation-only performance change that preserves the existing `lsp-analysis`, `lsp-language-features`, and `lsp-protocol` requirements; the change declares `skip_specs: true`.

### Modified Capabilities

None. No requirement changes: analysis freshness, diagnostics publication on open/change, navigation, completion, and hover behavior remain as specified. Existing freshness scenarios stay covered by the current LSP tests.

## Impact

- Affected source: `lsp/analysis_cache.mygo`, `lsp/source_snapshots.mygo`, `lsp/documents.mygo`, `lsp/workspace.mygo`, `lsp/package_sources.mygo`, `lsp/dependency_graph.mygo`, `lsp/directory_cache.mygo`, `lsp/semantic_analysis.mygo`, `lsp/semantic_diagnostics.mygo`, `lsp/diagnostics.mygo`, `lsp/symbol_index.mygo`, `lsp/workspace_symbols.mygo`, `lsp/symbol_resolution.mygo`, `lsp/completion_candidates.mygo`, `lsp/completion.mygo`, `lsp/hover.mygo`, `lsp/document_symbols.mygo`, plus their generated `zz_*.gen.go` companions.
- New benchmarks: `lsp/perf_benchmark_test.go` (handwritten Go in package `lsp`).
- Public LSP behavior and wire protocol are unchanged; no new dependencies.
- Regeneration and build: every edited `.mygo` file requires `./mygo sync lsp`, and `go build -o mygo-lsp ./cmd/mygo-lsp` must still succeed.
- Risk: on-disk changes that preserve both size and modification time are no longer detected without a `workspace/didChangeWatchedFiles` notification; this matches the metadata-based freshness model and is recorded in the design.
