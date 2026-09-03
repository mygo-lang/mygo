## Context

`codegen2` lowers a pattern-matching `switch (state, event)` whose case bodies
have a tuple return type into an anonymous `struct{F0 T0; F1 T1}{...}` value
that holds the tuple elements. Case bodies that destructure an **enum variant**
(e.g. `RunStarted { RunID, ..., InitialMessage }`) are expected to bind each
field to the corresponding Go field produced by that variant's type assertion.

Investigation (`/tmp/qmin`, self-contained, no project code involved) shows the
emitted Go for a variant field referenced inside a function call argument in a
tuple-returning case is produced as:

```go
struct{ F0 AgentState; F1 AgentEvent }{F0: state, F1: event}.F1.InitialMessage
```

The `event` element is correctly lifted to `.F1`, but the `.InitialMessage`
field access is emitted **directly on the enum value** instead of on the
result of the `.(RunStarted)` type assertion for the matched variant. This
happens only when (a) the switch subject is a multi-element tuple, (b) the
case body's expected return type is a tuple, and (c) the pattern-bound field
is referenced in a function-call argument position. A single-element switch or
a non-tuple return type does not reproduce.

Relevant code paths: the tail-aware switch lowering
`translateSwitchBranchesTail` in `internal/mygo/codegen2/tailcall.mygo`, which
handles `TuplePattern` subjects but, unlike the ordinary switch path
(`translateSwitchBranchesStmt` / `translateTupleVariantSwitchStmtAt` in
`internal/mygo/codegen2/translate_ast.mygo`), never emits the per-element enum
variant type assertion. The shared binding helpers `bindPattern` /
`bindPlainStructPatternFields` / `bindStructPatternFieldsWithTypes` in
`translate_ast.mygo` play a role, but the missing dispatch lives in the tail
tuple branch.

## Goals / Non-Goals

**Goals:**

- Fix the codegen so a pattern-bound enum variant field referenced in a
  tuple-returning switch case compiles to the bound local value.
- Keep generated Go for all currently-correct source shapes byte-for-byte
  unchanged where possible.
- Add a regression test that reproduces `(State, Slice[Command])` return +
  enum variant destructuring + function-argument use.

**Non-Goals:**

- New MyGO language features or syntax.
- Changing the type checker or parser; the input AST and mono types are correct.
- Performance optimization of generated switch lowering.

## Decisions

### 1. Tail tuple switches must emit per-element variant dispatch

The root cause is in the tail-aware switch lowering
`translateSwitchBranchesTail` (`internal/mygo/codegen2/tailcall.mygo`). Its
`TuplePattern(items)` branch only runs `bindTuplePattern(...)` and then emits
the case body unconditionally:

```mygo
case TuplePattern(items) =>
  let child = Ref.new(ctxChild(ctx))
  bindTuplePattern(child, items, goast.MustExprSource(target), 0)
  translateAstReturnExpr(current.Body, child)
```

For a tuple subject containing enum variants (e.g. `(Idle, RunStarted {...})`),
this never emits the `if v, ok := <elem>.(<Variant>); ok { ... }` guard. The
bound field therefore resolves to a bare selector on the anonymous tuple
element (`struct{...}{...}.F1.InitialMessage`) with no type assertion, and the
switch produces no runtime dispatch at all.

The fix must give the tail tuple branch the same per-element variant handling
that the ordinary switch path already has: bind non-variant elements with
`bindTuplePatternNonVariants`, then walk the tuple elements and emit a
`VariantIf` per enum variant (mirroring `translate_ast.mygo`'s
`translateTupleVariantSwitchStmtAt`). Each case body is still lowered through
`translateAstReturnExpr` so mutual tail calls inside it keep being rewritten
into state transitions.

- **Why this branch is special**: `translate_expr.mygo` routes a
  `SwitchExpr` through `translateAstReturnSwitch` (and therefore the tail
  lowering) whenever the function returns a tuple or is part of a
  mutual-tail-call plan. The previously-investigated ordinary switch path
  (`translateSwitchBranchesStmt` / `translateTupleVariantSwitchStmtAt`) is
  correct and already emits the assertion; it simply is not reached here.
- **Alternative considered**: threading a "needs variant assertion" flag
  through `bindPattern` / `bindPlainStructPatternFields` so the bare
  `struct{...}.F1.InitialMessage` selector becomes type-asserted. Rejected:
  those helpers lack the tuple element type needed to build the assertion, and
  patching them would not reintroduce the missing `VariantIf` dispatch that the
  tail path drops.

### 2. Share the tuple variant dispatch between ordinary and tail switches

Rather than inventing a parallel, tail-specific tuple walker, factor the
decision "is tuple element `i` an enum variant?" and the resulting
`VariantIf`-construction so both `translateTupleVariantSwitchStmtAt`
(ordinary) and the fixed `translateSwitchBranchesTail` `TuplePattern` branch
(tail) use the same logic, differing only in how a case body is lowered
(`translateSwitchCaseBodyStmt` vs `translateAstReturnExpr`).

- **Why**: keeps the two lowering paths' semantics in lock-step and prevents
  this class of divergence from regressing.

### 3. Regression test mirrors the reducer shape

The test must combine a two-element enum subject, tuple return type, and a
case body that passes the destructured field into a regular function call.

## Risks / Trade-offs

- [Loose coupling risk if the variant-lookahead is done in binding helpers]
  → Centralize the enum variant type lookup in one helper that returns the
    type-asserted base, and unit-test it against the minimal repro before
    running the full bootstrap corpus.
- [The exact expected code shape may shift slightly for the affected case]
  → The spec only requires that the generated Go compiles and semantics are
    preserved, not a specific spelling of the assertion.
