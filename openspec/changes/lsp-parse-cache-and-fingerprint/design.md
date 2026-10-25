## Context

See `proposal.md` for motivation. Current state that shapes the design:

- Edit handling is synchronous: `routeMessage` stores the overlay, then `publishOpenDiagnostics` publishes diagnostics for **every** open document (`lsp/file_events.mygo`), and each publication calls `packageDiagnostics` → `analyzePackage` (`lsp/semantic_diagnostics.mygo`).
- `analyzePackage` compares a fingerprint and, on mismatch, calls `compiler.AnalyzeEditorPackage`. That function creates a fresh `BootstrapState`, resets the editor inference cache, and `editorLoadDependency` parses every file of the package, every imported workspace MyGO package, and the Prelude (`internal/mygo/compiler/editor_analysis.mygo`, `editor_loader.mygo`).
- The fingerprint is a `\n`-joined list of `path|present|size|modified|version`. Its path order comes from `packageSourcePaths` (disk order + `store.docs.ToPairs()` map order) and from `EditorPackage.Sources.ToPairs()` (map order), so identical content can hash differently between calls (`lsp/package_sources.mygo`, `lsp/file_stamps.mygo`, `lsp/analysis_cache.mygo`).
- Open-document syntax is already cached per version in `store.SyntaxCache`, but only open documents use it; the edited document is parsed again by package analysis.
- Measured baseline (Ryzen 9 PRO 7940HS): 98.5 KB workspace → one edit 195 ms with parser2 ≈ 150 ms (77%); adding a never-edited 73.7 KB package → 423 ms (parser2 ≈ 319 ms); five open documents in one package → 971 ms / 2.2 s because each publication runs a full analysis.

## Goals / Non-Goals

**Goals:**
- Make package identity deterministic so unchanged packages reuse their snapshot within and across requests.
- Make one edit cost one package analysis and one parse of the changed document version, independent of open-document count and unchanged dependency size.
- Preserve exact diagnostic and language-feature results, including overlay/disk precedence and dependency invalidation.

**Non-Goals:**
- Parser2 throughput (separate change).
- Caching type inference across analyses; the inference pass still runs per refreshed package.
- Changing the LSP protocol surface, diagnostic wording, ordering, or positions.
- Persistent on-disk caches.

## Decisions

### 1. Canonical, order-independent package identity

Compute the identity from a sorted path list and a stable one-line-per-stamp serialization; sort both the discovered paths and the stored `WatchedPaths`. Build one stamp snapshot per analysis cycle and reuse it for all publications of that cycle.

Rationale: elements are already stamped correctly; only ordering is unstable. Sorting is O(n log n) on a small list and removes a map-iteration dependency without touching invalidation semantics.

Alternatives:
- Content hashing: stronger identity but reads every byte on every check; the existing stamps already change on version/mtime/size, so the extra cost buys little.
- Incremental/hashed set with commutative combining: fewer lines serialized but harder to debug and still needs the same stamp collection.

### 2. Per-file parse cache shared across analyses

Introduce a parse cache keyed by absolute source path. An entry holds the stamp it was produced from plus the parsed products needed by analysis: `codegen2.SourceFileInput` (with expression IDs), `typeinference2.PkgDeclSource`, and, only for open documents, the `parser2.SyntaxTree`.

Stamp shape distinguishes origin: open overlay → `overlay|version|content length`; disk → `disk|size|modification time`. A mismatch reparses; an overlay close naturally falls back to the disk stamp and misses.

Cache location: owned by the LSP `DocumentStore` and passed into `AnalyzeEditorPackage` as an optional `Ref` (or into a persistent `BootstrapState`). When no cache is supplied — compiler and CLI callers — behavior is unchanged.

Rationale: the expensive unit is one file parse+lower+ID assignment. Directory-level caching cannot help because editing one file would invalidate all its siblings. Per-file keys also automatically cover the Prelude and imported packages.

Alternatives:
- Keep `BootstrapState.SourcesCache` and make it process-wide: too coarse (directory granularity, no stamp validation).
- Cache only the CST and re-lower on miss: loses the ID/declaration products and repeats work.
- Cache the whole `EditorPackage` per package: already exists as `PackageSnapshot`; it does not help when the package itself changed, which is exactly when edits happen.

### 3. One parse per changed document version

The parse cache is the single source of truth for a file version. `cachedSyntax` (syntax diagnostics and language features) and the package loader both consult it. When a document is opened or changed, the first consumer parses and stores CST + AST products; the second consumer hits the entry. Unopened files store AST products only, keeping memory bounded by analysed source rather than by CST size.

Rationale: preserves the existing `SyntaxSnapshot` contract for open documents while removing the duplicate parse observed today.

### 4. Snapshot reuse without weakening invalidation

`store.Analyses` keeps one snapshot per package directory, compared by canonical identity. When several open documents publish diagnostics for the same version, the first refresh stores the snapshot and the rest compare equal and reuse it. `invalidatePackage` / `invalidatePackageSnapshot` continue to handle explicit file events; because parse entries are stamp-validated, they need no separate purge (an optional size cap can evict later).

Rationale: keeps the existing dependency-invalidation model, which already records dependents and watched paths.

### 5. Observability for tests and benchmarks

Add internal counters (parse hits, parse misses, package analyses, parsed bytes) on the cache owner. They are not exposed over the protocol; tests assert that one edit yields one package analysis and one parse of the changed path, and benchmarks report them.

## Risks / Trade-offs

- **Downstream mutation of reused AST** → Cache entries are treated as immutable. `AssignFileExprIDs` already returns a fresh `File`; add a warm-vs-cold equivalence regression (analyze, edit, re-analyze vs fresh process) covering diagnostics, symbols, hover, completion, definitions, references. If any pass mutates shared nodes, cache pre-ID lowering output and rerun the cheap ID step.
- **Memory growth from caching ASTs for many files** → Store CSTs only for open documents; rely on AST-only entries for the rest; measure heap in a large-workspace benchmark and add an LRU or workspace-scoped eviction if needed.
- **Wrong or incomplete identity** (e.g., dependency path missing) → Keep `WatchedPaths` semantics; add a test where editing an imported package changes results in the dependent package.
- **Overlay/disk ambiguity after close** → Origin is part of the stamp; explicit test: close an edited buffer and assert the next analysis reflects disk content.
- **Stale diagnostics ordering** → Diagnostics still derive from the current snapshot; the change only removes recomputation. Existing version checks in `publishDiagnostics` remain.
- **Cache key churn under `os.Stat` failures** → A missing file is represented as `present=false` in the stamp, matching today's `sourceStamp` behavior.

## Migration Plan

1. Land canonical identity first (small, independently verifiable; removes the 5× multi-document amplification).
2. Introduce the cache behind an injected `Ref`, defaulting to the current behavior when absent.
3. Switch `AnalyzeEditorPackage`/`editorLoadNamedInputs` to cache-aware loading; switch `cachedSyntax` to the same entries.
4. Verify with the LSP suite, warm/cold equivalence tests, and the multi-document benchmark; then remove any temporary instrumentation.

Rollback is configuration-free: stop injecting the cache (or pass `None`) and the server returns to the previous full-analysis path. No protocol or generated-code migration is involved.
