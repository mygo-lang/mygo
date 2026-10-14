## Why

Valid MyGO code that consumes a third package's type through an imported API can generate an unqualified Go type argument and fail with `undefined: Identity`. The consumer should not need an otherwise unnecessary source import and type annotation to make an inferred `Result` method call compile.

## What Changes

- Preserve the defining package's canonical identity through imported function and impl-method return types and subsequent generic inference.
- Supply file-local Go imports and collision-free aliases for foreign MyGO types referenced by emitted inferred type arguments and typed declarations, including nested type shapes.
- Reuse explicit imports, omit unused imports and self-imports, and prevent silent loss of foreign qualification during contextual type rendering.
- Add a three-package regression with no direct consumer import of the defining package, checking generated Go and runtime behavior.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `bootstrap-cross-package-method-dispatch`: Preserve third-package type identity in imported API results and emit the necessary qualified generic arguments and file-local imports.

## Impact

- Bootstrap inference and imported signature projection in `internal/mygo/typeinference2`.
- File import planning and contextual type rendering in `internal/mygo/codegen2`.
- Bootstrap integration tests in `internal/mygo/compiler` and compiler-generated Go artifacts refreshed through bootstrap synchronization during implementation.
- No language syntax changes, new runtime dependencies, or manual edits to generated application files.
