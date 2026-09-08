## 1. Reproduce and pin the failure

- [ ] 1.1 Build a hermetic scratch fixture: a `lib` package with a tiny
  `interface` + `impl ... : Interface` (one method, generic receiver) and a
  `main`/consumer package that imports it and calls the method. Verify
  `mygo --bootstrap sync` on the consumer reproduces `unknown field <Type>.<Method>`
  (the pre-fix baseline), and that the same consumer's *free-function* calls
  still compile (proving import resolution is not the issue).
- [ ] 1.2 Add a cross-module `replace` variant of the fixture (consumer module
  with `replace <lib-module> => ./lib`) and confirm the same `unknown field`
  diagnostic, establishing that the gap is cross-package in general, not only
  cross-module.

## 2. Project imported impl methods into the importer symbol index

- [ ] 2.1 In `internal/mygo/typeinference2/types.mygo`, add a projection that,
  for each imported package's `ImplDecl`, builds each method's `Scheme` against
  the package's private env, wraps package-local type names with
  `TQualifiedName(path, ...)` (reuse `wrapPkgTypesInMonoType`), and emits a
  `Symbol.ImplMethod` keyed by `alias + "." + implReceiverName(target, iface)`
  with the method name. Verify a unit test on the projection emits one
  `ImplMethod` symbol per impl method with the `alias.Name` receiver spelling.
- [ ] 2.2 Wire the projected symbols into the initial symbol list used by
  `InferPackageWithExternal` and `InferPackageWithExternalInfo` (alongside
  `myGoPackageStructSymbols`) so they reach `SymbolIndex`. Verify the task-1.1
  sibling-package fixture now infers without `unknown field`, and that
  `ch.Send` / `ch.Receive` unify to the impl-declared return types.
- [ ] 2.3 Confirm inherent impls (no interface) project too: extend the fixture
  with an inherent `impl` method on an exported struct and verify the consumer
  call infers. Verify the projection covers both `ImplDecl` with and without
  an interface.

## 3. Re-probe codegen2 and complete it if needed (design.md D2)

- [ ] 3.1 Re-run the task-1.1 / 1.2 fixtures through `mygo --bootstrap sync`
  now that inference accepts the call. Inspect the generated Go for the
  cross-package method call and record: (a) does a dispatch candidate match the
  imported package's impl, and (b) is the emitted helper alias-qualified (e.g.
  `concurrency.MygoIT...`) or a bare unqualified symbol? Verify by reading the
  generated file and, where possible, `go build`-ing the consumer module.
- [ ] 3.2 If 3.1 shows codegen2 does NOT seed a candidate for the imported
  impl: seed dispatch candidates from the imported packages' impls in
  `internal/mygo/codegen2/decls.mygo` (`seedImplCandidates` /
  `seedInherentCandidates`), keyed by the same `alias.Name` receiver spelling.
  Verify the generated call now resolves to the imported helper.
- [ ] 3.3 If 3.1 shows the emitted helper is bare/unqualified: qualify the
  helper identifier with the import alias in `translate_ast.mygo`
  (`implMethodCallExpr` / `implMethodSymbol`) and emit the helper's explicit
  type arguments when required. Verify the generated Go `go build`s in the
  consumer module with no `undefined` helper and no failed type-inference.
- [ ] 3.4 If 3.1 shows codegen2 already seeds the candidate AND qualifies the
  helper, record the confirmation (no code change) and move on.

## 4. Regression test and validation

- [ ] 4.1 Add a bootstrap regression test (under `internal/mygo`) covering the
  cross-package interface-method call: inference succeeds, the receiver
  unifies to the impl return type, and (if codegen2 is exercised) the emitted
  helper is alias-qualified. Use the minimal fixture from task 1, not
  `lib/concurrency`, so the test is hermetic. Verify the test fails on the
  pre-fix code and passes after.
- [ ] 4.2 Re-run the cross-module `replace` consumer end to end: `mygo
  --bootstrap sync` then `go build` in the consumer module (writable `GOCACHE`).
  Verify the build succeeds and a tiny `main` runs the imported method.
- [ ] 4.3 Run `go test ./internal/mygo/...` and the repo's full bootstrap test
  suite to confirm no regression (same pass set as before, no new failures).
  Verify exit 0 / no new failures, and `git status` shows only the intended
  source and test changes (no stray generated-file churn beyond the fixtures).

## 5. Scope guard (out of scope, record only)

- [ ] 5.1 Confirm the legacy (default) pipeline is untouched: `git diff` shows
  no edits under `internal/mygo/codegen/` (the legacy bare-symbol
  `_M4Send`/`_M7Receive` + missing-type-arg gap from design.md Open Questions
  remains for a separate change). Verify by inspection.
