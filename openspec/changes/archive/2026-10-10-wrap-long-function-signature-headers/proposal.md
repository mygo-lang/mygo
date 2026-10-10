# Proposal

## Why

Long function declarations and calls can remain difficult to scan even though the formatter already wraps over-wide comma-delimited groups. The formatter should automatically choose multiline layouts from the width of the complete declaration header or call expression, including the callee name, so developers do not have to hand-wrap source.

## What Changes

- Define canonical multiline layouts for function declaration headers and function calls whose complete rendered width exceeds the formatter's preferred 100-column limit.
- Apply the layout consistently to ordinary function declarations and implementation methods while keeping short headers compact.
- Measure a function call together with its callee when deciding whether the argument list needs wrapping, and preserve the callee when emitting the multiline form.
- Preserve the signature's parameter order, types, comments, and return type, and require idempotent formatting.

## Capabilities

### New Capabilities

<!-- None -->

### Modified Capabilities

- `formatter-cli`: Specify automatic multiline formatting for over-wide function headers and calls.

## Impact

- Formatter layout behavior in `internal/mygo/formatter` and its formatter fixtures, covering full-width call measurement and callee-preserving wrapping.
- No language syntax, parser behavior, or CLI interface changes.
