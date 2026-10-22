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

### Requirement: Leading pipe operators continue expressions across lines

Parser2 SHALL treat a line-leading `|>` or `<|` as continuation of the preceding expression regardless of indentation. The pipe levels SHALL parse `<|` as right-associative and looser than the left-associative `|>`, with both looser than `||`; simple `f <| x` and `x |> f` expressions SHALL keep their existing grouping. Intervening whitespace, blank lines and comments SHALL be preserved losslessly. Continuation SHALL NOT cross an explicit statement separator, declaration, branch or block boundary. Other line-leading operators SHALL retain their existing statement-boundary behavior.

#### Scenario: Both pipe operators may start continuation lines
- **WHEN** an expression is followed on the next line by `|>` or `<|` and a valid right operand, with equal or greater indentation
- **THEN** it parses as one expression equivalent to the corresponding single-line expression

#### Scenario: Mixed pipe chain retains grouping
- **WHEN** a multiline chain combines `|>`, `<|` and higher-precedence expressions
- **THEN** its AST has the same precedence and associativity as the same tokens written on one line, with chained `|>` grouping to the left and chained `<|` grouping to the right

#### Scenario: Chained backward pipe groups to the right
- **WHEN** source contains `f <| g <| x`
- **THEN** it lowers as `f <| (g <| x)` rather than `(f <| g) <| x`

#### Scenario: Backward pipe binds looser than forward pipe
- **WHEN** source contains `value |> f <| g`
- **THEN** it lowers as `(value |> f) <| g`, preserving both operator tokens and their source spans

#### Scenario: Comments and blank lines within a pipe chain
- **WHEN** whitespace, blank lines or comments separate an operand from a following line-leading pipe
- **THEN** the chain remains one expression and the CST reproduces all input bytes and source ranges

#### Scenario: Ordinary statements remain separate
- **WHEN** an expression is followed by an ordinary statement or a separate unary negative expression instead of a line-leading pipe
- **THEN** existing statement boundaries remain unchanged

#### Scenario: Continuation respects enclosing boundaries
- **WHEN** a pipe would need to cross a declaration, branch, block terminator or explicit statement separator to obtain its left operand
- **THEN** it does not attach to the expression before that boundary and malformed syntax is diagnosed

#### Scenario: Pipe without an operand is rejected
- **WHEN** a pipe has no left or right operand within its expression region
- **THEN** syntax parsing returns a located diagnostic while retaining malformed bytes, and semantic parsing rejects the source

#### Scenario: Existing trailing operator continuation
- **WHEN** a supported binary operator ends a line and its right operand follows on the next line
- **THEN** existing continuation behavior remains supported

### Requirement: Struct literal receivers retain their postfix chain

Parser2 SHALL continue parsing a postfix chain after a struct literal,
including a generic struct literal carrying explicit type arguments. When a
struct literal is followed by a field selection, method call, bracket suffix,
call, or cast, the result SHALL be one expression whose base is that struct
literal. The semantic AST and generated Go SHALL apply the suffix to the
constructed value, and syntax parsing SHALL NOT discard a trailing postfix
suffix or report it as an unrelated identifier error.

#### Scenario: Method call in an argument position
- **WHEN** a program calls a method on a struct literal directly inside a call
  argument, such as `fmt.Println(Point { x: 1, y: 2 }.Sum())`
- **THEN** compilation succeeds and the generated Go invokes the method
  implementation with the constructed value

#### Scenario: Method call on an assignment right-hand side
- **WHEN** a program binds the result of a method call on a struct literal,
  such as `let s = Point { x: 3, y: 4 }.Sum()`
- **THEN** the generated Go stores the method result rather than the bare
  struct literal

#### Scenario: Generic struct literal receiver
- **WHEN** a program calls a method on a generic struct literal with explicit
  type arguments, such as `Box[Int] { value: 42 }.Value()`
- **THEN** compilation succeeds and the generated Go invokes the generic
  method implementation with the constructed value

#### Scenario: Field selection after a struct literal
- **WHEN** a program selects a field from a struct literal, such as
  `Point { x: 1, y: 2 }.x`
- **THEN** the selection resolves to the field of the constructed value

#### Scenario: Postfix chain is retained losslessly
- **WHEN** a struct literal followed by a postfix suffix is syntax-parsed
- **THEN** every input byte is retained exactly once and the expression node
  spans both the literal and its postfix suffix
