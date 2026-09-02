## Context

See proposal.md for motivation and the bootstrap-workflow-parity delta for the
required behavior. `typeinference2` has distinct public entry points for
single-file inference, ordinary package inference, raw external declarations,
and already-inferred external package information. Each currently assembles an
environment and symbol table locally. The alias mismatch occurs when a symbol
table resolves fields against the import-only environment rather than the
environment that has predeclared package aliases.

## Goals / Non-Goals

**Goals:**

- Give all package inference modes one authoritative setup sequence for import
  seeding, local predeclaration, and local-symbol derivation.
- Ensure field and variant symbol types see package-local aliases independent
  of declaration order.
- Retain the public entry-point semantics needed by bootstrap compilation and
  focused code-generation tests.

**Non-Goals:**

- Remove `InferFile`, `InferPackage`, or externally consumed explicit-FFI
  wrappers.
- Merge raw external declarations with an already-inferred `PackageInfo`;
  those inputs have distinct correctness and performance semantics.
- Alter alias syntax, nominal `type` behavior, code generation, or imports.

## Decisions

### Centralize shared inference preparation, not public entry points

Introduce an internal preparation helper that accepts the effective current
declarations, Go package signatures, MyGO package imports, and a caller-supplied
base environment. It SHALL seed imports, predeclare local aliases/types/
functions/enums/impls, then build symbols using that predeclared environment.
It returns the prepared environment and symbol collection for the caller to
place into its `InferState`.

`InferFile` and ordinary package inference use an empty base environment;
`InferPackageWithExternal` supplies combined raw declarations; and
`InferPackageWithExternalInfo` supplies its external-info environment as the
base. The separate entry points retain ownership of declaration grouping,
typed-source splitting, diagnostics, and external field context.

Alternative considered: replace all APIs with one input-union entry point.
Rejected because callers distinguish raw declarations from already inferred
external state, and a union-style API would obscure that difference without
reducing meaningful behavior.

### Resolve every structural symbol against the predeclared environment

The symbol build step uses the completed local predeclaration environment.
This makes aliases transparent in struct fields and enum variant fields while
also preserving existing imported type and FFI qualification behavior.

Alternative considered: add alias expansion only inside struct-literal
unification. Rejected because aliases can occur in all structural symbol types;
fixing the consumer would leave enum variants and selectors dependent on source
ordering.

### Test package-level behavior through external-declaration inference

Add a regression test with separate declaration sources: a field that uses
`RunID`, an initializer using a string, and a later `RunID = String` alias. It
uses `InferPackageWithExternal` so it exercises the bootstrap production path.

Alternative considered: only test `InferFile`. Rejected because that wrapper
does not model the cross-file ordering or external-context path that failed.

## Risks / Trade-offs

- [Shared helper changes setup ordering for all inference entry points] -> Keep
  existing base environments and only centralize their current sequencing;
  run focused typeinference2 and bootstrap validation.
- [External package environments contain newest-first shadowing] -> Let the
  external-info entry point continue constructing its existing batched base
  environment before invoking the helper.
- [Generated Go for self-hosted modules becomes stale] -> Regenerate affected
  generated source through the repository's bootstrap workflow and validate it.

## Migration Plan

No source migration is required. Apply the refactor, run the targeted
regressions and bootstrap sync/build validation, and commit regenerated output
when the repository workflow requires it. If a new setup regression appears,
revert the helper adoption while retaining no user data or generated-file
migration burden.
