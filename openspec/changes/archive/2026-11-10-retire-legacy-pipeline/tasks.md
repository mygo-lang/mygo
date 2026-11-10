## 1. Remove the legacy compiler surface

- [x] 1.1 Reduce `internal/mygo/compiler/api.go` to the codegen3, module-root, import-path, directory-scan, and display helpers, move the surviving import-resolution helpers to `module_paths.go`, and delete `validate.go`, `package_index.go`, `infer.go`, `types.go`, and `utils.go`; verified with `GOCACHE=/tmp/gocache-retire go build ./internal/mygo/compiler`
- [x] 1.2 Delete `internal/mygo/parser`, `internal/mygo/ast`, `internal/mygo/typeinference`, `internal/mygo/codegen`, `internal/mygo/pkg`, and `internal/mygo/common`, then delete `internal/mygo/parser.go` and `internal/mygo/ast_aliases.go`; verified `go list ./...` no longer names them and `go build ./...` succeeds
- [x] 1.3 Remove the legacy `CompileDir`, `CompileDirNoPrelude`, `Sync`, and `SyncNoPrelude` wrappers from `internal/mygo/compiler.go`; verified `go doc ./internal/mygo` lists only the bootstrap entry points
- [x] 1.4 Delete `cmd/parsetest` and confirm `rg -n "internal/mygo/(ast|parser|typeinference|codegen|pkg)" --glob '*.go'` returns no imports outside deleted paths

## 2. Simplify the CLI

- [x] 2.1 Remove the `--legacy` parsing case, its conflict checks, and the `bootstrap := !legacy` selection from `cmd/mygo/main.go`; verified `go run ./cmd/mygo --legacy sync .` exits with the usage text and the CLI tests still generate default output
- [x] 2.2 Update the usage text to describe one default bootstrap pipeline plus `--bootstrap`, `--bootstrap-timing`, `--no-prelude`, `--codegen3`, and `--opt-level`; verified `go run ./cmd/mygo 2>&1` contains no `--legacy` mention

## 3. Adjust the test surface

- [x] 3.1 Delete the legacy compiler tests `internal/mygo/compiler/{legacy_guard_test.go,e2e_switch_test.go,prelude_test.go,translate_go_test.go,go_ffi_var_parity_test.go}` and `internal/mygo/compiler_stmt_test.go`, and migrate the remaining `enum_struct_variant`, `shared_conformance`, and `switch_exhaustiveness` tests to bootstrap entry points; verified the focused compiler tests pass
- [x] 3.2 Replace the legacy cases in `cmd/mygo/pipeline_test.go` with bootstrap-only coverage and a case asserting that `--legacy` is rejected before generation; verified `go test ./cmd/mygo` passes
- [x] 3.3 Run the full suite with a writable cache and the compiler timeout, `GOCACHE=/tmp/gocache-retire go test -timeout 600s ./...`; verified all packages pass

## 4. Update the specifications and documents

- [x] 4.1 Update `README.md`, `docs/spec.md`, and `docs/spec.zh.md` so they describe `parser2 -> typeinference2 -> codegen2` as the only pipeline and remove the `--legacy sync` instructions; verified both language files name the same default pipeline
- [x] 4.2 Update `docs/compiler/inline-go.md`, `docs/compiler/semantics.md`, `docs/compiler/core.md`, `docs/compiler2/core.md`, `docs/compiler2/differences.md`, `docs/compiler2/differences.zh.md`, and `KNOWN_ISSUES.md` to remove the legacy pipeline comparison and yacc-conflict note; verified `rg -n -- "--legacy|legacy pipeline|legacy 编译链" README.md docs KNOWN_ISSUES.md` reports no legacy-pipeline wording. The main `openspec/specs` files are updated by the later sync/archive step.

## 5. Verify the integrated result

- [x] 5.1 Build the CLI and run a self-host sync, `go run ./cmd/mygo sync internal/mygo/parser2`, then inspect `git status --short`; verified the command exits 0 and no generated file changed
- [x] 5.2 Run one codegen3 smoke package, `MYGO_CACHE=off go run ./cmd/mygo --codegen3 --opt-level=0 sync lib/text/parsec`, and verify it exits 0 without writing generated files into the repository
- [x] 5.3 Run `openspec validate retire-legacy-pipeline` and confirm the change still validates after the documentation and test edits
