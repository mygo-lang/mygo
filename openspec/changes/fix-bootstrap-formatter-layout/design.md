## Context

See proposal.md - Why for the three defect lines and the 2x2 harness bisect that localized them. Current state and constraints:

- The working tree already contains an uncommitted span-propagation effort (`annotateExprSource` metadata preservation, `declarationEndAnchor`, `inlineGoExitAnchor`, span inventories). Its declaration-exit fix is correct and must be retained: with it, `lib/concurrency/channel.mygo` round-trips at declaration level; only case/wide-conditional layout and raw-string content remain wrong.
- The lossless pipeline is `ParseFileLossless -> finishFile -> collectNodeSpans -> collectLayoutEvents`; the formatter consumes `LayoutEvent` lists through `remapLayoutEvent`/`projectRenderedEventTargets` and rewrites single lines in `renderBranchSpanAt`/`renderCaseSpanAt`. The bisect showed the working-tree regressions live in event production (parser2), not in formatter consumption.
- Compiler sources are MyGO-authored: behavioral changes go into `.mygo` sources and are regenerated into `zz_*.gen.go` via `go run -buildvcs=false ./cmd/mygo sync <pkg>`. Regeneration success is not validation; generated packages must compile and test immediately after each sync.

## Goals / Non-Goals

**Goals:**
- Layout events exist and are correctly anchored for every structurally parsed construct, so that formatter indent/expansion decisions never depend on discarded events.
- Raw (triple-quoted) multiline string literals render byte-for-byte verbatim, interior leading whitespace included.
- `mygo --bootstrap fmt < lib/concurrency/channel.mygo` is byte-identical to the input; formatter, parser2, and cmd tests pass.

**Non-Goals:**
- Formatter performance on large files under `--check` (explicitly deferred).
- Working-tree hygiene items (stray root `lossless.mygo`, `mygo` binary) beyond noting them for the user.
- Replacing the lossless event architecture or the AST-only parse API.

## Decisions

### D1: Branch exit anchors trace the parent construct, never the branch header alone

`branchEndAnchor` for `if-then` resolves the enclosing `if` node (parent lookup on structural paths) and takes the matching `end` of that `if` header, as at HEAD. The working-tree variant that scans from the `then` anchor itself is rejected: inside `case ... then ... end` it consumes the case body's own `end` as the branch exit and displaces the switch terminator. `elsif ... then` is not an independent block: it stays represented by the dedicated `transition:if-elsif` event (working-tree addition, retained), so restoring the parent-`if` scan for `if-then` does not regress elsif.

Alternative considered: keep the branch-local scan and add special-casing for case bodies. Rejected because it re-introduces the exact "scan blindly instead of tracing structure" failure class that caused the original interface-nesting bug.

Probe evidence (implementation phase, 2026-09-11): the working tree's exit anchors for nested `if/elsif/else` already resolve correctly (`enter:if-then` of the elsif branch exits at its own `end`), so restoring the parent-`if` scan is retained as defensive alignment with HEAD semantics rather than as the fix carrier for regression A.

### D2: Never drop event pairs; verification only selects anchors

`collectLayoutEvents` SHALL emit enter/exit events for every structural node kind it handles. Where the working tree currently returns early when `verifiedLayoutAnchor` fails, the event is still emitted; anchor verification may only choose between a token-verified anchor and the parser-owned fallback span. Event existence is a parser contract; anchor quality is a best-effort refinement. This restores wide-`if` expansion, which failed because the discarded pair left `expandsAfter` unreachable.

Alternative considered: keep discarding and make the formatter resilient. Rejected: silent event loss makes every downstream consumer re-derive structure from line heuristics, contradicting the spec's parser-owned-events requirement.

### D3: Protected lines render verbatim with zero indent delta

In the formatter's line renderer, any line inside a protected span (triple-quoted and backtick literals) is emitted as its original source text with no trimming and no target-indent application, rather than the current trim-and-reindent. The protection ranges already come from parser2 `tripleQuotedLines`/`pairedQuotedLines` and stay unchanged; only rendering behavior changes. The closing delimiter line is inside the protected range, so its column is preserved as authored. This supersedes the current behavior asserted by `TestFormatterPreservesMultilineTripleString` only in strictness (that test keeps passing: it checks content preservation).

Alternative considered: preserve only relative indentation (dedent by common prefix). Rejected per explicit user requirement: contents stay exactly as authored, and the common-prefix heuristic would still rewrite `code:` bodies whose first content line happens to be dedented.

### D5: Expression-span completion must preserve the recorded end position

Probe evidence (2026-09-11): the working tree's `finishExprEnd` default branch recomputes every non-special-cased expression kind's end as `tokenEnd(expr.Span.Start)`, which truncates composite spans to their header token: the `switch` node span collapses from HEAD's case-covering `3:10-6:8` to `3:3-3:9` (the `switch` keyword), and `if` likewise loses its branch extent (`3:3-3:5`). `collectLayoutEvents` derives `blockLike`/`affectsIndent`/`exitAnchor` from these spans, so the truncation mis-structures every nested `case` (switch judged inline, `affects=false`, exit pinned to the header line) - the primary cause of regression A.

`finishExprEnd` SHALL retain `expr.Span.End` whenever it is usable and does not precede the expression's start, and SHALL fall back to `tokenEnd` only when the recorded end is missing. Special-cased kinds (`BlockExpr`, delimited literals, `InlineGoExpr`) keep their existing boundary logic. This is a general-path repair, not a per-kind special case.

Alternative considered: revert span completion into `collectExprSpans` (HEAD-style `switchExprSpan` reconstruction) and drop the working-tree `finishExpr`. Rejected: the span-propagation effort intentionally moves completion into `finishExpr` so AST consumers see spans; dropping it re-opens the `annotateExprSource` metadata gap the working tree was fixing.

### D4: Sequence parser2 and formatter steps as separate sync-validate gates

Fix parser2 (D1+D2) and regenerate + test parser2 first; only then land the formatter protected-line change (D3) and regenerate + test the formatter. Each gate is `sync <pkg>` immediately followed by `go test` of the regenerated package and the `/tmp/mygo_repro` cases, so a bad regeneration is caught before it compounds (the failure mode that ended the previous attempt).

## Risks / Trade-offs

- [Restoring parent-`if` exit scan re-breaks a construct the working-tree variant was written to fix] → The elsif case is already covered by dedicated transition events; add a regression test for `if/elsif/else` blocks before changing `branchEndAnchor`, and keep the failing goldens as the acceptance signal.
- [Verbatim protected lines interact with composed-line mappings (expansions that insert lines near protected spans)] → Protected spans are opaque single tokens/trivia; expansions never split a literal. Add a test mixing a long delimited expression and a raw string on adjacent lines to confirm mappings stay 1:1.
- [D2 fallback spans could re-introduce imprecise anchors that previously caused wrong indentation] → Anchor imprecision degrades layout only where token verification fails, which after D1 should not include block terminators; the byte-identity gate on `channel.mygo` plus goldens bounds this.
- [Regeneration instability across parser2/formatter bootstrap order] → Follow D4 gates strictly; never hand-edit `zz_*.gen.go`.

## Migration Plan

All changes are additive edits to `.mygo` sources plus regenerated output; rollback is reverting the `.mygo` files and re-running `sync`. No on-disk data or external consumers migrate.
