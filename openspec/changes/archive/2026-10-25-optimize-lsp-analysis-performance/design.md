## Context

See `proposal.md - Why`. The LSP is written in MyGO under [`lsp/`](../../../lsp) and compiled into the Go package by `./mygo sync lsp`; the generated `zz_*.gen.go` files are checked in. `DocumentStore` already owns overlays, per-directory listings, package snapshots, and the dependency graph, and `PackageSnapshot.Index` already memoizes the semantic index for a package. The current bottlenecks all come from *how* that state is validated, not from missing caches:

- [`analysisFingerprint()`](../../../lsp/analysis_cache.mygo:15) concatenates every watched file's full content into one string on every check, and [`frozenAnalysisFingerprint()`](../../../lsp/source_snapshots.mygo:3) repeats the same work after analysis.
- `storeGet()` falls back to `os.ReadFile` for every unopened file, so the content-based fingerprint also re-reads the workspace from disk.
- Workspace and package path lists are deduplicated with `Slice.Contains`, and directory listings are re-flattened per call.
- Workspace symbols and references rebuild a full `SourceIndex` from scratch per request.
- Diagnostics parse the document separately from the syntax snapshot used by completion and symbols.

Message handling is synchronous in [`Main()`](../../../lsp/server.mygo:80), so reducing per-request work is the only lever this change uses.

## Goals / Non-Goals

**Goals:**

- Make warm analysis validation cost proportional to the number of watched files rather than to total source bytes.
- Make path deduplication linear in the number of paths.
- Reuse workspace-level indexing and open-document parsing across requests.
- Prove the improvement with a repeatable benchmark and allocation measurements, without changing LSP behavior.

**Non-Goals:**

- Concurrency, background analysis, request cancellation, or an ordered request queue.
- Incremental compiler/type-inference work; `AnalyzeEditorPackage` still analyzes a whole package on invalidation.
- Diagnostics debouncing or suppression; every open/change notification still publishes diagnostics.
- Protocol, capability, or feature additions.
- Caching on-disk file contents for navigation; `storeGet` keeps its current read-on-demand behavior.

## Decisions

### 1. Metadata stamps replace content-bearing fingerprints

Add a `FileStamp` value. Stamps are computed on demand from the overlay map or `os.Stat`; caching them would only risk serving stale metadata, and each path is visited once per validation pass.

```
struct FileStamp
  Path: String
  Present: Bool
  Size: Int64
  Modified: Int64
  Version: Int
end
```

`sourceStamp(store, path)` resolves one path:

- Open overlay: `Present=true`, `Version=Document.Version`, `Size=Content.Len()`, `Modified=0`.
- Otherwise: `os.Stat(path)`; `Err` yields `Present=false`, `Ok` yields `Size=info.Size()` and `Modified=info.ModTime().UnixNano()`.

`fingerprintOf(stamps)` renders one line per file (`path|present|size|modified|version`) and joins with `strings.Join`. `analysisFingerprint` becomes a thin path-to-stamp-to-string pipeline and no longer calls `storeGet`. `frozenAnalysisFingerprint` is removed; `refreshPackage` expands `watched` from the compiler's source inputs and then computes the same stamp fingerprint over `watched`, so the snapshot fingerprint always describes the file set that was analyzed.

*Alternative considered:* hash file contents, caching a digest per `(size, mtime)`. It preserves exact detection but requires reading every changed file just to hash it, and it still cannot detect a same-size same-mtime edit without re-reading. Metadata stamps capture the same practical freshness as the Go toolchain and keep the hot path allocation-light. `workspace/didChangeWatchedFiles` remains the authoritative invalidation for editors that send it.

### 2. Store the pre-analysis fingerprint, then revalidate on the next request

`analyzeFilePackage` stamps `watched` before calling `AnalyzeEditorPackage`. If a file changes while analysis runs, the next request computes a different stamp and re-analyzes; this is the same optimistic model the current content fingerprint uses. Storing the pre-analysis stamp avoids re-reading every file after analysis just to prove what was read.

### 3. Set-based deduplication

Use the Prelude `Set[String]` (already mapped to a Go map and used elsewhere in the compiler) for membership while collecting paths. Replace the `Contains`-then-`Append` pattern in `workspacePaths`, `packageSourcePaths`, `recordPackageDependencies`, and the dependency-closure expansion in `analyzeFilePackage`. `discoverMyGoSources` is deleted because `directorySources`/`cachedWorkspaceSources` already own discovery and the old function has no callers.

### 4. Directory epoch and flattened listing cache

`DocumentStore` gains `DirectoryEpoch: Int` and `WorkspaceListings: Map[String, CachedListing]` where `CachedListing{Epoch: Int, Paths: Slice[String]}`. `refreshDirectorySources` increments `DirectoryEpoch` whenever it replaces a listing; `directorySources` also increments on deletion. `cachedWorkspaceSources` still recurses to `os.Stat` each directory (so unnotified nested changes are noticed), but when the epoch is unchanged it returns the cached flattened slice for that directory instead of rebuilding it. Flattening uses `Slice.Concat` rather than per-element `Append`.

*Alternative considered:* invalidate only on `didChangeWatchedFiles`. That would skip the stat walk entirely but would stop detecting authoring changes made outside the editor without a notification, which the current tests require.

### 5. Workspace-level `SourceIndex` cache

Add `DocumentStore.WorkspaceIndexes: Map[String, WorkspaceIndexCache]` keyed by Prelude directory, with `WorkspaceIndexCache{Fingerprint: String, Index: SourceIndex}`. A helper `workspaceIndex(store, preludeDir)` computes `analysisFingerprint(store, workspacePaths(store))` and returns the cached index when the fingerprint matches; otherwise it calls `buildSourceIndex` and replaces that Prelude directory's entry. Keying by Prelude directory lets workspace symbols and references alternate without evicting each other.

Callers:

- `handleWorkspaceSymbols` with `PreludeDir=""`.
- `referenceSourceIndex` with the requesting package's resolved Prelude directory.
- `completionSourceIndex` fallback with the same Prelude resolution it already performs.

The returned `SourceIndex` is treated as read-only; handlers must not mutate `Symbols`, `Documents`, `Imports`, or `Types`. Including `PreludeDir` in the key avoids mutating a shared index. One entry per Prelude directory is enough for this codebase; a bounded LRU can be added later without changing callers.

### 6. Open-document syntax cache

Add `DocumentStore.SyntaxCache: Map[String, SyntaxCacheEntry]` with `SyntaxCacheEntry{Version: Int, Snapshot: SyntaxSnapshot}`. `cachedSyntax(store, uri)` returns the cached snapshot when the open document's version matches, otherwise calls `analyzeSyntax` and stores the result. Only overlays are cached, which bounds memory to the open document set; `storeDelete` removes the entry. Disk-backed files keep their existing path (workspace indexing handles them once per fingerprint).

`runMyGoDiagnostics` is split into a snapshot-based derivation plus a thin wrapper that parses for callers without a store (existing tests). `packageDiagnostics`, `handleCompletion`, `handleDocumentSymbols`, and the hover fallback use `cachedSyntax` so a single request parses a document at most once. Diagnostics continue to be computed and published on every open/change notification; only the redundant parse is removed.

### 7. Benchmark suite in handwritten Go

Add `lsp/perf_benchmark_test.go` in package `lsp` so it can call generated unexported functions directly. Benchmarks:

- `BenchmarkAnalysisFingerprint` with 500/2000/8000 overlay documents, reporting allocations.
- `BenchmarkWorkspacePaths` over a synthetic tree with 500 directories and 20 files each.
- `BenchmarkBuildSourceIndex` over 500 `.mygo` files.
- `BenchmarkAnalyzePackageWarm` over a fixture package, measuring repeated validation with no changes.
- `BenchmarkPackageDiagnostics` over one open document, measuring parse reuse.

Every benchmark calls `b.ReportAllocs()`; results are compared before and after with `-count 3`.

*Alternative considered:* MyGO tests under `lsp/*_test.mygo`. They can assert invariants but cannot express Go benchmarks or allocation counts, so the benchmark lives in Go while behavioral regression tests stay in MyGO.

## Risks / Trade-offs

- **Same-size, same-mtime disk edit is not detected without a file event** → Document the metadata freshness model; editors send `didChangeWatchedFiles`; a content-digest cache can be added later behind the same `FileStamp` abstraction.
- **Fingerprint race during `AnalyzeEditorPackage`** → Store the pre-analysis fingerprint; a concurrent change produces a different stamp on the next request and forces re-analysis.
- **Shared `SourceIndex` maps can be mutated by a careless handler** → Document the read-only contract and key the cache by Prelude directory; add a regression test that two workspace-symbol calls return identical results.
- **Syntax cache grows with open documents** → Cache only overlays and delete on close; no disk entries.
- **Directory epoch misses a change** → The recursive stat walk still runs before the cache is consulted, and `refreshDirectorySources` bumps the epoch on any replacement or deletion.
- **Generated Go drift** → Every `.mygo` edit is followed by `./mygo sync lsp`; tests and `go build ./cmd/mygo-lsp` run against regenerated files.
- **Benchmark noise from Go module/compiler caches** → Use `-count 3`, keep micro-benchmarks free of compiler calls, and report allocations alongside time.

## Migration Plan

1. Add the benchmark suite and record the pre-change baseline.
2. Introduce `FileStamp`/`sourceStamp`/`fingerprintOf`, switch `analysisFingerprint`, delete `frozenAnalysisFingerprint`, and update `refreshPackage`.
3. Replace path deduplication with `Set[String]`, delete `discoverMyGoSources`, and add the directory epoch/flattened-listing cache.
4. Add the workspace `SourceIndex` cache and wire workspace symbols, references, and completion fallback.
5. Add the open-document syntax cache and route diagnostics, completion, symbols, and hover through it.
6. Re-run benchmarks, run `go test ./lsp`, regenerate with `./mygo sync lsp`, and build `cmd/mygo-lsp`.

Rollback is a source revert plus `./mygo sync lsp`, because generated files are checked in and no persisted state or wire format changes.
