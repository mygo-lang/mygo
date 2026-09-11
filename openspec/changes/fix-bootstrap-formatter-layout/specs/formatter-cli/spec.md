## ADDED Requirements

### Requirement: Verbatim raw multiline string rendering

The formatter SHALL reproduce every line of a triple-quoted (raw) multiline string literal verbatim, including each line's original leading whitespace, interior spacing, and the closing delimiter's position. The formatter SHALL NOT dedent, re-indent, trim, or otherwise rewrite any line inside the literal's span, regardless of the enclosing construct's indentation.

#### Scenario: Embedded Go code keeps relative indentation
- **WHEN** source contains a triple-quoted string whose lines are indented relative to each other (for example embedded Go statements inside an inline Go `code:` field)
- **THEN** every interior line of the literal appears in the formatted output byte-for-byte as in the source

#### Scenario: Reformatting a file that contains raw strings
- **WHEN** the formatter output of a source containing triple-quoted strings is formatted again
- **THEN** the second output is byte-for-byte equal to the first, and raw string lines are unchanged by both passes

## MODIFIED Requirements

### Requirement: Parser-owned layout events

The lossless parser SHALL expose nested layout events derived from AST spans. Events SHALL retain structural paths, nesting depth, node spans, and positioned-token anchors for branch headers such as `case` and `else`. The formatter SHALL use these events for block and branch boundaries instead of inferring structure from source-line string matching.

The parser SHALL NOT omit an event for a structurally parsed construct because its anchor cannot be verified; anchor resolution SHALL trace the construct's actual boundaries (for example, a `then` branch of an `if` closes at the enclosing `if`'s own `end`, and a `case ... then ... end` body inside `switch` closes at its own `end` without consuming sibling or enclosing terminators).

#### Scenario: Anchored branch layout
- **WHEN** source contains nested `if/else` or `switch/case` constructs
- **THEN** the lossless result contains enter/exit events and header token anchors that allow the formatter to place branch boundaries deterministically

#### Scenario: Case body with then block inside switch
- **WHEN** source contains `case <pattern> then\n  <body>\nend` inside a `switch`
- **THEN** the events place the case body one indent level deeper than the `case` header, keep the case's `end` aligned with the `case` header, keep every `case` aligned one level inside `switch`, and consume no extra `end` beyond the case body's own

#### Scenario: Wide single-line conditional keeps expansion events
- **WHEN** a single-line `if ... => ... else ...` or `case ... => ...` construct exceeds the preferred line width
- **THEN** the lossless result still carries the branch events the formatter uses to convert the construct to block form

### Requirement: Preserve source content

The formatter SHALL preserve comments, string contents, inline Go contents, and all semantic source constructs while changing only formatting representation. For multiline string literals this SHALL include each line's original leading whitespace, so that the literal's rendered lines are byte-for-byte identical to the source lines within the literal's span.

#### Scenario: Source contains comments and strings
- **WHEN** source includes comments and strings containing whitespace-like characters
- **THEN** formatting preserves their contents and does not interpret them as formatter syntax

#### Scenario: Raw string inside indented construct
- **WHEN** a triple-quoted multiline string appears inside an inline Go block or other indented construct
- **THEN** the formatter preserves the literal's interior lines exactly as authored, even when the enclosing construct's indentation changes
