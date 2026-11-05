# bootstrap-generic-type-application Specification

## Purpose

Defines how MyGO applies type arguments to user-defined generic types without
allowing caller type parameters to be captured by the applied type's own
parameters, so nested signatures preserve the types the source declares.

## Requirements

### Requirement: Generic type arguments are applied simultaneously
Applying arguments to a generic type SHALL replace each declared type parameter
with exactly the corresponding supplied argument. A supplied argument that is
itself a type parameter SHALL remain that parameter and SHALL NOT be resolved
through another parameter of the applied type.

#### Scenario: A Result-returning callback keeps U and E distinct
- **WHEN** a function `apply[T, U, E]` declares `fn: func(T) -> Result[U, E]`
  and a caller explicitly supplies `Int`, `String`, and `Error`
- **THEN** the expected callback type SHALL be
  `func(Int) -> Result[String, Error]`
- **AND** the first result argument SHALL remain `String` rather than becoming
  `Error`

#### Scenario: A generic application preserves later enclosing parameters
- **WHEN** a nested type application uses the second or third type parameter of
  its enclosing declaration as an earlier argument position of a different
  generic type
- **THEN** each nested argument SHALL retain its own declared identity
- **AND** inference SHALL NOT collapse two distinct parameters into one

### Requirement: Nested generic signatures produce buildable Go
When inference accepts a generic callback or nested generic application, the
generated Go SHALL retain the same type argument order and SHALL compile.

#### Scenario: Explicit generic callback compiles
- **WHEN** a caller invokes `apply[Int, String, Error]` with a callback whose
  result is `Result[String, Error]`
- **THEN** the generated call SHALL pass `int`, `string`, and `error` as the
  three distinct type arguments
- **AND** the generated callback type SHALL be `Result[string, error]`
- **AND** `go build` SHALL succeed

#### Scenario: Inferred generic callback compiles
- **WHEN** the same callback is supplied without explicit call type arguments
- **THEN** inference SHALL derive `T`, `U`, and `E` independently from the
  value and callback types
- **AND** the generated Go SHALL compile and preserve successful and error
  payload behavior at runtime

### Requirement: Generic type aliases follow the same substitution rule
A type alias that expands to a user-defined generic type SHALL preserve the
same simultaneous replacement behavior as applying the target type directly.

#### Scenario: An alias of Result keeps its arguments separate
- **WHEN** an alias `Pair[A, B]` expands to `Result[A, B]` and a generic
  signature uses `Pair[U, E]`
- **THEN** the expanded result SHALL be `Result[U, E]` with two distinct
  arguments
- **AND** a call that supplies `String` and `Error` SHALL produce
  `Result[String, Error]`
