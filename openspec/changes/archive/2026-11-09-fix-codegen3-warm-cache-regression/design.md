## Context

See the proposal and delta specs. The current dependency-declaration path can call uncached compilation directly; a sampled warm build was inside Go FFI loading through that path. The summary walker has a recursion-stack guard but no completed-node memo: the recorded compiler dependency graph has 202 visits for 13 summaries, including 101 visits to prelude. This establishes repeated work, not its share of the measured timeout. Module-wide Go input scanning also runs during state creation with caching disabled. Cache metadata is staged and read through generated Go files; its cost has not yet been isolated.

Probing the actual staged self-host cache exposed a more direct cause: most metadata reconstruction attempts failed because named prelude types such as `Option` carry the prelude package path but have no explicit import alias. The failure silently selected the full frontend fallback. Metadata reconstructs import declarations as well as exported signatures; on-demand reuse must retain those imports for transitive collection, while metadata without sufficient declarations or valid integrity checks must fall back. A dependency already on the active compilation stack must not be treated as a successful cached cycle.

Allowing those reconstructed declarations into inference exposed a second problem. `ExportedDecls` gives functions and values unit-valued placeholders and does not reconstruct inherent `impl` blocks. The prelude path supplies these declarations as external source to inference, which tries to infer the placeholders. The self-host test reported `List.Cons` missing and a return-type error in `ListToSlice`. The metadata reader also leaves declaration positions empty. Reconstructing an AST successfully therefore does not establish that it is safe for inference.

`bootstrapDependencySourcesDiagnostic` needs typed declarations and their imports for transitive package collection. Prelude declarations take a separate external-source inference path. The package key includes both main and external-test inputs and their imports, but the existing importer metadata is built from the main package and does not establish completeness for either inference path. The compiler must therefore check the requested input kind as well as the cache entry. Active compilation and summary traversal both need cycle checks before a cached declaration is accepted.

The main `bootstrap-codegen3` spec's old ban on metadata contradicts the existing generator and the verified-reuse performance specification. This change replaces that prohibition with a verified metadata contract; it does not introduce a new public metadata ABI.

## Goals / Non-Goals

**Goals:**
- Eliminate the reproduced warm self-host regression and bound redundant work while retaining correctness and safe cache fallback.
- Make the measured dominant costs and any remaining performance limits visible in reproducible results.

**Non-Goals:**
- Changing language semantics, the generated representation, cache locking contracts, editor overlay precedence, or source-tree output containment.
- Assuming the sampled stack or dependency visit counts alone prove the full time attribution.

## Decisions

### Unify dependency reuse with a correctness-preserving fallback

Trace every route that supplies importer declarations, including `bootstrapDependencySourcesDiagnostic`, prelude loading, test inputs and transitive imports. Keep two distinct cache uses:

1. An output-only hit verifies the package summary, dependency inputs and generated entry before reusing generated files. It does not install reconstructed declaration stubs into the source or typed-declaration caches. If a later importer needs declarations, it must use the declaration path below.
2. A declaration hit requires a versioned, complete typed dependency summary. It supplies importer-facing information to an explicitly pretyped inference path. The compiler must not pass reconstructed placeholder bodies to the source inference path. An unchanged dependency with a complete verified summary can be reused without parsing, FFI collection or inference; otherwise use cycle-guarded compilation from current source.

The existing package metadata describes an exported API and dynamic dispatch. It does not prove that a package can be seeded as a typed dependency. Add a separate versioned cache representation and completeness marker for that purpose, or extend the cache summary with an equivalent distinct typed section. Capture the data actually consumed by inference and code generation: package identity and imports; inferred binding and function types; type declarations; inherent methods, typeclass instances, constraints and method signatures; dynamic-dispatch information; and source paths and declaration positions where diagnostics need them. Preserve the distinction between main, internal test and external test inputs. Record source paths relative to the package or module and rebase them on load so cache sharing does not embed checkout-specific absolute paths.

Seed the inference environment, method and instance lookup, and downstream code generation from the verified typed representation. Keep prelude on the same contract, including implicit prelude names, instead of treating metadata stubs as its source. If the current inference API cannot consume a pretyped dependency, extend the API and its callers explicitly. Do not infer unit-valued stub bodies or treat the presence of an API metadata file as a completeness marker. Compare inferred results and diagnostics with source compilation before enabling a declaration hit for a package or input kind. Preserve overlay-aware loading and bypass persistent disk summaries when active overlays are not represented.

### Memoize verified graph results and fingerprints within a build

Use separate visiting and successfully validated state, keyed by resolved package identity and all relevant source, dependency and build-context identities. Publish a memo only after complete validation succeeds; preserve cycle detection and input-key checks on every edge. Reuse a package source digest and validated summary across all roots of one invocation, not across independent invocations. Treat the build's input view consistently: where concurrent file mutation or overlay stamps cannot be safely represented, invalidate the memo or fall back, rather than relying on timestamps as content proof. Avoid duplicate digest calculations when constructing summaries; retain atomic cache publication, checksums and existing lock behavior.

### Avoid work that cannot contribute to a cache hit

Compute module and local replacement digests only when the cache is enabled and needed, and reuse their results within the invocation. Preserve the existing breadth of Go, module, external test and replacement invalidation for enabled caches; do not replace content hashing with unchecked mtime shortcuts or narrow dependency coverage without proof. Measure metadata copying, manifest verification, metadata scanning/decoding and FFI loads separately; reduce repeated verified work or stage only required content if compatible with integrity and output guarantees. Do not assume metadata copying is the dominant cause before measurement.

### Verify work avoided as well as elapsed time

Add deterministic counters or test hooks for package parsing, FFI, inference, summary validation, source hashing and metadata loading. Exercise a diamond/shared dependency graph and a changed importer, asserting bounded successful validation and no repeated front-end work on verified dependency hits. Compare output and diagnostics with cache-off, including corrupted summaries, missing entries, changed transitive sources, Go inputs, replacements, compiler/toolchain/options, external tests and overlay behavior.

Use isolated source trees and output/cache roots, same Go version and warmed Go build cache, separate caches per compiler, and alternating repeated runs. Measure both the pre-`cc5d576e9f` baseline and the final compiler on equivalent self-host build and test workloads; record source/revision and binary hashes, runtime/build backend, optimization level, cache state, stage counts, wall time and resource data. Include cold, warm, disabled and incremental cases; check generated output and diagnostics. Compare medians and ranges and investigate overlapping/noisy results; warm build and test must show no regression relative to baseline, not merely beat the 150-second timeout. If full comparisons are blocked or inconclusive, leave performance acceptance incomplete and document the reason.

## Risks / Trade-offs

- Incomplete typed summaries can seed the wrong declarations. Compare prelude, methods, instances, generics and changed importers against cache-off; fall back when completeness cannot be demonstrated.
- Cached declarations can lose source locations. Round-trip package-relative locations and verify diagnostic paths, lines and columns in another checkout.
- Validation memoization can retain stale or partial results. Scope it by build and complete input identity, mark success only after all checks, and test corruption, concurrent cache publication and input changes.
- LSP analysis can regress to saved disk declarations. Keep overlay-aware loading and run the unsaved dependency signature and close-buffer regressions.
- Metadata handling or the Go toolchain may dominate after traversal is fixed. Attribute stage costs, profile if needed, and fix material remaining causes before claiming the performance goal.
- Different revisions and host noise can distort the baseline. Record exact trees and tools, alternate repeated runs and separate source changes, cache preparation and Go toolchain time.

## Migration Plan

Give the typed dependency representation its own version and completeness marker. Older entries without a complete typed summary remain misses for declaration reuse; their generated output may be reused only if its own checks pass. Roll back by reverting the implementation and ignoring incompatible entries. Correctness must not require manual cache deletion. Update the conflicting main-spec requirement through the normal OpenSpec sync after implementation and verification.
