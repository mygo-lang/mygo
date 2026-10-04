# Proposal

## Why

Dynamic MyGO typeclass values already have generated wrappers and per-implementation constructors, but explicit `as` conversions can still become invalid Go interface conversions, and values passed to a known dynamic interface target require users to write the conversion themselves. The compiler should route both forms through the same typeclass evidence and wrapper metadata so dynamic values are valid, predictable, and convenient to use.

## What Changes

- Generate dynamic boxing for explicit conversions to a dynamic typeclass interface instead of emitting a Go type cast.
- Infer dynamic boxing from a known expected type at compatible call arguments and other supported target-typed expression sites, without requiring `as`.
- Require a unique compatible implementation and available dynamic constructor metadata; preserve the selected implementation in the wrapper.
- Keep ordinary Go assignability and static MyGO typeclass resolution unchanged. Expressions without a known target type, including unannotated branch joins, remain outside this change.

## Capabilities

### New Capabilities

<!-- None. -->

### Modified Capabilities

- `language/dynamic-typeclass-values`: Specify explicit and target-directed implicit boxing of compatible concrete values into dynamic typeclass wrappers.

## Impact

Affected areas include type inference/type ascription, expected-type propagation, dynamic implementation selection, generated Go expression lowering, and the dynamic constructor metadata consumed across package boundaries. Existing dynamic wrapper and dispatch APIs remain the runtime representation.
