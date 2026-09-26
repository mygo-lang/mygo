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

The existing formatter pipeline (`NodeSpan`/`DelimitedSpan`/`LayoutEvent` plus
multi-pass line and event remapping) is retained only as a temporary behavioral
reference during migration. The new formatter renders directly from Syntax
Tree nodes and tokens; it does not reconstruct legacy spans or events as an
intermediate representation. The initial Syntax Tree renderer is a structural
foundation, not yet a replacement for the complete formatter: before removal,
it must reproduce the canonical layout behavior across the supported fixture
corpus. This includes declaration spacing, function and function-literal
bodies, embedded and standalone conditional/switch/while blocks, recursively
measured nested delimiters, long signatures, and nested field wrapping.

The renderer should format from concrete CST ownership. A delimiter node owns
its opening and closing tokens, direct separators, and nested item nodes; width
measurement recursively uses the same renderer rules as emission. Wrapped
items increase indentation by their delimiter depth, and a closing delimiter
aligns with the construct's opening anchor. Function literals and control-flow
expressions use the same node-driven traversal wherever they occur, including
inside calls, collections, and declaration initializers. Formatable whitespace
is normalized at node boundaries; comments and opaque token ranges remain
byte-preserved and constrain line reflow around them.

The legacy renderer remains as a behavioral reference until these renderer
families pass the existing formatter goldens, protected-span checks, and
format-twice byte identity/idempotence checks. Only then remove the old
multi-pass implementation and its `NodeSpans`/`Delimited`/`LayoutEvents`, line
mapping, and event remapping consumers together. Do not use those structures as
an intermediate input to the Syntax Tree renderer.

Section 4 begins by giving the formatter its own ordered traversal over green
elements rather than over reconstructed spans.  `FormatterCursor`
(`formatter_cursor.mygo`) is a bounded reader of one `Slice[GreenElement]` with
an explicit `Index`; it never mutates or drops source, so a caller can still
slice the original range for protected spans after skipping.  Every trivia
decision routes through the single explicit `greenElementIsTrivia(item)`
predicate, and two skipping modes are distinguished: `SkipTrivia` ignores all
trivia for grammar decisions, while `SkipFormatableTrivia` stops at opaque
trivia.  Only formatable trivia (whitespace, via
`greenElementIsFormatableTrivia`) is a normalization gap the formatter may
discard and re-emit canonically; any other trivia is opaque and is replayed
byte-for-byte through `FormatterOpaqueTriviaRaw`.  This keeps the later
delimiter/block migrations (4.3-4.6) from re-introducing a private notion of
what may be normalized.

Task 4.2 widens "opaque source" from opaque trivia to every lexical form the
formatter must replay byte-for-byte.  `greenElementIsOpaqueSource` is the single
union predicate: formatable whitespace is the only element the formatter may
discard and re-emit canonically, while opaque trivia (comments), the literal
token kinds whose exact spelling carries meaning (ordinary, raw, and
triple-quoted strings plus rune literals), and inline-Go ranges are opaque.
`FormatterOpaqueSourceRaw` returns the exact replay text for an opaque element
and `None` otherwise.  Decoding a literal and re-encoding it is rejected because
it silently drops delimiter and escape spelling that the source already records,
and the inline-Go payload is not MyGO syntax at all, so it can never be
regenerated from decoded AST values.

Task 4.3 moves delimiter layout authority from the reconstructed ast2 token scan
to direct Syntax Tree traversal. The CST already encodes grouping - a nested
delimiter is a child node rather than a raw token - so a top-level separator is
exactly a direct `,` leaf of the owned group node, and string/rune interiors
never appear as delimiter leaves. The reconstruction counted raw delimiter tokens and
re-scanned literals to reach the same conclusion; reading it from the tree
removes that second authority.  The reported kinds deliberately match the legacy
frontier: calls, tuples, slices, sets, and struct literals are reported while
parenthesized expressions (`(x)`), the unit value (`()`), map literals, and
generic type-argument groups stay on the plain expression route.  The
`DelimitedSpan` byte spans (delimiter range, item ranges, separator ranges) are
pinned by MyGO regressions in `delimited_cst_test.mygo` against explicit
goldens, and `ParseFileLossless` now derives `LosslessFile.Delimited` from
`SyntaxTree.Root` instead of the retired scan.

### D5: Lowering owns semantic AST compatibility

Dedicated CST-to-ast2 lowering maps typed Syntax nodes to existing `ast2`
constructors and derives all semantic spans from CST ranges. Differential tests
compare valid-source AST shape, values and spans with the current parser before
switching public APIs.

Directly adapting existing parsec semantic actions to emit both AST and CST is
rejected: it keeps AST construction coupled to recovery and makes every grammar
alternative responsible for two representations.

The completed-corpus differential gate (task 3.10) drives the full valid-source
set under both the CST lowerer and the legacy parser and compares AST shape,
literal values, declaration membership, and spans.  The gate currently reports
62 files parsed by the legacy parser, 62 equal, 0 different.  Every difference
found during the gate was resolved inside the CST/parser2 path rather than by an
exemption, because each was a lowering defect, not an intentional divergence:

- Assignments nested inside a `while`/`if`/`elsif`/`else` body are still raw `=`
  leaves when the branch wrapper runs; the block pass now finds the top-level `=`
  (tracking delimiter depth and skipping function-literal bodies) and
  synthesizes an `AssignStatement` instead of leaving the run as raw leaves.
- A bare body expression whose trailing token is a binary operator continues on
  the next line; statement splitting now treats a trailing operator as a
  continuation rather than a statement boundary.
- A keyword block (`switch`, `if`, `while`) used as a call argument is passed
  through with its concrete node kind instead of being wrapped in a generic
  `Expression` node that lowering skips.
- Escaped rune and string literal patterns decode backslash escapes so a
  `case '\n'` / `case "\n"` pattern lowers to the character or string the legacy
  parser stores, not the raw source bytes.

The public API cutover (task 3.11) surfaced two more span-alignment differences
that were fixed in the CST/lowering path rather than exempted:

- An `elsif` branch lowers to a nested `IfExpr` in the else slot; its span now
  starts at the lowered condition instead of the `elsif` keyword, matching the
  legacy `exprWithPos(cond.Pos, ...)` start.
- A branch body block starts at its first statement token, not at the header
  keyword's trailing trivia; the leading trivia stays a sibling of the Block so
  the branch node still owns every source byte while the lowered Block range
  matches the legacy body span.

The legacy-grammar isolation (task 3.12) makes the cutover structural rather
than a mere runtime preference.  The production parse APIs (`ParseFile`,
`ParseFileAt`) now depend only on the CST/lowering path and the four shared
helpers (`emptySpan`, `isKeyword`, `defaultImportAlias`,
`syntaxDiagnosticError`).  The pre-cutover parsec grammar, its
`spanned*`/`stateSpan`/`Spanned` helpers, and the differential oracle live in a
`_test.mygo` source (`legacy_parser_test.mygo`), whose generated Go is a
`_test.go` file.  The legacy grammar is therefore excluded from the production
parser2 build and retained strictly as the valid-source differential oracle;
it becomes removable once the section-4 formatter migration drops the last
oracle consumer.

The legacy post-parse scanning (task 3.13) is removed where an equivalent
already exists and quarantined otherwise.  The independent character-level
scanner (`ScanState`/`scanStep`/`advanceScan`/`emitTrivia`/`scanComment`/
`scanWord`/`emitToken`) was production-dead once `scanLossless` derived its
token view from `LexSource` green leaves; only `isSpace`/`isWordChar` remain
because `syntax.mygo` still shares them.  `LosslessDeclarationCount`,
`layoutExitAnchor`, and `layoutAnchor` were likewise dead and are gone.  The
surviving reconstructed-span, delimited-span, and layout-event production stays
in `lossless.mygo`, now marked as a transitional adapter whose only production
entry point is `ParseFileLossless` and whose only production consumer is the
formatter; it is retired family by family in section 4 rather than kept as a
parallel semantic authority.

Task 3.13's physical-removal clause is therefore recorded as **done by
isolation** rather than by literal deletion.  The acceptance check "verify no
formatter-facing production path depends on them" is **deferred to a section-4
recheck**: `formatter.mygo` still consumes the reconstructed node spans,
delimited spans, and layout events (`NodeSpans`/`Delimited`/`LayoutEvents`/
`ParseFileLossless`), so those families can only be deleted once the section-4
formatter cursor migration (4.1, 4.3-4.6) supplies equivalent CST queries.  The
3.13 checkbox is ticked on the strength of the isolation (production parse APIs
no longer depend on any of this code; only the formatter does), with the
zero-dependency verification explicitly outstanding against section 4.

### D6: CST records semantic grammar alternatives before lowering

The CST grammar SHALL distinguish concrete declaration alternatives, expression
operators and operands, type applications, and pattern alternatives.  Each
semantic child range is represented by a green node or ordered token boundary;
generic `Declaration`/`Expression`/`Type`/`Pattern` wrappers alone are not a
lowering contract.  Lowering consumes only those CST nodes and derives every
`ast2` source position and span from their green ranges.

The initial implementation stages this by adding explicit node kinds for the
declaration, atom, operator, type, and pattern families used by the current
parser2 grammar, with unsupported valid forms kept on the legacy route until
their CST alternative exists.  Re-parsing `SyntaxTree.Source` during lowering
is rejected because it would retain a second semantic authority and make range
derivation unverifiable.

### D7: Lowering and cutover advance through explicit grammar frontiers

The CST grammar and lowering SHALL advance in the same ordered frontiers:

1. top-level declaration headers and their typed child slots;
2. type and pattern recursion, including tuples, variants, and nested generic
   applications;
3. expression atoms, operators, calls, field/index access, collection and
   struct literals, and delimited expression groups;
4. expression and declaration bodies, including `if` and `switch` branches;
5. the remaining declaration families, constraints, and implementation forms;
6. full-source differential compatibility and public API cutover.

Each frontier adds concrete CST alternatives, typed Syntax accessors, lowering
coverage, and focused MyGO regressions together.  A diagnostic-free Syntax
Tree that contains an alternative not yet handled by lowering SHALL return an
explicit lowering error; it SHALL neither silently produce a partial AST nor
fall back to reparsing source text.  The legacy parser remains the valid-source
oracle only until the differential gate for the final frontier passes.

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
4. Establish the Syntax Tree formatter traversal and opaque-source policy.
5. Complete generic declaration, expression, and function-literal rendering;
   preserve canonical spacing and nested control-flow layout.
6. Complete recursive delimiter measurement and rendering, including long
   signatures, nested collection/struct fields, separator placement, and
   closing-delimiter alignment.
7. Gate legacy formatter removal on the full formatter golden corpus,
   protected-span checks, and format-twice byte identity/idempotence.
8. Remove reconstructed formatter metadata and its adapter only after parser2,
   formatter, command, bootstrap synchronization, and full-project gates pass.

Rollback before step 5 is routing public entry points back to the legacy AST
parser and formatter adapter; CST remains an opt-in syntax API until parity is
established.
