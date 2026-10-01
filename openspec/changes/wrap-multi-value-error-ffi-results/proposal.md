## Why

Go APIs often return several useful values followed by an `error`. Bootstrap currently only converts a single value plus `error` into `Result`, leaving larger signatures awkward to consume as an ordinary expression and causing tuple-binding arity failures in affected call sites.

## What Changes

- Extend the Go FFI error boundary so signatures `(T1, T2, ..., error)` with at least two non-error values are exposed as `Result[(T1, T2, ...), Error]`.
- Preserve explicit tuple destructuring as an opt-in to the raw Go multi-return assignment, including the trailing error.
- Preserve existing `(T, error)`, lone `error`, and `(T, bool)` boundary behavior.

## Capabilities

### New Capabilities

### Modified Capabilities
- `bootstrap-ffi-boundary`: broaden automatic Result wrapping to multiple success values followed by a trailing error while preserving raw tuple binding.

## Impact

- Affects Go FFI signature inference and error-call lowering in the bootstrap compiler, including package functions and imported methods.
- Affects generated Result payload typing and tuple construction/deconstruction at the Go boundary.
- Updates the bootstrap FFI boundary specification and its compiler conformance tests; no new dependency or public Go API is introduced.
