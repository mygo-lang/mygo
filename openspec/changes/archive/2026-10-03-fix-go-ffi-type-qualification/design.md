# Design

## Context

See proposal.md for the triggering failure and impact. Go FFI type loading and imported MyGO package signature loading both construct inference types, so package identity must remain consistent at both boundaries.

## Goals / Non-Goals

**Goals:**
- Establish a canonical identity for each named Go type that is stable across import aliases and loaded package signatures.
- Preserve Go's nominal distinction between separately declared named types.
- Keep generated Go selectors/import behavior aligned with inference identity.

**Non-Goals:**
- Changing source syntax or making structurally similar named Go types interchangeable.
- Redesigning general Go type alias handling beyond what is needed for consistent identity.

## Decisions

- Normalize imported named types using their Go package identity, then map that identity to the source import alias at the boundary where MyGO signatures are qualified. This avoids using an unqualified type name, which can collide across packages, and avoids treating source aliases as nominal identity.
- Apply the same normalization recursively to function parameters, results, and callback signatures. Fixing only top-level Go selector inference could leave nested callback types inconsistent.
- Retain distinct identities for Go aliases according to existing FFI alias semantics; verify alias behavior while implementing rather than collapsing all aliases to their underlying type.

Alternatives considered:
- Accept unqualified and qualified names as equivalent during unification. Rejected because `Message` is ambiguous across packages and would weaken nominal distinctions.
- Change the affected application source to use a different annotation or conversion. Rejected because both signatures already describe the same imported Go type, so the compiler's identity resolution is the inconsistent part.

## Risks / Trade-offs

- [Package import paths and source aliases can differ] → Keep canonical package identity separate from the user-facing alias and test aliased imports.
- [Type qualification changes can affect other FFI call sites] → Cover nested signatures and ensure distinct named types remain incompatible.

## Migration Plan

No source migration is expected. Re-run bootstrap inference for the reported package and relevant compiler regression coverage after implementation; revert the normalization change if it causes unrelated FFI signatures to diverge.
