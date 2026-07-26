# Fix codegen2 typeclass dispatch — progress report

## Problem

`prelude/zz_prelude.gen.go` compilation error:

```
prelude/zz_prelude.gen.go:108:31: cannot use __mygo_match___mygo_expr_2.F0
  (variable of type A constrained by any) as E value in argument to EqualsFn_1
```

**Root cause**: In `MygoIT2EqFN8ResultEqGN1AN1EEGN6ResultGN1AN1EEEM6Equals`, `l.Equals(r)` should dispatch to `EqualsFn` (when `l: A`) or `EqualsFn_1` (when `l: E`), but the old codegen2 always fell back to the **last** registered function (`EqualsFn_1`) regardless of the receiver type.

## Changes Made

### 1. MonoType-based constraint arg storage

**`internal/mygo/typeinference2/types.mygo`**:
- Added `MethodConstraintKey` struct with `Receiver: String` and `Method: String` fields
- Added `ResolvedConstraintArgs: Map[MethodConstraintKey, Slice[ast2.MonoType]]` to `PackageInfo`
- Added `ResolvedConstraintArgs: Map[MethodConstraintKey, Slice[ast2.MonoType]]` to `InferState`

**`internal/mygo/typeinference2/infer.mygo`**:
- `inferImplMethods`: calls `storeResolvedConstraintArgs` after `inferExpr` to extract resolved predicate args (TVars) and store them in `state.ResolvedConstraintArgs`
- `storeResolvedConstraintArgs`: extracts `eir.Result.Predicates[i].Args[0]` for each predicate and stores as a list keyed by `MethodConstraintKey{Receiver: receiverName, Method: sig.Name}`
- `inferDecls` base case: changed from `{}` to `state.ResolvedConstraintArgs` to preserve accumulated args

### 2. Codegen2 constraint lookup

**`internal/mygo/codegen2/types.mygo`**:
- Changed `patternTypes` from `Map[String, String]` to `Map[String, ast2.MonoType]`
- Added `constraintFuncArgMonoTypes: Map[String, Slice[ast2.MonoType]]` to `egCtx`
- Added `variantFieldMonoTypes: Map[String, Slice[ast2.MonoType]]` (variant name → field MonoTypes)
- Changed `pkgInfo` from `Option[PkgInfo]` to `Option[PackageInfo]` in both `egCtx` and `Generator2`
- `newGenerator2` now stores `Some(pkgInfo)` (the full PackageInfo)

**`internal/mygo/codegen2/codegen2.mygo`**:
- `generateOneFile` passes full `info` to `newGenerator2`

**`internal/mygo/codegen2/decls.mygo`**:
- `addConstraintParams` / `addDictionaryMethods`: added `receiverName` and `fnTypeParams` parameters
- For each constraint, calls `resolvedConstraintArgFor` to look up resolved TVar from `PackageInfo.ResolvedConstraintArgs` (keyed by `MethodConstraintKey{Receiver, Method}`)
- `seedEnumVariantFieldTypes`: stores field type as MonoType using `TypeFromASTWithParams`
- `seedEnumVariantsForVariants`: passes `enumTypeParams` through

**`internal/mygo/codegen2/translate_ast.mygo`**:
- `extractVariantFieldType`: accepts `variantName` and `ctx`, looks up MonoType from `variantFieldMonoTypes`
- `receiverStaticType`: returns `Option[ast2.MonoType]`, falls back to `resolveExprMonoType`
- `dictFuncForMethod`: takes `Option[ast2.MonoType]`, uses `matchDictFuncByMono` with `monoTypeEquals`
- `monoTypeEquals`: compares MonoTypes structurally (TVar↔TVar, TParam↔TParam, TCon↔TCon, etc.)
- `matchDictFuncByMono`: iterates over constraint MonoTypes, compares with receiver using `monoTypeEquals`
- `matchingReceiverCandidate`: receives `Option[ast2.MonoType]`, converts to string for legacy matching

## Current State

The generated prelude STILL has `.Equals()` calls instead of `EqualsFn()` / `EqualsFn_1()`:

```go
__mygo_expr_3 = __mygo_match___mygo_expr_2.F0.Equals(...)
```

This means `dictFuncForMethod` returns `None`.

### Remaining issue

Two possible causes:

1. **`receiverStaticType` returns `None`**: `resolveExprMonoType(IdentExpr("l"), ctx)` should return `Some(TVar(aid))` from `expr.Type` (inference annotation), but the annotated expressions might not be reaching codegen2.

2. **`ResolvedConstraintArgs` is empty or key mismatch**: Even if receiver type IS resolved, the constraint MonoTypes would be `TUnit` (fallback from `resolvedConstraintArgFor`), causing `matchDictFuncByMono` to fail.

The debug print (`fmt.Println`) in `receiverStaticType` didn't appear in bootstrap output, suggesting either:
- `receiverStaticType` never reaches the `name == "l"` branch (patternTypes/locals have it)
- The debug output was lost in the massive bootstrap log

### Bootstrap propagation chain

```
inferDecls → InferPackageWithGoPackages → GenerateFiles → generateOneFile
                                                              ↓
                                                    newGenerator2(info)
                                                              ↓
                                              g.pkgInfo = Some(info)
                                                              ↓
                                    translateImplAstDecl → ctx.pkgInfo = g.pkgInfo
                                                              ↓
                                    translateImplAstMethods → ctx.pkgInfo = baseCtx.pkgInfo
                                                              ↓
                                       addConstraintParams → resolvedConstraintArgFor
```

The `PackageInfo.ResolvedConstraintArgs` map reference is shared through all copies (maps are reference types in Go).

### Files changed

- `internal/mygo/typeinference2/infer.mygo`
- `internal/mygo/typeinference2/types.mygo`
- `internal/mygo/codegen2/codegen2.mygo`
- `internal/mygo/codegen2/decls.mygo`
- `internal/mygo/codegen2/translate_ast.mygo`
- `internal/mygo/codegen2/types.mygo`
