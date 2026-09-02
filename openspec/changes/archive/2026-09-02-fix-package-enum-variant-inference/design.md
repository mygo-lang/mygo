## Context

See `proposal.md` for motivation and
`specs/package-enum-variant-inference/spec.md` for the required behavior.
`InferFile` initializes `InferState.PkgInfo` with the file declarations, but
package-level inference entry points currently initialize it as absent.
Named variant struct-literal inference consults that metadata to map a variant
constructor back to the enclosing enum.

## Goals / Non-Goals

**Goals:**

- Make all package-level inference entry points supply current-package
  declaration metadata consistently.
- Preserve existing named-variant field validation and inference behavior.
- Prove the external-declaration bootstrap path accepts a named variant in an
  enum-typed slice field.

**Non-Goals:**

- Change enum syntax, variant representation, code generation, or imports.
- Broaden type-inference behavior unrelated to named enum variants.

## Decisions

### Represent the current package in `InferState.PkgInfo`

Each package inference entry point will initialize `PkgInfo` from its effective
current declaration set: `allDecls` for ordinary package inference and
`combined` where external declarations participate in inference. This keeps
the existing named-variant lookup path and produces the enclosing enum type.

Alternative considered: infer `EnumName.VariantName` as a standalone nominal
type and add subtype-like conversion to `EnumName`. Rejected because variants
are already specified and generated as enum values, and it would introduce a
new type-system relation solely to compensate for missing context.

### Test the production package-with-external path

Add a focused `InferPackageWithExternal` regression test that declares
`Content`, `Message.Content: Slice[Content]`, and `Content.Text { ... }`.
This exercises the initialization path used by bootstrap compilation instead
of only the single-file convenience API.

Alternative considered: reuse only an `InferFile` test. Rejected because that
entry point already supplies `PkgInfo` and cannot prevent this regression.

## Risks / Trade-offs

- [External declarations shadow a same-named local enum] -> Use the current
  package declarations as the authoritative lookup scope and retain existing
  environment-based constructor resolution.
- [Behavior differs among package entry points] -> Initialize `PkgInfo`
  consistently and cover the external package path with a regression test.

## Migration Plan

No source migration is required. The change repairs an inference path that
previously rejected valid programs. If a regression appears, revert the state
initialization change; no generated artifact or user data needs migration.
