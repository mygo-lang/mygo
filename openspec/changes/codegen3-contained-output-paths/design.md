# Design

## Context

See [proposal.md](proposal.md) for the motivating failure. The existing codegen3 output mapping uses `filepath.Rel(ModuleRoot, packageDir)` and joins that result to the active build root. For a package in the Go module cache, that relative path begins with parent components, so the joined path can resolve back into the dependency checkout. Output paths are also consumed by generation, cleanup, cache publication, manifests, and Go overlays; the mapping must be consistent across all of them.

The active codegen3 build/cache output root remains the root for generated package files and overlay targets. This change does not move generated files into dependency checkouts or introduce a persistent generated directory in the repository.

## Goals / Non-Goals

**Goals:**

- Keep generated package output for both in-module and imported packages under the active output root.
- Give out-of-module packages deterministic directories derived from their MyGO import paths.
- Reject any resolved output path that escapes the root before performing filesystem writes or cleanup.
- Use one path mapping for main package files, external test files, cache manifests, and overlay entries.

**Non-Goals:**

- Change generated Go contents, ABI selection, import resolution, or package compilation order.
- Change the content-addressed cache key or make dependency source trees writable.
- Change the existing repository-clean policy or the location of requested binaries.

## Decisions

### Use import-path mapping for packages outside the module root

Keep the existing module-relative layout for package directories that are contained by `ModuleRoot`. For packages outside it, derive a stable relative output path from the package's canonical MyGO import path, under a dedicated namespace such as `external/<escaped-import-path>`. Encode path components so separators and platform-reserved characters cannot alter the directory structure. This provides stable overlay targets across runs and avoids including machine-specific module-cache paths.

Import paths are available from package declarations and the Go module mapping used during import resolution. If an external package cannot be assigned a canonical import path, codegen3 should stop with a diagnostic instead of falling back to its absolute source path.

### Enforce containment at the output boundary

After mapping, clean and make both the root and candidate absolute, compute their relative path, and reject the candidate if it is absolute, equals `..`, or begins with a parent traversal. Perform this validation before `MkdirAll`, writes, stale-output cleanup, manifest publication, and overlay construction. Keep the validated output directory as the single source of truth passed through those operations so a later stage cannot reconstruct an unsafe target.

This check is a compiler invariant, not a best-effort fallback. A failed check returns a diagnostic identifying the package and intended output root, and must not touch the candidate path.

### Preserve package-relative source provenance

Output directory naming is separate from generated source line directives. Continue deriving line directives from the source provenance already carried by the package inputs; do not expose absolute module-cache paths in generated text merely because the output is keyed by import path.

### Keep cache identity and output location separate

The content-addressed cache key continues to describe compiler and source inputs. The import-path directory is a deterministic output namespace used to stage files and connect overlays; it must not add the absolute checkout path to cache identity. Verify that cache restore and optional emit paths use the same validated mapping or independently enforce their own destination-root containment.

## Risks / Trade-offs

- [Two package sources may claim the same import path in one build] → Treat this as an import/package-resolution error or ensure the existing dependency graph selects one package per import path; do not silently overwrite another package's staged output.
- [Different import paths can collide after filesystem-safe encoding] → Use an injective escaping scheme for path components, or append a stable digest of the original import path.
- [A path may be lexically contained but redirected through a symlink] → Validate against the canonical output root and ensure generated subdirectories cannot redirect writes outside it; avoid trusting source-controlled symlinks as output directories.
- [Output mapping changes can leave old staged files] → Keep stale-output cleanup scoped to the newly mapped directory and let overlay manifests refer only to current generated files.

## Migration Plan

No user migration is required. Existing cache entries remain valid as generated source content and cache keys do not change; a build using the new compiler stages files under the new contained mapping and rebuilds or restores them through the normal cache path. Rollback consists of using the prior compiler; it does not require modifying dependency sources.
