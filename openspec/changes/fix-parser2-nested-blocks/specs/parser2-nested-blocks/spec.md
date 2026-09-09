## Purpose

Define reliable parser2 behavior for nested control-flow blocks so valid source preserves each construct's own terminator and is accepted without false missing-`end` errors.

## ADDED Requirements

### Requirement: Nested blocks preserve their own terminators

Parser2 SHALL parse nested `switch`, `if`, and `while` blocks as complete statements within an enclosing block, leaving the enclosing block's terminator available to its parent parser.

#### Scenario: Nested switch in a case-then body
- **WHEN** a `case ... then` body contains a complete nested `switch ... end` followed by the outer case's `end`
- **THEN** parser2 SHALL accept the source and associate each `end` with the corresponding block

#### Scenario: Nested conditional or loop in a case-then body
- **WHEN** a `case ... then` body contains a complete nested `if ... end` or `while ... end` followed by the outer case's `end`
- **THEN** parser2 SHALL accept the source without reporting the nested terminator as the outer block's terminator

#### Scenario: Missing nested terminator
- **WHEN** a nested block is not closed before end-of-input or the enclosing construct's boundary
- **THEN** parser2 SHALL report a parse error indicating the missing block terminator
