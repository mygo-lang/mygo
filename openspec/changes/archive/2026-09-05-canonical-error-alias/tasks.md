## 1. Self-hosted type inference

- [x] 1.1 Add regression tests that prove `TCon("Error")` and
  `TCon("error")` unify, that the shared spelling table and the Go FFI
  boundary resolve `error` to `Error` (while source type construction
  preserves the written spelling), and that equivalent spellings cross a
  declaration boundary. Verify with
  `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/typeinference2`
  before implementation and record the expected failures.
- [x] 1.2 Add a shared primitive spelling table (Go name ↔ MyGO name, with
  `error`/`Error` as one pair) and derive `GoTypeName`'s FFI mapping from it.
  Verify with
  `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/typeinference2`.
- [x] 1.3 Canonicalise Go-spelled builtin constructors (`TCon("error")` →
  `TCon("Error")`) at the unification entry point through the shared table,
  keeping the TCon/TCon comparison a plain name check and preserving existing
  application and type-variable behavior. Verify with
  `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/typeinference2`.

## 2. Self-hosted declaration lowering

- [x] 2.1 Add declaration tests asserting generated `error` for `Error`
  parameters/results, lower-case `error` signatures, and an `Error` component
  in a tuple result. Add a generic test whose type parameter is named `Error`
  and assert that it is preserved. Verify with
  `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/codegen2` before
  implementation and record the expected failures.
- [x] 2.2 Derive declaration primitive lowering from the shared spelling
  table so both `Error` and `error` lower to Go `error` without duplicated
  entries. Verify with
  `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/codegen2`.
- [x] 2.3 Add explicit type-parameter precedence to the AST declaration type
  path before unit, special, HKT, and primitive lowering. Verify that the
  generic named-`Error` test passes and existing generic declaration tests
  still pass with
  `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/codegen2`.

## 3. Bootstrap synchronization and validation

- [x] 3.1 Regenerate `typeinference2` from its MyGO sources with
  `go run ./cmd/mygo --bootstrap sync internal/mygo/typeinference2`; inspect
  that only the expected generated files changed.
- [x] 3.2 Regenerate `codegen2` from its MyGO sources with
  `go run ./cmd/mygo --bootstrap sync internal/mygo/codegen2`; inspect that
  only the expected generated files changed.
- [x] 3.3 Run focused and neighboring compiler validation with
  `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/typeinference2 ./internal/mygo/codegen2 ./internal/mygo/compiler`.
- [x] 3.4 Run OpenSpec validation with `openspec validate canonical-error-alias --strict`
  and resolve any planning-metadata or requirement-format errors.
