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

### D6: Expand only the outermost same-line delimited construct; take item text from item spans

`renderDelimitedSpanAt` walks `Delimited` spans and, for a qualifying single-line construct (`raw.Len() > 100`, has separators), rewrites its whole anchor line via `replaceSourceLine(lines, line-1, raw.Slice(0, Span.Start.Column-1) + renderDelimitedBlockBody(raw, item))`. Evidence (a `FormatSource`/`Delimited` dump on the minimal repro, 2026-09-12): an over-long line with nested constructs yields several same-line spans — e.g. outer struct `36..201`, inner struct `100..159`, call `137..157`. Two failure modes follow:

- The prefix `raw.Slice(0, Span.Start.Column-1)` is correct only for the *outermost* span. For a nested span the gap between the enclosing item's start and the inner delimiter holds real text (`Span:` label, `stateSpan(` callee), which the inner rewrite deletes.
- Because the recursion renders inner spans before outer ones and each render re-reads `lines` from the accumulated `rendered` value, an already-expanded construct shifts the anchor line so the outer render reads corrupted text.

Decision: `renderDelimitedSpanAt` SHALL wrap a span only when it is not contained within another same-line span it would also wrap; nested constructs are reproduced verbatim as part of their enclosing item's text. Item text SHALL be taken from each item's own span (`Items[j].Start.Column .. item-end`, stopping at the item separator) so a field label or callee name preceding an inner delimiter is never dropped. Acceptance is the minimal-repro test plus idempotence and the existing goldens; `lossless.mygo` self-format (byte-identity / idempotence) is the full-file gate but is not required to pass to land D6 in isolation.

Alternative considered: continue rendering every span but track line-offset drift. Rejected: reconstructing an inner construct inside an already-rewritten multi-line item is redundant and re-introduces the label/callee deletion; nesting should be preserved verbatim at the item level.

Follow-up correction (2026-09-12): "leave nested verbatim" alone is NOT idempotent. A keyword `if … then … else … end` carries `ExpandsAfter` unconditionally in parser2, so once a delimited wrap moves it onto its own row a second pass re-expands it and pass 1 ≠ pass 2. The canonical output does not keep such a construct verbatim — it renders it inline in `=>` arrow form (which parser2 does not re-expand when narrow), and a value-position `switch`/`case` that wraps renders in `then … end` block form. That requires a recursive reflow, not a single-pass item slice; verbatim-keeping is retained only as the fallback for constructs the reflow cannot yet decompose. See D9 and the canonical acceptance examples.

### D7: Localize lossless span scans with a source-position binary search

`ParseFileLossless` builds one `NodeSpan` per token (`collectNodeSpans`), then several passes (`collectDeclSpans`, `collectNestedPatternSpans`, `collectDelimitedSpans`, `collectLayoutEvents`) iterate those N spans and, for each, call token-search helpers (`lastTokenInSpan`, `tokenEndBefore`, `tokenLastEnd`, `firstTokenInSpan`, `firstArrowInSpan`, `spanStartsLiteral`, `spanStartsProtectedLiteral`) that scan `scanned.Tokens` from index 0 to the end. N spans x N token scans is O(N^2): measured ~4x growth per doubling (n=30 0.23s, 60 0.87s, 120 3.4s, 240 12.4s; full 1713-line self-parse ~70s).

Decision: `scanLossless` emits tokens ordered by source position, so add a pure binary-search `firstTokenAtOrAfter(tokens, pos)` returning the lower-bound index. Each hot helper starts scanning at that index and stops once `token.Span.Start` passes `span.End`, converting every per-span scan from O(N) to O(log N + local region). Declaration-bound scans (`tokenEndBefore`/`tokenLastEnd`) additionally start at the declaration's own position, so each declaration touches only its own region. Return semantics are unchanged (same first/last-in-span index, same end-before-limit position, same literal predicate); the change is purely the scan window. Acceptance: identical `NodeSpans`/`Delimited`/`LayoutEvents` content and a large drop in full-file self-parse wall time.

Alternative considered: a line -> first-token index map. Rejected in favor of binary search because it adds a persistent structure threaded through every helper, while binary search localizes each existing pure function independently with no signature changes to the collection passes.

Implementation outcome (2026-09-12): the plan held for forward scans but two corrections were required to keep behavior identical, and a performance floor was reached.

- **Backward/nearest-keyword scans cannot be lower-bound localized.** `layoutAnchorForSpan`, `branchHeaderAnchor`, and `separatorAnchorAt` locate the nearest keyword (`if`/`case`/`then`/`=>`) *at or before* the span start, so restricting them to `[lowerBound(span.Start) .. end]` skips the very token they must find (7 if/case golden tests failed when tried). They keep a full scan; instead `layoutAnchorForSpan` is gated by `isAnchorKind` so the many non-keyword node spans (type/param/expr/delimited/...) skip the scan entirely.
- **The dominant residual cost is GC, not scanning.** After localization, profiles showed ~35% runtime in GC/allocator driven by functional `Option`/`Slice` (`Some{}`, `UnwrapOr`, `Append`) per structural node, not by token scans. The per-node `nestedBodyAnchor` full-list scan (3.8% of CPU) was gated to `canOwnBlockBody(kind)`, dropping it to ~1.3%. Eliminating the remaining allocation churn would require abandoning the codebase's no-mutation/Option style, which is out of scope.
- **Net result (behavior identical, full suite green):** synthetic 240-func/7202-token parse ~12.4s -> ~9.0s; full `lossless.mygo` self-parse ~72s -> under a minute. Further gains are limited by the allocation model rather than algorithmic complexity.

### D8: Coordinate-correct multi-pass reflow (implemented, committed 3e102f2)

Each reflow pass mutates a growing text but looked its anchor up by the ORIGINAL parser2 line, so any construct sitting below an earlier (upper) `if`/`case`/`block` expansion read a row those expansions had already shifted downward (regression E). Because every pass processes its spans in descending line order, same-pass insertions always land on rows *below* the current anchor, so the only correction a pass needs is the cumulative insertions of PRIOR passes.

Decision: every pass returns per-expansion `PassEdit`s (`SourceLine` = original anchor line, `Added` = rows that expansion produced) and receives the concatenated prior-pass edits; a helper `lineShift(edits, originalLine) = Σ Added over edits with SourceLine < originalLine` recovers the current row before any `lines.Get`/`replaceSourceLine`. `renderASTSpans` threads `blockEdits → branchEdits → caseEdits → delimitedEdits`, feeding each pass the union of all earlier passes' edits. Branch/case expansion is additionally skipped when its anchor sits inside a same-line delimited literal (`insideSameLineDelimited`), so the delimited pass owns that physical row and the two passes no longer slice the same row twice. The collapsed `branchPassEdits`/`casePassEdits`/`firstExpandableBranch`/`firstExpandableCase` helpers are removed; single-expansion output is byte-identical because a collapsed edit equals a single per-line edit.

Alternative considered: track a global running line-offset. Rejected: shifts are non-uniform (only expansions strictly above a line affect it), so a per-line `lineShift` over source-keyed edits is the correct translation and composes across passes for free.

### D9: Recursive inline`=>`/block`then` reflow over the span/event model

Canonical examples A and B (below, supplied by the user) require that a single physical source line reflow into arbitrarily many output rows with per-node inline-vs-block decisions, `=>`↔`then` form switching, recursive delimiter/`switch` nesting, and correct indentation. The three-pass line-replacement model cannot express this. Because the formatter is trivia-preserving (comments, inline Go, raw strings live outside the AST), the reflow SHALL stay on the lossless span/event model (`DelimitedSpan` items/separators + `LayoutEvent` `Anchor`/`BodyAnchor`/`SeparatorAnchor`/`ExitAnchor`/`HeaderForm` + source slicing) and NOT switch to re-printing from the AST (which would lose trivia).

Form rule table (parser2 already encodes `ExpandsAfter`: keyword `if…then…else…end` is always expandable, arrow form only when wide):

| Construct | Inline (single line) | Wrapped (multi-line) |
|---|---|---|
| `if` | `if cond => a else b` (expression `if` always has `else`) | `if cond then` … `else` … `end` |
| `case` | `case pat => body` | `case pat then` … `end` |
| `{}` / `[]` / `()` | no comma after the last item | comma after every item (non-tuple) |
| keyword `if`/`case` used as a value inside a delimited construct | renders inline `=>` | — |

Reflow sketch: a recursive `reflowItem(text, spans, events, indent)` renders one wrapped-delimited item — a nested keyword `if` → inline `=>`; a nested wide delimited → recurse block; a value-position `switch`/`case` that wraps → block; otherwise verbatim (field labels / callee names preserved). Statement `case`/`branch` block expansion recurses into its body the same way (producing example B: outer `case … then`, inner `joinSpans(…)` wrapped, inner `switch … end` block with inline `=>` cases and a trailing comma after `end`). Milestones: **M1** = recursive delimited item reflow + inline-`=>` conversion → example A exact & idempotent; **M2** = statement case/branch body recursion → example B exact & idempotent; **M3** = full `lossless.mygo` (zero fragment lines, re-parses, byte-identical second pass) + full `./...` + bootstrap build. A code checkpoint commit lands after each milestone.

Alternative considered: text-level keyword→arrow rewriting on item strings. Rejected: false positives on string/rune/ident contents and no reliable `then`/`else`/`end` delimitation; anchors come from `LayoutEvent` spans instead.

### D10: Recursive reflow is the sole idempotent wrapper (statement shapes + verbatim lock) — M3 cure

**Status.** M1 (recursive delimited item reflow, example A) and M2 (recursive arrow-`case` value-construct reflow + verbatim lock, example B) are implemented and committed (`e1dceb4`). M2 also fixed a corruption bug where `delimItemLoop` let the LAST delimited item absorb the closing delimiter (`collectVariantTypeSpans(…))` → `…))`, emitting invalid mygo).

**M3 blocker (a 3-pass round-trip over the real `lossless.mygo` isolates two coupled defects).** After M2, `lossless.mygo` re-parses on pass 2 (exit 0, no fragment lines) but is NOT yet byte-identical on pass 2 (~372 lines churn). Root causes:

- **(A) Non-recursive flat wrapping.** `renderDelimitedSpanAt` expands only the outermost same-line span and leaves nested-but-over-long spans inline (`Span: ps.SourceSpan { … }` at 124 cols). Once the outer wrap moves such an item onto its own line, it is top-level-and-over-long on the NEXT pass and gets re-wrapped — so pass 1 ≠ pass 2. Outermost-only (D6) is inherently non-idempotent for nested long content.
- **(B) Global target coordinate drift.** With many prior expansions in a 1856-line file, `projectDelimitedTargets`/`remapLayoutEvent` mis-map a wrapped span's produced rows, so the item-level `+1` indent bump is lost — verified: the FIRST wrapped span (`joinProtectedLines(`) renders items at indent 4 at `head -200` but at indent 2 in the full file. This is pre-`e1dceb4` machinery (58bff5d), not introduced by reflow.

**Implementation vehicle (corrected after span probing).** Parser2 has NO single node span for a call *value* (the callee `expr` sits just BEFORE the `delimited:call` span, which starts on the opening delimiter), so slicing a statement's value region by span is fragile. But the flat delimited pass ALREADY preserves the full line prefix: `renderDelimitedSpanAt` keeps `raw.Slice(0, item.Span.Start.Column - 1)` — i.e. `  let x = joinSpans`, `  Span: `, the label + callee — and only replaces from the opening delimiter onward. So the fix belongs IN the flat delimited pass, not in `reflowPass` statement-slicing:

1. **Recursive body (fixes (A)).** `renderDelimitedBlockBody` renders each item, and an item that itself contains an over-long same-line delimited construct is wrapped recursively (reusing the `reflowDelimLines`/`reflowRegion` recursion) instead of emitted as inline text — so a nested long span is wrapped in the SAME pass and never survives as a fresh top-level line to be re-wrapped on pass 2.
2. **Baked indent + verbatim lock (fixes (B)).** The block body's continuation rows carry their FINAL indentation (base = the anchor line's own leading whitespace, `+1` level per nesting), and the expanded rows are added to the render step's `VerbatimLines` set (committed machinery: `foldLine` → `verbatimLine`). Because indent is baked from the source line, the rows no longer depend on `projectDelimitedTargets`/`remapLayoutEvent`, which is the machinery that drifts at file scale → (B) cannot fire.
3. **Idempotence guard.** Because pass 1 now produces the fully-wrapped, verbatim form, pass 2 sees already-multi-line constructs (not same-line over-long spans) and the delimited predicate (`Span.Start.Line == Span.End.Line`) is false for them → it leaves them untouched → byte-identical. The single-line over-long field value from (A) no longer occurs, so it is never a fresh wrap candidate.
4. `renderBranchSpanAt` / `renderCaseSpanAt` guards against reflow-authority lines and the M2 verbatim/depth-transparency machinery stay as committed; `caseBodyNeedsFlatBlock` keeps force-blocking a bare wide `case` body.

**Gate.** Full `lossless.mygo` byte-identical on a second pass, zero fragment lines, re-parses; all goldens stay green (`channel.mygo` byte-identity, comma-list wrap, example A, and Examples B/C); full `./...` sweep + bootstrap build green. A code checkpoint lands when M3 turns green.

Alternative considered (a): a separate `reflowPass` statement-slicing vehicle. Rejected per the span note above — fragile callee slicing. (b): iterate the flat pass to a fixed point. Rejected: fixes (A) but leaves (B) (indent still computed by the drifting target machinery) and amplifies drift.

### D11: Two independent parser2 event-coverage gaps block R1/R2 block layout (goals A & B)

The group-9 residual pass-to-pass drift and the group-11 forced-block findings share one underlying fact: the target-depth engine is driven ONLY by structural layout events, and two construct families emit none, so their bodies are invisible to `projectRenderedEventTargets*`. These are two independent, separately-shippable gaps; the group-14 rendering reflow depends on BOTH and must not precede them (group 11.2 proved a source-normalization pre-pass collapses at file scale).

**Goal A (tasks 12) — else-branch that is itself a block (`if`/`switch`).** `projectRenderedEventTargetsAcc` closes a then-branch at the sibling `enter:if-else` anchor row via `siblingElseLine` (`remapLayoutEvent`: `exit:if-then` → `siblingElseLine`). But `collectExprSpans` `case IfExpr` sets `elseBranch = None` when the else-branch is an `IfExpr` (to dodge a node-path `[2]` collision with the nested `if`), which SUPPRESSES the `if-else` `NodeSpan`. With no `enter:if-else`, `siblingElseLine` falls back to the enclosing `if`'s `end` → the then-branch never closes at `else`, and the `else` keyword row renders one indent level too deep. Minimal repro `if x > 0 then\n  1\n else\n  if x == 0 then\n    2\n else\n    3\n  end\n end` puts `else` at body indent instead of the `if` level. FIX: always emit the `if-else` `NodeSpan` at path `[2]`, and when the else-branch is an `IfExpr` re-home its whole subtree one path level deeper (`[2, 0, …]`) so it no longer collides with the `if-else` node at `[2]`. `exit:if-then` then anchors at `else` and `exit:if-else` at the outer `end`, exactly as the non-`if` else-branch case already does.

**Goal B (tasks 13) — func-literal bodies.** `collectExprSpans` `case FuncLitExpr(_, _, body)` only recurses into `body` and emits NO node span and NO layout event for the `func … end` itself, so a func-literal body is invisible to the depth engine. Rule R2 (break after the return type `Ret`) and any multi-line func-literal need a depth block. FIX: emit a `func-lit` `NodeSpan` covering `func … end` and an `enter:func-lit`/`exit:func-lit` pair whose `AffectsIndent` is true ONLY when the func-literal body is a BLOCK (multi-line) — inline func-literals stay depth-transparent so existing inline call chains (`.Map(func(x: X) -> Bool y end).U(false)`) remain byte-identical (no double indent). The matching `end` is located by the group-10 `ifEndAnchor`-style net token scan.

Both land as parser2-only changes (plus goldens) and are independently gated: A on `t_canon` `else`-indent + full sweep; B on a wrapped func-literal body indents one level while an inline func-literal chain stays byte-identical + full sweep. Group 14 re-attempts the whole-span block reflow only after both.

### D12: Fold the statement-`if` (elsif + multi-row-`then`) and delimited close-indent renderings into the whole-span authority (group 16)

Goals A/B gave the depth engine the events it lacked; the two remaining user-reported mis-formats are pure RENDERING-authority gaps in group 14, so the fix belongs entirely in `formatter.mygo`'s whole-span pass `spanIfPass` (and the delimited wrap), not in parser2. Two facts from the probe drive the design:

1. **An `elsif` chain is a right-nested `if` tree, not a flat node list.** For `if c1 then b1 elsif c2 then b2 else b3 end` the parser emits `enter:if[P]` (whole chain), `enter:if-then[P++1]` (`b1`), a `transition:if-elsif[P++1]` on the `elsif` row, a NESTED `enter:if[P++2]` (the elsif, sharing the OUTER `end`), and that nested if's own `enter:if-then[P++2++1]` / `enter:if-else[P++2++2]`. The nested elsif `if`'s `Anchor` points at the OUTER `if` row, so its condition must be sliced from the `transition:if-elsif` keyword end to its own `enter:if-then` anchor start — NOT from its `Anchor`. `spanIfRows` currently bails when slot `[2]` is not an `enter:if-else`, so the whole elsif chain falls to the generic depth engine (which both dedents the chain one level and skips R1 block-form). FIX: render the chain recursively at the SAME depth, header keyword `if`/`elsif` chosen by whether the node was reached through an `elsif` transition, and emit the closing `end` exactly ONCE (all links share the outer `end`). Value-position elsif chains remain inline via 14.0b; only statement-position chains are owned.

2. **`spanThenRows` supports only a single-row `then` value.** A statement `elsif`/`if` whose `then` body is a multi-row block (statements on following rows) drops ownership today. FIX: mirror the existing multi-row-`else` per-source-row verbatim re-indent path for `then`, attributing each produced row to its own origin line so the additive per-source-line edit model and the verbatim lock still hold.

The delimited close-indent defect (golden b) is the same drift class as 14.3 but narrow: a wrapped delimited block's closing `)`/`]`/`}` row must carry the anchor line's leading whitespace (the base the block body already uses); isolated it is already correct, the loss is a multi-pass composed-line projection artifact. Fixing it is scoped to the closing-delimiter row's base so the argument-anchor and close rows agree, and is gated to not worsen the round-trip-bad count.

Each sub-step is independently gated against a golden + the fixed spot-check set (`codegen2`, `decls`, `channel`, `formatter`, `parser`) for round-trip + idempotence and reverted on any regression, honoring the 15.1/15.4/14.1 lesson that piecemeal edits here move the failure rather than converge.

## Canonical acceptance examples

Example A — over-long single-line struct literal whose field values are inline keyword `if`s; output wraps the literal and renders each `if` inline in `=>` form:

```
func advanceScan(state: ScanState, raw: String, width: Int) -> ScanState
  ScanState { Input: state.Input, Index: state.Index + 1, Offset: state.Offset + width, Line: if raw == "\n" then state.Line + 1 else state.Line end, Column: if raw == "\n" then 1 else state.Column + 1 end, Tokens: state.Tokens, Trivia: state.Trivia }
end
```
```
func advanceScan(state: ScanState, raw: String, width: Int) -> ScanState
  ScanState {
    Input: state.Input,
    Index: state.Index + 1,
    Offset: state.Offset + width,
    Line: if raw == "\n" => state.Line + 1 else state.Line,
    Column: if raw == "\n" => 1 else state.Column + 1,
    Tokens: state.Tokens,
    Trivia: state.Trivia,
  }
end
```

Example B — `case =>` body containing a nested `switch` expression; the wide case wraps to `then … end`, its `joinSpans(…)` argument list wraps, and the value-position `switch` wraps to block with inline `=>` cases and a trailing comma after `end`:

```
  switch decl
    case FuncDecl(_, _, params, ret, _, _) => joinSpans(collectParamSpans(params, tokens, 0), switch ret case Some(target) => withPath(collectTypeSpans(target, functionReturnSpan(tokens, declSpan), tokens), [-1]) case None => [] end)
    case StructDecl(_, _, fields) => collectFieldSpans(fields, tokens, 0)
  end
```
```
  switch decl
    case FuncDecl(_, _, params, ret, _, _) then
      joinSpans(
        collectParamSpans(params, tokens, 0),
        switch ret
          case Some(target) => withPath(collectTypeSpans(target, functionReturnSpan(tokens, declSpan), tokens), [-1])
          case None => []
        end,
      )
    end
    case StructDecl(_, _, fields) => collectFieldSpans(fields, tokens, 0)
  end
```

Example C — a keyword `if` used as an arrow-`case` body (a corrected refinement of example B): the value `if` has an `else`, so the case wraps to `then … end` and the `if` block-expands (`then`/`else`/`end`), each body row indented one further level; the sibling `case None => span` stays inline:

```
  switch tokens.Get(index)
    case Some(token) => if token.Raw == "end" then tokenRangeSpan(tokens, index, index) else span end
    case None => span
  end
```
```
  switch tokens.Get(index)
    case Some(token) then
      if token.Raw == "end" then
        tokenRangeSpan(tokens, index, index)
      else
        span
      end
    end
    case None => span
  end
```

## Risks / Trade-offs

- [Restoring parent-`if` exit scan re-breaks a construct the working-tree variant was written to fix] → The elsif case is already covered by dedicated transition events; add a regression test for `if/elsif/else` blocks before changing `branchEndAnchor`, and keep the failing goldens as the acceptance signal.
- [Verbatim protected lines interact with composed-line mappings (expansions that insert lines near protected spans)] → Protected spans are opaque single tokens/trivia; expansions never split a literal. Add a test mixing a long delimited expression and a raw string on adjacent lines to confirm mappings stay 1:1.
- [D2 fallback spans could re-introduce imprecise anchors that previously caused wrong indentation] → Anchor imprecision degrades layout only where token verification fails, which after D1 should not include block terminators; the byte-identity gate on `channel.mygo` plus goldens bounds this.
- [D6 skipping nested-span expansion leaves long inner constructs un-wrapped] → Acceptable: correctness (no corruption) precedes width preference; the enclosing outermost wrap already breaks the line, and idempotence gates the outcome.
- [Regeneration instability across parser2/formatter bootstrap order] → Follow D4 gates strictly; never hand-edit `zz_*.gen.go`.

## Migration Plan

All changes are additive edits to `.mygo` sources plus regenerated output; rollback is reverting the `.mygo` files and re-running `sync`. No on-disk data or external consumers migrate.
