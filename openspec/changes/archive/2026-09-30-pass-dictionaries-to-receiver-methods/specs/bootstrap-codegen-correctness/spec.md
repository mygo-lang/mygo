## ADDED Requirements

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
