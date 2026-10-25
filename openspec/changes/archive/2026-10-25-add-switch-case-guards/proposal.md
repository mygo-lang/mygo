## Why

A `switch` case currently selects only by pattern. Expressing "this arm applies only when ..." requires nesting an `if` in the case body, which cannot fall through to later arms and duplicates fallback logic. Conditional case guards make the selection rule part of the case header while preserving the existing first-match-wins behavior.

## What Changes

- Add `case <pattern> if <guard> then ... end` and `case <pattern> if <guard> => body` syntax. Within a case header, the first top-level `if` before `then` or `=>` introduces the guard. An `if` expression inside a guard must be parenthesized.
- Evaluate a guard after its pattern matches and in that pattern's binding scope. A guard SHALL have type `Bool`. A false guard falls through to the next case; the switch subject and the guard are each evaluated once per attempted arm.
- Allow guards on any switch target and on a wildcard case (`case _ if ...`).
- A guarded case SHALL NOT count as exhaustiveness coverage. An enum switch still needs unguarded coverage of every variant or an unguarded wildcard. A guarded wildcard is not a catch-all. Non-enum switches remain non-exhaustively checked as before.
- Represent the guard in parser2 as a lossless `CaseGuard` CST node and preserve byte identity through formatting and recovery.
- Lower every switch that contains at least one guard through a matched-arm dispatch so tuple and nested patterns can fall through to the next top-level case without duplicating the remaining arms.
- Preserve each existing path's unmatched-switch behavior: ordinary expression switches keep their current result behavior, statement switches keep no-op behavior, and the tail-call path keeps its existing fallback.
- Extend the bootstrap parser2, typeinference2, and codegen2 pipelines. The legacy `--legacy sync` pipeline is out of scope for this change and SHALL reject guard syntax with an explicit unsupported-feature diagnostic rather than implementing partial semantics.

## Capabilities

### New Capabilities

- `language/switch-case-guards`: case guard syntax, guard evaluation and fallthrough semantics, guarded wildcard behavior, lossless parser2 representation, and code generation for guarded switches.

### Modified Capabilities

- `switch-exhaustiveness`: guarded cases do not contribute coverage, `case _ if ...` is not a catch-all, and the existing non-enum behavior is unchanged.

## Impact

- Parser2: CST node kinds, case-header structural parsing, pattern/guard lowering, recovery, and formatter layout.
- AST and inference: `SwitchCase` gains an optional guard; guard typing, binding scope, substitution, and predicate propagation.
- Code generation: a guarded-switch lowering path in `translate_ast.mygo`, `translate_expr.mygo`, and the parallel tail-call lowering.
- Tests and docs: parser/CST, formatter round-trip, inference diagnostics, exhaustiveness, codegen runtime behavior, and switch documentation.
- The legacy parser/compiler is not extended; it must diagnose guard syntax without generating partial output.
