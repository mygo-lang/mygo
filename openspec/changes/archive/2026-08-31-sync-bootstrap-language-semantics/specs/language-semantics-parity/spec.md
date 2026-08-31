## Purpose

Defines source-language behavior that both the production and self-hosted
bootstrap compilers must implement consistently for portable MyGO programs.

## ADDED Requirements

### Requirement: Shared language conformance
The production compiler and the bootstrap compiler SHALL accept or reject the
same in-scope MyGO source programs and SHALL preserve the same observable
source semantics. Differences in generated Go identifiers or internal lowering
strategies SHALL NOT constitute a language-level difference.

#### Scenario: A shared conformance fixture is compiled
- **WHEN** an in-scope fixture is compiled by both pipelines
- **THEN** both compilations SHALL either succeed with equivalent runtime
  behavior or fail with a diagnostic for the same source-language violation

### Requirement: String literal semantics
Double-quoted strings SHALL process the documented escape sequences.
Triple-quoted and backtick-quoted strings SHALL preserve all enclosed
characters verbatim, including backslashes and newlines, until their closing
delimiter. This includes triple-quoted values used by inline Go `code` fields.

#### Scenario: A triple-quoted string contains an escape
- **WHEN** a triple-quoted string contains `\\n`
- **THEN** its runtime value SHALL contain the two literal characters `\\` and
  `n` at that point

#### Scenario: A raw string spans lines
- **WHEN** a backtick-quoted raw string contains a newline before its closing
  backtick
- **THEN** compilation SHALL succeed and the runtime value SHALL preserve that
  newline and every backslash verbatim

### Requirement: Tuple binding patterns
Tuple bindings introduced by `let` SHALL support recursively nested tuple
patterns and `_` discard slots. The right-hand expression SHALL be evaluated
once, and every named leaf SHALL receive its corresponding tuple element.

#### Scenario: A nested binding discards an element
- **WHEN** source contains `let (a, (_, c)) = value`
- **THEN** `a` and `c` SHALL be bound to their matching leaves and no binding
  named `_` SHALL be introduced

### Requirement: Switch patterns in statement context
A switch expression whose result is discarded SHALL support every pattern form
supported in value context, including wildcard, binding, literal, tuple, and
enum-variant patterns.

#### Scenario: A literal statement switch is used for effects
- **WHEN** a discarded switch expression has literal cases whose bodies perform
  effects
- **THEN** compilation SHALL succeed and only the matching case body SHALL run

### Requirement: Loop-control statements
MyGO SHALL support `break` and `continue` as statements within a `while` body.
`break` SHALL exit the nearest enclosing loop and `continue` SHALL begin its
next iteration. Each compiler SHALL reject either statement when it is outside
of a loop.

#### Scenario: A loop continues and then breaks
- **WHEN** a while body uses `continue` for one iteration and `break` later
- **THEN** the skipped iteration SHALL not execute its remaining statements and
  the loop SHALL terminate at the break

#### Scenario: Loop control appears outside a loop
- **WHEN** `break` or `continue` appears outside every while body
- **THEN** compilation SHALL fail with a source-location diagnostic
