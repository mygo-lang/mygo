## 1. FFI Loader Extension (Go side)

- [x] 1.1 In `bootstrapGoPackageInfoFromTypes` in
  `internal/mygo/compiler/go_ffi_import.go`, extract the exported `*types.Struct`
  fields `{Name, Type}` and `Underlying` (underlying type string) for every
  exported named type, and update both `GoTypeSignature{...}` construction
  sites; verify `go test ./internal/mygo/compiler/...` passes and add a unit
  test asserting `go:net/http`'s Request has a `Header` field and `go:context`'s
  CancelFunc has underlying `func()`
- [x] 1.2 Add the `GoFieldSignature` struct in `typeinference2/types.mygo` and
  add `Fields` and `Underlying` fields to `GoTypeSignature`; verify
  `go test ./internal/mygo/typeinference2/...` passes (existing keyed literal
  constructions are unaffected)

## 2. Register Go FFI struct field symbols

- [x] 2.1 Next to `goSymbolsFromTypes`, add field symbol registration
  (`Symbol.StructField(typeName, fieldName, mono)`), resolving the field type
  string with `goTypeNameWith(fieldType, Some(pkg), receiverTypeParams)` and
  skipping fields that fail to resolve; verify a new inference test: `r.Header`
  with `r: Ref[http.Request]` type-checks and resolves to `http.Header`
- [x] 2.2 Chained method call end-to-end: `r.Header.Set("X-Test", "1")`
  type-checks and `GenerateFiles` emits valid Go containing a direct selector
  (`v.F0.Header.Set(...)`); verify the new codegen2
  `TestGenerateFilesChainedFFIMethodCall` passes

## 3. Package-aware raw multi-value returns

- [x] 3.1 Add `GoSignatureRawResultTypeWithPackage(sig, pkg)` in
  `typeinference2/types.mygo` (reusing `goSignatureTypesWithPackage`), keeping
  the old `GoSignatureRawResultType` as a thin wrapper; add a
  `GoSignatureForCallee` variant returning `(sig, pkg)`; verify a unit test:
  the WithTimeout signature + context package → `TTuple([Context,
  TFunc([],TUnit)])` and no longer reports `unresolved Go type name: CancelFunc`
- [x] 3.2 Switch `ffiRawTupleResultType` in `infer.mygo` and
  `translateFFITupleLetStmt` in `codegen2/translate_ast.mygo` to the
  package-aware path; verify the new codegen2 test
  `let (ctx, cancel) = context.WithTimeout(...)` emits
  `ctx, cancel := context.WithTimeout(...)`

## 4. Go named function types are callable

- [x] 4.1 In `goPackageTypeForNameWith`, special-case package-local named types
  whose underlying starts with `func(`/`chan` as `TFunc`/Chan resolution (all
  others keep the nominal `TQualifiedName`); verify a unit test that
  `GoTypeNameWithPackage("CancelFunc", contextPkg)` equals
  `TFunc([], TUnit)`
- [x] 4.2 Switch the GoMethod instantiation site in `infer.mygo` to
  package-aware signature resolution (locating the package via the method's
  owning type in `state.GoPackages`); verify a scenario where a method result
  names a package-local type passes inference
- [x] 4.3 Function-value call end-to-end: `cancel()` type-checks and emits
  `cancel()`; verify the new codegen2 `TestGenerateFilesCallsGoFuncTypeValue`
  passes

## 5. Regression protection (pin already-working behavior)

- [x] 5.1 Qualified struct literal tests: `http.Client { }` and
  `http.Client { Timeout: time.Second }` both emit `http.Client{...}`; verify
  the new codegen2 tests pass
- [x] 5.2 Pin the direct Go method call (`h.Set(...)` with receiver
  `Ref[http.Header]`) as a test; verify the new test passes

## 6. Bootstrap regeneration and full verification

- [x] 6.1 Regenerate `internal/mygo/{typeinference2,codegen2,compiler}/zz_*.gen.go`
  with the current bootstrap compiler and confirm `git diff` contains only the
  expected changes from this change
- [x] 6.2 `go test ./internal/mygo/...` is fully green and
  `go run ./cmd/mygo --bootstrap build examples/main` bootstraps successfully
- [x] 6.3 Gateway-style acceptance: add a `compile + go build` snippet using
  `http`/`context` FFI to the bootstrap tests (may live in a temporary module)
  and confirm the generated Go for all four scenarios compiles
