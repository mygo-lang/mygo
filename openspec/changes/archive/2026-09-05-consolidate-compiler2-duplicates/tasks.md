## 1. Primitive spelling consolidation (A1 / D1)

- [x] 1.1 Reduce `codegen2/types_util.mygo::canonicalMyGoTypeName` to
  `typeinference2.PrimitiveMyGoSpelling(name).UnwrapOr(name)`; add the
  `any`↔`Any` pair to `typeinference2`'s `GoPrimitivePair`, and drop
  `goPrimitiveType`'s now-redundant local `Any` entry (keep `Unit` local);
  verify `goPrimitiveType`-style golden tests in codegen2 still pass.
- [x] 1.2 Add a regression test asserting `byte`/`rune`/`error` and `any`
  canonicalize through `canonicalMyGoTypeName`; verify with
  `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/codegen2`.

## 2. FFI signature lookup merge + naming (A3/B1 / D3)

- [x] 2.1 Factor one predicate-parameterized walker
  (`GoSignatureForCallee` + `GoSignatureInPackages` + `GoSignatureInFuncs`),
  folding in `ffiMultiResultSignatureFromCallee` and the parallel
  `ffiRawTupleResultType*` scan. The walker lives in
  `typeinference2/infer.mygo` — the only cycle-free home shared by codegen2's
  boundary lowering and this package's raw-tuple scan (codegen2 imports
  typeinference2); `codegen2/translate_ast.mygo` keeps the thin accessors;
  verify existing FFI codegen/inference tests still pass.
- [x] 2.2 Rebuild `ffiOptionSignature`/`ffiResultSignature`/
  `ffiMultiResultSignature` as thin accessors over the shared walker with
  result-shape predicates; add the D6 suffix-policy comment at the walker;
  grep confirms no `ffiSignature*` option-family names or `...FromCallee`
  identifiers remain and
  `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/codegen2
  ./internal/mygo/typeinference2 ./internal/mygo/compiler` passes.

## 3. Package-aware Go type resolution (A2 / D2)

- [x] 3.1 In `typeinference2/types.mygo`, unify `GoTypeName`/
  `GoTypeNameWithPackage` behind one resolver taking `Option[GoPackageEntry]`
  and returning `Result[ast2.MonoType, String]` — a bare leaf resolves first
  against the declaration's generic type-parameter map (D2 extension), then
  to a nominal qualified name when dotted, and is an `Err` only when no
  parameter matches and no kind/package resolves; malformed `map[` is an
  `Err`. Keep both exported entry points (renamed per D6) and update
  `error_alias_test.mygo` to the `Result` shape; verify typeinference2 tests
  pass.
- [x] 3.2 Collapse `goSignatureTypes*`/`goSignatureTypesWithPackage*`,
  `GoSignatureType`/`GoSignatureTypeWithPackage`, and
  `goVariadicParamTypes`/`goVariadicParamTypesWithPackage` onto the same
  parameterization with `Result` propagation through the signature builders
  and the env-seeding chain (`seedGoPackageEnv`/`seedMyGoPackageEnv`/
  `cachedImportedPackage`/`buildImportedPackageCacheEntry`/
  `importedPackageEnv`/`prepareInferenceSetup`), all five `Infer*` entry
  points surfacing the error; add the D6 suffix-policy comment at the
  resolver; verify with `GOCACHE=/tmp/mygo-go-build go test
  ./internal/mygo/typeinference2 ./internal/mygo/compiler`.
- [x] 3.3 Resolve generic FFI type parameters to their declaration TParams:
  add `GoFuncSignature.TypeParams: Slice[String]`, thread the list from
  `go_ffi_import.go` (`goFuncTypeParams`, incl. generic receiver-type
  parameters for methods), bind it in `seedGoPackageFuncs` + the `GoMethod`
  selector path, and route every resolver recursion through `goTypeParamMap`
  so bare `Map` (the maps package's type parameter) and `map[K]V` results
  resolve — with regression tests in `error_alias_test.mygo`
  (`TestGoSignatureTypeResolvesGenericParams`) and a green `--bootstrap sync`
  over `go:maps` users. (Amended: the `bootstrap.mygo` string-signature path
  was removed as dead code; MyGO deps use `MyGoPackageInfo` decl inference.
  The shared resolver additionally handles composite map keys via bracket
  matching and maps `()` → `TUnit`.)

## 4. Shared utilities package common2 (A5 / D5)

- [x] 4.1 Inventory exact cross-package helper duplicates across
  typeinference2/codegen2. Migrated to `common2`: `sliceDrop`, `joinStrings`,
  `containsString`, `errorAtExpr`, `withExpressionSourceName`. Resolved
  separately (typed by compiler2 types, so they cannot live in the leaf
  `common2` package): `isGoPackageAlias` — codegen2's copy had no callers and
  was deleted, typeinference2's single copy retained; `ffiMultiResultPredicate`
  — codegen2 now passes an inline lambda to the shared walker, leaving one
  definition in typeinference2. `defaultImplMethod` is *not* a duplicate (the
  two defaults differ).
- [x] 4.2 Scaffold `internal/mygo/common2` as a self-hosted MyGO package
  (`common2.mygo` → `zz_common2.gen.go`, exported `SliceDrop`/`JoinStrings`/
  `ContainsString`/`ErrorAtExpr`/`WithExpressionSourceName`); it is picked up
  by the bootstrap sync dir walk, and `--bootstrap sync internal/mygo`
  regenerates its `zz_*.gen.go`.
- [x] 4.3 Move the duplicated helpers into `common2`, update `typeinference2`
  and `codegen2` to import `common2.<fn>` (268 call sites) and delete local
  copies; `codegen2_test.go`'s `TestSliceDropReturnsSuffixView` now exercises
  `common2.SliceDrop`; verify
  `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/typeinference2
  ./internal/mygo/codegen2 ./internal/mygo/compiler ./internal/mygo/common2`
  passes and `grep` shows no duplicate definitions.

## 5. Type-rendering skeleton (A4 / D4)

- [x] 5.1 Unify `substituteTypeParamsInTypeExpr`/`substituteTypeParamsMyGO`
  and `funcTypeSubstitutedResult`/`funcTypeSubstitutedResultMyGO` in
  `codegen2/decls.mygo` onto one recursion
  (`substituteTypeParamsByStyle` / `funcTypeSubstitutedResultByStyle`)
  parameterized by a `mygo: Bool` render-style flag; the four public entry
  points are thin wrappers (MyGO wrappers pass `Set.New[String]()` for the
  unused Go type-param set); verify codegen2/compiler tests pass.
- [x] 5.2 Merge the `monoTypeToGoStrIn` / `monoTypeToGoStrWithParamsIn`
  layers in `codegen2/types.mygo` into one rendering recursion
  (`monoTypeToGoStrWithParamsIn(t, params, aliases)` handling every shape
  directly) while keeping the public entry points as thin wrappers
  (`monoTypeToGoStrIn(t, aliases)` delegates with empty `params`);
  verify `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/codegen2
  ./internal/mygo/compiler`.

## 6. Bootstrap regeneration and validation

- [x] 6.1 Regenerate edited self-hosted packages with
  `cmd/mygo --bootstrap sync internal/mygo` (single-root sync covers the full
  compiler2 tree, including the new `common2` package discovered by the dir
  walk) and confirm only expected `zz_*.gen.go`/`zz_*.gen_test.go` files
  changed — 24 files regenerated, repeat runs are stable/idempotent.
- [x] 6.2 Run the full focused suite
  `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/typeinference2
  ./internal/mygo/codegen2 ./internal/mygo/compiler` and confirm green.
- [x] 6.3 Run OpenSpec validation:
  `openspec validate consolidate-compiler2-duplicates --strict` passes
  ("Change is valid").
