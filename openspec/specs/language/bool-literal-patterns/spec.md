# language/bool-literal-patterns Specification

## Purpose

Defines how MyGO recognizes, types, and matches boolean literal patterns in `switch`, including tuple and nested patterns, so `case true` and `case (true, false)` have the same status as existing number, string, and rune literal patterns.

## Requirements

### Requirement: Boolean literal patterns are accepted in pattern position

Parser2 SHALL recognize the keyword tokens `true` and `false` as literal patterns anywhere a pattern is accepted, including a whole switch pattern, tuple elements, variant payloads, and nested patterns. The parser SHALL retain the original token bytes in the CST, lower the pattern to a boolean literal pattern, and preserve it through formatting and lossless replay. A tuple pattern containing boolean literals SHALL keep all of its elements; a boolean literal element SHALL NOT be dropped.

#### Scenario: Single-value boolean switch
- **WHEN** source contains `switch flag` with `case true then 1 end` and `case false then 0 end`
- **THEN** the parser SHALL produce two boolean literal case patterns and compilation SHALL succeed

#### Scenario: Tuple boolean switch
- **WHEN** source contains `case (true, false)` against a `(Bool, Bool)` switch target
- **THEN** the parser SHALL produce a two-element tuple pattern whose elements are boolean literals, and compilation SHALL succeed

#### Scenario: Nested boolean pattern
- **WHEN** source contains `case (Some(true), false)`
- **THEN** the boolean literals SHALL be retained inside the variant payload and the tuple slot

#### Scenario: Lossless boolean pattern formatting
- **WHEN** a file containing boolean literal patterns is parsed and formatted
- **THEN** the syntax tree text SHALL replay the original source bytes and a second formatting pass SHALL produce identical text

### Requirement: Boolean literal patterns have type Bool

Type inference SHALL assign boolean literal patterns the type `Bool`. A switch target, or the corresponding tuple element of a switch target, SHALL unify with each boolean literal pattern at its position. If the target position is not `Bool`, the compiler SHALL reject the program with a pattern/target type mismatch diagnostic; boolean literal patterns SHALL NOT be treated as integer literals.

#### Scenario: Boolean target is accepted
- **WHEN** a `Bool` switch contains `case true` and `case false`
- **THEN** inference SHALL accept both patterns as `Bool`

#### Scenario: Non-Boolean target is rejected
- **WHEN** an `Int` switch contains `case true`
- **THEN** the compiler SHALL report a pattern/target type mismatch at the boolean pattern

#### Scenario: Tuple element mismatch is rejected
- **WHEN** a `(Bool, Int)` switch contains `case (true, true)`
- **THEN** the compiler SHALL report a type mismatch for the second tuple element and SHALL NOT report a tuple arity mismatch

#### Scenario: Boolean literal is not inferred as Int
- **WHEN** a `(Bool, Bool)` switch contains `case (true, false)`
- **THEN** inference SHALL type both literals as `Bool` and SHALL NOT unify them with `Int`

### Requirement: Boolean literal patterns match by value in every switch context

The compiler SHALL generate matching that selects a boolean literal case only when the subject value equals that literal. A tuple pattern SHALL require every element to match. A guarded boolean case SHALL evaluate its guard only after the boolean pattern matches, and a false guard SHALL allow later cases to be attempted. Statement, expression, tail, and nested-variant switch forms SHALL produce the same selection results.

#### Scenario: Four-arm flag tuple
- **WHEN** a `(Bool, Bool)` switch handles `(true, true)`, `(true, false)`, `(false, true)`, and `(false, false)` with distinct bodies
- **THEN** each input pair SHALL select its corresponding body in both codegen2 and codegen3 output

#### Scenario: Guarded boolean case falls through
- **WHEN** `case (true, _) if flag then ... end` matches `(true, v)` but `flag` is false, and a later case matches the value
- **THEN** the later case SHALL be attempted and the guarded case body SHALL NOT execute

#### Scenario: Single-value boolean switch
- **WHEN** a `Bool` switch contains `case true then 1 end` and `case false then 0 end`
- **THEN** the generated program SHALL return `1` for `true` and `0` for `false`

#### Scenario: Boolean literal inside a variant payload
- **WHEN** a switch matches `case Some(true) then ... end` against `Some(true)` and `Some(false)`
- **THEN** only the `Some(true)` input SHALL select that case

### Requirement: Boolean literal switches are not subject to enum exhaustiveness

A switch over `Bool`, or over a tuple containing `Bool`, SHALL follow the existing non-enum literal-switch rule and SHALL NOT require coverage of both `true` and `false`. A boolean literal switch without a wildcard SHALL compile. Enum exhaustiveness inside the same tuple SHALL remain unchanged.

#### Scenario: Partial boolean switch is accepted
- **WHEN** a `Bool` switch contains only `case true then 1 end`
- **THEN** compilation SHALL succeed as it does for a partial `Int` or `String` literal switch

#### Scenario: Boolean tuple position adds no coverage requirement
- **WHEN** a `(Bool, Int)` switch handles `(true, 0)` and `(false, 0)` without a wildcard
- **THEN** compilation SHALL succeed without a missing-variant diagnostic

#### Scenario: Nested enum coverage is unchanged
- **WHEN** a `(Color, Bool)` switch covers every `Color` variant and uses boolean literals only in the second position
- **THEN** the existing enum exhaustiveness result SHALL be unchanged
