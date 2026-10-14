## Context

See `proposal.md` for the motivation. The current state, established by
reproducing the failure on the compiler checkout:

- Method-sugar calls lower through `translateReceiverImplMethodCall` /
  `implMethodCallExpr` in `internal/mygo/codegen2/translate_ast.mygo`. When
  seed-time analysis marks a helper as needing explicit type arguments
  (`RequiresExplicitTypeArgs` / `SpellCount`, computed by `methodSpellCount` in
  `internal/mygo/codegen2/decls.mygo`), the call site resolves the prefix from
  the receiver expression's recorded `MonoType` via `receiverImplCallTypeArgs`.
- The receiver type is concrete only when inference substituted it. The
  original diagnosis blamed a missing normalization pass on unannotated
  bindings; experimentation shows the real gap is narrower: **explicit source
  type arguments are not applied during inference**. `inferCall` receives
  `typeArgs` and threads them into the typed AST for code generation only;
  `inferOrdinaryCall` was called without them, so `empty[Result[Int, String]]()`
  was inferred as `Option[<var>]` and every chained receiver kept a free
  variable.
- Controls that isolate the cause: a chain on a receiver whose type argument is
  inferable from the value compiles and spells correctly; a chain on a receiver
  whose type argument is written explicitly and appears only in the return type
  does not; adding a declared type on the intermediate binding masks it. The
  receiver type (`Option[A]` vs `Result[A, E]`), the package boundary, the
  builtin `Chan` receiver, and the impl/typeclass shape are all irrelevant.

## Goals / Non-Goals

**Goals:**

- Generate compiling Go for non-annotated chains of impl/typeclass method calls
  whose type parameters are non-inferable, by making inference apply written
  type arguments.
- Keep the receiver-method spell-out analysis and lowering unchanged.

**Non-Goals:**

- Changing the Go representation or public API of Prelude `Option`/`Result`.
- Adding a generation-time diagnostic for unresolved receiver methods.
  Experimentation shows inference already rejects an unresolvable method call
  (`unknown field Box.Unwrap`) before code generation, so such a diagnostic is
  unreachable for well-typed programs; adding one would be dead code.
- Applying written type arguments to receiver-style method callees (`x.m[T]()`),
  whose scheme is resolved through field dispatch rather than the package or
  lexical environment.

## Decisions

- **Apply explicit source type arguments during inference.** When a call
  supplies written type arguments and the callee resolves to a scheme with bound
  parameters, bind those parameters to the written types. `instantiateBoundSubstFor`
  assigns each bound parameter a fresh id sequentially from `state.FreshVarID`,
  so recomputing the substitution with the same state reproduces exactly the ids
  the callee instantiation used. The binding is composed into the callee's
  inferred result and applied to the typed callee subtree, so the call's
  recorded type and every enclosing chain receiver become concrete.
  Alternative considered: reconstruct the receiver's element type inside
  codegen, or normalize the typed expression at the declaration boundary.
  Both treat the symptom; the call's own type is what must be concrete, and
  inference is the single owner of that.
- **Restrict the rewrite to callee shapes whose scheme is reachable**: a bare
  identifier, an imported `alias.Name`, or an imported `alias.Enum.Variant`.
  Receiver-method callees keep their previous behavior; their written type
  arguments are rare and their schemes resolve through field dispatch.
- **Leave the plain-call fallback in `implMethodCallExpr` in place.**
  `methodSpellCount` spells conservatively when it cannot prove a position
  inferable, so a short resolved prefix can still produce valid Go through
  ordinary inference (verified by the imported `Measure` fixture). Erroring
  there rejects valid programs.

## Risks / Trade-offs

- [Written type arguments now constrain inference] → A written argument that
  conflicts with the value arguments is contradicted rather than silently
  ignored, which is the intended behavior; the full bootstrap suite and the
  self-hosting sync cover existing call sites.
- [Fresh-id reproduction drift] → The ids are recomputed from the same
  `InferState` the callee instantiation uses, and the instantiation is
  deterministic; the end-to-end fixture tests would fail if the ids diverged.
- [Generated files drift] → Only `typeinference2` outputs change; `codegen2`'s
  source edit is a comment and its generated files are unchanged.

## Migration Plan

No data or API migration. Rollback is reverting the source and generated files.

## Open Questions

None.
