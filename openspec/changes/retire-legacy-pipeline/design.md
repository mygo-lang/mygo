## Context

See proposal.md for the motivation. The current constraints that shape the removal:

- The CLI defaults to bootstrap. The only remaining selection of the legacy chain is the explicit `--legacy` flag, which is accepted only for `sync`.
- `internal/mygo/compiler/api.go` contains both legacy compile/load functions and codegen3 output-path, module-root, and import-path helpers. The latter are called from MYGO sources through FFI blocks and must survive the deletion.
- The word "legacy" also names the interface-based ADT ABI emitted by `codegen2` and compatibility constructor aliases. Those are not part of this change.
- The repository has no external consumers of `internal/` packages. The root `mygo` package is the only public Go surface that currently re-exports legacy entry points.

## Goals / Non-Goals

**Goals:**

- Remove every code path that can reach the legacy parser, type checker, or code generator, including tests, generated sources, and the yacc inputs.
- Keep default `sync`, `build`, `test`, `--no-prelude`, `--bootstrap`, and `--bootstrap-timing` behavior unchanged.
- Leave a single, documented pipeline in the CLI, specs, and compiler documentation.

**Non-Goals:**

- Migrating the remaining handwritten Go compiler code to MYGO.
- Changing the codegen2 default, its legacy interface ADT ABI, or the codegen3 opt-in policy.
- Redesigning `codegen2/goast`, metadata handling, or the Go FFI boundary.
- Preserving source compatibility for the removed root-package legacy APIs.

## Decisions

### Delete the legacy packages instead of hiding them behind a build tag

The legacy packages are removed outright. Keeping them under a `legacy` build tag would preserve two implementations, the yacc toolchain, and the generated parser while making ordinary builds skip them. A build tag would also leave the legacy tests and spec clauses in place, so the maintenance cost that motivates the retirement would remain.

Alternative considered: keep the packages but stop wiring the CLI. Rejected because the packages would still compile in `go test ./...` and continue to constrain shared helpers.

### Reject the removed flag through the normal usage path

`--legacy` is deleted from the flag parser. Because the CLI stops consuming it as a global flag, it falls through to the command switch and produces the ordinary usage error with exit status 2 before generation. A dedicated "legacy was removed" diagnostic was considered, but it would keep the removed selector in the parser and help text.

Alternative considered: accept `--legacy` and print a migration message. Rejected because the request is a complete removal and the CLI already rejects unknown selectors.

### Split `compiler/api.go` by caller, not by filename

The codegen3 helpers stay in Go because MYGO code reaches them through `go[]` blocks. The legacy compile, sync, package-loading, declaration-merging, and Prelude-loading functions are deleted together with `validate.go`, `package_index.go`, `infer.go`, `types.go`, and `utils.go`. The split is verified by building the compiler and running the codegen3 path tests, which exercise the surviving helpers.

Alternative considered: move the surviving helpers into a new Go file first. This is a mechanical refactor that can be folded into the deletion and would add a second review surface; the final package layout can keep the surviving helpers in `api.go`.

### Remove the legacy root-package API rather than redirecting it

`CompileDir`, `CompileDirNoPrelude`, `Sync`, `SyncNoPrelude`, `ParseFile`, `ParseFiles`, `MustParseInt`, and the exported AST aliases are removed. Redirecting `Sync` or `CompileDir` to bootstrap would keep the names while silently changing which pipeline they mean; `ParseFile` has no bootstrap equivalent at all. The bootstrap entry points remain available.

Alternative considered: keep `Sync` and `CompileDir` as deprecated aliases for the bootstrap functions. Rejected because the names no longer identify a distinct pipeline and the request is to remove the legacy chain.

### Keep the bootstrap compatibility flags

`--bootstrap` and `--bootstrap-timing` describe the default pipeline, not the legacy chain, so they remain. Removing them as part of this change would create a second CLI break unrelated to the retirement. `--bootstrap` can be reconsidered in a later cleanup.

### Treat the specification cleanup as part of the retirement

The legacy clauses in `bootstrap-workflow-parity`, `language/inline-go-boundary`, and `language/switch-case-guards` are removed in the same change. A follow-up cleanup would leave the archived change and the main specs describing a pipeline that no longer exists.

## Risks / Trade-offs

- [The `api.go` split drops a helper still used by MYGO code] -> Keep the codegen3 and module-resolution helpers named in the tasks, then run `go build ./...`, the codegen3 build/cache tests, and a self-host sync before committing.
- [Deleted helper packages are still referenced indirectly] -> Use `go build ./...` and `go vet`/tests as the reference gate; the compiler reports unresolved imports immediately.
- [Root-package API removal breaks an external user] -> This is declared **BREAKING** in the proposal. The bootstrap entry points stay; the migration is to call `SyncBootstrap`/`CompileDirBootstrap` and the MYGO parser through parser2.
- [Documentation or specs keep stale `--legacy` references] -> Finish with a repository-wide grep outside the archive and history files; the only remaining `legacy` mentions should refer to the codegen2 ADT ABI or historical documents.
- [Legacy tests were providing accidental coverage of shared helpers] -> Retain the shared conformance, bootstrap, codegen3, LSP, and formatter suites, and run the full `go test ./...` once with a writable `GOCACHE`.
- [The deletion touches a large number of files, making review noisy] -> Keep the change mechanical: delete whole packages first, then reduce `compiler/api.go`, then remove CLI branches and tests, then update specs and documents. Regenerate nothing unless a surviving MYGO source changes.

## Migration Plan

1. Land the package and compiler deletion with the CLI change.
2. Regenerate no MYGO output unless a surviving `.mygo` file is edited; the change only removes Go code.
3. Verify with a writable `GOCACHE`: `go build ./...`, focused `go test` for `cmd/mygo`, `internal/mygo/compiler`, `internal/mygo/codegen2`, `internal/mygo/codegen3`, `lsp`, then `./mygo sync` on a self-host package and an explicit `--codegen3` smoke run.
4. Roll back by reverting the change; the deleted packages and generated parser sources return with the commit.

No data or generated-output migration is required because the bootstrap output is unchanged.
