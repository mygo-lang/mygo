# parse-stage-performance Specification

## Purpose

Defines how the source-to-semantic-AST parse stage behaves as inputs grow, so
that compiling a large package stays predictable in time and stack use while
every existing diagnostic, CST byte, and AST identifier value stays unchanged.

## Requirements

### Requirement: Parse-stage time grows linearly with input size

Parsing a source file SHALL take time proportional to the number of tokens in
it. Doubling the size of a file whose shape is unchanged SHALL NOT more than
roughly double the parse time, and SHALL NOT degrade superlinearly as the
number of top-level declarations or statements increases.

#### Scenario: Doubling declarations stays proportional
- **WHEN** the same declaration shape is parsed at 375, 1500, and 6000
  top-level declarations
- **THEN** the parse time per declaration does not grow as declaration count
  grows

#### Scenario: Statement-heavy and declaration-heavy inputs both stay linear
- **WHEN** a file contains many small declarations and another file of similar
  size contains few declarations with long bodies
- **THEN** both parse in time proportional to their token counts

### Requirement: Flat source sequences parse in bounded stack

The parse stage SHALL complete without exhausting the goroutine stack for
inputs whose structure is flat rather than nested. Recursion depth SHALL depend
on syntactic nesting depth, not on the number of consecutive tokens,
declarations, statements, or sibling elements.

#### Scenario: A very large flat file parses
- **WHEN** a single source file contains about 12000 top-level declarations
  (roughly 600 KB) and no deep nesting
- **THEN** parsing completes and produces a lossless CST and semantic AST
  instead of failing with a stack overflow

#### Scenario: A long token stream parses
- **WHEN** a single file contains several hundred thousand tokens
- **THEN** lexing completes with stack use independent of the token count

### Requirement: Tail recursion is lowered regardless of member parameter lists

The compiler SHALL lower a tail-recursive component to a single constant-stack
trampoline when every member returns the same result types, shares the same type
parameters and `using` constraints, and every call inside the component is in
tail position, even when the members take different parameter lists. A
component whose members already share one parameter list SHALL keep its current
generated output unchanged.

#### Scenario: A component with mixed arities is trampolined
- **WHEN** a recursive component contains members taking one, three, four, and
  five parameters that all return the same result type and call each other in
  tail position
- **THEN** the generated Go contains one trampoline for the component, and no
  member call consumes an additional Go stack frame

#### Scenario: Arguments keep their evaluation order
- **WHEN** a lowered tail call passes arguments that read the caller's
  parameters
- **THEN** every argument is evaluated once, left to right, before any
  parameter slot is overwritten

#### Scenario: Equal-parameter components are untouched
- **WHEN** the compiler regenerates a package whose components already shared
  one parameter list
- **THEN** the generated Go for those trampolines and their wrappers is
  byte-identical to the previous output

### Requirement: AST identifier values are preserved exactly

Assigning identifiers to declarations, statements, expressions, and cases SHALL
produce the same numeric values, in the same traversal order, as the current
implementation. Existing generated Go for any accepted source SHALL remain
unchanged.

#### Scenario: Identical identifiers after the change
- **WHEN** any source file accepted today is parsed by the changed compiler
- **THEN** every AST node carries the identifier value it carried before, and
  code generated from it is byte-identical

### Requirement: Observable parse results are unchanged

Diagnostics SHALL keep their message text, span, and ordering, the CST SHALL
keep its byte-for-byte lossless property, and lowering SHALL produce the same
semantic AST shape, including for malformed input that requires recovery.

#### Scenario: Differential corpus stays equal
- **WHEN** the repository corpus is parsed through the changed pipeline and
  compared against the legacy parser
- **THEN** every file that previously agreed still agrees, and no file that
  previously parsed now fails

#### Scenario: Formatting stays idempotent
- **WHEN** the formatter runs over already formatted sources
- **THEN** output is unchanged and the byte-preservation checks pass

### Requirement: Parse-stage performance is measured

The repository SHALL provide benchmarks for lexing, syntax parsing, and
lowering at three input sizes, reporting time and allocation counts, so that
regressions are visible before and after each optimization.

#### Scenario: Benchmarks report cost per size
- **WHEN** the parser benchmarks run with allocation reporting enabled
- **THEN** each size reports nanoseconds per operation, bytes per operation,
  and allocations per operation
