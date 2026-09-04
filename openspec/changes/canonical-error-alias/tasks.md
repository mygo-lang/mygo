## 1. Self-hosted type inference

- [ ] 1.1 Add regression tests that prove `TCon("Error")` and
  `TCon("error")` unify, that source `error` type syntax constructs canonical
  `Error`, and that equivalent spellings cross a declaration boundary. Verify
  with `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/typeinference2`
  before implementation and record the expected failures.
- [ ] 1.2 Add the canonical-name helper and apply it to bare source type
  construction in `typeFromAST`, `typeFromASTWithParams`, and the
  environment-aware constructor. Verify with
  `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/typeinference2`.
- [ ] 1.3 Make TCon constructor-name comparison use the canonical alias while
  preserving existing application and type-variable behavior. Verify with
  `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/typeinference2`.

## 2. Self-hosted declaration lowering

- [ ] 2.1 Add declaration tests asserting generated `error` for `Error`
  parameters/results, lower-case `error` signatures, and an `Error` component
  in a tuple result. Add a generic test whose type parameter is named `Error`
  and assert that it is preserved. Verify with
  `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/codegen2` before
  implementation and record the expected failures.
- [ ] 2.2 Add the canonical and lower-case builtin error entries to the
  declaration primitive mapping. Verify with
  `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/codegen2`.
- [ ] 2.3 Add explicit type-parameter precedence to the AST declaration type
  path before unit, special, HKT, and primitive lowering. Verify that the
  generic named-`Error` test passes and existing generic declaration tests
  still pass with
  `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/codegen2`.

## 3. Bootstrap synchronization and validation

- [ ] 3.1 Regenerate `typeinference2` from its MyGO sources with
  `go run ./cmd/mygo --bootstrap sync internal/mygo/typeinference2`; inspect
  that only the expected generated files changed.
- [ ] 3.2 Regenerate `codegen2` from its MyGO sources with
  `go run ./cmd/mygo --bootstrap sync internal/mygo/codegen2`; inspect that
  only the expected generated files changed.
- [ ] 3.3 Run focused and neighboring compiler validation with
  `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/typeinference2 ./internal/mygo/codegen2 ./internal/mygo/compiler`.
- [ ] 3.4 Run OpenSpec validation with `openspec validate canonical-error-alias --strict`
  and resolve any planning-metadata or requirement-format errors.
