## Context

See `proposal.md` for motivation. The current self-hosted pipeline builds
`parser2 -> ast2 -> typeinference2 -> codegen2`, so inference operates on
already-parsed source files and must preserve the typed AST consumed by
`codegen2`.

On the current checkout, syncing `internal/mygo/typeinference2` takes about
2.4 s in `parser2`, 4.6 s in `typeinference2`, and 2.4 s in `codegen2`. A
`perf` sample of 5508 task-clock events attributed 848 samples to the
`InferPackage` stack. Within that stack, the largest inclusive hotspots were
`sourceImportEnv` (18.4%), Go package seeding (22.3%), `batchFreeVars` (17.7%),
`goPrimitivePairs` (14.9%), and `envGetTypeScheme` (11.1%). These percentages
overlap, but they identify structural repetition rather than algorithmic cost
in unification.

Each source file currently calls `sourceImportEnv` during two predeclaration
passes, symbol-table construction, and source inference; package-output
construction seeds imports once more. Go package members are converted from
signature strings on every call. MyGO package entries are cached, but the cache
stores only ordered entries: `envPutBatch` and `envPutBatchLatest` still rebuild
the lookup index and the free-variable union on every cache hit.

## Goals / Non-Goals

**Goals:**

- Make repeated import conversion happen once per source-file setup and reuse
  the immutable result across all inference phases.
- Cache both Go and MyGO import batches with their lookup indexes and
  free-variable unions, while preserving the existing cache-key discipline.
- Remove allocation-heavy constant lookups from the Go type parser.
- Reduce redundant declaration predeclaration without changing forward-alias
  resolution.
- Remove the remaining quadratic typed-declaration assembly and repeated
  per-file symbol-index construction.
- Establish benchmarks and a baseline before changing behavior.

**Non-Goals:**

- Replacing the Hindley-Milner algorithm, changing unification rules, or
  changing inferred types.
- Redesigning the persistent local `Env` chain or `envGetTypeScheme` in this
  change; profile the result first and revisit that design separately if it is
  still material.
- Incremental package inference, LSP caching changes, or persistent on-disk
  caches.
- Changing `codegen2` output or the IR3/codegen3 pipeline.
- Collapsing the source predeclaration model into a new declaration graph.

## Decisions

### Decision: Precompute one ordered import overlay per source file

`InferenceSetup` will build, for each `PkgDeclSource`, an ordered list of
immutable batch layers that represents that file's Go and MyGO imports. The
layers are built once from the source declarations and package metadata, then
applied to whichever base environment a phase needs.

This keeps lexical lookup and `ToSlice` ordering intact while removing the
repeated signature parsing, member seeding, conflict checking, and batch
construction from the two predeclare passes, `buildSourceDeclSymbols`,
`inferSourceFile`, and `packageOutputEnv`.

Alternatives considered:

- Cache a complete `Env` per source file: rejected because each phase layers
  imports onto a different base environment and shadowing would change.
- Merge every import into one flat map: rejected because `PackageInfo.Env`
  exposes an ordered `EnvEntry` list and duplicate-name precedence is
  observable.

### Decision: Reuse prebuilt `BatchBindings` instead of rebuilding batch internals

`ImportedPackageCacheEntry` will carry the reusable batch for its exported
entries, including the lookup map and free-variable union. A new `envPutBatch`
variant will attach an existing `BatchBindings` to a base environment without
re-indexing or re-walking the schemes. Go packages get an equivalent process
level cache keyed by import path, alias, and a signature that changes whenever
the exported Go surface changes.

The existing `envPutBatch` and `envPutBatchLatest` remain the construction
paths for ad hoc source-declaration layers, because their duplicate-name rules
differ. Cached batches are immutable values and can be shared across importers.

Alternatives considered:

- Keep recomputing only `FreeVars` on cache hits: rejected because profiling
  shows `batchFreeVars` is one of the largest inference-stack costs.
- Key Go caches by path and alias alone: rejected because separate workspaces
  or test fixtures can publish different surfaces for the same path.

### Decision: Use a zero-allocation primitive-spelling lookup

`PrimitiveGoSpelling` and `PrimitiveMyGoSpelling` will use a direct `switch` or
an immutable package-level table instead of constructing a new 21-element
`GoPrimitivePair` slice at each call. The mapping stays in one place so the
`Error`/`error`, `Any`/`any`, and channel-related behavior remains centralized.

Alternatives considered:

- Lazily initialize a global slice: rejected because a direct switch is simpler,
  allocation-free, and avoids shared mutable state.

### Decision: The first predeclaration pass performs one refresh; the final pass performs the full refresh

`predeclareSourceFiles` currently runs twice, and each invocation calls the
two-pass `predeclareAllFunctionsWithGoPackages`. The first outer pass will use a
single-pass predeclare because later files and later aliases are not visible yet.
The second outer pass keeps the full two-pass refresh that resolves forward
aliases. This removes one of four predeclare scans per file without changing the
final environment.

The no-type-alias fast path is deferred until differential tests prove it. A
package with no aliases may eventually skip the second outer pass, but that is
a separate measured decision.

Alternatives considered:

- Collect declarations in a separate graph and resolve signatures in one pass:
  rejected because it changes the ordering model and is too broad for this
  optimization.

### Decision: Assemble typed declarations and symbol indexes linearly

`inferDeclsAt` will fill an output slice in source order instead of prepending
each typed declaration into a newly allocated slice. `buildSourceSymbolTable`
will return the symbol index alongside the symbol slice, and `InferenceSetup`
will carry it so each file does not rebuild the same index.

The typed declaration order and symbol precedence remain contracts. The change
only removes per-element allocation and repeated indexing work.

Alternatives considered:

- Sort typed declarations after inference: rejected because inferred order is
  not necessarily recoverable from source positions and would add work.

### Decision: Cache the free-variable union on every persistent Env layer

The local-binding benchmark exposed a second quadratic path that the import
work does not touch: every `let` calls `generalize`, which recomputes free
variables for every parent environment layer. At 8000 bindings this reaches
19.3 s and 4.66 GB allocated for one inference.

Each `Env` node will store the union of free type variables from its own entry
or batch and its parent. `Env.Put`, `envPutBatch`, and `envPutBatchLatest`
compute that union once when the layer is created; `envFreeVars` then unions the
caller's accumulator with the cached slice. The existing bound-variable removal
and predicate handling move into the same helper so their semantics stay
identical.

Alternatives considered:

- Replace the persistent chain with a mutable hash map: rejected because
  sibling inference branches rely on immutable snapshots.
- Cache only the batch free variables: rejected because the local binding
  benchmark shows the per-entry chain traversal is the dominant cost.

## Risks / Trade-offs

- **[Cached batches become stale across workspaces or editor requests]** ->
  Include package identity and a surface signature in every cache key, and
  clear all new caches in `ResetEditorInferenceCache`.
- **[Flattening imports changes shadowing or exported-entry order]** -> Keep
  imports as ordered layers, preserve `envPutBatch` versus
  `envPutBatchLatest`, and compare `PackageInfo.Env` order before and after.
- **[Reducing predeclare passes breaks forward aliases]** -> Keep the full
  refresh on the final outer pass, add alias-heavy fixtures, and require
  byte-identical generated Go.
- **[Cache sharing introduces races or mutation]** -> Treat cached batches as
  immutable; never mutate a shared `BatchBindings`, its map, entries, or free
  variables after publication.
- **[Benchmarks hide cold cost behind warm caches]** -> Report cold and warm
  import-cache runs separately and record CLI stage timing as the integration
  baseline.
- **[Optimization changes generated output]** -> Gate each step on a clean
  regeneration diff and a second-sync fixed point before moving to the next
  step.
- **[Cached Env free variables diverge from the old chain walk]** -> Route all
  constructors through one helper, compare `envFreeVars` against the old
  recursive computation on representative environments, and keep the existing
  shadowing fixtures in the differential run.

## Migration Plan

1. Add the MyGO benchmark suite under `internal/mygo/typeinference2` and record
   cold/warm time, bytes, and allocations for every workload.
2. Introduce the per-source import overlay and switch the four existing import
   consumers to it; verify generated output and environment order are
   unchanged.
3. Add cached `BatchBindings` for MyGO imports and a signature-keyed Go package
   batch cache; clear both from the editor reset path.
4. Replace the primitive-pair allocation path with the zero-allocation lookup.
5. Reduce the first predeclare pass to one refresh and run alias-heavy
   differential fixtures.
6. Replace typed-declaration prepending with index-filled assembly and pass the
   precomputed symbol index through `InferenceSetup`.
7. Store free-variable unions on `Env` layers and switch generalization to the
   cached value; verify long local-binding sequences become linear.
8. Regenerate all affected `.mygo` outputs, rebuild the CLI, sync twice, and
   run the focused and full validation suites before recording final stage
   timings against the baseline.

Rollback is a revert of the individual step followed by regeneration; no
on-disk format or public API changes are introduced.

## Open Questions

- None for the implementation approach. The final scope of the second
  predeclaration pass depends on benchmark and differential evidence gathered
  during the apply phase.

