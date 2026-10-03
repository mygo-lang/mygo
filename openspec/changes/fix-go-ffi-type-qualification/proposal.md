# Proposal

## Why

Go FFI signatures and MyGO package signatures can refer to the same imported Go named type using different spellings. Inference currently drops the Go import alias in some paths, so a valid callback that passes `mail.Message` to a backend is rejected as a mismatch between `Message` and `mail.Message`.

## What Changes

- Preserve consistent package-qualified identity for named Go types across Go FFI signatures, MyGO package signatures, and callback inference.
- Ensure valid values and callbacks using the same imported named Go type unify successfully, while distinct named types remain distinguishable.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `bootstrap-ffi-boundary`: Specify consistent identity and unification for named Go types across imported package and callback boundaries.

## Impact

The type inference and Go import metadata paths in `internal/mygo/typeinference/` are expected to be affected. No source language syntax or generated Go API changes are intended.
