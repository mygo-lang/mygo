## Why

Bootstrap package inference prepares type aliases, declarations, and field
symbols through duplicated setup paths.  Some paths build structural symbols
before aliases are available, causing a valid alias such as `type RunID =
String` to fail when a struct field is initialized with a string literal.

## What Changes

- Centralize bootstrap inference environment and symbol-table preparation so
  every package inference mode resolves local aliases before deriving field and
  enum-variant symbol types.
- Keep the existing public inference entry points, but make single-file and
  package wrappers delegate to the shared setup rather than reimplementing it.
- Preserve the distinct handling of raw external declarations and previously
  inferred external package information.
- Add regression coverage for a forward, cross-file type alias used by a
  struct field literal, including the bootstrap-with-external-declarations
  path.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `bootstrap-workflow-parity`: Bootstrap package inference accepts valid
  package-local type aliases consistently across source-file ordering and
  external declaration context.

## Impact

- `internal/mygo/typeinference2/types.mygo` and its generated Go output.
- `internal/mygo/typeinference2/infer_test.go` regression coverage.
- Bootstrap `sync` and `build` users with package-local aliases referenced by
  struct fields or enum variant fields.
