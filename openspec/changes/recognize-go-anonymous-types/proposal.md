## Why

MyGO currently resolves imported Go signature strings as package-qualified
names. A structurally anonymous type such as the interface returned by
`renderer/html.WithHardWraps` is therefore split at an internal dot and becomes
a bogus nominal type. Valid Go calls then fail with a `type-mismatch`, even
though Go's own assignability rules accept the argument.

## What Changes

- Recognize anonymous Go type syntax in FFI signatures before attempting
  qualified-name resolution, starting with non-empty anonymous interfaces and
  preserving existing handling for anonymous functions, composites, channels,
  and slices.
- Preserve a stable Go identity for anonymous signature types so repeated uses
  of the same Go type unify, and so boundary checks can consult the real Go
  type rather than a reconstructed name.
- At Go FFI call boundaries, accept anonymous interface values when Go's
  `types.AssignableTo` accepts them for the parameter type, including method
  sets promoted through embedded interfaces.
- Keep generated Go passing the original value directly. No conversion,
  helper function, or workaround should be required in consumer code.
- Add regression coverage using a local Go fixture shaped like Goldmark's
  `WithHardWraps` / `WithRendererOptions` pair, plus negative cases for
  incompatible anonymous interfaces.
- Preserve existing behavior for named Go types, ordinary MyGO unification,
  and Go types that are not assignable.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `bootstrap-ffi-boundary`: anonymous Go interface types in FFI signatures are
  recognized and participate in Go assignability at call boundaries instead of
  being misparsed as package-qualified names.

## Impact

- `internal/mygo/compiler/go_ffi_import.go`: collect and retain enough identity
  for anonymous types produced by `go/types`.
- `internal/mygo/typeinference2/types.mygo`: classify anonymous type syntax
  before qualified-name splitting.
- `internal/mygo/typeinference2/infer.mygo`: route anonymous interface
  arguments through Go assignability at FFI call boundaries.
- `internal/mygo/typeinference2/go_assignability.go`: resolve anonymous source
  types through the Go type service when checking assignability.
- Compiler tests and a local fixture covering anonymous embedded interfaces;
  no MyGO source syntax or generated ABI change is intended.
- Unblocks `commune-backend/internal/content/rich.mygo`, where Goldmark's
  `WithHardWraps` and `WithUnsafe` return anonymous interfaces passed to
  `WithRendererOptions(...renderer.Option)`.
