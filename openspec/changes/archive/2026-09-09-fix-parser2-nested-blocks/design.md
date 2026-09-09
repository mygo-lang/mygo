## Context

See `proposal.md` for motivation. `blockUntil` was initially suspected because
the final diagnostic named `end` in a deeply nested `switch`/`if` body. Direct
parsing of the complete `stripReceiverArg` function instead localised the first
failure to `"\u0000"`: `stringChar` recognises existing single-character
escapes such as `\0`, but has no `\u` branch. Once the string parser fails, the
enclosing expression and block parsers propagate a generic failure, so the
reported missing terminator is secondary rather than causal.

## Goals / Non-Goals

**Goals:**

- Parse `\uXXXX` in ordinary quoted strings as one Unicode scalar value.
- Make malformed Unicode escapes fail at the string-literal parser rather than
  being accepted as unrelated syntax.
- Keep all existing ordinary-string escape behavior unchanged.

**Non-Goals:**

- Do not alter block terminator ownership, the AST, or type inference.
- Do not add `\UXXXXXXXX`, `\xNN`, named Unicode escapes, or escape processing
  to raw and multiline strings in this change.

## Decisions

### Parse Unicode escapes in `stringChar`

Add a `\u` branch after the existing leading backslash parser. It consumes
exactly four ASCII hexadecimal digits, converts the resulting value to a rune,
and returns that rune to the existing `String.FromRunes` construction. This
keeps escaping local to ordinary quoted strings and shares the established
parser-combinator error and backtracking behavior.

Raw scanning followed by a post-processing unescape pass was rejected: it
would duplicate current escape semantics, make diagnostics less precise, and
could accidentally change raw or multiline string behavior.

### Accept Unicode scalar values only

The decoded value SHALL be a Unicode scalar value. Surrogate values
`U+D800` through `U+DFFF` are rejected even though each fits four hexadecimal
digits, because they are UTF-16 code units rather than standalone Unicode
characters. A supplementary-plane character is out of scope for `\uXXXX` and
can be represented directly in source until a future `\UXXXXXXXX` feature is
designed.

Treating surrogate code units as runes was rejected because it would create
ill-formed Unicode strings and make future supplementary-plane support
ambiguous.

### Preserve and test the real diagnostic boundary

The parser will retain normal parse-combinator error propagation; the
regression tests assert that malformed Unicode syntax is rejected and that the
complete former bootstrap source shape parses. This avoids encoding unstable
outer `expected end` wording as the parser's contract while ensuring the root
cause cannot regress.

## Risks / Trade-offs

- [Risk] A partially consumed `\u` branch could mask another escape-parser
  alternative. -> Mitigation: commit to the Unicode branch once `\u` is read,
  and test invalid hex and truncation cases.
- [Risk] Hex conversion logic could accept non-scalars. -> Mitigation: validate
  each digit and reject the surrogate range before creating a rune.
- [Risk] Regenerated Go may diverge from MyGo source. -> Mitigation: regenerate
  `zz_parser.gen.go` through bootstrap sync and test the generated package.

## Migration Plan

No source migration is needed. Existing `\0` escapes remain valid; sources may
opt into `\u0000` and other valid `\uXXXX` escapes after the parser change.
Rollback is the normal revert of the parser source and regenerated output.
