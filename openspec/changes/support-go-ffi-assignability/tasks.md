# Tasks

## 1. Go Assignability Service

- [ ] 1.1 Add a narrow Go type service that resolves canonical Go source and target types and reports `go/types` assignability; verify focused unit cases for named types, aliases, interfaces, and pointer method sets.
- [ ] 1.2 Thread the service through bootstrap package loading and inference entry points while keeping non-FFI inference usable without it; verify existing bootstrap inference tests pass.
- [ ] 1.3 Map FFI `Ref[T]` arguments to Go `*T` in assignability queries and verify a pointer-receiver implementation is accepted for its interface parameter.

## 2. Boundary Inference and Regression Coverage

- [ ] 2.1 Invoke Go assignability only after ordinary unification fails for a Go FFI argument, preserving argument locations and strict rejection; verify positive and negative call-argument diagnostics.
- [ ] 2.2 Add an end-to-end regression matching `smtp.NewServer(Ref[SMTPBackend])` with a pointer-receiver implementation of `smtp.Backend`; verify bootstrap compilation and generated Go type-checking succeed.
- [ ] 2.3 Add regression cases for compatible Go interface assignments and incompatible named types; verify only Go-assignable calls are accepted.
- [ ] 2.4 Document the Go FFI assignability boundary and its `Ref[T]` pointer mapping; verify the compiler2 FFI documentation matches the implemented behavior.

## 3. Integration Verification

- [ ] 3.1 Run the focused compiler, typeinference2, and bootstrap test suites and build the reported mailgun-api-gateway package with bootstrap; verify generated Go compiles with Go's type checker.
