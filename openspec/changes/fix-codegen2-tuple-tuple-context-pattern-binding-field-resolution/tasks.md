## 1. Reproduce

- [x] 1.1 Create a self-contained regression case (two-element enum subject + tuple return type + pattern-bound field used as a function argument) and verify it fails with `type AgentEvent has no field or method InitialMessage`
- [x] 1.2 Reduce to the smallest failing snippet and confirm the non-tuple-return and single-element-switch variants pass, matching the explored conditions

## 2. Fix codegen2 tuple-subject variant binding

- [x] 2.1 In `internal/mygo/codegen2/tailcall.mygo`, fix the `TuplePattern(items)` branch of `translateSwitchBranchesTail` so tuple subjects containing enum variants emit the variant type assertion and per-element dispatch. Replace the bare `bindTuplePattern(...)` + unconditional `translateAstReturnExpr(...)` with `bindTuplePatternNonVariants(...)` plus a tail-aware tuple variant dispatcher (mirroring `translate_ast.mygo`'s `translateTupleVariantSwitchStmtAt`, but keeping `translateAstReturnExpr` for each case body so tail calls are still rewritten), then verify the minimal repro's generated Go compiles
- [x] 2.2 Ensure existing tuple-returning switch cases that are currently correct still generate identical Go (no unrelated churn)

## 3. Regression and validation

- [x] 3.1 Commit a regression test that would fail without the fix and pass with it
- [x] 3.2 Run `./mygo --bootstrap build ./...` and the codegen2 test suite and confirm all tests pass
