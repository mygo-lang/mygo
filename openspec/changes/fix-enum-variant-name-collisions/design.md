## Context

See proposal.md for motivation. The source form already carries the complete
constructor identity: `EnumName.VariantName`. The failure arises because parts
of the inference implementation reduce that identity to `VariantName` before
retrieving a constructor scheme. The bootstrap path is directly affected; the
production path needs equivalent collision regression coverage and any
necessary resolution correction on its independently maintained branch.

## Goals / Non-Goals

**Goals:**

- Preserve full enum ownership while resolving qualified variant construction.
- Keep bare variant names usable only in contexts that already have a target
  enum for disambiguation.
- Produce two self-contained commits: production first, bootstrap second.

**Non-Goals:**

- Changing MyGO enum syntax, payload layout, or generated Go representation.
- Defining ambiguous unqualified construction semantics.
- Reworking unrelated import or pattern-matching behavior.

## Decisions

### Resolve qualified construction with a qualified constructor key

Both compiler paths will treat a qualified literal as a lookup keyed by its
enum and variant names. Bootstrap will register or derive a local
`EnumName.VariantName` constructor scheme and use that key from named-variant
literal inference. The bare key remains available only for existing pattern
resolution that is constrained by the switch target type.

This is preferred over selecting the first matching bare variant because the
source itself is explicit, and it avoids declaration-order dependence. It is
also preferred over banning duplicate variant names because reuse across
independent enums is a valid and useful source-language pattern.

### Keep production and bootstrap fixes independent

The production commit will contain only production inference/codegen changes
and production regression tests. The bootstrap commit will contain only
`typeinference2`/bootstrap-generation changes and bootstrap regression tests.
Each commit must pass its own focused tests and must not depend on files from
the other commit, allowing the production commit to be cherry-picked onto the
older compiler branch.

### Test the collision matrix at the compiler boundary

Regression fixtures will put a named-payload `Running` and a zero-payload
`Running` in one package, construct the qualified payload variant, and verify
successful inference/compilation and generated Go validity where supported.
A second focused case will use different named payload fields to prove the
field schema comes from the qualified owner.

## Risks / Trade-offs

- [Bare constructor behavior is shared with patterns] -> Keep bare lookup
  unchanged for target-type-guided patterns; add a focused pattern regression
  if the production branch's fix touches that path.
- [Bootstrap generated sources must remain self-hostable] -> Edit MyGO source
  and regenerate checked-in `zz_*.gen.go` using the established bootstrap
  workflow; run bootstrap package tests afterward.
- [Old branch diverges from current production source] -> Restrict the first
  commit to the old branch's existing inference interfaces and tests, then
  cherry-pick it before applying the current-branch bootstrap commit.

## Migration Plan

1. Implement and verify the production-only change, then commit it as an
   independent conventional commit for cherry-picking.
2. Implement and verify the bootstrap-only change on the current branch, then
   commit it separately.
3. If either regression appears, revert only its corresponding commit; no
   source migration is required because this is a compiler correctness fix.
