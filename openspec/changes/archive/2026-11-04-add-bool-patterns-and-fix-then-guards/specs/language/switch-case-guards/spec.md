## ADDED Requirements

### Requirement: Then-form guarded cases preserve their own structural terminator

Parser2 SHALL determine a case's header terminator with delimiter-depth-aware scanning that starts at that case, so a `then` or `=>` inside a parenthesized guard expression SHALL NOT be mistaken for the case header terminator. A parenthesized `if` expression in a `then`-form guard SHALL remain part of the guard expression, and the case's own `end` SHALL pair with that case. Arrow-form guarded cases SHALL keep their existing behavior, and lossless replay, formatter idempotence, and guard evaluation semantics SHALL be preserved.

#### Scenario: Parenthesized if guard in then form
- **WHEN** source contains `case x if (if x > 0 then true else false end) then x end`
- **THEN** the parenthesized `if` SHALL be the guard expression, the case SHALL pair with its own `end`, and compilation SHALL succeed

#### Scenario: Parenthesized if guard in arrow form remains accepted
- **WHEN** source contains `case x if (if x > 0 then true else false end) => x`
- **THEN** parsing and guard evaluation SHALL match the existing arrow-form behavior

#### Scenario: Guarded tuple case in then form
- **WHEN** source contains `case (a, b) if (if a then b else false end) then ... end` followed by another case
- **THEN** the first case SHALL pair with its own `end` and the following case SHALL be parsed as a sibling case

#### Scenario: Nested parentheses in a then-form guard
- **WHEN** a then-form guard contains a parenthesized `if` expression nested inside another parenthesized expression
- **THEN** only the case's own depth-zero `then` SHALL terminate its header

#### Scenario: Unparenthesized second top-level if is still rejected
- **WHEN** source contains a then-form `case P if a && if b then c else d end then ... end`
- **THEN** the compiler SHALL still reject the case with the existing diagnostic requiring the guard's `if` expression to be parenthesized
