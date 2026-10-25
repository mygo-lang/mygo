## 1. Baseline and guardrails

- [ ] 1.1 Add `lsp` benchmarks for a multi-file package with one open document (`BenchmarkEditorAnalyzeEditOneOpen`) and with five open documents (`BenchmarkEditorAnalyzeEditFiveOpen`), including a never-edited dependency package; verify `go test ./lsp -run XXX -bench 'BenchmarkEditorAnalyzeEdit' -benchmem` runs and records ns/op and allocs/op.
- [ ] 1.2 Add cache counters (parse hits, parse misses, package analyses) to the analysis path without changing results; verify a unit test can read the counters after a warm package analysis.
- [ ] 1.3 Record the pre-change numbers for scenarios A (edited package + Prelude) and B (A plus an unchanged 70–80 KB dependency) in the benchmark output or test log; verify the recorded medians match the proposal's baseline order of magnitude.
- [ ] 1.4 Add a test fixture builder that creates a temp module with two packages and the repository prelude replace directive, so benchmarks and tests share one realistic workspace shape; verify both groups below reuse it.

## 2. Deterministic package identity

- [ ] 2.1 Sort the path list used by `analysisFingerprint` and make `fingerprintOf` order-independent for the same path set; verify a unit test computes equal fingerprints from shuffled input paths.
- [ ] 2.2 Store `PackageSnapshot.WatchedPaths` in canonical order and compare snapshots by the canonical identity; verify a unit test calls `analyzePackage` twice for the same package and observes one package analysis.
- [ ] 2.3 Collect file stamps once per analysis cycle and reuse them for every open-document publication; verify a test with five open documents performs one stamp collection per edited version.
- [ ] 2.4 Add a regression test that five open documents in one package, with one edit, perform at most one `AnalyzeEditorPackage` call; verify it fails against the pre-change behavior and passes after 2.1–2.3.

## 3. Per-file parse cache

- [ ] 3.1 Define the cache entry (stamp, source origin, `codegen2.SourceFileInput`, `typeinference2.PkgDeclSource`, optional `parser2.SyntaxTree`) and stamp derivation for overlay versus disk files; verify unit tests cover overlay version change, disk mtime/size change, and missing file.
- [ ] 3.2 Add the cache owner to the LSP document store and thread it through `compiler.AnalyzeEditorPackage` and the editor loader as an optional argument; verify compiler-only callers (CLI/bootstrap sync) still compile and behave unchanged.
- [ ] 3.3 Make the editor loader consult the cache before `bootstrapParseSource` and populate it after a parse; verify a test asserts zero parses for unchanged files on the second analysis of the same sources.
- [ ] 3.4 Ensure Prelude and imported workspace packages are cached by the same per-file mechanism; verify scenario B shows no additional parse time when the unchanged dependency is enlarged.

## 4. One parse per changed document version

- [ ] 4.1 Expose the parsed products from `cachedSyntax` to the package-analysis loader (or have both consult the same cache entry) so the edited document is parsed once per version; verify a test counts one parse for the edited path per edit.
- [ ] 4.2 Keep CST storage limited to open documents while keeping AST products for all analysed files; verify a memory-oriented test asserts a closed document no longer retains its CST entry.

## 5. Invalidation and lifecycle

- [ ] 5.1 Verify overlay close falls back to disk stamps and reparses when content differs; add a test that closes an edited buffer and asserts disk content is analysed.
- [ ] 5.2 Verify editing an imported package signature invalidates dependents and reparses only affected files; add a test covering the dependent package's updated result.
- [ ] 5.3 Confirm watched-file notifications still invalidate package snapshots while parse entries are validated by stamp; verify the existing watched-file tests still pass.
- [ ] 5.4 Add a warm-vs-cold equivalence test: analyze, edit one file, analyze again, and compare diagnostics, symbols, hover, completion, definitions, and references against a fresh analysis of the same current sources.

## 6. Integration and performance verification

- [ ] 6.1 Regenerate MyGO outputs after `.mygo` edits (`go run ./cmd/mygo sync .`) and verify `go build ./...` succeeds with regenerated `zz_*.gen.go`.
- [ ] 6.2 Run `go test ./lsp ./internal/mygo/compiler ./internal/mygo/parser2` and verify all existing tests pass.
- [ ] 6.3 Run `go test ./...` and verify the full suite passes.
- [ ] 6.4 Re-run the task 1.1 benchmarks and verify: five-open-document edit time is within ~1.2× of the one-open-document edit time, and one edit reparses only the changed file plus genuinely changed dependencies.
- [ ] 6.5 Verify scenario B's per-edit time no longer grows with the unchanged dependency's size beyond the inference-floor noise band, and record final numbers in the change notes.
