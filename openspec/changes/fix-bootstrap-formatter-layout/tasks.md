## 1. Regression safety net (parser2 layout events)

- [ ] 1.1 Add formatter regression tests in `internal/mygo/formatter/formatter_test.mygo` (MyGO source, then regenerate): `case ... then ... end` inside `switch` keeps case indent one level inside `switch`, case `end` aligns with `case`, and no dangling trailing `end` remains; verify the test fails on the current working tree before any fix.
- [ ] 1.2 Add a parser2 layout-event test in `internal/mygo/parser2/parser_test.go`: for the case-then-body sample and a wide single-line `if ... => ... else ...`, assert the enter/exit event pairs exist with anchors pointing at the `case`/`if`/`else`/`then` tokens (spec: "Case body with then block inside switch", "Wide single-line conditional keeps expansion events"); verify it fails on the current tree.
- [ ] 1.3 Add an `if/elsif/else` block-format regression test (guards design D1) and a raw-string golden asserting byte-verbatim interior lines for an indented `code: """..."""` literal (spec: "Embedded Go code keeps relative indentation"); verify the raw-string test fails on current HEAD behavior.

## 2. parser2 layout-event fixes (gate: parser2 green)

- [ ] 2.1 In `internal/mygo/parser2/lossless.mygo`, restore parent-`if` tracing in `branchEndAnchor` for `if-then` while retaining the `transition:if-elsif` events (design D1, defensive alignment); keep `declarationEndAnchor`/`inlineGoExitAnchor` behavior unchanged.
- [ ] 2.2 In `collectLayoutEvents`, emit enter/exit events for every handled structural kind instead of returning early on failed `verifiedLayoutAnchor`; verification selects the anchor (token-verified vs parser-owned fallback span) but never removes events (design D2).
- [ ] 2.3 In `internal/mygo/parser2/lossless.mygo`, make `finishExprEnd` retain a usable `expr.Span.End` for non-special-cased expression kinds and fall back to `tokenEnd` only when absent (design D5); composite spans such as `switch`/`if` must cover their branches, verified via the layout-event probe (switch event `affectsIndent=true`, exit at the switch's own `end`).
- [ ] 2.4 Regenerate parser2 via `GOCACHE=/tmp/... go run -buildvcs=false ./cmd/mygo sync internal/mygo/parser2` and immediately run `go test -buildvcs=false ./internal/mygo/parser2`; fix source (never `zz_*.gen.go`) until both succeed.
- [ ] 2.5 Run `go test -buildvcs=false ./internal/mygo/formatter` and the repro cases (case-indent, wide-if, wide-case) through `--bootstrap fmt`: case indent/dangling-`end` and wide-`if` expansion regressions are gone; the three failing goldens pass.

## 3. Formatter raw-string verbatim (gate: formatter green)

- [ ] 3.1 In `internal/mygo/formatter/formatter.mygo` line renderer, emit every line inside a protected span verbatim from source - no trim, no target-indent application, closing delimiter included (design D3); `TestFormatterPreservesMultilineTripleString` and the new raw-string golden pass at `.mygo`-source review level before regeneration.
- [ ] 3.2 Add an adjacent-construct test: a long wrapped delimited list on the line before a raw string keeps both the wrap and the verbatim literal (design risk: composed-line mappings).
- [ ] 3.3 Regenerate the formatter via `go run -buildvcs=false ./cmd/mygo sync internal/mygo/formatter`, then `go test -buildvcs=false ./internal/mygo/formatter ./internal/mygo/parser2 ./cmd/mygo` until green.

## 4. End-to-end acceptance

- [ ] 4.1 Verify `go run -buildvcs=false ./cmd/mygo --bootstrap fmt < lib/concurrency/channel.mygo` is byte-identical to the input (empty diff, exit 0).
- [ ] 4.2 Verify idempotence: formatting the previous output again yields no diff; confirm the formatted `channel.mygo` still compiles by running the existing `lib/concurrency` bootstrap build/tests.
- [ ] 4.3 Run the full repo test sweep for touched packages plus `git diff --check`; confirm no hand-edited `zz_*.gen.go` drift by re-running both syncs and checking for regenerated-file changes; report working-tree hygiene items (stray root `lossless.mygo`, `mygo` binary) to the user without deleting them.
