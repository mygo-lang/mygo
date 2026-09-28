## Why

Formatting `internal/mygo/typeinference2/env.mygo` joins the `let fieldType = ...` binding and the following recursive call onto one line inside an arrow-form `case Some(field)` body. This changes the source's established multiline statement layout and makes the formatter output harder to read.

## What Changes

- Preserve statement boundaries when formatting a case branch whose body contains a `let` binding followed by a final expression.
- Add a formatter regression fixture for the reported `fieldsForStructInEnvAt` shape and retain formatter idempotence.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The existing formatter specification already requires each statement in a multiline case body to appear on its own line; this change repairs an implementation defect.

## Impact

- `internal/mygo/formatter/syntax_blocks.mygo` and its generated Go output, if the investigation confirms the renderer owns the defect.
- Formatter regression tests and generated test output.
- No public API or dependency changes.
