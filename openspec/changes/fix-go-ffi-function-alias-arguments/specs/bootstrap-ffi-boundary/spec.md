# Spec Delta

## MODIFIED Requirements

### Requirement: Values of Go named function types are callable
When a value's type is a Go named type whose underlying type is a function signature (for example `context.CancelFunc`), bootstrap inference SHALL treat the value as callable with the signature's parameters and results, and the generated Go SHALL emit a direct call of the value.

#### Scenario: Calling a CancelFunc value
- **WHEN** a bootstrap-compiled function binds `cancel` from a `go:context` call result typed `CancelFunc`, assigns it to a `let` binding of that type, and then evaluates `cancel()`
- **THEN** inference SHALL accept the call (no `cannot unify ... with function type` error) and the generated Go SHALL contain the direct invocation `cancel()`

## ADDED Requirements

### Requirement: Go FFI unit/error results unify with Result unit values
Bootstrap SHALL represent a Go FFI result consisting of a lone `error`, or a `(struct{}, error)` pair, as `Result[(), Error]` so either shape unifies with the MyGO unit-result type.

#### Scenario: Unifying a lone error result
- **WHEN** a Go FFI call returns only `error` and is used where `Result[(), Error]` is expected
- **THEN** inference SHALL unify the call result with `Result[(), Error]`, preserving the existing success and error behavior

#### Scenario: Unifying a struct{} and error result
- **WHEN** a Go FFI call returns `(struct{}, error)` and is used where `Result[(), Error]` is expected
- **THEN** inference SHALL treat the successful `struct{}` payload as MyGO unit and unify the call result with `Result[(), Error]`

### Requirement: Go callback error results interoperate with Result-returning MyGO functions
When a Go FFI function parameter has a function signature returning either a lone `error` or `(struct{}, error)`, bootstrap SHALL accept a MyGO callback with a corresponding `Result[(), Error]` return when its other parameter and result types match. Generated Go SHALL adapt the callback to the required Go return arity and propagate `Ok(())` as nil error and `Err(e)` as the Go error result.

#### Scenario: Passing Result callback to a lone-error Go callback type
- **WHEN** a MyGO callback returning `Result[(), Error]` is passed to a Go function parameter of type `func(...) error`
- **THEN** inference SHALL accept the compatible signature and generated Go SHALL adapt the callback so `Ok(())` returns nil and `Err(e)` returns `e`

#### Scenario: Passing Result callback to a unit-and-error Go callback type
- **WHEN** a MyGO callback returning `Result[(), Error]` is passed to a Go function parameter of type `func(...) (struct{}, error)`
- **THEN** inference SHALL accept the compatible signature and generated Go SHALL adapt the callback so `Ok(())` returns `(struct{}{}, nil)` and `Err(e)` returns `(struct{}{}, e)`

#### Scenario: Rejecting an incompatible callback signature
- **WHEN** a MyGO callback passed to a Go named function type differs in arity or in a non-unit parameter or result type
- **THEN** bootstrap inference SHALL reject the argument with a type mismatch
