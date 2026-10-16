## Context

See `proposal.md` for motivation. Current state that shapes the approach:

- `inferTypeAs` (`internal/mygo/typeinference2/infer.mygo`) unifies the operand
  type with the ascribed type. On failure it returns a source-located diagnostic
  when either side contains an array (`arrayTypeContains`), which is what
  currently rejects `Int[32] as Slice[Int]`.
- Mono types: arrays are `TArray(elem, length)`; `Slice[T]` is
  `TApp(TCon(Slice), [T])`. Go rendering already maps both forms.
- `translateTypeAsAst` (`internal/mygo/codegen2/translate_ast.mygo`) emits a Go
  conversion `Target(value)` for every non-literal ascription. The `goast`
  package has no slice-expression builder today.
- Typed expression nodes carry their inferred `Type`, so codegen can observe both
  the operand type and the ascribed target type without a new AST node.
- Go only permits slicing an array when the operand is addressable; `f()[:]` and
  `[3]int{...}[:]` are compile errors, while `value[:]`, `s.Field[:]`, and `p[:]`
  (array pointer) are fine.

## Goals / Non-Goals

**Goals:**

- Accept `T[N] as Slice[T]` in type inference and lower it to the Go slice
  expression `value[:]`.
- Unify element types, including nested element types (`Int[3][2] as Slice[Int[2]]`).
- Emit valid Go for non-addressable array operands without silently changing the
  behavior of addressable ones.

**Non-Goals:**

- No new indexing or slicing expression syntax; the parser keeps its current
  postfix handling.
- No Slice-to-array conversion and no implicit array-to-slice conversion.
- No length-polymorphic prelude helper (array lengths are not generic).

## Decisions

1. **Recognize the conversion in `inferTypeAs`, not in `unify`.** `unify` models
   type equality; `T[N]` and `Slice[T]` are not equal. Adding the rule next to the
   existing array diagnostic keeps every other context unchanged. Alternative
   considered: let `inferExprExpected` accept `TArray` where a `Slice` is
   expected. Rejected because that context also feeds literal inference, where the
   expected type deliberately decides literal shape.

2. **Shape-based detection.** The conversion applies when the target is
   `TApp(TCon(Slice), [elem])` and the operand type is `TArray(arrayElem, _)` with
   `arrayElem` unifiable with `elem` under the current substitution. The result
   type is the substituted target; the operand node keeps its own array type so
   codegen can recognize the pair.

3. **Codegen keys off the same types.** `translateTypeAsAst` checks the typed
   operand `Type` (array) against the target (slice) and emits a slice expression
   instead of a conversion. Alternative considered: add a dedicated AST node or a
   marker in the typed tree. Rejected as heavier than needed; the type pair is
   already carried by the typed AST.

4. **Addressability-aware lowering.** For addressable operands emit `value[:]`,
   preserving Go aliasing (the slice shares the operand's backing array). For
   non-addressable operands (call results, array literals) emit an immediately
   invoked function that copies the value into a local and slices the local, for
   example `func() []int { tmp := f(); return tmp[:] }()`. There is nothing to
   alias for a fresh value, so this changes no observable behavior while keeping
   the generated Go valid.

## Risks / Trade-offs

- [Addressability] Slicing a non-addressable array value is invalid Go -> covered
  by decision 4 and validated by an end-to-end test that ascribes a call result.
- [Error surface] Element-type mismatches must keep reporting the conflicting
  types -> the new branch only accepts when element unification succeeds and
  otherwise falls through to the existing array-aware diagnostic.
- [Aliasing expectations] A slice produced from a variable aliases that variable,
  while one produced from a fresh value does not -> documented in
  `docs/compiler/collections.md`; both match plain Go semantics.

## Migration Plan

1. Change `typeinference2`, then re-sync `typeinference2` output.
2. Change `codegen2` (plus the `goast` builder), then re-sync `codegen2`.
3. Re-sync `compiler` and rebuild the CLI before compiling any non-prelude source.
4. Roll back by reverting the three sources and their regenerated `zz_*.gen.go`
   files; no data migration and no on-disk format change is involved.

