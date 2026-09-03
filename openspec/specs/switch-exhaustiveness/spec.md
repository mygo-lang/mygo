# switch-exhaustiveness Specification

## Purpose

Defines the compile-time exhaustiveness requirements for MyGO `switch` expressions and statements, ensuring every possible enum value is handled before code generation.

## Requirements

### Requirement: Enum switch must cover every variant
When a MyGO `switch` matches on an enum value, the compiler SHALL require that every declared variant of that enum is handled by a `case` or that a wildcard `_` pattern is present. Otherwise, the compiler SHALL reject the program with a compile error identifying the missing variant(s).

#### Scenario: Missing enum variant is rejected
- **GIVEN** an enum `Color` with variants `Red`, `Green`, and `Blue`
- **WHEN** a `switch` matches a `Color` value and only handles `Red` and `Green`
- **THEN** the compiler SHALL return a compile error listing `Blue` as a missing variant

#### Scenario: All variants covered is accepted
- **GIVEN** an enum `Color` with variants `Red`, `Green`, and `Blue`
- **WHEN** a `switch` matches a `Color` value and handles all three variants
- **THEN** compilation succeeds without a wildcard pattern

#### Scenario: Wildcard accepts incomplete coverage
- **GIVEN** an enum `Color` with variants `Red`, `Green`, and `Blue`
- **WHEN** a `switch` matches a `Color` value, handles `Red`, and includes a `_` wildcard case
- **THEN** compilation succeeds

### Requirement: Nested enum patterns are checked recursively
When a `switch` matches on a pattern containing nested patterns, such as a tuple of enums, each enum position SHALL satisfy the same exhaustiveness rules. A missing variant at any nested position SHALL be reported as a compile error.

#### Scenario: Incomplete nested handle is rejected
- **GIVEN** an enum `Color` with variants `Red`, `Green`, and `Blue`
- **WHEN** a `switch` matches `(Color, Int)` and only handles `(Red, _)`, `(Green, _)`, and `(Blue, _)` where the third variant `Blue` is missing from the enum's actual definition
- **THEN** the compiler SHALL reject the program as non-exhaustive

#### Scenario: Complete nested coverage is accepted
- **GIVEN** an enum `Color` with variants `Red`, `Green`, and `Blue`
- **WHEN** a `switch` matches `(Color, Bool)` and handles `(Red, _)`, `(Green, _)`, and `(Blue, _)`
- **THEN** compilation succeeds because every `Color` variant is covered

### Requirement: Statement-form switch must be exhaustive
When a `switch` is used in statement position (not as a tail expression), it SHALL satisfy the same exhaustiveness rules as expression-position `switch`. Omitting a variant or a wildcard in statement form SHALL be a compile error.

#### Scenario: Statement switch missing a variant is rejected
- **GIVEN** an enum `Color` with variants `Red`, `Green`, and `Blue`
- **WHEN** a statement-position `switch` matches a `Color` and only handles `Red` and `Green`
- **THEN** the compiler SHALL reject the program with a non-exhaustive switch error

#### Scenario: Statement switch with wildcard is accepted
- **GIVEN** an enum `Color` with variants `Red`, `Green`, and `Blue`
- **WHEN** a statement-position `switch` matches a `Color`, handles `Red` and `Green`, and includes a `_` wildcard case
- **THEN** compilation succeeds

### Requirement: Built-in non-enum types are not exhaustively checked
When a `switch` matches on a value whose type is not an enum (for example `Int`, `String`, or `Float`), the compiler SHALL NOT attempt exhaustiveness checking and SHALL accept the switch without a wildcard.

#### Scenario: Non-enum switch without wildcard is accepted
- **GIVEN** a variable `x: Int`
- **WHEN** a `switch` matches `x` with only an integer literal case
- **THEN** compilation succeeds without reporting non-exhaustiveness

#### Scenario: Tuple containing non-enum is not exhaustively checked
- **GIVEN** a value `pair: (Int, String)`
- **WHEN** a `switch` matches `pair` with only selected literal cases
- **THEN** compilation succeeds
