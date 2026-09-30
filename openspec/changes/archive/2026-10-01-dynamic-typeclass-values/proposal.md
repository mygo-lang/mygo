## Why

MyGO typeclass interfaces provide compile-time constraints and static dispatch, so values of different concrete types that implement the same interface cannot currently share a runtime collection. A library-level dynamic wrapper should retain each value's implementation and allow callers to use compiled implementations without rebuilding unchanged dependency packages.

## What Changes

- Add a normal library API for packaging a value with its selected typeclass implementation, so packages for the same interface can share one runtime type.
- Generate implementation adapters from existing `impl` declarations and use them for dynamic dispatch through the library API.
- Publish compiler-readable package metadata inside a generated Go source comment, including exported signatures and implementation adapters.
- Let callers read dependency metadata and generate their own dispatch when recompiled, without requiring dependency `.mygo` sources.
- Preserve generic typeclass methods where the dynamic adapter ABI can represent them, and keep ordinary static typeclass dispatch unchanged.
- Limit dynamic calls to single-package operations: the adapter may use the value stored in that package for a hidden-type operand, but callers do not supply a second hidden-type value. Hidden-type results may be repackaged when their implementation evidence is known.
- Do not add language syntax, change existing syntax behavior, or provide cross-package hidden-type equality.

## Capabilities

### New Capabilities
- `language/dynamic-typeclass-values`: Library-level packaging and dynamic dispatch for values implementing MyGO typeclass interfaces.

### Modified Capabilities

## Impact

- Library/runtime APIs for dynamic values and generated typeclass implementation adapters.
- Compiler code generation and metadata loading for precompiled MyGO dependencies; no parser or existing syntax changes.
- Generated Go package artifacts, which carry metadata in a comment-only generated source file.
