## 1. Bootstrap inference state propagation

- [x] 1.1 Update named enum-variant field recursion to pass the state returned
  by each inferred field expression, and verify bootstrap synchronization of
  `internal/mygo/typeinference2` succeeds.

## 2. Regression coverage

- [x] 2.1 Add a bootstrap inference regression program with a named
  enum-variant literal followed by a constrained empty slice, and verify the
  test fails before the fix and passes after it.
- [x] 2.2 Run `go test ./internal/mygo/typeinference2/...` and verify all
  bootstrap type inference tests pass.
