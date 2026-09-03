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
