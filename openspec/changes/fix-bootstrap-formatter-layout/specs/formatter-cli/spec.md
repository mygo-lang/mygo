## ADDED Requirements

### Requirement: Verbatim raw multiline string rendering

The formatter SHALL reproduce every line of a triple-quoted (raw) multiline string literal verbatim, including each line's original leading whitespace, interior spacing, and the closing delimiter's position. The formatter SHALL NOT dedent, re-indent, trim, or otherwise rewrite any line inside the literal's span, regardless of the enclosing construct's indentation.

#### Scenario: Embedded Go code keeps relative indentation
- **WHEN** source contains a triple-quoted string whose lines are indented relative to each other (for example embedded Go statements inside an inline Go `code:` field)
- **THEN** every interior line of the literal appears in the formatted output byte-for-byte as in the source

#### Scenario: Reformatting a file that contains raw strings
- **WHEN** the formatter output of a source containing triple-quoted strings is formatted again
- **THEN** the second output is byte-for-byte equal to the first, and raw string lines are unchanged by both passes

### Requirement: Wrapped delimited constructs preserve source content

When the formatter expands an over-long delimited construct (struct literal, call argument list, or list) onto multiple lines, it SHALL preserve every source character inside the construct. Text that precedes an inner opening delimiter within an item — a field label such as `Span:` or a callee name such as `stateSpan(` — SHALL be retained. When multiple delimited constructs share one source line and are nested, the formatter SHALL expand only the outermost construct and reproduce inner constructs verbatim as part of their enclosing item's text; it SHALL NOT let the expansion of one construct delete or reorder the text of another construct on the same line.

A construct nested inside an expanded delimited item SHALL be reflowed recursively rather than left as raw text when it itself needs a form change: a keyword `if`/`case` value renders inline in `=>` form, and a value-position `switch`/`case` that must wrap renders in `then … end` block form. Leaving a keyword `if`/`case` verbatim is permitted only when it needs no form change, because parser2 marks keyword conditionals as always-expandable and a second pass would otherwise re-expand them and break idempotence.

#### Scenario: Nested struct and call in one over-long struct literal
- **WHEN** a single-line struct literal longer than the preferred width contains a field whose value is a nested struct, and that inner struct has a field whose value is a call such as `Span: stateSpan(state, reply.State)`
- **THEN** the formatted output keeps every field label (`Span:`, `Value:`, `State:`, `Error:`) and the callee name (`stateSpan`) intact, with their arguments/fields correctly nested, and no fragment such as a bare `state,`/`reply.State,`/`)` replaces `Span: stateSpan(...)`

#### Scenario: Reformatting a file with nested delimited constructs is idempotent
- **WHEN** the formatted output of a source containing nested single-line delimited constructs is formatted again
- **THEN** the second output is byte-for-byte equal to the first, with no deleted labels, callee names, or stray separator-only lines

### Requirement: Cross-pass coordinate correctness

The formatter applies several reflow passes (block, branch, case, delimited) to a single text that grows as each pass runs. When a later pass expands a construct located by its ORIGINAL source line, it SHALL read and rewrite the row that earlier upper expansions shifted that line to, rather than the original row. Formatting SHALL NOT drop content, relocate a construct onto an unrelated line, or emit separator-only (` , `) or close-delimiter-only (` ) `) fragment lines as a result of an expansion sitting above later expandable content.

#### Scenario: Expansion above later expandable content
- **WHEN** an `if`/`case`/block expansion appears above a later over-long delimited construct on a subsequent line
- **THEN** the later construct still expands correctly in place with no content loss, and the output contains no separator-only or close-delimiter-only fragment lines

#### Scenario: Reformatting is stable under stacked expansions
- **WHEN** the output of formatting a file that mixes upper expansions with later delimited constructs is formatted again
- **THEN** the second output is byte-for-byte equal to the first

### Requirement: Inline and block conditional/case form

The formatter SHALL render an `if` or `case` in `=>` arrow form when it stays on a single line and in `then … end` block form when it is wrapped onto multiple lines. An `if` used as an expression (value position) SHALL always retain an `else` branch. A keyword `if`/`case` used as a value inside a delimited construct that stays inline SHALL render in `=>` arrow form. Delimited constructs (`{}`, `[]`, `()`) SHALL add no comma after the last item when kept on one line, and SHALL add a comma after every item (including the last) when wrapped, except tuples.

#### Scenario: Inline keyword conditional nested in a wrapped literal renders as arrow
- **WHEN** a single-line struct literal is wrapped and one field's value is `if cond then a else b end` that fits on the field's line
- **THEN** that field renders as `field: if cond => a else b,` with no `then`/`end`

#### Scenario: Wide case body wraps to block with inline arrow cases
- **WHEN** a `case pat => body` is wide and its body contains a `switch` value with several `case pat => result` arms that each fit on one line
- **THEN** the outer case renders `case pat then … end`, the nested `switch` renders in block form, and each inner arm stays single-line `case pat => result`

### Requirement: Single-authority recursive wrapping is idempotent at file scale

The formatter SHALL wrap every over-long construct — delimited literals, value `switch`/`case`, value `if`, and statement/expression positions such as `let x = <over-long value>`, assignments, and bare call/ expression statements — through ONE recursive reflow pass that renders each affected source line in a single pass and locks its produced rows verbatim (baked canonical indentation). Because wrapping is recursive within that pass, a construct nested inside an over-long construct SHALL be wrapped in the SAME pass rather than left inline to be re-wrapped later; and because wrapped rows carry their final indentation, the wrapped-item indent SHALL NOT depend on cross-pass line-shift bookkeeping. Formatting SHALL therefore be byte-for-byte idempotent on large files with many stacked expansions.

#### Scenario: A statement whose value is an over-long nested construct wraps once and stays
- **WHEN** a statement line such as `let x = foo(bar(a, b), baz { … })` or a field value `Span: ps.SourceSpan { Start: …, End: … }` exceeds the preferred width, including when its nested sub-constructs also exceed it
- **THEN** the first format pass breaks the value into its canonical multi-line form with correct per-level indentation and correct trailing commas, and formatting that output again is byte-for-byte identical

#### Scenario: Wrapped-item indentation is stable as earlier content grows
- **WHEN** a file accumulates many expansions above a wrapped delimited construct (as in a full generated file)
- **THEN** the wrapped construct's item rows still carry exactly one additional indent level and its closing delimiter aligns with the construct's opening line, matching the same construct formatted in isolation

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
