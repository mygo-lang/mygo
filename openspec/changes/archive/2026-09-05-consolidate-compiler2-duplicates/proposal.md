## Why

The self-hosted compiler2 family (`parser2`, `typeinference2`, `codegen2`,
`compiler`) accumulated several residual duplicate implementations while the
shared `GoPrimitivePair` table and other consolidation landed. The same
mapping is copy-pasted in parallel switches, the same AST/Go-type traversal is
repeated with slightly different parameterization, package-aware
`...WithPackage` variants duplicate their base functions, and a handful of
shared utilities are redefined per package. These copies drift (the local
`canonicalMyGoTypeName` table is missing `byte`/`rune`/`error` and adds a
non-table `any`), and the
compatibility entry points carry inconsistent names (`ffiSignature*` vs the
`ffiOptionSignature*` accessor it backs).

## What Changes

Consolidate the compiler2 family onto shared single-source logic. Output
shape is preserved except for one deliberate boundary change (see A2): Go FFI
signature types that cannot be resolved now fail with an error instead of
silently degenerating to `any`.

- **A1 — one primitive spelling table**: delete codegen2's local
  `canonicalMyGoTypeName` switch and route it through the shared
  `typeinference2.PrimitiveMyGoSpelling` (from `GoPrimitivePair`). Reconcile
  the table so `byte`/`rune`/`error` canonicalize consistently and decide the
  fate of the extra `any` mapping (bring it into the table).
- **A2 — collapse the Go-type `*WithPackage` family**: merge `GoTypeName` /
  `GoTypeNameWithPackage`, `goSignatureTypes*` / `goSignatureTypesWithPackage*`,
  `GoSignatureType` / `GoSignatureTypeWithPackage`, and
  `goVariadicParamTypes` / `goVariadicParamTypesWithPackage` into one
  parameterized resolver (optional package entry) with thin wrappers. The
  resolver returns `Result` and **errors on unresolvable boundary types**
  instead of falling back to `TVar`/`any`; the error propagates through the
  env-seeding chain to the `Infer*` entry points. `any`↔`Any` joins
  `GoPrimitivePair` so a declared `any` still resolves. The resolver also
  carries the declaration's generic type-parameter map (`GoFuncSignature`'s
  new `TypeParams` field, populated from go/types), so bare names like
  `maps.Clone`'s `M` or `maps.All`'s
  `Map` resolve to their bounded `TParam` ids, and Go `map[K]V` maps to
  MyGO's `Map`, instead of failing at the boundary.
- **A3/B1 — one FFI signature lookup**: merge the triplicated
  `ffiOptionSignature` / `ffiResultSignature` / `ffiMultiResultSignature`
  scans plus the parallel `ffiRawTupleResultType*` scan into one walker
  parameterized by a results predicate, and normalize the misnamed
  `ffiSignature*` option family to the `ffiOptionSignature*` convention.
- **B2 — context-suffix naming policy**: establish and document the
  `...WithPackage`/`...WithParams`/`...In`/`...Into` suffix convention as one
  naming axis each, drop input-shape `...FromCallee` variants by folding them
  into the shared walkers, and keep the widely-used `...Into` tail-recursion
  accumulators (an intentional MyGO idiom) unchanged.
- **A4 — a single type-renderer skeleton**: converge the Go-vs-MyGO renderer
  twins (`substituteTypeParamsInTypeExpr`/`substituteTypeParamsMyGO`,
  `funcTypeSubstitutedResult`/`funcTypeSubstitutedResultMyGO`, and the
  `monoTypeToGoStrIn`/`monoTypeToGoStrWithParamsIn` layers) onto one traversal
  skeleton parameterized by output style/context.
- **A5 — shared utilities in `internal/mygo/common2`**: move the cross-package
  helper copies (`sliceDrop`, `joinStrings`, `containsString`, and siblings
  defined identically in more than one compiler2 package) into a new
  `internal/mygo/common2` package and import them from the compiler2 packages.

The large `*UsesName` / `*UsesPreludeName` AST predicate family is noted as a
follow-up and is **Non-Goal** here.

## Capabilities

This is a pure internal refactor of the compiler2 pipeline; no spec-level
language behavior changes. The change declares `skip_specs: true` and creates
no capability delta.

## Impact

- `internal/mygo/common2` (new): shared utility package.
- `internal/mygo/typeinference2`: `types.mygo` Go-type conversion
  consolidation (`GoTypeName`/`*WithPackage` family).
- `internal/mygo/codegen2`: `types_util.mygo` (primitive spelling,
  `canonicalMyGoTypeName`), `types.mygo` (renderers), `translate_ast.mygo`
  (FFI signature lookup + naming normalization), `decls.mygo` (renderer
  twins), `utils.mygo` / `types_util.mygo` (shared utils removed).
- Bootstrap regeneration: modified self-hosted packages need
  `cmd/mygo --bootstrap sync` to regenerate `zz_*.gen.go`.
