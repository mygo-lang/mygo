## Context

See `proposal.md` for motivation and `specs/` for the behavior contract. The current parser2 case branch is built by a generic structural pass that treats `if` as a block opener, while the case header is split later at its first top-level `then` or `=>`. The current code generator lowers an unguarded switch as a recursive if/else chain whose `next` arm is embedded in each mismatch branch.

Those two structures create the main constraints:

- A guard `if` must not be allowed to consume the case's closing `end` during structural parsing.
- A guard that fails must continue with the next top-level case, including after tuple or nested-variant assertions have partially matched.
- Reusing the existing `next` embedding would duplicate the remaining arms inside every guard failure path and can grow exponentially when many cases are guarded.

## Goals / Non-Goals

**Goals:**

- Make guarded cases available on every switch target and on statement, expression, and tail positions.
- Keep guard parsing lossless and formatter-idempotent.
- Preserve first-match-wins behavior with no subject or guard re-evaluation.
- Keep unguarded switches on the existing code-generation path.
- Keep the legacy pipeline from silently dropping guards.

**Non-Goals:**

- Implementing guard syntax in the legacy `--legacy sync` compiler.
- Changing the existing unmatched-switch behavior of any pipeline.
- Adding pattern alternatives, guard lists, or `when` syntax.
- Changing enum exhaustiveness rules unrelated to guards.

## Decisions

### Use the first top-level `if` in a case header as the guard introducer

The case-header grammar is `case pattern [if guard_expr] (then | =>)`. At header top level, the first `if` after the pattern is always the guard introducer, matching the user's chosen disambiguation rule. An `if` expression inside the guard must be parenthesized; after the case terminator, `if` remains an ordinary body expression.

This avoids introducing a new `when` keyword and keeps the familiar Rust/Scala shape. The cost is that nested if-expressions in guards require parentheses, which the parser should diagnose explicitly rather than misparse. The alternative of an enum-only guard was rejected because guards are useful for literal and tuple switches as well.

### Build case headers with a dedicated structural path

When `buildDelimitedChildren` encounters a `case` inside a switch, it should build that case's header separately: find the top-level `then` or `=>` using delimiter-depth scanning, keep the header tokens raw at top level, and then build the body with the normal structural rules. Parenthesized groups in the header still recurse normally, so a parenthesized if-expression receives a normal `IfBlock` inside its `ParenGroup`.

This is preferred over a boolean `inCaseHeader` flag threaded through the generic builder because the header/body transition is explicit and nested switches do not have to infer their context from raw token order. It also keeps the guard `if` from pairing with the case's `end`.

### Add a guard field to `SwitchCase`

`ast2.SwitchCase` gains `Guard: Option[Expr]`. A guard is branch-level selection logic, not a reusable pattern, so it does not belong in the `Pattern` enum. This confines changes to case construction, inference, substitution, and code generation instead of every pattern walk.

Parser2 inserts a distinct guard region after the pattern and before the header terminator. Lowering records the guard expression and its span. The case-body finder must skip the guard region; otherwise the first expression after the pattern would be mistaken for the body.

### Infer the guard before the case body

After `envWithPattern` creates the case environment, infer the guard in that environment and unify its type with `Bool`. Compose the resulting substitution and predicates before inferring the body, so pattern-bound names used by the guard have their final types and the body sees the same substitution. Apply the substitution to the guard as well as the body, and include the guard in expression-ID assignment and inferred-import collection.

### Exclude guarded cases from enum coverage

Exhaustiveness receives only unguarded case patterns. This applies recursively to nested tuple patterns: a guarded tuple case contributes no coverage at any nested position. `case _ if ...` is likewise excluded. Non-enum switches remain unchecked, so a guarded integer/string/tuple switch does not gain a new exhaustiveness requirement.

Guarded cases still undergo pattern validation, binding checks, and type inference. Only coverage aggregation changes.

### Use a matched-arm dispatch for every switch containing a guard

If any case in a switch has a guard, lower the entire switch through a linear matched-arm path. Every arm is gated by `if !matched`; a successful pattern and guard set `matched = true` immediately before executing the body. This is uniform across enum, literal, wildcard, tuple, and nested-variant patterns, and a guard failure simply leaves `matched` false so the next top-level arm is attempted.

The alternative of embedding the remaining arms in the guard's `else` is simple but duplicates `next` and can produce exponential generated code. A synthetic loop with `break` was rejected because a user `break` or `continue` in a case body would change target. Setting `matched` before the body preserves the body's control-flow semantics and avoids a final cleanup assignment that becomes unreachable after `return`.

Unguarded switches keep the existing recursive lowering, limiting regression risk. Existing unmatched behavior is preserved: statement switches remain no-ops when nothing matches, ordinary expression switches retain their current result behavior, and the tail-call path retains its fallback.

### Thread pattern success instead of `next` in the guarded path

The guarded path needs a success continuation rather than the existing `next` slice. Tuple and nested-variant matches become nested `if` assertions; a failed assertion does nothing and lets the outer `if !matched` block try the next case. The guard sits at the innermost success point where all pattern bindings are available.

Binding-use analysis must include the guard. `patternArgsUsedInBody` and `structPatternFieldsUsedInBody` currently decide whether a real value name is needed; a binding used only by the guard must force a real name, or generated code will bind `_` and the guard cannot read the value.

### Add a parallel guarded tail-call path

The tail-call lowering has its own switch and tuple dispatch. It needs the same matched-arm behavior and guard-aware binding analysis, while retaining its existing tail-rewrite and fallback behavior. A shared helper for building the pattern-success chain should be used where practical to prevent the two paths from drifting.

### Reject guards in the legacy pipeline

The legacy parser, type checker, and code generator are not extended. `--legacy sync` must detect guard syntax and fail with a clear unsupported-feature diagnostic before emitting generated output. This avoids a second, subtly different implementation and prevents guards from being silently ignored.

## Risks / Trade-offs

- [Case-header structural parsing regresses nested switches or arrow-form cases] -> Add focused CST tests for `then`, `=>`, mixed arms, nested switches, and parenthesized guard if-expressions; keep lossless replay and formatter idempotence checks.
- [The guarded lowering changes behavior for unguarded arms in the same switch] -> Gate the new path on the presence of any guard, test mixed guarded/unguarded switches, and keep existing unguarded switch tests unchanged.
- [Tuple or nested-pattern success is marked too early] -> Set `matched` only at the innermost success point after all assertions and the guard; test partial tuple matches and nested variant mismatches.
- [Bindings used only in a guard are optimized away] -> Make usage analysis consider both guard and body, including nested variant arguments and struct fields.
- [Tail-call behavior diverges from ordinary codegen] -> Share arm-construction helpers where possible and add a guarded tail-recursive fixture.
- [A guard's if-expression is misparsed as the header terminator] -> Require and diagnose parentheses for guard if-expressions; test both accepted and rejected forms.
- [Legacy generation silently ignores the guard] -> Detect guards in the legacy parser or validator and emit an unsupported-feature error before code generation.

## Migration Plan

1. Add parser2 CST representation, header parsing, guard lowering, and tests.
2. Add `SwitchCase.Guard`, inference, substitution, ID/import traversal, and exhaustiveness filtering.
3. Add the guarded code-generation path for ordinary switches and the parallel tail-call path.
4. Add the legacy unsupported-feature diagnostic.
5. Update switch documentation and add parser, formatter, inference, exhaustiveness, codegen, and CLI coverage.
6. Because the compiler is self-hosted, regenerate the changed compiler stages in dependency order and rebuild their consumers before running focused and broader tests.

Rollback is additive: remove the guard syntax path and generated compiler artifacts, leaving unguarded switch behavior unchanged. No existing source programs depend on guard syntax, so there is no data migration.
