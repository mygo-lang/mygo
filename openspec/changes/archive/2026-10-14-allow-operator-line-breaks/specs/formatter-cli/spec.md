## ADDED Requirements

### Requirement: Preserve authored pipe line breaks

The formatter SHALL preserve whether an authored pipe boundary breaks before or after `|>` or `<|`, even when the expression fits within 100 columns. It SHALL normalize continuation indentation to two spaces beyond the owning statement or expression item, with successive chain continuations aligned. Comments SHALL retain their text and source order; blank-line counts may be normalized.

#### Scenario: Leading pipe layout is preserved
- **WHEN** valid source contains an expression followed by line-leading pipes
- **THEN** formatting retains those leading pipe boundaries and aligns continuation lines with one additional indentation level

#### Scenario: Trailing pipe layout is preserved
- **WHEN** valid source breaks after a pipe operator
- **THEN** formatting retains the break after that operator rather than collapsing the chain

#### Scenario: Short inline pipe remains compact
- **WHEN** a pipe expression has no authored pipe breaks and fits within the preferred width
- **THEN** formatting keeps it inline unless another existing layout constraint requires expansion

#### Scenario: Pipe comments remain intact
- **WHEN** comments occur around authored pipe continuation boundaries
- **THEN** formatting preserves their text and order without allowing a line comment to swallow an operand

### Requirement: Wrap wide binary expressions after operators

The formatter SHALL wrap over-wide single-line binary expressions at legal boundaries after supported arithmetic, comparison, logical or pipe operators, using a preferred width of 100 columns including indentation and enclosing prefixes. It SHALL prefer lower-precedence boundaries and use deterministic source-order choices, preserve precedence, associativity, parentheses and operand order, and integrate recursively with existing delimited layout. It SHALL NOT split unary operators, tokens or protected literal/inline-Go contents. Unbreakable fragments may exceed the preferred width. Output SHALL remain parseable, semantically equivalent and byte-for-byte idempotent.

#### Scenario: Long arithmetic and logical expressions wrap automatically
- **WHEN** a single-line arithmetic, comparison or logical expression exceeds 100 columns and has legal binary breakpoints
- **THEN** formatting inserts breaks after binary operators and indents continuation lines by one level

#### Scenario: Long inline pipe wraps after its operators
- **WHEN** an inline pipe chain exceeds the preferred width without authored pipe breaks
- **THEN** automatic pipe breaks occur after operators and the resulting layout is stable

#### Scenario: Prefix and nesting contribute to width
- **WHEN** a binary expression occurs in a binding, assignment, return, condition, argument or field value
- **THEN** its available width accounts for indentation and enclosing prefixes, and nested constructs use compatible recursive layouts

#### Scenario: Mixed precedence remains equivalent
- **WHEN** formatting wraps mixed-precedence expressions, explicit parentheses or mixed pipe directions
- **THEN** reparsing produces an equivalent expression structure without reordering or regrouping operations

#### Scenario: Protected fragments exceed the preferred width
- **WHEN** a long token, string or inline-Go fragment cannot be safely broken
- **THEN** it remains intact even if its line exceeds 100 columns

#### Scenario: Existing pipe breaks and automatic wrapping coexist
- **WHEN** an authored multiline pipe chain still has over-wide operand expressions
- **THEN** authored pipe boundaries remain preserved while legal additional breaks are inserted after binary operators

#### Scenario: Reformatting wrapped expressions is stable
- **WHEN** source containing wrapped expressions, comments and nested constructs is formatted a second time
- **THEN** the output is byte-for-byte identical to the first result
