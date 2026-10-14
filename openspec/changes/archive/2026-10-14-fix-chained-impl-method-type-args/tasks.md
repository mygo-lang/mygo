## 1. Reproduce and pin the regression

- [x] 1.1 Add an end-to-end fixture compiling an imported typeclass method chain
  `lib.MakeChan[Result[Int, String]](2)` then `Receive().Unwrap().Unwrap()`,
  asserting the generated Go carries `Receive[Result[int, string]]`,
  `MygoIN6OptionM6Unwrap[Result[int, string]]`, and
  `MygoIN6ResultM6Unwrap[int, string]`, and building it with `go build`
  (`TestCompileDirBootstrapCompilesChainedExplicitTypeArgumentUnwraps`)
- [x] 1.2 Add the same-shape fixture with every nominal type from the Prelude
  (`empty[Result[Int, String]]().Unwrap().Unwrap()`) and the same assertions
  (`TestCompileDirBootstrapCompilesPreludeTypeArgumentChain`)
- [x] 1.3 Confirm both fixtures fail against the pre-change compiler with
  `cannot infer A` before the inference fix lands

## 2. Apply written type arguments during inference

- [x] 2.1 Thread the call's written type arguments into `inferOrdinaryCall` and
  bind the callee scheme's bound parameters to them, composing the substitution
  into the callee result and the typed callee subtree; verify tasks 1.1-1.3 now
  resolve the receiver type arguments
- [x] 2.2 Verify a declared receiver or result annotation no longer changes the
  emitted type arguments compared to the non-annotated form

## 3. Regenerate and verify

- [x] 3.1 Regenerate `internal/mygo/typeinference2` (and `internal/mygo/codegen2`,
  whose generated files are unchanged) with a self-hosting `mygo --bootstrap
  sync` and verify the generated Go builds
- [x] 3.2 Run `go test ./...` and verify every package passes
- [x] 3.3 Re-run the original report reproduction and verify the generated Go
  builds and the previously required intermediate annotation is no longer needed

## 4. Not implemented (recorded during apply)

- [x] 4.1 Drop the planned generation-time diagnostic for unresolved receiver
  methods: inference already rejects an unresolvable method call
  (`unknown field Box.Unwrap`) before code generation, and reverting the
  receiver-side error restored a valid imported-`Measure` fixture, so the
  diagnostic would be unreachable dead code. The corresponding spec requirement
  was removed. See `design.md` - Non-Goals.
