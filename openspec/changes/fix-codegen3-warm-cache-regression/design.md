## Context

See the proposal and delta specs. The current dependency-declaration path can call uncached compilation directly; a sampled warm build was inside Go FFI loading through that path. The summary walker has a recursion-stack guard but no completed-node memo: the recorded compiler dependency graph has 202 visits for 13 summaries, including 101 visits to prelude. This establishes repeated work, not its share of the measured timeout. Module-wide Go input scanning also runs during state creation with caching disabled. Cache metadata is staged and read through generated Go files; its cost has not yet been isolated.

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

Trace every route that supplies importer declarations, including `bootstrapDependencySourcesDiagnostic`, early hits, test inputs and transitive imports. Route codegen3's on-demand request through the same verified summary and entry checks as other hits, and retain a cycle-guarded source compilation path for missing, invalid or incomplete metadata. Ensure reused declarations include everything downstream inference and code generation need, including import relationships, dynamic metadata and source locations. Do not blindly replace the uncached call with the top-level compile function: confirm its state, recursion, codegen and error contracts first. Preserve overlay-aware source loading for non-codegen3/editor paths and bypass persistent disk summaries whenever they cannot represent active overlays.

### Memoize verified graph results and fingerprints within a build

Use separate visiting and successfully validated state, keyed by resolved package identity and all relevant source, dependency and build-context identities. Publish a memo only after complete validation succeeds; preserve cycle detection and input-key checks on every edge. Reuse a package source digest and validated summary across all roots of one invocation, not across independent invocations. Treat the build's input view consistently: where concurrent file mutation or overlay stamps cannot be safely represented, invalidate the memo or fall back, rather than relying on timestamps as content proof. Avoid duplicate digest calculations when constructing summaries; retain atomic cache publication, checksums and existing lock behavior.

### Avoid work that cannot contribute to a cache hit

Compute module and local replacement digests only when the cache is enabled and needed, and reuse their results within the invocation. Preserve the existing breadth of Go, module, external test and replacement invalidation for enabled caches; do not replace content hashing with unchecked mtime shortcuts or narrow dependency coverage without proof. Measure metadata copying, manifest verification, metadata scanning/decoding and FFI loads separately; reduce repeated verified work or stage only required content if compatible with integrity and output guarantees. Do not assume metadata copying is the dominant cause before measurement.

### Verify work avoided as well as elapsed time

Add deterministic counters or test hooks for package parsing, FFI, inference, summary validation, source hashing and metadata loading. Exercise a diamond/shared dependency graph and a changed importer, asserting bounded successful validation and no repeated front-end work on verified dependency hits. Compare output and diagnostics with cache-off, including corrupted summaries, missing entries, changed transitive sources, Go inputs, replacements, compiler/toolchain/options, external tests and overlay behavior.

Use isolated source trees and output/cache roots, same Go version and warmed Go build cache, separate caches per compiler, and alternating repeated runs. Measure both the pre-`cc5d576e9f` baseline and the final compiler on equivalent self-host build and test workloads; record source/revision and binary hashes, runtime/build backend, optimization level, cache state, stage counts, wall time and resource data. Include cold, warm, disabled and incremental cases; check generated output and diagnostics. Compare medians and ranges and investigate overlapping/noisy results; warm build and test must show no regression relative to baseline, not merely beat the 150-second timeout. If full comparisons are blocked or inconclusive, leave performance acceptance incomplete and document the reason.

## Risks / Trade-offs

- [Incomplete metadata or lost imports] → Exercise transitive and changed-importer behavior against cache-off; fall back when completeness cannot be demonstrated.
- [Memoized stale or partial validation] → Scope by build and complete input identity, mark success only after all checks, and test corruption, concurrent cache publication and source/input changes.
- [Regression to saved disk declarations in LSP] → Keep overlay-aware loading and run existing unsaved dependency signature and close-buffer regressions.
- [Metadata handling or Go toolchain dominates after fixing traversal] → Attribute stage costs, profile if needed, and fix material remaining causes before claiming the performance goal.
- [Different revisions and host noise distort baseline] → Record exact trees and tools, alternate repeated runs and separate source changes, cache preparation and Go toolchain time.

## Migration Plan

Keep compatible cache entries when their verification remains sufficient; bump the relevant format when semantics or stored information change, making older entries safe misses. Roll back by reverting the implementation and ignoring incompatible cache entries; do not require manual cache deletion for correctness. Update the conflicting main-spec requirement through the normal OpenSpec sync after implementation and verification.
