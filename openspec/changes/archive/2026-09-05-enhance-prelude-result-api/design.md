## Context

The prelude is compiled by the self-hosted bootstrap compiler itself: every `.mygo`
addition must pass the method/typeclass dispatch in `typeinference2` and `codegen2`
and be regenerated into `zz_*.gen.go` via `mygo --bootstrap sync`. Existing precedent
for the approach:

- `impl[A, E] Result[A, E]` inherent impl already hosts `ToOption`/`Flatten`
  (two-type-parameter inherent impls work; generated Go uses
  `MygoIN6ResultM<method>` mangling).
- Pure-MyGO `switch` over enum variants is the established implementation style
  (`OptionIEnumerable`, `ResultEq`); exercising it in the prelude dogfoods the
  compiler.
- `using Eq[A], Eq[E]` multi-constraint dispatch had a codegen bug (see
  `plans/fix-result-eq-equals.md`); single-constraint and constraint-free methods are
  low risk.
- KNOWN_ISSUES #13 warns that multi-type-parameter *typeclass* impls are fragile;
  inherent impls (non-typeclass) are the proven path.
- The prelude already splits impls by concern: `prelude.mygo` holds enums/interfaces/
  standalone helpers, `option.mygo` holds all `Option` impls.

## Goals / Non-Goals

**Goals:**
- Method-set parity with the Rust std `Result`/`Option` combinators listed in the
  proposal, implemented as inherent impl methods consumed through method-call sugar.
- Constraint-free methods only in the committed surface; `Default`-constrained methods
  (`UnwrapOrDefault`, `MapOrElse` fallback) gated on a dispatch smoke test.
- File organization mirroring `option.mygo`: all `Result` impls live in a new
  `prelude/result.mygo`.

**Non-Goals:**
- `Into` blanket derivation from `From` (compiler feature, deferred).
- A `?`/try early-return operator (language feature, deferred; the prelude cannot
  express it).
- `Result` implementing `IEnumerable` — rejected: `Filter` has no "empty" state for
  `Err` (see Decisions).
- Clone/Copy-based methods (`copied`/`cloned`), `as_ref`/`as_mut` (no Clone/Copy
  traits; `Ref.new` on locals is semantically iffy — see KNOWN_ISSUES "sumList"),
  and `into_ok`/`into_err` (no `Infallible` type).

## Decisions

### D1: New `prelude/result.mygo` hosting all Result impls
Create `result.mygo` and move the existing `impl[A, E] Result[A, E]` (`ToOption`,
`Flatten`) and `impl[A, E] ResultEq[A, E]` from `prelude.mygo` into it, then add the
new combinators there. This makes the layout fully symmetric with `option.mygo`
(Option enum in `prelude.mygo`, Option impls in `option.mygo`):

```
prelude.mygo     enums, interfaces, standalone helpers (OptionToResult/Panic/Zero/...)
option.mygo      OptionIEnumerable + impl[A] Option[A] + OptionEq
result.mygo      impl[A, E] Result[A, E] (existing + new) + ResultEq
```

The `enum Result[A, E]` declaration stays in `prelude.mygo` alongside `enum Option[A]`.
All prelude files compile into one package, so this is pure organization: no compiler
changes, no import cycles, and the bootstrap pipeline picks up the new file
automatically (regenerated as `zz_result.gen.go`). Moving existing code is churn but
keeps each type's impls in exactly one file; the alternative (leave existing impls in
`prelude.mygo`, add only new methods in `result.mygo`) splits `Result` across two
files and was rejected.

### D2: Result does not implement IEnumerable
`IEnumerable.Filter` must return the container type, but a filtered `Err` has no
representation (no empty state, no error value to synthesize). Rust makes the same
call: `Result` has no `Iterator` impl, only `iter()` (0-or-1 items). Consumers needing
filter/fold-style access convert via `ToOption` first.

### D3: All combinators are receiver-first methods in the inherent impl
`impl[A, E] Result[A, E]` (resp. `impl[A] Option[A]`) with the receiver as the first
parameter, matching existing `ToOption`/`Flatten`/`UnwrapOr`. Method names are
explicitly CamelCase (`IsOk`, `MapErr`, `AndThen`, ...) following the prelude
convention; the Rust snake_case API is only the semantic blueprint.

### D4: Panic methods reuse the existing `Panic` helper
`Unwrap`/`Expect`/`UnwrapErr`/`ExpectErr` lower to `Panic(...)` on the unexpected
variant. `Expect` panics with the message and does not require `ToString[E]`, keeping
all confined methods constraint-free; no error-payload formatting is added to panic
messages in this change.

### D5: Typeclass constraints only where unavoidable
`UnwrapOrDefault` (both types) uses `using Default[A]`; `MapOrElse`'s fallback branch
takes `func(E) -> A` without constraints. The `using` methods are the only ones
touching `constraintFuncForMethod` dispatch in an inherent impl — a path without
prelude precedent — so they are implemented after a smoke test compiles and runs one
constrained method. If dispatch does not resolve, they are dropped and the spec's
"Default-constrained extraction" requirement is re-scoped before implementation.

### D6: Conversion naming and compatibility
Existing `Result.ToOption` is kept; the new reverse accessor is `ToErr`
(`Result -> Option[E]`), symmetric and unambiguous in method position. `Option.OkOr`
replaces the standalone `OptionToResult` in new code, but the standalone function is
kept for source compatibility in this change. `OptionFilter` is removed as a confirmed
duplicate of `Filter` (spec: "Standalone OptionFilter function" removal).

### D7: Signature shapes (Rust-aligned, simplified)
- `Map[B]`: `func(res: Result[A, E], fn: func(A) -> B) -> Result[B, E]`
- `MapErr[E2]`: `func(res, fn: func(E) -> E2) -> Result[A, E2]`
- `AndThen[B]`: `func(res, fn: func(A) -> Result[B, E]) -> Result[B, E]`
- `OrElse[E2]`: `func(res, fn: func(E) -> Result[A, E2]) -> Result[A, E2]`
- `And[B]`: `func(res, other: Result[B, E]) -> Result[B, E]`
- `Or[E2]`: `func(res, other: Result[A, E2]) -> Result[A, E2]`
- `Transpose`: `Option[Result[A, E]] -> Result[Option[A], E]` — preferred as an
  `Option`-side method if method sugar resolves on `Option[Result[A, E]]`, otherwise a
  standalone function (observable behavior identical either way).

## Risks / Trade-offs

- [Multi-parameter inherent impl dispatch regression] → Constraint-free methods are
  structurally identical to the existing proven `ToOption`/`Flatten`; the constrained
  set (D5) is gated behind a smoke test and droppable without breaking the core spec.
- [`using` constraints in inherent impls may not resolve] → smoke test first; fallback
  is to drop the constrained methods from this change and add them in a dedicated
  change once the compiler supports the dispatch path.
- [Method-name collisions with typeclass methods (`Map`, `Flatten`, `OrElse`)] →
  Mangled Go symbols (`MygoIN6ResultM3Map` etc.) are distinct per receiver; no source
  conflict in method-call resolution.
- [Moving existing impls across files may ripple generated output] → Pure relocation,
  no behavior change; verified by regenerating and running the full test suites.
- [Bootstrap regeneration churn] → Follow the existing `mygo --bootstrap sync` +
  `go build ./...` loop; prelude changes regenerate `zz_result.gen.go`,
  `zz_option.gen.go`, and `zz_prelude.gen.go` only.

## Migration Plan

- Implement constraint-free methods in `result.mygo`/`option.mygo` and move the
  existing `Result` impls; regenerate `zz_*.gen.go`; run `go build ./...` and the
  compiler test suites.
- Smoke-test one constrained method (`UnwrapOrDefault`) before committing the rest of
  the `using Default[A]` set.
- Keep `OptionToResult`; remove `OptionFilter` after confirming no in-repo callers
  (grep `OptionFilter` over `lib/`, `internal/`, `examples/`).
- Update `docs/compiler/ffi.md` Option/Result section with the final method list.

## Open Questions

- Whether the compiler's method-sugar resolution can target `Option[Result[A, E]]`
  for `Transpose` as a method, or whether a standalone function is required (won't
  change the spec contract — observable behavior is identical).
- Exact panic message text for `Expect` when `E` is a `String` (message-only vs.
  message + payload); settled in implementation without affecting spec scenarios.
