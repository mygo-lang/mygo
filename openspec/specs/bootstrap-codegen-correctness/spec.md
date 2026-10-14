# bootstrap-codegen-correctness Specification

## Purpose

Defines correctness guarantees for bootstrap-generated Go so valid MyGO source
remains buildable when it uses tuple returns and automatically supplied Prelude
helpers across separately generated files.

## Requirements

### Requirement: Bootstrap preserves tuple-returning switch results
When a MyGO function declares a tuple return type and its final expression is
a `switch`, bootstrap generation SHALL return the selected tuple as the
function's individual Go result values. It SHALL NOT return a single
anonymous tuple value in that position.

#### Scenario: A tuple-returning reducer ends in a switch
- **WHEN** a bootstrap-compiled function declares `-> (State, Slice[Command])`
  and its final expression is a pattern-matching `switch` whose cases produce
  two-element tuples
- **THEN** its generated Go SHALL compile and each selected case SHALL provide
  the function's two declared Go return values

### Requirement: Bootstrap imports Prelude helpers used by emitted code
For every generated non-Prelude Go file, bootstrap SHALL add the Prelude dot
import when its emitted declarations reference Prelude types, constructors, or
lowered helper functions, including helpers selected through collection or
string method dispatch.

#### Scenario: Collection methods lower to Prelude helpers
- **WHEN** a bootstrap-compiled source file uses `Slice.Append`, `Slice.Each`,
  or `String.Len`
- **THEN** the generated Go file SHALL include the required Prelude import and
  compile against the Prelude helper symbols

### Requirement: Bootstrap passes dictionaries to receiver-style typeclass method helpers
When bootstrap lowers a receiver-style call to a typeclass implementation
method whose declaration has `using` constraints, it SHALL pass the matching
instance dictionary argument for every constraint to the generated helper,
even when that method implementation does not use the dictionary. Matching
SHALL use the inferred constrained type, and arguments SHALL follow the helper's
declared constraint and method order. The generated Go SHALL compile.

#### Scenario: Set Contains receives Eq evidence
- **WHEN** a bootstrap-compiled function calls `VALID_HEADERS.Contains(key)`
  where `VALID_HEADERS` is `Set[String]` and the selected `Contains` method
  declares `using Eq[String]`
- **THEN** the generated helper call SHALL include the `String` equality
  dictionary argument and the generated Go SHALL compile, even if the Set
  implementation uses Go map lookup without invoking that argument

#### Scenario: Receiver method requires multiple dictionaries
- **WHEN** a receiver-style typeclass method declares multiple `using`
  constraints and is called with concrete constrained types
- **THEN** the generated helper call SHALL include the matching dictionary for
  each constraint in declaration order and the generated Go SHALL compile

### Requirement: Bootstrap does not emit unused Prelude imports
Bootstrap SHALL omit a Prelude dot import from a generated file when no emitted
Go declaration references a Prelude identifier. Source-only interface
declarations that are erased from Go output SHALL NOT independently require an
import.

#### Scenario: An erased interface is the only Prelude reference
- **WHEN** a source file declares interfaces whose signatures use `Result` or
  other Prelude types, but the generated Go file contains no declaration that
  references Prelude identifiers
- **THEN** the generated Go file SHALL omit the Prelude import and compile
  without an unused-import diagnostic

### Requirement: Pattern-bound variables resolved under tuple-return switch cases
When bootstrap compiles a two-tuple `switch (a, b)` whose case body has a
tuple return type and references a pattern-bound variable inside a function
call argument, the generated Go SHALL reference the local binding, not an
anonymous-tuple field access.

#### Scenario: Function argument uses a destructured variant field in a tuple-returning case
- **WHEN** a function declares `-> (State, Slice[Command])`, its final
  expression is a `switch (state, event)` pattern match, and a case destructures
  an enum variant as `RunStarted { RunID, TaskID, UserID, SessionID, InitialMessage }`
  then calls a helper with `InitialMessage` as an argument before returning a
  two-element tuple literal
- **THEN** the generated Go SHALL bind `InitialMessage` to the variant field
  value and compile without a `type AgentEvent has no field or method InitialMessage`
  diagnostic
- **AND** each selected case SHALL still yield the function's two declared Go
  return values

### Requirement: Bare identifiers in nested pattern positions resolve to zero-field variants
A bare identifier used in a pattern position SHALL be resolved against the enum
declaration that owns that position. When the owning type's declaration names
the identifier as a zero-field enum constructor, the pattern SHALL be treated
as that variant pattern and SHALL NOT introduce a binding. Resolution SHALL
apply recursively to arguments of nested variant patterns and to elements of
nested tuple patterns. The generated Go SHALL compile.

#### Scenario: Nested Ok(Some(x)) and Ok(None) cases in one switch
- **WHEN** a `switch` over a `Result[Option[T], E]` has cases
  `case Ok(Some(x))` and `case Ok(None)`, and at least one case body does not
  read a bound name
- **THEN** the `Ok(None)` pattern SHALL resolve to a zero-field variant pattern
  with no binding, the generated Go SHALL compile without a
  `declared and not used` diagnostic, and each case SHALL perform its own type
  assertion on the scrutinee

#### Scenario: Sibling cases both assert the same variant
- **WHEN** two sibling cases of a `switch` match the same outer variant with
  different nested arguments, for example `case Ok(Some(x))` and
  `case Ok(None)`
- **THEN** the generated Go SHALL declare a separate assertion binding per case
  that binds a payload, and elide the binding for the case whose payload is a
  resolved zero-field variant

#### Scenario: Deeply nested variant pattern with an unread branch
- **WHEN** a case pattern nests at least two levels, such as
  `case Ok(Some(Ok(x)))`, and a sibling case matches the outer variant with a
  bare nullary pattern
- **THEN** each level of the pattern SHALL be resolved against its owning field
  type, each level SHALL emit an assertion whose binding is used or elided, and
  the generated Go SHALL compile

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
