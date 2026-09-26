# formatter-cli Specification

## Purpose

Provide consistent, automatable, editor-independent formatting for MyGO source so developers, scripts, and continuous integration can share the same formatting rules.

## Requirements

### Requirement: MyGO-authored formatter core

The formatter traversal and rendering behavior SHALL be authored in MyGO source and compiled into the repository's generated Go implementation. Go code SHALL provide only the public bridge and CLI/I/O integration, except where required by the existing generation workflow.

#### Scenario: Generated formatter implementation
- **WHEN** the formatter is built
- **THEN** its behavior comes from checked-in MyGO formatter source and the generated Go output is reproducible from that source

### Requirement: Preserve positioned source trivia

The formatter SHALL consume parser2's lossless Syntax Tree and its positioned tokens and trivia. Comments, strings, inline Go, delimiters, protected whitespace, and source locations SHALL remain associated with their original source spans and SHALL NOT be reconstructed from abstract AST values alone.

#### Scenario: AST and trivia are combined
- **WHEN** valid source contains comments or protected literal content around nested syntax
- **THEN** formatting may change layout outside those spans while preserving the protected spans exactly

### Requirement: Lossless parser source model

Parser2 SHALL expose a lossless CST and navigable Syntax Tree whose positioned tokens and trivia retain raw source text plus start and end positions. The CST SHALL be the formatter's structural source of truth, and valid AST nodes SHALL retain complete source spans sufficient to associate semantic nodes with the Syntax Tree. The AST-only parse API SHALL remain unchanged.

#### Scenario: Parser preserves source ranges
- **WHEN** parser2 parses valid source containing comments, strings, inline Go, and nested expressions
- **THEN** the Syntax Tree contains the original token/trivia spelling and ranges without reconstructing protected text from AST values

#### Scenario: Nested type and pattern spans
- **WHEN** parser2 parses nested generic types, tuple patterns, or variant patterns
- **THEN** the Syntax Tree contains a structural node for each nested syntax construct and its source range

### Requirement: Generic syntax-driven formatting

The formatter SHALL select layout from Syntax Tree node kinds, child structure, rendered width, and source spans. It SHALL use the same traversal rules for all functions, function literals, calls, and expressions, without matching specific function names, method names, or body text.

The Syntax Tree renderer SHALL handle declarations and statement bodies, function literals, nested expressions and control-flow blocks, and delimited groups through CST ownership. Delimited layout SHALL measure the rendered construct recursively, wrap separators and nested fields deterministically, and keep closing delimiters aligned with their owning construct. The renderer SHALL preserve comments and opaque source ranges byte-for-byte while normalizing only formatable trivia. The legacy multi-pass renderer and its reconstructed span, event, and line-mapping inputs SHALL be removed only after the Syntax Tree renderer passes the existing formatter golden corpus and byte-identity/idempotence checks.

#### Scenario: Arbitrary function literals
- **WHEN** two function literals have different names, parameters, return types, or body expressions
- **THEN** both are formatted by the same generic Syntax Tree traversal and protected spans remain exact

#### Scenario: Nested wide delimited constructs
- **WHEN** valid source contains nested calls, collections, or struct literals whose rendered width exceeds the formatter limit
- **THEN** the formatter wraps the owned delimiter groups recursively, keeps nested indentation and closing delimiters canonical, and produces the same result on a second formatting pass

#### Scenario: Existing canonical layouts remain stable
- **WHEN** the Syntax Tree renderer formats the supported formatter fixture corpus
- **THEN** declaration spacing, inline and block conditionals, function literals, long signatures, and delimited wrapping match their canonical goldens before the legacy renderer is removed

### Requirement: Syntax-owned layout structure

The formatter SHALL obtain block, branch, and delimiter boundaries by traversing parser2 Syntax Tree structure. It SHALL NOT require a separately reconstructed layout-event stream or source-line matching to identify `case`, `if`, `elsif`, `else`, `then`, or `end` boundaries.

#### Scenario: Nested branch layout
- **WHEN** source contains nested `if`/`else` or `switch`/`case` constructs
- **THEN** the formatter obtains their boundaries from Syntax Tree nodes and tokens and places the formatted branches deterministically

### Requirement: Deterministic source formatting
The formatter SHALL accept valid MyGO source text and produce deterministic formatted source text without changing program semantics.

#### Scenario: Formatting valid source
- **WHEN** valid `.mygo` source is passed to the formatter
- **THEN** it returns formatted source using one stable layout for the same input

The formatter SHALL prefer lines no wider than 100 columns and SHALL choose a multiline representation when a compact expression would exceed that width.

#### Scenario: Long expression
- **WHEN** a declaration or control-flow expression exceeds the preferred line width
- **THEN** the formatter emits a stable multiline layout rather than an excessively wide single line

#### Scenario: Formatting is idempotent
- **WHEN** formatted output is passed to the formatter again
- **THEN** the second output is byte-for-byte equal to the first output

### Requirement: Preserve source content

The formatter SHALL preserve comments, string contents, inline Go contents, and all semantic source constructs while changing only formatting representation. For multiline string literals this SHALL include each line's original leading whitespace, so that the literal's rendered lines are byte-for-byte identical to the source lines within the literal's span.

#### Scenario: Source contains comments and strings
- **WHEN** source includes comments and strings containing whitespace-like characters
- **THEN** formatting preserves their contents and does not interpret them as formatter syntax

#### Scenario: Raw string inside indented construct
- **WHEN** a triple-quoted multiline string appears inside an inline Go block or other indented construct
- **THEN** the formatter preserves the literal's interior lines exactly as authored, even when the enclosing construct's indentation changes

### Requirement: Format conditional statements consistently

The formatter SHALL use compact arrow syntax for single-line `if` statements and strict `then` block syntax for multiline `if` statements. A multiline `if` SHALL place each branch body on its own indented line and place `end` on its own line.

#### Scenario: Single-line if
- **WHEN** an `if` statement and its branches fit on one line
- **THEN** the formatter emits `if cond => xxx else yyy`

#### Scenario: Multiline if
- **WHEN** an `if` statement contains a multiline body or branch
- **THEN** the formatter emits a multiline `then` block with line breaks before and after `else`, and before `end`

### Requirement: Format switch cases consistently

The formatter SHALL use `case XXX => xxx` for a single-line case and SHALL use `case XXX then` followed by a multiline case body for a multiline case. It SHALL NOT emit arrow syntax for a multiline case, even when that syntax is accepted by the parser.

#### Scenario: Single-line case
- **WHEN** a case body fits on one line
- **THEN** the formatter emits `case XXX => xxx`

#### Scenario: Multiline case
- **WHEN** a case body contains multiple lines or statements
- **THEN** the formatter emits `case XXX then`, places each body statement on its own line, and places `end` on its own line

### Requirement: Report invalid input safely
The formatter SHALL report parser syntax diagnostics with a useful source location and SHALL NOT return output that could be mistaken for a complete formatted file.

#### Scenario: Invalid source
- **WHEN** syntax parsing returns one or more diagnostics for the input
- **THEN** the formatter returns an error identifying a diagnostic location and no successful formatted result

### Requirement: Format files from the command line
The `mygo fmt` command SHALL format one or more `.mygo` files, writing formatted content back to files by default and returning a non-zero exit status if formatting fails.

#### Scenario: Format a file in place
- **WHEN** the user runs `mygo fmt path/to/file.mygo` on valid input
- **THEN** the file is replaced with formatted content and the command exits successfully

#### Scenario: Multiple files
- **WHEN** the user supplies multiple `.mygo` file paths
- **THEN** each file is processed independently and failures are reported with their paths

### Requirement: Support standard input and check mode
The `mygo fmt` command SHALL support formatting standard input to standard output and SHALL provide a check mode that reports unformatted input without modifying files.

#### Scenario: Standard input
- **WHEN** formatted input is supplied through standard input
- **THEN** the command writes formatted source to standard output without requiring a source file path

#### Scenario: Check mode detects changes
- **WHEN** check mode is run against a valid file whose contents differ from formatter output
- **THEN** the command reports the file as unformatted, leaves it unchanged, and exits non-zero

#### Scenario: Check mode accepts formatted files
- **WHEN** check mode is run against a file already equal to formatter output
- **THEN** the command reports success and does not modify the file

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
