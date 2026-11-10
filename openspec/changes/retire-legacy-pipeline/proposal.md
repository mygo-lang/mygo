## Why

The legacy compilation chain (`parser` -> `ast` -> `typeinference` -> `codegen`) is reachable only through `mygo --legacy sync`. The bootstrap pipeline has been the default since the 2026-10-10 pipeline switch, so the legacy chain is a frozen second implementation: roughly 31k lines of Go, a yacc toolchain, duplicate language behavior, and its own tests and specification clauses. Keeping it preserves a second set of semantics that no longer receives language features and makes every pipeline question carry two answers.

## What Changes

- Remove the `--legacy` flag and the legacy `sync` path. `sync`, `build`, and `test` always use bootstrap; `--legacy` becomes an unknown flag and fails with the normal usage error.
- Delete the legacy compiler packages `internal/mygo/parser`, `internal/mygo/ast`, `internal/mygo/typeinference`, `internal/mygo/codegen`, `internal/mygo/pkg`, and `internal/mygo/common`, including generated parser sources and their tests.
- Delete the legacy-only compiler helpers and files in `internal/mygo/compiler` (`validate.go`, `package_index.go`, `infer.go`, `types.go`, `utils.go`, and the legacy half of `api.go`), while keeping the codegen3 output and module-resolution helpers used by the bootstrap MYGO code.
- Delete `cmd/parsetest` and the root-package legacy re-exports in `internal/mygo/parser.go` and `internal/mygo/ast_aliases.go`.
- **BREAKING**: remove the legacy root-package APIs `CompileDir`, `CompileDirNoPrelude`, `Sync`, `SyncNoPrelude`, `ParseFile`, `ParseFiles`, `MustParseInt`, and the exported AST type aliases. Keep the bootstrap entry points (`CompileDirBootstrap`, `SyncBootstrap`, and their timing variants).
- Keep `--bootstrap` as a compatibility alias, `--bootstrap-timing` for stage timing, and `--no-prelude` behavior on the bootstrap path.
- Remove legacy-only CLI and compiler tests, and replace pipeline tests with bootstrap-only coverage that asserts `--legacy` is rejected as an unknown flag.
- Retire the legacy clauses from the affected specifications and update `README.md`, `docs/spec.md`, `docs/spec.zh.md`, `docs/compiler/inline-go.md`, `docs/compiler/semantics.md`, `docs/compiler2/core.md`, and `docs/compiler2/differences*.md`.
- Leave the remaining handwritten Go migration, the `codegen2` Go AST/messaging boundary, and the codegen2/codegen3 default decision out of scope. `codegen2` remains the default backend and keeps its legacy interface-based ADT ABI.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `bootstrap-workflow-parity`: the CLI no longer selects a legacy pipeline; `--bootstrap` remains the compatibility alias and the help text describes one default pipeline.
- `language/inline-go-boundary`: the legacy-backend rejection requirement is removed; inline Go behavior on the bootstrap backends is unchanged.
- `language/switch-case-guards`: the legacy-pipeline rejection requirement is removed; guarded cases remain accepted by the bootstrap pipeline.

## Impact

- Code: deletion of the legacy compiler packages and their generated files, plus the legacy half of `internal/mygo/compiler` and the root-package re-exports.
- CLI: `mygo --legacy ...` stops being a supported command; default `sync`, `build`, `test`, and `--no-prelude` behavior is unchanged.
- Public Go API: legacy entry points and AST aliases are removed; bootstrap entry points remain.
- Tests: the legacy parser, type-inference, and codegen suites are deleted; bootstrap, codegen3, LSP, and formatter suites remain.
- Specs and docs: three capability specs and the pipeline documentation lose their legacy-pipeline clauses; the legacy ADT ABI wording in codegen2/codegen3 documents stays because that is a different meaning of "legacy".
