## ADDED Requirements

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
