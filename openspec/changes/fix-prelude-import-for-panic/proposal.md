# Proposal

## Why

Generated Go code can call a prelude function such as `Panic` without importing the prelude package when that function's FFI signature is represented using Go built-in types. The generated package then fails to compile because the call is unresolved.

## What Changes

- Require generated Go files that reference prelude declarations to include the prelude import, including references whose declaration types are lowered to Go built-ins.
- Add regression coverage for direct calls to `Panic` and similar prelude helpers.

## Capabilities

### New Capabilities
- `prelude-imports`: Import behavior for generated Go code that references prelude declarations.

### Modified Capabilities

## Impact

- Affects codegen2 prelude-reference detection and generated Go imports.
- Adds codegen regression coverage; no MyGO syntax or public API changes.
