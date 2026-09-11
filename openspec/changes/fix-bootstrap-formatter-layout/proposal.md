## Why

The uncommitted parser2/formatter work corrected the declaration-nesting bug (top-level `func`/`impl` after an `interface` were indented into it) but introduced two layout regressions, and a third defect is shared by HEAD and the working tree: triple-quoted raw strings lose their relative line indentation. `mygo fmt` therefore still cannot round-trip `lib/concurrency/channel.mygo`, and three formatter golden tests fail on the working tree (`TestFormatterIfAndCaseGolden`, `TestFormatterPatternAndMultilineGolden`, `TestFormatterWideControlUsesBlocks`).

Verified by a 2x2 harness bisect of HEAD versus working-tree `zz_*.gen.go` files:

- Regression A (introduced by working-tree parser2): `case ... then ... end` inside `switch` loses its extra indent level and the switch's `end` is displaced, leaving a dangling trailing `end`. Traced to `collectLayoutEvents` discarding whole enter/exit event pairs when `verifiedLayoutAnchor` rejects an anchor, plus `branchEndAnchor` scanning from the `if-then` anchor itself instead of the parent `if`, so a case-body `end` is consumed as the branch exit.
- Regression B (introduced by working-tree parser2): wide single-line `if ... => ... else ...` is no longer expanded into block form; the expansion pass sees no usable events for the construct.
- Bug C (present at HEAD and in the working tree): lines inside triple-quoted strings are re-indented to the enclosing construct's target indent, flattening embedded Go code (e.g. the `code: """..."""` bodies in `lib/concurrency/channel.mygo`).

## What Changes

- Restore correct layout-event production in parser2 lossless for `case` bodies and wide `if`/`case` expansion: anchors are resolved by tracing the actual construct (parent-`if` exit for `if-then` branches, declaration/inline-Go exits retained), and events are never silently discarded when an anchor cannot be verified.
- Make the formatter emit protected regions, in particular triple-quoted (raw) multiline strings, verbatim: every line inside the literal, including its leading whitespace, is reproduced byte-for-byte; no dedent, re-indent, or trimming. Closing-delimiter column is preserved as part of the literal content.
- Acceptance anchors: `mygo --bootstrap fmt < lib/concurrency/channel.mygo` is byte-identical to its input, formatter/parser2/cmd tests pass, and the three failing goldens turn green without weakening their expectations.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `formatter-cli`: (1) Parser-owned layout events must remain complete and correctly anchored for `case`, `if`, `else`, and `then` constructs, including `case ... then ... end` bodies inside `switch` and wide single-line conditionals; discarding an event pair for an unverifiable anchor is not permitted. (2) Source-content preservation is strengthened for raw multiline strings: all lines of a triple-quoted literal are reproduced verbatim with their original relative indentation.

## Impact

- `internal/mygo/parser2/lossless.mygo` (layout-event construction: `collectLayoutEvents`, `verifiedLayoutAnchor`, `branchEndAnchor`) and, if needed, `parser.mygo` span propagation already in progress.
- `internal/mygo/formatter/formatter.mygo` (protected-line rendering path in `renderSourceLinesWithTargets` so protected lines bypass indent rewriting).
- Regenerated `zz_parser.gen.go`, `zz_lossless.gen.go`, `zz_formatter.gen.go`, `zz_formatter.gen_test.go` via `go run ./cmd/mygo sync` (never hand-edited).
- Tests: `internal/mygo/formatter/formatter_test.mygo` golden/behavioral tests, `internal/mygo/parser2/parser_test.go`, plus the `lib/concurrency/channel.mygo` byte-identity check as the end-to-end gate.
- Out of scope: formatter `--check` large-file performance (deferred by prior decision); unrelated working-tree hygiene (stray root-level `lossless.mygo`, `mygo` binary).
