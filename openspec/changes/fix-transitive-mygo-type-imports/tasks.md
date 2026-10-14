## 1. Reproduction and semantic identity

- [ ] 1.1 Add an auth/storage/server fixture with an imported repository method and function returning `Result[auth.Identity, String]`; verify the unannotated consumer reproduces the missing qualifier before the fix and record the canonical return type from inference.
- [ ] 1.2 Preserve canonical foreign paths in imported signature projection wherever the fixture exposes identity loss; verify inference assertions distinguish same-named nominal types from separate packages and preserve nested type arguments.

## 2. File import planning

- [ ] 2.1 Implement recursive collection of foreign package dependencies from current-file inferred declarations, expressions, and selected impl arguments; verify focused probes cover applications, arrays, collections, tuples, and function/variadic types.
- [ ] 2.2 Integrate MyGO type dependency candidates with existing file import planning, reusing explicit paths and allocating deterministic collision-free aliases; verify alternate aliases and same-named packages produce correct imports on repeated generation.
- [ ] 2.3 Track the current canonical package path and retain only imports referenced by emitted Go; verify synthetic current-package rendering avoids self-imports and sibling files without foreign references receive no inferred import.

## 3. Contextual type rendering

- [ ] 3.1 Share contextual package resolution between string and structured type lowering, preserving nested generic parameter context; verify the three-package fixture emits qualified Identity arguments and generated typed locals consistently.
- [ ] 3.2 Propagate a diagnostic for unresolved foreign package mappings without changing context-free diagnostic formatting; verify a synthetic missing-mapping probe fails generation with the type and defining path while local and dot-imported types render correctly.

## 4. Integration validation

- [ ] 4.1 Complete generated-module runtime regressions for chained Unwrap, direct imported functions, nested foreign types, alias conflicts, per-file imports, and the existing explicit-import/type-annotation workaround; verify generated modules pass `go test -buildvcs=false` and return expected values.
- [ ] 4.2 Bootstrap-sync changed compiler packages in dependency order using a task-local writable GOCACHE, without manually editing generated Go; verify affected typeinference2, codegen2, and compiler tests pass against refreshed artifacts and the main example remains runnable.
- [ ] 4.3 Run `openspec validate fix-transitive-mygo-type-imports --strict` and review the final diff; verify every scenario has validation evidence, new handwritten MyGO files stay below 500 lines, and only scoped compiler, test, generated, and change files are modified.
