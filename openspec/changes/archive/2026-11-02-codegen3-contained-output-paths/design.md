# Design

## Context

See [proposal.md](proposal.md) for the motivating failure. The existing codegen3 output mapping uses `filepath.Rel(ModuleRoot, packageDir)` and joins that result to the active build root. For a package in the Go module cache, that relative path begins with parent components, so the joined path can resolve back into the dependency checkout. Output paths are also consumed by generation, cleanup, cache publication, manifests, and Go overlays; the mapping must be consistent across all of them.

The active codegen3 build/cache output root is a per-build temporary directory inside `MYGO_CACHE` (or the system temporary directory when caching is disabled). All staged modules, generated files, temporary module files, overlays, and manifests for that build live there. Go refuses overlay replacements beneath `GOMODCACHE`, so external MyGO modules need a staged module copy and a temporary module file that redirects module resolution to that copy. The project's `go.mod` and `go.sum` remain the source of module requirements and are never edited.

## Goals / Non-Goals

**Goals:**

- Keep generated package output for both in-module and imported packages under the active output root.
- Give out-of-module packages deterministic directories derived from their MyGO import paths.
- Reject any resolved output path that escapes the root before performing filesystem writes or cleanup.
- Build external MyGO packages without using `GOMODCACHE` paths as Go overlay targets.
- Keep the source module's Go files and package layout available to the Go toolchain in the staged copy.
- Keep one path mapping for main package files, external test files, cache manifests, and overlay entries.

**Non-Goals:**

- Change generated Go contents, ABI selection, import resolution, or package compilation order.
- Change the content-addressed cache key or make dependency source trees writable.
- Change the existing repository-clean policy or the location of requested binaries.
- Persist staged module copies or temporary module files after the build finishes.

## Decisions

### Stage external modules by canonical module path

Keep the existing module-relative layout for package directories contained by `ModuleRoot`. For an external package, identify its enclosing module root and module path, then stage that module under `BuildRoot/external/<escaped-module-path>`. Package directories inside the staged module preserve their module-relative layout, so the final package path is deterministic from its canonical import path. Encode module-path components injectively so separators and platform-reserved characters cannot alter the directory structure. This avoids machine-specific module-cache paths and keeps generated output in the active build root.

Copy the external module tree into the staging root once per module. Preserve ordinary Go files and package directories required by the dependency graph. Do not follow symlinks outside the source module; fail with a diagnostic if the staged tree cannot be kept within the output root. If the source directory has no canonical module path, stop with a diagnostic rather than falling back to its absolute filesystem path.

### Redirect Go module resolution with a temporary module file

Copy the importing project's `go.mod` to a per-build `.mod` file under `BuildRoot`, add local `replace` directives from each external module path to its staged module root, and run `go list`, `go build`, and `go test` with `-modfile`. Keep any generated checksum file alongside that temporary module file. Existing replacements for staged modules must be overridden in the temporary copy only. Never edit the project's `go.mod` or `go.sum`.

External generated Go is written into the corresponding package directory in the staged module. The content-addressed cache still stores only generated outputs; on a cache hit, restore those files into the staged package directory. In-module generated files remain staged separately and are exposed through the Go overlay. The overlay must not contain target paths beneath `GOMODCACHE`.

### Enforce containment at the output boundary

After mapping, clean and make both the root and candidate absolute, compute their relative path, and reject the candidate if it is absolute, equals `..`, or begins with a parent traversal. Also resolve existing ancestors and reject symlink paths that escape the output root. Perform this validation before copying module files, `MkdirAll`, writes, stale-output cleanup, cache publication, manifest publication, and overlay construction. Keep the validated output directory as the single source of truth passed through those operations so a later stage cannot reconstruct an unsafe target.

This check is a compiler invariant, not a best-effort fallback. A failed check returns a diagnostic identifying the package and intended output root, and must not touch the candidate path.

### Keep source provenance and overlay targets separate

Output directory naming is separate from generated source line directives. Continue deriving line directives from source provenance already carried by package inputs; do not expose absolute module-cache paths in generated text merely because the output is keyed by import path. Overlay keys for in-module packages refer to their expected generated file paths. External package code is compiled from the staged module and does not use an overlay key in the original dependency directory.

### Keep cache identity and output location separate

The content-addressed cache key continues to describe compiler and source inputs. Import-path directories are a deterministic staging namespace and must not add absolute checkout paths to cache identity. Verify that cache restore and optional emit paths use the same validated mapping or independently enforce their own destination-root containment. Recreate staged modules and the temporary module file for each build; they are disposable build artifacts, not shared cache entries.

## Risks / Trade-offs

- [Two package sources may claim the same import path in one build] → Treat this as an import/package-resolution error or ensure the existing dependency graph selects one package per import path; do not silently overwrite another package's staged output.
- [Different import paths can collide after filesystem-safe encoding] → Use an injective escaping scheme for path components, or append a stable digest of the original import path.
- [Copying a full external module can add build time and temporary disk use] → Stage each module once per build and preserve content-addressed reuse of generated files.
- [Temporary module replacements may conflict with existing replacements] → Apply replacement directives only to the per-build module file and verify the effective replacement with `go list -m` before building.
- [A source module may contain symlinks or unusual filesystem entries] → Reject entries that resolve outside the source module or cannot be copied safely, with a diagnostic before generation proceeds.
- [A path may be lexically contained but redirected through a symlink] → Validate against the canonical output root and ensure generated subdirectories cannot redirect writes outside it; avoid trusting source-controlled symlinks as output directories.
- [A temporary module file could affect package selection] → Copy the project's complete module file and alter only `replace` directives for staged MyGO modules; leave requirements and versions unchanged.
- [Output mapping changes can leave old staged files] → Use a fresh per-build staging root and let overlay manifests refer only to current generated files.

## Migration Plan

No user migration is required. Existing cache entries remain valid as generated source content and cache keys do not change; each build creates fresh staged module copies and a temporary module file, then restores generated files through the normal cache path. Rollback consists of using the prior compiler; it does not require modifying dependency sources.
