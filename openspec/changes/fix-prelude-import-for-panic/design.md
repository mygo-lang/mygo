# Design

## Context

See proposal.md for the triggering failure. Codegen currently decides whether to add the dot import by scanning source identifiers against names assembled from external typed declarations. FFI declarations whose metadata types are normalized to Go built-ins can fall outside that name set, even though the generated call remains unqualified.

## Goals / Non-Goals

**Goals:**
- Make prelude import selection track actual prelude declaration references in a generated file.
- Cover direct helper calls and retain per-file import selection.

**Non-Goals:**
- Change how prelude functions are declared or how their Go names are emitted.
- Change MyGO source syntax, FFI type mapping, or package metadata formats.

## Decisions

- Determine prelude use from resolved declaration ownership (or equivalent stable symbol identity) before lowering erases that ownership, rather than relying solely on the declaration's type spellings. This handles `Panic` even when its argument is represented as Go `string`.
- Keep import selection scoped to each generated file, matching the existing per-file generation model. Only files that reference prelude declarations receive the import.
- Add regression coverage using a package with a `Panic` call and assert the generated file imports the prelude. Also retain a no-reference case to guard against unused imports.

## Risks / Trade-offs

- [Reference tracking misses a lowering path] -> Exercise direct calls and generated helper references in codegen coverage; use resolved symbol identity before lowering where possible.
- [Broader import detection emits an unused dot import] -> Keep detection file-scoped and verify a file with no prelude references remains import-free.
