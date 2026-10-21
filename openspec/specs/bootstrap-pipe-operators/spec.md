# bootstrap-pipe-operators Specification

## Purpose

Defines how the self-hosted bootstrap pipeline types, diagnoses and generates
Go code for the forward and backward pipe operators.

## Requirements

### Requirement: Forward pipe applies a function operand

The bootstrap pipeline SHALL accept `left |> right` when `right` has a
single-argument function type whose parameter accepts the type of `left`.
The expression SHALL have the function's result type, and generated Go SHALL
contain one call that passes `left` as the argument to the function value
produced by `right`. The same rule SHALL apply when the function is a named
function, a generic function, a parameter of function type or a function
returned by another expression.

#### Scenario: Forward pipe calls a named function
- **WHEN** `double` has type `Int -> Int` and source contains `1 |> double`
- **THEN** compilation succeeds with type `Int` and the generated Go calls
  `double` with `1` as its only argument

#### Scenario: Forward pipe calls a function-typed parameter
- **WHEN** a function parameter `f` has type `Int -> String` and its body
  contains `1 |> f`
- **THEN** compilation succeeds with type `String` and the generated Go calls
  the parameter value with `1`

#### Scenario: Forward pipe chain groups to the left
- **WHEN** source contains `1 |> double |> inc`
- **THEN** it is typed as `(1 |> double) |> inc` and the generated Go calls
  `inc` with the result of calling `double` with `1`

### Requirement: Backward pipe applies a function operand

The bootstrap pipeline SHALL accept `left <| right` when `left` has a
single-argument function type whose parameter accepts the type of `right`.
The expression SHALL have the function's result type, and generated Go SHALL
contain one call that passes `right` as the argument to the function value
produced by `left`. The same rule SHALL apply when the function is a named
function, a generic function, a parameter of function type or a function
returned by another expression.

#### Scenario: Backward pipe calls a named function
- **WHEN** `parse` has type `String -> Int` and source contains `parse <| input`
- **THEN** compilation succeeds with type `Int` and the generated Go calls
  `parse` with `input` as its only argument

#### Scenario: Backward pipe chain groups to the right
- **WHEN** `inc` and `double` both have type `Int -> Int` and source contains
  `inc <| double <| 2`
- **THEN** it is typed as `inc <| (double <| 2)` and the generated Go calls
  `inc` with the result of calling `double` with `2`

#### Scenario: Backward pipe accepts a function-typed parameter
- **WHEN** a function parameter `f` has type `Int -> String` and its body
  contains `f <| 1`
- **THEN** compilation succeeds with type `String` and the generated Go calls
  the parameter value with `1`

### Requirement: Pipe operands are typed independently of the result

The bootstrap pipeline SHALL type each pipe operand from its own context and
SHALL NOT require either operand to have the pipe expression's result type.
Generated Go SHALL evaluate the left operand before the right operand, matching
source order, and SHALL preserve side effects from both operands exactly once.

#### Scenario: Result type is not imposed on the argument operand
- **WHEN** `f` has type `Int -> String` and `1 |> f` is bound in a context that
  expects `String`
- **THEN** the literal `1` is typed as `Int`, the expression is typed as
  `String`, and compilation succeeds

#### Scenario: Operand side effects keep source order
- **WHEN** both operands of a pipe expression call functions with observable
  effects
- **THEN** the left operand's effect occurs before the right operand's effect,
  and each effect occurs once

### Requirement: Invalid pipe operands produce located diagnostics

When a pipe's function operand is not a single-argument function or its
parameter cannot accept the argument operand, the bootstrap pipeline SHALL
reject the program with a source-located diagnostic that names the pipe
direction. It SHALL NOT emit Go for that expression.

#### Scenario: Forward pipe rejects a non-function operand
- **WHEN** source contains `1 |> 2`
- **THEN** compilation fails with a located `|>` diagnostic describing the
  function-type mismatch

#### Scenario: Backward pipe rejects an incompatible argument
- **WHEN** `f` has type `Int -> Int` and source contains `f <| "text"`
- **THEN** compilation fails with a located `<|` diagnostic describing the
  parameter mismatch

#### Scenario: Mixed pipe chain follows the declared grouping
- **WHEN** source combines `|>` and `<|` with higher-precedence expressions
- **THEN** typing follows the parser's grouping, with `<|` binding looser than
  `|>` and chained `<|` grouping to the right

