## MODIFIED Requirements

### Requirement: Enum switch must cover every variant
When a MyGO `switch` matches on an enum value, the compiler SHALL require that every declared variant of that enum is handled by an unguarded `case` or that an unguarded wildcard `_` pattern is present. A case with a guard SHALL NOT contribute coverage, even when its pattern names a variant or is a wildcard. Otherwise, the compiler SHALL reject the program with a compile error identifying the missing variant(s).

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
- **WHEN** a `switch` matches a `Color` value, handles `Red`, and includes an unguarded `_` wildcard case
- **THEN** compilation succeeds

#### Scenario: Guarded variant does not cover its variant
- **GIVEN** an enum `Color` with variants `Red`, `Green`, and `Blue`
- **WHEN** a `switch` handles `Red` and `Blue` unguarded, and handles `Green` only as `case Green if flag`
- **THEN** the compiler SHALL report `Green` as a missing variant

#### Scenario: Guarded wildcard does not accept incomplete coverage
- **GIVEN** an enum `Color` with variants `Red`, `Green`, and `Blue`
- **WHEN** a `switch` handles `Red` unguarded and handles the remaining values only with `case _ if flag`
- **THEN** the compiler SHALL report `Green` and `Blue` as missing variants

#### Scenario: Unguarded case after a guarded case restores coverage
- **GIVEN** an enum `Color` with variants `Red`, `Green`, and `Blue`
- **WHEN** a `switch` contains `case Green if flag` followed by an unguarded `case Green`
- **THEN** `Green` SHALL count as covered and compilation SHALL succeed when `Red` and `Blue` are also covered

### Requirement: Nested enum patterns are checked recursively
When a `switch` matches on a pattern containing nested patterns, such as a tuple of enums, each enum position SHALL satisfy the same exhaustiveness rules. A missing variant at any nested position SHALL be reported as a compile error. A case with a guard SHALL NOT contribute coverage at any nested position.

#### Scenario: Incomplete nested handle is rejected
- **GIVEN** an enum `Color` with variants `Red`, `Green`, and `Blue`
- **WHEN** a `switch` matches `(Color, Int)` and only handles `(Red, _)`, `(Green, _)`, and `(Blue, _)` where the third variant `Blue` is missing from the enum's actual definition
- **THEN** the compiler SHALL reject the program as non-exhaustive

#### Scenario: Complete nested coverage is accepted
- **GIVEN** an enum `Color` with variants `Red`, `Green`, and `Blue`
- **WHEN** a `switch` matches `(Color, Bool)` and handles `(Red, _)`, `(Green, _)`, and `(Blue, _)`
- **THEN** compilation succeeds because every `Color` variant is covered

#### Scenario: Guarded nested tuple case does not cover
- **GIVEN** an enum `Color` with variants `Red`, `Green`, and `Blue`
- **WHEN** a `switch` matches `(Color, Bool)` and handles all three `Color` variants only through guarded tuple cases
- **THEN** the compiler SHALL report the enum variants as missing

## ADDED Requirements

### Requirement: Guards do not introduce exhaustiveness checks for non-enum switches
A `switch` whose target type is not an enum SHALL remain exempt from exhaustiveness checking when its cases have guards. Omitting a wildcard SHALL remain accepted, and a false guard SHALL follow the switch's existing unmatched behavior.

#### Scenario: Guarded integer switch without a wildcard is accepted
- **GIVEN** a variable `x: Int`
- **WHEN** a `switch` matches `x` with only `case 0 if flag => 1`
- **THEN** compilation SHALL succeed without reporting non-exhaustiveness
