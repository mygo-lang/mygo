# Proposal

## Why

Long function declarations can remain difficult to scan even though the formatter already wraps over-wide comma-delimited groups. A consistent multiline signature layout will make the function name, parameters, closing parenthesis, and return type easier to read.

## What Changes

- Define a canonical multiline layout for function declaration headers whose rendered width exceeds the formatter's preferred 100-column limit.
- Apply the layout consistently to ordinary function declarations and implementation methods while keeping short headers compact.
- Preserve the signature's parameter order, types, comments, and return type, and require idempotent formatting.

## Capabilities

### New Capabilities

<!-- None -->

### Modified Capabilities

- `formatter-cli`: Specify multiline formatting for over-wide function declaration and implementation method headers.

## Impact

- Formatter layout behavior in `internal/mygo/formatter` and its formatter fixtures.
- No language syntax, parser behavior, or CLI interface changes.
