## 1. Reproduce

- [ ] 1.1 Create a self-contained regression case (two-element enum subject + tuple return type + pattern-bound field used as a function argument) and verify it fails with `type AgentEvent has no field or method InitialMessage`
- [ ] 1.2 Reduce to the smallest failing snippet and confirm the non-tuple-return and single-element-switch variants pass, matching the explored conditions

## 2. Fix codegen2 tuple-subject variant binding

- [ ] 2.1 In `internal/mygo/codegen2/translate_ast.mygo`, make the tuple-element binding path (`bindPlainStructPatternFields` / `bindStructPatternFieldsWithTypes` or its caller) emit the enum variant type assertion (`(<elem>).(<Variant>)`) so field selectors target the asserted variant, then verify the minimal repro's generated Go compiles
- [ ] 2.2 Ensure existing tuple-returning switch cases that are currently correct still generate identical Go (no unrelated churn)

## 3. Regression and validation

- [ ] 3.1 Commit a regression test that would fail without the fix and pass with it
- [ ] 3.2 Run `./mygo --bootstrap build ./...` and the codegen2 test suite and confirm all tests pass
