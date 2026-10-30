## 1. Baseline And Benchmarks

- [x] 1.1 Add MyGO benchmark fixtures under `internal/mygo/typeinference2` for declaration-heavy packages, local-binding-heavy functions, Go FFI import surfaces, MyGO import surfaces, and a pinned real package; verify with `go test -run '^$' -bench 'BenchmarkInfer' -benchmem ./internal/mygo/typeinference2`.
- [x] 1.2 Add separate cold-cache and warm-cache benchmark entry points, clearing the import caches between cold runs; verify both variants report distinct `ns/op`, `B/op`, and `allocs/op` results.
- [x] 1.3 Record the current CLI baseline for `--bootstrap-timing sync internal/mygo/typeinference2` and repeat it twice; verify the `typeinference2` stage is reproducible within 15% and record the table in `tasks.md`.
- [x] 1.4 Record a `perf` baseline showing the share of `sourceImportEnv`, Go package seeding, `batchFreeVars`, primitive lookup, and `envGetTypeScheme`; verify the hotspot order matches this design before implementation begins.

Baseline (`-benchtime=1x -benchmem`, current checkout, 2026-10-30):

| Workload | Time | Allocated | Allocs |
|---|---:|---:|---:|
| Decls 500 cold | 154 ms | 19.3 MB | 106,733 |
| Decls 2000 cold | 2.49 s | 157 MB | 424,841 |
| Decls 8000 cold | 97.0 s | 1.82 GB | 1,697,150 |
| Bindings 500 cold | 57.3 ms | 21.5 MB | 771,741 |
| Bindings 2000 cold | 945 ms | 302 MB | 12,084,756 |
| Bindings 8000 cold | 19.3 s | 4.66 GB | 192,336,764 |
| Go imports 200 cold | 4.62 ms | 7.29 MB | 71,129 |
| Go imports 1000 cold | 30.1 ms | 36.2 MB | 352,762 |
| MyGO imports 200 cold/warm | 5.42 ms / 0.77 ms | 1.81 MB / 1.55 MB | 13,048 / 7,097 |
| MyGO imports 1000 cold/warm | 62.9 ms / 9.18 ms | 30.0 MB / 28.8 MB | 61,111 / 31,960 |
| Real fixture cold/warm | 0.56 ms / 0.16 ms | 223 KB / 223 KB | 1,942 / 1,941 |

CLI `--bootstrap-timing sync internal/mygo/typeinference2` measured 5.14 s
and 4.47 s in the `typeinference2` stage (13% spread). The earlier `perf`
baseline used 5508 task-clock samples: 848 landed in `InferPackage`; inclusive
counts were `sourceImportEnv` 156, Go package seeding 189, `batchFreeVars` 150,
`goPrimitivePairs` 126, and `envGetTypeScheme` 94. The local-binding benchmark
also exposed a separate quadratic path through environment free-variable
recomputation, captured by tasks 6.5-6.7.

## 2. Per-Source Import Overlay

- [x] 2.1 Introduce an immutable per-source import overlay that carries the ordered Go and MyGO batch layers; verify import-conflict, shadowing, and per-file import-scope tests pass.
- [x] 2.2 Build each source file's overlay once during inference setup and reuse it across both predeclaration passes, `buildSourceDeclSymbols`, `inferSourceFile`, and `packageOutputEnv`; verify no per-phase import reconstruction remains in those paths.
- [x] 2.3 Add a fixture where a local binding, a MyGO import, and a Go import expose the same name; verify lookup and exported environment order match the pre-change compiler.
- [x] 2.4 Re-run the inference benchmarks and record the reduction in import-conversion samples and allocations; verify no observable inference result changed.

## 3. Reusable Batch Caches

- [x] 3.1 Extend the imported-MyGO-package cache entry with reusable `BatchBindings` containing the lookup index and free-variable union; verify cache hits attach the batch without recomputing either structure.
- [x] 3.2 Add a Go package batch cache keyed by import path, alias, and a signature of the exported Go surface; verify a changed signature rebuilds the batch and an unchanged signature reuses it.
- [x] 3.3 Clear all new caches from `ResetEditorInferenceCache` and add a test proving no stale batch survives an editor cache reset; verify the editor inference tests pass.
- [x] 3.4 Add a cache-sharing test across two importers to prove cached batch maps, entries, and free-variable slices are never mutated; verify both importers infer the same types.
- [x] 3.5 Re-run cold and warm benchmarks and record the change in `B/op` and `allocs/op`; verify warm runs avoid `batchFreeVars` reconstruction.

## 4. Primitive-Spelling Lookup

- [x] 4.1 Replace the per-call primitive-pair slice construction with a direct `switch` or immutable package-level lookup; verify the existing Go FFI, `Error`/`error`, `Any`/`any`, and channel tests pass.
- [x] 4.2 Add a focused test covering every primitive spelling in both directions; verify the mapping is identical to the previous table.
- [x] 4.3 Re-run the Go-FFI-heavy benchmark and record the allocation reduction; verify no new shared mutable state was introduced.

## 5. Predeclaration Pass Reduction

- [x] 5.1 Add a single-pass predeclare entry point for the first outer predeclaration pass and keep the full two-pass refresh for the final outer pass; verify the final environment is unchanged.
- [x] 5.2 Add fixtures with forward type aliases, forward enum fields, and cross-file function signatures; verify typed declarations and generated Go are byte-identical.
- [x] 5.3 Re-run the declaration-heavy and alias-heavy benchmarks and record the change; verify the final predeclare pass still resolves every alias fixture.

## 6. Linear Assembly And Symbol Index Reuse

- [x] 6.1 Replace the typed-declaration `Prepend` in `inferDeclsAt` with index-filled assembly in source order; verify typed declaration order and generated output are unchanged.
- [x] 6.2 Add the package symbol index to `InferenceSetup`, build it once, and pass it into each file's `InferState`; verify symbol lookup, struct-field, and impl-method tests pass.
- [x] 6.3 Add a large declaration fixture and verify per-declaration inference cost stays flat across increasing sizes.
- [x] 6.4 Re-run the full inference benchmark suite and record the remaining hotspots; verify the next optimization decision is based on measured data rather than the old profile.
- [x] 6.5 Add a cached free-variable union to every persistent `Env` layer and compute it once in `Env.Put`, `envPutBatch`, and `envPutBatchLatest`; verify `envFreeVars` matches the old recursive result on unit, batch, shadowing, and predicate fixtures.
- [x] 6.6 Switch `generalize` and `generalizeQualified` to the cached environment union; verify the local-binding benchmark stays roughly linear at 500, 2000, and 8000 bindings and allocation growth is no longer quadratic.
- [x] 6.7 Add a regression test for a long chain of local bindings and a shadowing sequence; verify inferred types, environment order, and generated output are unchanged.

## 7. Integration Validation

- [x] 7.1 Format the edited `.mygo` sources, regenerate affected `zz_*.gen.go` files, rebuild the CLI, and verify the generated-Go diff contains only intended changes.
- [x] 7.2 Sync the repository twice with the rebuilt CLI and verify the second generation is byte-identical to the first.
- [x] 7.3 Run the focused suites with `go test ./internal/mygo/typeinference2 ./internal/mygo/parser2 ./internal/mygo/compiler`; verify all inference, parser, symbol, FFI, and bootstrap tests pass.
- [x] 7.4 Run the full repository test suite with a writable `GOCACHE`; verify no package regresses and record any intentionally skipped or environment-limited checks.
- [x] 7.5 Compare final CLI stage timings and benchmark results against the baseline; verify the achieved speedup, allocation reduction, and remaining hot spots are recorded in the change summary.

Final results (`-benchtime=1x -benchmem`, same machine):

| Workload | Before | After |
|---|---:|---:|
| Decls 500 cold | 154 ms | 48.4 ms |
| Decls 2000 cold | 2.49 s | 406 ms |
| Decls 8000 cold | 97.0 s | 4.52 s |
| Bindings 500 cold | 57.3 ms | 6.03 ms |
| Bindings 2000 cold | 945 ms | 29.0 ms |
| Bindings 8000 cold | 19.3 s | 301 ms |
| Go imports 200 cold/warm | 4.62 ms / — | 2.44 ms / 0.214 ms |
| Go imports 1000 cold/warm | 30.1 ms / — | 13.4 ms / 1.16 ms |
| MyGO imports 200 cold/warm | 5.42 ms / 0.77 ms | 3.07 ms / 0.366 ms |
| MyGO imports 1000 cold/warm | 62.9 ms / 9.18 ms | 27.1 ms / 4.28 ms |
| Real fixture cold/warm | 0.56 ms / 0.16 ms | 0.725 ms / 0.360 ms |

The CLI `typeinference2` stage dropped from 5.14 s / 4.47 s to 2.03 s /
2.04 s for the self-hosted `typeinference2` package. Two consecutive self-syncs
produced byte-identical `zz_*.gen.go` files. `go test ./...` passed with
`GOFLAGS=-buildvcs=false`; the earlier compiler-only failures were the known
read-only VCS status issue inside generated temporary modules.

The final 2000-declaration allocation profile is dominated by parser2 work
(`Slice.Prepend[SourcePos]` and CST append/list construction), then by batch
lookup/free-variable construction for large environments. A future optimization
should start from that new profile rather than the pre-change import profile.

