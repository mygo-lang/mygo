# compiler3 documentation

This directory is the implementation and acceptance index for the IR3/codegen3
pipeline.

| Spec area | Contract document |
| --- | --- |
| Package IR units, source provenance, call kinds, effects, verification, determinism | [ir.md](ir.md) |
| Canonical `Option`/`Result`/List/enum/recursion/generic layouts | [representation.md](representation.md) |
| `Match3`, `Simplify3`, `Tailcall3`, `Unbox3`, `Closure3`, `Inline3`, `Specialize3` | [optimizer.md](optimizer.md) |
| Per-file Go emission, imports, line directives, joins, matches, tail calls, FFI, output boundary | [codegen.md](codegen.md) |
| Shared `MYGO_CACHE`, keying, overlays, locking, cleaning, emit path | [cache.md](cache.md) |
| Inline-Go and FFI boundary audit | [inline-go-audit.md](inline-go-audit.md) |
| Phase 1 codegen2 baseline | [baseline.md](baseline.md) |
| Measured allocation, compile-time, generated-compiler, and stack-depth results | [performance.md](performance.md) |

## Spec requirement coverage

### `bootstrap-ir3-pipeline`

- package-level provenance and imported signatures: `ir.md`, sections “Unit and
  ownership” and “Joins, matches, and provenance”;
- explicit call kinds, joins, match failure, and tail markers: `ir.md`,
  “Expressions and ANF” and “Calls and tail positions”;
- effects/coeffects and optimization safety: `ir.md` and `optimizer.md`;
- observable behavior and panic/mutation ordering: `ir.md` and `optimizer.md`;
- deterministic construction/output: `ir.md`, “Determinism”;
- verification and non-optimizing O0 path: `ir.md`, “Verification” and “Pipeline
  boundary”.

### `bootstrap-value-representation`

- compiler-versioned canonical layouts and mixed-version rejection:
  `representation.md`, “Versioning”;
- `Option`, `Result`, List, enum recursion, generics, HKT: `representation.md`,
  “Matrix” and “Recursion rule”;
- FFI, dynamic typeclass, and inline-Go boundary adaptations:
  `representation.md`, “Boundaries” and `codegen.md`, “FFI and inline Go”.

### `bootstrap-codegen3`

- behavioral parity, control flow, match trees, and tail calls: `codegen.md`;
- per-file provenance, imports, and line directives: `codegen.md`, “Per-file
  emission” and “Imports”;
- no metadata and source-derived dependencies: `codegen.md`, “Output boundary”;
- test command/cache defaults: `cache.md`;
- cache key, concurrent publication, corruption recovery, disabled/offline
  behavior, cleaning and debug emit: `cache.md`;
- explicit backend selection and rollback: `codegen.md`, “Output boundary” and
  the CLI/backend tests.

Repository-clean builds, test execution, and generated-output rejection are
enforced by the compiler tests; the contract is stated in `codegen.md` and
`cache.md`.
