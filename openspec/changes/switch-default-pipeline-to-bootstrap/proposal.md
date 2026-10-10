## Why

The CLI currently requires `--bootstrap` to select the self-hosted compiler, so ordinary sync and build commands still use the legacy pipeline. Make bootstrap the normal CLI workflow while retaining an explicit legacy sync escape hatch.

## What Changes

- **BREAKING**: Default `mygo sync` and `mygo build` to parser2, typeinference2, and codegen2.
- Add `mygo --legacy sync <path>` as the explicit legacy generation command; reject `--legacy` for other commands.
- Retain `--bootstrap` as a compatibility alias for the default pipeline and preserve bootstrap timing and Prelude selection.
- Reject conflicting pipeline flags before generation, independently of flag order.
- Update CLI help and compiler workflow documentation to explain the new default and migration command.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `bootstrap-workflow-parity`: Define default CLI pipeline selection, legacy sync selection, and pipeline flag compatibility.

## Impact

- CLI parsing and dispatch in `cmd/mygo/main.go`, with focused CLI regression coverage.
- README and compiler workflow documentation describing the default pipeline.
- Existing scripts using plain sync or build will generate bootstrap output; legacy generation requires an explicit flag.
- Existing Go compiler APIs retain their current pipeline semantics. No compiler algorithm, generated-file format, formatter behavior, or external dependency changes are planned.
