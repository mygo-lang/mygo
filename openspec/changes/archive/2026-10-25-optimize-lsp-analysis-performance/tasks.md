## 1. Baseline and Benchmark Harness

- [x] 1.1 Build the bootstrap compiler with `go build -o mygo ./cmd/mygo` and verify `./mygo sync lsp` is available for later regeneration.
- [x] 1.2 Add `lsp/perf_benchmark_test.go` with `BenchmarkAnalysisFingerprint` covering 500, 2000, and 8000 overlay documents and `b.ReportAllocs()`; verify it runs with `go test ./lsp -run '^$' -bench BenchmarkAnalysisFingerprint -benchmem`.
- [x] 1.3 Add `BenchmarkWorkspacePaths` over a synthetic tree and `BenchmarkBuildSourceIndex` over 500 `.mygo` files; verify both run with `-bench 'WorkspacePaths|BuildSourceIndex'`.
- [x] 1.4 Add `BenchmarkAnalyzePackageWarm` and `BenchmarkPackageDiagnostics` over fixture sources; verify both run without touching stdout protocol output.
- [x] 1.5 Run `GOCACHE=/tmp/mygo-lsp-bench go test ./lsp -run '^$' -bench . -benchmem -count 3` and record the pre-change numbers in `openspec/changes/optimize-lsp-analysis-performance/benchmark-results.md`.

## 2. Metadata Fingerprints

- [x] 2.1 Add `lsp/file_stamps.mygo` with `FileStamp`, `sourceStamp(store, path)`, and `fingerprintOf(stamps)` using `os.Stat` size/modification time and overlay version; add a MyGO test covering overlay, disk, and missing files, then verify after `./mygo sync lsp` with `go test ./lsp -run TestFileStamp`.
- [x] 2.2 Rewrite `analysisFingerprint` in `lsp/analysis_cache.mygo` to build stamps and join metadata lines with `strings.Join`; verify `analysisFingerprint` no longer calls `storeGet` and `go test ./lsp -run TestWorkspaceDiscoveryAndReadOnlyAnalysis` passes.
- [x] 2.3 Remove `frozenAnalysisFingerprint` from `lsp/source_snapshots.mygo` and make `refreshPackage` compute the stamp fingerprint over the expanded `watched` list; verify the unsaved-dependency, deletion, and unnotified-disk-change scenarios in `lsp/workspace_test.mygo` and `lsp/workspace_refresh_test.mygo` pass.
- [x] 2.4 Add a MyGO invariant test proving a 1 MB overlay's fingerprint stays small and does not embed document content; verify it fails against the old implementation shape and passes after `./mygo sync lsp`.

## 3. Path Deduplication and Workspace Listing Cache

- [x] 3.1 Replace `Slice.Contains`-then-`Append` collection in `workspacePaths` and `packageSourcePaths` with `Set[String]` membership, and delete the unused `discoverMyGoSources`; verify `go test ./lsp -run 'TestWorkspace|TestCurrentIncompleteMemberCompletions'` passes.
- [x] 3.2 Replace the same pattern in `recordPackageDependencies` and the dependency-closure expansion in `analyzeFilePackage`; verify `go test ./lsp -run 'TestImportedAliasNavigationAndOverlay|TestWorkspaceDiscoveryAndReadOnlyAnalysis'` passes.
- [x] 3.3 Add `DirectoryEpoch` and `WorkspaceListings` to `DocumentStore`, bump the epoch in `refreshDirectorySources` and on listing deletion, and reuse the cached flattened slice in `cachedWorkspaceSources` while using `Slice.Concat` to flatten; verify `BenchmarkWorkspacePaths` reports lower allocations than the recorded baseline.

## 4. Workspace Source Index Cache

- [x] 4.1 Add `WorkspaceIndexCache` and `workspaceIndex(store, preludeDir)` keyed by workspace fingerprint and Prelude directory; verify a second call with unchanged inputs returns the same symbols without rebuilding.
- [x] 4.2 Route `handleWorkspaceSymbols`, `referenceSourceIndex`, and the `completionSourceIndex` fallback through `workspaceIndex`; verify `go test ./lsp -run 'TestIndexedSiblingDefinitionAndReferences|TestImportedAliasNavigationAndOverlay'` passes.
- [x] 4.3 Add a regression test asserting two consecutive workspace-symbol queries and two consecutive references queries return identical results, guarding the read-only index contract.

## 5. Open-Document Syntax Cache

- [x] 5.1 Add `SyntaxCache` to `DocumentStore` and a `cachedSyntax(store, uri)` helper keyed by overlay version; clear the entry in `storeDelete`; verify `go test ./lsp -run 'TestClosedDocumentUsesDisk|TestLifecycleAndDocumentOverlay'` passes.
- [x] 5.2 Split `runMyGoDiagnostics` into a `SyntaxSnapshot`-based derivation plus the existing parsing wrapper, and make `packageDiagnostics` use `cachedSyntax`; verify `go test ./lsp -run 'TestSyntaxDiagnosticsIgnoreStringContents|TestTypeDiagnosticAndCorrection'` passes and one diagnostics request parses the document once.
- [x] 5.3 Use `cachedSyntax` in `handleCompletion`, `handleDocumentSymbols`, and the hover fallback; verify `go test ./lsp -run 'TestHoverTokenBoundaries|TestScopeTypesHoverAndMemberNavigation|TestEnumInterfaceAndImplSymbols'` passes.
- [x] 5.4 Verify `BenchmarkPackageDiagnostics` allocation count drops relative to the recorded baseline.

## 6. Verification and Regeneration

- [x] 6.1 Run `./mygo sync lsp`, then `go test ./lsp` and `go build -o mygo-lsp ./cmd/mygo-lsp`; verify all pass and the binary is produced.
- [x] 6.2 Re-run `GOCACHE=/tmp/mygo-lsp-bench go test ./lsp -run '^$' -bench . -benchmem -count 3` and append post-change numbers plus the allocation/latency deltas to `benchmark-results.md`.
- [x] 6.3 Run the repository's relevant Go test suites for compiler, codegen, and prelude to confirm the regenerated LSP package does not regress shared code.
- [x] 6.4 Run `openspec validate optimize-lsp-analysis-performance --strict` and `git diff --check`; verify both pass with no formatting or validation errors.
