## Context

The self-hosted compiler already represents Go's builtin error interface as
`Error` when it imports Go type information (`GoTypeName`), while source type
construction preserves the spelling found in MyGO syntax. Its unifier
compares constructor names literally. Separately, declaration type lowering
kept its own primitive-name mapping with no `Error` entry, so a signature
written with the language alias could become a Go identifier rather than the
builtin.

The boundary mechanism fixes both sides: every Go FFI `error` becomes
`Error`, every `Error`/`error` in a declaration lowers to Go `error`, and the
unifier canonicalizes the Go spelling at entry so the two spellings never need
special-cased comparison. Code generation must still preserve explicit generic
type parameters.

## Goals / Non-Goals

**Goals:**

- Provide one canonical internal constructor for the builtin and canonicalize
  the accepted lower-case Go spelling at the FFI boundary and unification
  entry, not at source construction.
- Make constructor comparison alias-safe so types already represented by either
  spelling can unify, without a comparison special case.
- Lower both source spellings to `error` in generated Go declaration types.
- Keep explicit type parameters, including one named `Error`, ahead of builtin
  lowering on the declaration AST path.
- Regenerate affected bootstrap modules from their MyGO sources.

**Non-Goals:**

- Do not introduce general type aliases or recursive canonicalization for every
  builtin name; the spelling table only covers the existing primitive set.
- Do not change Result, Option, or other Prelude representation choices.
- Do not redesign generated-file imports or Go FFI lowering beyond the error
  alias behavior needed above.

## Decisions

### One shared primitive spelling table

Add a `GoPrimitivePair` table (Go name ↔ MyGO constructor name) to
typeinference2 covering the existing primitive set, with `error`/`Error` as a
single pair. Two lookups derive from it:

- `PrimitiveGoSpelling(name)` accepts either spelling and returns the Go name
  used in generated code (`Error` and `error` both map to `error`).
- `PrimitiveMyGoSpelling(name)` accepts the Go spelling and returns the
  canonical MyGO constructor name (Go FFI `error` becomes `Error`).

The three use sites become lookups on this one table instead of parallel
switch entries:

1. `GoTypeName` (Go FFI → MonoType) resolves primitives through
   `PrimitiveMyGoSpelling`, producing the canonical `Error` constructor.
2. The unifier canonicalizes both sides at entry (`normalizeTypeEntry`):
   any remaining Go-spelled builtin `TCon` is rewritten through
   `PrimitiveMyGoSpelling`, so the TCon/TCon case keeps a plain name
   comparison and no alias logic lives in structural unification.
3. Codegen's declaration primitive lowering delegates to
   `PrimitiveGoSpelling`, so both spellings lower to `error` across string and
   AST paths for parameters, results, tuple components, and nested type
   expressions.

Source type construction (`typeFromAST` and friends) is intentionally left
unchanged: the written spelling survives until it reaches a boundary, and
user declarations that shadow the builtin keep resolving through the
environment lookup.

Alternative considered: a dedicated canonical-name helper applied at every
source-construction site. That scattered the alias across the constructor
paths and duplicated the Go↔MyGO primitive mapping that already existed in
codegen, so the abstraction was instead placed in one shared table.

Alternative considered: accepting both spellings in the unifier's TCon/TCon
comparison. It fixes direct unification but leaves every other name comparison
unprotected and duplicates the alias relationship; entry normalization through
the shared table covers all paths once.

### Preserve explicit type parameters before builtin lowering

The AST declaration path currently checks unit and primitive names before
checking whether a bare named type is one of the declaration's type parameters.
The string path already gives type parameters precedence. Add the same type
parameter check to the AST path before unit, special, HKT, and primitive
lowering. This is necessary to avoid turning a generic parameter named `Error`
into the Go builtin when the new primitive entry is enabled.

Alternative considered: reserve the name `Error` from generic declarations.
That would be a language compatibility change and is broader than the alias
bug being fixed.

## Risks / Trade-offs

- [Entry normalization could rewrite a user constructor spelled `error`] ->
  Source construction preserves the written spelling and resolves user
  declarations through the environment first; unification entry only rewrites
  the exact Go builtin spelling (`error` to `Error`) through the
  one-directional table lookup.
- [Adding `Error` primitive lowering could affect a user type with the same
  name] -> Preserve explicit type parameters first, and rely on the established
  language-level status of `Error` as the Go boundary alias. Cover both a
  builtin signature and a type-parameter case in tests.
- [AST and string declaration paths can drift] -> Add equivalent type-parameter
  precedence and assertion coverage for both paths where practical, then keep
  generated outputs synchronized.
- [Bootstrap regeneration can produce large diffs] -> Limit source edits to the
  identified modules and run the documented sync command for each modified
  self-hosted package before running package tests.

## Migration Plan

1. Add inference canonicalization and alias-aware unification tests.
2. Add declaration lowering tests for `Error`, lower-case `error`, tuple
   results, and an explicit `Error` type parameter.
3. Implement the MyGO changes in `typeinference2` and `codegen2`.
4. Regenerate each modified self-hosted package with the bootstrap sync command
   shown in tasks.
5. Run focused Go tests first, then the broader package tests. Rollback is a
   revert of the source and regenerated `zz_*.gen.go` files; no data or API
   migration is required.

## Open Questions

None.
