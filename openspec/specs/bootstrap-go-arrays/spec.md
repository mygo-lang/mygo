# bootstrap-go-arrays Specification

## Purpose

Define native fixed-length Go arrays in the bootstrap compiler, including type identity, exact initialization, explicit repetition, source tooling, and interoperability, so MyGO programs can express array values without implicit padding or losing their static lengths.

## Requirements

### Requirement: Fixed-length array types
The bootstrap compiler SHALL accept `T[N]` as an array type where N is a non-negative integer literal representable by the target Go int. Length SHALL participate in type identity. Successive suffixes SHALL wrap the preceding type from left to right. Array types SHALL be usable in annotations, fields, aliases, parameters, returns, and generic type arguments, including generic element types.

#### Scenario: Nested and generic arrays
- **WHEN** source uses `Int32[5][3]`, `Slice[Int32[5]]`, or a generic function parameter `T[5]`
- **THEN** the corresponding Go types are `[3][5]int32`, `[][5]int32`, and `[5]T`

#### Scenario: Invalid length
- **WHEN** an array length is negative, fractional, a variable, a constant expression, or outside the target int range
- **THEN** compilation fails with a source-located array length diagnostic

#### Scenario: Distinct lengths
- **WHEN** an `Int[3]` value is passed where `Int[5]` is required
- **THEN** compilation fails and identifies the conflicting array lengths

### Requirement: Exact array literal initialization
The compiler SHALL interpret a list literal as an array when an explicit annotation, ascription, function parameter, return, or field supplies an expected array type. It MUST require exactly N elements and recursively check element types and nested lengths. It MUST NOT insert missing zero values or implicitly convert Slice values into arrays. List literals without an expected array type SHALL retain Slice inference.

#### Scenario: Exact elements
- **WHEN** source contains `[1, 2, 3] as Int32[3]`
- **THEN** compilation produces an array containing those three values with Go type `[3]int32`

#### Scenario: Missing or extra elements
- **WHEN** source contains `[1, 2, 3] as Int[5]`, `[1, 2, 3] as Int[2]`, or `[] as Int[5]`
- **THEN** compilation fails and reports expected and actual element counts

#### Scenario: Zero-length array
- **WHEN** source contains `[] as Int[0]`
- **THEN** compilation succeeds with Go type `[0]int`

#### Scenario: Context and nesting
- **WHEN** a literal initializes an array field, argument, annotated binding, or return, including nested arrays
- **THEN** expected types determine the array and element types and every array level requires its exact declared count

#### Scenario: Slice compatibility
- **WHEN** a list literal has no array context or an existing Slice value is ascribed an array type
- **THEN** the literal remains a Slice and the Slice-to-array ascription fails

### Requirement: Explicit array repetition
The compiler SHALL support an array-valued left operand multiplied by a non-negative integer literal K, producing an array of the same element type and length N times K. It SHALL repeat the whole input sequence in order, evaluate the left operand exactly once even when K is zero, and copy elements using Go value semantics. It SHALL reject invalid repeat counts and result lengths outside the target Go int range. Numeric multiplication SHALL retain its existing behavior; scalar-times-array and Slice repetition are outside this operation.

#### Scenario: Requested shorthand and precedence
- **WHEN** source contains `[0] as Int[1] * 5`
- **THEN** it is interpreted as `([0] as Int[1]) * 5` and produces `Int[5]` with five zero elements

#### Scenario: Sequence repetition
- **WHEN** source contains `([1, 2] as Int[2]) * 3`
- **THEN** the result is `Int[6]` containing `[1, 2, 1, 2, 1, 2]`

#### Scenario: Single evaluation and zero count
- **WHEN** an array-producing expression with side effects is repeated by 3 or by 0
- **THEN** the expression executes once, with the latter producing an empty array of the same element type

#### Scenario: Invalid repetition
- **WHEN** the repeat count is negative, fractional, runtime-dependent, a constant expression, or causes length overflow
- **THEN** compilation fails with a source-located repetition diagnostic

### Requirement: Native Go array interoperability
The compiler SHALL emit native Go arrays and recognize Go `[N]T` signatures, preserving lengths across imported functions and fields. Array assignment and parameter passing SHALL use Go value semantics, including reference sharing for reference-valued elements.

#### Scenario: Go boundary round trip
- **WHEN** MyGO passes `Int32[5]` to a Go function accepting `[5]int32` and receives `[5]int32` from Go
- **THEN** both directions compile without a wrapper or implicit Slice conversion and the returned MyGO type is `Int32[5]`

#### Scenario: Copy behavior
- **WHEN** an array is copied and one copy's value element is changed through supported Go interoperation
- **THEN** the other array's value element is unchanged, while copied reference-valued elements still refer to the same objects

### Requirement: Array source tooling
Formatting and type displays SHALL preserve array lengths, nested structure, and the meaning of repetition expressions. Existing generic applications and numeric multiplication SHALL continue to parse and format correctly.

#### Scenario: Formatting round trip
- **WHEN** source containing `Option[Int32][5]`, nested arrays, and `[0] as Int[1] * 5` is formatted twice and compiled
- **THEN** formatting is idempotent and the array types and repetition semantics are preserved
