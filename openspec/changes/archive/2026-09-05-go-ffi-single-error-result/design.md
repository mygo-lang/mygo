## Context

See `proposal.md` - Why. The boundary convention "trailing `error` becomes
`Result`" lives in two places that must agree:

- **Inference** (`internal/mygo/typeinference2/types.mygo`,
  `goSignatureResultShape`): the FFI call's MyGO type. It maps
  `(T, error)` -> `Result[T, error]` and `(T, bool)` -> `Option[T]`, but for a
  single result it returns the raw element type, so `func Foo() error`
  currently infers as raw `Error`, never `Result`.
- **Codegen** (`internal/mygo/codegen2/translate_ast.mygo`):
  `ffiResultPredicate` requires exactly two results with a trailing `error`, and
  `translateFFIResultCall` lowers `a, b := call()` then builds `Ok(a)/Err(b)`.
  A lone-`error` call matches none of the FFI predicates and never wraps.

The seed codegen (`internal/mygo/codegen/translate_call.go`,
`goSigErrorResultType`) has the same two-result-only rule but is the legacy
compiler; this change focuses on codegen2 (see D4).

## Goals / Non-Goals

**Goals:**
- Type a Go FFI call whose signature is a lone trailing `error` as
  `Result[(), error]` at inference time.
- Lower that call to Go that produces `Ok`/`Err` with a unit payload.
- Reuse the existing `(T, error)` machinery rather than adding a parallel
  `Option`-based decoding for lone errors.

**Non-Goals:**
- No new `Option[Error]` boundary shape; the decision is `Result[(), error]`.
- No change to the `(T, error)` -> `Result[T, error]` or `(T, bool)` ->
  `Option[T]` paths.
- No change to lone-`error` single-element inline Go blocks
  (`go[(error)] {...}`); only recorded Go FFI signatures are in scope.

## Decisions

### D1. Inference: map a lone `error` result to `Result[(), error]`
In `goSignatureResultShape` (`internal/mygo/typeinference2/types.mygo`), extend
the single-result branch: when `results.Len() == 1` and the lone element
canonicalizes to the Go error type (`TCon("Error")` or `TCon("error")`), return
`tCon("Result", [ast2.MonoType.TUnit, second])`; otherwise keep returning the
raw element as today.

Rationale: inference is what stamps the call with `Result[(), error]`, so
downstream codegen's `expected` monotype already carries the Result shape and
the existing `translateFFIResultCall` type-extraction (inner / error type) works
unchanged. Choosing `Result[(), error]` over `Option[Error]` keeps the error rule
uniform (see proposal) and reuses `Result`'s combinators; `Option[Error]` would
need a new type mapping and a new lowering both here and in codegen.

Alternatives considered: adding a fresh `Option[Error]` mapping - rejected
(inverted polarity, no combinator reuse, extra type + lowering). Keeping the raw
`Error` type - rejected (that is the current gap the proposal closes).

### D2. Codegen predicate: accept a lone trailing `error`
Widen `ffiResultPredicate` (`internal/mygo/codegen2/translate_ast.mygo`) so a
signature is an error-bearing FFI call when its last result is `error`, at any
arity: `sig.Results.Len() >= 1 && <last result> == "error"`. This keeps the
existing two-result match and adds the one-result case.

Rationale: the predicate drives both expression lowering and the
statement-position discard (`translateFFIDiscardStmt`), so one widening
fixes both - a lone-`error` call used as a statement automatically becomes a
raw call expression, satisfying the spec's discard scenario with no extra code.

### D3. Codegen lowering: branch on result arity
In `translateFFIResultCall`, stop ignoring the resolved signature and use it to
choose between two bodies:

- `Results.Len() == 2` (existing `(T, error)`): emit `a, b := call(); if b !=
  nil { Err(e) } else { Ok(a) }` - unchanged.
- `Results.Len() == 1` (lone `error`): emit `err := call(); if err != nil {
  Err(...) } else { Ok(struct{}{}) }`. The `Ok` arm synthesizes the unit payload
  as a `struct{}{}` composite literal (`goast.CompositeLit` of an empty struct
  type) instead of referencing a value temp, because the call has no payload.

The unit type already renders as `struct{}` in generic position
(`monoTypeToGoStrWithParamsIn` `TUnit` case), so `Result[(), error]` becomes
`Result[struct{}, error]` and the payload literal type-checks.

Rationale: the branch is driven by the recorded signature arity, which is the
single source of truth for how many native values the call returns; the
`Result[(), error]` shape is the same on both arms.

### D4. Seed codegen parity is optional / deferred
`internal/mygo/codegen/translate_call.go` (`goSigErrorResultType` /
`wrapGoErrorResultCall`) is the legacy seed compiler. Matching it is only worth
doing if the old compiler must behave identically; the project is consolidating
on codegen2. Recommend deferring seed parity (tracked as an optional task) so
this change stays scoped to the bootstrap/self-hosted pipeline.

## Risks / Trade-offs

- [Behavior change: a lone-`error` FFI call was previously typed as raw
  `Error`; now it is `Result[(), error]`. Any existing code binding the raw
  value breaks.] -> Only programs relying on undocumented raw-error FFI (the
  gap being closed) are affected; they migrate to `Result` combinators
  (`IsOk`/`IsErr`/`UnwrapOr`/`ToErr`) or bind-and-discard.
- [Over-broad predicate could catch a non-error lone bool?] -> No: D2 checks
  the last result equals `error`, so a lone `bool` is unaffected.
- [Generated `struct{}{}` payload relies on unit rendering as `struct{}` in
  generic position.] -> Confirmed in `monoTypeToGoStrWithParamsIn`; covered by a
  codegen e2e test asserting the emitted `Ok[struct{}, error](struct{}{})`.
- [Widening the predicate routes lone-`error` calls through the Result
  lowering path; if inference and codegen disagree, they double-convert.] ->
  Both are updated in lockstep (D1 + D3); a codegen e2e test guards this.

## Migration Plan

No data or deployment migration. Ship inference + codegen2 changes together
with tests and the `ffi.md` docs update; the seed compiler can follow later if
parity is required (D4).

## Open Questions

- None that change the specs, approach, or task breakdown. Whether to pursue
  seed-codegen parity is captured as an optional task rather than an open
  question.
