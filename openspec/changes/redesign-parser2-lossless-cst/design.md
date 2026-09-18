## Context

See proposal.md - Why. parser2 currently parses directly to `ast2`, then
performs an independent character scan and reconstructs spans, delimiters and
layout events for the formatter. That scanner has already diverged from parser
literal rules, and formatter correctness depends on composing independently
derived line and event metadata. The bootstrap compiler consumes parser2 and
its MyGO-authored sources must remain synchronizable.

## Goals / Non-Goals

**Goals:**

- Establish one lexer-owned, lossless syntactic representation for parser2.
- Make recovered CST availability and strict AST/compiler failure compatible.
- Give formatter a structural, trivia-preserving input without heuristic
  layout-event reconstruction.
- Preserve valid-program `ast2` and public parser API compatibility during
  migration.

**Non-Goals:**

- Best-effort formatting, type inference, or code generation for invalid input.
- Incremental reparsing, editor/LSP protocol work, or changing MyGO grammar.
- Retaining old lossless metadata as a second formatter authority after the
  migration completes.

## Decisions

### D1: A single lexer emits the complete lossless token stream

The lexer emits significant tokens and trivia in source order, with byte
offsets and line/column ranges. Every input byte is represented once; comments
and all literal forms are lexed by this same authority rather than by a later
character scanner.

This eliminates lexer drift such as treating a backtick inside an ordinary
string as a raw-string delimiter. Reusing the old post-parse scanner is
rejected because it perpetuates two lexical definitions.

### D2: CST uses immutable green nodes with red Syntax views

The parser creates compact immutable green nodes whose children are nodes or
tokens. Syntax Node/Token views add parent navigation, offsets, ranges, and
typed accessors without copying the tree. A Token Tree is a view/projection of
this CST: delimiters and keyword-delimited blocks are represented by grammar
nodes, not merely paired punctuation.

A mutable AST-like CST is rejected because formatter traversal and later
incremental reuse need stable identity and inexpensive sharing. A delimiter-only
token tree is rejected because it cannot express `if`/`elsif`/`else`/`end` or
`switch`/`case` ownership.

### D3: Recovery is syntax-only; semantic lowering is all-or-nothing

Syntax parsing records diagnostics and inserts error nodes while synchronizing
at grammar-aware boundaries (top-level declaration starts, block branch/end
keywords, and delimiter boundaries). `ParseSyntax` returns this recovered tree.
`ParseFile` and compiler/formatter entry points first reject any diagnostic;
only diagnostic-free Syntax Trees lower to `ast2`.

Returning partial `ast2` is rejected: downstream inference/code generation
would have to distinguish recovery placeholders from real language constructs.
Failing at the first syntax error is rejected for the syntax API because it
prevents editor tooling and diagnostics from seeing following valid code.

### D4: Formatter traverses Syntax Tree and re-emits tokens

Formatter layout recursion owns complete CST nodes for blocks, branches and
delimited expressions. It preserves opaque token ranges (comments, literals,
inline Go) byte-for-byte and only normalizes trivia in explicitly formatable
gaps. Formatting remains gated on zero syntax diagnostics.

The existing `NodeSpan`/`DelimitedSpan`/`LayoutEvent` plus multi-pass line-map
pipeline is retained only as a temporary migration adapter. Continuing to make
it authoritative is rejected because it re-derives facts that CST already
contains.

### D5: Lowering owns semantic AST compatibility

Dedicated CST-to-ast2 lowering maps typed Syntax nodes to existing `ast2`
constructors and derives all semantic spans from CST ranges. Differential tests
compare valid-source AST shape, values and spans with the current parser before
switching public APIs.

Directly adapting existing parsec semantic actions to emit both AST and CST is
rejected: it keeps AST construction coupled to recovery and makes every grammar
alternative responsible for two representations.

## Risks / Trade-offs

- [Bootstrap self-hosting exposes unsupported new MyGO constructs] -> stage the
  CST model in bootstrap-compilable MyGO, synchronize after each coherent slice,
  and test generated parser2 immediately.
- [Recovery consumes a valid following construct] -> use explicit synchronization
  sets per grammar context and fixtures with multiple independent errors.
- [CST and ast2 behavior drift during migration] -> retain the existing parser
  as an oracle for valid-source differential tests until the cutover.
- [Formatter rewrite regresses established canonical output] -> migrate one
  construct family at a time with byte-identity and idempotence goldens.
- [Tree/token memory cost grows] -> use shared immutable children and ranges;
  measure representative parser2 and prelude files before removing adapters.

## Migration Plan

1. Introduce the lexer token/trivia model and coverage invariants behind a new
   syntax-parse API; keep existing AST parsing in production.
2. Implement CST grammar and recovery with focused syntax dumps and diagnostics
   tests, then add typed Syntax views.
3. Implement diagnostic-free CST-to-ast2 lowering and run valid-source
   differential tests before routing `ParseFile` through it.
4. Migrate formatter ownership by CST construct family, keeping existing output
   goldens and invalid-input failure behavior.
5. Remove post-parse scanning, reconstructed spans/events, and their formatter
   adapter only after the new path passes parser2, formatter, command, bootstrap
   synchronization, and full-project gates.

Rollback before step 5 is routing public entry points back to the legacy AST
parser and formatter adapter; CST remains an opt-in syntax API until parity is
established.
