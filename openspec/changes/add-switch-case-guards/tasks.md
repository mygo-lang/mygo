## 1. Parser2 case-header and guard syntax

- [x] 1.1 Add a distinct guard region to the parser2 CST and verify a parsed guarded case exposes the guard separately from the pattern and body while replaying the original bytes.
- [x] 1.2 Implement the dedicated case-header structural path so a top-level guard `if` does not open an `IfBlock` or consume the case's `end`; verify `then`, `=>`, mixed-arm, and nested-switch CST tests.
- [x] 1.3 Parse the guard expression between the pattern and the top-level `then`/`=>`, accepting parenthesized if-expressions; verify accepted guard forms and reject an unparenthesized second top-level `if` with a targeted diagnostic.
- [x] 1.4 Add recovery for a missing guard expression and malformed case headers; verify diagnostics are returned, malformed text is retained, and later declarations remain available.
- [x] 1.5 Lower the guard into `ast2.SwitchCase.Guard` for both `then` and `=>` forms, and ensure case-body extraction skips the guard region; verify AST lowering tests distinguish guard from a body `if` expression.
- [x] 1.6 Update formatter handling for guarded case headers and verify formatting is byte-idempotent with comments and both case-body forms.

## 2. AST, inference, and exhaustiveness

- [x] 2.1 Add `Guard: Option[Expr]` to `ast2.SwitchCase` and update bootstrap constructors and traversal sites; verify the bootstrap compiler builds and existing unguarded switch tests still pass.
- [x] 2.2 Assign a guard expression ID and include the guard in typed-substitution rewriting; verify ID and generic-substitution tests cover a guarded case.
- [x] 2.3 Infer the guard in the pattern binding environment, unify it with `Bool`, and propagate its substitution and predicates before inferring the body; verify true/false guards, non-Boolean guards, and pattern bindings used only by the guard.
- [x] 2.4 Include guard expressions in inferred import collection; verify a guard referencing an imported symbol generates the required import.
- [x] 2.5 Exclude guarded cases from enum coverage, including guarded tuple cases and `case _ if`, while retaining guard and pattern validation; verify guarded-variant, guarded-wildcard, nested-tuple, and unguarded-fallback exhaustiveness tests.
- [x] 2.6 Verify non-enum switches with guarded cases remain exempt from exhaustiveness checking and still accept a switch without a wildcard.

## 3. Guarded code generation for ordinary switches

- [x] 3.1 Add guarded-switch detection and route the entire switch through the guarded path when any case has a guard; verify unguarded generated output is unchanged by existing codegen tests.
- [x] 3.2 Implement the matched-arm dispatch for statement and expression switches with `matched` set before the body; verify mixed guarded/unguarded cases execute first-match-wins.
- [x] 3.3 Implement guarded wildcard, binding, literal, variant, struct-variant, tuple, and nested-variant arm construction with a success continuation rather than a duplicated `next` chain; verify each pattern form falls through to the next case when its pattern or guard fails.
- [x] 3.4 Make pattern binding-use analysis consider the guard as well as the body, including nested variant arguments and struct fields; verify `case Some(x) if x > 100 => 1` and similar guard-only bindings generate valid Go.
- [x] 3.5 Lower `case _ if guard` as a conditional arm and never as the final unconditional `else`; verify a following unguarded `_` remains the fallback.
- [x] 3.6 Preserve existing unmatched behavior for guarded switches: statement no-op, ordinary expression behavior, and the existing tail fallback; verify each context.
- [x] 3.7 Verify a call-expression switch subject is evaluated once when multiple guarded arms are tested.

## 4. Tail-call lowering

- [x] 4.1 Detect guards in tail-position switches and route them through a guarded tail path; verify an unguarded tail switch keeps its existing generated shape.
- [x] 4.2 Implement matched-arm tail lowering with tail-rewritten case bodies and guard fallthrough; verify a guarded recursive function returns the correct result for matching, failing-guard, and fallback cases.
- [x] 4.3 Apply guard-aware binding-use analysis to the tail path and verify a binding used only in the guard compiles and executes correctly.

## 5. Legacy pipeline diagnostic

- [x] 5.1 Detect guard syntax in the legacy `--legacy sync` path and stop before code generation with an explicit unsupported-feature diagnostic; verify the command exits nonzero and reports case guards as unsupported.
- [x] 5.2 Verify the legacy failure leaves no silent generated output that omits the guard.

## 6. Documentation and validation

- [x] 6.1 Update the switch documentation with guard syntax, parenthesized guard if-expressions, guard fallthrough, and guarded-case exhaustiveness; verify the examples match the implemented behavior.
- [x] 6.2 Add end-to-end fixtures for enum, integer, string, tuple, nested-variant, guarded-wildcard, and statement-form guarded switches; verify compilation and runtime results.
- [x] 6.3 Run parser2, formatter, type-inference, exhaustiveness, codegen, tail-call, and CLI tests; verify all focused suites pass.
- [x] 6.4 Regenerate the changed self-hosted compiler stages in dependency order and rebuild their consumers; verify generated Go compiles and no unrelated generated files change.
- [x] 6.5 Run the broader relevant test suites with a bounded task-local cache and verify no regressions in unguarded switch behavior.
- [x] 6.6 Run `openspec validate add-switch-case-guards --type change --strict` and verify the change artifacts are valid.
