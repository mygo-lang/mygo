## 1. Lock down the root cause

- [ ] 1.1 Add a parser2 / AST regression asserting that `case Ok(None)` lowers
  to `VariantPattern("Ok", [BindPattern("None")])` — documenting the intended
  context-free parse that motivates D1.
- [ ] 1.2 Add a typeinference2 regression asserting that after inference the
  same pattern is `VariantPattern("Ok", [VariantPattern("None", [])])`.

## 2. Recursive nested bare-variant resolution

- [ ] 2.1 Extend `resolveBareVariantPattern` to recurse into
  `VariantPattern` arguments, resolving each argument against the parent
  variant's field type after enum parameter substitution (design.md D2).
- [ ] 2.2 Recurse into `TuplePattern` items against the tuple element type.
- [ ] 2.3 Leave `StructVariantPattern` and `BindPattern` leaves untouched
  (design.md D3).

## 3. Verify and self-host

- [ ] 3.1 Run the typeinference2 and codegen2 unit tests.
- [ ] 3.2 Sync `typeinference2` with the existing `./mygo`, then sync
  `codegen2` with the rebuilt compiler; confirm both succeed and are
  idempotent on a second run.
- [ ] 3.3 Sync the remaining self-hosted packages (parser2, formatter) and
  confirm each succeeds.
- [ ] 3.4 Re-sync `tutorial-video-generation` and confirm
  `go build ./internal/comfy/` passes, then remove the
  `toStep(v: Option[...])` workaround in `wait.mygo` and re-verify.

## 4. Documentation

- [ ] 4.1 Add a changelog entry noting that nested bare enum constructors now
  resolve, and that stale `.gen.go` files must be regenerated.
