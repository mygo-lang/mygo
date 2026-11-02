# Proposal

## Why

When codegen3 compiles an imported MyGO package from the Go module cache, the package path can be outside the current module root. The current relative-path mapping can then escape the intended output root and attempt to write generated files into the dependency checkout, which may be read-only and must remain untouched.

## What Changes

- Keep every codegen3 generated output under the active project build/cache output root, including outputs for dependencies resolved from the Go module cache or local replacements.
- Map packages outside the module root to deterministic, import-path-based output directories.
- Validate the resolved output directory is contained by the configured output root before creating directories or writing files; fail with a clear diagnostic if it is not.
- Apply the same mapping and containment rule to external test package outputs and overlay targets.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `bootstrap-codegen3`: Generated output for imported packages must remain contained in the active output root, including when the package source is outside the main module.

## Impact

Affected code includes codegen3 output path selection, package output writing, external test generation, cache/manifest publication, and overlay construction in `internal/mygo/compiler`. Generated Go content and package import behavior remain unchanged; only the output location mapping and validation change.
