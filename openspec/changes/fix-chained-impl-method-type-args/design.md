## Context

See `proposal.md` for the motivation. The relevant current state, established by
reproducing the failure on the current compiler checkout:

- Method-sugar calls lower through `translateReceiverImplMethodCall` /
  `implMethodCallExpr` in `internal/mygo/codegen2/translate_ast.mygo`. When
  seed-time analysis marks a helper as needing explicit type arguments
  (`RequiresExplicitTypeArgs` / `SpellCount`, computed by `methodSpellCount` in
  `internal/mygo/codegen2/decls.mygo`), the call site resolves the prefix from
  the receiver expression's recorded `MonoType` via `receiverImplCallTypeArgs`.
  If the resolved count does not equal `SpellCount`, codegen silently emits a
  plain helper call.
- `Option.Unwrap` and `Result.Unwrap` carry their type parameters only inside
  the interface-shaped containers `Option[A]` / `Result[A, E]`, so
  `methodSpellCount` reports `SpellCount = 1` and `2` respectively. Go cannot
  infer those arguments, which is why the plain-call fallback fails to build.
- The receiver's recorded `MonoType` is only fully substituted when a declared
  or expected type drives a normalization pass (`applyTypedExprSubst` in
  `internal/mygo/typeinference2/infer.mygo`). Without an annotation, the
  intermediate call expressions of a chain keep unresolved type variables.
- Observable evidence: the same chain generates correct, compiling Go when a
  declared type is present on an intermediate binding or on the result, and
  uncompilable Go when no annotation is present. When method dispatch fails
  entirely, codegen falls through to ordinary call translation, which renders
  the `FieldExpr` callee as a Go selector `recv.Unwrap()` and reports no
  diagnostic (`sync` exits 0).

## Goals / Non-Goals

**Goals:**

- Generate compiling Go for non-annotated chains of impl/typeclass method calls
  whose type parameters are non-inferable.
- Make the remaining unresolvable cases fail loudly with a source-located
  diagnostic instead of emitting invalid or inference-dependent Go.

**Non-Goals:**

- Changing the Go representation or public API of Prelude `Option`/`Result`.
- Replacing the seed-time spell analysis with a general Go type-inference
  implementation.
- Changing orphan/coherence rules or method resolution precedence.

## Decisions

- **Normalize inferred types at the declaration boundary even without an
  annotation.** The spell-out depends on the receiver node carrying a concrete
  type. The fix is to make the typed expression that reaches codegen fully
  substituted, rather than teaching codegen to reconstruct solutions. This
  keeps one owner for type resolution (inference) and matches the existing
  contract comment that codegen dispatches from the annotation and never
  reconstructs it. Alternative considered: reconstruct the receiver element type
  in codegen from the call's own result annotation; rejected as a second,
  partial inference engine.
- **Keep the explicit-type-argument mechanism and extend it to chains.** The
  chain already produces the right helper shape for the inner call; only the
  argument prefix is missing. No new lowering is introduced.
- **Turn the two fallbacks into diagnostics.** When `receiverImplCallTypeArgs`
  cannot produce exactly `SpellCount` arguments, and when
  `translateImplMethodCall` returns no candidate, generation SHALL report the
  method and source position instead of emitting `recv.Method()` or a plain
  call. Alternative considered: keep the plain-call fallback and rely on Go's
  partial inference; rejected because Go cannot infer parameters that appear
  only inside interface-shaped containers, which is the whole reason the
  mechanism exists.
- **Cover the reported shape and its variants directly.** Regression tests
  should assert the emitted helper arguments for `Receive().Unwrap().Unwrap()`
  and build the generated package, with receiver types taken from both the
  Prelude and an imported package.

## Risks / Trade-offs

- [Breadth of the normalization change] → Apply substitution uniformly at the
  expression/declaration boundaries that already normalize, and verify with the
  existing bootstrap regression suite plus a self-hosting sync.
- [New diagnostics may reject code that previously "compiled" after a manual
  annotation] → Scope the diagnostic to the case where the helper genuinely
  requires explicit arguments; keep non-generic helpers and helpers whose
  parameters are inferable on the existing plain-call path.
- [Generated files drift] → Regenerate the affected `zz_*.gen.go` in bootstrap
  order and include them in the change.

## Migration Plan

No data or API migration. Implementation regenerates the self-hosted packages
touched by the fix and runs bootstrap sync; rollback is reverting the change.

## Open Questions

- Whether the normalization boundary should live in `inferBlockBind` alone or be
  shared with other binding forms (package-level bindings, match arms). Resolve
  during implementation by adding the boundary that the failing cases exercise
  first, then widening only if a covered regression still fails.
