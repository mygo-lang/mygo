## Why

A Go FFI call whose recorded signature returns only a trailing `error`
(for example `os.Chdir(dir)` or `log.Sync()` - `func Foo() error`) currently
falls through every FFI boundary predicate in codegen2 (`ffiResultPredicate`
requires `(T, error)`, `ffiOptionPredicate` requires `(T, bool)`,
`ffiMultiResult` requires more than one result), so it is treated as an
ordinary call and never gets the `Result` wrapping that every other
error-bearing Go call receives. This leaves a gap in the boundary rule:
"trailing `error` to `Result`" should hold uniformly, including when there is
no payload value.

## What Changes

- Widen the codegen2 error-return predicate so a Go function/method whose
  signature is a lone `error` result (`func Foo() error`) is recognized as an
  error-bearing FFI call.
- Lower such a call at the boundary to `Result[(), error]`: `Ok(())` when the
  Go call returns `nil`, `Err(e)` when it returns a non-nil error.
- Because `()` unit renders as `struct{}` in generic position, the generated
  Go uses `Result[struct{}, error]` and synthesizes a `struct{}{}` payload for
  the `Ok` arm (the raw call returns a single value, so no two-result
  `a, b := call()` shape is used).
- Align the seed codegen (`internal/mygo/codegen`) only if it is kept in
  sync for the same behavior; the primary target is codegen2.
- No change to the existing `(T, error)` -> `Result[T, error]` path or the
  `(T, bool)` -> `Option[T]` path.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `bootstrap-ffi-boundary`: the FFI boundary now also decodes a lone trailing
  `error` result into `Result[(), error]`, matching the existing rule that
  every error-bearing Go FFI call is `Result`-shaped.

## Impact

- `internal/mygo/codegen2/translate_ast.mygo` - `ffiResultPredicate` widened +
  `translateFFIResultCall` handling the 1-result (unit-payload) case.
- Optionally `internal/mygo/codegen/translate_call.go` (seed codegen) to keep
  the old compiler consistent (`goSigErrorResultType` / `wrapGoErrorResultCall`).
- `docs/compiler/ffi.md` - extend the "Automatic `Result` wrapping" section to
  document the lone-`error` -> `Result[(), error]` shape.
- Test coverage in codegen2 (compile-and-lower) and, if seed codegen is kept
  in sync, its unit tests.
