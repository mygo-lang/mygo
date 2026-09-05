## Purpose

Provide consistent, automatable, editor-independent formatting for MyGO source so developers, scripts, and continuous integration can share the same formatting rules.

### Requirement: MyGO-authored formatter core

The formatter traversal and rendering behavior SHALL be authored in MyGO source and compiled into the repository's generated Go implementation. Go code SHALL provide only the public bridge and CLI/I/O integration, except where required by the existing generation workflow.

#### Scenario: Generated formatter implementation
- **WHEN** the formatter is built
- **THEN** its behavior comes from checked-in MyGO formatter source and the generated Go output is reproducible from that source

### Requirement: Preserve positioned source trivia

The formatter SHALL combine parser2/ast2 structure with positioned tokens and trivia. Comments, strings, inline Go, delimiters, protected whitespace, and source locations SHALL remain associated with their original source spans and SHALL NOT be reconstructed from abstract AST values alone.

#### Scenario: AST and trivia are combined
- **WHEN** valid source contains comments or protected literal content around nested syntax
- **THEN** formatting may change layout outside those spans while preserving the protected spans exactly

## ADDED Requirements

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
The formatter SHALL preserve comments, string contents, inline Go contents, and all semantic source constructs while changing only formatting representation.

#### Scenario: Source contains comments and strings
- **WHEN** source includes comments and strings containing whitespace-like characters
- **THEN** formatting preserves their contents and does not interpret them as formatter syntax

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
The formatter SHALL report parse errors with a useful source location and SHALL NOT return output that could be mistaken for a complete formatted file.

#### Scenario: Invalid source
- **WHEN** the input cannot be parsed as MyGO
- **THEN** the formatter returns an error identifying the failure location and no successful formatted result

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
