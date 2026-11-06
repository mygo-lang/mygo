## Why

Generated Go can gain a Go import that the package's own sources never
declared and the file never uses as a package. A concrete regression: lsp
imports codegen2; after codegen2 added "import token go:go/token" in MyGO
source, every generated lsp file started emitting "import go/token" and
failed to compile because token there is a local variable. The pipeline
currently merges dependency Go imports into the consumer's inferred import set
and then validates usage with a textual alias scan, so a local binding with
the same name keeps the unused import alive.

## What Changes

- Separate the Go package universe used for dependency signature resolution
  from the set considered for a package's own emitted imports. Only Go imports
  declared by the package being compiled may seed inferred Go imports.
- Make inferred Go imports usage-aware: a file emits a Go import only when its
  emitted Go resolves an identifier to that package, not when an unrelated
  local variable reuses the alias text.
- Keep explicit source imports, compiler-synthesized prelude imports, and
  inferred foreign-type imports working as they do today.
- Add regression coverage for a dependency Go import that collides with a
  local identifier, using the lsp plus codegen2 shape.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- per-file-import-resolution: tighten per-output-file import selection so
  dependency-declared Go imports do not leak into a consumer package, and so
  import usage detection respects identifier binding instead of matching alias
  text.

## Impact

- internal/mygo/compiler/bootstrap.mygo: Go package collection feeds both
  signature resolution and import emission from one list.
- internal/mygo/codegen2/codegen2.mygo: generateOneFile passes
  info.GoPackages to inferredGoImports, which currently adds every entry.
- Generated-file import filtering (goast.ReferencedImportAliases) matches
  selector text without scope information.
- Regenerated lsp/zz_*.gen.go files must no longer import go/token. No public
  API or language syntax changes.
