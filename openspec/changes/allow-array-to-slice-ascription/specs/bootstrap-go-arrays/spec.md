## ADDED Requirements

### Requirement: Array-to-slice ascription
The compiler SHALL accept `value as Slice[T]` when `value` has array type `T[N]`,
producing a `Slice[T]` that is a Go slice view of the array (`value[:]`). The
array element type, including nested element types, MUST unify with the slice
element type. The ascription MUST be usable wherever an expression is accepted,
including bindings, returns, arguments, and fields. An ascription whose element
types do not unify, and any Slice-to-array ascription, MUST fail with a
source-located diagnostic. Array type identity, exact literal initialization, and
repetition semantics SHALL be unchanged.

#### Scenario: Basic ascription
- **WHEN** source contains `values as Slice[Int]` where `values` has type `Int[32]`
- **THEN** the expression has type `Slice[Int]`, the generated Go is `values[:]`, and the resulting slice has length 32 with the same elements

#### Scenario: Nested element types
- **WHEN** source contains `matrix as Slice[Int[2]]` where `matrix` has type `Int[3][2]`
- **THEN** compilation succeeds with Go type `[][2]int`

#### Scenario: Element type mismatch
- **WHEN** source contains `values as Slice[String]` where `values` has type `Int[32]`
- **THEN** compilation fails with a source-located diagnostic naming the conflicting types

#### Scenario: Slice-to-array ascription still fails
- **WHEN** source contains `items as Int[32]` where `items` has type `Slice[Int]`
- **THEN** compilation fails and identifies the conflicting Slice and array types

#### Scenario: Ascription in return position
- **WHEN** a function declared to return `Slice[Int]` ends with `values as Slice[Int]` for an `Int[32]` value
- **THEN** compilation succeeds and the function returns the slice view

