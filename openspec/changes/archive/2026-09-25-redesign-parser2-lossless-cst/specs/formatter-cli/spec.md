## MODIFIED Requirements

### Requirement: Preserve positioned source trivia

The formatter SHALL consume parser2's lossless Syntax Tree and its positioned
tokens and trivia. Comments, strings, inline Go, delimiters, protected
whitespace, and source locations SHALL remain associated with their original
source spans and SHALL NOT be reconstructed from abstract AST values alone.

#### Scenario: AST and trivia are combined
- **WHEN** valid source contains comments or protected literal content around
  nested syntax
- **THEN** formatting may change layout outside those spans while preserving
  the protected spans exactly

### Requirement: Lossless parser source model

Parser2 SHALL expose a lossless CST and navigable Syntax Tree whose positioned
tokens and trivia retain raw source text plus start and end positions. The CST
SHALL be the formatter's structural source of truth, and valid AST nodes SHALL
retain complete source spans sufficient to associate semantic nodes with the
Syntax Tree. The AST-only parse API SHALL remain unchanged.

#### Scenario: Parser preserves source ranges
- **WHEN** parser2 parses valid source containing comments, strings, inline Go,
  and nested expressions
- **THEN** the Syntax Tree contains the original token/trivia spelling and
  ranges without reconstructing protected text from AST values

#### Scenario: Nested type and pattern spans
- **WHEN** parser2 parses nested generic types, tuple patterns, or variant
  patterns
- **THEN** the Syntax Tree contains a structural node for each nested syntax
  construct and its source range

### Requirement: Generic syntax-driven formatting

The formatter SHALL select layout from Syntax Tree node kinds, child structure,
rendered width, and source spans. It SHALL use the same traversal rules for all
functions, function literals, calls, and expressions, without matching
specific function names, method names, or body text.

The Syntax Tree renderer SHALL handle declarations and statement bodies,
function literals, nested expressions and control-flow blocks, and delimited
groups through CST ownership. Delimited layout SHALL measure the rendered
construct recursively, wrap separators and nested fields deterministically,
and keep closing delimiters aligned with their owning construct. The renderer
SHALL preserve comments and opaque source ranges byte-for-byte while
normalizing only formatable trivia. The legacy multi-pass renderer and its
reconstructed span, event, and line-mapping inputs SHALL be removed only after
the Syntax Tree renderer passes the existing formatter golden corpus and
byte-identity/idempotence checks.

#### Scenario: Arbitrary function literals
- **WHEN** two function literals have different names, parameters, return
  types, or body expressions
- **THEN** both are formatted by the same generic Syntax Tree traversal and
  protected spans remain exact

#### Scenario: Nested wide delimited constructs
- **WHEN** valid source contains nested calls, collections, or struct literals
  whose rendered width exceeds the formatter limit
- **THEN** the formatter wraps the owned delimiter groups recursively, keeps
  nested indentation and closing delimiters canonical, and produces the same
  result on a second formatting pass

#### Scenario: Existing canonical layouts remain stable
- **WHEN** the Syntax Tree renderer formats the supported formatter fixture
  corpus
- **THEN** declaration spacing, inline and block conditionals, function
  literals, long signatures, and delimited wrapping match their canonical
  goldens before the legacy renderer is removed

### Requirement: Syntax-owned layout structure

The formatter SHALL obtain block, branch, and delimiter boundaries by
traversing parser2 Syntax Tree structure. It SHALL NOT require a separately
reconstructed layout-event stream or source-line matching to identify `case`,
`if`, `elsif`, `else`, `then`, or `end` boundaries.

#### Scenario: Nested branch layout
- **WHEN** source contains nested `if`/`else` or `switch`/`case` constructs
- **THEN** the formatter obtains their boundaries from Syntax Tree nodes and
  tokens and places the formatted branches deterministically

### Requirement: Report invalid input safely

The formatter SHALL report parser syntax diagnostics with a useful source
location and SHALL NOT return output that could be mistaken for a complete
formatted file.

#### Scenario: Invalid source
- **WHEN** syntax parsing returns one or more diagnostics for the input
- **THEN** the formatter returns an error identifying a diagnostic location and
  no successful formatted result
