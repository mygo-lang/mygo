# language/switch-case-guards Specification

## Purpose

Defines conditional `case` guards for MyGO `switch`, including their syntax, binding scope, fallthrough behavior, lossless syntax representation, and compatibility diagnostics.

## Requirements

### Requirement: Case guards are introduced by the first top-level `if` in a case header
A `switch` case SHALL accept an optional guard between its pattern and its `then` or `=>` terminator. The first top-level `if` before that terminator SHALL introduce the guard. A second `if` expression used inside the guard SHALL be parenthesized. An `if` expression after the case's `then` or `=>` SHALL remain an ordinary case-body expression.

#### Scenario: Then-form guarded case
- **WHEN** source contains `case Some(x) if x > 100 then x end`
- **THEN** the guard SHALL be `x > 100` and the case body SHALL be `x`

#### Scenario: Arrow-form guarded case
- **WHEN** source contains `case Some(x) if x > 100 => x`
- **THEN** the guard SHALL be `x > 100` and the case body SHALL be `x`

#### Scenario: Parenthesized if expression in a guard
- **WHEN** source contains `case Some(x) if (if x > 0 then true else false end) => x`
- **THEN** the parenthesized `if` SHALL be parsed as part of the guard expression

#### Scenario: Unparenthesized second top-level if is rejected
- **WHEN** source contains `case P if a && if b then c else d end => e`
- **THEN** the compiler SHALL reject the case with a diagnostic requiring the guard's `if` expression to be parenthesized

#### Scenario: If expression after the case terminator is a body expression
- **WHEN** source contains `case P => if cond then a else b end`
- **THEN** the `if` SHALL belong to the case body and SHALL NOT be treated as a guard

### Requirement: A guard is evaluated after its pattern matches and in the pattern binding scope
When a case pattern matches, the compiler SHALL evaluate that case's guard in the scope containing the pattern's bindings. The guard SHALL have type `Bool`. A guard that evaluates to false SHALL behave as if the case pattern did not match, and its bindings SHALL NOT escape to later cases. The guard SHALL be evaluated at most once for an attempted case, and the switch subject SHALL NOT be re-evaluated solely because a guard is present.

#### Scenario: A true guard selects the case body
- **WHEN** `case Some(x) if x > 100` matches a value whose payload is `101`
- **THEN** that case body SHALL execute

#### Scenario: A false guard falls through to the next case
- **WHEN** `case Some(x) if x > 100` matches a value whose payload is `50`, and a later `case Some(x)` exists
- **THEN** the later case SHALL be attempted and its binding SHALL be independent of the guarded case

#### Scenario: A non-Boolean guard is rejected
- **WHEN** a guarded case uses an `Int` or `String` expression as its guard
- **THEN** the compiler SHALL reject the program with a type error at the guard

#### Scenario: A guard is not evaluated when the pattern does not match
- **WHEN** a guarded variant case is tested against a different variant and the guard has an observable side effect
- **THEN** the guard SHALL NOT be evaluated

#### Scenario: The subject is evaluated once
- **WHEN** a switch subject is a call expression and multiple cases have guards
- **THEN** the call SHALL execute once for the switch, not once per guarded case

### Requirement: Guards work on any switch target and on wildcard cases
A guarded case SHALL be allowed for enum, built-in, and tuple switch targets. A wildcard case SHALL accept a guard. A guarded wildcard SHALL NOT act as an unconditional catch-all and SHALL NOT contribute exhaustiveness coverage.

#### Scenario: Guard on an integer switch
- **WHEN** an `Int` switch contains `case 0 if flag => 1` followed by `case _ => 0`
- **THEN** compilation SHALL succeed and the guarded case SHALL execute only when `flag` is true

#### Scenario: Guard on a tuple switch
- **WHEN** a tuple switch contains `case (Some(x), None) if x > 0 => x` followed by a fallback case
- **THEN** compilation SHALL succeed and a failing guard SHALL allow the fallback case to be attempted

#### Scenario: Guarded wildcard falls through
- **WHEN** an enum switch contains `case _ if flag => -1` followed by `case _ => 0`
- **THEN** `-1` SHALL be produced only when `flag` is true; otherwise the later wildcard SHALL produce `0`

#### Scenario: Guarded wildcard does not replace full coverage
- **WHEN** an enum switch covers only `Red` with an unguarded case and handles the remaining variants with `case _ if flag`
- **THEN** the compiler SHALL report the remaining variants as uncovered unless an unguarded wildcard or unguarded variant cases are present

### Requirement: Guard syntax is lossless and recoverable in parser2
Parser2 SHALL preserve every source byte of a guarded case, including the guard introducer, guard expression, terminator, body, comments, and trivia. The syntax tree SHALL represent the guard as a distinct structural region so lowering does not confuse it with the case body. Formatting SHALL preserve guard semantics and SHALL be idempotent. Malformed guard syntax SHALL produce a diagnostic while retaining the malformed source text for recovery.

#### Scenario: A guarded case round-trips losslessly
- **WHEN** parser2 parses source containing `case Some(x) if x > 100 then x end` with comments in the header
- **THEN** the syntax tree text SHALL reproduce the original source byte for byte

#### Scenario: Formatting a guarded case is idempotent
- **WHEN** the formatter formats a file containing guarded cases in `then` and `=>` forms
- **THEN** a second formatting pass SHALL produce the same text

#### Scenario: Malformed guard recovers
- **WHEN** a case contains a guard introducer without a guard expression
- **THEN** parser2 SHALL return a diagnostic and retain the malformed region in the syntax tree

### Requirement: Legacy pipeline rejects guard syntax explicitly
The bootstrap pipeline SHALL accept guarded cases. The legacy `--legacy sync` pipeline SHALL reject guard syntax with an explicit unsupported-feature diagnostic and SHALL NOT emit generated output that silently ignores the guard.

#### Scenario: Legacy sync rejects a guarded case
- **WHEN** a program containing `case P if cond => body` is processed by `--legacy sync`
- **THEN** the command SHALL fail with a diagnostic identifying case guards as unsupported by the legacy pipeline
