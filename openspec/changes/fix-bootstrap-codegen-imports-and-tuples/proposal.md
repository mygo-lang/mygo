## Why

The bootstrap compiler accepts a MyGO project source but emits Go
that cannot compile: a top-level tuple-valued `switch` is returned as one
anonymous struct, while per-file Prelude imports are both omitted for method
dispatch and emitted when only erased interface signatures reference Prelude
types. This prevents bootstrap from serving as a reliable compilation path for
ordinary multi-file applications.

## What Changes

- Preserve multi-result return lowering when a top-level function result is a
  tuple-valued `switch` expression.
- Add the Prelude dot import to generated files that reference Prelude
  dictionary helpers through inherent or typeclass method dispatch.
- Omit the Prelude dot import from generated files whose output has no Prelude
  identifiers, including files where Prelude types occur only in erased
  interface declarations.
- Add focused bootstrap regressions that compile the generated Go output.

## Capabilities

### New Capabilities

- `bootstrap-codegen-correctness`: Defines bootstrap code-generation behavior
  for tuple returns and per-file Prelude imports.

### Modified Capabilities

- `bootstrap-workflow-parity`: Bootstrap builds must produce Go-compilable
  per-file output when Prelude helpers are used or are absent from emitted Go.

## Impact

- Affected implementation: `internal/mygo/codegen2` and bootstrap compiler
  tests under `internal/mygo/compiler`.
- No MyGO syntax, public CLI option, runtime API, or third-party dependency
  changes are expected.
