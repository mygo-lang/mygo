## Why

Imports are currently collected across a whole MyGO package, even though the compiler emits one Go file per `.mygo` source. This leaks imports into generated files that do not use them and makes it unclear which import names are visible while checking an individual source file.

## What Changes

- Make explicit MyGO imports, including Go FFI imports, visible only in the source file that declares them.
- Preserve canonical type identity and method/impl metadata across package-local function signatures, so callers can use returned values without importing their defining Go package.
- Emit imports independently for each generated Go file, based on the imports required by that file's emitted declarations and expressions.
- Keep package-wide declarations and typeclass implementations available across source files.

## Capabilities

### New Capabilities
- `per-file-import-resolution`: Defines file-scoped import visibility, cross-file propagation of qualified type identity, and per-generated-file import emission.

### Modified Capabilities

## Impact

- Affects package loading and type inference environments, especially source-file-aware import resolution and imported function signature metadata.
- Affects Go code generation import collection for each generated file.
- Requires regression coverage for Go FFI types and methods, MyGO functions and typeclass impls, explicit import aliases, and unused imports.
