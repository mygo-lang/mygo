## Why

MyGO uses `error` as the spelling of Go's built-in error type, while the
self-hosted inference model canonicalizes the same value as `Error`. The
absence of a canonical alias makes otherwise equivalent spellings fail to
unify, and generated signatures can leak the internal `Error` identifier
instead of emitting Go's `error` builtin.

## What Changes

- Establish `Error` as the canonical internal constructor for Go's `error`
  builtin.
- Canonicalize source or imported `error` type names to that constructor during
  self-hosted type inference, both with and without an inference environment.
- Ensure `TCon("Error")` and `TCon("error")` remain unifiable as the same
  builtin type.
- Lower canonical `Error` type expressions in generated Go declarations and
  calls to `error`, including parameters and tuple result components.
- Preserve ordinary user-declared identifiers named `Error` or `error` unless
  they resolve to the Go builtin.

## Capabilities

### New Capabilities

- `bootstrap-error-alias`: Canonical aliasing and Go lowering for the builtin
  `error` type in the self-hosted inference and generation pipeline.

### Modified Capabilities

## Impact

- `internal/mygo/typeinference2`: type-expression construction and TCon
  unification.
- `internal/mygo/codegen2`: primitive type lowering and generated function
  signatures.
- Bootstrap regeneration and tests for the modified self-hosted packages.
- Existing MyGO programs declaring an incompatible user-defined `Error` may
  have changed resolution precedence; user declarations should continue to
  shadow the builtin.
