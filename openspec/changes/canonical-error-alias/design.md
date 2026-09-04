## Context

The self-hosted compiler already represents Go's builtin error interface as
`Error` when it imports Go type information, but its source type constructor
preserves the spelling found in MyGO syntax. Its unifier compares constructor
names literally. Separately, declaration type lowering has a mapping for
primitive names but omits the canonical `Error` entry, so a signature written
with the language alias can become a Go identifier rather than the builtin.

The production inference pipeline has the intended precedent: it canonicalizes
lower-case `error` to `Error` and compares compatible constructor names. The
self-hosted pipeline needs the same boundary behavior, while code generation
must preserve explicit generic type parameters.

## Goals / Non-Goals

**Goals:**

- Provide one canonical internal constructor for the builtin and canonicalize
  the accepted lower-case Go spelling when constructing source types.
- Make constructor comparison alias-safe so types already represented by either
  spelling can unify.
- Lower both source spellings to `error` in generated Go declaration types.
- Keep explicit type parameters, including one named `Error`, ahead of builtin
  lowering on the declaration AST path.
- Regenerate affected bootstrap modules from their MyGO sources.

**Non-Goals:**

- Do not introduce general type aliases or recursive canonicalization for every
  builtin name.
- Do not change Result, Option, or other Prelude representation choices.
- Do not redesign generated-file imports or Go FFI lowering beyond the error
  alias behavior needed above.

## Decisions

### Add a small canonical-name helper to typeinference2

Add a package-local helper that maps only the exact lower-case source name
`error` to `Error`. Apply it to bare named types in the environment-free
constructor, the constructor that knows explicit type parameters, and the
environment-aware constructor.

Ordering matters:

1. HKT/type-parameter detection remains first so a type parameter named `error`
   is not rewritten.
2. Existing environment lookup remains next in the environment-aware path so a
   resolved declaration can shadow the builtin spelling.
3. The fallback constructor is canonicalized.

The environment-free path applies the helper to a zero-argument named type;
application constructors retain their existing behavior.

Alternative considered: leave construction unchanged and only special-case
comparison. This would make direct constructor unification work but would still
allow one inferred type to expose a noncanonical spelling and would not mirror
the production pipeline.

### Compare constructor names through the same helper

Use canonical constructor-name equality in the self-hosted unifier's TCon/TCon
case. This covers types arriving from Go imports or internal constructors that
did not pass through the source type-expression helpers. Compare only the bare
constructor names; existing application handling remains responsible for
constructor and argument structure.

Alternative considered: add an alias table to the unifier. A one-name builtin
does not need a new data structure, and the helper keeps construction and
comparison consistent.

### Treat `Error` as a primitive declaration type

Add `Error` to the declaration-path primitive mapping with Go result `error`.
Also accept the exact lower-case source spelling in that primitive mapping so
both language aliases lower the same way. This affects string and AST lowering
for function parameters, results, tuple components, and nested declaration type
expressions without adding a special case to declaration emitters.

Alternative considered: special-case only function signatures. The primitive
mapping already exists and declaration emitters intentionally reuse it, so a
signature-only branch would diverge from surrounding type lowering.

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

- [Canonicalizing a resolved user alias could hide a package-local
  declaration] -> In the environment-aware path, perform environment lookup
  before canonicalizing the fallback. Keep the helper exact and one-directional
  (`error` to `Error` only).
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
