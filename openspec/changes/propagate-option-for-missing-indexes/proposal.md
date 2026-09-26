## Why

`UnwrapOr` defaults at indexed reads can silently turn a missing element into a plausible value, such as `TUnit`, an empty string, or a wildcard pattern. Across `internal/mygo`, this obscures broken length invariants and makes malformed intermediate data harder to diagnose; propagating `Option[T]` makes absence explicit at each relevant helper and caller.

## What Changes

- Audit every `UnwrapOr` use in MyGO source under `internal/mygo` and distinguish missing-index fallbacks from intentional defaults.
- Change index and recursive helpers that currently fabricate fallback values for absent elements to return `Option[T]`, and make callers explicitly handle `None`.
- Preserve `UnwrapOr(default)` when the default is intentional, including deliberate empty collection initialization or documented fallback behavior.
- Regenerate derived Go files from MyGO sources after implementation.

## Capabilities

### New Capabilities
- `mygo-option-index-handling`: Defines explicit absence propagation for missing indexed values in MyGO implementation code.

### Modified Capabilities

## Impact

- Affects MyGO source files throughout `internal/mygo`, including AST, parser, formatter, compiler, type inference, code generation, and shared utilities.
- Changes internal helper signatures and their call sites; no user-facing MyGO syntax or intended default behavior is meant to change.
- Generated Go artifacts corresponding to changed MyGO sources will need regeneration.
