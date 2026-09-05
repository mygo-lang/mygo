## 1. Inference boundary type

- [ ] 1.1 In `internal/mygo/typeinference2/types.mygo`, extend the single-result
      branch of `goSignatureResultShape` so a lone result that canonicalizes to
      the Go error type (`TCon("Error")`/`TCon("error")`) returns
      `tCon("Result", [TUnit, <that element>])` instead of the raw element;
      verify `go test ./internal/mygo/typeinference2/` still passes
- [ ] 1.2 Add a `typeinference2` unit test: a `GoFuncSignature` with
      `Results: ["error"]` resolved via `GoSignatureTypeWithPackage` yields
      `Result[(), Error]`; verify `go test ./internal/mygo/typeinference2/ -run
      SingleError` passes

## 2. Codegen2 lowering

- [ ] 2.1 In `internal/mygo/codegen2/translate_ast.mygo`, widen
      `ffiResultPredicate` to treat a signature as error-bearing when its last
      result string is `error` at any arity (>= 1), preserving the existing
      two-result match; verify existing `(T, error)` codegen tests still pass
- [ ] 2.2 In `translateFFIResultCall`, use the resolved `GoFuncSignature` to
      branch: for `Results.Len() == 1`, emit `err := call()` and build the `Ok`
      arm with a synthesized `struct{}{}` payload (`goast.CompositeLit` of an
      empty struct type) instead of a value temp, keeping the two-result path
      unchanged; verify a codegen e2e test asserts the emitted
      `Ok[struct{}, error](struct{}{})` literal
- [ ] 2.3 Confirm a lone-`error` FFI call in statement position lowers to the
      raw call (via the widened predicate reaching `translateFFIDiscardStmt`)
      and is not emitted as an unused `Result`; verify via the e2e test in 3.1

## 3. End-to-end and docs

- [ ] 3.1 Add a codegen2 compile-and-lower test in `internal/mygo/codegen2/`
      whose `.mygo` calls a Go function/method returning only `error` (e.g. an
      `os.Chdir`/`Flush`-style call); assert compilation succeeds and the
      generated Go branches on the error to produce `Ok`/`Err`; verify
      `go test ./internal/mygo/codegen2/` passes
- [ ] 3.2 Run the full suite `go test ./internal/mygo/...` and verify nothing
      regresses from the widened predicate / new Result shape
- [ ] 3.3 Update `docs/compiler/ffi.md` "Automatic `Result` wrapping" section to
      document that a lone trailing `error` result lowers to `Result[(), error]`
      (`Result[struct{}, error]` in generated Go); verify the doc renders

## 4. Optional seed-codegen parity

- [ ] 4.1 (Optional) If parity with the legacy seed compiler is required, widen
      `goSigErrorResultType`/`wrapGoErrorResultCall` in
      `internal/mygo/codegen/translate_call.go` for `len(results) == 1` with a
      lone `error`, emitting a `struct{}{}` payload; verify seed codegen tests
      pass. Skip if seed parity is not needed
