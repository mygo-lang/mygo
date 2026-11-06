## Context

The editor loader populates BootstrapState.SourcesCache with overlay-aware parsed inputs before dependency type information is requested. Since commit e95bac3, imported packages can require on-demand inference so inferred package-level bindings are available to an importer. That on-demand path currently loads sources with bootstrapLoadPackageSourcesDiagnostic, a disk-only entry point, so it replaces the editor's overlay view with saved text. See proposal.md for the user-visible failure.

## Goals / Non-Goals

**Goals:**

- Ensure on-demand dependency inference uses the same source snapshot as the importing package.
- Preserve inferred package-level binding metadata in dependent analysis.
- Keep cache scope, LSP responses, and protocol behavior unchanged for unaffected packages.

**Non-Goals:**

- Redesigning workspace indexing or package analysis invalidation.
- Adding new LSP methods, fields, or protocol capabilities.
- Adding navigation into external Go source.
- Changing type inference rules or exported binding semantics.

## Decisions

### Prefer the populated source snapshot for on-demand inference

Change compileUncachedBootstrapDir to load package inputs through bootstrapLoadCachedSourcesDiagnostic(dir, state). That helper returns the editor-populated SourcesCache entry when one exists and otherwise loads the package from disk. Dependency inference therefore sees an open overlay when present, while ordinary compilation continues to use the current disk source.

This keeps the on-demand dependency pass introduced for exported bindings. The pass still infers the dependency, populates TypedSourcesCache, and exposes inferred package-level declarations to the importer. Only the source snapshot feeding that pass changes.

Alternatives considered:

- Skip on-demand inference in editor mode. This loses inferred binding types for exported package-level values and would regress the capability added by e95bac3.
- Thread the overlay map into the bootstrap dependency APIs. This duplicates the source snapshot already held in SourcesCache and expands the API surface across compiler stages.
- Delete or bypass parse caches when an overlay exists. Parse caching is not the source of the stale signature; the dependency inference path still needs the overlay-aware source set.

### Keep typed dependency data scoped to one analysis state

TypedSourcesCache remains per BootstrapState. Editor analysis creates a new state for each package analysis, so cached typed declarations cannot leak into a later document version. The parse cache remains the only cross-request cache and already keys overlays by document version.

### Validate the behavior at both package and protocol levels

The regression tests cover direct dependency analysis, semantic navigation, and a real stdin/stdout LSP session. Exported binding tests guard the inference feature that motivated the on-demand dependency pass.

## Risks / Trade-offs

- Other callers of compileUncachedBootstrapDir will now prefer a populated SourcesCache over a fresh disk read. The state is scoped to one compile or analysis operation; callers that need a forced disk refresh must invalidate the source cache explicitly. Mitigation: run the complete LSP suite and the exported binding compiler tests, then run the broader compiler suite during implementation.
- A long-lived BootstrapState could observe a disk change after its source cache is populated. The LSP creates a new state after fingerprint invalidation, so editor requests use the current overlay or disk snapshot. Mitigation: preserve the existing per-analysis state boundary and document the source-cache invariant.
- Using cached parsed inputs can change diagnostic provenance if editor and bootstrap parsing diverge. Both paths use the same parser and AST construction; the existing editor tests and full LSP suite validate location-sensitive behavior. Mitigation: keep the change limited to source selection.

## Migration Plan

This is a behavioral bug fix with no data or protocol migration. Deploy through the normal compiler and LSP release path. If a regression appears, revert the source-selection change and restore the disk-only dependency load; the regression tests can then distinguish the old stale-overlay behavior from unrelated failures.
