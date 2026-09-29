## ADDED Requirements

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
