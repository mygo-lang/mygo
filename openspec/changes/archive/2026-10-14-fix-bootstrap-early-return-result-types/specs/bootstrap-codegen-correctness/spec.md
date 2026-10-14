## ADDED Requirements

### Requirement: Explicit returns preserve the enclosing function result type
Bootstrap SHALL constrain each explicit return value to the return type of its enclosing function, including returns inside conditional branches, switch cases, loops, and nested blocks. Generated generic constructors SHALL retain the resulting type arguments rather than replacing context-resolved parameters with `any`.

#### Scenario: Early Err returns a unit Result
- **WHEN** a function declares `Result[(), String]`, conditionally executes `return Err("boom")`, and otherwise returns `Ok(())`
- **THEN** the generated early return SHALL construct `Err[struct{}, string]`
- **AND** the returned failure's `IsErr()` SHALL be `true`, `IsOk()` SHALL be `false`, and `UnwrapErr()` SHALL return `"boom"`

#### Scenario: Early constructors preserve non-unit and generic success types
- **WHEN** a function returning `Result[Int, String]` or a generic function returning `Result[A, String]` executes an inferred early `Err` constructor
- **THEN** the generated constructor SHALL retain `int` or the declared generic parameter `A` respectively

#### Scenario: Nested branches and loop bodies return correctly typed failures
- **WHEN** an early `Err` return occurs in a nested conditional branch, a switch case, or a loop body of a function returning `Result[(), String]`
- **THEN** the generated Go SHALL compile and a taken failure path SHALL be recognized as an error

#### Scenario: Early None uses the declared Option type
- **WHEN** a function returning `Option[Int]` executes an early `return None` and otherwise returns `Some(1)`
- **THEN** the generated constructor SHALL retain the `int` type argument and the returned absence SHALL be recognized by `IsNone()`

#### Scenario: Explicit generic workaround remains compatible
- **WHEN** a function returning `Result[(), String]` uses an explicitly typed `Err[(), String]` early return
- **THEN** its generated and runtime behavior SHALL match the corresponding inferred constructor

### Requirement: Return contexts are scoped to individual functions
Bootstrap SHALL check returns inside a function literal against that literal's own return type, independently of its enclosing function. Unannotated functions SHALL infer a return type consistent with their explicit return values and value-producing tail paths.

#### Scenario: Closure has a different result type from its enclosing function
- **WHEN** a function returning `Result[(), String]` contains a closure returning `Option[Int]` with an early `return None`
- **THEN** the closure SHALL retain `Option[Int]` and SHALL NOT use the outer function's `Result` type for its return

#### Scenario: Unannotated function infers consistent return paths
- **WHEN** an unannotated function has an early `return Err("boom")` and a value-producing tail `Ok(1)`
- **THEN** bootstrap SHALL infer `Result[Int, String]` consistently for both paths

### Requirement: Incompatible explicit returns are rejected during inference
Bootstrap SHALL reject an explicit return value incompatible with its enclosing function's result type before emitting Go, with a diagnostic identifying the return expression's source location.

#### Scenario: Early return has the wrong error payload
- **WHEN** a function returning `Result[(), String]` contains `return Err(1)` before a valid tail result
- **THEN** bootstrap SHALL report a return type mismatch at the invalid return value rather than emitting an incompatible generic instance

#### Scenario: Closure return conflicts with its own signature
- **WHEN** a closure returning `Option[Int]` contains an early return of a `Result` value
- **THEN** bootstrap SHALL reject the closure's return at its source location
