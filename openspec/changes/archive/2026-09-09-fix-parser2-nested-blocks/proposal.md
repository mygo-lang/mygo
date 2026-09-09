## Why

Bootstrap compilation of `typeinference2` reported `parse error: expected end`
at a nested `case ... then` body. The diagnosis was misleading: parser2 reached
the ordinary string literal `"\u0000"`, whose `\uXXXX` escape was unsupported,
then the enclosing block parser surfaced the failed expression as a missing
terminator. The source's nested `switch`, `if`, and `while` blocks were valid.

## What Changes

- Add support for four-hex-digit Unicode escapes (`\uXXXX`) in ordinary parser2
  string literals.
- Reject malformed, incomplete, and surrogate Unicode escapes with a string
  literal parse error at the escape location.
- Preserve existing ordinary-string escapes, raw strings, multiline strings,
  and block parsing behavior.
- Add parser2 regression coverage for Unicode escapes and retain the existing
  nested-block regression coverage as independent protection against real
  terminator-boundary failures.

## Capabilities

### New Capabilities

- `parser2-nested-blocks`: Correct parser2 string-escape handling so valid
  nested control-flow source is not misdiagnosed as missing an `end`.

### Modified Capabilities

<!-- No existing spec requirement is being modified; this adds parser2 behavior coverage. -->

## Impact

- Affects `internal/mygo/parser2/parser.mygo` and its generated Go output.
- Adds parser2 tests; no AST or public API changes.
- Downstream bootstrap compilation can use Unicode escapes in ordinary strings.
