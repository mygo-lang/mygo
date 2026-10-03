# Proposal

## Why

The bootstrap compiler rejects valid Go FFI calls when an argument is assignable under Go's type rules but cannot be unified by MyGO's nominal HM type model. This blocks ordinary Go integrations such as passing `Ref[SMTPBackend]` (the Go pointer `*SMTPBackend`) to a function expecting a Go interface that the pointer implements.

## What Changes

- Define Go FFI argument compatibility using Go's own assignability rules at Go call boundaries.
- Preserve MyGO inference as the primary type inference engine; use a Go type service for Go-specific compatibility decisions, including pointer method sets and interface satisfaction.
- Keep incompatible Go named types rejected and preserve useful argument-location diagnostics.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `bootstrap-ffi-boundary`: Go FFI call arguments accept Go-assignable values, including pointer values represented by `Ref[T]` passed to implemented interface parameters.

## Impact

- Affects bootstrap FFI metadata/loading and call-argument checking in `internal/mygo/compiler` and `internal/mygo/typeinference2`.
- Uses Go's existing `go/types` service; does not introduce a second Go type inference engine or require `typeinference2` to model every Go type form.
- Adds regression coverage for pointer-to-interface assignments and rejection of non-assignable values.
