# Tasks

## 1. Normalize FFI Unit/Error Results

- [ ] 1.1 Map Go FFI call results of lone `error` and `(struct{}, error)` to `Result[(), Error]`; add inference regressions showing both unify with an annotated Result unit value.
- [ ] 1.2 Apply the same result normalization when resolving anonymous and named Go callback signatures; verify aliases and defined function types preserve their parameter and non-unit result types.

## 2. Adapt Result Callbacks to Go

- [ ] 2.1 Add boundary-scoped callback compatibility for MyGO `Result[(), Error]` callbacks and Go error-only or unit/error callback signatures; verify incompatible arity, parameters, and non-unit results are rejected.
- [ ] 2.2 Generate a Go ABI adapter that maps `Ok(())` and `Err(e)` to the Go callback return convention, and verify generated Go compiles for both `func(...) error` and `func(...) (struct{}, error)` fixtures.
- [ ] 2.3 Add an end-to-end SMTP-style named `SendFunc` fixture and verify the callback is invoked once, errors propagate, and the generated code uses an adapter only where the Go ABI requires it.
