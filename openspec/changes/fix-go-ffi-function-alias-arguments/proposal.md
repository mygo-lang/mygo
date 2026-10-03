# Proposal

## Why

Bootstrap inference currently models Go's error-only and unit-plus-error result shapes inconsistently with MyGO's `Result[(), Error]`. That prevents Go FFI functions and named callback types using either Go convention from interoperating naturally with MyGO Result values, including the SMTP `SendFunc` callback.

## What Changes

- Normalize Go FFI functions returning a lone `error` or `(struct{}, error)` to the MyGO result shape `Result[(), Error]` for unification.
- Apply the same mapping inside imported Go function types, including named callback aliases.
- Generate the required Go ABI adapter when a MyGO callback returns one `Result[(), Error]` value but the Go callback ABI expects `error` or `(struct{}, error)`.
- Continue rejecting callback signatures whose argument or result types are incompatible.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `bootstrap-ffi-boundary`: specify unit/error result unification and Go callback interoperability.

## Impact

Bootstrap Go signature loading, type inference, and code generation for Go FFI calls and callback values; regression coverage for error-only and unit/error results. Source syntax and public Go APIs remain unchanged.
