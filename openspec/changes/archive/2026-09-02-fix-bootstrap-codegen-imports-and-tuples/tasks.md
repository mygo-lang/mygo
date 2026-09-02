## 1. Tuple-return switch lowering

- [x] 1.1 Update compiler2 tail-position switch lowering so a top-level function declared with a tuple return emits individual Go return values for every switch case; verify with a focused bootstrap fixture that ends a `(State, Slice[Command])` function in a pattern-matching switch and compiles the generated Go.
- [x] 1.2 Preserve existing tuple-as-single-value behavior for function literals while changing top-level returns; verify existing tuple return and tuple binding compiler tests still pass.

## 2. Per-file Prelude import correctness

- [x] 2.1 Make compiler2 Prelude import selection account for Prelude dictionary/helper identifiers introduced by Slice and String method dispatch; verify a bootstrap-generated source using `Append`, `Each`, and `Len` includes the dot import and Go-compiles.
- [x] 2.2 Prevent erased interface-only Prelude references from adding an import to otherwise Prelude-free generated Go; verify a generated interface-only file has no Prelude import and the package Go-compiles without an unused-import error.
- [x] 2.3 Add a multi-file bootstrap regression that combines helper-using and interface-only files, asserts each file's import behavior, and verifies the full generated package compiles.

## 3. Verification

- [x] 3.1 Run the focused compiler/bootstrap regression tests and `go test ./internal/mygo/compiler ./internal/mygo/codegen2 ./internal/mygo/typeinference2` with a writable Go cache; verify all pass.
