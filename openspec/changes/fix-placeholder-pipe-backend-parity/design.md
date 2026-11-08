## Context

The existing specification defines `_` as partial application with supplied expressions re-evaluated when the resulting function runs, while pipe operands evaluate left to right. typeinference2 gives the placeholder call a residual function type. codegen2 has a dedicated placeholder-call lowering; IR3 currently lowers calls and `_` as ordinary applications and variables. The reported codegen2 failure remains unverified, and parser, inference, generation, Go compilation, and runtime are all possible failure boundaries.

## Goals / Non-Goals

**Goals:** Preserve the existing semantic contracts across both backends and locate the exact reported failure before changing the relevant stage. Produce typed, compilable Go for residual functions with existing dispatch and FFI behavior.

**Non-Goals:** Change pipe precedence, add implicit argument insertion, change placeholder evaluation to capture supplied values eagerly, or expand support for currently forbidden placeholder forms.

## Decisions

1. **Reproduce end to end first.** Use equivalent direct and piped callback fixtures on each backend, checking parser diagnostics, inferred types, emitted Go, Go build and runtime. Treat codegen2's root cause as unknown until isolated. This avoids assuming a pipe bug when a parser or function literal issue is responsible. Add targeted tests at whichever stage fails.

2. **Lower partial applications into typed residual functions before ordinary call emission in the IR3 path.** Recognize only direct `_` call arguments, create fresh collision-safe parameters in placeholder order using instantiated concrete types, and lower a body containing the original callee and supplied expressions with holes replaced by parameters. Reuse the normal call lowering and boundary/dispatch paths, carrying type, source positions, explicit type arguments, dictionary resolution and call metadata. A shared typed desugaring before backend selection is an alternative if investigation demonstrates it preserves all existing metadata and semantics; avoid a pipe-only rewrite because stored and immediate partial applications must also work.

3. **Preserve evaluation boundaries.** Constructing the residual function must not evaluate its callee, receiver, or supplied arguments; those execute inside the function once per invocation in ordinary call order. Implicit dictionaries follow their existing call-site resolution rule. Pipe lowering must evaluate operand expressions in source order even for `|>` where emitted call order is reversed; verify IR normalization and each optimization level rather than assuming `ApplyExpr` preserves it. Callback creation and its body remain separate events.

4. **Use existing inference and diagnostics as the authority.** Preserve concrete type checks and unresolved-generic diagnostics; do not silently emit `_` or generic function values. Reuse existing rejection of variadic, intrinsic, nested and callee placeholders. If the reproduction exposes a shared frontend defect, make the smallest general fix and cover both backends.

5. **Verify behavior, not just emitted text.** Run equivalent fixtures through Go compilation and execution on codegen2 and codegen3, including optimization disabled and enabled. Check effect traces, result values, callback count, nested/multiple placeholders, generics, receiver/dictionary dispatch and supported FFI conversions, plus representative diagnostics. Keep focused stage tests for precise failures.

## Risks / Trade-offs

- [Typed AST metadata can be lost when introducing functions] → Preserve type substitutions, source spans, signatures, call classification and FFI metadata; test generic and foreign calls.
- [Closure conversion or optimizer hoists effects] → Runtime effect traces exercise creation and repeated invocation at each relevant optimization level.
- [Pipe order and deferred arguments are easily conflated] → Trace operand creation separately from function-body evaluation for both pipe directions.
- [Full placeholder parity has a wider surface than the reported example] → Reuse ordinary call lowering and cover methods, constructors, dictionary calls and FFI with focused representative cases.

## Migration Plan

No user migration is expected. Land regression tests and implementation together; reverting the change restores previous compiler behavior without a data migration.
