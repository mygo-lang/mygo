# Proposal

## Why

The bootstrap compiler rejects valid Go FFI calls when an argument is assignable under Go's type rules but cannot be unified by MyGO's nominal HM type model. This blocks ordinary Go integrations such as passing `Ref[SMTPBackend]` (the Go pointer `*SMTPBackend`) to a function expecting a Go interface that the pointer implements.

## What Changes

- Define Go FFI argument compatibility using Go's own assignability rules at Go call boundaries.
- Preserve MyGO inference as the primary type inference engine; use a Go type service for Go-specific compatibility decisions, including pointer method sets and interface satisfaction.
- Keep incompatible Go named types rejected and preserve useful argument-location diagnostics.
- Make an empty tuple pattern match the unit payload in a `Result`, so Go `error` calls represented as `Result[(), Error]` can be matched with `Ok(())`.
- Filter automatic prelude imports for generated external test files by actual per-file usage, preventing unused dot-imports.
- Resolve static inherent method calls on MyGO types imported from another MyGO package to the imported package's mangled helper symbols.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `bootstrap-ffi-boundary`: Go FFI call arguments accept Go-assignable values, including pointer values represented by `Ref[T]` passed to implemented interface parameters; unit payload patterns work for wrapped Go errors, external test output avoids unused prelude imports, and imported MyGO static inherent methods dispatch to their mangled helpers.

## Impact

- Affects bootstrap FFI metadata/loading and call-argument checking in `internal/mygo/compiler` and `internal/mygo/typeinference2`.
- Affects unit-pattern inference in `internal/mygo/typeinference2` and per-file generated import selection in `internal/mygo/codegen2` / bootstrap external-test generation.
- Affects code generation for package-qualified static inherent method calls across MyGO package boundaries.
- Uses Go's existing `go/types` service; does not introduce a second Go type inference engine or require `typeinference2` to model every Go type form.
- Adds regression coverage for Go assignability, unit patterns over `Result[(), Error]`, external-test prelude imports, imported static inherent method dispatch, and rejection of non-assignable values.
