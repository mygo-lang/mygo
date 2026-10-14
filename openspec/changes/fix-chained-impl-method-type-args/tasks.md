## 1. Reproduce and pin the regression

- [ ] 1.1 Add a codegen2 regression test that compiles a non-annotated
  `results.Receive().Unwrap().Unwrap()` chain where `results` is
  `concurrency.MakeChan[Result[PostRecord, String]]` and asserts the generated
  helper calls carry `Result[PostRecord, string]` and `[PostRecord, string]`;
  confirm it fails against the current compiler
- [ ] 1.2 Add the same-shape regression for a Prelude-defined receiver type
  (`Option[Result[Int, String]]`) and confirm it fails the same way
- [ ] 1.3 Add a build test that runs `go build` on the generated package for
  both cases and confirm the current compiler fails with `cannot infer A`

## 2. Inference normalization

- [ ] 2.1 Ensure the typed expression recorded for an unannotated binding's
  value is fully substituted so intermediate call expressions of a chain carry
  concrete types; verify tasks 1.1-1.3 now resolve the receiver type arguments
- [ ] 2.2 Widen the normalization boundary only if a covered regression still
  fails, and record which boundary was added; verify the full codegen2 and
  typeinference2 test suites still pass

## 3. Codegen spell-out for chains

- [ ] 3.1 Confirm chained receiver-method calls resolve and emit the explicit
  type-argument prefix through `receiverImplCallTypeArgs` and verify the
  generated helper signatures match `SpellCount`
- [ ] 3.2 Verify a declared receiver or result annotation no longer changes the
  emitted type arguments compared to the non-annotated form

## 4. Diagnose unresolved dispatch

- [ ] 4.1 Replace the plain-call fallback when the resolved argument count is
  below `SpellCount` with a source-located diagnostic naming the method; verify
  an unresolvable case fails generation instead of emitting a plain call
- [ ] 4.2 Stop lowering an undispatchable impl/typeclass method call to a Go
  receiver-method selector such as `recv.Unwrap()`; verify the same case reports
  a diagnostic and that `sync` no longer reports success for it

## 5. Regenerate and verify

- [ ] 5.1 Regenerate the affected `zz_*.gen.go` files in bootstrap order and
  verify a self-hosting `mygo --bootstrap sync` of the touched packages succeeds
- [ ] 5.2 Run the codegen2, typeinference2, and compiler regression suites and
  verify they pass
- [ ] 5.3 Re-run the original reproduction from the report and verify the
  generated Go builds and the previously required intermediate annotation is no
  longer needed
