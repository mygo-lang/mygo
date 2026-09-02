## Why

Named enum-variant literals currently fail to advance inference state after
checking a field. A later empty collection can reuse an already-bound type
variable, yielding an unrelated type mismatch rather than its independent
fresh element type.

## What Changes

- Preserve the inference state produced by each named enum-variant field while
  recursively checking the remaining fields.
- Add regression coverage for a named enum-variant literal followed by an empty
  slice, ensuring their fresh type variables remain independent.

## Capabilities

### New Capabilities

- `bootstrap-enum-variant-inference`: Correct inference-state propagation for
  named enum-variant struct literals in the bootstrap type inference pipeline.

### Modified Capabilities

- None.

## Impact

- Affects `internal/mygo/typeinference2` and its tests.
- No source-language syntax, public API, or dependency changes.
