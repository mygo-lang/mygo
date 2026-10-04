# Tasks

## 1. Go Assignability Service

- [x] 1.1 Add a narrow Go type service that resolves canonical Go source and target types and reports `go/types` assignability; verify focused unit cases for named types, aliases, interfaces, and pointer method sets.
- [x] 1.2 Thread the service through bootstrap package loading and inference entry points while keeping non-FFI inference usable without it; verify existing bootstrap inference tests pass.
- [x] 1.3 Map FFI `Ref[T]` arguments to Go `*T` in assignability queries and verify a pointer-receiver implementation is accepted for its interface parameter.

## 2. Boundary Inference and Regression Coverage

- [x] 2.1 Invoke Go assignability only after ordinary unification fails for a Go FFI argument, preserving argument locations and strict rejection; verify positive and negative call-argument diagnostics.
- [x] 2.2 Add an end-to-end regression matching `smtp.NewServer(Ref[SMTPBackend])` with a pointer-receiver implementation of `smtp.Backend`; verify bootstrap compilation and generated Go type-checking succeed.
- [x] 2.3 Add regression cases for compatible Go interface assignments and incompatible named types; verify only Go-assignable calls are accepted.
- [x] 2.4 Document the Go FFI assignability boundary and its `Ref[T]` pointer mapping; verify the compiler2 FFI documentation matches the implemented behavior.
- [x] 2.5 Treat an empty `TuplePattern` as unit when its target is `TUnit`; add inference and code generation regressions for `Ok(())` over a Go `error` call and verify the generated Go compiles.
- [x] 2.6 Filter automatic prelude imports per generated external test file by actual prelude-name usage; verify unused imports are omitted, required helper imports remain, and the external test package compiles.
- [x] 2.7 Resolve static inherent method calls on package-qualified types imported from another MyGO package to the imported package's mangled helper; verify `backends.SendPulseBackend.New`, `backends.SMTPBackend.New`, and `backends.DispatchBackend.New` generate helper calls rather than Go type selectors.

## 3. Integration Verification

- [ ] 3.1 After tasks 2.5–2.7, run the focused compiler, typeinference2, codegen2, and bootstrap suites, then bootstrap-build the reported mailgun-api-gateway package; verify generated Go compiles with Go's type checker and all reported integration failures are resolved.
