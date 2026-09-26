# parser2-lossless-cst Specification

## Purpose

Provide parser2 with a single lossless syntactic source of truth that preserves
all input text, supports useful recovery diagnostics, and lowers valid MyGO
programs to the existing semantic AST without weakening compiler correctness.

## Requirements

### Requirement: Parser2 produces a lossless concrete syntax tree

Parser2 SHALL expose a lossless CST for every input through a syntax-parse API.
The CST SHALL preserve every input byte exactly once through ordered tokens and
trivia, and every token/trivia item SHALL retain its raw spelling and source
range. The CST SHALL represent grammar structure, including declarations,
expressions, types, patterns, delimited constructs, and keyword-delimited
blocks.

#### Scenario: Valid source preserves comments and literal boundaries
- **WHEN** source contains comments, ordinary strings, raw strings,
  triple-quoted strings, inline Go, and nested delimiters
- **THEN** its CST tokens and trivia reproduce the original source byte for
  byte and retain their original source ranges

#### Scenario: Block structure is represented directly
- **WHEN** source contains nested `if`/`elsif`/`else`/`end` and
  `switch`/`case`/`end` constructs
- **THEN** the CST contains structural nodes for those constructs and their
  delimiters without requiring formatter-side source-line matching

### Requirement: Syntax and Token Tree navigation is stable

Parser2 SHALL expose navigable Syntax Node and Syntax Token views over the
lossless CST. A view SHALL provide node kind, ordered children, parent or
sibling navigation where applicable, and the source range occupied by the
view. Trivia SHALL remain associated with the token sequence it surrounds.

#### Scenario: Nested source can be navigated by range and structure
- **WHEN** a consumer obtains the Syntax view for a nested call inside a
  function body
- **THEN** it can navigate to the enclosing body and its ordered child tokens
  and obtain source ranges consistent with the original text

### Requirement: Syntax parsing recovers malformed regions

The syntax-parse API SHALL return a lossless CST and one or more diagnostics
for malformed source when recovery can continue. Recovered malformed text
SHALL be retained in error nodes or tokens, and valid declarations following
a recovered region SHALL remain available in source order.

#### Scenario: Recovery retains a later declaration
- **WHEN** a declaration has a missing separator or terminator followed by a
  syntactically valid top-level declaration
- **THEN** syntax parsing returns diagnostics and a CST containing both the
  malformed region and the later declaration

#### Scenario: Recovery preserves malformed literal text
- **WHEN** a source literal is malformed
- **THEN** syntax parsing returns a diagnostic and retains the literal's input
  bytes in the CST rather than reconstructing or discarding them

### Requirement: Semantic parse APIs reject syntax diagnostics

The existing AST-producing parser APIs and compiler-facing parse entry points
SHALL return a parse error and SHALL NOT return an `ast2.File` if syntax
parsing reports any syntax diagnostic. A successful AST result SHALL be
lowered from the lossless Syntax Tree and retain source spans for its semantic
nodes.

#### Scenario: Compiler input has a recoverable syntax error
- **WHEN** source has a recoverable syntax error
- **THEN** the syntax-parse API can expose its recovered CST, while the
  AST-producing parser API returns an error with a useful source location

#### Scenario: Valid source retains AST compatibility
- **WHEN** valid source is parsed through the existing AST-producing API
- **THEN** it returns an AST equivalent to the supported parser2 language
  semantics and source spans point into the original source
