## 1. Primitive spelling consolidation (A1 / D1)

- [ ] 1.1 Reduce `codegen2/types_util.mygo::canonicalMyGoTypeName` to
  `typeinference2.PrimitiveMyGoSpelling(name).UnwrapOr(any→Any fallback)` and
  delete the local switch; verify `goPrimitiveType`-style golden tests in
  codegen2 still pass.
- [ ] 1.2 Add a regression test asserting `byte`/`rune`/`error` and `any`
  canonicalize through `canonicalMyGoTypeName`; verify with
  `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/codegen2`.

## 2. FFI signature lookup merge + naming (A3/B1 / D3)

- [ ] 2.1 Factor one `ffiSignatureForCallee` + `ffiSignatureInPackages` +
  `ffiSignatureInFuncs` walker in `codegen2/translate_ast.mygo` parameterized
  by a results predicate, folding in `ffiMultiResultSignatureFromCallee` and
  the parallel `ffiRawTupleResultType*` scan; verify existing FFI codegen
  tests still pass.
- [ ] 2.2 Rebuild `ffiOptionSignature`/`ffiResultSignature`/
  `ffiMultiResultSignature` as thin accessors and rename the option walker to
  `ffiOptionSignature*`; add the D6 suffix-policy comment at the walker; grep
  confirms no `ffiSignature*` option-family names or `...FromCallee` suffixes
  remain and `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/codegen2`
  passes.

## 3. Package-aware Go type resolution (A2 / D2)

- [ ] 3.1 In `typeinference2/types.mygo`, unify `GoTypeName`/
  `GoTypeNameWithPackage` behind one resolver taking `Option[GoPackageEntry]`;
  keep both exported entry points (renamed per D6) and verify typeinference2
  tests (incl. `error_alias_test`) pass.
- [ ] 3.2 Collapse `goSignatureTypes*`/`goSignatureTypesWithPackage*`,
  `GoSignatureType`/`GoSignatureTypeWithPackage`, and
  `goVariadicParamTypes`/`goVariadicParamTypesWithPackage` onto the same
  parameterization with thin wrappers; add the D6 suffix-policy comment at
  the resolver; verify with `GOCACHE=/tmp/mygo-go-build go test
  ./internal/mygo/typeinference2 ./internal/mygo/compiler`.

## 4. Shared utilities package common2 (A5 / D5)

- [ ] 4.1 Inventory exact cross-package helper duplicates (`sliceDrop`,
  `joinStrings`, `containsString`, plus any identical siblings) across
  typeinference2/codegen2; list them for migration.
- [ ] 4.2 Scaffold `internal/mygo/common2` as a self-hosted MyGO package and
  add it to the bootstrap sync directory set; verify `go run ./cmd/mygo
  --bootstrap sync internal/mygo/common2` regenerates its `zz_*.gen.go`.
- [ ] 4.3 Move the duplicated helpers into `common2`, update `typeinference2`
  and `codegen2` to import `common2.<fn>` and delete local copies; verify
  `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/typeinference2
  ./internal/mygo/codegen2` passes and `grep` shows no duplicate definitions.

## 5. Type-rendering skeleton (A4 / D4)

- [ ] 5.1 Unify `substituteTypeParamsInTypeExpr`/`substituteTypeParamsMyGO`
  and `funcTypeSubstitutedResult`/`funcTypeSubstitutedResultMyGO` in
  `codegen2/decls.mygo` onto one recursion parameterized by render style;
  verify codegen2 declaration tests pass.
- [ ] 5.2 Merge the `monoTypeToGoStrIn` /
  `monoTypeToGoStrWithParamsIn` layers in `codegen2/types.mygo` into one
  rendering recursion with optional params/aliases while keeping the public
  entry points; verify `GOCACHE=/tmp/mygo-go-build go test
  ./internal/mygo/codegen2 ./internal/mygo/compiler`.

## 6. Bootstrap regeneration and validation

- [ ] 6.1 Regenerate edited self-hosted packages with
  `go run ./cmd/mygo --bootstrap sync internal/mygo/typeinference2
  internal/mygo/codegen2 internal/mygo/common2` and confirm only expected
  `zz_*.gen.go`/`zz_*.gen_test.go` files changed.
- [ ] 6.2 Run the full focused suite
  `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/typeinference2
  ./internal/mygo/codegen2 ./internal/mygo/compiler` and confirm green.
- [ ] 6.3 Run OpenSpec validation:
  `openspec validate consolidate-compiler2-duplicates --strict` and resolve
  any metadata/format errors.
