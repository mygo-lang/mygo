## 1. Define the shared contract and fixtures

- [x] 1.1 Add table-driven shared conformance fixtures for verbatim triple-quoted strings, multiline raw strings, nested/discarding tuple bindings, statement-context switch patterns, and loop control; verify each fixture records the expected acceptance, diagnostic, or runtime result for both pipelines.
- [x] 1.2 Update `docs/spec.md` and `docs/compiler/semantics.md` to specify verbatim triple-quoted strings, multiline raw strings, and `break`/`continue`; verify every documented example has a matching fixture or focused compiler test.

## 2. Production compiler lane (separate cherry-pickable commit)

- [x] 2.1 Add `break` and `continue` tokens, AST statements, loop-nesting validation, inference handling, and Go lowering to the production parser/compiler; verify valid nested loops run correctly and out-of-loop uses report source locations.
- [x] 2.2 Preserve production triple-quoted string content verbatim, including inline Go source fragments; verify parser and end-to-end inline Go tests pass.
- [x] 2.3 Extend production tests for the relevant tuple and statement-switch behavior to serve as the authoritative baseline; verify `go test ./internal/mygo/parser/... ./internal/mygo/typeinference/... ./internal/mygo/codegen/... ./internal/mygo/compiler/...` passes.
- [x] 2.4 Commit only production compiler sources and their tests as `production: add loop control and align string semantics`; verify `git show --name-only` contains no parser2, ast2, typeinference2, codegen2, or bootstrap-orchestration source changes.

## 3. Bootstrap language-parity lane (separate cherry-pickable commit)

- [x] 3.1 Migrate ast2 tuple-let bindings from flat names to recursive binding patterns and update parser2, expression-ID assignment, inference, and codegen2; verify nested patterns and `_` evaluate their right-hand side once.
- [x] 3.2 Preserve parser2 triple-quoted and raw-string content verbatim using the shared contract; verify parser2 tests cover the same fixtures as production.
- [x] 3.3 Complete statement-context switch lowering for binding, literal, tuple, wildcard, and variant patterns; verify effect-only switch fixtures compile and execute correctly.
- [x] 3.4 Validate bootstrap `break` and `continue` loop nesting and preserve nearest-loop lowering; verify both valid behavior and source-location errors for invalid placement.
- [x] 3.5 Regenerate all affected checked-in bootstrap `.gen.go` files after the MyGO source changes; verify `go test ./internal/mygo/parser2/... ./internal/mygo/ast2/... ./internal/mygo/typeinference2/... ./internal/mygo/codegen2/...` passes.
- [x] 3.6 Commit only bootstrap pipeline sources, generated bootstrap artifacts, and their tests as `feat(compiler2): align language semantics`; verify `git show --name-only` contains no production parser, AST, inference, or codegen source changes.

## 4. Bootstrap workflow parity (separate bootstrap-only commit)

- [x] 4.1 Thread a no-prelude option through bootstrap CLI entry points, state, dependency traversal, inference, and generation; verify `mygo --bootstrap --no-prelude sync` compiles a prelude-free temporary module without resolving prelude.
- [x] 4.2 Classify bootstrap source files by declared package name before inference/generation: keep `*_test.mygo` files declaring the main package in that package, and infer `name_test` files as a separate unit with main declarations as external context; add the main-package Go dot import only as codegen metadata for the external unit; verify temporary modules cover both internal tests without self-imports and external tests that call exported main-package symbols with `go test` after bootstrap sync.
- [x] 4.3 Make `GenerateSource` and `GenerateSourceAt` load the normal non-prelude external context; verify `Option` source produces Go with the required prelude import and the existing HKT regression passes.
- [x] 4.4 Preserve deterministic bootstrap written-file reporting and Go-recognized `_test.go` names; verify repeated sync runs return the same ordered file list.
- [x] 4.5 Commit only bootstrap orchestration/codegen2 sources, generated bootstrap artifacts, and workflow tests as `feat(compiler2): align compilation workflow`; verify this commit remains independently applicable without production-source edits.

## 5. Convergence verification and documentation

- [x] 5.1 Run every shared fixture through both compilation paths and compare the specified result; verify success fixtures produce parseable Go and runtime fixtures have equivalent observable output.
- [x] 5.2 Update `docs/compiler2/differences.md` to remove resolved differences and retain only deliberate, documented limitations; verify it does not describe any behavior now covered by the conformance suite.
- [x] 5.3 Run the complete production, bootstrap, compiler end-to-end, and generated-file consistency suites; verify all commands pass with a writable `GOCACHE`.
- [x] 5.4 Commit shared fixtures and documentation separately as `docs(compiler2): record bootstrap language parity`; verify the history cleanly exposes the production-only, bootstrap-only, and shared convergence commits for cherry-picking.
