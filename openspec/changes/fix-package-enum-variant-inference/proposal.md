## Why

Bootstrap compilation infers named enum-variant struct literals correctly for a
single file but loses their enclosing enum type during package-level inference.
This makes valid values such as `Content.Text { Text: text }` incompatible with
`Slice[Content]` fields, preventing multi-file packages from compiling.

## What Changes

- Preserve current-package declaration metadata in package-level inference state.
- Resolve named enum-variant literals to their enclosing enum type in all
  package inference entry points, including bootstrap compilation with external
  declarations.
- Add a regression test for a `Slice[Content]` struct field initialized with a
  `Content.Text { ... }` value through `InferPackageWithExternal`.

## Capabilities

### New Capabilities

- `package-enum-variant-inference`: Package-level type inference preserves and
  uses current-package enum declarations when typing named enum-variant
  literals.

### Modified Capabilities

- None.

## Impact

- `internal/mygo/typeinference2/types.mygo` package inference state setup.
- `internal/mygo/typeinference2/infer_test.go` regression coverage.
- Bootstrap users compiling packages with named enum variants stored in
  enum-typed collections or struct fields.
